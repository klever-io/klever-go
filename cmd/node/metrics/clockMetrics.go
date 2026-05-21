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
		// ClockOffset is signed: negative means OS clock is fast relative to NTP
		// servers, positive means it is slow. Route through SetInt64Value so the
		// sign survives the float64 round-trip done by the metrics provider.
		appStatusHandler.SetInt64Value(core.MetricClockOffsetNs, syncTimer.ClockOffset().Nanoseconds())

		// Zero before the first successful sync — publish 0 explicitly so
		// `time() - value > threshold` alarms only fire once a sync has
		// actually happened (avoids the "node booted but NTP is broken" alarm
		// firing identically to "node has been up for years without resync").
		var lastSyncUnix uint64
		if ts := syncTimer.LastSyncTimestamp(); !ts.IsZero() {
			lastSyncUnix = uint64(ts.Unix()) // #nosec G115 -- unix seconds fit in uint64 well beyond year 2262
		}
		appStatusHandler.SetUInt64Value(core.MetricClockLastSyncTimestamp, lastSyncUnix)
	}
}
