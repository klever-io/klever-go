package mock

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/eventNotifier"
)

// EpochStartNotifierStub -
type EpochStartNotifierStub struct {
	RegisterHandlerCalled            func(handler eventNotifier.ActionHandler)
	UnregisterHandlerCalled          func(handler eventNotifier.ActionHandler)
	NotifyAllPrepareCalled           func(blk data.HeaderHandler)
	NotifyAllCalled                  func(blk data.HeaderHandler)
	NotifyEpochChangeConfirmedCalled func(epoch uint32)
}

// NotifyEpochChangeConfirmed -
func (esnm *EpochStartNotifierStub) NotifyEpochChangeConfirmed(epoch uint32) {
	if esnm.NotifyEpochChangeConfirmedCalled != nil {
		esnm.NotifyEpochChangeConfirmedCalled(epoch)
	}
}

// RegisterHandler -
func (esnm *EpochStartNotifierStub) RegisterHandler(handler eventNotifier.ActionHandler) {
	if esnm.RegisterHandlerCalled != nil {
		esnm.RegisterHandlerCalled(handler)
	}
}

// UnregisterHandler -
func (esnm *EpochStartNotifierStub) UnregisterHandler(handler eventNotifier.ActionHandler) {
	if esnm.UnregisterHandlerCalled != nil {
		esnm.UnregisterHandlerCalled(handler)
	}
}

// NotifyAllPrepare -
func (esnm *EpochStartNotifierStub) NotifyAllPrepare(blk data.HeaderHandler) {
	if esnm.NotifyAllPrepareCalled != nil {
		esnm.NotifyAllPrepareCalled(blk)
	}
}

// NotifyAll -
func (esnm *EpochStartNotifierStub) NotifyAll(blk data.HeaderHandler) {
	if esnm.NotifyAllCalled != nil {
		esnm.NotifyAllCalled(blk)
	}
}

// IsInterfaceNil -
func (esnm *EpochStartNotifierStub) IsInterfaceNil() bool {
	return esnm == nil
}
