package notifier_test

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/stretchr/testify/assert"
)

func TestNewEpochStartSubscriptionHandler(t *testing.T) {
	t.Parallel()

	essh := notifier.NewEpochStartSubscriptionHandler()
	assert.NotNil(t, essh)
	assert.False(t, essh.IsInterfaceNil())
}

func TestEpochStartSubscriptionHandler_RegisterHandlerNilHandlerShouldNotAdd(t *testing.T) {
	t.Parallel()

	essh := notifier.NewEpochStartSubscriptionHandler()
	essh.RegisterHandler(nil)

	handlers, mutHandlers := essh.RegisteredHandlers()
	mutHandlers.RLock()
	assert.Equal(t, 0, len(handlers))
	mutHandlers.RUnlock()
}

func TestEpochStartSubscriptionHandler_RegisterHandlerOkHandlerShouldAdd(t *testing.T) {
	t.Parallel()

	essh := notifier.NewEpochStartSubscriptionHandler()
	handler := notifier.NewHandlerForEpochStart(func(blk data.HeaderHandler) {}, nil, 0)

	essh.RegisterHandler(handler)

	handlers, mutHandlers := essh.RegisteredHandlers()
	mutHandlers.RLock()
	assert.Equal(t, 1, len(handlers))
	mutHandlers.RUnlock()
}

func TestEpochStartSubscriptionHandler_UnregisterHandlerNilHandlerShouldDoNothing(t *testing.T) {
	t.Parallel()

	essh := notifier.NewEpochStartSubscriptionHandler()

	// first register a handler
	handler := notifier.NewHandlerForEpochStart(func(blk data.HeaderHandler) {}, nil, 0)
	essh.RegisterHandler(handler)

	// then try to unregister but a nil handler is given
	essh.UnregisterHandler(nil)
	handlers, mutHandlers := essh.RegisteredHandlers()
	mutHandlers.RLock()
	// length of the slice should still be 1
	assert.Equal(t, 1, len(handlers))
	mutHandlers.RUnlock()
}

func TestEpochStartSubscriptionHandler_UnregisterHandlerOklHandlerShouldRemove(t *testing.T) {
	t.Parallel()

	essh := notifier.NewEpochStartSubscriptionHandler()

	// first register a handler
	handler := notifier.NewHandlerForEpochStart(func(blk data.HeaderHandler) {}, nil, 0)
	essh.RegisterHandler(handler)

	// then unregister the same handler
	essh.UnregisterHandler(handler)
	handlers, mutHandlers := essh.RegisteredHandlers()
	mutHandlers.RLock()
	// length of the slice should be 0 because the handler was unregistered
	assert.Equal(t, 0, len(handlers))
	mutHandlers.RUnlock()
}

func TestEpochStartSubscriptionHandler_NotifyAll(t *testing.T) {
	t.Parallel()

	firstHandlerWasCalled := false
	secondHandlerWasCalled := false
	lastCalled := 0
	essh := notifier.NewEpochStartSubscriptionHandler()

	// register 2 handlers
	handler1 := notifier.NewHandlerForEpochStart(func(blk data.HeaderHandler) {
		firstHandlerWasCalled = true
		lastCalled = 1
	}, nil, 1)
	handler2 := notifier.NewHandlerForEpochStart(func(blk data.HeaderHandler) {
		secondHandlerWasCalled = true
		lastCalled = 2
	}, nil, 2)

	essh.RegisterHandler(handler1)
	essh.RegisterHandler(handler2)

	// make sure that the handler were not called yet
	assert.False(t, firstHandlerWasCalled)
	assert.False(t, secondHandlerWasCalled)

	// now we call the NotifyAll method and all handlers should be called
	essh.NotifyAll(&block.Block{Header: &block.BlockHeader{}})
	assert.True(t, firstHandlerWasCalled)
	assert.True(t, secondHandlerWasCalled)
	assert.Equal(t, lastCalled, 2)
}

func TestEpochStartSubscriptionHandler_NotifyAllPrepare(t *testing.T) {
	t.Parallel()

	firstHandlerWasCalled := false
	secondHandlerWasCalled := false
	lastCalled := 0
	essh := notifier.NewEpochStartSubscriptionHandler()

	// prepare handlers are the second argument; NotifyOrder must drive the
	// call order exactly as it does for NotifyAll
	handler1 := notifier.NewHandlerForEpochStart(nil, func(blk data.HeaderHandler) {
		firstHandlerWasCalled = true
		lastCalled = 1
	}, 1)
	handler2 := notifier.NewHandlerForEpochStart(nil, func(blk data.HeaderHandler) {
		secondHandlerWasCalled = true
		lastCalled = 2
	}, 2)

	essh.RegisterHandler(handler2)
	essh.RegisterHandler(handler1)

	essh.NotifyAllPrepare(&block.Block{Header: &block.BlockHeader{}})
	assert.True(t, firstHandlerWasCalled)
	assert.True(t, secondHandlerWasCalled)
	assert.Equal(t, 2, lastCalled)
}

func TestEpochStartSubscriptionHandler_ConcurrentNotifyIsRaceFree(t *testing.T) {
	t.Parallel()

	// NotifyAll and NotifyAllPrepare sort the shared handler slice; under a
	// read lock two concurrent notifications race on the sort's swaps, which
	// the race detector catches. This test pins the write-lock fix.
	essh := notifier.NewEpochStartSubscriptionHandler()
	for i := 3; i >= 1; i-- {
		order := uint32(i)
		essh.RegisterHandler(notifier.NewHandlerForEpochStart(
			func(blk data.HeaderHandler) {}, func(blk data.HeaderHandler) {}, order))
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			blk := &block.Block{Header: &block.BlockHeader{}}
			if n%2 == 0 {
				essh.NotifyAll(blk)
			} else {
				essh.NotifyAllPrepare(blk)
			}
		}(i)
	}
	close(start)
	wg.Wait()
}
