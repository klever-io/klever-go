package contexts

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto/factory"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon/testexecutor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/kvm/wasmbytes"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

var defaultHasher = &blake2b.Blake2b{}

const counterWasmCode = "./../../test/contracts/counter/output/counter.wasm"

var vmType = []byte("type")

func InitializeVMAndWasmer() *contextmock.VMHostMock {
	gasSchedule := config.MakeGasMapForTests()

	host := &contextmock.VMHostMock{}

	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(gasSchedule)
	host.MeteringContext = mockMetering
	host.BlockchainContext, _ = NewBlockchainContext(host, worldmock.NewMockWorld())
	host.OutputContext, _ = NewOutputContext(host)
	host.CryptoHook = factory.NewVMCrypto()
	host.ForkControllerContext = commonMock.NewForkControllerStub()
	return host
}

func makeDefaultRuntimeContext(t *testing.T, host vmhost.VMHost) *runtimeContext {
	execFactory := testexecutor.NewDefaultTestExecutorFactory(t)
	exec, err := execFactory.CreateExecutor(executor.ExecutorFactoryArgs{
		VMHooks:          vmhooks.NewVMHooksImpl(host),
		ExecutionTimeout: time.Minute,
	})
	require.Nil(t, err)
	runtimeCtx, err := NewRuntimeContext(
		host,
		vmType,
		builtInFunctions.NewBuiltInFunctionContainer(),
		exec,
		defaultHasher,
	)
	require.Nil(t, err)
	require.NotNil(t, runtimeCtx)

	return runtimeCtx
}
func TestNewRuntimeContextErrors(t *testing.T) {
	host := InitializeVMAndWasmer()
	bfc := builtInFunctions.NewBuiltInFunctionContainer()
	hasher := defaultHasher

	execFactory := testexecutor.NewDefaultTestExecutorFactory(t)
	exec, err := execFactory.CreateExecutor(executor.ExecutorFactoryArgs{
		VMHooks:          vmhooks.NewVMHooksImpl(host),
		ExecutionTimeout: time.Minute,
	})
	require.Nil(t, err)

	t.Run("NilHost", func(t *testing.T) {
		runtimeCtx, err := NewRuntimeContext(nil, vmType, bfc, exec, hasher)
		require.Nil(t, runtimeCtx)
		require.ErrorIs(t, err, vmhost.ErrNilVMHost)
	})
	t.Run("NilVMType", func(t *testing.T) {
		runtimeCtx, err := NewRuntimeContext(host, nil, bfc, exec, hasher)
		require.Nil(t, runtimeCtx)
		require.ErrorIs(t, err, vmhost.ErrNilVMType)
	})
	t.Run("NilBuiltinFuncContainer", func(t *testing.T) {
		runtimeCtx, err := NewRuntimeContext(host, vmType, nil, exec, hasher)
		require.Nil(t, runtimeCtx)
		require.ErrorIs(t, err, vmhost.ErrNilBuiltInFunctionsContainer)
	})
	t.Run("NilExecutor", func(t *testing.T) {
		runtimeCtx, err := NewRuntimeContext(host, vmType, bfc, nil, hasher)
		require.Nil(t, runtimeCtx)
		require.ErrorIs(t, err, vmhost.ErrNilExecutor)
	})
	t.Run("NilHasher", func(t *testing.T) {
		runtimeCtx, err := NewRuntimeContext(host, vmType, bfc, exec, nil)
		require.Nil(t, runtimeCtx)
		require.ErrorIs(t, err, vmhost.ErrNilHasher)
	})
}

func TestNewRuntimeContext(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	require.Equal(t, &vmcommon.ContractCallInput{}, runtimeCtx.vmInput)
	require.Equal(t, []byte{}, runtimeCtx.codeAddress)
	require.Equal(t, "", runtimeCtx.callFunction)
	require.Equal(t, false, runtimeCtx.readOnly)
}

func TestRuntimeContext_InitState(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.vmInput = nil
	runtimeCtx.codeAddress = []byte("some address")
	runtimeCtx.callFunction = "a function"
	runtimeCtx.readOnly = true
	runtimeCtx.iTracker.codeSize = 1024

	runtimeCtx.InitState()

	require.Equal(t, &vmcommon.ContractCallInput{}, runtimeCtx.vmInput)
	require.Equal(t, []byte{}, runtimeCtx.codeAddress)
	require.Equal(t, "", runtimeCtx.callFunction)
	require.Equal(t, false, runtimeCtx.readOnly)
	require.Equal(t, uint64(0), runtimeCtx.iTracker.codeSize)
}

func TestRuntimeContext_CodeSizeFix(t *testing.T) {
	host := InitializeVMAndWasmer()

	runtimeContext := makeDefaultRuntimeContext(t, host)
	defer runtimeContext.ClearWarmInstanceCache()

	runtimeContext.iTracker.codeSize = 1024

	runtimeContext.InitState()
	require.Equal(t, uint64(0), runtimeContext.GetSCCodeSize())
}

