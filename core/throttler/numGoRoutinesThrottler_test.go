package throttler_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewNumGoRoutinesThrottler_WithNegativeShouldError(t *testing.T) {
	t.Parallel()

	nt, err := throttler.NewNumGoRoutinesThrottler(-1)

	assert.Nil(t, nt)
	assert.Equal(t, common.ErrNotPositiveValue, err)
}

func TestNewNumGoRoutinesThrottler_WithZeroShouldError(t *testing.T) {
	t.Parallel()

	nt, err := throttler.NewNumGoRoutinesThrottler(0)

	assert.Nil(t, nt)
	assert.Equal(t, common.ErrNotPositiveValue, err)
}

func TestNewNumGoRoutinesThrottler_ShouldWork(t *testing.T) {
	t.Parallel()

	nt, err := throttler.NewNumGoRoutinesThrottler(1)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(nt))
}

func TestNumGoRoutinesThrottler_CanProcessMessageWithZeroCounter(t *testing.T) {
	t.Parallel()

	nt, _ := throttler.NewNumGoRoutinesThrottler(1)

	assert.True(t, nt.CanProcess())
}

func TestNumGoRoutinesThrottler_CanProcessMessageCounterEqualsMax(t *testing.T) {
	t.Parallel()

	nt, _ := throttler.NewNumGoRoutinesThrottler(1)
	nt.StartProcessing()

	assert.False(t, nt.CanProcess())
}

func TestNumGoRoutinesThrottler_CanProcessMessageCounterIsMaxLessThanOne(t *testing.T) {
	t.Parallel()

	max := int32(45)
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	for i := int32(0); i < max-1; i++ {
		nt.StartProcessing()
	}

	assert.True(t, nt.CanProcess())
}

func TestNumGoRoutinesThrottler_CanProcessMessageCounterIsMax(t *testing.T) {
	t.Parallel()

	max := int32(45)
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	for i := int32(0); i < max; i++ {
		nt.StartProcessing()
	}

	assert.False(t, nt.CanProcess())
}

func TestNumGoRoutinesThrottler_CanProcessMessageCounterIsMaxLessOneFromEndProcessMessage(t *testing.T) {
	t.Parallel()

	max := int32(45)
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	for i := int32(0); i < max; i++ {
		nt.StartProcessing()
	}
	nt.EndProcessing()

	assert.True(t, nt.CanProcess())
}

func TestNumGoRoutinesThrottler_TryStartProcessingNeverExceedsMax(t *testing.T) {
	t.Parallel()

	const max = 5
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	const numCallers = 200
	started := int32(0)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if nt.TryStartProcessing() {
				atomic.AddInt32(&started, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(max), atomic.LoadInt32(&started))
	assert.False(t, nt.CanProcess())

	nt.EndProcessing()
	assert.True(t, nt.TryStartProcessing())
	assert.False(t, nt.TryStartProcessing())
}

func TestNumGoRoutinesThrottler_TryStartProcessingKeepsLimitWhileSlotsAreReleased(t *testing.T) {
	t.Parallel()

	const max = 4
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	inFlight, exceeded := int32(0), int32(0)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 200; j++ {
				if !nt.TryStartProcessing() {
					continue
				}
				if atomic.AddInt32(&inFlight, 1) > max {
					atomic.StoreInt32(&exceeded, 1)
				}
				atomic.AddInt32(&inFlight, -1)
				nt.EndProcessing()
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Zero(t, atomic.LoadInt32(&exceeded), "more than max callers held a slot at once")
	assert.True(t, nt.CanProcess())
}
