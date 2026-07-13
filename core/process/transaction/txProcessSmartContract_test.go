package transaction_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/vmcommon"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	nodeConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	smartContract "github.com/klever-io/klever-go/core/process/smartContract"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	"github.com/klever-io/klever-go/kapps"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/storage/txcache"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/klever-io/klever-go/core/process"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/core/process/transaction"
	proto "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	gproto "google.golang.org/protobuf/proto"

	"github.com/klever-io/klever-go/common/mock"
)

// Precompiled module that imports smallIntGetUnsignedArgument, exports memory/init/upgrade,
// and makes init loop for the number of iterations in argument 0. Embedding it keeps the
// timeout regression independent of an external WAT compiler.
const scDeployTimeoutBoundaryBusyInitWASMHex = "0061736d0100000001090260000060017f017e02230103656e761b736d616c6c496e74476574556e7369676e6564417267756d656e74000103030200000503010001071b0304696e6974000107757067726164650002066d656d6f727902000a1e021901027e4100100021010340200042017c22002001540d000b0b02000b"

const scDeployTimeoutMainnetExecutionTimeoutMs = uint32(500)

type scDeployTimeoutOutcome struct {
	err             error
	resultCode      dataTransaction.Transaction_TXResultCode
	duration        time.Duration
	contractAddress []byte
	codeLen         int
	deployed        bool
	timedOut        bool
}

func TestTXProcessor_validateSCTransaction(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		// setup
		Name      string
		AfterFork bool
		ExecData  []byte
		// assert
		ExpectedError      error
		ExpectedResultCode proto.Transaction_TXResultCode
	}{
		{
			Name:      "Should pass",
			AfterFork: true,
			ExecData:  []byte{1},
		},
		{
			Name:               "Should fail on no data",
			AfterFork:          true,
			ExpectedError:      process.ErrInvalidContractOrRawDataSize,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			Name:               "Should fail before fork",
			AfterFork:          false,
			ExpectedError:      process.ErrInvalidTransactionType,
			ExpectedResultCode: proto.Transaction_ContractNotFound,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			forkController := mock.ForkControllerStub{
				EnableSmartContractsValue: scenario.AfterFork,
			}
			txProc := transaction.NewTxProcessorExportTest()
			txProc.SetForkController(&forkController)

			kappContext := mock.KAppContextStub{
				GetExecDataCalled: func() []byte {
					return scenario.ExecData
				},
			}
			tx := &proto.Transaction{}
			err := txProc.ValidateSCTransaction(&kappContext, tx)
			assert.Equal(t, err, scenario.ExpectedError)
			assert.Equal(t, tx.ResultCode, scenario.ExpectedResultCode)
		})
	}
}