func TestRuntimeContext_NewWasmerInstance(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	var dummy []byte
	err := runtimeCtx.StartWasmerInstance(dummy, gasLimit, false)
	require.NotNil(t, err)
	require.EqualError(t, err, "invalid bytecode: ")
	require.Zero(t, runtimeCtx.GetSCCodeSize())

	gasLimit = uint64(100000000)
	dummy = []byte("contract")
	err = runtimeCtx.StartWasmerInstance(dummy, gasLimit, false)
	require.NotNil(t, err)

	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err = runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)
	require.Equal(t, vmhost.BreakpointNone, runtimeCtx.GetRuntimeBreakpointValue())
	require.Equal(t, uint64(len(contractCode)), runtimeCtx.GetSCCodeSize())
}

func TestRuntimeContext_StartWasmerInstanceKeysCacheOnExecutedCode(t *testing.T) {
	contractCode := vmhost.GetSCCode(counterWasmCode)
	executedHash := defaultHasher.Compute(string(contractCode))
	gasLimit := uint64(100000000)
	committedHash := bytes.Repeat([]byte{0xAB}, 32)

	var savedKey []byte

	host := InitializeVMAndWasmer()
	host.BlockchainContext = &hostmock.BlockchainContextStub{
		GetCodeHashCalled:      func([]byte) []byte { return committedHash },
		GetCompiledCodeCalled:  func([]byte) (bool, []byte) { return false, nil },
		SaveCompiledCodeCalled: func(codeHash []byte, _ []byte) { savedKey = codeHash },
	}
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)
	require.Equal(t, executedHash, runtimeCtx.iTracker.CodeHash())
	require.Equal(t, executedHash, savedKey)
}

func TestRuntimeContext_StartWasmerInstanceDoesNotReuseWarmInstanceForDifferentCode(t *testing.T) {
	firstCode := vmhost.GetSCCode(counterWasmCode)
	secondCode := vmhost.GetSCCode("./../../test/contracts/init-simple/output/init-simple.wasm")
	gasLimit := uint64(100000000)
	committedHash := bytes.Repeat([]byte{0xAB}, 32)

	host := InitializeVMAndWasmer()
	host.BlockchainContext = &hostmock.BlockchainContextStub{
		GetCodeHashCalled:     func([]byte) []byte { return committedHash },
		GetCompiledCodeCalled: func([]byte) (bool, []byte) { return false, nil },
	}
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)
	runtimeCtx.SetCodeAddress([]byte("smartcontract"))

	err := runtimeCtx.StartWasmerInstance(firstCode, gasLimit, false)
	require.Nil(t, err)
	require.True(t, runtimeCtx.HasFunction("increment"))
	firstInstanceID := runtimeCtx.iTracker.instance.ID()

	err = runtimeCtx.StartWasmerInstance(secondCode, gasLimit, false)
	require.Nil(t, err)
	require.NotEqual(t, firstInstanceID, runtimeCtx.iTracker.instance.ID())
	require.False(t, runtimeCtx.HasFunction("increment"))
}

func TestRuntimeContext_StateSettersAndGetters(t *testing.T) {
	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}

	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	arguments := [][]byte{[]byte("argument 1"), []byte("argument 2")}
	kdaTransfer := &vmcommon.KDATransfer{
		KDAValue:      big.NewInt(4242),
		KDATokenName:  []byte("random_token"),
		KDATokenType:  uint32(core.NonFungible),
		KDATokenNonce: 94,
	}

	vmInput := vmcommon.VMInput{
		CallerAddr:   []byte("caller"),
		Arguments:    arguments,
		KDATransfers: []*vmcommon.KDATransfer{kdaTransfer},
	}
	callInput := &vmcommon.ContractCallInput{
		VMInput:       vmInput,
		RecipientAddr: []byte("recipient"),
		Function:      "test function",
	}

	runtimeCtx.InitStateFromContractCallInput(callInput)
	require.Equal(t, []byte("caller"), runtimeCtx.GetVMInput().CallerAddr)
	require.Equal(t, []byte("recipient"), runtimeCtx.GetContextAddress())
	require.Equal(t, "test function", runtimeCtx.FunctionName())
	require.Equal(t, vmType, runtimeCtx.GetVMType())
	require.Equal(t, arguments, runtimeCtx.Arguments())

	runtimeInput := runtimeCtx.GetVMInput()
	require.Zero(t, big.NewInt(4242).Cmp(runtimeInput.KDATransfers[0].KDAValue))
	require.True(t, bytes.Equal([]byte("random_token"), runtimeInput.KDATransfers[0].KDATokenName))
	require.Equal(t, uint32(core.NonFungible), runtimeInput.KDATransfers[0].KDATokenType)
	require.Equal(t, uint64(94), runtimeInput.KDATransfers[0].KDATokenNonce)

	vmInput2 := vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: []byte("caller2"),
			Arguments:  arguments,
		},
	}
	runtimeCtx.SetVMInput(&vmInput2)
	require.Equal(t, []byte("caller2"), runtimeCtx.GetVMInput().CallerAddr)

	runtimeCtx.SetCodeAddress([]byte("smartcontract"))
	require.Equal(t, []byte("smartcontract"), runtimeCtx.codeAddress)
}

