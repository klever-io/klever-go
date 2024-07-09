package hostCoretest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	executorwrapper "github.com/klever-io/klever-go/kvm/executor/wrapper"
	vmMath "github.com/klever-io/klever-go/kvm/math"
	twoscomplement "github.com/klever-io/klever-go/kvm/math/twos-complement"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/testcommon/testexecutor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/kvm/wasmer2"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var counterKey = []byte("COUNTER")
var WASMLocalsLimit = uint64(4000)
var maxUint8AsInt = math.MaxUint8
var newAddress = test.MakeTestSCAddress("new smartcontract")
var mBufferKey = []byte("mBuffer")
var managedBuffer = []byte{0xff, 0x2a, 0x26, 0x5f, 0x8b, 0xcb, 0xdc, 0xaf,
	0xd5, 0x85, 0x19, 0x14, 0x1e, 0x57, 0x81, 0x24,
	0xcb, 0x40, 0xd6, 0x4a, 0x50, 0x1f, 0xba, 0x9c,
	0x11, 0x84, 0x7b, 0x28, 0x96, 0x5b, 0xc7, 0x37,
	0xd5, 0x85, 0x19, 0x14, 0x1e, 0x57, 0x81, 0x24,
	0xcb, 0x40, 0xd6, 0x4a, 0x50, 0x1f, 0xba, 0x9c,
	0x11, 0x84, 0x7b, 0x28, 0x96, 0x5b, 0xc7, 0x37,
	0xd5, 0x85, 0x19, 0x14, 0x1e, 0x57, 0x81, 0x24}

var UniqueCodeHash = []byte{1}

const (
	get                     = "get"
	increment               = "increment"
	callRecursive           = "callRecursive"
	parentCallsChild        = "parentCallsChild"
	parentPerformAsyncCall  = "parentPerformAsyncCall"
	parentFunctionChildCall = "parentFunctionChildCall"
)

func init() {
	test.ParentCompilationCostSameCtx = uint64(len(test.GetTestSCCode("exec-same-ctx-parent", "../../", "../../../")))
	test.ChildCompilationCostSameCtx = uint64(len(test.GetTestSCCode("exec-same-ctx-child", "../../", "../../../")))

	test.ParentCompilationCostDestCtx = uint64(len(test.GetTestSCCode("exec-dest-ctx-parent", "../../", "../../../")))
	test.ChildCompilationCostDestCtx = uint64(len(test.GetTestSCCode("exec-dest-ctx-child", "../../", "../../../")))
}

func TestSCMem(t *testing.T) {
	testString := "this is some random string of bytes"
	returnData := [][]byte{
		[]byte(testString),
		{35},
	}
	for _, c := range testString {
		returnData = append(returnData, []byte{byte(c)})
	}

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("misc", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(100000).
			WithFunction("iterate_over_byte_array").
			Build()).
		AndAssertResults(func(host vmhost.VMHost, blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(returnData...)
		})
}

func TestExecution_DeployNewAddressErr(t *testing.T) {
	errNewAddress := errors.New("new address error")

	input := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1000).
		WithContractCode([]byte("contract")).
		Build()

	test.BuildInstanceCreatorTest(t).
		WithInput(input).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
				require.Equal(t, input.CallerAddr, address)
				return &worldmock.Account{}, nil
			}
			stubBlockchainHook.NewAddressCalled = func(creatorAddress []byte, nonce uint64, vmType []byte) ([]byte, error) {
				require.Equal(t, input.CallerAddr, creatorAddress)
				require.Equal(t, uint64(0), nonce)
				require.Equal(t, test.DefaultVMType, vmType)
				return nil, errNewAddress
			}
		}).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				ReturnMessage(errNewAddress.Error())
		})
}

func TestExecution_DeployOutOfGas(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(8).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.OutOfGas().
				ReturnMessage(vmhost.ErrNotEnoughGas.Error())
		})
}

func TestExecution_DeployNotWASM(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(9).
			WithContractCode([]byte("not WASM")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_DeployWASM_WrongInit_Wasmer2(t *testing.T) {
	testExecutionDeployWASMWrongInit(t, wasmer2.ExecutorFactory())
}

func testExecutionDeployWASMWrongInit(t *testing.T, executorFactory executor.ExecutorAbstractFactory) {
	test.BuildInstanceCreatorTest(t).
		WithExecutorFactory(executorFactory).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithContractCode(test.GetTestSCCode("init-wrong", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_DeployWASM_WrongMethods(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithContractCode(test.GetTestSCCode("signatures", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_DeployWASM_Successful(t *testing.T) {
	input := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1000).
		WithContractCode(test.GetTestSCCode("init-correct", "../../")).
		WithKDATransfers([]*vmcommon.KDATransfer{
			{
				KDAValue: big.NewInt(88),
			},
		}).
		WithArguments([]byte{0}).
		Build()
	test.BuildInstanceCreatorTest(t).
		WithInput(input).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte("init successful")).
				GasRemaining(226).
				Code(newAddress, input.ContractCode)
			// BalanceDelta(newAddress, 88)
		})
}

func TestExecution_DeployWASM_Popcnt(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithArguments().
			WithContractCode(test.GetTestSCCode("init-simple-popcnt", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte{3})
		})
}

func TestExecution_DeployWASM_AtMaximumLocals(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithContractCode(makeBytecodeWithLocals(WASMLocalsLimit)).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()
		})
}

func TestExecution_DeployWASM_MoreThanMaximumLocals(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithContractCode(makeBytecodeWithLocals(WASMLocalsLimit + 1)).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_DeployWASM_Init_Errors(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(2000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithArguments([]byte{1}).
			WithContractCode(test.GetTestSCCode("init-correct", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.UserError()
		})
}

func TestExecution_DeployWASM_Init_InfiniteLoop_Errors(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithArguments([]byte{2}).
			WithContractCode(test.GetTestSCCode("init-correct", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.OutOfGas()
		})
}

func TestExecution_ManyDeployments(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	ownerNonce := uint64(23)
	numDeployments := 1000

	tester := test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(100000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithCallerAddr([]byte("owner")).
			WithContractCode(test.GetTestSCCode("init-simple", "../../")).
			Build()).
		WithAddress(newAddress).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
				return &worldmock.Account{Nonce: ownerNonce}, nil
			}
			stubBlockchainHook.NewAddressCalled = func(creatorAddress []byte, nonce uint64, vmType []byte) ([]byte, error) {
				ownerNonce++
				return []byte(string(newAddress) + " " + fmt.Sprint(ownerNonce)), nil
			}
		})

	for i := 0; i < numDeployments; i++ {
		tester.AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()
		})
	}
}

