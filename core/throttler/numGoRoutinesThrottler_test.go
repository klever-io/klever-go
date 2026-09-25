package throttler_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNumGoRoutinesThrottler_TryStartProcessing(t *testing.T) {
	t.Parallel()

	nt, _ := throttler.NewNumGoRoutinesThrottler(2)

	assert.True(t, nt.TryStartProcessing())
	assert.True(t, nt.TryStartProcessing())
	assert.False(t, nt.TryStartProcessing())
	assert.False(t, nt.CanProcess())

	nt.EndProcessing()
	assert.True(t, nt.TryStartProcessing())
	assert.False(t, nt.TryStartProcessing())
}

func TestNumGoRoutinesThrottler_TryStartProcessingConcurrentNeverExceedsMax(t *testing.T) {
	t.Parallel()

	const max = int32(10)
	const numCallers = 1000
	nt, _ := throttler.NewNumGoRoutinesThrottler(max)

	start := make(chan struct{})
	release := make(chan struct{})
	var admitted, refused, inFlight, peak int32

	wg := sync.WaitGroup{}
	wg.Add(numCallers)
	for i := 0; i < numCallers; i++ {
		go func() {
			defer wg.Done()
			<-start
			if !nt.TryStartProcessing() {
				atomic.AddInt32(&refused, 1)
				return
			}
			atomic.AddInt32(&admitted, 1)
			current := atomic.AddInt32(&inFlight, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if current <= p || atomic.CompareAndSwapInt32(&peak, p, current) {
					break
				}
			}
			<-release
			atomic.AddInt32(&inFlight, -1)
			nt.EndProcessing()
		}()
	}

	close(start)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&admitted)+atomic.LoadInt32(&refused) == numCallers
	}, 10*time.Second, time.Millisecond)
	close(release)
	wg.Wait()

	assert.Equal(t, max, admitted)
	assert.Equal(t, int32(numCallers)-max, refused)
	assert.LessOrEqual(t, peak, max)
	assert.True(t, nt.CanProcess())
}