func TestRuntimeContext_PushPopInstance(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	oldCodeSize := uint64(len(contractCode))
	newCodeSize := oldCodeSize + 84

	gasLimit := uint64(100000000)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)
	require.Equal(t, oldCodeSize, runtimeCtx.GetSCCodeSize())

	instance := runtimeCtx.iTracker.instance

	runtimeCtx.pushInstance()
	runtimeCtx.iTracker.instance = &contextmock.InstanceMock{}
	runtimeCtx.iTracker.codeSize = newCodeSize
	require.Equal(t, newCodeSize, runtimeCtx.GetSCCodeSize())
	require.Equal(t, 1, len(runtimeCtx.iTracker.instanceStack))

	runtimeCtx.popInstance()
	require.Equal(t, oldCodeSize, runtimeCtx.GetSCCodeSize())
	require.NotNil(t, runtimeCtx.iTracker.instance)
	require.Equal(t, instance, runtimeCtx.iTracker.instance)
	require.Equal(t, 0, len(runtimeCtx.iTracker.instanceStack))

	runtimeCtx.pushInstance()
	require.Equal(t, 1, len(runtimeCtx.iTracker.instanceStack))
}

func TestRuntimeContext_PushPopState(t *testing.T) {
	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	vmInput := vmcommon.VMInput{
		OriginalCallerAddr: []byte("caller"),
		CallerAddr:         []byte("caller"),
		GasProvided:        1000,
		KDATransfers:       make([]*vmcommon.KDATransfer, 0),
	}

	funcName := "test_func"
	scAddress := []byte("smartcontract")
	input := &vmcommon.ContractCallInput{
		VMInput:       vmInput,
		RecipientAddr: scAddress,
		Function:      funcName,
	}
	runtimeCtx.InitStateFromContractCallInput(input)

	runtimeCtx.iTracker.instance = &contextmock.InstanceMock{}
	runtimeCtx.PushState()
	require.Equal(t, 1, len(runtimeCtx.stateStack))

	// change state
	runtimeCtx.SetCodeAddress([]byte("dummy"))
	runtimeCtx.SetVMInput(nil)
	runtimeCtx.SetReadOnly(true)

	require.Equal(t, []byte("dummy"), runtimeCtx.codeAddress)
	require.Nil(t, runtimeCtx.GetVMInput())
	require.True(t, runtimeCtx.ReadOnly())

	runtimeCtx.PopSetActiveState()

	// check state was restored correctly
	require.Equal(t, scAddress, runtimeCtx.GetContextAddress())
	require.Equal(t, funcName, runtimeCtx.FunctionName())
	require.Equal(t, input, runtimeCtx.GetVMInput())
	require.False(t, runtimeCtx.ReadOnly())
	require.Nil(t, runtimeCtx.Arguments())

	runtimeCtx.iTracker.instance = &contextmock.InstanceMock{}
	runtimeCtx.PushState()
	require.Equal(t, 1, len(runtimeCtx.stateStack))

	runtimeCtx.iTracker.instance = &contextmock.InstanceMock{}
	runtimeCtx.PushState()
	require.Equal(t, 2, len(runtimeCtx.stateStack))

	runtimeCtx.PopDiscard()
	require.Equal(t, 1, len(runtimeCtx.stateStack))

	runtimeCtx.ClearStateStack()
	require.Equal(t, 0, len(runtimeCtx.stateStack))
}

func TestRuntimeContext_CountContractInstancesOnStack(t *testing.T) {
	alpha := []byte("alpha")
	beta := []byte("beta")
	gamma := []byte("gamma")

	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}

	testVMType := []byte("type")
	execFactory := testexecutor.NewDefaultTestExecutorFactory(t)
	exec, err := execFactory.CreateExecutor(executor.ExecutorFactoryArgs{
		VMHooks:          vmhooks.NewVMHooksImpl(host),
		ExecutionTimeout: time.Minute,
	})
	require.Nil(t, err)
	runtime, _ := NewRuntimeContext(
		host,
		testVMType,
		builtInFunctions.NewBuiltInFunctionContainer(),
		exec,
		defaultHasher,
	)

	vmInput := vmcommon.VMInput{
		CallerAddr:  []byte("caller"),
		GasProvided: 1000,
	}
	input := &vmcommon.ContractCallInput{
		VMInput:  vmInput,
		Function: "function",
	}

	input.RecipientAddr = alpha
	runtime.InitStateFromContractCallInput(input)
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.iTracker.instance = &contextmock.InstanceMock{}
	runtime.PushState()
	input.RecipientAddr = beta
	runtime.InitStateFromContractCallInput(input)
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.iTracker.instance = &contextmock.InstanceMock{}
	runtime.PushState()
	input.RecipientAddr = gamma
	runtime.InitStateFromContractCallInput(input)
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.iTracker.instance = &contextmock.InstanceMock{}
	runtime.PushState()
	input.RecipientAddr = alpha
	runtime.InitStateFromContractCallInput(input)
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.PushState()
	input.RecipientAddr = gamma
	runtime.InitStateFromContractCallInput(input)
	require.Equal(t, uint64(2), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.PopSetActiveState()
	runtime.PopSetActiveState()
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(gamma))

	runtime.PopDiscard()
	require.Equal(t, uint64(1), runtime.CountSameContractInstancesOnStack(alpha))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(beta))
	require.Equal(t, uint64(0), runtime.CountSameContractInstancesOnStack(gamma))
}

