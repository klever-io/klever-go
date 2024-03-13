package mock

import (
	"github.com/klever-io/klever-go/data"
)

// EpochStartTriggerStub -
type EpochStartTriggerStub struct {
	currentEpochStartSlot            uint64
	EpochCalled                      func() uint32
	EpochFinalityAttestingSlotCalled func() uint64
	EpochStartHdrHashCalled          func() []byte
	EpochStartSlotCalled             func() uint64
	PrevEpochStartSlotCalled         func() uint64
	GetSavedStateKeyCalled           func() []byte
	IsEpochStartCalled               func() bool
	LoadStateCalled                  func(key []byte) error
	RequestEpochStartIfNeededCalled  func(interceptedHeader data.HeaderHandler)
	RevertStateToBlockCalled         func(interceptedHeader data.HeaderHandler) error
	SetFinalityAttestingSlotCalled   func(slot uint64)
	SetProcessedCalled               func(interceptedHeader data.HeaderHandler)
	UpdateCalled                     func(slot uint64, nonce uint64)
}

// Epoch -
func (e *EpochStartTriggerStub) Epoch() uint32 {
	if e.EpochCalled != nil {
		return e.EpochCalled()
	}
	return 0
}

// EpochFinalityAttestingSlot -
func (e *EpochStartTriggerStub) EpochFinalityAttestingSlot() uint64 {
	if e.EpochFinalityAttestingSlotCalled != nil {
		return e.EpochFinalityAttestingSlotCalled()
	}
	return 0
}

// EpochStartSlot -
func (e *EpochStartTriggerStub) EpochStartSlot() uint64 {
	if e.EpochStartSlotCalled != nil {
		return e.EpochStartSlotCalled()
	}
	return e.currentEpochStartSlot
}

// EpochStartSlot -
func (e *EpochStartTriggerStub) SetCurrentEpochStartSlot(value uint64) {
	e.currentEpochStartSlot = value
}

// PrevEpochStartSlot -
func (e *EpochStartTriggerStub) PrevEpochStartSlot() uint64 {
	if e.PrevEpochStartSlotCalled != nil {
		return e.PrevEpochStartSlotCalled()
	}
	return 0
}

// EpochStartHash -
func (e *EpochStartTriggerStub) EpochStartHdrHash() []byte {
	return e.EpochStartHdrHashCalled()
}

// GetSavedStateKey -
func (e *EpochStartTriggerStub) GetSavedStateKey() []byte {
	if e.GetSavedStateKeyCalled != nil {
		return e.GetSavedStateKeyCalled()
	}

	return []byte("epoch start trigger key")
}

// IsEpochStart --
func (e *EpochStartTriggerStub) IsEpochStart() bool {
	if e.IsEpochStartCalled != nil {
		return e.IsEpochStartCalled()
	}

	return false
}

// LoadState -
func (e *EpochStartTriggerStub) LoadState(key []byte) error {
	return e.LoadStateCalled(key)
}

// RequestEpochStartIfNeeded -
func (e *EpochStartTriggerStub) RequestEpochStartIfNeeded(interceptedHeader data.HeaderHandler) {
	if e.RequestEpochStartIfNeededCalled != nil {
		e.RequestEpochStartIfNeededCalled(interceptedHeader)
	}
}

// RevertStateToBlock -
func (e *EpochStartTriggerStub) RevertStateToBlock(interceptedHeader data.HeaderHandler) error {
	if e.RevertStateToBlockCalled != nil {
		return e.RevertStateToBlockCalled(interceptedHeader)
	}

	return nil
}

// SetFinalityAttestingSlot -
func (e *EpochStartTriggerStub) SetFinalityAttestingSlot(slot uint64) {
	if e.SetFinalityAttestingSlotCalled != nil {
		e.SetFinalityAttestingSlotCalled(slot)
	}
}

// SetProcessed -
func (e *EpochStartTriggerStub) SetProcessed(interceptedHeader data.HeaderHandler) {
	if e.SetProcessedCalled != nil {
		e.SetProcessedCalled(interceptedHeader)
	}
}

// Update -
func (e *EpochStartTriggerStub) Update(slot uint64, nonce uint64) {
	if e.UpdateCalled != nil {
		e.UpdateCalled(slot, nonce)
	}
}

// IsInterfaceNil -
func (e *EpochStartTriggerStub) IsInterfaceNil() bool {
	return e == nil
}
