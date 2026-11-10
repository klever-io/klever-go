package kda_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	vmconfig "github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/vmcommon"
)

const (
	storeFuncGasCost  = uint64(100)
	storeRatePerByte  = uint64(10)
	storeGasProvided  = uint64(10_000_000)
	maxVarintFieldLen = uint64(1 + binary.MaxVarintLen64)
)

// nftRecordSize bounds the value of an NFT holder record; like contract storage, the key is not priced.
const nftRecordSize = maxVarintFieldLen

// sftNewNonceRecordsSize bounds the values a new SFT nonce stores: its MetaV2 (Circulation,
// MaxSupply, the Metadata and Hash framing and a 32-byte hash) and one holder record.
const sftNewNonceRecordsSize = 5*maxVarintFieldLen + 32

// overflowingRate is a StorePerByte whose product with a record size of 11 overflows uint64 and,
// multiplied without saturation, wraps to a small number. A variable, so the tests can compute the
// wrapped product at run time.
var overflowingRate = uint64(math.MaxUint64/maxVarintFieldLen + 1)

// newStoreRateController returns a KDA controller and a production builtin container in which
// only StorePerByte gets the test rate, so a builtin wired to another base cost fails the charge.
func newStoreRateController(t *testing.T, fixAuditChangesV7 bool) (kapp.KAppController, vmcommon.BuiltInFunctionContainer) {
	t.Helper()

	kc, err := createMockControllersWithFork(mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixAuditChangesV7))
	require.NoError(t, err)

	baseOperationCosts := vmconfig.FillGasMapBaseOperationCosts(1)
	baseOperationCosts["StorePerByte"] = storeRatePerByte

	creator, err := builtInFunctions.NewBuiltInFunctionsCreator(builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		GasMap: map[string]map[string]uint64{
			core.BaseOperationCostString: baseOperationCosts,
			core.BuiltInCostString:       vmconfig.FillGasMapBuiltInCosts(storeFuncGasCost),
		},
		Marshalizer:     &mock.ProtoMarshalizerMock{},
		AccountsCacher:  kc.GetAccountsKApp().GetAccountsCacher(),
		KAppController:  kc,
		MapDNSAddresses: make(map[string]struct{}),
		ForkController:  kc.GetForkController(),
	})
	require.NoError(t, err)
	require.NoError(t, creator.CreateBuiltInFunctionContainer())

	return kc, creator.BuiltInFunctionContainer()
}

func getBuiltin(t *testing.T, container vmcommon.BuiltInFunctionContainer, name string) vmcommon.BuiltinFunction {
	t.Helper()

	function, err := container.Get(name)
	require.NoError(t, err)

	return function
}

func processBuiltin(function vmcommon.BuiltinFunction, gas uint64, arguments ...[]byte) (*vmcommon.VMOutput, error) {
	return function.ProcessBuiltinFunction(&vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{CallerAddr: defaultSender, GasProvided: gas, Arguments: arguments},
	})
}

func mintedValue(t *testing.T, kc kapp.KAppController) int64 {
	t.Helper()

	_, asset, err := kc.GetKDAKApp().GetKDA(defaultAssetID)
	require.NoError(t, err)

	return asset.MintedValue
}