func TestRuntimeContext_Instance(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	gasPoints := uint64(100)
	runtimeCtx.SetPointsUsed(gasPoints)
	require.Equal(t, gasPoints, runtimeCtx.GetPointsUsed())

	funcName := "increment"
	input := &vmcommon.ContractCallInput{
		VMInput:       vmcommon.VMInput{},
		RecipientAddr: []byte("addr"),
		Function:      funcName,
	}
	runtimeCtx.InitStateFromContractCallInput(input)

	functionName, err := runtimeCtx.FunctionNameChecked()
	require.Nil(t, err)
	require.NotEmpty(t, functionName)

	input.Function = "func"
	runtimeCtx.InitStateFromContractCallInput(input)
	functionName, err = runtimeCtx.FunctionNameChecked()
	require.Equal(t, executor.ErrFuncNotFound, err)
	require.Empty(t, functionName)

	hasInitFunction := runtimeCtx.HasFunction(vmhost.InitFunctionName)
	require.True(t, hasInitFunction)

	runtimeCtx.ClearWarmInstanceCache()
	require.Nil(t, runtimeCtx.iTracker.instance)
}

func TestRuntimeContext_Breakpoints(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	mockOutput := &contextmock.OutputContextMock{
		OutputAccountMock: NewVMOutputAccount([]byte("address")),
	}
	mockOutput.OutputAccountMock.Code = []byte("code")
	mockOutput.SetReturnMessage("")

	host.OutputContext = mockOutput

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	// Set and get curent breakpoint value
	require.Equal(t, vmhost.BreakpointNone, runtimeCtx.GetRuntimeBreakpointValue())
	runtimeCtx.SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
	require.Equal(t, vmhost.BreakpointOutOfGas, runtimeCtx.GetRuntimeBreakpointValue())

	runtimeCtx.SetRuntimeBreakpointValue(vmhost.BreakpointNone)
	require.Equal(t, vmhost.BreakpointNone, runtimeCtx.GetRuntimeBreakpointValue())

	// Signal user error
	mockOutput.SetReturnCode(vmcommon.Ok)
	mockOutput.SetReturnMessage("")
	runtimeCtx.SetRuntimeBreakpointValue(vmhost.BreakpointNone)

	runtimeCtx.SignalUserError("something happened")
	require.Equal(t, vmhost.BreakpointSignalError, runtimeCtx.GetRuntimeBreakpointValue())
	require.Equal(t, vmcommon.VMUserError, mockOutput.ReturnCode())
	require.Equal(t, "something happened", mockOutput.ReturnMessage())

	// Fail execution
	mockOutput.SetReturnCode(vmcommon.Ok)
	mockOutput.SetReturnMessage("")
	runtimeCtx.SetRuntimeBreakpointValue(vmhost.BreakpointNone)

	runtimeCtx.FailExecution(nil)
	require.Equal(t, vmhost.BreakpointExecutionFailed, runtimeCtx.GetRuntimeBreakpointValue())
	require.Equal(t, vmcommon.VMExecutionFailed, mockOutput.ReturnCode())
	require.Equal(t, "execution failed", mockOutput.ReturnMessage())

	mockOutput.SetReturnCode(vmcommon.Ok)
	mockOutput.SetReturnMessage("")
	runtimeCtx.SetRuntimeBreakpointValue(vmhost.BreakpointNone)
	require.Equal(t, vmhost.BreakpointNone, runtimeCtx.GetRuntimeBreakpointValue())

	runtimeError := errors.New("runtime error")
	runtimeCtx.FailExecution(runtimeError)
	require.Equal(t, vmhost.BreakpointExecutionFailed, runtimeCtx.GetRuntimeBreakpointValue())
	require.Equal(t, vmcommon.VMExecutionFailed, mockOutput.ReturnCode())
	require.Equal(t, runtimeError.Error(), mockOutput.ReturnMessage())
}

func memLoad(runtimeCtx *runtimeContext, offset int32, length int32) ([]byte, error) {
	return runtimeCtx.GetInstance().MemLoad(executor.MemPtr(offset), length)
}

func memStore(runtimeCtx *runtimeContext, offset int32, data []byte) error {
	return runtimeCtx.GetInstance().MemStore(executor.MemPtr(offset), data)
}

func TestRuntimeContext_MemLoadStoreOk(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	memContents, err := memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, memContents)

	pageSize := uint32(65536)
	require.Equal(t, 2*pageSize, runtimeCtx.iTracker.instance.MemLength())

	memContents = []byte("test data")
	err = memStore(runtimeCtx, 10, memContents)
	require.Nil(t, err)
	require.Equal(t, 2*pageSize, runtimeCtx.iTracker.instance.MemLength())

	memContents, err = memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte{'t', 'e', 's', 't', ' ', 'd', 'a', 't', 'a', 0}, memContents)
}

