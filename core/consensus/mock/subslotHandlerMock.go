package mock

import "github.com/klever-io/klever-go/core/consensus"

// SubslotHandlerMock -
type SubslotHandlerMock struct {
	DoWorkCalled           func(slotHandler consensus.SlotManager) bool
	PreviousCalled         func() int
	NextCalled             func() int
	CurrentCalled          func() int
	StartTimeCalled        func() int64
	EndTimeCalled          func() int64
	NameCalled             func() string
	JobCalled              func() bool
	CheckCalled            func() bool
	ConsensusChannelCalled func() chan bool
}

// DoWork -
func (srm *SubslotHandlerMock) DoWork(slotHandler consensus.SlotManager) bool {
	return srm.DoWorkCalled(slotHandler)
}

// Previous -
func (srm *SubslotHandlerMock) Previous() int {
	return srm.PreviousCalled()
}

// Next -
func (srm *SubslotHandlerMock) Next() int {
	return srm.NextCalled()
}

// Current -
func (srm *SubslotHandlerMock) Current() int {
	return srm.CurrentCalled()
}

// StartTime -
func (srm *SubslotHandlerMock) StartTime() int64 {
	return srm.StartTimeCalled()
}

// EndTime -
func (srm *SubslotHandlerMock) EndTime() int64 {
	return srm.EndTimeCalled()
}

// Name -
func (srm *SubslotHandlerMock) Name() string {
	return srm.NameCalled()
}

// ConsensusChannel -
func (srm *SubslotHandlerMock) ConsensusChannel() chan bool {
	return srm.ConsensusChannelCalled()
}

// IsInterfaceNil returns true if there is no value under the interface
func (srm *SubslotHandlerMock) IsInterfaceNil() bool {
	return srm == nil
}
