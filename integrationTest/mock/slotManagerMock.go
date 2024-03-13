package mock

import (
	"time"

	"github.com/klever-io/klever-go/ntp"
)

// SlotManagerMock -
type SlotManagerMock struct {
	SlotIndex              int64
	IndexField             int64
	TimestampField         time.Time
	TimeDurationField      time.Duration
	RemainingTimeField     time.Duration
	GenesisTimeField       time.Time
	SyncTimer              ntp.SyncTimer
	GenesisTimestampCalled func() time.Time
	IndexCalled            func() int64
	TimeDurationCalled     func() time.Duration
	TimestampCalled        func() time.Time
	UpdateSlotCalled       func(time.Time)
	RemainingTimeCalled    func(startTime time.Time, maxTime time.Duration) time.Duration
	BeforeGenesisCalled    func() bool
}

// GenesisTimestamp -
func (smm *SlotManagerMock) GenesisTimestamp() time.Time {
	return smm.GenesisTimeField
}

// BeforeGenesis -
func (smm *SlotManagerMock) BeforeGenesis() bool {
	if smm.BeforeGenesisCalled != nil {
		return smm.BeforeGenesisCalled()
	}
	return false
}

// Index -
func (smm *SlotManagerMock) Index() int64 {
	if smm.IndexCalled != nil {
		return smm.IndexCalled()
	}

	return smm.SlotIndex
}

// TimeDuration -
func (smm *SlotManagerMock) TimeDuration() time.Duration {
	if smm.TimeDurationCalled != nil {
		return smm.TimeDurationCalled()
	}

	return 4000 * time.Millisecond
}

// Timestamp -
func (smm *SlotManagerMock) Timestamp() time.Time {
	if smm.TimestampCalled != nil {
		return smm.TimestampCalled()
	}

	return smm.TimestampField
}

// UpdateSlot -
func (smm *SlotManagerMock) UpdateSlot(timestamp time.Time) {
	if smm.UpdateSlotCalled != nil {
		smm.UpdateSlotCalled(timestamp)
		return
	}

	smm.SlotIndex++
}

// RemainingTime -
func (smm *SlotManagerMock) RemainingTime(startTime time.Time, maxTime time.Duration) time.Duration {
	currentTime := smm.SyncTimer.CurrentTime()
	elapsedTime := currentTime.Sub(startTime)
	remainingTime := maxTime - elapsedTime

	return remainingTime
}

func (smm *SlotManagerMock) ValidateSlotTimestamp(slot int64, timestamp int64) bool {
	return true
}

// IsInterfaceNil returns true if there is no value under the interface
func (smm *SlotManagerMock) IsInterfaceNil() bool {
	return smm == nil
}