// From FixAuditChangesV7 a contract's NonFungible mint pays StorePerByte for each holder record it
// stores, and an SFT mint of a new nonce for the nonce's records; other mints and every mint before
// V7 pay only the flat cost.
func TestKDATrigger_NFTMintGasPerStoredRecord(t *testing.T) {
	t.Parallel()

	limitController, _ := newStoreRateController(t, true)
	mintBatchLimit := limitController.GetProposalController().GetParameterInt(kapps.EnumParameter_MaxNFTMintBatch)
	require.Positive(t, mintBatchLimit)

	mintAsset := func(assetTrigger vmcommon.BuiltinFunction, gas uint64, assetID []byte, nonce []byte, amount int64) (*vmcommon.VMOutput, error) {
		return processBuiltin(assetTrigger, gas,
			big.NewInt(int64(transaction.AssetTriggerContract_Mint)).Bytes(),
			assetID,
			nonce,
			big.NewInt(amount).Bytes(),
			defaultSender,
			nil,
		)
	}
	mint := func(assetTrigger vmcommon.BuiltinFunction, gas uint64, nonce []byte, amount int64) (*vmcommon.VMOutput, error) {
		return mintAsset(assetTrigger, gas, defaultAssetID, nonce, amount)
	}

	newNFT := func(t *testing.T, fixAuditChangesV7 bool) (kapp.KAppController, vmcommon.BuiltinFunction) {
		kc, container := newStoreRateController(t, fixAuditChangesV7)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

		return kc, getBuiltin(t, container, core.BuiltInFunctionAssetTrigger)
	}

	t.Run("before V7 an NFT mint pays only the flat cost", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, false)
		vmOutput, err := mint(assetTrigger, storeGasProvided, nil, 3)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost, vmOutput.GasRemaining)
		require.Equal(t, int64(3), mintedValue(t, kc))
	})

	t.Run("from V7 an NFT mint pays for each holder record", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		vmOutput, err := mint(assetTrigger, storeGasProvided, nil, 3)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-storeRatePerByte*3*nftRecordSize, vmOutput.GasRemaining)
		require.Equal(t, int64(3), mintedValue(t, kc))
	})

	t.Run("from V7 gas below the per-record cost is refused and nothing is minted", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		cost := storeFuncGasCost + storeRatePerByte*3*nftRecordSize
		vmOutput, err := mint(assetTrigger, cost-1, nil, 3)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))

		vmOutput, err = mint(assetTrigger, cost, nil, 3)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
		require.Equal(t, int64(3), mintedValue(t, kc))
	})

	t.Run("from V7 a fungible mint pays only the flat cost", func(t *testing.T) {
		t.Parallel()

		kc, container := newStoreRateController(t, true)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)
		vmOutput, err := mint(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeGasProvided, nil, 10)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost, vmOutput.GasRemaining)
	})

	t.Run("from V7 an SFT mint of an existing nonce pays only the flat cost", func(t *testing.T) {
		t.Parallel()

		kc, container := newStoreRateController(t, true)
		createSFTWithMintedNonce(t, kc, nil, false)
		vmOutput, err := mint(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeGasProvided, big.NewInt(1).Bytes(), 1000)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost, vmOutput.GasRemaining)
	})

	for _, fixAuditChangesV7 := range []bool{false, true} {
		expectedCost := storeFuncGasCost
		if fixAuditChangesV7 {
			expectedCost += storeRatePerByte * sftNewNonceRecordsSize
		}

		t.Run(fmt.Sprintf("V7=%t an SFT mint of a new nonce", fixAuditChangesV7), func(t *testing.T) {
			t.Parallel()

			kc, container := newStoreRateController(t, fixAuditChangesV7)
			createSFTWithMintedNonce(t, kc, nil, false)
			vmOutput, err := mint(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeGasProvided, nil, 10)
			require.NoError(t, err)
			require.Equal(t, storeGasProvided-expectedCost, vmOutput.GasRemaining)
			require.Equal(t, int64(2), mintedValue(t, kc))
		})
	}

	t.Run("from V7 gas below the new SFT nonce cost is refused and no nonce is created", func(t *testing.T) {
		t.Parallel()

		kc, container := newStoreRateController(t, true)
		createSFTWithMintedNonce(t, kc, nil, false)
		assetTrigger := getBuiltin(t, container, core.BuiltInFunctionAssetTrigger)
		cost := storeFuncGasCost + storeRatePerByte*sftNewNonceRecordsSize

		vmOutput, err := mint(assetTrigger, cost-1, nil, 0)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Equal(t, int64(1), mintedValue(t, kc))

		// a mint of 0 tokens still creates the nonce, so it pays the same
		vmOutput, err = mint(assetTrigger, cost, nil, 0)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
		require.Equal(t, int64(2), mintedValue(t, kc))
	})

	t.Run("from V7 the charged size covers the new SFT nonce's stored records", func(t *testing.T) {
		t.Parallel()

		kc, container := newStoreRateController(t, true)
		createSFTWithMintedNonce(t, kc, nil, false)
		_, err := mint(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeGasProvided, nil, math.MaxInt64/2)
		require.NoError(t, err)

		// read through the cache, where the mint's writes are; the record must hold its largest fields
		meta, err := kc.GetSystemAccountKApp().SFTGetMeta(defaultAssetID, []byte("2"))
		require.NoError(t, err)
		require.Equal(t, int64(math.MaxInt64/2), meta.GetCirculation())
		require.Len(t, meta.GetMetadata().GetHash(), 32)
		metaBytes, err := (&mock.ProtoMarshalizerMock{}).Marshal(meta)
		require.NoError(t, err)

		acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
		require.NoError(t, err)
		holder, err := acc.DataTrieTracker().RetrieveValue(kdautils.ToKDAKey(defaultAssetID, []byte("2")))
		require.NoError(t, err)
		require.NotEmpty(t, holder)

		require.LessOrEqual(t, uint64(len(metaBytes)+len(holder)), sftNewNonceRecordsSize)
	})

	// the charge saturates, so an amount whose charge overflows uint64 cannot wrap below the flat
	// cost and reach the KDA kapp
	t.Run("from V7 an NFT mint whose charge overflows fails for gas", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		vmOutput, err := mint(assetTrigger, storeGasProvided, nil, math.MaxInt64)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))
	})

	// with a rate whose product with the stored size overflows uint64, a wrapping multiplication
	// would leave a charge below the gas provided
	t.Run("from V7 an NFT mint whose rate times size overflows fails for gas", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		assetTrigger.SetNewGasConfig(&vmcommon.GasCost{
			BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: overflowingRate},
			BuiltInCost:       vmcommon.BuiltInCost{AssetTrigger: storeFuncGasCost},
		})
		wrappedCost := storeFuncGasCost + overflowingRate*nftRecordSize
		require.Less(t, wrappedCost, storeGasProvided)

		vmOutput, err := mint(assetTrigger, wrappedCost, nil, 1)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))
	})

	t.Run("from V7 an SFT mint of an ID ending in the separator creates a new nonce and pays for it", func(t *testing.T) {
		t.Parallel()

		kc, container := newStoreRateController(t, true)
		createSFTWithMintedNonce(t, kc, nil, false)
		vmOutput, err := mintAsset(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeGasProvided,
			append(append([]byte{}, defaultAssetID...), '/'), nil, 10)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-storeRatePerByte*sftNewNonceRecordsSize, vmOutput.GasRemaining)
		require.Equal(t, int64(2), mintedValue(t, kc))
	})

	t.Run("from V7 an asset ID ending in the separator pays like the plain ID", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		vmOutput, err := mintAsset(assetTrigger, storeGasProvided, append(append([]byte{}, defaultAssetID...), '/'), nil, 3)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-storeRatePerByte*3*nftRecordSize, vmOutput.GasRemaining)
		require.Equal(t, int64(3), mintedValue(t, kc))
	})

	t.Run("from V7 the charged record size covers the stored holder record", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		_, err := mint(assetTrigger, storeGasProvided, nil, 3)
		require.NoError(t, err)

		acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
		require.NoError(t, err)
		key := kdautils.ToKDAKey(defaultAssetID, []byte("3"))
		record, err := acc.DataTrieTracker().RetrieveValue(key)
		require.NoError(t, err)
		require.NotEmpty(t, record)
		require.LessOrEqual(t, uint64(len(record)), nftRecordSize, "the stored value fits the charged value bound")
	})

	t.Run("from V7 a mint of exactly MaxNFTMintBatch tokens pays for each holder record", func(t *testing.T) {
		t.Parallel()

		kc, assetTrigger := newNFT(t, true)
		vmOutput, err := mint(assetTrigger, storeGasProvided, nil, mintBatchLimit)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-storeRatePerByte*uint64(mintBatchLimit)*nftRecordSize, vmOutput.GasRemaining)
		require.Equal(t, mintBatchLimit, mintedValue(t, kc))
	})

	mintArgs := func(nonce []byte, amount []byte) [][]byte {
		return [][]byte{
			big.NewInt(int64(transaction.AssetTriggerContract_Mint)).Bytes(),
			defaultAssetID,
			nonce,
			amount,
			defaultSender,
			nil,
		}
	}

	// a mint that writes no holder records costs nothing extra, so the KDA kapp's own error reaches the caller
	unpricedMints := []struct {
		name   string
		amount []byte
	}{
		{"a zero amount", nil},
		// eight 0xff bytes read as -1, which would saturate the charge if it were priced
		{"a negative amount", bytes.Repeat([]byte{0xff}, 8)},
	}
	for _, unpriced := range unpricedMints {
		t.Run("from V7 "+unpriced.name+" is refused by the KDA kapp at the flat cost", func(t *testing.T) {
			t.Parallel()

			kc, assetTrigger := newNFT(t, true)
			vmOutput, err := processBuiltin(assetTrigger, storeFuncGasCost, mintArgs(nil, unpriced.amount)...)
			require.ErrorIs(t, err, process.ErrInvalidArgument)
			require.Nil(t, vmOutput)
			require.Zero(t, mintedValue(t, kc))
		})
	}

	// the charge does not repeat the KDA kapp's checks, so a mint it refuses is priced like one it
	// accepts: below the charge it fails for gas, at the charge the kapp refuses it and nothing is minted
	pricedRefusals := []struct {
		name   string
		nonce  []byte
		amount int64
	}{
		{"a nonce on an NFT", big.NewInt(1).Bytes(), 3},
		{"an amount above MaxNFTMintBatch", nil, mintBatchLimit + 1},
	}
	for _, refused := range pricedRefusals {
		t.Run("from V7 "+refused.name+" is priced, then refused by the KDA kapp", func(t *testing.T) {
			t.Parallel()

			kc, assetTrigger := newNFT(t, true)
			cost := storeFuncGasCost + storeRatePerByte*uint64(refused.amount)*nftRecordSize // #nosec G115 -- positive
			args := mintArgs(refused.nonce, big.NewInt(refused.amount).Bytes())

			vmOutput, err := processBuiltin(assetTrigger, cost-1, args...)
			require.Equal(t, common.ErrNotEnoughGas, err)
			require.Nil(t, vmOutput)

			vmOutput, err = processBuiltin(assetTrigger, cost, args...)
			require.ErrorIs(t, err, process.ErrInvalidArgument)
			require.Nil(t, vmOutput)
			require.Zero(t, mintedValue(t, kc))
		})
	}

	// with only the flat cost provided, any charge for the missing asset would surface as ErrNotEnoughGas
	t.Run("a mint of a missing asset fails the same on both sides of V7", func(t *testing.T) {
		t.Parallel()

		var errs []error
		for _, fixAuditChangesV7 := range []bool{false, true} {
			_, container := newStoreRateController(t, fixAuditChangesV7)
			vmOutput, err := mint(getBuiltin(t, container, core.BuiltInFunctionAssetTrigger), storeFuncGasCost, nil, 3)
			require.Error(t, err)
			require.Nil(t, vmOutput)
			errs = append(errs, err)
		}
		require.Equal(t, errs[0], errs[1])
		require.NotErrorIs(t, errs[1], common.ErrNotEnoughGas)
	})
}

