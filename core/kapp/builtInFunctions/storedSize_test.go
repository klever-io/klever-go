package builtInFunctions

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
)

const (
	sizeTestMintBatch = int64(10)
	sizeTestRate      = uint64(10)
	sizeTestFuncCost  = uint64(100)
)

var sizeTestAssetID = []byte("NFT-1234")

// sizeTestController returns a controller whose KDA kapp loads every asset as the given one, or
// fails with loadErr, and whose current contract is contractID.
func sizeTestController(asset *kapps.KDAData, loadErr error, contractID int) *stub.KAppControllerStub {
	return &stub.KAppControllerStub{
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, asset, loadErr
				},
			}
		},
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &mock.ProposalControllerStub{
				GetParameterIntCalled: func(_ kapps.EnumParameter) int64 { return sizeTestMintBatch },
			}
		},
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return &mock.KAppContextStub{ContractIDCalled: func() int { return contractID }}
		},
	}
}

// nftHolderRecordSize bounds the value of one NFT holder record.
const nftHolderRecordSize = maxVarintFieldSize

// The value bounds cover the largest record the kapps store: a whitelist limit and an NFT holder
// balance, at any int64.
func TestMaxVarintFieldSize_BoundsStoredValues(t *testing.T) {
	t.Parallel()

	marshaller := &mock.ProtoMarshalizerMock{}
	for _, value := range []int64{1, math.MaxInt64, math.MinInt64, -1} {
		whitelist, err := marshaller.Marshal(&kapps.WhitelistData{Limit: value})
		require.NoError(t, err)
		require.LessOrEqual(t, uint64(len(whitelist)), maxVarintFieldSize)

		holder, err := marshaller.Marshal(&kapps.UserKDA{Balance: value})
		require.NoError(t, err)
		require.LessOrEqual(t, uint64(len(holder)), maxVarintFieldSize)
	}
}

func TestWhitelistStoredSize(t *testing.T) {
	t.Parallel()

	require.Zero(t, whitelistStoredSize(nil))

	whitelist := map[string]*transaction.WhitelistInfo{
		"aa":   {Limit: 1},
		"bbbb": {Limit: 2},
	}
	require.Equal(t, 2*maxVarintFieldSize, whitelistStoredSize(whitelist))
}

func TestMintStoredSize(t *testing.T) {
	t.Parallel()

	nft := &kapps.KDAData{AssetType: kapps.KDAData_NonFungible}

	t.Run("an NFT mint pays for one holder record per token", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, 3*nftHolderRecordSize, mintStoredSize(sizeTestController(nft, nil, 0), sizeTestAssetID, 3))
	})

	t.Run("an ID ending in the separator is priced as the asset", func(t *testing.T) {
		t.Parallel()

		assetID := append(append([]byte{}, sizeTestAssetID...), kapps.Sp...)
		require.Equal(t, 2*nftHolderRecordSize, mintStoredSize(sizeTestController(nft, nil, 0), assetID, 2))
	})

	t.Run("the batch limit itself is priced", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, uint64(sizeTestMintBatch)*nftHolderRecordSize,
			mintStoredSize(sizeTestController(nft, nil, 0), sizeTestAssetID, sizeTestMintBatch))
	})

	// the kapp refuses these, but the charge does not repeat its checks: they are priced as written
	for name, tc := range map[string]struct {
		assetID []byte
		amount  int64
	}{
		"an amount above the batch limit": {assetID: sizeTestAssetID, amount: sizeTestMintBatch + 1},
		"a nonce after the separator":     {assetID: []byte("NFT-1234/5"), amount: 1},
		"more than one separator":         {assetID: []byte("NFT-1234//"), amount: 1},
	} {
		t.Run(name+" is priced", func(t *testing.T) {
			t.Parallel()

			require.Equal(t, uint64(tc.amount)*nftHolderRecordSize, // #nosec G115 -- positive
				mintStoredSize(sizeTestController(nft, nil, 0), tc.assetID, tc.amount))
		})
	}

	t.Run("the asset is the part before the first separator", func(t *testing.T) {
		t.Parallel()

		var loaded []byte
		controller := sizeTestController(nft, nil, 0)
		controller.GetKDAKAppCalled = func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(id []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					loaded = id
					return nil, nft, nil
				},
			}
		}
		require.Equal(t, nftHolderRecordSize, mintStoredSize(controller, []byte("NFT-1234/5/6"), 1))
		require.Equal(t, sizeTestAssetID, loaded)
	})

	t.Run("an amount that overflows saturates", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, uint64(math.MaxUint64), mintStoredSize(sizeTestController(nft, nil, 0), sizeTestAssetID, math.MaxInt64))
	})

	sft := &kapps.KDAData{AssetType: kapps.KDAData_SemiFungible}

	// a mint without a nonce creates one, whatever the amount: the kapp creates it for 0 too
	for name, tc := range map[string]struct {
		assetID []byte
		amount  int64
	}{
		"an SFT mint of a new nonce":                 {assetID: sizeTestAssetID, amount: 1000},
		"an SFT mint of a new nonce with 0 tokens":   {assetID: sizeTestAssetID, amount: 0},
		"an SFT mint of an ID ending in a separator": {assetID: []byte("NFT-1234/"), amount: 1},
	} {
		t.Run(name+" pays for the nonce's records", func(t *testing.T) {
			t.Parallel()

			require.Equal(t, sftNewNonceSize, mintStoredSize(sizeTestController(sft, nil, 0), tc.assetID, tc.amount))
		})
	}

	for name, tc := range map[string]struct {
		asset   *kapps.KDAData
		loadErr error
		assetID []byte
		amount  int64
	}{
		"an NFT mint of 0 tokens":          {asset: nft, assetID: sizeTestAssetID, amount: 0},
		"a negative NFT amount":            {asset: nft, assetID: sizeTestAssetID, amount: -1},
		"a negative SFT amount":            {asset: sft, assetID: sizeTestAssetID, amount: -1},
		"an asset that fails to load":      {asset: nft, loadErr: errors.New("load"), assetID: sizeTestAssetID, amount: 1},
		"an asset that loads as no record": {asset: nil, assetID: sizeTestAssetID, amount: 1},
		"a fungible asset":                 {asset: &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, assetID: sizeTestAssetID, amount: 1},
		"an SFT mint of an existing nonce": {asset: sft, assetID: []byte("NFT-1234/5"), amount: 1000},
	} {
		t.Run(name+" adds no record", func(t *testing.T) {
			t.Parallel()

			require.Zero(t, mintStoredSize(sizeTestController(tc.asset, tc.loadErr, 0), tc.assetID, tc.amount))
		})
	}
}

