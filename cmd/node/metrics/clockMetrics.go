package metrics

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/tools/check"
)

// StartClockMetricsPolling registers a polling function that publishes the NTP
// syncer's current clock offset and the timestamp of the last successful sync.
// These two signals cover every blockchain-operator alarm:
//
//   - abs(klv_clock_offset_ns) > tolerance         → drift will hurt consensus
//   - time() - klv_clock_last_sync_timestamp > 2×sync_period → NTP path broken
//
// Returned io.Closer stops the polling goroutine; nil if validation fails.
func StartClockMetricsPolling(
	ash core.AppStatusHandler,
	syncTimer ntp.SyncTimer,
	pollingInterval time.Duration,
) (io.Closer, error) {
	if check.IfNil(ash) {
		return nil, errors.New("nil AppStatusHandler")
	}
	if check.IfNil(syncTimer) {
		return nil, errors.New("nil SyncTimer")
	}

	appStatusPollingHandler, err := appStatusPolling.NewAppStatusPolling(ash, pollingInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot init AppStatusPolling for clock metrics: %w", err)
	}

	pollingFunc := buildClockMetricsPollingFunc(syncTimer)
	err = appStatusPollingHandler.RegisterPollingFunc(pollingFunc)
	if err != nil {
		return nil, fmt.Errorf("cannot register clock metrics polling function: %w", err)
	}

	// Prime before the recurring poll so /node/status returns the current
	// offset on request 1 (AppStatusPolling.Poll sleeps before its first tick).
	pollingFunc(ash)

	appStatusPollingHandler.Poll()
	return appStatusPollingHandler, nil
}

func buildClockMetricsPollingFunc(syncTimer ntp.SyncTimer) func(core.AppStatusHandler) {
	return func(appStatusHandler core.AppStatusHandler) {
		// Read offset and last-sync timestamp under a single RLock so a scrape
		// can't observe a fresh offset paired with a stale timestamp (or vice
		// versa) if a sync lands between two separate getter calls.
		offset, lastSync := syncTimer.ClockSnapshot()

		// ClockOffset is signed: negative means OS clock is fast relative to NTP
		// servers, positive means it is slow. SetInt64Value (not SetUInt64Value)
		// is required because uint64 can't represent negative values.
		appStatusHandler.SetInt64Value(core.MetricClockOffsetNs, offset.Nanoseconds())

		// Zero before the first successful sync — publish 0 explicitly so
		// `time() - value > threshold` alarms only fire once a sync has
		// actually happened. The unix > 0 guard defends against a pathological
		// pre-1970 RTC: uint64(negative-int64) wraps to a huge value that
		// would silently keep "stale-sync" alarms quiet forever.
		var lastSyncUnix uint64
		if !lastSync.IsZero() {
			if unix := lastSync.Unix(); unix > 0 {
				lastSyncUnix = uint64(unix)
			}
		}
		appStatusHandler.SetUInt64Value(core.MetricClockLastSyncTimestamp, lastSyncUnix)
	}
}
