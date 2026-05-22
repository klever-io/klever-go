package mock

import (
	"time"
)

// SyncTimerMock mocks the implementation for a SyncTimer
type SyncTimerMock struct {
	ClockOffsetCalled       func() time.Duration
	LastSyncTimestampCalled func() time.Time
	ClockSnapshotCalled     func() (time.Duration, time.Time)
	CurrentTimeCalled       func() time.Time
}

// StartSyncingTime method does the time synchronization at every syncPeriod time elapsed. This should be started as a go routine
func (stm *SyncTimerMock) StartSyncingTime() {
	panic("implement me")
}

// ClockOffset method gets the current time offset
func (stm *SyncTimerMock) ClockOffset() time.Duration {
	if stm.ClockOffsetCalled != nil {
		return stm.ClockOffsetCalled()
	}

	return time.Duration(0)
}

// LastSyncTimestamp returns the configured last sync timestamp or the zero time
func (stm *SyncTimerMock) LastSyncTimestamp() time.Time {
	if stm.LastSyncTimestampCalled != nil {
		return stm.LastSyncTimestampCalled()
	}

	return time.Time{}
}

// ClockSnapshot returns the configured (offset, lastSync) pair under a single
// callback so tests can assert atomic-pair semantics. Falls back to the
// individual getters when no explicit snapshot stub is set, which preserves
// the existing behavior for tests that only configure one or both fields.
func (stm *SyncTimerMock) ClockSnapshot() (time.Duration, time.Time) {
	if stm.ClockSnapshotCalled != nil {
		return stm.ClockSnapshotCalled()
	}

	return stm.ClockOffset(), stm.LastSyncTimestamp()
}

// FormattedCurrentTime method gets the formatted current time on which is added a given offset
func (stm *SyncTimerMock) FormattedCurrentTime() string {
	return time.Unix(0, 0).String()
}

// CurrentTime method gets the current time on which is added the current offset
func (stm *SyncTimerMock) CurrentTime() time.Time {
	if stm.CurrentTimeCalled != nil {
		return stm.CurrentTimeCalled()
	}

	return time.Unix(0, 0)
}

// Close -
func (stm *SyncTimerMock) Close() error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (stm *SyncTimerMock) IsInterfaceNil() bool {
	return stm == nil
}