func TestExecution_MultipleInstances_SameVMHooks(t *testing.T) {
	code := test.GetTestSCCode("counter", "../../")

	input := test.DefaultTestContractCallInput()
	input.GasProvided = 1000000
	input.Function = get

	defaultFactory := testexecutor.NewDefaultTestExecutorFactory(t)
	executorFactory := executorwrapper.SimpleWrappedExecutorFactory(defaultFactory)
	host1 := test.NewTestHostBuilder(t).
		WithExecutorFactory(executorFactory).
		WithBlockchainHook(test.BlockchainHookStubForCall(code, nil)).
		Build()
	defer func() {
		host1.Reset()
	}()
	_, _, _, _, runtimeContext1, _ := host1.GetContexts()
	runtimeContextMock := contextmock.NewRuntimeContextWrapper(&runtimeContext1)
	host1.SetRuntimeContext(runtimeContextMock)

	for i := 0; i < 5; i++ {
		vmOutput, err := host1.RunSmartContractCall(input)
		verify := test.NewVMOutputVerifier(t, vmOutput, err)
		verify.Ok()
	}

	var vmHooksPtr = make(map[uintptr]bool)
	for _, instance := range executorFactory.LastCreatedExecutor.GetContractInstances(code) {
		vmHooksPtr[instance.GetVMHooksPtr()] = true
	}
	require.False(t, len(vmHooksPtr) > 1)
}

func TestExecution_MultipleVMs_OverlappingDifferentVMHooks(t *testing.T) {
	t.Skip()
	code := test.GetTestSCCode("counter", "../../")

	input := test.DefaultTestContractCallInput()
	input.GasProvided = 1000000
	input.Function = get

	executorFactory1 := executorwrapper.SimpleWrappedExecutorFactory(wasmer2.ExecutorFactory())
	host1 := test.NewTestHostBuilder(t).
		WithExecutorFactory(executorFactory1).
		WithBlockchainHook(test.BlockchainHookStubForCall(code, nil)).
		Build()
	defer func() {
		host1.Reset()
	}()
	_, _, _, _, runtimeContext1, _ := host1.GetContexts()
	runtimeContextMock := contextmock.NewRuntimeContextWrapper(&runtimeContext1)
	host1.SetRuntimeContext(runtimeContextMock)

	executorFactory2 := executorwrapper.SimpleWrappedExecutorFactory(wasmer2.ExecutorFactory())
	host2 := test.NewTestHostBuilder(t).
		WithExecutorFactory(executorFactory2).
		WithBlockchainHook(test.BlockchainHookStubForCall(code, nil)).
		Build()
	defer func() {
		host2.Reset()
	}()
	_, _, _, _, runtimeContext2, _ := host2.GetContexts()
	runtimeContextMock = contextmock.NewRuntimeContextWrapper(&runtimeContext2)
	host2.SetRuntimeContext(runtimeContextMock)

	runNContractsForHostAndVerify(t, host2, input, 5)
	runNContractsForHostAndVerify(t, host1, input, 5)
	runNContractsForHostAndVerify(t, host2, input, maxUint8AsInt+1)

	var host1VMHooksPtr = make(map[uintptr]bool)
	for _, instance := range executorFactory1.LastCreatedExecutor.GetContractInstances(code) {
		host1VMHooksPtr[instance.GetVMHooksPtr()] = true
	}
	for _, instance := range executorFactory2.LastCreatedExecutor.GetContractInstances(code) {
		_, found := host1VMHooksPtr[instance.GetVMHooksPtr()]
		require.False(t, found)
	}
}

func TestExecution_MultipleVMs_CleanInstanceWhileOthersAreRunning(t *testing.T) {
	code := test.GetTestSCCode("counter", "../../")

	input := test.DefaultTestContractCallInput()
	input.GasProvided = 1000000
	input.Function = get

	interHostsChan := make(chan string)
	host1Chan := make(chan string)

	host1 := test.NewTestHostBuilder(t).
		WithBlockchainHook(test.BlockchainHookStubForCall(code, nil)).
		Build()
	defer func() {
		host1.Reset()
	}()
	_, _, _, _, runtimeContext1, _ := host1.GetContexts()
	runtimeContextMock := contextmock.NewRuntimeContextWrapper(&runtimeContext1)
	runtimeContextMock.FunctionNameCheckedFunc = func() (string, error) {
		interHostsChan <- "waitForHost2"
		return runtimeContextMock.GetWrappedRuntimeContext().FunctionNameChecked()
	}
	host1.SetRuntimeContext(runtimeContextMock)

	host2 := test.NewTestHostBuilder(t).
		WithBlockchainHook(test.BlockchainHookStubForCall(code, nil)).
		Build()
	defer func() {
		host2.Reset()
	}()
	_, _, _, _, runtimeContext2, _ := host2.GetContexts()
	runtimeContextMock = contextmock.NewRuntimeContextWrapper(&runtimeContext2)
	runtimeContextMock.FunctionNameCheckedFunc = func() (string, error) {
		// wait to make sure host1 is running also
		<-interHostsChan
		// wait for host1 to finish
		<-interHostsChan
		return runtimeContextMock.GetWrappedRuntimeContext().FunctionNameChecked()
	}
	host2.SetRuntimeContext(runtimeContextMock)

	var vmOutput1 *vmcommon.VMOutput
	var err1 error
	go func() {
		vmOutput1, err1 = host1.RunSmartContractCall(input)
		interHostsChan <- "finish"
		host1Chan <- "finish"
	}()

	vmOutput2, err2 := host2.RunSmartContractCall(input)

	<-host1Chan

	verify1 := test.NewVMOutputVerifier(t, vmOutput1, err1)
	verify1.Ok()

	verify2 := test.NewVMOutputVerifier(t, vmOutput2, err2)
	verify2.Ok()
}

func TestExecution_Deploy_DisallowFloatingPoint(t *testing.T) {
	test.BuildInstanceCreatorTest(t).
		WithInput(test.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(88),
				},
			}).
			WithArguments([]byte{2}).
			WithContractCode(test.GetTestSCCode("num-with-fp", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_CallGetUserAccountErr(t *testing.T) {
	errGetAccount := errors.New("get code error")
	test.BuildInstanceCallTest(t).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100).
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
				return nil, errGetAccount
			}
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractNotFound().
				ReturnMessage(vmhost.ErrContractNotFound.Error())
		})
}

func TestExecution_NotEnoughGasForGetCode(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(0).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.OutOfGas().
				ReturnMessage(vmhost.ErrNotEnoughGas.Error())
		})
}

func TestExecution_CallOutOfGas(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("counter", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(0).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.OutOfGas().
				ReturnMessage(vmhost.ErrNotEnoughGas.Error())
		})
}