// encodeWhitelist encodes addresses in the builtin's whitelist argument format, each with a limit of 5.
func encodeWhitelist(addresses ...[]byte) []byte {
	encoded := binary.BigEndian.AppendUint32(nil, uint32(len(addresses))) // #nosec G115
	for _, address := range addresses {
		encoded = append(encoded, address...)
		encoded = binary.BigEndian.AppendUint32(encoded, 1)
		encoded = append(encoded, 5)
	}

	return encoded
}

func whitelistAddresses(count int) [][]byte {
	addresses := make([][]byte, count)
	for i := range addresses {
		addresses[i] = makeAddress(fmt.Sprintf("whitelisted-%d", i))
	}

	return addresses
}

// whitelistRecordsSize bounds the values of the addresses' whitelist records; the keys are not priced.
func whitelistRecordsSize(addresses [][]byte) uint64 {
	return uint64(len(addresses)) * maxVarintFieldLen
}

func requireWhitelisted(t *testing.T, kc kapp.KAppController, addresses [][]byte, whitelisted bool) {
	t.Helper()

	// LoadKApp creates the ITO kapp if no ITO was configured yet, so the records are always read
	itoKApp, err := kc.GetAccountsKApp().GetAccountsCacher().LoadKApp(kapps.ITOKAppAddress)
	require.NoError(t, err)
	for _, address := range addresses {
		// a removed record is kept as a deletion marker, which reads back as an error
		record, err := itoKApp.DataTrieTracker().RetrieveValue(kdautils.ToITOWhitelistKey(defaultAssetID, hex.EncodeToString(address)))
		require.Equal(t, whitelisted, err == nil && len(record) > 0, "address %x", address)
		if whitelisted {
			require.LessOrEqual(t, uint64(len(record)), maxVarintFieldLen, "the stored value fits the charged value bound")
		}
	}
}

