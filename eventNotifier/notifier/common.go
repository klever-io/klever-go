package notifier

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/eventNotifier"
)

var _ eventNotifier.ActionHandler = (*handlerStruct)(nil)

// handlerStruct represents a struct which satisfies the SubscribeFunctionHandler interface
type handlerStruct struct {
	act     func(blk data.HeaderHandler)
	prepare func(blk data.HeaderHandler)
	id      uint32
}

// NewHandlerForEpochStart will return a struct which will satisfy the above interface
func NewHandlerForEpochStart(
	actionFunc func(blk data.HeaderHandler),
	prepareFunc func(blk data.HeaderHandler),
	id uint32,
) eventNotifier.ActionHandler {
	handler := handlerStruct{
		act:     actionFunc,
		prepare: prepareFunc,
		id:      id,
	}

	return &handler
}

// EpochStartPrepare will notify the subscriber to prepare for a start of epoch.
// The event can be triggered multiple times
func (hs *handlerStruct) EpochStartPrepare(blk data.HeaderHandler) {
	if hs.act != nil {
		hs.prepare(blk)
	}
}

// EpochStartAction will notify the subscribed function if not nil
func (hs *handlerStruct) EpochStartAction(blk data.HeaderHandler) {
	if hs.act != nil {
		hs.act(blk)
	}
}

// NotifyOrder returns the notification order for a start of epoch event
func (hs *handlerStruct) NotifyOrder() uint32 {
	return hs.id
}
