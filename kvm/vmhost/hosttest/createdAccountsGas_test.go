package hostCoretest

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kapps"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

const (
	// rounds and transfers in the large calls; the small calls do one round and one batch, so the
	// difference prices a repetition without the cost of the call itself
	repetitions = 4
	// the ITO sells 1 unit for packPrice KLV; half the price is ITO royalties, 1% of that to each split,
	// so every split and the royalties receiver is paid a non-zero amount
	packPrice          = 1_000_000
	itoPercentage      = 5000
	splitITOPercentage = 100
)

var createdAccountsAsset = []byte("CAG-1234")

// TestCreatedAccountsGas records the gas a contract pays for the accounts that the builtins it calls
// create (KLC-2938). A round is KleverAssetTrigger UpdateRoyalties to MaxTransferRoyalties fresh split
// addresses and a fresh royalties receiver, then a KleverBuy of the asset's ITO, which pays all of them
// and creates each with LoadUserAccount. The baseline sends KLV to as many fresh addresses. Each side
// runs once with one repetition and once with several in a single call, against the node's gas
// schedule. Every call must succeed and create the expected accounts, and the gas per account is
// logged.
//
// It runs on both sides of FixAuditChangesV7. Before it, an account the Buy creates costs less than a
// transfer. From it, KleverBuy charges a contract transfer (the VM's TransferValue and the Transfer
// builtin) for each account it creates, so it costs at least what the contract's own transfer does.
// A Buy given one gas less than that fails for gas after the ITO kapp has paid everyone, and once
// the transaction discards the call, no account it paid exists.
func TestCreatedAccountsGas(t *testing.T) {
	// the node's gas schedule: the scenario schedule prepare loads prices the VM API and the builtins
	// on a different scale
	gasMap, err := blockchainConfig.LoadGasScheduleConfig("../../../config/node/gasScheduleV1.yaml")
	require.NoError(t, err)

	for _, fixV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV7=%t", fixV7), func(t *testing.T) {
			buyMarginal, transferMarginal := measureCreatedAccounts(t, gasMap, fixV7)
			if fixV7 {
				require.GreaterOrEqual(t, buyMarginal, transferMarginal, "an account a Buy creates costs at least a transfer")
				return
			}
			require.Less(t, buyMarginal, transferMarginal, "before the fork an account a Buy creates costs less than a transfer")
		})
	}
}