func TestExecution_CallWasmerError(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode([]byte("not WASM"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(increment).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_ChangeWasmerOpcodeCosts(t *testing.T) {
	contractCode := test.GetTestSCCode("misc", "../../")

	log := logger.GetOrCreate("vm/test")

	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(test.BlockchainHookStubForCall(contractCode, big.NewInt(0))).
		Build()
	defer func() {
		host.Reset()
	}()
	gasSchedule := host.GetGasScheduleMap()

	input := test.CreateTestContractCallInputBuilder().
		WithGasProvided(10000).
		WithFunction("iterate_over_byte_array").
		Build()

	vmOutput, err := host.RunSmartContractCall(input)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
	gasRemainingBeforeChange := vmOutput.GasRemaining
	log.Trace("gas remaining before change", "gas", gasRemainingBeforeChange)

	gasSchedule["WASMOpcodeCost"]["BrIf"] += 20
	host.GasScheduleChange(gasSchedule)

	vmOutput, err = host.RunSmartContractCall(input)
	verify = test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
	gasRemainingAfterChange := vmOutput.GasRemaining
	log.Trace("gas remaining after change", "gas", gasRemainingAfterChange)
	log.Trace("gas difference after change", "gas diff", gasRemainingBeforeChange-gasRemainingAfterChange)

	require.NotEqual(t, gasRemainingBeforeChange, gasRemainingAfterChange)
}

func TestExecution_ChangeWasmerAPICosts(t *testing.T) {
	contractCode := test.GetTestSCCode("misc", "../../")

	log := logger.GetOrCreate("vm/test")

	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(test.BlockchainHookStubForCall(contractCode, big.NewInt(0))).
		Build()
	defer func() {
		host.Reset()
	}()
	gasSchedule := host.GetGasScheduleMap()

	input := test.CreateTestContractCallInputBuilder().
		WithGasProvided(10000).
		WithFunction("iterate_over_byte_array").
		Build()

	vmOutput, err := host.RunSmartContractCall(input)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
	gasRemainingBeforeChange := vmOutput.GasRemaining
	log.Trace("gas remaining before change", "gas", gasRemainingBeforeChange)

	gasSchedule["BaseOpsAPICost"]["Finish"]++
	host.GasScheduleChange(gasSchedule)

	vmOutput, err = host.RunSmartContractCall(input)
	verify = test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
	gasRemainingAfterChange := vmOutput.GasRemaining
	log.Trace("gas remaining after change", "gas", gasRemainingAfterChange)
	log.Trace("gas difference after change", "gas diff", gasRemainingBeforeChange-gasRemainingAfterChange)

	require.NotEqual(t, gasRemainingBeforeChange, gasRemainingAfterChange)
}

func TestExecution_CallSCMethod_Init(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("counter", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("init").
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.UserError().
				ReturnMessage(vmhost.ErrInitFuncCalledInRun.Error())
		})
}

func TestExecution_CallSCMethod_MissingFunction(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("counter", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("wrong").
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.FunctionNotFound()
		})
}

func TestExecution_Call_Successful(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("counter", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(increment).
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetStorageDataCalled = func(scAddress []byte, key []byte) ([]byte, uint32, error) {
				return big.NewInt(1001).Bytes(), 0, nil
			}
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				Storage(
					test.CreateStoreEntry(test.ParentAddress).WithKey(counterKey).WithValue(big.NewInt(1002).Bytes()),
				)
		})
}

func TestExecution_CachingCompiledCode(t *testing.T) {
	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithBuiltinFunctions().
		Build()
	defer func() {
		host.Reset()
	}()

	scAddress := test.MakeTestSCAddress("counter")
	code := test.GetTestSCCode("counter", "../../")
	scAcct := world.CreateSmartContractAccount(test.ParentAddress, scAddress, code, world)
	world.PutAccount(scAcct)

	input := test.CreateTestContractCallInputBuilder().
		WithRecipientAddr(scAddress).
		WithGasProvided(100000).
		WithFunction(increment).
		Build()

	vmOutput, err := host.RunSmartContractCall(input)
	require.Nil(t, err)
	require.Zero(t, vmOutput.ReturnCode)
	require.NotEqual(t, vmOutput.GasRemaining, 100000)

	for i := 0; i < 3; i++ {
		vmOutput, err = host.RunSmartContractCall(input)
		require.Nil(t, err)
		require.Zero(t, vmOutput.ReturnCode)
		require.NotEqual(t, vmOutput.GasRemaining, 100000)
	}
}

func TestExecution_ManagedBuffers(t *testing.T) {
	var functionNumber = 0
	var mBuffer = [...]string{"mBufferMethod", "mBufferNewTest", "mBufferNewFromBytesTest", "mBufferSetRandomTest",
		"mBufferGetLengthTest", "mBufferGetBytesTest", "mBufferAppendTest", "mBufferToBigIntUnsignedTest",
		"mBufferToBigIntSignedTest", "mBufferFromBigIntUnsignedTest", "mBufferFromBigIntSignedTest",
		"mBufferStorageStoreTest", "mBufferStorageLoadTest"}

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferMethod
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(
					managedBuffer,
					[]byte("succ")).
				Storage(
					test.CreateStoreEntry(test.ParentAddress).WithKey(mBufferKey).WithValue(managedBuffer),
				)
		})
	functionNumber++

	numberOfReps := 100
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferNewTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(
					[]byte{byte(numberOfReps - 1)})
		})
	functionNumber++

	lengthOfBuffer := 64
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferNewFromBytesTest
			WithArguments([]byte{byte(numberOfReps)}, []byte{byte(lengthOfBuffer)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(
					managedBuffer)
		})
	functionNumber++

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferSetRandomTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer)
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferGetLengthTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(
					[]byte{byte(numberOfReps)})
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferGetBytesTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer, randomBuffer[:numberOfReps])
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferAppendTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			finalBuffer := make([]byte, 0)
			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
				finalBuffer = append(finalBuffer, randomBuffer...)
			}
			verify.Ok().
				ReturnData(finalBuffer)
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferToBigIntUnsignedTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer, randomBuffer)
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferToBigIntSignedTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer, twoscomplement.ToBytes(big.NewInt(0).SetBytes(randomBuffer))[1:])
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferFromBigIntUnsignedTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer, randomBuffer)
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferFromBigIntSignedTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			randomBuffer := make([]byte, numberOfReps)
			for i := 0; i < numberOfReps; i++ {
				_, _ = randReader.Read(randomBuffer)
			}
			verify.Ok().
				ReturnData(randomBuffer, twoscomplement.ToBytes(big.NewInt(0).SetBytes(randomBuffer))[1:])
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferStorageStoreTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			lastRandomBuffer := make([]byte, numberOfReps)
			lastRandomKey := make([]byte, 5)
			storage := make([]test.StoreEntry, 0)
			for i := 0; i < numberOfReps; i++ {
				keyBuffer := make([]byte, 5)
				randomBuffer := make([]byte, numberOfReps)
				_, _ = randReader.Read(keyBuffer)
				_, _ = randReader.Read(randomBuffer)
				entry := test.CreateStoreEntry(test.ParentAddress).WithKey(keyBuffer).WithValue(randomBuffer)
				storage = append(storage, entry)
				if i == numberOfReps-1 {
					lastRandomBuffer = randomBuffer
					lastRandomKey = keyBuffer
				}
			}
			verify.Ok().
				ReturnData(lastRandomBuffer, lastRandomKey).
				Storage(
					storage...,
				)
		})

	functionNumber++
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction(mBuffer[functionNumber]). // mBufferStorageLoadTest
			WithArguments([]byte{byte(numberOfReps)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			randReader := buildRandomizer(host)

			lastRandomBuffer := make([]byte, numberOfReps)
			lastRandomKey := make([]byte, 5)
			storage := make([]test.StoreEntry, 0)
			for i := 0; i < numberOfReps; i++ {
				keyBuffer := make([]byte, 5)
				randomBuffer := make([]byte, numberOfReps)
				_, _ = randReader.Read(keyBuffer)
				_, _ = randReader.Read(randomBuffer)
				entry := test.CreateStoreEntry(test.ParentAddress).WithKey(keyBuffer).WithValue(randomBuffer)
				storage = append(storage, entry)
				if i == numberOfReps-1 {
					lastRandomBuffer = randomBuffer
					lastRandomKey = keyBuffer
				}
			}
			verify.Ok().
				ReturnData(lastRandomBuffer, lastRandomKey).
				Storage(
					storage...,
				)
		})
}

