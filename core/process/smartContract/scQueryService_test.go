package smartContract

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const DummyScAddress = "00000000000000000500fabd9501b7e5353de57a4e319857c2fb99089770720a"

// GetClosedUnbufferedChannel returns an instance of a 'chan struct{}' that is already closed
func GetClosedUnbufferedChannel() chan struct{} {
	ch := make(chan struct{})
	close(ch)

	return ch
}

func createTestBlockchain() *commonMock.BlockChainMock {
	return &commonMock.BlockChainMock{
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &commonMock.HeaderHandlerStub{
				GetNonceCalled: func() uint64 {
					return 0
				},
				GetSlotCalled: func() uint64 {
					return 0
				},
				GetProducerPublicKeyCalled: func() []byte {
					return []byte("signature")
				},
			}
		},
	}
}

func createMockArgumentsForSCQuery() ArgsNewSCQueryService {
	return ArgsNewSCQueryService{
		VmContainer:              &mock.VMContainerMock{},
		EconomicsFee:             &commonMock.FeeHandlerStub{},
		BlockChainHook:           &mock.BlockchainHookStub{},
		BlockChain:               createTestBlockchain(),
		WasmVMChangeLocker:       &sync.RWMutex{},
		AllowExternalQueriesChan: GetClosedUnbufferedChannel(),
		MaxGasLimitPerQuery:      1_500_000_000,
		GetNodeState: func() core.NodeState {
			return 0
		},
	}
}

func TestNewSCQueryService_NilVmShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.VmContainer = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNoVM, err)
}

func TestNewSCQueryService_NilFeeHandlerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.EconomicsFee = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNilEconomicsFeeHandler, err)
}

func TestNewSCQueryService_NilBLockChainShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.BlockChain = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNilBlockChain, err)
}

func TestNewSCQueryService_NilBLockChainHookShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.BlockChainHook = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNilBlockChainHook, err)
}

func TestNewSCQueryService_NilWasmVMLockerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.WasmVMChangeLocker = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNilLocker, err)
}

func TestNewSCQueryService_NilAllowExternalQueriesChanShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.AllowExternalQueriesChan = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	assert.Equal(t, process.ErrNilAllowExternalQueriesChan, err)
}

func TestNewSCQueryService_ShouldWork(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	target, err := NewSCQueryService(args)

	assert.NotNil(t, target)
	assert.Nil(t, err)
	assert.False(t, target.IsInterfaceNil())
}

func TestExecuteQuery_GetNilAddressShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	target, _ := NewSCQueryService(args)

	query := process.SCQuery{
		ScAddress: nil,
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	output, err := target.ExecuteQuery(&query)

	assert.Nil(t, output)
	assert.Equal(t, process.ErrNilScAddress, err)
}

func TestExecuteQuery_EmptyFunctionShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	target, _ := NewSCQueryService(args)

	query := process.SCQuery{
		ScAddress: []byte{0},
		FuncName:  "",
		Arguments: [][]byte{},
	}

	output, err := target.ExecuteQuery(&query)

	assert.Nil(t, output)
	assert.Equal(t, process.ErrEmptyFunctionName, err)
}

func TestExecuteQuery_ShouldPerformActionsInRegardsToAllowanceChannel(t *testing.T) {
	t.Parallel()

	chanAllowedQueries := make(chan struct{})
	args := createMockArgumentsForSCQuery()
	args.AllowExternalQueriesChan = chanAllowedQueries
	target, _ := NewSCQueryService(args)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "func",
		Arguments: [][]byte{},
	}

	output, err := target.ExecuteQuery(&query)
	assert.Equal(t, process.ErrQueriesNotAllowedYet, err)
	assert.Nil(t, output)

	close(chanAllowedQueries)
	_, err = target.ExecuteQuery(&query)
	assert.NoError(t, err)
}