// From FixAuditChangesV7 a contract's ConfigITO and AddToWhitelist pay StorePerByte for each
// whitelist record they store; RemoveFromWhitelist and both calls before V7 pay only the flat cost.
func TestITOBuiltins_WhitelistGasPerStoredRecord(t *testing.T) {
	t.Parallel()

	addresses := whitelistAddresses(3)
	recordsCost := storeRatePerByte * whitelistRecordsSize(addresses)

	configITO := func(function vmcommon.BuiltinFunction, gas uint64, whitelist []byte) (*vmcommon.VMOutput, error) {
		return processBuiltin(function, gas,
			defaultAssetID, defaultSender,
			big.NewInt(int64(transaction.ConfigITOContract_PausedITO)).Bytes(),
			nil, nil, nil, nil, nil, nil, nil,
			whitelist,
			make([]byte, 4),
		)
	}

	itoTrigger := func(function vmcommon.BuiltinFunction, gas uint64, triggerType transaction.ITOTriggerContract_EnumITOTriggerType, whitelist []byte) (*vmcommon.VMOutput, error) {
		return processBuiltin(function, gas, big.NewInt(int64(triggerType)).Bytes(), defaultAssetID, whitelist)
	}

	newITO := func(t *testing.T, fixAuditChangesV7 bool) (kapp.KAppController, vmcommon.BuiltInFunctionContainer) {
		kc, container := newStoreRateController(t, fixAuditChangesV7)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

		return kc, container
	}

	for _, fixAuditChangesV7 := range []bool{false, true} {
		expectedCost := storeFuncGasCost
		if fixAuditChangesV7 {
			expectedCost += recordsCost
		}

		t.Run(fmt.Sprintf("V7=%t ConfigITO with a whitelist", fixAuditChangesV7), func(t *testing.T) {
			t.Parallel()

			kc, container := newITO(t, fixAuditChangesV7)
			vmOutput, err := configITO(getBuiltin(t, container, core.BuiltInFunctionConfigITO), storeGasProvided, encodeWhitelist(addresses...))
			require.NoError(t, err)
			require.Equal(t, storeGasProvided-expectedCost, vmOutput.GasRemaining)
			requireWhitelisted(t, kc, addresses, true)
		})

		t.Run(fmt.Sprintf("V7=%t AddToWhitelist", fixAuditChangesV7), func(t *testing.T) {
			t.Parallel()

			kc, container := newITO(t, fixAuditChangesV7)
			_, err := configITO(getBuiltin(t, container, core.BuiltInFunctionConfigITO), storeGasProvided, encodeWhitelist())
			require.NoError(t, err)

			itoTriggerFunc := getBuiltin(t, container, core.BuiltInFunctionITOTrigger)
			vmOutput, err := itoTrigger(itoTriggerFunc, storeGasProvided, transaction.ITOTriggerContract_AddToWhitelist, encodeWhitelist(addresses...))
			require.NoError(t, err)
			require.Equal(t, storeGasProvided-expectedCost, vmOutput.GasRemaining)
			requireWhitelisted(t, kc, addresses, true)

			vmOutput, err = itoTrigger(itoTriggerFunc, storeGasProvided, transaction.ITOTriggerContract_RemoveFromWhitelist, encodeWhitelist(addresses...))
			require.NoError(t, err)
			require.Equal(t, storeGasProvided-storeFuncGasCost, vmOutput.GasRemaining)
			requireWhitelisted(t, kc, addresses, false)
		})
	}

	// only StorePerByte changes, so a builtin that keeps the old rate after a GasScheduleChange fails
	t.Run("from V7 a new gas config sets the per-byte rate", func(t *testing.T) {
		t.Parallel()

		newRate := &vmcommon.GasCost{
			BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: 2 * storeRatePerByte},
			BuiltInCost:       vmcommon.BuiltInCost{ConfigITO: storeFuncGasCost, ITOTrigger: storeFuncGasCost},
		}

		kc, container := newITO(t, true)
		configITOFunc := getBuiltin(t, container, core.BuiltInFunctionConfigITO)
		configITOFunc.SetNewGasConfig(newRate)
		vmOutput, err := configITO(configITOFunc, storeGasProvided, encodeWhitelist(addresses...))
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-2*recordsCost, vmOutput.GasRemaining)
		requireWhitelisted(t, kc, addresses, true)

		kc, container = newITO(t, true)
		_, err = configITO(getBuiltin(t, container, core.BuiltInFunctionConfigITO), storeGasProvided, encodeWhitelist())
		require.NoError(t, err)
		itoTriggerFunc := getBuiltin(t, container, core.BuiltInFunctionITOTrigger)
		itoTriggerFunc.SetNewGasConfig(newRate)
		vmOutput, err = itoTrigger(itoTriggerFunc, storeGasProvided, transaction.ITOTriggerContract_AddToWhitelist, encodeWhitelist(addresses...))
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-2*recordsCost, vmOutput.GasRemaining)
		requireWhitelisted(t, kc, addresses, true)
	})

	t.Run("from V7 gas below the per-record cost is refused and nothing is whitelisted", func(t *testing.T) {
		t.Parallel()

		kc, container := newITO(t, true)
		configITOFunc := getBuiltin(t, container, core.BuiltInFunctionConfigITO)
		vmOutput, err := configITO(configITOFunc, storeFuncGasCost+recordsCost-1, encodeWhitelist(addresses...))
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		requireWhitelisted(t, kc, addresses, false)

		_, err = configITO(configITOFunc, storeGasProvided, encodeWhitelist())
		require.NoError(t, err)

		itoTriggerFunc := getBuiltin(t, container, core.BuiltInFunctionITOTrigger)
		vmOutput, err = itoTrigger(itoTriggerFunc, storeFuncGasCost+recordsCost-1, transaction.ITOTriggerContract_AddToWhitelist, encodeWhitelist(addresses...))
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		requireWhitelisted(t, kc, addresses, false)

		vmOutput, err = itoTrigger(itoTriggerFunc, storeFuncGasCost+recordsCost, transaction.ITOTriggerContract_AddToWhitelist, encodeWhitelist(addresses...))
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
		requireWhitelisted(t, kc, addresses, true)
	})
}