func buildRandomizer(host vmhost.VMHost) io.Reader {
	// building the randomizer
	blockchainContext := host.Blockchain()
	previousRandomSeed := blockchainContext.LastRandomSeed()
	currentRandomSeed := blockchainContext.CurrentRandomSeed()
	txHash := host.Runtime().GetCurrentTxHash()

	blocksRandomSeed := append(previousRandomSeed, currentRandomSeed...)
	randomSeed := append(blocksRandomSeed, txHash...)
	randReader := vmMath.NewSeedRandReader(randomSeed)
	return randReader
}

func TestExecution_ManagedBuffers_SetByteSlice(t *testing.T) {
	runTestMBufferSetByteSliceDeploy(t, vmcommon.Ok)

	// Correct copying from "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890" over "abcdefghijklmnopqrstuvwxyz"
	runTestMBufferSetByteSlice(t, 0, 4, vmcommon.Ok, []byte("ABCDefghijklmnopqrstuvwxyz"))
	runTestMBufferSetByteSlice(t, 0, 8, vmcommon.Ok, []byte("ABCDEFGHijklmnopqrstuvwxyz"))
	runTestMBufferSetByteSlice(t, 18, 8, vmcommon.Ok, []byte("abcdefghijklmnopqrABCDEFGH"))
	runTestMBufferSetByteSlice(t, 10, 12, vmcommon.Ok, []byte("abcdefghijABCDEFGHIJKLwxyz"))
	runTestMBufferSetByteSlice(t, 25, 1, vmcommon.Ok, []byte("abcdefghijklmnopqrstuvwxyA"))
	runTestMBufferSetByteSlice(t, 0, 26, vmcommon.Ok, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))

	// Bounds exceeded, source remains unchanged lowercase.
	runTestMBufferSetByteSlice(t, 18, 9, vmcommon.Ok, []byte("abcdefghijklmnopqrstuvwxyz"))
	runTestMBufferSetByteSlice(t, -1, 2, vmcommon.Ok, []byte("abcdefghijklmnopqrstuvwxyz"))
	runTestMBufferSetByteSlice(t, 25, 2, vmcommon.Ok, []byte("abcdefghijklmnopqrstuvwxyz"))
	runTestMBufferSetByteSlice(t, 0, 27, vmcommon.Ok, []byte("abcdefghijklmnopqrstuvwxyz"))
}

func runTestMBufferSetByteSliceDeploy(t *testing.T, retCode vmcommon.ReturnCode) {
	input := test.CreateTestContractCreateInputBuilder().
		WithKDATransfers([]*vmcommon.KDATransfer{
			{
				KDAValue: big.NewInt(1000),
			},
		}).
		WithGasProvided(100_000).
		WithContractCode(test.GetTestSCCode("managed-buffers", "../../")).
		Build()

	test.BuildInstanceCreatorTest(t).
		WithInput(input).
		AndAssertResults(func(blockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ReturnCode(retCode)
		})

}

func runTestMBufferSetByteSlice(
	tb testing.TB,
	startPos int,
	copyLen int,
	retCode vmcommon.ReturnCode,
	expectedReturn []byte,
) {
	test.BuildInstanceCallTest(tb).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-buffers", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("mBufferSetByteSliceTest").
			WithArguments([]byte{byte(startPos)}, []byte{byte(copyLen)}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ReturnCode(retCode)
			if retCode == vmcommon.Ok {
				verify.ReturnData(expectedReturn)
			}
		})
}

func TestExecution_Call_GasConsumptionOnLocals(t *testing.T) {
	gasWithZeroLocals, gasSchedule := callCustomSCAndGetGasUsed(t, 0)
	costPerLocal := uint64(gasSchedule.WASMOpcodeCost.LocalAllocate)

	UnmeteredLocals := uint64(gasSchedule.WASMOpcodeCost.LocalsUnmetered)

	// Any number of local variables below `UnmeteredLocals` must be instantiated
	// without metering, i.e. gas-free.
	for _, locals := range []uint64{1, UnmeteredLocals / 2, UnmeteredLocals} {
		gasUsed, _ := callCustomSCAndGetGasUsed(t, locals)
		require.Equal(t, gasWithZeroLocals, gasUsed)
	}

	// Any number of local variables above `UnmeteredLocals` must be instantiated
	// with metering, i.e. will cost gas.
	for _, locals := range []uint64{UnmeteredLocals + 1, UnmeteredLocals * 2, UnmeteredLocals * 4} {
		gasUsed, _ := callCustomSCAndGetGasUsed(t, locals)
		meteredLocals := locals - UnmeteredLocals
		costOfLocals := costPerLocal * meteredLocals
		expectedGasUsed := gasWithZeroLocals + costOfLocals
		require.Equal(t, expectedGasUsed, gasUsed)
	}
}

func callCustomSCAndGetGasUsed(t *testing.T, locals uint64) (uint64, *config.GasCost) {
	var gasSchedule *config.GasCost
	var gasUsed uint64

	gasLimit := uint64(100000)
	code := makeBytecodeWithLocals(locals)

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(code)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(gasLimit).
			WithFunction("answer").
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			gasSchedule = host.Metering().GasSchedule()
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			compilationCost := uint64(len(code)) * gasSchedule.BaseOperationCost.CompilePerByte
			gasUsed = gasLimit - verify.VmOutput.GasRemaining - compilationCost
			verify.Ok()
		})

	return gasUsed, gasSchedule
}