func TestRuntimeContext_MemoryIsBlank(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := "./../../test/contracts/init-simple/output/init-simple.wasm"
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	totalPages := 2
	memoryContents := runtimeCtx.iTracker.instance.MemDump()
	require.Equal(t, runtimeCtx.iTracker.instance.MemLength(), uint32(len(memoryContents)))
	require.Equal(t, totalPages*int(vmhost.WASMPageSize), len(memoryContents))

	for i, value := range memoryContents {
		if value != byte(0) {
			msg := fmt.Sprintf("Non-zero value found at %d in Wasmer memory: 0x%X", i, value)
			require.Fail(t, msg)
		}
	}
}

func TestRuntimeContext_MemLoadCases(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	var offset int32
	var length int32
	// Offset too small
	offset = -3
	length = 10
	memContents, err := memLoad(runtimeCtx, offset, length)
	require.True(t, errors.Is(err, executor.ErrMemoryBadBounds))
	require.Nil(t, memContents)

	// Offset too larget
	offset = int32(runtimeCtx.iTracker.instance.MemLength() + 1)
	length = 10
	memContents, err = memLoad(runtimeCtx, offset, length)
	require.True(t, errors.Is(err, executor.ErrMemoryBadBounds))
	require.Nil(t, memContents)

	// Negative length
	offset = 10
	length = -2
	memContents, err = memLoad(runtimeCtx, offset, length)
	require.True(t, errors.Is(err, executor.ErrMemoryNegativeLength))
	require.Nil(t, memContents)

	// Requested end too large
	memContents = []byte("test data")
	offset = int32(runtimeCtx.iTracker.instance.MemLength() - 9)
	err = memStore(runtimeCtx, offset, memContents)
	require.Nil(t, err)

	offset = int32(runtimeCtx.iTracker.instance.MemLength() - 9)
	length = 9
	memContents, err = memLoad(runtimeCtx, offset, length)
	require.Nil(t, err)
	require.Equal(t, []byte("test data"), memContents)

	offset = int32(runtimeCtx.iTracker.instance.MemLength() - 8)
	length = 9
	memContents, err = memLoad(runtimeCtx, offset, length)
	require.Nil(t, err)
	require.Equal(t, []byte{'e', 's', 't', ' ', 'd', 'a', 't', 'a'}, memContents)

	// Zero length
	offset = int32(runtimeCtx.iTracker.instance.MemLength() - 8)
	length = 0
	memContents, err = memLoad(runtimeCtx, offset, length)
	require.Nil(t, err)
	require.Equal(t, []byte{}, memContents)
}

func TestRuntimeContext_MemStoreCases(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	require.Equal(t, 2*vmhost.WASMPageSize, runtimeCtx.iTracker.instance.MemLength())

	// Bad lower bounds
	memContents := []byte("test data")
	offset := int32(-2)
	err = memStore(runtimeCtx, offset, memContents)
	require.True(t, errors.Is(err, executor.ErrMemoryBadBounds))

	// Write something, then overwrite, then overwrite with empty byte slice
	memContents = []byte("this is a message")
	offset = int32(runtimeCtx.iTracker.instance.MemLength() - 100)
	err = memStore(runtimeCtx, offset, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, offset, 17)
	require.Nil(t, err)
	require.Equal(t, []byte("this is a message"), memContents)

	memContents = []byte("this is something")
	err = memStore(runtimeCtx, offset, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, offset, 17)
	require.Nil(t, err)
	require.Equal(t, []byte("this is something"), memContents)

	memContents = []byte{}
	err = memStore(runtimeCtx, offset, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, offset, 17)
	require.Nil(t, err)
	require.Equal(t, []byte("this is something"), memContents)
}

func TestRuntimeContext_MemStoreForbiddenGrowth(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(1)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	instance := runtimeCtx.iTracker.instance
	require.Equal(t, 2*vmhost.WASMPageSize, instance.MemLength())

	memContents := []byte("test data")

	// Memory growth via MemStore forbidden
	offset := int32(instance.MemLength() - 4)
	err = memStore(runtimeCtx, offset, memContents)
	require.True(t, errors.Is(err, executor.ErrMemoryBadBoundsUpper))
	require.Equal(t, 2*vmhost.WASMPageSize, instance.MemLength())

	// Memory growth via MemStore forbidden
	memContents = make([]byte, vmhost.WASMPageSize+100)
	offset = int32(instance.MemLength() - 50)
	err = memStore(runtimeCtx, offset, memContents)
	require.True(t, errors.Is(err, executor.ErrMemoryBadBoundsUpper))
	require.Equal(t, 2*vmhost.WASMPageSize, instance.MemLength())
}

