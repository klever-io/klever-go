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

	// Close joins the goroutine, so the count is frozen the moment Close
	// returns — no settling window needed.
	stable := atomic.LoadInt32(&callCount)
	time.Sleep(2 * pollingDuration)
	assert.Equal(t, stable, atomic.LoadInt32(&callCount), "expected no further handler invocations after Close returns")
}

func TestAppStatusPolling_Close_WaitsForInFlightHandler(t *testing.T) {
	t.Parallel()

	pollingDuration := time.Second
	handlerStarted := make(chan struct{})
	var handlerDone int32

	asp, err := appStatusPolling.NewAppStatusPolling(&statusHandler.NilStatusHandler{}, pollingDuration)
	require.Nil(t, err)

	var once sync.Once
	err = asp.RegisterPollingFunc(func(_ core.AppStatusHandler) {
		once.Do(func() {
			close(handlerStarted)
			// Hold the tick open long enough that a non-blocking Close would
			// demonstrably return while this handler is still running.
			time.Sleep(500 * time.Millisecond)
			atomic.StoreInt32(&handlerDone, 1)
		})
	})
	require.Nil(t, err)

	asp.Poll()

	// Close only once a handler is provably mid-flight.
	<-handlerStarted
	require.NoError(t, asp.Close())

	assert.Equal(t, int32(1), atomic.LoadInt32(&handlerDone),
		"Close must not return while a registered handler is still executing")
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

	asp.Poll()
	t.Cleanup(func() { close(release) })

	<-handlerStarted

	// Close must give up rather than block the caller forever: in production
	// closeAllComponents still has Store.CloseAll and the trie persisters to
	// run under a shared watchdog.
	start := time.Now()
	err = asp.Close()
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, appStatusPolling.ErrCloseTimeout)
	assert.Less(t, elapsed, 5*time.Second, "Close should have given up near closeJoinTimeout")
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
		go func() {
			defer wg.Done()
			_ = asp.Close()
		}()
		wg.Wait()
	}
}
