package mock

// ShuffledOutHandlerStub -
type ShuffledOutHandlerStub struct {
	ProcessCalled         func() error
	RegisterHandlerCalled func(handler func())
}

// Process -
func (s *ShuffledOutHandlerStub) Process() error {
	if s.ProcessCalled != nil {
		return s.ProcessCalled()
	}

	return nil
}

// RegisterHandler -
func (s *ShuffledOutHandlerStub) RegisterHandler(handler func()) {
	if s.RegisterHandlerCalled != nil {
		s.RegisterHandlerCalled(handler)
	}
}

// IsInterfaceNil -
func (s *ShuffledOutHandlerStub) IsInterfaceNil() bool {
	return s == nil
}