func TestExecution_ExecuteOnSameContext_Simple(t *testing.T) {
	parentGasUsed := uint64(521)
	childGasUsed := uint64(6870)
	executionCost := parentGasUsed + childGasUsed

	var returnData [][]byte

	returnData = append(returnData, []byte("child"))
	returnData = append(returnData, []byte{})
	for i := 1; i < 100; i++ {
		returnData = append(returnData, []byte{byte(i)})
	}
	returnData = append(returnData, []byte{})
	returnData = append(returnData, []byte("child"))
	returnData = append(returnData, []byte{})
	for i := 1; i < 100; i++ {
		returnData = append(returnData, []byte{byte(i)})
	}
	returnData = append(returnData, []byte{})
	returnData = append(returnData, []byte("parent"))

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-same-ctx-simple-parent", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(test.ChildAddress).
				WithCode(test.GetTestSCCode("exec-same-ctx-simple-child", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction(parentFunctionChildCall).
			WithGasProvided(test.GasProvided).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				// test.ParentAddress
				// BalanceDelta(test.ParentAddress, 0).
				GasUsed(test.ParentAddress, parentGasUsed+childGasUsed).
				// test.ChildAddress
				// BalanceDelta(test.ChildAddress, 0).
				GasUsed(test.ChildAddress, 0).
				// other
				GasRemaining(test.GasProvided - executionCost).
				ReturnData(returnData...)
		})
}

func TestExecution_Call_Breakpoints(t *testing.T) {
	t.Parallel()

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("breakpoint", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("testFunc").
			WithArguments([]byte{15}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte{100})
		})
}

func TestExecution_Call_Breakpoints_UserError(t *testing.T) {
	t.Parallel()
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("breakpoint", "../../"))).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("testFunc").
			WithArguments([]byte{1}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.UserError().
				ReturnData().
				ReturnMessage("exit here")
		})
}

func TestExecution_ExecuteOnSameContext_Recursive_Direct_ErrMaxInstances(t *testing.T) {
	recursiveCalls := byte(11)
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-same-ctx-recursive", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction(callRecursive).
			WithGasProvided(test.GasProvided).
			WithArguments([]byte{recursiveCalls}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			if host.Runtime().SyncExecAPIErrorShouldFailExecution() == false {
				verify.Ok().
					// Balance(test.ParentAddress, 1000).
					// BalanceDelta(test.ParentAddress, 0).
					ReturnData(
						[]byte(fmt.Sprintf("Rfinish%03d", recursiveCalls)),
						[]byte("fail"),
					).
					Storage(
						test.CreateStoreEntry(test.ParentAddress).
							WithKey([]byte(fmt.Sprintf("Rkey%03d.........................", recursiveCalls))).
							WithValue([]byte(fmt.Sprintf("Rvalue%03d", recursiveCalls))),
					)
				require.Equal(t, int64(1), host.ManagedTypes().GetBigIntOrCreate(16).Int64())
			} else {
				verify.ExecutionFailed().
					ReturnMessage(vmhost.ErrExecutionFailed.Error()).
					HasRuntimeErrors(vmhost.ErrMaxInstancesReached.Error(), vmhost.ErrExecutionFailed.Error()).
					GasRemaining(0)
			}
		})
}

func TestExecution_ExecuteOnSameContext_Recursive_Mutual_SCs_OutOfGas(t *testing.T) {
	// Call parentFunctionChildCall() of the parent SC, which will call the child
	// SC and pass some arguments using executeOnDestContext().
	recursiveCalls := byte(5)

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-same-ctx-recursive-parent", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(test.ChildAddress).
				WithCode(test.GetTestSCCode("exec-same-ctx-recursive-child", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction(parentCallsChild).
			WithGasProvided(10000).
			WithArguments([]byte{recursiveCalls}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			if host.Runtime().SyncExecAPIErrorShouldFailExecution() == false {
				verify.OutOfGas().
					ReturnMessage(vmhost.ErrNotEnoughGas.Error()).
					GasRemaining(0)
			} else {
				verify.OutOfGas().
					ReturnMessage(vmhost.ErrNotEnoughGas.Error()).
					HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error()).
					GasRemaining(0)
			}
		})
}

func TestExecution_ExecuteOnDestContext_Recursive_Mutual_SCs_OutOfGas(t *testing.T) {
	// Call parentFunctionChildCall() of the parent SC, which will call the child
	// SC and pass some arguments using executeOnDestContext().

	recursiveCalls := byte(5)

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-dest-ctx-recursive-parent", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(test.ChildAddress).
				WithCode(test.GetTestSCCode("exec-dest-ctx-recursive-child", "../../")).
				WithBalance(1000),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction(parentCallsChild).
			WithGasProvided(10000).
			WithArguments([]byte{recursiveCalls}).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			if host.Runtime().SyncExecAPIErrorShouldFailExecution() == false {
				verify.OutOfGas().
					ReturnMessage(vmhost.ErrNotEnoughGas.Error())
			} else {
				verify.OutOfGas().
					ReturnMessage(vmhost.ErrNotEnoughGas.Error()).
					HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error()).
					GasRemaining(0)
			}
		})
}

func TestExecution_ExecuteOnSameContext_MultipleChildren(t *testing.T) {
	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithBuiltinFunctions().
		Build()
	defer func() {
		host.Reset()
	}()

	alphaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/alpha", "alpha", "../../")
	alpha := test.AddTestSmartContractToWorld(world, "alphaSC", alphaCode)
	alpha.Balance = big.NewInt(100)
	world.PutAccount(alpha)

	betaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/beta", "beta", "../../")
	gammaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/gamma", "gamma", "../../")
	deltaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/delta", "delta", "../../")

	_ = test.AddTestSmartContractToWorld(world, "betaSC", betaCode)
	_ = test.AddTestSmartContractToWorld(world, "gammaSC", gammaCode)
	_ = test.AddTestSmartContractToWorld(world, "deltaSC", deltaCode)

	expectedReturnData := [][]byte{
		[]byte("arg1"),
		[]byte("succ"),
		[]byte("arg2"),
		[]byte("succ"),
		[]byte("arg3"),
		[]byte("succ"),
	}

	// Alpha uses executeOnSameContext() to call beta, gamma and delta one after
	// the other, in the same transaction.
	input := test.DefaultTestContractCallInput()
	input.Function = "callChildrenDirectly_SameCtx"
	input.GasProvided = 1000000
	input.RecipientAddr = alpha.Address

	vmOutput, err := host.RunSmartContractCall(input)

	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok().
		ReturnData(expectedReturnData...)
}