// sftNewNonceSize covers the largest records a new SFT nonce stores: its MetaV2 in the system
// account, with a hash of the node's hasher size, and a holder record.
func TestSFTNewNonceSize_BoundsStoredRecords(t *testing.T) {
	t.Parallel()

	marshaller := &mock.ProtoMarshalizerMock{}
	meta, err := marshaller.Marshal(&kapps.MetaV2{
		Circulation: math.MinInt64,
		MaxSupply:   math.MinInt64,
		Metadata:    &kapps.MetaV2Data{Hash: make([]byte, sftMetaHashSize)},
	})
	require.NoError(t, err)
	holder, err := marshaller.Marshal(&kapps.UserKDA{Balance: math.MinInt64})
	require.NoError(t, err)

	require.LessOrEqual(t, uint64(len(meta)+len(holder)), sftNewNonceSize)
	require.Equal(t, sftMetaHashSize, (&blake2b.Blake2b{}).Size(), "the node's hasher digest")
}

func TestMetadataSize(t *testing.T) {
	t.Parallel()

	contract := &transaction.AssetTriggerContract{MIME: []byte("image/png")}
	txData := [][]byte{[]byte("first"), []byte("second-entry")}
	base := uint64(len(contract.MIME)) + 2*maxVarintFieldSize

	require.Equal(t, base+uint64(len(txData[0])), metadataSize(contract, txData, 0))
	require.Equal(t, base+uint64(len(txData[1])), metadataSize(contract, txData, 1))
	require.Equal(t, base, metadataSize(contract, txData, 2), "a contract ID past the data counts only the MIME")
	require.Equal(t, base, metadataSize(contract, txData, -1), "a negative contract ID counts only the MIME")
	require.Equal(t, base, metadataSize(contract, nil, 0))
}

func TestKleverAssetTrigger_StoredSize(t *testing.T) {
	t.Parallel()

	nft := &kapps.KDAData{AssetType: kapps.KDAData_NonFungible}
	e := &kleverAssetTrigger{kappController: sizeTestController(nft, nil, 1)}
	txData := [][]byte{[]byte("zero"), []byte("one-entry")}

	updateMetadata := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		MIME:        []byte("text/plain"),
	}
	require.Equal(t, metadataSize(updateMetadata, txData, 1), e.storedSize(updateMetadata, txData))

	mint := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     sizeTestAssetID,
		Amount:      4,
	}
	require.Equal(t, 4*nftHolderRecordSize, e.storedSize(mint, txData))

	logo := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateLogo,
		Logo:        strings.Repeat("a", 100),
	}
	require.Zero(t, e.storedSize(logo, txData))
}

