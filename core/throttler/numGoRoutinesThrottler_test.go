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
