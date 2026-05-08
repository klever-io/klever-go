package prewarmer

import (
	"sync"
	"time"
)

// MetricsSink is the slice of an AppStatusHandler the reporter consumes.
// Defining it locally lets the prewarmer package stay free of the broader
// core.AppStatusHandler import. The production AppStatusHandler trivially
// satisfies it via its SetUInt64Value method.
type MetricsSink interface {
	SetUInt64Value(key string, value uint64)
}

// MetricKeys bundles the metric names this reporter writes. Production wires
// these to the constants in core/metrics.go so naming stays consistent with
// the rest of the node's telemetry; tests can pass arbitrary strings.
type MetricKeys struct {
	Enqueued                 string
	Dropped                  string
	CompileSucceeded         string
	CompileFailed            string
	SkippedAlreadyCached     string
	SkippedDuplicateInFlight string
	SkippedFetchFailed       string
	QueueDepth               string
	QueueCapacity            string
}

// MetricsReporter samples a Prewarmer's Stats() at a fixed interval and
// pushes the values into an AppStatusHandler-shaped sink. Capacity is set
// once at start; the rest are updated each tick.
//
// Sampling (rather than incrementing per-event) is intentional: it keeps the
// hot OnTxAdded / prewarm paths free of any sink calls, and the per-tick
// snapshot gives consistent values across all metrics for the same instant.
type MetricsReporter struct {
	pw     *Prewarmer
	sink   MetricsSink
	keys   MetricKeys
	tick   time.Duration
	quit   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
}

// NewMetricsReporter constructs a reporter. Tick must be > 0; production
// typically uses 5–15 seconds. A nil sink or nil pw makes Start a no-op.
func NewMetricsReporter(pw *Prewarmer, sink MetricsSink, keys MetricKeys, tick time.Duration) *MetricsReporter {
	return &MetricsReporter{
		pw:   pw,
		sink: sink,
		keys: keys,
		tick: tick,
		quit: make(chan struct{}),
	}
}

// Start spawns the sampling goroutine. Idempotent — repeat calls are no-ops.
func (r *MetricsReporter) Start() {
	if r == nil || r.pw == nil || r.sink == nil || r.tick <= 0 {
		return
	}
	r.once.Do(func() {
		// Capacity is static; publish once at start.
		r.sink.SetUInt64Value(r.keys.QueueCapacity, uint64(r.pw.Stats().QueueCapacity))
		r.wg.Add(1)
		go r.loop()
	})
}

// Close stops the sampling goroutine. Returns nil; satisfies io.Closer so
// the reporter can be threaded into Process.Closers.
func (r *MetricsReporter) Close() error {
	if r == nil {
		return nil
	}
	select {
	case <-r.quit:
		// already closed
	default:
		close(r.quit)
	}
	r.wg.Wait()
	return nil
}

func (r *MetricsReporter) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()
	r.publish()
	for {
		select {
		case <-r.quit:
			return
		case <-ticker.C:
			r.publish()
		}
	}
}

func (r *MetricsReporter) publish() {
	s := r.pw.Stats()
	r.sink.SetUInt64Value(r.keys.Enqueued, s.Enqueued)
	r.sink.SetUInt64Value(r.keys.Dropped, s.Dropped)
	r.sink.SetUInt64Value(r.keys.CompileSucceeded, s.CompileSucceeded)
	r.sink.SetUInt64Value(r.keys.CompileFailed, s.CompileFailed)
	r.sink.SetUInt64Value(r.keys.SkippedAlreadyCached, s.SkippedAlreadyCached)
	r.sink.SetUInt64Value(r.keys.SkippedDuplicateInFlight, s.SkippedDuplicateInFlight)
	r.sink.SetUInt64Value(r.keys.SkippedFetchFailed, s.SkippedFetchFailed)
	// QueueDepth is a u64 of an int that is always >= 0 by chan-len semantics.
	r.sink.SetUInt64Value(r.keys.QueueDepth, uint64(s.QueueDepth))
}
