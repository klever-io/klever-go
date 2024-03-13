package smartContract_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/smartContract"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
)

func TestNewScQueryServiceDispatcher_NilEmptyListShouldErr(t *testing.T) {
	t.Parallel()

	sqsd, err := smartContract.NewScQueryServiceDispatcher(nil)
	assert.True(t, check.IfNil(sqsd))
	assert.True(t, errors.Is(err, process.ErrNilOrEmptyList))

	sqsd, err = smartContract.NewScQueryServiceDispatcher(make([]process.SCQueryService, 0))
	assert.True(t, check.IfNil(sqsd))
	assert.True(t, errors.Is(err, process.ErrNilOrEmptyList))
}

func TestNewScQueryServiceDispatcher_OneElementIsNilShouldErr(t *testing.T) {
	t.Parallel()

	sqsd, err := smartContract.NewScQueryServiceDispatcher([]process.SCQueryService{
		&ScQueryStub{},
		nil,
		&ScQueryStub{},
	})
	assert.True(t, check.IfNil(sqsd))
	assert.True(t, errors.Is(err, process.ErrNilScQueryElement))
}

func TestNewScQueryServiceDispatcher_ShouldWork(t *testing.T) {
	t.Parallel()

	sqsd, err := smartContract.NewScQueryServiceDispatcher([]process.SCQueryService{
		&ScQueryStub{},
		&ScQueryStub{},
	})
	assert.False(t, check.IfNil(sqsd))
	assert.Nil(t, err)
	assert.Equal(t, 2, len(sqsd.GetList()))
}

func TestScQueryServiceDispatcher_ExecuteQueryShouldCallInRoundRobinFashion(t *testing.T) {
	t.Parallel()

	calledElement1 := 0
	calledElement2 := 0
	sqsd, _ := smartContract.NewScQueryServiceDispatcher([]process.SCQueryService{
		&ScQueryStub{
			ExecuteQueryCalled: func(query *process.SCQuery) (*vmcommon.VMOutput, error) {
				calledElement1++

				return nil, nil
			},
		},
		&ScQueryStub{
			ExecuteQueryCalled: func(query *process.SCQuery) (*vmcommon.VMOutput, error) {
				calledElement2++

				return nil, nil
			},
		},
	})

	_, _ = sqsd.ExecuteQuery(nil)
	_, _ = sqsd.ExecuteQuery(nil)
	_, _ = sqsd.ExecuteQuery(nil)

	assert.Equal(t, 2, calledElement1)
	assert.Equal(t, 1, calledElement2)
}

func TestScQueryServiceDispatcher_ShouldWorkInAConcurrentManner(t *testing.T) {
	t.Parallel()

	calledElement1 := uint32(0)
	calledElement2 := uint32(0)
	sqsd, _ := smartContract.NewScQueryServiceDispatcher([]process.SCQueryService{
		&ScQueryStub{
			ExecuteQueryCalled: func(query *process.SCQuery) (*vmcommon.VMOutput, error) {
				atomic.AddUint32(&calledElement1, 1)

				return nil, nil
			},
		},
		&ScQueryStub{
			ExecuteQueryCalled: func(query *process.SCQuery) (*vmcommon.VMOutput, error) {
				atomic.AddUint32(&calledElement2, 1)

				return nil, nil
			},
		},
	})

	numCalls := 100
	split := numCalls / 2
	wg := &sync.WaitGroup{}
	wg.Add(numCalls)
	for i := 0; i < numCalls; i++ {
		go func() {
			_, _ = sqsd.ExecuteQuery(nil)
			wg.Done()
		}()
	}

	wg.Wait()

	assert.Equal(t, uint32(split), atomic.LoadUint32(&calledElement1))
	assert.Equal(t, uint32(split), atomic.LoadUint32(&calledElement2))
}

func TestNewScQueryServiceDispatcher_CloseShouldWork(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")
	closeCalled1 := false
	closeCalled2 := false
	sqsd, _ := smartContract.NewScQueryServiceDispatcher([]process.SCQueryService{
		&ScQueryStub{
			CloseCalled: func() error {
				closeCalled1 = true
				return expectedErr
			},
		},
		&ScQueryStub{
			CloseCalled: func() error {
				closeCalled2 = true
				return nil
			},
		},
	})

	err := sqsd.Close()
	assert.Equal(t, expectedErr, err)
	assert.True(t, closeCalled1)
	assert.True(t, closeCalled2)
}