func TestExecution_ExecuteOnDestContext_MultipleChildren(t *testing.T) {
	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithBuiltinFunctions().
		Build()
	defer func() {
		host.Reset()
	}()

	alphaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/alpha", "alpha", "../../")
	alpha := test.AddTestSmartContractToWorld(world, "alphaSC", alphaCode)
	alpha.Balance = big.NewInt(100)
	world.PutAccount(alpha)

	betaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/beta", "beta", "../../")
	gammaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/gamma", "gamma", "../../")
	deltaCode := test.GetTestSCCodeModule("exec-sync-ctx-multiple/delta", "delta", "../../")

	_ = test.AddTestSmartContractToWorld(world, "betaSC", betaCode)
	_ = test.AddTestSmartContractToWorld(world, "gammaSC", gammaCode)
	_ = test.AddTestSmartContractToWorld(world, "deltaSC", deltaCode)

	expectedReturnData := [][]byte{
		[]byte("arg1"),
		[]byte("succ"),
		[]byte("arg2"),
		[]byte("succ"),
		[]byte("arg3"),
		[]byte("succ"),
	}

	// Alpha uses executeOnDestContext() to call beta, gamma and delta one after
	// the other, in the same transaction.
	input := test.DefaultTestContractCallInput()
	input.Function = "callChildrenDirectly_DestCtx"
	input.GasProvided = 1000000
	input.RecipientAddr = alpha.Address

	vmOutput, err := host.RunSmartContractCall(input)

	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok().
		ReturnData(expectedReturnData...)
}

func TestExecution_ExecuteOnDestContextByCaller_SimpleTransfer(t *testing.T) {
	// The child contract is designed to send some tokens back to its caller, as
	// many as requested. The parent calls the child using
	// executeOnDestContextByCaller(), which means that the child will not see
	// the parent as its caller, but the original caller of the transaction
	// instead. Thus, the original caller (the user address) will receive 42
	// tokens, and not the parent, even if the parent is the one making the call
	// to the child.

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCodeModule("exec-dest-ctx-by-caller/parent", "parent", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(test.ChildAddress).
				WithCode(test.GetTestSCCodeModule("exec-dest-ctx-by-caller/child", "child", "../../")).
				WithBalance(1000),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction("call_child").
			WithGasProvided(5000).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ReturnCode(vmcommon.VMContractInvalid)
		})
}

func TestExecution_DeployNewContractFromExistingCode_Success(t *testing.T) {
	sourceAddress := test.MakeTestSCAddress("sourceAddress")
	sourceCode := test.GetTestSCCode("init-correct", "../../")
	generatedNewAddress := []byte("newAddress")

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(sourceAddress).
				WithCode(sourceCode).
				WithBalance(1000),
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("deployer-fromanother-contract", "../../")).
				WithBalance(1000),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction("deployCodeFromAnotherContract").
			WithArguments(sourceAddress).
			WithGasProvided(1_000_000).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().
				Code(generatedNewAddress, sourceCode).
				CodeMetadata(generatedNewAddress, test.DefaultCodeMetadata).
				ReturnData(
					// returned by the new deployed contract from the existing source code
					[]byte("init successful"),
					// returned by the deployer contract
					[]byte("succ"),
				)
		})
}

func TestExecution_CreateNewContract_Fail(t *testing.T) {
	childCode := test.GetTestSCCode("init-correct", "../../")

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("deployer", "../../")).
				WithBalance(1000),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction("deployChildContract").
			WithGasProvided(1_000_000).
			WithArguments([]byte{'A'}, []byte{1}).
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetStorageDataCalled = func(address []byte, key []byte) ([]byte, uint32, error) {
				if bytes.Equal(address, test.ParentAddress) {
					if bytes.Equal(key, []byte{'A'}) {
						return childCode, 0, nil
					}
					return nil, 0, nil
				}
				return nil, 0, vmhost.ErrInvalidAccount
			}
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.WithTrace().ExecutionFailed().
				ReturnMessage("error signalled by smartcontract")
		})
}

