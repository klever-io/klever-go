package mock

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/eventNotifier"
)

// EpochStartNotifierStub -
type EpochStartNotifierStub struct {
	RegisterHandlerCalled   func(handler eventNotifier.ActionHandler)
	UnregisterHandlerCalled func(handler eventNotifier.ActionHandler)
	NotifyAllCalled         func(hdr data.HeaderHandler)
	NotifyAllPrepareCalled  func(hdr data.HeaderHandler)
	epochStartHdls          []eventNotifier.ActionHandler
}

// RegisterHandler -
func (esnm *EpochStartNotifierStub) RegisterHandler(handler eventNotifier.ActionHandler) {
	if esnm.RegisterHandlerCalled != nil {
		esnm.RegisterHandlerCalled(handler)
	}

	esnm.epochStartHdls = append(esnm.epochStartHdls, handler)
}

// UnregisterHandler -
func (esnm *EpochStartNotifierStub) UnregisterHandler(handler eventNotifier.ActionHandler) {
	if esnm.UnregisterHandlerCalled != nil {
		esnm.UnregisterHandlerCalled(handler)
	}

	for i, hdl := range esnm.epochStartHdls {
		if hdl == handler {
			esnm.epochStartHdls = append(esnm.epochStartHdls[:i], esnm.epochStartHdls[i+1:]...)
			break
		}
	}
}

// NotifyAllPrepare -
func (esnm *EpochStartNotifierStub) NotifyAllPrepare(metaHdr data.HeaderHandler) {
	if esnm.NotifyAllPrepareCalled != nil {
		esnm.NotifyAllPrepareCalled(metaHdr)
	}

	for _, hdl := range esnm.epochStartHdls {
		hdl.EpochStartPrepare(metaHdr)
	}
}

// NotifyAll -
func (esnm *EpochStartNotifierStub) NotifyAll(hdr data.HeaderHandler) {
	if esnm.NotifyAllCalled != nil {
		esnm.NotifyAllCalled(hdr)
	}

	for _, hdl := range esnm.epochStartHdls {
		hdl.EpochStartAction(hdr)
	}
}

// IsInterfaceNil -
func (esnm *EpochStartNotifierStub) IsInterfaceNil() bool {
	return esnm == nil
}
