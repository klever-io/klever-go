package mock

import (
	"github.com/klever-io/klever-go/core/consensus"
)

// ChronologyHandlerMock -
type ChronologyHandlerMock struct {
	AddSubslotCalled        func(consensus.SubslotHandler)
	RemoveAllSubslotsCalled func()
	StartSlotCalled         func()
	EpochCalled             func() uint32
}

// Epoch -
func (chrm *ChronologyHandlerMock) Epoch() uint32 {
	if chrm.EpochCalled != nil {
		return chrm.EpochCalled()
	}
	return 0
}

// AddSubslot -
func (chrm *ChronologyHandlerMock) AddSubslot(subslotHandler consensus.SubslotHandler) {
	if chrm.AddSubslotCalled != nil {
		chrm.AddSubslotCalled(subslotHandler)
	}
}

// RemoveAllSubslots -
func (chrm *ChronologyHandlerMock) RemoveAllSubslots() {
	if chrm.RemoveAllSubslotsCalled != nil {
		chrm.RemoveAllSubslotsCalled()
	}
}

// StartSlots -
func (chrm *ChronologyHandlerMock) StartSlots() {
	if chrm.StartSlotCalled != nil {
		chrm.StartSlotCalled()
	}
}

// Close -
func (chrm *ChronologyHandlerMock) Close() error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (chrm *ChronologyHandlerMock) IsInterfaceNil() bool {
	return chrm == nil
}
