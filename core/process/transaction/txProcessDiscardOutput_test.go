package transaction_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/smartContract"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/data/transaction"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// slower than the tolerance band's lower bound, so a live validator accepts the leader's timeout
var discardOutputValidatorDelay = core.MinSCExecutionTimeout*(100-core.DefaultTolerancePercentage)/100 + 20*time.Millisecond

type discardOutputCase struct {
	deploy bool
	fixV7  bool
	mode   vmcommon.ExecutionMode
}

type discardOutputResult struct {
	err       error
	tx        *transaction.Transaction
	txHash    []byte
	txLogs    process.TransactionLogProcessor
	vmOutputs storage.Cacher
}

// runs one SC tx through the real txProcessor and scProcessor against a block recording `recorded`
func runDiscardOutputTx(t *testing.T, tc discardOutputCase, recorded transaction.Transaction_TXResultCode) discardOutputResult {
	contractAddress := smartContractAddress("c")
	deployedAddress := smartContractAddress("n")

	args := createBaseKAppsProcessingArgs()
	initKLVAndKFIintoKapps(loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress), loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress))

	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV7Value = tc.fixV7
	args.ForkController = forkController

	sleepIfValidator := func() {
		if tc.mode == vmcommon.ExecutionModeValidator {
			time.Sleep(discardOutputValidatorDelay)
		}
	}
	vmExecutor := &contextmock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			sleepIfValidator()
			return &vmcommon.VMOutput{
				ReturnCode:   vmcommon.Ok,
				GasRemaining: 400,
				Logs:         []*vmcommon.LogEntry{{Identifier: []byte("callEvent"), Address: contractAddress}},
			}, nil
		},
		RunSmartContractCreateCalled: func(_ *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
			sleepIfValidator()
			return &vmcommon.VMOutput{
				ReturnCode:   vmcommon.Ok,
				GasRemaining: 400,
				OutputAccounts: map[string]*vmcommon.OutputAccount{
					string(deployedAddress): {
						Address:             deployedAddress,
						Code:                []byte("deployed code"),
						CodeMetadata:        []byte{1, 0},
						CodeDeployerAddress: testOwnerAddress,
					},
				},
			}, nil
		},
		GetExecutionModeCalled: func() vmcommon.ExecutionMode { return tc.mode },
	}

	txLogs, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{Marshalizer: marshal.NewProtoMarshalizer()})
	require.Nil(t, err)
	vmOutputs, err := lrucache.NewCache(10)
	require.Nil(t, err)
	txFeeHandler, err := postprocess.NewFeeAccumulator()
	require.Nil(t, err)

	args.ScProcessor, err = smartContract.NewSmartContractProcessor(smartContract.ArgsNewSmartContractProcessor{
		VmContainer: &contextmock.VMContainerMock{
			GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) { return vmExecutor, nil },
		},
		ArgsParser:  smartContract.NewArgumentParser(),
		Hasher:      args.Hasher,
		Marshalizer: args.Marshalizer,
		BlockChainHook: &contextmock.BlockchainHookStub{
			GetKAppControllerCalled:            func() kapp.KAppController { return args.KAppController },
			FilterCodeMetadataForUpgradeCalled: func(input []byte) ([]byte, error) { return input, nil },
		},
		AccountsCacher:     args.AccountsCacher,
		BuiltInFunctions:   builtInFunctions.NewBuiltInFunctionContainer(),
		PubkeyConv:         args.PubkeyConv,
		TxFeeHandler:       txFeeHandler,
		TxLogsProcessor:    txLogs,
		EconomicsFee:       args.EconomicsFee,
		GasSchedule:        notifierMock.NewGasScheduleNotifierMock(gasScheduleForSC()),
		WasmVMChangeLocker: &sync.RWMutex{},
		VMOutputCacher:     vmOutputs,
		ForkController:     forkController,
		AllowDeploy:        true,
	})
	require.Nil(t, err)

	execTx := NewTXProcessor(t, args)

	contract := transaction.SmartContract{Type: transaction.SmartContract_SCDeploy}
	if !tc.deploy {
		callee, errLoad := args.AccountsCacher.LoadUser(contractAddress)
		require.Nil(t, errLoad)
		callee.SetCode([]byte("code"))
		callee.SetCodeMetadata([]byte{0, vmcommon.MetadataPayable})
		require.Nil(t, args.AccountsCacher.SaveUser(callee))
		require.Nil(t, args.AccountsCacher.SaveAll())

		contract = transaction.SmartContract{Type: transaction.SmartContract_SCInvoke, Address: contractAddress}
	}

	tx, err := createTransactionMock(&contract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	require.Nil(t, err)
	tx.RawData.Data = [][]byte{[]byte("abba@0500@0000")}
	tx.GasLimit = 1000

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	blk := createBlockHeader()
	blk.TxHashes = [][]byte{hash}
	blk.TxResults = []uint32{uint32(recorded)}

	err = execTx.ProcessTransaction(blk, hash, tx)

	return discardOutputResult{err: err, tx: tx, txHash: hash, txLogs: txLogs, vmOutputs: vmOutputs}
}

