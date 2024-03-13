package slot

import (
	"math"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/tools/check"
)

// slotManager defines the data needed by block slot
type slotManager struct {
	genesisTime  time.Time
	index        int64         // represents the index of the slot in the current chronology (current time - genesis time) / slot duration
	timestamp    time.Time     // represents the start time of the slot in the current chronology genesis time + slot index * slot duration
	timeDuration time.Duration // represents the duration of the slot in current chronology
	syncTimer    ntp.SyncTimer
	startSlot    int64
}

// NewSlotManager defines a new slot object
func NewSlotManager(
	genesisTimestamp time.Time,
	currentTimestamp time.Time,
	slotTimeDuration time.Duration,
	syncTimer ntp.SyncTimer,
	startSlot int64,
) (*slotManager, error) {

	if check.IfNil(syncTimer) {
		return nil, common.ErrNilSyncTimer
	}

	s := slotManager{
		genesisTime:  genesisTimestamp,
		timeDuration: slotTimeDuration,
		timestamp:    genesisTimestamp,
		syncTimer:    syncTimer,
		startSlot:    startSlot,
	}
	s.UpdateSlot(currentTimestamp)
	return &s, nil
}

// UpdateSlot updates the index and the time stamp of the slot depending of the genesis time and the current time given
func (s *slotManager) UpdateSlot(currentTimestamp time.Time) {
	delta := currentTimestamp.Sub(s.GenesisTimestamp()).Nanoseconds()

	index := int64(math.Floor(float64(delta)/float64(s.timeDuration.Nanoseconds()))) + s.startSlot

	if s.index != index {
		s.index = index
		s.timestamp = s.GenesisTimestamp().Add(time.Duration((index - s.startSlot) * s.timeDuration.Nanoseconds()))
	}
}

func (s *slotManager) ValidateSlotTimestamp(slot int64, timestamp int64) bool {
	estimatedBase := s.GenesisTimestamp().Add(time.Duration((slot - s.startSlot) * s.timeDuration.Nanoseconds()))
	estimatedTop := s.GenesisTimestamp().Add(time.Duration((slot + 1 - s.startSlot) * s.timeDuration.Nanoseconds()))

	return timestamp >= estimatedBase.Unix() && timestamp < estimatedTop.Unix()
}

// GenesisTimestamp returns the genesis time stamp
func (s *slotManager) GenesisTimestamp() time.Time {
	return s.genesisTime
}

// BeforeGenesis returns true if slot index is before start slot
func (s *slotManager) BeforeGenesis() bool {
	return s.index <= s.startSlot
}

// Index returns the index of the slot in current epoch
func (s *slotManager) Index() int64 {
	return s.index
}

// Timestamp returns the time stamp of the slot
func (s *slotManager) Timestamp() time.Time {
	return s.timestamp
}

// TimeDuration returns the duration of the slot
func (s *slotManager) TimeDuration() time.Duration {
	return s.timeDuration
}

// RemainingTime returns the remaining time in the current round given by the current time, slot start time and
// safe threshold percent
func (s *slotManager) RemainingTime(startTime time.Time, maxTime time.Duration) time.Duration {
	currentTime := s.syncTimer.CurrentTime()
	elapsedTime := currentTime.Sub(startTime)
	remainingTime := maxTime - elapsedTime

	return remainingTime
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *slotManager) IsInterfaceNil() bool {
	return s == nil
}
