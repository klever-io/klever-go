package appStatusPolling_test

import (
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

	// Close does not synchronously join the goroutine, so a tick already
	// selected may complete after Close returns. Wait long enough that the
	// goroutine has observed `done`, then assert the count is stable across a
	// second polling interval.
	time.Sleep(2 * pollingDuration)
	stable := atomic.LoadInt32(&callCount)
	time.Sleep(2 * pollingDuration)
	assert.Equal(t, stable, atomic.LoadInt32(&callCount), "expected no further handler invocations after Close settles")
}