func TestRuntimeContext_MemLoadStoreVsInstanceStack(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.SetMaxInstanceStackSize(2)

	gasLimit := uint64(100000000)
	path := counterWasmCode
	contractCode := vmhost.GetSCCode(path)
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	// Write "test data1" to the WASM memory of the current instance
	memContents := []byte("test data1")
	err = memStore(runtimeCtx, 10, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte("test data1"), memContents)

	// Push the current instance down the instance stack
	runtimeCtx.pushInstance()
	require.Equal(t, 1, len(runtimeCtx.iTracker.instanceStack))

	// Create a new Wasmer instance
	contractCode = vmhost.GetSCCode(path)
	err = runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	// Write "test data2" to the WASM memory of the new instance
	memContents = []byte("test data2")
	err = memStore(runtimeCtx, 10, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte("test data2"), memContents)

	// Pop the initial instance from the stack, making it the 'current instance'
	runtimeCtx.popInstance()
	require.Equal(t, 0, len(runtimeCtx.iTracker.instanceStack))

	// Check whether the previously-written string "test data1" is still in the
	// memory of the initial instance
	memContents, err = memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte("test data1"), memContents)

	// Write "test data3" to the WASM memory of the initial instance (now current)
	memContents = []byte("test data3")
	err = memStore(runtimeCtx, 10, memContents)
	require.Nil(t, err)

	memContents, err = memLoad(runtimeCtx, 10, 10)
	require.Nil(t, err)
	require.Equal(t, []byte("test data3"), memContents)
}

func TestRuntimeContext_PopSetActiveStateIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.PopSetActiveState()

	require.Equal(t, 0, len(runtimeCtx.stateStack))
}

func TestRuntimeContext_PopDiscardIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	runtimeCtx.PopDiscard()

	require.Equal(t, 0, len(runtimeCtx.stateStack))
}

func TestRuntimeContext_PopInstanceIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	host := InitializeVMAndWasmer()

	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.popInstance()

	require.Equal(t, 0, len(runtimeCtx.stateStack))
}

// recordingExecutor hands out mock instances and keeps a reference to each one,
// so a test can inspect an instance that the runtime context allocated and then
// dropped internally.
type recordingExecutor struct {
	*contextmock.ExecutorMock
	created []executor.Instance

	// failCache makes the handed out instances fail to serialize their module,
	// which is the only way to reach the early return in saveCompiledCode.
	failCache bool
}

// cacheFailingInstance is an instance whose module cannot be serialized. It stays
// perfectly usable otherwise, which mirrors the native contract: vm_exec_instance_cache
// takes a const instance pointer and leaves it untouched on failure.
type cacheFailingInstance struct {
	*contextmock.InstanceMock
}

func (instance *cacheFailingInstance) Cache() ([]byte, error) {
	return nil, errors.New("caching failed")
}

func (exec *recordingExecutor) record(code []byte) (executor.Instance, error) {
	var instance executor.Instance = contextmock.NewInstanceMock(code)
	if exec.failCache {
		instance = &cacheFailingInstance{InstanceMock: instance.(*contextmock.InstanceMock)}
	}

	exec.created = append(exec.created, instance)
	return instance, nil
}

func (exec *recordingExecutor) NewInstanceWithOptions(
	code []byte,
	_ executor.CompilationOptions,
) (executor.Instance, error) {
	return exec.record(code)
}

func (exec *recordingExecutor) NewInstanceFromCompiledCodeWithOptions(
	code []byte,
	_ executor.CompilationOptions,
) (executor.Instance, error) {
	return exec.record(code)
}

func TestRuntimeContext_StartWasmerInstanceCleansInstanceRejectedByTracker(t *testing.T) {
	contractCode := []byte("some contract code")
	gasLimit := uint64(100000000)

	testCases := []struct {
		name             string
		precompiledCache bool
	}{
		{name: "from bytecode", precompiledCache: false},
		{name: "from compiled code", precompiledCache: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			host := InitializeVMAndWasmer()
			runtimeCtx := makeDefaultRuntimeContext(t, host)
			defer runtimeCtx.ClearWarmInstanceCache()

			exec := &recordingExecutor{ExecutorMock: contextmock.NewExecutorMock(nil)}
			runtimeCtx.ReplaceVMExecutor(exec)
			runtimeCtx.SetMaxInstanceStackSize(1)

			if testCase.precompiledCache {
				host.Blockchain().SaveCompiledCode(defaultHasher.Compute(string(contractCode)), contractCode)
			}

			// Fill the tracker so that the instance about to be created is rejected.
			fillTracker(t, runtimeCtx.iTracker)
			numRunningBefore := runtimeCtx.iTracker.numRunningInstances

			err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
			require.ErrorIs(t, err, errTooManyInstances)

			// The instance was allocated before the tracker refused it, so the
			// caller has to free it; nothing else can reach it afterwards.
			require.Len(t, exec.created, 1)
			require.True(t, exec.created[0].IsAlreadyCleaned(),
				"instance rejected by the tracker was not cleaned, its native allocation leaks")

			// The rejection must not have touched the tracker.
			require.Len(t, runtimeCtx.iTracker.instances, warmCacheSize-2)
			require.Equal(t, numRunningBefore, runtimeCtx.iTracker.numRunningInstances)
		})
	}
}

