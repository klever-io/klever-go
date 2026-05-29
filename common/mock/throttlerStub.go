package mock

import "sync/atomic"

// ThrottlerStub -
type ThrottlerStub struct {
	CanProcessCalled      func() bool
	StartProcessingCalled func()
	EndProcessingCalled   func()
	StartWasCalled        bool
	EndWasCalled          bool
	startProcessingCount  int32
	endProcessingCount    int32
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
	atomic.AddInt32(&ts.startProcessingCount, 1)
	if ts.StartProcessingCalled != nil {
		ts.StartProcessingCalled()
	}
}

// EndProcessing -
func (ts *ThrottlerStub) EndProcessing() {
	ts.EndWasCalled = true
	atomic.AddInt32(&ts.endProcessingCount, 1)
	if ts.EndProcessingCalled != nil {
		ts.EndProcessingCalled()
	}
}

// StartProcessingCount returns the number of StartProcessing invocations.
func (ts *ThrottlerStub) StartProcessingCount() int32 {
	return atomic.LoadInt32(&ts.startProcessingCount)
}

// EndProcessingCount returns the number of EndProcessing invocations.
func (ts *ThrottlerStub) EndProcessingCount() int32 {
	return atomic.LoadInt32(&ts.endProcessingCount)
}

// IsInterfaceNil -
func (ts *ThrottlerStub) IsInterfaceNil() bool {
	return ts == nil
}
