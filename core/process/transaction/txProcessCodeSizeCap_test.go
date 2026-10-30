package transaction_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/core/process/smartContract"
	"github.com/klever-io/klever-go/data/transaction"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// An indirect deploy over the cap must fail the whole tx: the call value goes back to the sender.
func TestProcessTransaction_OverLimitIndirectDeployRevertsCallValue(t *testing.T) {
	t.Parallel()

	const callValue = int64(45)

	callerAddress := smartContractAddress("c")
	newContractAddress := smartContractAddress("n")

	args := createBaseKAppsProcessingArgs()
	initKLVAndKFIintoKapps(loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress), loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress))
	senderBalance := loadUserAccount(args.AccountsCacher, testOwnerAddress).GetBalance(kdautils.KLVIdentifier, false)

	callValueMoved := false
	txFeeHandler, err := postprocess.NewFeeAccumulator()
	require.Nil(t, err)

	args.ScProcessor, err = smartContract.NewSmartContractProcessor(smartContract.ArgsNewSmartContractProcessor{
		VmContainer: &contextmock.VMContainerMock{
			GetCalled: func(key []byte) (vmcommon.VMExecutionHandler, error) {
				return &contextmock.VMExecutionHandlerStub{
					RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
						sender := loadUserAccount(args.AccountsCacher, testOwnerAddress)
						callValueMoved = sender.GetBalance(kdautils.KLVIdentifier, false) == senderBalance-callValue
						return &vmcommon.VMOutput{
							ReturnCode:   vmcommon.Ok,
							GasRemaining: 400,
							OutputAccounts: map[string]*vmcommon.OutputAccount{
								string(newContractAddress): {
									Address:             newContractAddress,
									Code:                bytes.Repeat([]byte("d"), int(common.MaxLeafSize)+1),
									CodeMetadata:        []byte{1, 0},
									CodeDeployerAddress: input.RecipientAddr,
								},
							},
						}, nil
					},
				}, nil
			},
		},
		ArgsParser:  &contextmock.ArgumentParserMock{},
		Hasher:      args.Hasher,
		Marshalizer: args.Marshalizer,
		BlockChainHook: &contextmock.BlockchainHookStub{
			GetKAppControllerCalled: func() kapp.KAppController { return args.KAppController },
		},
		AccountsCacher:     args.AccountsCacher,
		BuiltInFunctions:   builtInFunctions.NewBuiltInFunctionContainer(),
		PubkeyConv:         args.PubkeyConv,
		TxFeeHandler:       txFeeHandler,
		TxLogsProcessor:    &contextmock.TxLogsProcessorStub{},
		EconomicsFee:       args.EconomicsFee,
		GasSchedule:        notifierMock.NewGasScheduleNotifierMock(gasScheduleForSC()),
		WasmVMChangeLocker: &sync.RWMutex{},
		VMOutputCacher:     txcache.NewDisabledCache(),
		ForkController:     args.ForkController,
		AllowDeploy:        true,
	})
	require.Nil(t, err)

	execTx := NewTXProcessor(t, args)

	caller, err := args.AccountsCacher.LoadUser(callerAddress)
	require.Nil(t, err)
	caller.SetCode([]byte("code"))
	caller.SetCodeMetadata([]byte{0, vmcommon.MetadataPayable})
	require.Nil(t, args.AccountsCacher.SaveUser(caller))
	require.Nil(t, args.AccountsCacher.SaveAll())

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		Address:   callerAddress,
		CallValue: map[string]*transaction.CallValue{string(kdautils.KLVIdentifier): {Amount: callValue}},
	}
	tx, err := createTransactionMock(&contract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	require.Nil(t, err)
	tx.RawData.Data = [][]byte{[]byte("abba@0500@0000")}
	tx.GasLimit = 1000

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	require.EqualError(t, err, process.ErrContractCodeTooLarge.Error())
	require.Equal(t, transaction.Transaction_VMExecutionFailed, tx.ResultCode)
	require.True(t, callValueMoved)

	sender, err := args.AccountsCacher.LoadUser(testOwnerAddress)
	require.Nil(t, err)
	require.Equal(t, senderBalance, sender.GetBalance(kdautils.KLVIdentifier, false))

	caller, err = args.AccountsCacher.LoadUser(callerAddress)
	require.Nil(t, err)
	require.Zero(t, caller.GetBalance(kdautils.KLVIdentifier, false))

	_, err = args.AccountsCacher.GetExistingUser(newContractAddress)
	require.Error(t, err)
}

func smartContractAddress(filler string) []byte {
	address := make([]byte, core.NumInitCharactersForScAddress-core.VMTypeLen)
	address = append(address, common.WasmVirtualMachine...)
	return append(address, bytes.Repeat([]byte(filler), 22)...)
}

func gasScheduleForSC() map[string]map[string]uint64 {
	return map[string]map[string]uint64{
		common.BaseOpsAPICost: {},
		common.BuiltInCost:    {core.BuiltInFunctionTransfer: 2000},
	}
}