// measureCreatedAccounts runs the rounds and the transfers with V7 as given, logs the gas per account
// and returns the gas one more created account adds to a Buy call and to a call of transfers.
func measureCreatedAccounts(t *testing.T, gasMap map[string]map[string]uint64, fixV7 bool) (uint64, uint64) {
	v7Epoch := uint32(0)
	if !fixV7 {
		v7Epoch = 1_000
	}
	forkController, err := fork.NewForkController(blockchainConfig.EnableEpochs{FixAuditChangesV7: v7Epoch}, &commonMock.EpochNotifierStub{})
	require.NoError(t, err)
	world, _, host, err := prepareWithForks(t, owner, gasMap, forkController)
	require.NoError(t, err)
	host.SetBuiltInFunctionsContainer(world.BuiltinFuncs.Container)

	sc := testcommon.MakeTestSCAddressWithVMType("createdAccounts", testcommon.DefaultVMType)
	deployCreatedAccounts(t, host, world, sc)
	seedCreatedAccountsITO(t, world, sc)

	perRound := core.MaxTransferRoyalties + 1
	round := 0
	// roundArgs returns the forward arguments of rounds rounds, each Buy given buyGas (0 for all the
	// call has left), and the addresses they pay
	roundArgs := func(rounds int, buyGas uint64) ([][]byte, [][]byte) {
		args := make([][]byte, 0)
		created := make([][]byte, 0)
		for range rounds {
			splits := freshAddresses(fmt.Sprintf("split-%d-", round), core.MaxTransferRoyalties)
			receiver := freshAddresses(fmt.Sprintf("royalty-%d-", round), 1)[0]
			round++

			royalties := encodeRoyalties(receiver, splits)
			decoded, errDecode := builtInFunctions.DecodeRoyaltiesData(royalties)
			require.NoError(t, errDecode)
			require.Equal(t, receiver, decoded.Address)
			require.Len(t, decoded.SplitRoyalties, core.MaxTransferRoyalties)

			args = append(args, builtinCall(core.BuiltInFunctionAssetTrigger, 0,
				enumArg(int64(transaction.AssetTriggerContract_UpdateRoyalties)), createdAccountsAsset, royalties)...)
			args = append(args, builtinCall(core.BuiltInFunctionBuy, buyGas,
				enumArg(int64(transaction.BuyContract_ITOBuy)), createdAccountsAsset, kdautils.KLVIdentifier,
				big.NewInt(1).Bytes(), big.NewInt(packPrice).Bytes())...)
			created = append(created, append(splits, receiver)...)
		}
		return args, created
	}
	// buyCall runs rounds UpdateRoyalties + Buy rounds in one call and returns its gas
	buyCall := func(rounds int) uint64 {
		args, created := roundArgs(rounds, 0)
		gas := callCreatedAccounts(t, host, world, sc, "forward", args...)
		require.Equal(t, rounds*perRound, countExisting(world, created))
		return gas
	}
	batch := 0
	// transferCall sends KLV to accounts fresh addresses in one call and returns its gas
	transferCall := func(accounts int) uint64 {
		fresh := freshAddresses(fmt.Sprintf("transfer-%d-", batch), accounts)
		batch++
		gas := callCreatedAccounts(t, host, world, sc, "sendKLV", append([][]byte{big.NewInt(1).Bytes()}, fresh...)...)
		require.Equal(t, accounts, countExisting(world, fresh))
		return gas
	}

	oneRound, manyRounds := buyCall(1), buyCall(repetitions)
	oneBatch, manyBatches := transferCall(perRound), transferCall(repetitions*perRound)

	extra := uint64((repetitions - 1) * perRound)
	buyMarginal := (manyRounds - oneRound) / extra
	transferMarginal := (manyBatches - oneBatch) / extra
	t.Logf("UpdateRoyalties + Buy: 1 round %d gas, %d rounds %d gas", oneRound, repetitions, manyRounds)
	t.Logf("KLV transfers: %d accounts %d gas, %d accounts %d gas", perRound, oneBatch, repetitions*perRound, manyBatches)
	t.Logf("per created account, call included (%d accounts): Buy %d gas, transfer %d gas",
		perRound, oneRound/uint64(perRound), oneBatch/uint64(perRound))
	t.Logf("per created account, one more in the same call: Buy %d gas, transfer %d gas (%.1fx)",
		buyMarginal, transferMarginal, float64(transferMarginal)/float64(buyMarginal))

	if fixV7 {
		// the Buy's cost from V7: the flat cost and a contract transfer per account it creates; an ITO
		// buy of a fungible asset stores no records
		contractTransfer := gasMap["BaseOpsAPICost"]["TransferValue"] + gasMap["BuiltInCost"]["Transfer"]
		buyCost := gasMap["BuiltInCost"]["Buy"] + uint64(perRound)*contractTransfer

		args, created := roundArgs(1, buyCost-1)
		vmOutput := runCreatedAccounts(t, host, sc, "forward", gasProvided, args...)
		require.NotEqual(t, vmcommon.Ok, vmOutput.ReturnCode)
		require.Contains(t, vmOutput.ReturnMessage, common.ErrNotEnoughGas.Error(), "the Buy itself fails for gas")
		// what txProcess does with a failed transaction: discard what the kapps wrote to the cacher
		require.NoError(t, world.RollbackChanges())
		require.Zero(t, countExisting(world, created), "a Buy that fails for gas leaves no account")

		args, created = roundArgs(1, buyCost)
		callCreatedAccounts(t, host, world, sc, "forward", args...)
		require.Equal(t, perRound, countExisting(world, created), "the exact cost is enough")
	}

	return buyMarginal, transferMarginal
}

func deployCreatedAccounts(t *testing.T, host vmhost.VMHost, world *worldmock.MockWorld, sc []byte) {
	t.Helper()
	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: owner,
		CreatorNonce:   1024,
		NewAddress:     sc,
	})
	vmOutput, err := host.RunSmartContractCreate(&vmcommon.ContractCreateInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  owner,
			CallType:    vm.DirectCall,
			GasProvided: gasProvided,
		},
		ContractCode: testcommon.GetTestSCCode("created-accounts", "../../"),
	})
	require.NoError(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode, vmOutput.ReturnMessage)
	require.NoError(t, world.UpdateAccounts(vmOutput.OutputAccounts, nil))
}

