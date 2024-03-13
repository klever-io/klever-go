package disabled

import (
	"github.com/klever-io/klever-go/data"
)

type epochStartTrigger struct {
}

// NewEpochStartTrigger returns a new instance of epochStartTrigger
func NewEpochStartTrigger() *epochStartTrigger {
	return &epochStartTrigger{}
}

// Update -
func (e *epochStartTrigger) Update(_ uint64, _ uint64) {
}

// ReceivedHeader -
func (e *epochStartTrigger) ReceivedHeader(_ data.HeaderHandler) {
}

// IsEpochStart -
func (e *epochStartTrigger) IsEpochStart() bool {
	return false
}

// EpochStartHdrHash -
func (e *epochStartTrigger) EpochStartHdrHash() []byte {
	return nil
}

// SetCurrentEpochStartSlot -
func (e *epochStartTrigger) SetCurrentEpochStartSlot(slot uint64) {
}

// Epoch -
func (e *epochStartTrigger) Epoch() uint32 {
	return 0
}

// EpochStartSlot -
func (e *epochStartTrigger) EpochStartSlot() uint64 {
	return 0
}

// EpochStartSlot -
func (e *epochStartTrigger) PrevEpochStartSlot() uint64 {
	return 0
}

// SetProcessed -
func (e *epochStartTrigger) SetProcessed(_ data.HeaderHandler) {
}

// RevertStateToBlock -
func (e *epochStartTrigger) RevertStateToBlock(_ data.HeaderHandler) error {
	return nil
}

// EpochStartMetaHdrHash -
func (e *epochStartTrigger) EpochStartMetaHdrHash() []byte {
	return nil
}

// GetSavedStateKey -
func (e *epochStartTrigger) GetSavedStateKey() []byte {
	return nil
}

// LoadState -
func (e *epochStartTrigger) LoadState(_ []byte) error {
	return nil
}

// SetFinalityAttestingSlot -
func (e *epochStartTrigger) SetFinalityAttestingSlot(_ uint64) {
}

// EpochFinalityAttestingSlot -
func (e *epochStartTrigger) EpochFinalityAttestingSlot() uint64 {
	return 0
}

// RequestEpochStartIfNeeded -
func (e *epochStartTrigger) RequestEpochStartIfNeeded(_ data.HeaderHandler) {
}

// IsInterfaceNil -
func (e *epochStartTrigger) IsInterfaceNil() bool {
	return e == nil
}
