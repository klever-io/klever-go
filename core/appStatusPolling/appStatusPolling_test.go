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

	err = asp.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetInt64Value(core.MetricNumConnectedPeers, int64(10))
	})
	assert.Nil(t, err)

	asp.Poll()
	t.Cleanup(func() { _ = asp.Close() })

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

	err = asp.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetInt64Value(core.MetricNumConnectedPeers, int64(10))
	})
	assert.Nil(t, err)

	asp.Poll()
	// Poll until the goroutine has fired at least once — tolerant of -race jitter.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&callCount) > 0
	}, 3*pollingDuration, 50*time.Millisecond, "expected goroutine to fire at least once before Close")

	assert.NoError(t, asp.Close())
}

func TestAppStatusPolling_Close_WaitsForInFlightHandler(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	handlerStarted := make(chan struct{})
	var releaseOnce sync.Once
	releaseFn := func() {
		releaseOnce.Do(func() { close(release) })
	}

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)

	var once sync.Once
	err = asp.RegisterPollingFunc(func(_ core.AppStatusHandler) {
		once.Do(func() {
			close(handlerStarted)
			<-release
		})
	})
	require.Nil(t, err)

	asp.Poll()
	t.Cleanup(releaseFn)

	<-handlerStarted

	errCh := make(chan error, 1)
	go func() {
		errCh <- asp.Close()
	}()

	select {
	case err = <-errCh:
		t.Fatalf("Close returned while the handler was still running: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	releaseFn()

	select {
	case err = <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after the handler finished")
	}
}

func TestAppStatusPolling_Close_BeforePollDoesNotBlock(t *testing.T) {
	t.Parallel()

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)

	// The goroutine is started by Poll, not the constructor, so Close on a
	// never-polled instance must return immediately rather than join nothing.
	// The result travels over a channel so the assertion runs on the test
	// goroutine — asserting from the spawned one would panic the whole package
	// run ("Log in goroutine after ... has completed") if the timeout won.
	errCh := make(chan error, 1)
	go func() {
		errCh <- asp.Close()
	}()

	select {
	case err = <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked although Poll was never called")
	}
}

func TestAppStatusPolling_Close_GivesUpOnStuckHandler(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	handlerStarted := make(chan struct{})

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
	require.Nil(t, err)

	var once sync.Once
	err = asp.RegisterPollingFunc(func(_ core.AppStatusHandler) {
		once.Do(func() {
			close(handlerStarted)
			<-release // stuck until the test lets go
		})
	})
	require.Nil(t, err)

	var releaseOnce sync.Once
	releaseFn := func() {
		releaseOnce.Do(func() { close(release) })
	}
	asp.Poll()
	t.Cleanup(releaseFn)

	<-handlerStarted

	// Close must give up rather than block the caller forever: in production
	// closeAllComponents still has Store.CloseAll and the trie persisters to
	// run under a shared watchdog.
	err, elapsed := closeWithin(t, asp, 5*time.Second)
	assert.ErrorIs(t, err, appStatusPolling.ErrCloseTimeout)
	assert.Greater(t, elapsed, 500*time.Millisecond, "Close returned ErrCloseTimeout without waiting")
	assert.Less(t, elapsed, 5*time.Second, "Close should have given up near closeJoinTimeout")

	// A retry while the handler is still stuck waits on the same joiner.
	err, elapsed = closeWithin(t, asp, 5*time.Second)
	assert.ErrorIs(t, err, appStatusPolling.ErrCloseTimeout)
	assert.Greater(t, elapsed, 500*time.Millisecond, "retried Close did not wait for the in-flight handler")

	releaseFn()

	err, elapsed = closeWithin(t, asp, 2*time.Second)
	assert.NoError(t, err)
	assert.Less(t, elapsed, time.Second, "Close after the handler finished should return promptly")
}

func TestAppStatusPolling_PollAfterCloseIsNoOp(t *testing.T) {
	t.Parallel()

	var callCount int32
	ash := mock.AppStatusHandlerStub{
		SetInt64ValueHandler: func(key string, value int64) {
			atomic.AddInt32(&callCount, 1)
		},
	}
	asp, err := appStatusPolling.NewAppStatusPolling(&ash, time.Second)
	require.Nil(t, err)

	err = asp.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetInt64Value(core.MetricNumConnectedPeers, int64(10))
	})
	require.Nil(t, err)

	require.NoError(t, asp.Close())

	// Must not start a goroutine that reuses the WaitGroup after Wait returned.
	asp.Poll()

	// The polling floor is one second, so wait past one tick. This is an
	// absence check: if Poll did nothing, there is no event to wait on.
	time.Sleep(2 * time.Second)
	assert.Zero(t, atomic.LoadInt32(&callCount), "Poll after Close must not fire handlers")
	assert.NoError(t, asp.Close())
}

func TestAppStatusPolling_ConcurrentPollAndCloseShouldNotPanic(t *testing.T) {
	t.Parallel()

	// Poll racing Close is what can drive wg.Go's Add into a live wg.Wait,
	// which sync answers with a process-killing "WaitGroup misuse" panic.
	// Probabilistic by nature, so hammer it.
	for i := 0; i < 200; i++ {
		asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, time.Second)
		require.Nil(t, err)

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
