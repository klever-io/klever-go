package smartContract

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingVM holds the first call until release is closed; later calls return at once
type blockingVM struct {
	started  chan struct{}
	release  chan struct{}
	numCalls int32
}

func newBlockingVM() *blockingVM {
	return &blockingVM{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (b *blockingVM) handler() *mock.VMExecutionHandlerStub {
	return &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			if atomic.AddInt32(&b.numCalls, 1) == 1 {
				close(b.started)
				<-b.release
			}
			return &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}, nil
		},
	}
}

func createQueryServiceWithVM(t *testing.T, vm vmcommon.VMExecutionHandler, maxQueryWait time.Duration) *SCQueryService {
	args := createMockArgumentsForSCQuery()
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return vm, nil
		},
	}
	args.EconomicsFee = &commonMock.FeeHandlerStub{MaxGasLimitPerBlockValue: uint64(math.MaxUint64)}
	args.MaxQueryWait = maxQueryWait

	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	return service
}

func newTestQuery() *process.SCQuery {
	return &process.SCQuery{
		ScAddress: []byte(DummyScAddress),
		FuncName:  "function",
		Arguments: [][]byte{},
	}
}

func TestNewSCQueryService_MaxQueryWaitDefaultsWhenUnset(t *testing.T) {
	t.Parallel()

	service := createQueryServiceWithVM(t, &mock.VMExecutionHandlerStub{}, 0)
	assert.Equal(t, DefaultMaxQueryWait, service.maxQueryWait)

	service = createQueryServiceWithVM(t, &mock.VMExecutionHandlerStub{}, 250*time.Millisecond)
	assert.Equal(t, 250*time.Millisecond, service.maxQueryWait)
}

func TestSCQueryService_ExecuteQuery_RefusedAsBusyWithinBoundWhileVMIsHeld(t *testing.T) {
	t.Parallel()

	const maxQueryWait = 50 * time.Millisecond
	vm := newBlockingVM()
	service := createQueryServiceWithVM(t, vm.handler(), maxQueryWait)

	heavyDone := make(chan error, 1)
	go func() {
		_, err := service.ExecuteQuery(newTestQuery())
		heavyDone <- err
	}()
	<-vm.started

	// several queued queries, as from many clients: each one is refused within the bound
	const numWaiting = 8
	var wg sync.WaitGroup
	errs := make([]error, numWaiting)
	elapsed := make([]time.Duration, numWaiting)
	for i := 0; i < numWaiting; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			_, errs[idx] = service.ExecuteQuery(newTestQuery())
			elapsed[idx] = time.Since(start)
		}(i)
	}
	wg.Wait()

	for i := 0; i < numWaiting; i++ {
		assert.True(t, errors.Is(errs[i], process.ErrQueryServiceBusy), "query %d: %v", i, errs[i])
		assert.GreaterOrEqual(t, elapsed[i], maxQueryWait)
		assert.Less(t, elapsed[i], maxQueryWait+time.Second, "query %d waited past the bound", i)
	}

	close(vm.release)
	require.NoError(t, <-heavyDone)

	// once the VM is free, a query runs normally
	output, err := service.ExecuteQuery(newTestQuery())
	require.NoError(t, err)
	assert.Equal(t, vmcommon.Ok, output.ReturnCode)
}

func TestSCQueryService_ExecuteQuery_WaitingQueryRunsWhenVMFreesWithinBound(t *testing.T) {
	t.Parallel()

	vm := newBlockingVM()
	service := createQueryServiceWithVM(t, vm.handler(), time.Minute)

	heavyDone := make(chan error, 1)
	go func() {
		_, err := service.ExecuteQuery(newTestQuery())
		heavyDone <- err
	}()
	<-vm.started

	lightDone := make(chan error, 1)
	go func() {
		_, err := service.ExecuteQuery(newTestQuery())
		lightDone <- err
	}()

	select {
	case err := <-lightDone:
		t.Fatalf("query ran while the VM was held: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(vm.release)
	require.NoError(t, <-heavyDone)
	require.NoError(t, <-lightDone)
	assert.Equal(t, int32(2), atomic.LoadInt32(&vm.numCalls))
}

// recordingLocker is a RWMutex that counts the readers holding it
type recordingLocker struct {
	sync.RWMutex
	readers int32
}

func (l *recordingLocker) RLock() {
	l.RWMutex.RLock()
	atomic.AddInt32(&l.readers, 1)
}

func (l *recordingLocker) RUnlock() {
	atomic.AddInt32(&l.readers, -1)
	l.RWMutex.RUnlock()
}

func TestSCQueryService_ExecuteQuery_RunsOnceUnderVMChangeLock(t *testing.T) {
	t.Parallel()

	locker := &recordingLocker{}
	numCalls := int32(0)
	heldOnCall := make([]bool, 0, 1)
	vm := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			atomic.AddInt32(&numCalls, 1)
			heldOnCall = append(heldOnCall, atomic.LoadInt32(&locker.readers) == 1)
			// a contract can return any message, this one included; it is not retried
			return &vmcommon.VMOutput{ReturnCode: vmcommon.VMUserError, ReturnMessage: "allocation error"}, nil
		},
	}

	args := createMockArgumentsForSCQuery()
	args.VmContainer = &mock.VMContainerMock{
		GetCalled: func(_ []byte) (vmcommon.VMExecutionHandler, error) {
			return vm, nil
		},
	}
	args.EconomicsFee = &commonMock.FeeHandlerStub{MaxGasLimitPerBlockValue: uint64(math.MaxUint64)}
	args.WasmVMChangeLocker = locker
	service, err := NewSCQueryService(args)
	require.NoError(t, err)

	output, err := service.ExecuteQuery(newTestQuery())
	require.NoError(t, err)
	assert.Equal(t, vmcommon.VMUserError, output.ReturnCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&numCalls))
	assert.Equal(t, []bool{true}, heldOnCall)
	assert.Equal(t, int32(0), atomic.LoadInt32(&locker.readers))
}

func TestSCQueryService_ExecuteQuery_VMErrorIsReturned(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("vm failure")
	vm := &mock.VMExecutionHandlerStub{
		RunSmartContractCallCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			return nil, expectedErr
		},
	}
	service := createQueryServiceWithVM(t, vm, 0)

	output, err := service.ExecuteQuery(newTestQuery())
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, expectedErr))
}