func TestTXProcessor_smartContract(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		// setup
		name                    string
		ExecData                []byte
		TxContractType          proto.TXContract_ContractType
		SmartContractActionType proto.SmartContract_SCType
		ActionResultCode        vmcommon.ReturnCode
		ActionError             error
		GetUserError            error
		// assert
		ExpectedError      error
		ExpectedResultCode proto.Transaction_TXResultCode
		MustCallInvokeSC   bool
		MustCallDeploySC   bool
	}{
		{
			name:               "Should fail on no data",
			ExpectedError:      process.ErrInvalidContractOrRawDataSize,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			name:               "Should fail on invalid contract type",
			ExecData:           []byte{1},
			TxContractType:     proto.TXContract_BuyContractType,
			ExpectedError:      common.ErrInvalidContract,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			name:                    "Should fail on invalid smart contract action type",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: 100,
			ExpectedError:           common.ErrSmartContractTypeInvalid,
			ExpectedResultCode:      proto.Transaction_ParameterInvalid,
		},
		{
			name:                    "Should call deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ExpectedError:           nil,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should call invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ExpectedError:           nil,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallInvokeSC:        true,
		},
		{
			name:                    "Should handle error on deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ActionError:             vmcommon.ErrInvalidVMType,
			ExpectedError:           vmcommon.ErrInvalidVMType,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should handle error on invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ActionError:             vmcommon.ErrInvalidVMType,
			ExpectedError:           vmcommon.ErrInvalidVMType,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallInvokeSC:        true,
		},
		{
			name:                    "Should handle not ok result code on deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ActionResultCode:        vmcommon.VMContractInvalid,
			ExpectedResultCode:      proto.Transaction_ContractInvalid,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should handle not ok result code on invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ActionResultCode:        vmcommon.VMContractInvalid,
			ExpectedResultCode:      proto.Transaction_ContractInvalid,
			MustCallInvokeSC:        true,
		},
		{
			name:               "Should fail if invalid address on invokeSC",
			ExecData:           []byte{1},
			TxContractType:     proto.TXContract_SmartContractType,
			GetUserError:       common.ErrAccountNotFound,
			ExpectedError:      common.ErrAccountNotFound,
			ExpectedResultCode: proto.Transaction_AccountError,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// scenario setup
			deploySCCalled := false
			invokeSCCalled := false

			// environment setup
			scProcessor := &mock.SmartContractProcessorStub{}
			forkController := &mock.ForkControllerStub{}
			accountsCacher := &mock.AccountsCacherStub{}
			forkController.EnableSmartContractsValue = true
			txProc := transaction.NewTxProcessorExportTest()
			txProc.SetForkController(forkController)
			txProc.SetSCProcessor(scProcessor)
			txProc.SetAccountsCacher(accountsCacher)

			// stub configuration
			kappContext := mock.KAppContextStub{
				GetExecDataCalled: func() []byte {
					return scenario.ExecData
				},
			}
			scProcessor.DeploySmartContractCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
				deploySCCalled = true
				return scenario.ActionResultCode, scenario.ActionError
			}
			scProcessor.ExecuteSmartContractTransactionCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
				invokeSCCalled = true
				return scenario.ActionResultCode, scenario.ActionError
			}
			accountsCacher.GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
				if scenario.GetUserError != nil {
					return nil, scenario.GetUserError
				}
				return &mock.UserAccountHandlerStub{}, nil
			}

			// setup TX
			userAccount := mock.UserAccountHandlerStub{}
			txContract := &proto.TXContract{
				Type:      scenario.TxContractType,
				Parameter: &anypb.Any{},
			}
			txContractData := &proto.SmartContract{
				Type: scenario.SmartContractActionType,
			}
			err := anypb.MarshalFrom(txContract.Parameter, txContractData, gproto.MarshalOptions{})
			assert.Nil(t, err)
			tx := &proto.Transaction{
				RawData: &proto.Transaction_Raw{
					Contract: []*proto.TXContract{txContract},
				},
			}

			// validate empty data
			err = txProc.SmartContract(&kappContext, &userAccount, tx)
			if scenario.ExpectedError != nil {
				assert.Equal(t, scenario.ExpectedError, err)
			}
			assert.Equal(t, scenario.ExpectedResultCode, tx.ResultCode)
			assert.Equal(t, scenario.MustCallDeploySC, deploySCCalled)
			assert.Equal(t, scenario.MustCallInvokeSC, invokeSCCalled)
		})
	}
}

