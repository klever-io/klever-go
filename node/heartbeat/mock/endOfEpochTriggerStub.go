package mock

// EpochStartTriggerStub -
type EpochStartTriggerStub struct {
	EpochCalled func() uint32
}

// Epoch -
func (e *EpochStartTriggerStub) Epoch() uint32 {
	if e.EpochCalled != nil {
		return e.EpochCalled()
	}
	return 0
}

// IsInterfaceNil -
func (e *EpochStartTriggerStub) IsInterfaceNil() bool {
	return e == nil
}
