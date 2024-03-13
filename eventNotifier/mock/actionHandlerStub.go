package mock

import (
	"github.com/klever-io/klever-go/data"
)

// ActionHandlerStub -
type ActionHandlerStub struct {
	EpochStartActionCalled  func(blk data.HeaderHandler)
	EpochStartPrepareCalled func(blk data.HeaderHandler)
	NotifyOrderCalled       func() uint32
}

// EpochStartAction -
func (ahs *ActionHandlerStub) EpochStartAction(blk data.HeaderHandler) {
	if ahs.EpochStartActionCalled != nil {
		ahs.EpochStartActionCalled(blk)
	}
}

// EpochStartPrepare -
func (ahs *ActionHandlerStub) EpochStartPrepare(blk data.HeaderHandler) {
	if ahs.EpochStartPrepareCalled != nil {
		ahs.EpochStartPrepareCalled(blk)
	}
}

// NotifyOrder -
func (ahs *ActionHandlerStub) NotifyOrder() uint32 {
	if ahs.NotifyOrderCalled != nil {
		return ahs.NotifyOrderCalled()
	}

	return 0
}
