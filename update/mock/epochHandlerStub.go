package mock

// EpochHandlerStub -
type EpochHandlerStub struct {
	EpochCalled           func() uint32
	ForceEpochStartCalled func(slot uint64)
}

// MetaEpoch -
func (ehs *EpochHandlerStub) Epoch() uint32 {
	if ehs.EpochCalled != nil {
		return ehs.EpochCalled()
	}

	return uint32(0)
}

// ForceEpochStart -
func (ehs *EpochHandlerStub) ForceEpochStart(slot uint64) {
	if ehs.ForceEpochStartCalled != nil {
		ehs.ForceEpochStartCalled(slot)
	}
}

// IsInterfaceNil -
func (ehs *EpochHandlerStub) IsInterfaceNil() bool {
	return ehs == nil
}