func TestExecuteQuery_AllowanceChannelShouldWorkUnderConcurrentRequests(t *testing.T) {
	t.Parallel()

	chanAllowedQueries := make(chan struct{})
	args := createMockArgumentsForSCQuery()
	args.AllowExternalQueriesChan = chanAllowedQueries
	target, _ := NewSCQueryService(args)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "func",
		Arguments: [][]byte{},
	}

	defer func() {
		r := recover()
		assert.Nil(t, r)
	}()

	numTries := 200
	wg := sync.WaitGroup{}
	wg.Add(numTries)

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(chanAllowedQueries)
	}()

	for i := 0; i < numTries; i++ {
		go func(idx int) {
			select {
			case <-chanAllowedQueries:
				_, err := target.ExecuteQuery(&query)
				assert.NoError(t, err)
			default:
				output, err := target.ExecuteQuery(&query)
				assert.Equal(t, process.ErrQueriesNotAllowedYet, err)
				assert.Nil(t, output)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func TestExecuteQuery_ShouldReceiveQueryCorrectly(t *testing.T) {
	t.Parallel()

	funcName := "function"
	scAddress := []byte(DummyScAddress)
	args := []*big.Int{big.NewInt(42), big.NewInt(43)}
	runWasCalled := false

	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			runWasCalled = true
			assert.Equal(t, int64(42), big.NewInt(0).SetBytes(input.Arguments[0]).Int64())
			assert.Equal(t, int64(43), big.NewInt(0).SetBytes(input.Arguments[1]).Int64())
			assert.Equal(t, scAddress, input.CallerAddr)
			assert.Equal(t, funcName, input.Function)

			return &vmcommon.VMOutput{
				ReturnCode: vmcommon.Ok,
			}, nil
		},
	}
	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}

	target, _ := NewSCQueryService(argsNewSCQuery)

	dataArgs := make([][]byte, len(args))
	for i, arg := range args {
		dataArgs[i] = append(dataArgs[i], arg.Bytes()...)
	}
	query := process.SCQuery{
		ScAddress: scAddress,
		FuncName:  funcName,
		Arguments: dataArgs,
	}

	_, _ = target.ExecuteQuery(&query)
	assert.True(t, runWasCalled)
}

func TestExecuteQuery_ReturnsCorrectly(t *testing.T) {
	t.Parallel()

	d := [][]byte{[]byte("90"), []byte("91")}

	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			return &vmcommon.VMOutput{
				ReturnCode: vmcommon.Ok,
				ReturnData: d,
			}, nil
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	vmOutput, err := target.ExecuteQuery(&query)

	assert.Nil(t, err)
	assert.Equal(t, d[0], vmOutput.ReturnData[0])
	assert.Equal(t, d[1], vmOutput.ReturnData[1])
}

func TestExecuteQuery_GasProvidedShouldBeApplied(t *testing.T) {
	t.Parallel()

	t.Run("no gas defined, should use max int64", func(t *testing.T) {
		t.Parallel()

		runSCWasCalled := false
		mockVM := &mock.VMExecutionHandlerStub{
			RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
				require.Equal(t, uint64(math.MaxInt64), input.GasProvided)
				runSCWasCalled = true
				return &vmcommon.VMOutput{}, nil
			},
		}
		argsNewSCQuery := createMockArgumentsForSCQuery()
		argsNewSCQuery.VmContainer = &mock.VMContainerMock{
			GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
				return mockVM, nil
			},
		}
		argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
			MaxGasLimitPerBlockValue: uint64(math.MaxInt64),
		}

		argsNewSCQuery.MaxGasLimitPerQuery = 0 // no gas defined

		target, _ := NewSCQueryService(argsNewSCQuery)

		query := process.SCQuery{
			ScAddress: []byte(DummyScAddress),
			FuncName:  "function",
			Arguments: [][]byte{},
		}

		_, err := target.ExecuteQuery(&query)
		require.Nil(t, err)
		require.True(t, runSCWasCalled)
	})

	t.Run("custom gas defined, should use it", func(t *testing.T) {
		t.Parallel()

		maxGasLimit := uint64(1_500_000_000)
		runSCWasCalled := false
		mockVM := &mock.VMExecutionHandlerStub{
			RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
				require.Equal(t, maxGasLimit, input.GasProvided)
				runSCWasCalled = true
				return &vmcommon.VMOutput{}, nil
			},
		}
		argsNewSCQuery := createMockArgumentsForSCQuery()
		argsNewSCQuery.VmContainer = &mock.VMContainerMock{
			GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
				return mockVM, nil
			},
		}
		argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
			MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
		}

		argsNewSCQuery.MaxGasLimitPerQuery = maxGasLimit

		target, _ := NewSCQueryService(argsNewSCQuery)

		query := process.SCQuery{
			ScAddress: []byte(DummyScAddress),
			FuncName:  "function",
			Arguments: [][]byte{},
		}

		_, err := target.ExecuteQuery(&query)
		require.Nil(t, err)
		require.True(t, runSCWasCalled)
	})
}

