package appStatusPolling_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppStatusPooling_NilAppStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	_, err := appStatusPolling.NewAppStatusPolling(nil, time.Second)
	assert.Equal(t, err, appStatusPolling.ErrNilAppStatusHandler)
}

func TestNewAppStatusPooling_NegativePollingDurationShouldErr(t *testing.T) {
	t.Parallel()

	_, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Duration(-1))
	assert.Equal(t, err, appStatusPolling.ErrPollingDurationToSmall)
}

func TestNewAppStatusPooling_ZeroPollingDurationShouldErr(t *testing.T) {
	t.Parallel()

	_, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, 0)
	assert.Equal(t, err, appStatusPolling.ErrPollingDurationToSmall)
}

func TestNewAppStatusPooling_OkValsShouldPass(t *testing.T) {
	t.Parallel()

	_, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	assert.Nil(t, err)
}

func TestNewAppStatusPolling_RegisterHandlerFuncShouldErr(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	assert.Nil(t, err)

	err = asp.RegisterPollingFunc(nil)
	assert.Equal(t, appStatusPolling.ErrNilHandlerFunc, err)
}

func TestAppStatusPolling_Poll_TestNumOfConnectedAddressesCalled(t *testing.T) {
	t.Parallel()

	pollingDuration := time.Second
	chDone := make(chan struct{}, 1)
	ash := mock.AppStatusHandlerStub{
		SetInt64ValueHandler: func(key string, value int64) {
			select {
			case chDone <- struct{}{}:
			default:
			}
		},
	}
	asp, err := appStatusPolling.NewAppStatusPolling(&ash, pollingDuration)
	assert.Nil(t, err)
	asp.SetCloseJoinTimeout(longJoinTimeout)

	err = asp.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetInt64Value(core.MetricNumConnectedPeers, int64(10))
	})
	assert.Nil(t, err)

	asp.Poll()
	t.Cleanup(func() { assert.NoError(t, asp.Close()) })

	select {
	case <-chDone:
	case <-time.After(2 * pollingDuration):
		assert.Fail(t, "timeout calling SetInt64Value")
	}
}

func TestAppStatusPolling_Close_IsIdempotent(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	assert.Nil(t, err)

	assert.NoError(t, asp.Close())
	assert.NoError(t, asp.Close())
}

func TestAppStatusPolling_Close_StopsGoroutine(t *testing.T) {
	t.Parallel()

	pollingDuration := time.Second
	var callCount int32
	ash := mock.AppStatusHandlerStub{
		SetInt64ValueHandler: func(key string, value int64) {
			atomic.AddInt32(&callCount, 1)
		},
	}
	asp, err := appStatusPolling.NewAppStatusPolling(&ash, pollingDuration)
	assert.Nil(t, err)
	asp.SetCloseJoinTimeout(longJoinTimeout)

	err = asp.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetInt64Value(core.MetricNumConnectedPeers, int64(10))
	})
	assert.Nil(t, err)

	asp.Poll()
	stopped := asp.StoppedChan()
	// Poll until the goroutine has fired at least once — tolerant of -race jitter.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&callCount) > 0
	}, 3*pollingDuration, 50*time.Millisecond, "expected goroutine to fire at least once before Close")

	assert.NoError(t, asp.Close())

	// Nothing can fire after Close returned because the goroutine is already
	// gone. Assert that directly rather than sampling the count over a sleep.
	select {
	case <-stopped:
	default:
		t.Fatal("Close returned before the polling goroutine exited")
	}
}

func TestAppStatusPolling_Close_WaitsForInFlightHandler(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)
	asp.SetCloseJoinTimeout(longJoinTimeout)

	started, release, _ := blockOnFirstTick(t, asp)

	asp.Poll()
	<-started

	errCh := make(chan error, 1)
	go func() {
		errCh <- asp.Close()
	}()

	select {
	case err = <-errCh:
		t.Fatalf("Close returned while the handler was still running: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	release()

	select {
	case err = <-errCh:
		require.NoError(t, err)
	case <-time.After(longJoinTimeout):
		t.Fatal("Close did not return after the handler finished")
	}
}

func TestAppStatusPolling_Close_BeforePollDoesNotBlock(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)

	// The goroutine is started by Poll, not the constructor, so Close on a
	// never-polled instance must return immediately rather than join nothing.
	err, _ = closeWithin(t, asp, 5*time.Second)
	assert.NoError(t, err)
}

func TestAppStatusPolling_Close_GivesUpOnStuckHandler(t *testing.T) {
	t.Parallel()

	const joinTimeout = 200 * time.Millisecond

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)
	asp.SetCloseJoinTimeout(joinTimeout)

	started, release, _ := blockOnFirstTick(t, asp)

	asp.Poll()
	<-started

	// Close must give up rather than block the caller forever: in production
	// closeAllComponents still has Store.CloseAll and the trie persisters to
	// run under a shared watchdog.
	err, elapsed := closeWithin(t, asp, 5*time.Second)
	assert.ErrorIs(t, err, appStatusPolling.ErrCloseTimeout)
	assert.Greater(t, elapsed, joinTimeout/2, "Close returned ErrCloseTimeout without waiting")

	// A retry while the handler is still stuck waits on the same channel.
	err, elapsed = closeWithin(t, asp, 5*time.Second)
	assert.ErrorIs(t, err, appStatusPolling.ErrCloseTimeout)
	assert.Greater(t, elapsed, joinTimeout/2, "retried Close did not wait for the in-flight handler")

	release()

	asp.SetCloseJoinTimeout(longJoinTimeout)
	err, _ = closeWithin(t, asp, longJoinTimeout)
	assert.NoError(t, err)
}

