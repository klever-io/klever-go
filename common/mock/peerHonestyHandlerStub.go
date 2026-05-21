package mock

// PeerHonestyHandlerStub -
type PeerHonestyHandlerStub struct {
	ChangeScoreCalled func(pk string, topic string, units int)
	CloseCalled      func() error
}

// ChangeScore -
func (phhs *PeerHonestyHandlerStub) ChangeScore(pk string, topic string, units int) {
	if phhs.ChangeScoreCalled != nil {
		phhs.ChangeScoreCalled(pk, topic, units)
	}
}

// Close -
func (phhs *PeerHonestyHandlerStub) Close() error {
	if phhs.CloseCalled != nil {
		return phhs.CloseCalled()
	}
	return nil
}

// IsInterfaceNil -
func (phhs *PeerHonestyHandlerStub) IsInterfaceNil() bool {
	return phhs == nil
}