func TestExecuteQuery_WhenNotOkCodeShouldNotErr(t *testing.T) {
	t.Parallel()

	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			return &vmcommon.VMOutput{
				ReturnCode:    vmcommon.VMOutOfGas,
				ReturnMessage: "add more gas",
			}, nil
		},
	}
	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	returnedData, err := target.ExecuteQuery(&query)

	assert.Nil(t, err)
	assert.NotNil(t, returnedData)
	assert.Contains(t, returnedData.ReturnMessage, "add more gas")
}

func TestExecuteQuery_ShouldCallRunScSequentially(t *testing.T) {
	t.Parallel()

	running := int32(0)

	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			atomic.AddInt32(&running, 1)
			time.Sleep(time.Millisecond)

			val := atomic.LoadInt32(&running)
			assert.Equal(t, int32(1), val)

			atomic.AddInt32(&running, -1)

			return &vmcommon.VMOutput{
				ReturnCode: vmcommon.Ok,
			}, nil
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}
	target, _ := NewSCQueryService(argsNewSCQuery)

	noOfGoRoutines := 50
	wg := sync.WaitGroup{}
	wg.Add(noOfGoRoutines)
	for i := 0; i < noOfGoRoutines; i++ {
		go func() {
			query := process.SCQuery{
				ScAddress: []byte(DummyScAddress),
				FuncName:  "function",
				Arguments: [][]byte{},
			}

			_, _ = target.ExecuteQuery(&query)
			wg.Done()
		}()
	}

	wg.Wait()
}

func TestSCQueryService_ExecuteQueryShouldNotIncludeCallerAddressAndValue(t *testing.T) {
	t.Parallel()

	callerAddressAndCallValueAreNotSet := false
	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			if len(input.KDATransfers) == 0 && bytes.Equal(input.CallerAddr, input.RecipientAddr) {
				callerAddressAndCallValueAreNotSet = true
			}
			return &vmcommon.VMOutput{
				ReturnCode: vmcommon.Ok,
				ReturnData: [][]byte{[]byte("ok")},
			}, nil
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}
	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	_, err := target.ExecuteQuery(&query)
	require.NoError(t, err)
	require.True(t, callerAddressAndCallValueAreNotSet)
}

func TestSCQueryService_ExecuteQueryShouldIncludeCallerAddressAndValue(t *testing.T) {
	t.Parallel()

	expectedCallerAddr := []byte("caller addr")
	expectedValue := big.NewInt(37)
	callerAddressAndCallValueAreSet := false
	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			if bytes.Equal(input.CallerAddr, expectedCallerAddr) &&
				len(input.KDATransfers) == 1 &&
				input.KDATransfers[0].KDAValue.Cmp(expectedValue) == 0 {
				callerAddressAndCallValueAreSet = true
			}
			return &vmcommon.VMOutput{
				ReturnCode: vmcommon.Ok,
				ReturnData: [][]byte{[]byte("ok")},
			}, nil
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}
	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress:  []byte(DummyScAddress),
		FuncName:   "function",
		CallerAddr: expectedCallerAddr,
		CallValue:  map[string]int64{"KLV": expectedValue.Int64()},
		Arguments:  [][]byte{},
	}

	_, err := target.ExecuteQuery(&query)
	require.NoError(t, err)
	require.True(t, callerAddressAndCallValueAreSet)
}

func TestSCQueryService_ShouldFailIfStateChanged(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()

	rootHashCalled := false
	args.BlockChain.(*commonMock.BlockChainMock).GetCurrentBlockRootHashCalled = func() []byte {
		if !rootHashCalled {
			rootHashCalled = true
			return []byte("first root hash")
		}
		return []byte("second root hash")
	}

	qs, _ := NewSCQueryService(args)

	res, err := qs.ExecuteQuery(&process.SCQuery{
		SameScState: true,
		ScAddress:   []byte(DummyScAddress),
		FuncName:    "function",
	})
	require.Nil(t, res)
	require.True(t, errors.Is(err, process.ErrStateChangedWhileExecutingVmQuery))
}