func TestTXProcessor_SCDeployTimeoutBoundaryDeterministic(t *testing.T) {
	wasmCode := decodeSCDeployTimeoutBusyInitWASM(t)
	disabledOutcome := runSCDeployTimeoutDeployment(t, wasmCode, 1,
		scDeployTimeoutMainnetExecutionTimeoutMs, false)
	require.Error(t, disabledOutcome.err)
	require.Contains(t, disabledOutcome.err.Error(), "deployment is disabled")
	require.Equal(t, vmcommon.VMUserError.ResultCode(), disabledOutcome.resultCode)
	require.False(t, disabledOutcome.deployed)
	require.False(t, disabledOutcome.timedOut)

	loopCount := calibrateSCDeployTimeoutLoopCount(t, wasmCode)
	t.Logf("SC deploy timeout regression calibrated init loop count: %d", loopCount)

	timeoutLoopCount := uint64(float64(loopCount) * 1.10)
	const attempts = 12
	outcomes := make([]scDeployTimeoutOutcome, 0, attempts)
	for attempt := 0; attempt < attempts; attempt++ {
		outcome := runSCDeployTimeoutDeployment(t, wasmCode, timeoutLoopCount,
			scDeployTimeoutMainnetExecutionTimeoutMs, true)
		outcomes = append(outcomes, outcome)
	}

	t.Logf("SC deploy deterministic timeout batch loopCount=%d timeouts=%d outcomes=%s",
		timeoutLoopCount, countSCDeployTimeouts(outcomes),
		formatSCDeployTimeoutDurations(outcomes))

	for _, outcome := range outcomes {
		require.Truef(t, outcome.timedOut,
			"expected deterministic timeout outcome, got err=%v resultCode=%s duration=%s codeLen=%d",
			outcome.err, outcome.resultCode, outcome.duration, outcome.codeLen)
		require.False(t, outcome.deployed)
		require.Error(t, outcome.err)
		require.Equal(t, vmcommon.VMUserError.ResultCode(), outcome.resultCode)
		require.True(t, strings.Contains(outcome.err.Error(), "timeout") || errors.Is(outcome.err, vmhost.ErrExecutionFailedWithTimeout))
		require.Zero(t, outcome.codeLen)
	}
}

func decodeSCDeployTimeoutBusyInitWASM(t *testing.T) []byte {
	t.Helper()

	wasmCode, err := hex.DecodeString(scDeployTimeoutBoundaryBusyInitWASMHex)
	require.NoError(t, err)
	require.NotEmpty(t, wasmCode)
	return wasmCode
}

func calibrateSCDeployTimeoutLoopCount(t *testing.T, wasmCode []byte) uint64 {
	t.Helper()
	const (
		calibrationTimeoutMs = uint32(5_000)
		targetDuration       = time.Duration(scDeployTimeoutMainnetExecutionTimeoutMs) *
			time.Millisecond
	)
	warmup := runSCDeployTimeoutDeployment(t, wasmCode, 500_000,
		calibrationTimeoutMs, true)
	require.Truef(t, warmup.deployed, "warm-up deployment failed: err=%v resultCode=%s",
		warmup.err, warmup.resultCode)
	loopCount := uint64(500_000)
	var lower uint64
	var upper uint64
	for step := 0; step < 20; step++ {
		outcome := runSCDeployTimeoutDeployment(t, wasmCode, loopCount,
			calibrationTimeoutMs, true)
		require.Truef(t, outcome.deployed, "calibration deployment failed: err=%v resultCode=%s",
			outcome.err, outcome.resultCode)
		if outcome.duration >= targetDuration {
			upper = loopCount
			break
		}
		lower = loopCount
		scale := float64(targetDuration) /
			float64(maxSCDeployTimeoutDuration(outcome.duration, time.Millisecond))
		if scale < 1.2 {
			scale = 1.2
		}
		if scale > 8 {
			scale = 8
		}
		loopCount = uint64(float64(loopCount) * scale)
	}
	if upper == 0 {
		t.Fatalf("could not calibrate SC deploy busy init loop to the timeout boundary")
	}
	if lower == 0 {
		lower = upper / 2
	}
	best := upper
	bestDistance := time.Duration(1<<63 - 1)
	for step := 0; step < 10 && lower+1 < upper; step++ {
		mid := lower + (upper-lower)/2
		outcome := runSCDeployTimeoutDeployment(t, wasmCode, mid,
			calibrationTimeoutMs, true)
		require.Truef(t, outcome.deployed, "calibration deployment failed: err=%v resultCode=%s",
			outcome.err, outcome.resultCode)
		distance := absSCDeployTimeoutDuration(outcome.duration - targetDuration)
		if distance < bestDistance {
			best = mid
			bestDistance = distance
		}
		if outcome.duration < targetDuration {
			lower = mid
		} else {
			upper = mid
		}
	}
	return best
}

