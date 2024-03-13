package slot_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

const slotTimeDuration = 10 * time.Millisecond

func TestSlot_NewSlotShouldErrNilSyncTimer(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	s, err := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, nil, 0)

	assert.Nil(t, s)
	assert.Equal(t, common.ErrNilSyncTimer, err)
}

func TestSlot_NewSlotShouldWork(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, err := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(s))
}

func TestSlot_UpdateSlotShouldNotChangeAnything(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)
	oldIndex := s.Index()
	oldTimestamp := s.Timestamp()

	s.UpdateSlot(genesisTime)

	newIndex := s.Index()
	newTimestamp := s.Timestamp()

	assert.Equal(t, oldIndex, newIndex)
	assert.Equal(t, oldTimestamp, newTimestamp)
}

func TestSlot_UpdateSlotShouldAdvanceOneSlot(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)
	oldIndex := s.Index()
	s.UpdateSlot(genesisTime.Add(slotTimeDuration))
	newIndex := s.Index()

	assert.Equal(t, oldIndex, newIndex-1)
}

func TestSlot_ValidateSlot(t *testing.T) {
	t.Parallel()

	slotTime := time.Second

	genesisTime := time.Now()
	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTime, syncTimerMock, 0)
	oldIndex := s.Index()

	nextSlot := oldIndex + 10
	nextTimestamp := genesisTime.Add(10 * slotTime)
	assert.True(t, s.ValidateSlotTimestamp(nextSlot, nextTimestamp.Unix()))
}

func TestSlot_IndexShouldReturnFirstIndex(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)
	s.UpdateSlot(genesisTime.Add(slotTimeDuration / 2))
	index := s.Index()

	assert.Equal(t, int64(0), index)
}

func TestSlot_TimestampShouldReturnTimestampOfTheNextSlot(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)
	s.UpdateSlot(genesisTime.Add(slotTimeDuration + slotTimeDuration/2))
	timestamp := s.Timestamp()

	assert.Equal(t, genesisTime.Add(slotTimeDuration), timestamp)
}

func TestSlot_TimeDurationShouldReturnTheDurationOfOneSlot(t *testing.T) {
	t.Parallel()

	genesisTime := time.Now()

	syncTimerMock := &mock.SyncTimerMock{}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)
	timeDuration := s.TimeDuration()

	assert.Equal(t, slotTimeDuration, timeDuration)
}

func TestSlot_RemainingTimeInCurrentSlotShouldReturnPositiveValue(t *testing.T) {
	t.Parallel()

	genesisTime := time.Unix(1, 0)

	syncTimerMock := &mock.SyncTimerMock{}

	timeElapsed := int64(slotTimeDuration - 1*time.Second)

	syncTimerMock.CurrentTimeCalled = func() time.Time {
		return time.Unix(1, timeElapsed)
	}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)

	remainingTime := s.RemainingTime(s.Timestamp(), s.TimeDuration())

	assert.Equal(t, time.Duration(int64(s.TimeDuration())-timeElapsed), remainingTime)
	assert.True(t, remainingTime > 0)
}

func TestSlot_RemainingTimeInCurrentSlotShouldReturnNegativeValue(t *testing.T) {
	t.Parallel()

	genesisTime := time.Unix(1, 0)

	syncTimerMock := &mock.SyncTimerMock{}

	timeElapsed := int64(slotTimeDuration + 1*time.Second)

	syncTimerMock.CurrentTimeCalled = func() time.Time {
		return time.Unix(1, timeElapsed)
	}

	s, _ := slot.NewSlotManager(genesisTime, genesisTime, slotTimeDuration, syncTimerMock, 0)

	remainingTime := s.RemainingTime(s.Timestamp(), s.TimeDuration())

	assert.Equal(t, time.Duration(int64(s.TimeDuration())-timeElapsed), remainingTime)
	assert.True(t, remainingTime < 0)
}