func (r discardOutputResult) logsOfTx() []*transaction.Log {
	txLogs := make([]*transaction.Log, 0)
	for _, logData := range r.txLogs.GetAllCurrentLogs() {
		if logData.TxHash == string(r.txHash) {
			txLogs = append(txLogs, logData.LogHandler.(*transaction.Log))
		}
	}

	return txLogs
}

func (r discardOutputResult) hasEvent(identifier string) bool {
	for _, txLog := range r.logsOfTx() {
		for _, event := range txLog.Events {
			if string(event.Identifier) == identifier {
				return true
			}
		}
	}

	return false
}

func (r discardOutputResult) requireOutputKept(t *testing.T, deploy bool) {
	require.NotEmpty(t, r.logsOfTx())
	require.Equal(t, deploy, r.hasEvent(core.SCDeployIdentifier))
	require.Equal(t, !deploy, r.hasEvent("callEvent"))
	_, cached := r.vmOutputs.Get(r.txHash)
	require.True(t, cached)
}

func (r discardOutputResult) requireOutputDiscarded(t *testing.T) {
	require.Empty(t, r.logsOfTx())
	require.False(t, r.hasEvent(core.SCDeployIdentifier))
	_, cached := r.vmOutputs.Get(r.txHash)
	require.False(t, cached)
}

// KLC-2891: a tx accepted as the leader's timeout keeps none of its local run's logs or cached output.
func TestProcessTransaction_AcceptLeaderDiscardsSCOutput(t *testing.T) {
	t.Parallel()

	modes := map[string]vmcommon.ExecutionMode{"replay": vmcommon.ExecutionModeReplay, "validator": vmcommon.ExecutionModeValidator}
	for _, deploy := range []bool{false, true} {
		for _, fixV7 := range []bool{true, false} {
			for modeName, mode := range modes {
				tc := discardOutputCase{deploy: deploy, fixV7: fixV7, mode: mode}
				name := fmt.Sprintf("deploy=%v/V7=%v/%s", deploy, fixV7, modeName)

				t.Run(name+"/leader timeout", func(t *testing.T) {
					t.Parallel()

					res := runDiscardOutputTx(t, tc, transaction.Transaction_VMExecutionFailed)

					if deploy && !fixV7 {
						// pre-V7 a deploy records no execution time, so the result check is skipped
						require.Nil(t, res.err)
						require.Equal(t, transaction.Transaction_Ok, res.tx.ResultCode)
						res.requireOutputKept(t, deploy)
						return
					}

					require.True(t, errors.Is(res.err, process.ErrTransactionResultMismatchAcceptLeader), "got: %v", res.err)
					require.Equal(t, transaction.Transaction_VMExecutionFailed, res.tx.ResultCode)
					res.requireOutputDiscarded(t)
				})

				t.Run(name+"/leader success", func(t *testing.T) {
					t.Parallel()

					res := runDiscardOutputTx(t, tc, transaction.Transaction_Ok)

					require.Nil(t, res.err)
					require.Equal(t, transaction.Transaction_Ok, res.tx.ResultCode)
					res.requireOutputKept(t, deploy)
				})

				// any other result error leaves the output alone
				t.Run(name+"/leader user error", func(t *testing.T) {
					t.Parallel()

					res := runDiscardOutputTx(t, tc, transaction.Transaction_VMUserError)

					if deploy && !fixV7 {
						require.Nil(t, res.err)
					} else {
						require.True(t, errors.Is(res.err, process.ErrTransactionResultMismatch), "got: %v", res.err)
					}
					res.requireOutputKept(t, deploy)
				})
			}
		}
	}
}
