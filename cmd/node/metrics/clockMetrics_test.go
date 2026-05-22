package metrics

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureHandler records SetInt64Value and SetUInt64Value calls so tests can
// assert which keys the polling function published and with what values.
type captureHandler struct {
	mu  sync.Mutex
	i64 map[string]int64
	u64 map[string]uint64
}

func newCaptureHandler() (*captureHandler, *mock.AppStatusHandlerStub) {
	cap := &captureHandler{
		i64: map[string]int64{},
		u64: map[string]uint64{},
	}
	stub := &mock.AppStatusHandlerStub{
		SetInt64ValueHandler: func(key string, value int64) {
			cap.mu.Lock()
			cap.i64[key] = value
			cap.mu.Unlock()
		},
		SetUInt64ValueHandler: func(key string, value uint64) {
			cap.mu.Lock()
			cap.u64[key] = value
			cap.mu.Unlock()
		},
	}
	return cap, stub
}

func TestStartClockMetricsPolling_NilAppStatusHandler(t *testing.T) {
	syncTimer := &consensusMock.SyncTimerMock{}

	closer, err := StartClockMetricsPolling(nil, syncTimer, time.Second)

	assert.Error(t, err)
	assert.Nil(t, closer)
}

func TestStartClockMetricsPolling_NilSyncTimer(t *testing.T) {
	_, stub := newCaptureHandler()

	closer, err := StartClockMetricsPolling(stub, nil, time.Second)

	assert.Error(t, err)
	assert.Nil(t, closer)
}

func TestStartClockMetricsPolling_InvalidPollingInterval(t *testing.T) {
	_, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{}

	closer, err := StartClockMetricsPolling(stub, syncTimer, 0)

	assert.Error(t, err)
	assert.Nil(t, closer)
}

func TestStartClockMetricsPolling_PrimesMetricsBeforeFirstTick(t *testing.T) {
	// Verify the polling function runs once synchronously so /node/status
	// returns real values on the first scrape, not zero.
	const offset = time.Duration(-38) * time.Millisecond
	syncTs := time.Unix(1_700_000_000, 0)

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled:       func() time.Duration { return offset },
		LastSyncTimestampCalled: func() time.Time { return syncTs },
	}

	closer, err := StartClockMetricsPolling(stub, syncTimer, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, closer)
	defer func() { _ = closer.Close() }()

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, offset.Nanoseconds(), cap.i64[core.MetricClockOffsetNs])
	assert.Equal(t, uint64(syncTs.Unix()), cap.u64[core.MetricClockLastSyncTimestamp])
}

func TestBuildClockMetricsPollingFunc_NegativeOffsetPreservesSign(t *testing.T) {
	// The "OS clock fast" case (operator-relevant: a fast clock makes the node
	// emit timestamps in the future relative to peers, breaking quorum). Sign
	// must survive the SetInt64Value round-trip.
	const offset = time.Duration(-123_456_789) * time.Nanosecond

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled: func() time.Duration { return offset },
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, offset.Nanoseconds(), cap.i64[core.MetricClockOffsetNs])
	assert.Less(t, cap.i64[core.MetricClockOffsetNs], int64(0), "negative offset must remain negative")
}

func TestBuildClockMetricsPollingFunc_PositiveOffset(t *testing.T) {
	const offset = 42 * time.Millisecond

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled: func() time.Duration { return offset },
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, offset.Nanoseconds(), cap.i64[core.MetricClockOffsetNs])
}

func TestBuildClockMetricsPollingFunc_ZeroTimestampBeforeFirstSync(t *testing.T) {
	// A node that has never successfully synced (e.g. NTP timeouts at boot)
	// must publish timestamp=0, not a Unix-epoch translation of the zero time.
	// Operators alarm on `time() - klv_clock_last_sync_timestamp > threshold`;
	// leaking time.Time{}.Unix() (≈ -6e10) would make the alarm fire spuriously
	// or never fire at all depending on the operator's filter expression.
	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		// LastSyncTimestampCalled left nil → returns time.Time{}
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, int64(0), cap.i64[core.MetricClockOffsetNs])
	assert.Equal(t, uint64(0), cap.u64[core.MetricClockLastSyncTimestamp],
		"zero time.Time must translate to 0, not the Unix epoch of the zero time")
}

func TestBuildClockMetricsPollingFunc_LastSyncTimestampPublished(t *testing.T) {
	syncTs := time.Unix(1_700_000_000, 0)

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		LastSyncTimestampCalled: func() time.Time { return syncTs },
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, uint64(syncTs.Unix()), cap.u64[core.MetricClockLastSyncTimestamp])
}