// encodePacks encodes one pack of amount tokens priced at price units of currency, in the
// builtin's pack argument format.
func encodePacks(currency []byte, amount int64, price int64) []byte {
	encodeBigUint := func(encoded []byte, value int64) []byte {
		valueBytes := big.NewInt(value).Bytes()
		encoded = binary.BigEndian.AppendUint32(encoded, uint32(len(valueBytes))) // #nosec G115
		return append(encoded, valueBytes...)
	}

	encoded := binary.BigEndian.AppendUint32(nil, 1)
	encoded = binary.BigEndian.AppendUint32(encoded, uint32(len(currency))) // #nosec G115
	encoded = append(encoded, currency...)
	encoded = binary.BigEndian.AppendUint32(encoded, 1)
	encoded = encodeBigUint(encoded, amount)

	return encodeBigUint(encoded, price)
}

// From FixAuditChangesV7 a contract's ITO buy of an NFT pack pays StorePerByte for each holder
// record the mint stores, as a direct NFT mint does, and a transfer for each account it creates
// (KLC-2938); before V7 it pays only the flat cost.
func TestKleverBuy_ITOBuyGasPerStoredRecord(t *testing.T) {
	t.Parallel()

	const packAmount, packPrice = int64(3), int64(1)
	recordsCost := storeRatePerByte * uint64(packAmount) * nftRecordSize
	// the buy pays the ITO receiver, which has no account yet; every other payee is the buyer. Like
	// every builtin cost here, a transfer costs storeFuncGasCost.
	itoReceiver := makeAddress("fresh-ito-receiver")
	const createdAccounts = 1
	createdCost := createdAccounts * storeFuncGasCost

	newNFTITO := func(t *testing.T, fixAuditChangesV7 bool) (kapp.KAppController, vmcommon.BuiltinFunction) {
		kc, container := newStoreRateController(t, fixAuditChangesV7)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

		// the pack is priced in KLV, which the ITO checks for as an asset; the sender holds KLV
		kdaKApp, _, err := kc.GetKDAKApp().GetKDA(nil)
		require.NoError(t, err)
		klv, err := (&mock.ProtoMarshalizerMock{}).Marshal(&kapps.KDAData{
			ID:         kdautils.KLVIdentifier,
			Ticker:     kdautils.KLVIdentifier,
			AssetType:  kapps.KDAData_Fungible,
			Precision:  6,
			Properties: &kapps.PropertiesData{},
			Attributes: &kapps.AttributesData{},
		})
		require.NoError(t, err)
		require.NoError(t, kdaKApp.DataTrieTracker().SaveKeyValue(kdautils.ToKDAKey(kdautils.KLVIdentifier, nil), klv))
		require.NoError(t, kc.GetAccountsKApp().GetAccountsCacher().UpdateKapp(kdaKApp))
		buyer, err := kc.GetAccountsKApp().GetAccountsCacher().LoadUser(defaultSender)
		require.NoError(t, err)
		require.NoError(t, buyer.AddToBalance(1_000_000, nil, true))
		require.NoError(t, kc.GetAccountsKApp().GetAccountsCacher().UpdateUser(buyer))

		_, err = processBuiltin(getBuiltin(t, container, core.BuiltInFunctionConfigITO), storeGasProvided,
			defaultAssetID, itoReceiver,
			big.NewInt(int64(transaction.ConfigITOContract_ActiveITO)).Bytes(),
			nil, nil, nil, nil, nil, nil, nil,
			encodeWhitelist(),
			encodePacks(kdautils.KLVIdentifier, packAmount, packPrice),
		)
		require.NoError(t, err)

		return kc, getBuiltin(t, container, core.BuiltInFunctionBuy)
	}

	buy := func(buyFunc vmcommon.BuiltinFunction, gas uint64) (*vmcommon.VMOutput, error) {
		return processBuiltin(buyFunc, gas,
			big.NewInt(int64(transaction.BuyContract_ITOBuy)).Bytes(),
			defaultAssetID,
			kdautils.KLVIdentifier,
			big.NewInt(packAmount).Bytes(),
			big.NewInt(packAmount*packPrice).Bytes(),
		)
	}

	for _, fixAuditChangesV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("V7=%t an ITO buy of an NFT pack", fixAuditChangesV7), func(t *testing.T) {
			t.Parallel()

			kc, buyFunc := newNFTITO(t, fixAuditChangesV7)
			expectedCost := storeFuncGasCost
			if fixAuditChangesV7 {
				expectedCost += recordsCost + createdCost
			}
			vmOutput, err := buy(buyFunc, storeGasProvided)
			require.NoError(t, err)
			require.Equal(t, storeGasProvided-expectedCost, vmOutput.GasRemaining)
			require.Equal(t, packAmount, mintedValue(t, kc))
		})
	}

	t.Run("from V7 gas below the per-record cost is refused and nothing is minted", func(t *testing.T) {
		t.Parallel()

		kc, buyFunc := newNFTITO(t, true)
		vmOutput, err := buy(buyFunc, storeFuncGasCost+recordsCost-1)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))
	})

	// the accounts a buy creates are charged after the ITO kapp has run, so the call fails then and the
	// transaction discards what the kapp wrote, as txProcess does with ResetAll
	t.Run("from V7 gas below the created-account cost fails the buy", func(t *testing.T) {
		t.Parallel()

		kc, buyFunc := newNFTITO(t, true)
		cacher := kc.GetAccountsKApp().GetAccountsCacher()
		// the asset and its ITO come from earlier transactions
		require.NoError(t, cacher.SaveAll())
		cost := storeFuncGasCost + recordsCost + createdCost
		vmOutput, err := buy(buyFunc, cost-1)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		cacher.ResetAll(true)
		require.Zero(t, mintedValue(t, kc))

		vmOutput, err = buy(buyFunc, cost)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
		require.Equal(t, packAmount, mintedValue(t, kc))
	})

	// the charge saturates, so an amount whose charge overflows uint64 cannot wrap below the flat
	// cost and reach the ITO kapp
	t.Run("from V7 an ITO buy whose charge overflows fails for gas", func(t *testing.T) {
		t.Parallel()

		kc, buyFunc := newNFTITO(t, true)
		vmOutput, err := processBuiltin(buyFunc, storeGasProvided,
			big.NewInt(int64(transaction.BuyContract_ITOBuy)).Bytes(),
			defaultAssetID,
			kdautils.KLVIdentifier,
			big.NewInt(math.MaxInt64).Bytes(),
			big.NewInt(packAmount*packPrice).Bytes(),
		)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))
	})

	// with a rate whose product with the stored size overflows uint64, a wrapping multiplication
	// would leave a charge below the gas provided
	t.Run("from V7 an ITO buy whose rate times size overflows fails for gas", func(t *testing.T) {
		t.Parallel()

		kc, buyFunc := newNFTITO(t, true)
		buyFunc.SetNewGasConfig(&vmcommon.GasCost{
			BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: overflowingRate},
			BuiltInCost:       vmcommon.BuiltInCost{Buy: storeFuncGasCost},
		})
		wrappedCost := storeFuncGasCost + overflowingRate*uint64(packAmount)*nftRecordSize
		require.Less(t, wrappedCost, storeGasProvided)

		vmOutput, err := buy(buyFunc, wrappedCost)
		require.Equal(t, common.ErrNotEnoughGas, err)
		require.Nil(t, vmOutput)
		require.Zero(t, mintedValue(t, kc))
	})

	// the ITO kapp refuses every SFT buy (an SFT cannot have an ITO), but the charge does not repeat
	// that check: from V7 a buy of an SFT ID is priced as a new nonce, then refused by the ITO kapp
	t.Run("an ITO buy of an SFT is refused by the ITO kapp on both sides of V7", func(t *testing.T) {
		t.Parallel()

		buyArgs := [][]byte{
			big.NewInt(int64(transaction.BuyContract_ITOBuy)).Bytes(),
			defaultAssetID,
			kdautils.KLVIdentifier,
			big.NewInt(1).Bytes(),
			big.NewInt(packPrice).Bytes(),
		}

		var errs []error
		for _, fixAuditChangesV7 := range []bool{false, true} {
			kc, container := newStoreRateController(t, fixAuditChangesV7)
			createSFTWithMintedNonce(t, kc, nil, false)
			buyFunc := getBuiltin(t, container, core.BuiltInFunctionBuy)

			gas := storeFuncGasCost
			if fixAuditChangesV7 {
				gas += storeRatePerByte * sftNewNonceRecordsSize
				vmOutput, err := processBuiltin(buyFunc, gas-1, buyArgs...)
				require.Equal(t, common.ErrNotEnoughGas, err)
				require.Nil(t, vmOutput)
			}

			vmOutput, err := processBuiltin(buyFunc, gas, buyArgs...)
			require.Error(t, err)
			require.Nil(t, vmOutput)
			require.Equal(t, int64(1), mintedValue(t, kc))
			errs = append(errs, err)
		}
		require.Equal(t, errs[0], errs[1])
		require.NotErrorIs(t, errs[1], common.ErrNotEnoughGas)
	})

	// only StorePerByte changes, so a builtin that keeps the old rate after a GasScheduleChange fails
	t.Run("from V7 a new gas config sets the per-byte rate", func(t *testing.T) {
		t.Parallel()

		kc, buyFunc := newNFTITO(t, true)
		buyFunc.SetNewGasConfig(&vmcommon.GasCost{
			BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: 2 * storeRatePerByte},
			BuiltInCost:       vmcommon.BuiltInCost{Buy: storeFuncGasCost},
		})
		vmOutput, err := buy(buyFunc, storeGasProvided)
		require.NoError(t, err)
		require.Equal(t, storeGasProvided-storeFuncGasCost-2*recordsCost, vmOutput.GasRemaining)
		require.Equal(t, packAmount, mintedValue(t, kc))
	})

	// the ITO kapp loads the asset by the ID as given, so it refuses an ID ending in the separator;
	// the charge does not repeat that check, so from V7 the buy is priced before the ITO kapp refuses it
	t.Run("an ITO buy of an ID ending in the separator is refused by the ITO kapp on both sides of V7", func(t *testing.T) {
		t.Parallel()

		buyArgs := [][]byte{
			big.NewInt(int64(transaction.BuyContract_ITOBuy)).Bytes(),
			append(append([]byte{}, defaultAssetID...), '/'),
			kdautils.KLVIdentifier,
			big.NewInt(packAmount).Bytes(),
			big.NewInt(packAmount * packPrice).Bytes(),
		}

		var errs []error
		for _, fixAuditChangesV7 := range []bool{false, true} {
			kc, buyFunc := newNFTITO(t, fixAuditChangesV7)
			if fixAuditChangesV7 {
				vmOutput, err := processBuiltin(buyFunc, storeFuncGasCost+recordsCost-1, buyArgs...)
				require.Equal(t, common.ErrNotEnoughGas, err)
				require.Nil(t, vmOutput)
			}

			vmOutput, err := processBuiltin(buyFunc, storeGasProvided, buyArgs...)
			require.Error(t, err)
			require.Nil(t, vmOutput)
			require.Zero(t, mintedValue(t, kc))
			errs = append(errs, err)
		}
		require.Equal(t, errs[0], errs[1])
		require.NotErrorIs(t, errs[1], common.ErrNotEnoughGas)
	})
}
