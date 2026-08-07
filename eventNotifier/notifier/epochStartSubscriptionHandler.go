package notifier

import (
	"sort"
	"sync"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/eventNotifier"
)

// EpochStartNotifier defines which actions should be done for handling new epoch's events
type EpochStartNotifier interface {
	RegisterHandler(handler eventNotifier.ActionHandler)
	UnregisterHandler(handler eventNotifier.ActionHandler)
	NotifyAll(blk data.HeaderHandler)
	NotifyAllPrepare(blk data.HeaderHandler)
	NotifyEpochChangeConfirmed(epoch uint32)
	RegisterForEpochChangeConfirmed(handler func(epoch uint32))
	IsInterfaceNil() bool
}

var _ EpochStartNotifier = (*epochStartSubscriptionHandler)(nil)

// epochStartSubscriptionHandler will handle subscription of function and notifying them
type epochStartSubscriptionHandler struct {
	epochStartHandlers    []eventNotifier.ActionHandler
	epochFinalizedHandler []func(epoch uint32)
	mutEpochStartHandler  sync.RWMutex
}

// NewEpochStartSubscriptionHandler returns a new instance of epochStartSubscriptionHandler
func NewEpochStartSubscriptionHandler() *epochStartSubscriptionHandler {
	return &epochStartSubscriptionHandler{
		epochStartHandlers:    make([]eventNotifier.ActionHandler, 0),
		epochFinalizedHandler: make([]func(epoch uint32), 0),
		mutEpochStartHandler:  sync.RWMutex{},
	}
}

// RegisterHandler will subscribe a function so it will be called when NotifyAll method is called
func (essh *epochStartSubscriptionHandler) RegisterHandler(handler eventNotifier.ActionHandler) {
	if handler != nil {
		essh.mutEpochStartHandler.Lock()
		essh.epochStartHandlers = append(essh.epochStartHandlers, handler)
		essh.mutEpochStartHandler.Unlock()
	}
}

// UnregisterHandler will unsubscribe a function from the slice
func (essh *epochStartSubscriptionHandler) UnregisterHandler(handlerToUnregister eventNotifier.ActionHandler) {
	if handlerToUnregister != nil {
		essh.mutEpochStartHandler.Lock()
		for idx, handler := range essh.epochStartHandlers {
			if handler == handlerToUnregister {
				essh.epochStartHandlers = append(essh.epochStartHandlers[:idx], essh.epochStartHandlers[idx+1:]...)
			}
		}
		essh.mutEpochStartHandler.Unlock()
	}
}

// NotifyAll will call all the subscribed functions from the internal slice
func (essh *epochStartSubscriptionHandler) NotifyAll(blk data.HeaderHandler) {
	// the sort mutates the shared slice, so a write lock is required; a read
	// lock would let two concurrent notifications race on the sort's swaps
	essh.mutEpochStartHandler.Lock()

	sort.Slice(essh.epochStartHandlers, func(i, j int) bool {
		return essh.epochStartHandlers[i].NotifyOrder() < essh.epochStartHandlers[j].NotifyOrder()
	})

	for i := 0; i < len(essh.epochStartHandlers); i++ {
		essh.epochStartHandlers[i].EpochStartAction(blk)
	}
	essh.mutEpochStartHandler.Unlock()
}

// NotifyAllPrepare will call all the subscribed clients to notify them that an epoch change block has been
// observed, but not yet confirmed/committed. Some components may need to do some initialisation/preparation
func (essh *epochStartSubscriptionHandler) NotifyAllPrepare(blk data.HeaderHandler) {
	// write lock for the same reason as NotifyAll: the sort mutates the slice
	essh.mutEpochStartHandler.Lock()
	sort.Slice(essh.epochStartHandlers, func(i, j int) bool {
		return essh.epochStartHandlers[i].NotifyOrder() < essh.epochStartHandlers[j].NotifyOrder()
	})

	for i := 0; i < len(essh.epochStartHandlers); i++ {
		essh.epochStartHandlers[i].EpochStartPrepare(blk)
	}
	essh.mutEpochStartHandler.Unlock()
}

// RegisterForEpochChangeConfirmed will register the handler function to be called when epoch change is confirmed
func (essh *epochStartSubscriptionHandler) RegisterForEpochChangeConfirmed(handler func(epoch uint32)) {
	if handler == nil {
		return
	}

	essh.mutEpochStartHandler.Lock()
	essh.epochFinalizedHandler = append(essh.epochFinalizedHandler, handler)
	essh.mutEpochStartHandler.Unlock()
}

// NotifyEpochChangeConfirmed will call all the subscribed clients to notify them that an epoch change is confirmed
func (essh *epochStartSubscriptionHandler) NotifyEpochChangeConfirmed(epoch uint32) {
	essh.mutEpochStartHandler.RLock()
	for _, handler := range essh.epochFinalizedHandler {
		go handler(epoch)
	}
	essh.mutEpochStartHandler.RUnlock()
}

// IsInterfaceNil -
func (essh *epochStartSubscriptionHandler) IsInterfaceNil() bool {
	return essh == nil
}