func TestBuildClockMetricsPollingFunc_NegativeUnixGuard(t *testing.T) {
	// Pathological case: a non-zero timestamp whose Unix() is negative (pre-1970
	// RTC after a successful sync). Without the guard, uint64(negative-int64)
	// wraps to ~1.8e19, which would silently bypass any "stale sync" alarm
	// expressed as `time() - value > threshold`. Must publish 0 instead.
	preEpoch := time.Date(1969, 6, 1, 0, 0, 0, 0, time.UTC)
	require.True(t, preEpoch.Unix() < 0, "test setup: timestamp must precede 1970")

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		LastSyncTimestampCalled: func() time.Time { return preEpoch },
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, uint64(0), cap.u64[core.MetricClockLastSyncTimestamp],
		"negative Unix seconds must publish 0, not the uint64 wrap-around")
}

func TestBuildClockMetricsPollingFunc_UsesAtomicSnapshot(t *testing.T) {
	// Verify the polling func reads (offset, lastSync) via the single-RLock
	// snapshot, not as two independent getter calls. Forcing the legacy getters
	// to fail the assertion proves the snapshot path is what publishes the metrics.
	const offset = 1234 * time.Nanosecond
	syncTs := time.Unix(1_700_000_000, 0)

	cap, stub := newCaptureHandler()
	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled: func() time.Duration {
			t.Errorf("ClockOffset() must not be called — polling must use ClockSnapshot()")
			return 0
		},
		LastSyncTimestampCalled: func() time.Time {
			t.Errorf("LastSyncTimestamp() must not be called — polling must use ClockSnapshot()")
			return time.Time{}
		},
		ClockSnapshotCalled: func() (time.Duration, time.Time) {
			return offset, syncTs
		},
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	pollingFunc(stub)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, offset.Nanoseconds(), cap.i64[core.MetricClockOffsetNs])
	assert.Equal(t, uint64(syncTs.Unix()), cap.u64[core.MetricClockLastSyncTimestamp])
}

// TestClockMetricsAppearInPrometheusOutput is an end-to-end check that
// exercises the real statusHandler + clockMetrics polling combination,
// ensuring both metrics appear in the /node/metrics scrape output with the
// sign of klv_clock_offset_ns surviving the int64 → Prometheus number
// formatting.
func TestClockMetricsAppearInPrometheusOutput(t *testing.T) {
	const offset = time.Duration(-38) * time.Millisecond
	syncTs := time.Unix(1_700_000_000, 0)

	sm := statusHandler.NewStatusMetrics()
	sm.SetStringValue(core.MetricChainID, "testnet")

	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled:       func() time.Duration { return offset },
		LastSyncTimestampCalled: func() time.Time { return syncTs },
	}

	// Long interval so only the synchronous prime call runs during the assert;
	// avoids racing against the background poller.
	closer, err := StartClockMetricsPolling(sm, syncTimer, time.Hour)
	require.NoError(t, err)
	defer func() { _ = closer.Close() }()

	prom := sm.StatusMetricsWithoutP2PPrometheusString()

	assert.True(t, strings.Contains(prom, core.MetricClockOffsetNs),
		"missing %s in: %s", core.MetricClockOffsetNs, prom)
	assert.True(t, strings.Contains(prom, core.MetricClockLastSyncTimestamp),
		"missing %s in: %s", core.MetricClockLastSyncTimestamp, prom)

	expectedOffsetLine := fmt.Sprintf(`%s{chainID="testnet"} %d`,
		core.MetricClockOffsetNs, offset.Nanoseconds())
	assert.Contains(t, prom, expectedOffsetLine,
		"negative offset must appear with sign preserved")

	expectedTimestampLine := fmt.Sprintf(`%s{chainID="testnet"} %d`,
		core.MetricClockLastSyncTimestamp, syncTs.Unix())
	assert.Contains(t, prom, expectedTimestampLine)
}

// TestClockMetricsAppearInStatusMap checks that both metrics land in the JSON
// map returned by /node/status (used by dashboards/operator tooling that read
// the structured map rather than the scrape format).
func TestClockMetricsAppearInStatusMap(t *testing.T) {
	const offset = 42 * time.Millisecond
	syncTs := time.Unix(1_700_000_000, 0)

	sm := statusHandler.NewStatusMetrics()
	syncTimer := &consensusMock.SyncTimerMock{
		ClockOffsetCalled:       func() time.Duration { return offset },
		LastSyncTimestampCalled: func() time.Time { return syncTs },
	}

	closer, err := StartClockMetricsPolling(sm, syncTimer, time.Hour)
	require.NoError(t, err)
	defer func() { _ = closer.Close() }()

	statusMap := sm.StatusMetricsMapWithoutP2P()

	assert.Equal(t, offset.Nanoseconds(), statusMap[core.MetricClockOffsetNs])
	assert.Equal(t, uint64(syncTs.Unix()), statusMap[core.MetricClockLastSyncTimestamp])
}