// seedCreatedAccountsITO gives the contract a fungible asset it owns, an active ITO selling one unit
// for packPrice KLV, and the KLV to buy it in every round.
func seedCreatedAccountsITO(t *testing.T, world *worldmock.MockWorld, sc []byte) {
	t.Helper()
	kc := world.KAppController
	kc.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: sc,
		Block:          &block.Block{Header: &block.BlockHeader{Timestamp: 1000}},
	}))

	contract, err := world.AccountsCacher.LoadUser(sc)
	require.NoError(t, err)
	require.NoError(t, contract.AddToBalance(100*packPrice, kdautils.KLVIdentifier, true))
	require.NoError(t, world.AccountsCacher.UpdateUser(contract))

	// the ITO receiver exists already, so each round creates only the split and royalty accounts
	itoReceiver := freshAddresses("ito-receiver-", 1)[0]
	receiverAccount, err := world.AccountsCacher.LoadUser(itoReceiver)
	require.NoError(t, err)
	require.NoError(t, world.AccountsCacher.UpdateUser(receiverAccount))

	asset := &kapps.KDAData{
		ID:           createdAccountsAsset,
		AssetType:    kapps.KDAData_Fungible,
		OwnerAddress: sc,
		MaxSupply:    1 << 40,
		Properties:   &kapps.PropertiesData{CanMint: true},
		Attributes:   &kapps.AttributesData{},
		Roles:        []*kapps.RolesData{{Address: kapps.ITOKAppAddress, HasRoleMint: true}},
		Royalties:    &kapps.RoyaltiesData{Address: sc},
	}
	kdaKApp, err := world.AccountsCacher.LoadKApp(kapps.KDAKAppAddress)
	require.NoError(t, err)
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKApp, asset.ID, asset))
	require.NoError(t, world.AccountsCacher.UpdateKapp(kdaKApp))

	ito := &kapps.ITOData{
		IsActive:        true,
		ReceiverAddress: itoReceiver,
		PackData: map[string]*kapps.PackData{
			string(kdautils.KLVIdentifier): {Packs: []*kapps.Pack{{Amount: 1, Price: packPrice}}},
		},
	}
	raw, err := worldmock.WorldMarshalizer.Marshal(ito)
	require.NoError(t, err)
	itoKApp, err := world.AccountsCacher.LoadKApp(kapps.ITOKAppAddress)
	require.NoError(t, err)
	require.NoError(t, itoKApp.DataTrieTracker().SaveKeyValue(kdautils.ToITOKey(asset.ID), raw))
	require.NoError(t, world.AccountsCacher.UpdateKapp(itoKApp))

	require.NoError(t, world.CommitChangesToDB())
}

// callCreatedAccounts runs function on the contract, keeps its state and returns the gas it used.
func callCreatedAccounts(t *testing.T, host vmhost.VMHost, world *worldmock.MockWorld, sc []byte, function string, args ...[]byte) uint64 {
	t.Helper()
	vmOutput := runCreatedAccounts(t, host, sc, function, gasProvided, args...)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode, "%s: %s", function, vmOutput.ReturnMessage)
	require.NoError(t, world.UpdateAccounts(vmOutput.OutputAccounts, nil))

	return gasProvided - vmOutput.GasRemaining
}

// runCreatedAccounts runs function on the contract with gas and returns its output, kept or not.
func runCreatedAccounts(t *testing.T, host vmhost.VMHost, sc []byte, function string, gas uint64, args ...[]byte) *vmcommon.VMOutput {
	t.Helper()
	vmOutput, err := host.RunSmartContractCall(&vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  owner,
			Arguments:   args,
			CallType:    vm.DirectCall,
			GasProvided: gas,
		},
		RecipientAddr: sc,
		Function:      function,
	})
	require.NoError(t, err)
	return vmOutput
}

// encodeRoyalties lays out royalties the way builtInFunctions.DecodeRoyaltiesData reads them: the
// receiver, no transfer royalties, no market royalties, each split with only an ITO percentage, then
// the ITO percentage and no ITO fixed amount.
func encodeRoyalties(receiver []byte, splits [][]byte) []byte {
	var buf bytes.Buffer
	u32 := func(v uint32) { _ = binary.Write(&buf, binary.BigEndian, v) }
	noBigUint := func() { u32(0) }

	buf.Write(receiver)
	u32(0)                   // transfer percentages
	noBigUint()              // transfer fixed
	u32(0)                   // market percentage
	noBigUint()              // market fixed
	u32(uint32(len(splits))) // #nosec G115 -- at most MaxTransferRoyalties
	for _, split := range splits {
		buf.Write(split)
		for _, percentage := range []uint32{0, 0, 0, 0, splitITOPercentage, 0} {
			u32(percentage)
		}
	}
	u32(itoPercentage)
	noBigUint() // ITO fixed

	return buf.Bytes()
}

// freshAddresses returns n user addresses that no test has created.
func freshAddresses(prefix string, n int) [][]byte {
	addresses := make([][]byte, n)
	for i := range addresses {
		addresses[i] = make([]byte, 32)
		copy(addresses[i], fmt.Sprintf("%s%02d", prefix, i))
	}

	return addresses
}

func countExisting(world *worldmock.MockWorld, addresses [][]byte) int {
	existing := 0
	for _, address := range addresses {
		if _, err := world.AccountsAdapter.GetExistingAccount(address); err == nil {
			existing++
		}
	}

	return existing
}

// builtinCall is one entry of the contract's forward arguments: the builtin, the gas it gets (0 for
// all the call has left), its argument count and its arguments.
func builtinCall(builtin string, gas uint64, args ...[]byte) [][]byte {
	return append([][]byte{[]byte(builtin), new(big.Int).SetUint64(gas).Bytes(), big.NewInt(int64(len(args))).Bytes()}, args...)
}

func enumArg(value int64) []byte {
	return big.NewInt(value).Bytes()
}