func TestSCQueryService_ShouldWorkIfStateDidntChange(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()

	args.BlockChain.(*commonMock.BlockChainMock).GetCurrentBlockHeaderCalled = func() data.HeaderHandler {
		return &block.Block{
			Header: &block.BlockHeader{
				TrieRoot: []byte("same root hash"),
			},
		}
	}

	qs, _ := NewSCQueryService(args)

	res, err := qs.ExecuteQuery(&process.SCQuery{
		SameScState: true,
		ScAddress:   []byte(DummyScAddress),
		FuncName:    "function",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestNewSCQueryService_CloseShouldWork(t *testing.T) {
	t.Parallel()

	closeCalled := false
	argsNewSCQueryService := ArgsNewSCQueryService{
		VmContainer: &mock.VMContainerMock{
			CloseCalled: func() error {
				closeCalled = true
				return nil
			},
		},
		EconomicsFee:             &commonMock.FeeHandlerStub{},
		BlockChainHook:           &mock.BlockchainHookStub{},
		BlockChain:               createTestBlockchain(),
		WasmVMChangeLocker:       &sync.RWMutex{},
		AllowExternalQueriesChan: GetClosedUnbufferedChannel(),
		GetNodeState: func() core.NodeState {
			return 0
		},
	}

	target, err := NewSCQueryService(argsNewSCQueryService)
	require.NoError(t, err)
	err = target.Close()
	assert.Nil(t, err)
	assert.True(t, closeCalled)
}

func TestSCQueryService_ExecuteQueryShouldReturnErrorIfVMReturnsError(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("some error")
	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			return nil, expectedError
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	_, err := target.ExecuteQuery(&query)

	assert.Equal(t, expectedError, err)
}

func TestSCQueryService_ExecuteQueryShouldReturnErrorIfVMReturnsErrorWithMessage(t *testing.T) {
	t.Parallel()

	mockVM := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
			return &vmcommon.VMOutput{
				ReturnCode:    vmcommon.VMUserError,
				ReturnMessage: "allocation error",
			}, nil
		},
	}

	argsNewSCQuery := createMockArgumentsForSCQuery()
	argsNewSCQuery.VmContainer = &mock.VMContainerMock{
		GetCalled: func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
			return mockVM, nil
		},
	}
	argsNewSCQuery.EconomicsFee = &commonMock.FeeHandlerStub{
		MaxGasLimitPerBlockValue: uint64(math.MaxUint64),
	}

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}

	_, err := target.ExecuteQuery(&query)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "allocation error")
}

func TestSCQueryService_ExecuteQueryShouldReturnErrorInvalidCallValue(t *testing.T) {
	t.Parallel()

	funcName := "function"
	scAddress := []byte(DummyScAddress)

	argsNewSCQuery := createMockArgumentsForSCQuery()

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: scAddress,
		FuncName:  funcName,
		CallValue: map[string]int64{"KLV1": 10}, // Invalid Token
	}

	_, err := target.ExecuteQuery(&query)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid ticker name", err.Error())
}

func TestSCQueryService_ExecuteQueryShouldReturnErrorInvalidSCAddress(t *testing.T) {
	t.Parallel()

	funcName := "function"
	scAddress := []byte("00")

	argsNewSCQuery := createMockArgumentsForSCQuery()

	target, _ := NewSCQueryService(argsNewSCQuery)

	query := process.SCQuery{
		ScAddress: scAddress,
		FuncName:  funcName,
		Arguments: [][]byte{},
	}

	_, err := target.ExecuteQuery(&query)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid VM type", err.Error())
}

func TestNewSCQueryService_NilBootstrapperShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.GetNodeState = nil
	target, err := NewSCQueryService(args)

	assert.Nil(t, target)
	require.NotNil(t, err)
	assert.Equal(t, process.ErrNilBootstrapper, err)
}

func TestSCQueryService_ShouldFailIfNodeIsNotSynced(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.GetNodeState = func() core.NodeState {
		return core.NsNotSynchronized
	}

	qs, _ := NewSCQueryService(args)

	res, err := qs.ExecuteQuery(&process.SCQuery{
		ShouldBeSynced: true,
		ScAddress:      []byte(DummyScAddress),
		FuncName:       "function",
	})
	require.Nil(t, res)
	require.Equal(t, process.ErrNodeIsNotSynced, err)
}

func TestSCQueryService_ShouldWorkIfNodeIsSynced(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.GetNodeState = func() core.NodeState {
		return core.NsSynchronized
	}

	qs, _ := NewSCQueryService(args)

	res, err := qs.ExecuteQuery(&process.SCQuery{
		ShouldBeSynced: true,
		ScAddress:      []byte(DummyScAddress),
		FuncName:       "function",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
}

/////////////////////////////////////////
// Per-query KApp context isolation    //
/////////////////////////////////////////

// queryServiceOverController builds an SCQueryService whose hook exposes the given
// controller, recording every context the service installs on it.
func queryServiceOverController(t *testing.T, readOnly bool) (*SCQueryService, *[]kapp.KappContext) {
	t.Helper()

	installed := make([]kapp.KappContext, 0)
	controller := &kvmStub.KAppControllerStub{
		IsReadOnlyCalled: func() bool { return readOnly },
		SetCurrentKAppContextCalled: func(ctx kapp.KappContext) {
			installed = append(installed, ctx)
		},
	}

	args := createMockArgumentsForSCQuery()
	args.BlockChainHook = &mock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController { return controller },
	}
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return &mock.VMExecutionHandlerStub{
				RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
					return &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}, nil
				},
			}, nil
		},
	}

	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	return service, &installed
}

