package eventNotifier

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
)

// ActionHandler defines the action taken on epoch start event
type ActionHandler interface {
	EpochStartAction(blk data.HeaderHandler)
	EpochStartPrepare(blk data.HeaderHandler)
	NotifyOrder() uint32
}

// EpochStartEventNotifier provides Register and Unregister functionality for the end of epoch events
type EpochStartEventNotifier interface {
	RegisterHandler(handler ActionHandler)
	UnregisterHandler(handler ActionHandler)
	IsInterfaceNil() bool
}

// RequestHandler defines the methods through which request to data can be made
type RequestHandler interface {
	RequestHeader(hash []byte)
	RequestHeaderByNonce(nonce uint64)
	RequestStartOfEpochBlock(epoch uint32)
	RequestInterval() time.Duration
	SetNumPeersToQuery(key string, intra int, cross int) error
	GetNumPeersToQuery(key string) (int, int, error)
	IsInterfaceNil() bool
}

// RegistrationHandler provides Register and Unregister functionality for the end of epoch events
type RegistrationHandler interface {
	RegisterHandler(handler ActionHandler)
	UnregisterHandler(handler ActionHandler)
	IsInterfaceNil() bool
}

// TriggerHandler defines the functionalities for an start of epoch trigger
type TriggerHandler interface {
	Close() error
	ForceEpochStart(slot uint64)
	IsEpochStart() bool
	Epoch() uint32
	Update(slot uint64, nonce uint64)
	EpochStartSlot() uint64
	PrevEpochStartSlot() uint64
	EpochStartHdrHash() []byte
	GetSavedStateKey() []byte
	LoadState(key []byte) error
	SetProcessed(header data.HeaderHandler)
	SetFinalityAttestingSlot(slot uint64)
	EpochFinalityAttestingSlot() uint64
	RevertStateToBlock(header data.HeaderHandler) error
	SetCurrentEpochStartSlot(slot uint64)
	RequestEpochStartIfNeeded(interceptedHeader data.HeaderHandler)
	SetAppStatusHandler(handler core.AppStatusHandler) error
	IsInterfaceNil() bool
}

// Notifier defines which actions should be done for handling new epoch's events
type Notifier interface {
	NotifyAll(hdr data.HeaderHandler)
	NotifyAllPrepare(metaHdr data.HeaderHandler)
	NotifyEpochChangeConfirmed(epoch uint32)
	IsInterfaceNil() bool
}