func runSCDeployTimeoutDeployment(
	t *testing.T,
	wasmCode []byte,
	loopCount uint64,
	timeoutMs uint32,
	isGenesisProcessing bool,
) scDeployTimeoutOutcome {
	t.Helper()
	world := worldmock.NewMockWorld()
	gasSchedule := kvmConfig.MakeGasMapForTests()
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(nodeConfig.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
		SmartContracts:        0,
	}, epochNotifier)
	require.NoError(t, err)
	forkController.EpochConfirmed(0)
	require.NoError(t, world.InitBuiltinFunctions(gasSchedule, forkController))
	kdaTransferParser, err := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)
	require.NoError(t, err)
	hostParams := &vmhost.VMHostParameters{
		VMType:                              common.WasmVirtualMachine,
		KDATransferParser:                   kdaTransferParser,
		BuiltInFuncContainer:                world.BuiltinFuncs.Container,
		EpochNotifier:                       epochNotifier,
		Hasher:                              worldmock.DefaultHasher,
		ForkController:                      forkController,
		GasSchedule:                         gasSchedule,
		ProtectedKeyPrefix:                  scDeployTimeoutProtectedKeyPrefixes(),
		TimeOutForSCExecutionInMilliseconds: timeoutMs,
		TimeOutTolerancePercentage:          15,
		ExecutionMode:                       vmcommon.ExecutionModeLeader,
	}
	host, err := hostCore.NewVMHost(world, hostParams)
	require.NoError(t, err)
	defer host.Reset()
	args := createArgsForTxProcessorWithAccounts(
		world.AccountsAdapter,
		world.PeersAdapter,
		world.KAppsAdapter,
		world.AccountsCacher,
	)
	args.KAppController = world.KAppController
	args.ForkController = forkController
	feeHandler := freeFeeHandlerMock()
	feeHandler.ComputeGasCalled = func(tx *dataTransaction.Transaction, _ *dataTransaction.CostResponse) (uint64, uint64, error) {
		return tx.GetGasLimit(), tx.GetGasLimit(), nil
	}
	args.EconomicsFee = feeHandler
	scProcessor, err :=
		smartContract.NewSmartContractProcessor(smartContract.ArgsNewSmartContractProcessor{
			VmContainer: &contextmock.VMContainerMock{
				GetCalled: func(key []byte) (vmcommon.VMExecutionHandler, error) {
					require.Equal(t, common.WasmVirtualMachine, key)
					return host, nil
				},
			},
			ArgsParser:       smartContract.NewArgumentParser(),
			Hasher:           args.Hasher,
			Marshalizer:      args.Marshalizer,
			BlockChainHook:   scDeployTimeoutBlockchainHook(world),
			BuiltInFunctions: world.BuiltinFuncs.Container, PubkeyConv: args.PubkeyConv,
			TxFeeHandler:        args.TxFeeHandler,
			EconomicsFee:        feeHandler,
			GasSchedule:         notifierMock.NewGasScheduleNotifierMock(gasSchedule),
			TxLogsProcessor:     &contextmock.TxLogsProcessorStub{},
			ForkController:      forkController,
			VMOutputCacher:      txcache.NewDisabledCache(),
			WasmVMChangeLocker:  &sync.RWMutex{},
			AccountsCacher:      world.AccountsCacher,
			IsGenesisProcessing: isGenesisProcessing,
		})
	require.NoError(t, err)
	args.ScProcessor = scProcessor
	AddBalanceAccount(world.AccountsCacher, 1_000_000_000, nil, testOwnerAddress)
	require.NoError(t, world.AccountsCacher.SaveAll())
	execTx := NewTXProcessor(t, args)
	scContract := dataTransaction.SmartContract{
		Type: dataTransaction.SmartContract_SCDeploy,
	}
	tx, err := createTransactionMock(&scContract,
		dataTransaction.TXContract_SmartContractType, testOwnerAddress, 0)
	require.NoError(t, err)
	tx.GasLimit = 10_000_000_000_000
	tx.RawData.Data = [][]byte{[]byte(encodeSCDeployTimeoutDeployData(wasmCode,
		loopCount))}
	_, txHash, err := execTx.PreProcessTransaction(tx)
	require.NoError(t, err)
	start := time.Now()
	err = execTx.ProcessTransaction(createBlockHeader(), txHash, tx)
	duration := time.Since(start)
	outcome := scDeployTimeoutOutcome{
		err:             err,
		resultCode:      tx.ResultCode,
		duration:        duration,
		contractAddress: append([]byte{}, world.LastCreatedContractAddress...),
	}
	if len(world.LastCreatedContractAddress) > 0 {
		contractAcc, loadErr :=
			world.AccountsCacher.GetExistingUser(world.LastCreatedContractAddress)
		if loadErr == nil {
			outcome.codeLen =
				len(world.AccountsCacher.GetCode(contractAcc.GetCodeHash()))
		}
	}
	outcome.deployed = err == nil &&
		tx.ResultCode == dataTransaction.Transaction_Ok &&
		len(outcome.contractAddress) > 0 &&
		outcome.codeLen > 0
	outcome.timedOut = err != nil &&
		tx.ResultCode == vmcommon.VMUserError.ResultCode() &&
		(strings.Contains(err.Error(), "timeout") || errors.Is(err,
			vmhost.ErrExecutionFailedWithTimeout))
	return outcome
}