func dummyQuery() *process.SCQuery {
	return &process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}
}

// A read-only (query) controller must get a FRESH KApp context per query. It is
// built once per query element at boot and lives for the whole process, and its
// receipt slice is append-only, so reusing one leaks receipts for the node's
// entire uptime and makes every built-in dispatch copy more data.
func TestSCQueryService_ExecuteQuery_ReadOnly_ResetsKAppContextPerQuery(t *testing.T) {
	t.Parallel()

	service, installed := queryServiceOverController(t, true)

	for range 3 {
		_, err := service.ExecuteQuery(dummyQuery())
		require.NoError(t, err)
	}

	require.Len(t, *installed, 3, "each query must install its own KApp context")

	for i, ctx := range *installed {
		require.NotNil(t, ctx)
		require.Empty(t, ctx.Receipts().Get(), "a freshly installed context must carry no receipts")
		if i > 0 {
			require.NotSame(t, (*installed)[i-1], ctx, "contexts must not be reused across queries")
		}
	}
}

// A writable controller must be left alone: clobbering the live context of a
// service wired to the production controller would discard in-flight receipts.
func TestSCQueryService_ExecuteQuery_Writable_LeavesKAppContextAlone(t *testing.T) {
	t.Parallel()

	service, installed := queryServiceOverController(t, false)

	_, err := service.ExecuteQuery(dummyQuery())
	require.NoError(t, err)

	require.Empty(t, *installed, "a writable controller must keep the context it already has")
}

// A nil controller must not panic the query path.
func TestSCQueryService_ExecuteQuery_NilController_NoPanic(t *testing.T) {
	t.Parallel()

	args := createMockArgumentsForSCQuery()
	args.BlockChainHook = &mock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController { return nil },
	}
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return &mock.VMExecutionHandlerStub{
				RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
					return &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}, nil
				},
			}, nil
		},
	}

	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, err = service.ExecuteQuery(dummyQuery())
		require.NoError(t, err)
	})
}

// Order matters: the context must be replaced BEFORE the VM runs, otherwise the
// query executes against the previous query's accumulated receipts and only gets a
// clean context after the fact.
func TestSCQueryService_ExecuteQuery_ReadOnly_ResetsContextBeforeRunningTheVM(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 2)
	controller := &kvmStub.KAppControllerStub{
		IsReadOnlyCalled: func() bool { return true },
		SetCurrentKAppContextCalled: func(_ kapp.KappContext) {
			calls = append(calls, "setContext")
		},
	}

	args := createMockArgumentsForSCQuery()
	args.BlockChainHook = &mock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController { return controller },
	}
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return &mock.VMExecutionHandlerStub{
				RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
					calls = append(calls, "runContract")
					return &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}, nil
				},
			}, nil
		},
	}

	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	_, err = service.ExecuteQuery(dummyQuery())
	require.NoError(t, err)

	require.Equal(t, []string{"setContext", "runContract"}, calls)
}

// A failing query must still have installed a fresh context: the reset is what keeps
// the long-lived query controller from accumulating receipts, so it cannot be
// conditional on the VM succeeding.
func TestSCQueryService_ExecuteQuery_ReadOnly_ResetsContextEvenWhenTheVMFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("vm exploded")

	installed := make([]kapp.KappContext, 0)
	controller := &kvmStub.KAppControllerStub{
		IsReadOnlyCalled: func() bool { return true },
		SetCurrentKAppContextCalled: func(ctx kapp.KappContext) {
			installed = append(installed, ctx)
		},
	}

	args := createMockArgumentsForSCQuery()
	args.BlockChainHook = &mock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController { return controller },
	}
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return &mock.VMExecutionHandlerStub{
				RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
					return nil, expectedErr
				},
			}, nil
		},
	}

	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	_, err = service.ExecuteQuery(dummyQuery())
	require.ErrorIs(t, err, expectedErr)

	require.Len(t, installed, 1, "the context must be reset regardless of the query outcome")
	require.NotNil(t, installed[0])
}