// SetNewGasConfig refreshes the per-byte rate along with the flat cost on every builtin that charges
// per stored byte, so a GasScheduleChange reaches nodes that do not restart.
func TestSetNewGasConfig_RefreshesStorePerByte(t *testing.T) {
	t.Parallel()

	gasCost := &vmcommon.GasCost{
		BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: 7},
		BuiltInCost: vmcommon.BuiltInCost{
			AssetTrigger: 11,
			Buy:          12,
			ConfigITO:    13,
			ITOTrigger:   14,
		},
	}

	assetTrigger := &kleverAssetTrigger{}
	assetTrigger.SetNewGasConfig(nil)
	assetTrigger.SetNewGasConfig(gasCost)
	require.Equal(t, uint64(11), assetTrigger.funcGasCost)
	require.Equal(t, uint64(7), assetTrigger.storePerByte)

	buy := &kleverBuy{}
	buy.SetNewGasConfig(nil)
	buy.SetNewGasConfig(gasCost)
	require.Equal(t, uint64(12), buy.funcGasCost)
	require.Equal(t, uint64(7), buy.storePerByte)

	configITO := &kleverConfigITO{}
	configITO.SetNewGasConfig(nil)
	configITO.SetNewGasConfig(gasCost)
	require.Equal(t, uint64(13), configITO.funcGasCost)
	require.Equal(t, uint64(7), configITO.storePerByte)

	itoTrigger := &kleverITOTrigger{}
	itoTrigger.SetNewGasConfig(nil)
	itoTrigger.SetNewGasConfig(gasCost)
	require.Equal(t, uint64(14), itoTrigger.funcGasCost)
	require.Equal(t, uint64(7), itoTrigger.storePerByte)
}

// itoKappTriggerStub accepts every ITO trigger.
type itoKappTriggerStub struct {
	kapp.ITOKapp
}

func (s *itoKappTriggerStub) Trigger(_ []byte, _ *transaction.ITOTriggerContract) (transaction.Transaction_TXResultCode, error) {
	return transaction.Transaction_Ok, nil
}

// From FixAuditChangesV7 an AddToWhitelist pays per stored whitelist record; RemoveFromWhitelist
// and every trigger before V7 pay only the flat cost.
func TestKleverITOTrigger_WhitelistGasPerStoredRecord(t *testing.T) {
	t.Parallel()

	addresses := [][]byte{make([]byte, 32), make([]byte, 32)}
	addresses[1][0] = 1
	whitelist := binary.BigEndian.AppendUint32(nil, uint32(len(addresses))) // #nosec G115
	recordsSize := uint64(0)
	for _, address := range addresses {
		whitelist = append(whitelist, address...)
		whitelist = binary.BigEndian.AppendUint32(whitelist, 1)
		whitelist = append(whitelist, 5)
		recordsSize += maxVarintFieldSize
	}
	recordsCost := sizeTestRate * recordsSize

	newITOTrigger := func(t *testing.T, fixAuditChangesV7 bool) vmcommon.BuiltinFunction {
		t.Helper()

		itoTrigger, err := NewKleverITOTriggerFunc(
			sizeTestFuncCost,
			sizeTestRate,
			&mock.ProtoMarshalizerMock{},
			&mock.AccountsCacherStub{},
			mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixAuditChangesV7),
			&stub.KAppControllerStub{GetITOKAppCalled: func() kapp.ITOKapp { return &itoKappTriggerStub{} }},
		)
		require.NoError(t, err)

		return itoTrigger
	}

	trigger := func(itoTrigger vmcommon.BuiltinFunction, gas uint64, triggerType transaction.ITOTriggerContract_EnumITOTriggerType) (*vmcommon.VMOutput, error) {
		return itoTrigger.ProcessBuiltinFunction(&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallerAddr:  make([]byte, 32),
				GasProvided: gas,
				Arguments:   [][]byte{big.NewInt(int64(triggerType)).Bytes(), sizeTestAssetID, whitelist},
			},
		})
	}

	t.Run("before V7 AddToWhitelist pays only the flat cost", func(t *testing.T) {
		t.Parallel()

		vmOutput, err := trigger(newITOTrigger(t, false), sizeTestFuncCost, transaction.ITOTriggerContract_AddToWhitelist)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
	})

	t.Run("from V7 AddToWhitelist pays for each record", func(t *testing.T) {
		t.Parallel()

		itoTrigger := newITOTrigger(t, true)
		vmOutput, err := trigger(itoTrigger, sizeTestFuncCost+recordsCost-1, transaction.ITOTriggerContract_AddToWhitelist)
		require.ErrorIs(t, err, common.ErrNotEnoughGas)
		require.Nil(t, vmOutput)

		vmOutput, err = trigger(itoTrigger, sizeTestFuncCost+recordsCost, transaction.ITOTriggerContract_AddToWhitelist)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
	})

	t.Run("from V7 RemoveFromWhitelist pays only the flat cost", func(t *testing.T) {
		t.Parallel()

		vmOutput, err := trigger(newITOTrigger(t, true), sizeTestFuncCost, transaction.ITOTriggerContract_RemoveFromWhitelist)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
	})
}