func TestExecution_Mocked_Wasmer_Instances(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *contextmock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("callChild", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						host.Output().Finish([]byte("parent returns this"))
						host.Metering().UseGas(500)
						_, err := host.Storage().SetStorage([]byte("parent"), []byte("parent storage"))
						require.Nil(t, err)
						childInput := test.DefaultTestContractCallInput()
						childInput.CallerAddr = test.ParentAddress
						childInput.RecipientAddr = test.ChildAddress
						childInput.KDATransfers = []*vmcommon.KDATransfer{
							{
								KDAValue: big.NewInt(4),
							},
						}

						childInput.Function = "doSomething"
						childInput.GasProvided = 1000
						_, err = host.ExecuteOnDestContext(childInput)
						require.Nil(t, err)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(0).
				WithMethods(func(childInstance *contextmock.InstanceMock, config interface{}) {
					childInstance.AddMockMethod("doSomething", func() *contextmock.InstanceMock {
						host := childInstance.Host
						host.Output().Finish([]byte("child returns this"))
						host.Metering().UseGas(100)
						_, err := host.Storage().SetStorage([]byte("child"), []byte("child storage"))
						require.Nil(t, err)
						return childInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("callChild").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				// test.ParentAddress
				// Balance(test.ParentAddress, 1000).
				// BalanceDelta(test.ParentAddress, -4).
				GasUsed(test.ParentAddress, 547).
				// BalanceDelta(test.ChildAddress, 4).
				GasUsed(test.ChildAddress, 146).
				GasRemaining(307).
				ReturnData([]byte("parent returns this"), []byte("child returns this")).
				Storage(
					test.CreateStoreEntry(test.ParentAddress).WithKey([]byte("parent")).WithValue([]byte("parent storage")),
					test.CreateStoreEntry(test.ChildAddress).WithKey([]byte("child")).WithValue([]byte("child storage")),
				)
		})
	assert.Nil(t, err)
}

func TestExecution_Mocked_Warm_Instances_Same_Contract_Same_Address(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *contextmock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("callChild", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						instance := contextmock.GetMockInstance(host)

						vmhooks.WithFaultAndHost(host, vmhost.ErrNotEnoughGas, true)

						childInput := test.DefaultTestContractCallInput()
						childInput.CallerAddr = test.ParentAddress
						childInput.RecipientAddr = test.ParentAddress
						childInput.KDATransfers = []*vmcommon.KDATransfer{
							{
								KDAValue: big.NewInt(4),
							},
						}
						childInput.Function = "doSomething"
						childInput.GasProvided = 1000
						_, err := host.ExecuteOnDestContext(childInput)
						require.NotNil(t, err)

						return instance
					})
					parentInstance.AddMockMethod("doSomething", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						instance := contextmock.GetMockInstance(host)
						host.Runtime().SignalUserError("my user error")
						return instance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(2000).
			WithFunction("callChild").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.OutOfGas()
		})
	assert.Nil(t, err)
}

func TestExecution_Mocked_Warm_Instances_Same_Contract_Different_Address(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithCodeHash(UniqueCodeHash).
				WithMethods(func(parentInstance *contextmock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("callChild", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						instance := contextmock.GetMockInstance(host)

						vmhooks.WithFaultAndHost(host, vmhost.ErrNotEnoughGas, true)

						childInput := test.DefaultTestContractCallInput()
						childInput.CallerAddr = test.ParentAddress
						childInput.RecipientAddr = test.ChildAddress
						childInput.KDATransfers = []*vmcommon.KDATransfer{
							{
								KDAValue: big.NewInt(4),
							},
						}
						childInput.Function = "doSomething"
						childInput.GasProvided = 1000
						_, err := host.ExecuteOnDestContext(childInput)
						require.NotNil(t, err)

						return instance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(1000).
				WithCodeHash(UniqueCodeHash).
				WithMethods(func(childInstance *contextmock.InstanceMock, config interface{}) {
					childInstance.AddMockMethod("doSomething", func() *contextmock.InstanceMock {
						host := childInstance.Host
						instance := contextmock.GetMockInstance(host)
						host.Runtime().SignalUserError("my user error")
						return instance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(2000).
			WithFunction("callChild").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.OutOfGas()
		})
	require.Nil(t, err)
}

func TestExecution_Mocked_ClearReturnData(t *testing.T) {
	zero := "zero"
	one := "one"
	two := "two"
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *contextmock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("callChild", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						instance := contextmock.GetMockInstance(host)
						childInput := test.DefaultTestContractCallInput()
						childInput.CallerAddr = test.ParentAddress
						childInput.RecipientAddr = test.ChildAddress
						childInput.Function = "doSomething"
						childInput.GasProvided = 1000
						returnValue := contracts.ExecuteOnDestContextInMockContracts(host, childInput, big.NewInt(0))
						assert.Equal(t, int32(0), returnValue)

						instance.BreakpointValue = 0
						returnData := vmhooks.GetReturnDataWithHostAndTypedArgs(host, -1)
						assert.Equal(t, vmhost.BreakpointExecutionFailed, instance.BreakpointValue)
						assert.Nil(t, returnData)

						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 0)
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)
						assert.Equal(t, zero, string(returnData))

						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 1)
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)
						assert.Equal(t, one, string(returnData))

						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 2)
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)
						assert.Equal(t, two, string(returnData))

						instance.BreakpointValue = 0
						vmhooks.DeleteFromReturnDataWithHost(host, 0)
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 0)
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)
						assert.Equal(t, one, string(returnData))

						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 1)
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)
						assert.Equal(t, two, string(returnData))

						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 2)
						assert.Equal(t, vmhost.BreakpointExecutionFailed, instance.BreakpointValue)
						assert.Nil(t, returnData)

						instance.BreakpointValue = 0
						vmhooks.DeleteFromReturnDataWithHost(host, 0)
						vmhooks.DeleteFromReturnDataWithHost(host, 0)
						remainingReturnData := host.Output().ReturnData()
						assert.Equal(t, remainingReturnData, [][]byte{})
						assert.Equal(t, vmhost.BreakpointNone, instance.BreakpointValue)

						instance.BreakpointValue = 0
						vmhooks.CleanReturnDataWithHost(host)
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 0)
						assert.Equal(t, vmhost.BreakpointExecutionFailed, instance.BreakpointValue)
						assert.Nil(t, returnData)
						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 1)
						assert.Equal(t, vmhost.BreakpointExecutionFailed, instance.BreakpointValue)
						assert.Nil(t, returnData)
						instance.BreakpointValue = 0
						returnData = vmhooks.GetReturnDataWithHostAndTypedArgs(host, 2)
						assert.Equal(t, vmhost.BreakpointExecutionFailed, instance.BreakpointValue)
						assert.Nil(t, returnData)

						return instance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(0).
				WithMethods(func(childInstance *contextmock.InstanceMock, config interface{}) {
					childInstance.AddMockMethod("doSomething", func() *contextmock.InstanceMock {
						host := childInstance.Host
						instance := contextmock.GetMockInstance(host)
						host.Output().Finish([]byte(zero))
						host.Output().Finish([]byte(one))
						host.Output().Finish([]byte(two))
						return instance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("callChild").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			// No assertions here, because they were performed during the instance call
		})
	require.Nil(t, err)
}

var codeOpcodes = test.GetTestSCCode("opcodes", "../../")

func TestExecution_Opcodes_MemoryGrow(t *testing.T) {
	maxGrows := uint32(math.MaxUint32)
	maxDelta := uint32(10)
	argDelta := int64(10)
	runMemGrowTest(t, maxGrows, maxDelta, argDelta, 10, vmcommon.Ok)
}

func TestExecution_Opcodes_MemoryGrow_Limit(t *testing.T) {
	maxGrows := uint32(10)
	maxDelta := uint32(10)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta), int64(maxGrows-1), vmcommon.Ok)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta), int64(maxGrows), vmcommon.Ok)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta), int64(maxGrows+1), vmcommon.VMExecutionFailed)
}

func TestExecution_Opcodes_MemoryGrowDelta(t *testing.T) {
	maxGrows := uint32(10)
	maxDelta := uint32(10)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta-1), 1, vmcommon.Ok)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta), 1, vmcommon.Ok)
	runMemGrowTest(t, maxGrows, maxDelta, int64(maxDelta+1), 1, vmcommon.VMExecutionFailed)
}

func BenchmarkOpcodeMemoryGrow(b *testing.B) {
	maxGrows := uint32(math.MaxUint32)
	maxDelta := uint32(10)
	argDelta := int64(10)
	runMemGrowTest(b, maxGrows, maxDelta, argDelta, int64(b.N), vmcommon.Ok)
}

func runMemGrowTest(
	tb testing.TB,
	maxMemGrow uint32,
	maxMemGrowDelta uint32,
	argMemGrowDelta int64,
	reps int64,
	expectedRetCode vmcommon.ReturnCode,
) {
	repsBigInt := big.NewInt(reps)
	repsBytes := vmhost.PadBytesLeft(repsBigInt.Bytes(), 8)

	deltaBigInt := big.NewInt(argMemGrowDelta)
	deltaBytes := vmhost.PadBytesLeft(deltaBigInt.Bytes(), 8)

	test.BuildInstanceCallTest(tb).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(codeOpcodes)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(80000).
			WithFunction("memGrowDelta").
			WithArguments(repsBytes, deltaBytes).
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			gasSchedule := host.Metering().GasSchedule()
			gasSchedule.WASMOpcodeCost.MaxMemoryGrow = maxMemGrow
			gasSchedule.WASMOpcodeCost.MaxMemoryGrowDelta = maxMemGrowDelta
		}).
		AndAssertResults(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.WithTrace().ReturnCode(expectedRetCode)
			if expectedRetCode == vmcommon.VMExecutionFailed {
				vmOutput := verify.VmOutput
				require.Len(tb, vmOutput.Logs, 1)
				fmt.Println(vmOutput.Logs[0].Data[0])
				fmt.Println(vmOutput.Logs[0].Data[0])

				require.Contains(tb, string(vmOutput.Logs[0].Data[0]), vmhost.ErrMemoryLimit.Error())
			}
		})
}

