package process_test

import (
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/node/heartbeat/mock"
	"github.com/klever-io/klever-go/node/heartbeat/storage"
	"github.com/stretchr/testify/require"
)

// countMonitorGoroutines counts how many goroutines are currently parked
// inside startValidatorProcessing (identified by the function name in their stack).
func countMonitorGoroutines(t *testing.T) int {
	t.Helper()
	var buf strings.Builder
	if err := pprof.Lookup("goroutine").WriteTo(&logWriter{&buf}, 1); err != nil {
		t.Fatalf("pprof dump failed: %v", err)
	}
	return strings.Count(buf.String(), "startValidatorProcessing")
}

type logWriter struct{ b *strings.Builder }

func (w *logWriter) Write(p []byte) (int, error) {
	w.b.Write(p)
	return len(p), nil
}

func TestMonitor_CloseStopsGoroutine(t *testing.T) {
	// No t.Parallel: depends on global goroutine count.

	storer, err := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	require.NoError(t, err)
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	before := countMonitorGoroutines(t)
	t.Logf("monitor goroutines BEFORE create: %d", before)

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	// Wait until the new goroutine is visible in pprof.
	require.Eventually(t, func() bool {
		runtime.Gosched()
		return countMonitorGoroutines(t) > before
	}, 2*time.Second, 10*time.Millisecond,
		"expected new startValidatorProcessing goroutine to appear")

	// Now call Close and verify the goroutine actually exits.
	closeDone := make(chan error, 1)
	go func() { closeDone <- mon.Close() }()

	select {
	case err := <-closeDone:
		require.NoError(t, err, "Close returned error")
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s — goroutine leak / Wait blocked forever")
	}

	// Wait until pprof reflects the goroutine exit. Use non-increase rather than
	// strict equality to tolerate other tests' goroutines on the global count.
	require.Eventually(t, func() bool {
		runtime.GC()
		runtime.Gosched()
		return countMonitorGoroutines(t) <= before
	}, 2*time.Second, 10*time.Millisecond,
		"goroutine did not exit on Close")
}

func TestMonitor_CloseDoesNotHangOnFreshMonitor(t *testing.T) {
	// Verify that Close on a freshly-created Monitor (before any ticker fire)
	// returns promptly and does not block on wg.Wait().
	t.Parallel()

	storer, err := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	require.NoError(t, err)
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	closeErr := make(chan error, 1)
	go func() { closeErr <- mon.Close() }()

	select {
	case err := <-closeErr:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung even before any ticker fire")
	}
}

func TestMonitor_CloseIsIdempotent(t *testing.T) {
	// Verify that calling Close() multiple times does NOT panic
	// (close-of-closed-channel) and returns nil on every call.
	t.Parallel()

	storer, err := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	require.NoError(t, err)
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	require.NotPanics(t, func() {
		require.NoError(t, mon.Close())
		require.NoError(t, mon.Close())
		require.NoError(t, mon.Close())
	}, "Close must be idempotent and not panic on repeated calls")
}

func TestMonitor_ConcurrentClose(t *testing.T) {
	// Fire many goroutines all racing to Close the same Monitor.
	// Validates the sync.Once guard works correctly under contention.
	t.Parallel()

	storer, err := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	require.NoError(t, err)
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	const concurrency = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- mon.Close()
		}()
	}

	close(start)

	doneAll := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneAll)
	}()

	select {
	case <-doneAll:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close did not converge within 5s")
	}

	close(errs)
	for err := range errs {
		require.NoError(t, err, "Close must return nil for every concurrent caller")
	}
}

func TestMonitor_NoGoroutineLeakAcrossManyCreates(t *testing.T) {
	// Create and Close 50 monitors; verify no accumulation of startValidatorProcessing goroutines.
	// No t.Parallel: depends on global goroutine count.

	storer, err := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	require.NoError(t, err)
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	before := countMonitorGoroutines(t)

	const cycles = 50
	var wg sync.WaitGroup
	closeErrs := make(chan error, cycles)
	for i := 0; i < cycles; i++ {
		mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)
		wg.Add(1)
		go func() {
			defer wg.Done()
			closeErrs <- mon.Close()
		}()
	}
	wg.Wait()
	close(closeErrs)
	for err := range closeErrs {
		require.NoError(t, err)
	}

	// Wait until the global count drops back to baseline (non-increase).
	require.Eventually(t, func() bool {
		runtime.GC()
		runtime.Gosched()
		return countMonitorGoroutines(t) <= before
	}, 2*time.Second, 10*time.Millisecond,
		"goroutine leak detected after %d create/close cycles", cycles)
}