func TestRuntimeContext_StartWasmerInstanceKeepsWarmInstanceRejectedByTracker(t *testing.T) {
	// Must be a decodable module that leaves the tables alone, otherwise the warm
	// instance is evicted before the tracker ever gets to refuse it.
	contractCode := moduleReadingTable()
	codeHash := defaultHasher.Compute(string(contractCode))
	gasLimit := uint64(100000000)

	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	exec := &recordingExecutor{ExecutorMock: contextmock.NewExecutorMock(nil)}
	runtimeCtx.ReplaceVMExecutor(exec)
	runtimeCtx.SetMaxInstanceStackSize(1)

	require.NoError(t, runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false))
	warmInstance, ok := runtimeCtx.iTracker.GetWarmInstance(codeHash)
	require.True(t, ok)

	// The warm cache outlives the transaction while the tracked instances map does
	// not, so the next transaction meets this instance as untracked.
	runtimeCtx.iTracker.InitState()
	fillTracker(t, runtimeCtx.iTracker)

	require.ErrorIs(t, runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false), errTooManyInstances)

	// Unlike the cold paths, a rejected warm instance must not be cleaned: being
	// refused by the tracker is not a transfer of ownership, the cache still holds it.
	require.False(t, warmInstance.IsAlreadyCleaned(),
		"rejected warm instance was cleaned while still owned by the warm cache")
	stillCached, ok := runtimeCtx.iTracker.GetWarmInstance(codeHash)
	require.True(t, ok)
	require.Same(t, warmInstance, stillCached)

	// Rejected on the warm path, before any cold fallback could allocate.
	require.Len(t, exec.created, 1)
}

func TestRuntimeContext_StartWasmerInstanceSavesWarmInstanceWhenCachingFails(t *testing.T) {
	contractCode := []byte("some contract code")
	codeHash := defaultHasher.Compute(string(contractCode))
	gasLimit := uint64(100000000)

	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()

	exec := &recordingExecutor{ExecutorMock: contextmock.NewExecutorMock(nil), failCache: true}
	runtimeCtx.ReplaceVMExecutor(exec)
	runtimeCtx.SetMaxInstanceStackSize(1)

	// Serializing the module fails, but that is not an execution error: the
	// instance is started and used as usual.
	err := runtimeCtx.StartWasmerInstance(contractCode, gasLimit, false)
	require.Nil(t, err)

	// Pins that the failing branch of saveCompiledCode was the one taken.
	found, _ := host.Blockchain().GetCompiledCode(codeHash)
	require.False(t, found)

	// Saving the warm instance is what gives the instance an owner. Skipping it
	// leaves an instance that the warm cache never took and that PopSetActiveState
	// will not clean either, since its codeHash is not on the stack.
	warmInstance, ok := runtimeCtx.iTracker.GetWarmInstance(codeHash)
	require.True(t, ok, "instance was not saved as warm, its native allocation leaks")
	require.Same(t, runtimeCtx.iTracker.Instance(), warmInstance)
}

func startFunctionMarker(t *testing.T, runtimeCtx *runtimeContext) byte {
	instance := runtimeCtx.iTracker.Instance()
	require.NotNil(t, instance)

	memory, err := instance.MemLoad(0, 1)
	require.Nil(t, err)

	return memory[0]
}

func TestRuntimeContext_StartWasmerInstanceRejectsStartSectionOnNewCode(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance(contextmock.WasmCodeWithStartSection(), 1000, true)

	require.ErrorIs(t, err, vmhost.ErrContractHasStartSection)
}

func TestRuntimeContext_StartWasmerInstanceAcceptsNewCodeWithoutStartSection(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance(contextmock.WasmCodeWithoutStartSection(), 1000, true)

	require.Nil(t, err)
	require.Equal(t, byte(0), startFunctionMarker(t, runtimeCtx))
}

func TestRuntimeContext_StartWasmerInstanceRejectsUndecodableCodeOnNewCode(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance([]byte("not a wasm module"), 1000, true)

	require.ErrorIs(t, err, vmhost.ErrContractCodeNotDecodable)
}

func TestRuntimeContext_StartWasmerInstanceAcceptsStartSectionBeforeAuditV4(t *testing.T) {
	host := InitializeVMAndWasmer()
	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV4Value = false
	host.ForkControllerContext = forkController

	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance(contextmock.WasmCodeWithStartSection(), 1000, true)

	require.Nil(t, err)
	require.Equal(t, contextmock.StartSectionMarker, startFunctionMarker(t, runtimeCtx))
}

func TestRuntimeContext_StartWasmerInstanceIgnoresStartSectionOnExistingCode(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	err := runtimeCtx.StartWasmerInstance(contextmock.WasmCodeWithStartSection(), 1000, false)

	require.Nil(t, err)
	require.Equal(t, contextmock.StartSectionMarker, startFunctionMarker(t, runtimeCtx))
}