func TestExecution_Opcodes_MemGrowWrongIndex(t *testing.T) {
	code := test.GetTestSCCode("memgrow-wrong", "../../")
	reps := int64(1)
	repsBigInt := big.NewInt(reps)
	repsBytes := vmhost.PadBytesLeft(repsBigInt.Bytes(), 8)

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(code)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(80000).
			WithFunction("memGrowWrongIndex").
			WithArguments(repsBytes).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}

func TestExecution_Opcodes_MemorySize(t *testing.T) {
	reps := big.NewInt(10000)
	repsBytes := vmhost.PadBytesLeft(reps.Bytes(), 8)

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(codeOpcodes)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithGasProvided(1000000).
			WithFunction("memSize").
			WithArguments(repsBytes).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()
		})
}

func TestExecutionRuntimeCodeSizeUpgradeContract(t *testing.T) {
	oldCode := test.GetTestSCCode("answer", "../../")
	newCode := test.GetTestSCCode("counter", "../../")

	expectedCodeSize := uint64(len(newCode))

	testCase := test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithOwner(test.UserAddress).
				WithCode(oldCode)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithCallerAddr(test.UserAddress).
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1_000_000).
			WithFunction("answer").
			Build())

	testCase.
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()
			require.Equal(t,
				uint64(len(oldCode)),
				host.Runtime().GetSCCodeSize())
		})

	testCase.
		WithInput(test.CreateTestContractCallInputBuilder().
			WithCallerAddr(test.UserAddress).
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1_000_000).
			WithFunction(vmhost.UpgradeFunctionName).
			WithArguments(newCode, test.DefaultCodeMetadata).
			Build())

	testCase.AndAssertResultsWithoutReset(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
		verify.Ok()
		require.Equal(t,
			expectedCodeSize,
			host.Runtime().GetSCCodeSize())
	})
}

func TestExecution_WarmInstance_ExecutionStatus(t *testing.T) {
	testCase := test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("breakpoint", "../../")))

	makeInput := func(behaviour byte) *vmcommon.ContractCallInput {
		return test.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("testFunc").
			WithArguments([]byte{behaviour}).
			Build()
	}

	vmInputOk := makeInput(0)
	vmInputUserError := makeInput(1)
	vmInputExecutionFailed := makeInput(2)

	testCase.WithInput(vmInputOk).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().ReturnData([]byte{100}).ReturnMessage("")
		})

	testCase.WithInput(vmInputUserError).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.UserError().ReturnData().ReturnMessage("exit here")
		})

	testCase.WithInput(vmInputOk).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().ReturnData([]byte{100}).ReturnMessage("")
		})

	testCase.WithInput(vmInputExecutionFailed).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnData().ReturnMessage("execution failed")
		})

	testCase.WithInput(vmInputExecutionFailed).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnData().ReturnMessage("execution failed")
		})

	testCase.WithInput(vmInputExecutionFailed).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnData().ReturnMessage("execution failed")
		})

	testCase.WithInput(vmInputExecutionFailed).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnData().ReturnMessage("execution failed")
		})

	testCase.WithInput(vmInputOk).
		AndAssertResultsWithoutReset(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok().ReturnData([]byte{100}).ReturnMessage("")
		})
}

func TestExecution_Mocked_OnSameFollowedByOnDest(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *contextmock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("callChild", func() *contextmock.InstanceMock {
						host := parentInstance.Host
						host.Output().Finish([]byte("parent returns this"))
						host.Metering().UseGas(500)
						vmhooks.ExecuteOnSameContextWithTypedArgs(host, 1000, big.NewInt(4), []byte("doSomething"), test.ChildAddress, make([][]byte, 2))
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(100).
				WithMethods(func(childInstance *contextmock.InstanceMock, config interface{}) {
					childInstance.AddMockMethod("doSomething", func() *contextmock.InstanceMock {
						host := childInstance.Host
						host.Output().Finish([]byte("child returns this"))
						host.Metering().UseGas(100)
						vmhooks.ExecuteOnDestContextWithTypedArgs(host, 100, big.NewInt(2), []byte("doSomethingNephew"), test.NephewAddress, make([][]byte, 2))
						return childInstance
					})
				}),
			test.CreateMockContract(test.NephewAddress).
				WithBalance(0).
				WithMethods(func(nephewInstance *contextmock.InstanceMock, config interface{}) {
					nephewInstance.AddMockMethod("doSomethingNephew", func() *contextmock.InstanceMock {
						host := nephewInstance.Host
						host.Output().Finish([]byte("newphew returns this"))
						caller := host.Runtime().GetVMInput().CallerAddr
						if bytes.Equal(caller, test.ParentAddress) {
							host.Output().Finish([]byte("OK"))
						}
						return nephewInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("callChild").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			accountHandler, _ := world.GetUserAccount(test.ParentAddress)
			accountHandler.DataTrieTracker().SaveKeyValue([]byte("child"), test.ChildData)
			world.AccountsCacher.SaveAll()
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte("parent returns this"), []byte("child returns this"), []byte("newphew returns this"), []byte("OK"))
		})
	require.Nil(t, err)
}

// makeBytecodeWithLocals rewrites the bytecode of "answer" to change the
// number of i64 locals it instantiates
func makeBytecodeWithLocals(numLocals uint64) []byte {
	originalCode := test.GetTestSCCode("answer-locals", "../../")
	firstSlice := originalCode[:0x5B]
	secondSlice := originalCode[0x5C:]

	encodedNumLocals := vmhost.U64ToLEB128(numLocals)
	extraBytes := len(encodedNumLocals) - 1

	result := make([]byte, 0)
	result = append(result, firstSlice...)
	result = append(result, encodedNumLocals...)
	result = append(result, secondSlice...)

	result[0x57] = byte(int(result[0x57]) + extraBytes)
	result[0x59] = byte(int(result[0x59]) + extraBytes)

	return result
}

// modifyERC20BytecodeWithCustomTransferEvent rewrites the bytecode of the ERC20
// contract to change the first bytes of its transferEvent bytes
func modifyERC20BytecodeWithCustomTransferEvent(erc20Bytecode []byte, replaceBytes []byte) {
	transferEventBytecodeOffset := 0x144B

	for i, b := range replaceBytes {
		erc20Bytecode[transferEventBytecodeOffset+i] = b
	}
}

func runNContractsForHostAndVerify(tb testing.TB, host vmhost.VMHost, input *vmcommon.ContractCallInput, n int) {
	for i := 0; i < n; i++ {
		vmOutput, err := host.RunSmartContractCall(input)
		verify := test.NewVMOutputVerifier(tb, vmOutput, err)
		verify.Ok()
	}
}