func encodeSCDeployTimeoutDeployData(wasmCode []byte, loopCount uint64) string {
	return strings.Join([]string{
		hex.EncodeToString(wasmCode),
		hex.EncodeToString(common.WasmVirtualMachine),
		hex.EncodeToString((&vmcommon.CodeMetadata{Upgradeable: true, Payable: true}).ToBytes()),
		hex.EncodeToString(encodeSCDeployTimeoutUint64(loopCount)),
	}, "@")
}

func encodeSCDeployTimeoutUint64(value uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, value)
	return buf
}

func scDeployTimeoutBlockchainHook(world *worldmock.MockWorld) *contextmock.BlockchainHookStub {
	return &contextmock.BlockchainHookStub{
		GetUserAccountCalled: world.GetUserAccount,
		GetCodeCalled: func(account state.UserAccountHandler) []byte {
			return world.AccountsCacher.GetCode(account.GetCodeHash())
		},
		IsSmartContractCalled: world.IsSmartContract,
		IsPayableCalled: func(_ []byte) (bool, error) {
			return true, nil
		},
		ProcessBuiltInFunctionCalled:            world.ProcessBuiltInFunction,
		GetBuiltinFunctionNamesCalled:           world.GetBuiltinFunctionNames,
		GetSnapshotCalled:                       world.GetSnapshot,
		RevertToSnapshotCalled:                  world.RevertToSnapshot,
		ExecuteSmartContractCallOnOtherVMCalled: world.ExecuteSmartContractCallOnOtherVM,
		FilterCodeMetadataForUpgradeCalled:      func(input []byte) ([]byte, error) { return input, nil },
		GetBuiltinFunctionsContainerCalled: func() vmcommon.BuiltInFunctionContainer {
			return world.BuiltinFuncs.Container
		},
		ResetCountersCalled:    func() {},
		GetCounterValuesCalled: func() map[string]uint64 { return map[string]uint64{} },
		GetKAppControllerCalled: func() kapp.KAppController {
			return world.KAppController
		},
	}
}

func scDeployTimeoutProtectedKeyPrefixes() [][]byte {
	return [][]byte{
		[]byte(kapps.ProtectedKleverKeyPrefix),
		[]byte(kapps.ProtectedKLVKeyPrefix),
		[]byte(kapps.ProtectedKFIKeyPrefix),
		[]byte(kapps.KDAPrefix),
	}
}

func maxSCDeployTimeoutDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func absSCDeployTimeoutDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return -duration
	}
	return duration
}

func formatSCDeployTimeoutDurations(outcomes []scDeployTimeoutOutcome) string {
	var buffer bytes.Buffer
	for i, outcome := range outcomes {
		if i > 0 {
			buffer.WriteString(",")
		}
		marker := "ok"
		if outcome.timedOut {
			marker = "timeout"
		}
		buffer.WriteString(fmt.Sprintf("%s:%s", marker,
			outcome.duration.Round(time.Millisecond)))
	}
	return buffer.String()
}

func countSCDeployTimeouts(outcomes []scDeployTimeoutOutcome) int {
	timeouts := 0
	for _, outcome := range outcomes {
		if outcome.timedOut {
			timeouts++
		}
	}

	return timeouts
}