func moduleWithTableFunction(body ...byte) []byte {
	module := []byte{
		0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x04, 0x04, 0x01, 0x70, 0x00, 0x01,
		0x05, 0x03, 0x01, 0x00, 0x01,
		0x07, 0x11, 0x02, 0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	}
	module = append(module, 0x04, 'm', 'a', 'i', 'n', 0x00, 0x00)

	function := append([]byte{0x00}, body...)
	module = append(module, 0x0A, byte(len(function)+2), 0x01, byte(len(function)))

	return append(module, function...)
}

func moduleMutatingTable() []byte {
	return moduleWithTableFunction(0x41, 0x00, 0xD0, 0x70, 0x26, 0x00, 0x0B)
}

func moduleMutatingTableWithPaddedOpcode() []byte {
	return moduleWithTableFunction(0xD0, 0x70, 0x41, 0x00, 0xFC, 0x8F, 0x00, 0x00, 0x1A, 0x0B)
}

func moduleReadingTable() []byte {
	return moduleWithTableFunction(0x41, 0x00, 0x25, 0x00, 0x1A, 0x0B)
}

func seedWarmInstance(t *testing.T, runtimeCtx *runtimeContext, contract []byte) []byte {
	codeHash := defaultHasher.Compute(string(contract))
	runtimeCtx.iTracker.SetCodeHash(codeHash)
	require.Nil(t, runtimeCtx.iTracker.SetNewInstance(&contextmock.InstanceMock{}, Precompiled))
	runtimeCtx.iTracker.SaveAsWarmInstance()
	require.True(t, runtimeCtx.iTracker.warmInstanceCache.Has(codeHash))

	return codeHash
}

func TestRuntimeContext_WarmInstanceRefusedForTableMutatingModule(t *testing.T) {
	for name, contract := range map[string][]byte{
		"canonical opcode": moduleMutatingTable(),
		"padded opcode":    moduleMutatingTableWithPaddedOpcode(),
	} {
		t.Run(name, func(t *testing.T) {
			host := InitializeVMAndWasmer()
			runtimeCtx := makeDefaultRuntimeContext(t, host)
			defer runtimeCtx.ClearWarmInstanceCache()
			runtimeCtx.SetMaxInstanceStackSize(1)

			codeHash := seedWarmInstance(t, runtimeCtx, contract)

			reused, err := runtimeCtx.useWarmInstanceIfExists(1000, false, wasmbytes.MutatesTables(contract))

			require.Nil(t, err)
			require.False(t, reused)
			require.False(t, runtimeCtx.iTracker.warmInstanceCache.Has(codeHash))
		})
	}
}

func TestRuntimeContext_WarmInstanceReusedForTableReadingModule(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	contract := moduleReadingTable()
	codeHash := seedWarmInstance(t, runtimeCtx, contract)

	reused, err := runtimeCtx.useWarmInstanceIfExists(1000, false, wasmbytes.MutatesTables(contract))

	require.Nil(t, err)
	require.True(t, reused)
	require.True(t, runtimeCtx.iTracker.warmInstanceCache.Has(codeHash))
}

func TestRuntimeContext_TableMutatingModuleLeavesNoLeakedInstance(t *testing.T) {
	host := InitializeVMAndWasmer()
	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	require.Nil(t, runtimeCtx.StartWasmerInstance(moduleMutatingTable(), 1000, false))
	runtimeCtx.EndExecution()

	require.Nil(t, runtimeCtx.ValidateInstances())
}

func TestRuntimeContext_StartWasmerInstanceRefusesTableMutatingModuleBeforeAuditV4(t *testing.T) {
	host := InitializeVMAndWasmer()
	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV4Value = false
	host.ForkControllerContext = forkController

	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	contract := moduleMutatingTable()
	codeHash := seedWarmInstance(t, runtimeCtx, contract)
	seeded, found := runtimeCtx.iTracker.GetWarmInstance(codeHash)
	require.True(t, found)

	require.Nil(t, runtimeCtx.StartWasmerInstance(contract, 1000, false))

	require.NotEqual(t, Warm, runtimeCtx.iTracker.cacheLevel)
	require.NotEqual(t, seeded.ID(), runtimeCtx.iTracker.Instance().ID())
}

func TestRuntimeContext_StartWasmerInstanceRefusesWarmInstanceForTableMutatingModule(t *testing.T) {
	host := InitializeVMAndWasmer()
	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV4Value = true
	host.ForkControllerContext = forkController

	runtimeCtx := makeDefaultRuntimeContext(t, host)
	defer runtimeCtx.ClearWarmInstanceCache()
	runtimeCtx.SetMaxInstanceStackSize(1)

	contract := moduleMutatingTable()
	codeHash := seedWarmInstance(t, runtimeCtx, contract)
	seeded, found := runtimeCtx.iTracker.GetWarmInstance(codeHash)
	require.True(t, found)

	require.Nil(t, runtimeCtx.StartWasmerInstance(contract, 1000, false))

	require.NotEqual(t, Warm, runtimeCtx.iTracker.cacheLevel)
	require.NotEqual(t, seeded.ID(), runtimeCtx.iTracker.Instance().ID())
}