func TestAppStatusPolling_Close_RunsNoBatchAfterClose(t *testing.T) {
	t.Parallel()

	pollingDuration := time.Second

	// Once done is closed the loop can have two ready cases and select picks at
	// random, so a regression only shows up about half the time. Repeat.
	for i := 0; i < 3; i++ {
		asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, pollingDuration)
		require.Nil(t, err)
		asp.SetCloseJoinTimeout(longJoinTimeout)

		started, release, calls := blockOnFirstTick(t, asp)

		asp.Poll()
		<-started

		// Outlast a full period while the batch is held, so a tick is queued.
		time.Sleep(pollingDuration + 300*time.Millisecond)

		errCh := make(chan error, 1)
		go func() {
			errCh <- asp.Close()
		}()
		require.Eventually(t, asp.CloseRequested, 5*time.Second, 5*time.Millisecond,
			"Close did not signal the polling goroutine")

		release()
		require.NoError(t, <-errCh)
		require.Equal(t, int32(1), calls(), "a handler batch ran after Close closed done")
	}
}

func TestAppStatusPolling_PollAfterCloseIsNoOp(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)

	require.NoError(t, asp.Close())

	asp.Poll()

	// Assert on what the guard controls: no goroutine is started at all. An
	// absence-of-handler-calls check would pass even without the guard, because
	// a late goroutine sees done already closed and returns before its first tick.
	assert.False(t, asp.PollStarted(), "Poll after Close must not start the goroutine")
	assert.NoError(t, asp.Close())
}

func TestAppStatusPolling_SecondPollIsNoOp(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)
	asp.SetCloseJoinTimeout(longJoinTimeout)

	// A second goroutine would double every metric update at the same interval,
	// so the second Poll must leave the first goroutine's channel in place.
	asp.Poll()
	first := asp.StoppedChan()
	require.NotNil(t, first)

	asp.Poll()
	t.Cleanup(func() { assert.NoError(t, asp.Close()) })

	assert.True(t, first == asp.StoppedChan(), "a second Poll replaced the polling goroutine")
}

func TestAppStatusPolling_ConcurrentPollAndCloseShouldNotPanic(t *testing.T) {
	t.Parallel()

	// Poll and Close both write the shutdown state, so this is what -race reads
	// for a missing mutClose. Probabilistic by nature, so hammer it.
	for i := 0; i < 200; i++ {
		asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
		require.Nil(t, err)
		asp.SetCloseJoinTimeout(longJoinTimeout)

		asp.Poll()

		var wg sync.WaitGroup
		wg.Add(3)
		for j := 0; j < 2; j++ {
			go func() {
				defer wg.Done()
				asp.Poll()
			}()
		}
		closeErr := make(chan error, 1)
		go func() {
			defer wg.Done()
			closeErr <- asp.Close()
		}()
		wg.Wait()
		require.NoError(t, <-closeErr)
	}
}

// longJoinTimeout is used wherever the join is expected to succeed, so a loaded
// CI runner cannot turn a passing test into an ErrCloseTimeout.
const longJoinTimeout = 30 * time.Second

// blockOnFirstTick registers a handler that blocks on its first call until the
// returned release runs. started closes as that call begins, and calls reports
// how many handler invocations have happened.
func blockOnFirstTick(
	t *testing.T,
	asp *appStatusPolling.AppStatusPolling,
) (started <-chan struct{}, release func(), calls func() int32) {
	t.Helper()

	startedCh := make(chan struct{})
	releaseCh := make(chan struct{})
	var startOnce, releaseOnce sync.Once
	var callCount int32

	releaseFn := func() {
		releaseOnce.Do(func() { close(releaseCh) })
	}

	err := asp.RegisterPollingFunc(func(_ core.AppStatusHandler) {
		atomic.AddInt32(&callCount, 1)
		startOnce.Do(func() {
			close(startedCh)
			<-releaseCh
		})
	})
	require.Nil(t, err)
	t.Cleanup(releaseFn)

	return startedCh, releaseFn, func() int32 { return atomic.LoadInt32(&callCount) }
}

// closeWithin runs Close off the test goroutine so a missing timeout fails
// here instead of hanging until the package deadline.
func closeWithin(t *testing.T, asp *appStatusPolling.AppStatusPolling, limit time.Duration) (error, time.Duration) {
	t.Helper()

	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- asp.Close()
	}()

	select {
	case err := <-errCh:
		return err, time.Since(start)
	case <-time.After(limit):
		t.Fatalf("Close blocked longer than %s", limit)
		return nil, 0
	}
}
