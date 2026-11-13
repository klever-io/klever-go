package mock

// ThrottlerStub -
type ThrottlerStub struct {
	CanProcessCalled      func() bool
	StartProcessingCalled func()
	EndProcessingCalled   func()
	StartWasCalled        bool
	EndWasCalled          bool
}

// CanProcess -
func (ts *ThrottlerStub) CanProcess() bool {
	if ts.CanProcessCalled != nil {
		return ts.CanProcessCalled()
	}

	return true
}

// StartProcessing -
func (ts *ThrottlerStub) StartProcessing() {
	ts.StartWasCalled = true
	if ts.StartProcessingCalled != nil {
		ts.StartProcessingCalled()
	}
}

// TryStartProcessing -
func (ts *ThrottlerStub) TryStartProcessing() bool {
	if !ts.CanProcess() {
		return false
	}
	ts.StartProcessing()

	return true
}

// EndProcessing -
func (ts *ThrottlerStub) EndProcessing() {
	ts.EndWasCalled = true
	if ts.EndProcessingCalled != nil {
		ts.EndProcessingCalled()
	}
}

// IsInterfaceNil -
func (ts *ThrottlerStub) IsInterfaceNil() bool {
	return ts == nil
}
