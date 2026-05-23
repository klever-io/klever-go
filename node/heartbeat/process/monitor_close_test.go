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

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	before := countMonitorGoroutines(t)
	t.Logf("monitor goroutines BEFORE create: %d", before)

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	// Give the goroutine a moment to actually start and park in select
	time.Sleep(50 * time.Millisecond)
	runtime.Gosched()

	afterCreate := countMonitorGoroutines(t)
	t.Logf("monitor goroutines AFTER create: %d", afterCreate)

	if afterCreate <= before {
		t.Fatalf("expected new startValidatorProcessing goroutine to appear, before=%d after=%d", before, afterCreate)
	}

	// Now call Close and verify the goroutine actually exits
	closeDone := make(chan error, 1)
	go func() { closeDone <- mon.Close() }()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s — goroutine leak / Wait blocked forever")
	}

	// Give scheduler a tick to reflect the goroutine exit in pprof
	time.Sleep(50 * time.Millisecond)
	runtime.Gosched()

	afterClose := countMonitorGoroutines(t)
	t.Logf("monitor goroutines AFTER close: %d", afterClose)

	if afterClose != before {
		// Dump goroutines to show what's still around
		var dump strings.Builder
		_ = pprof.Lookup("goroutine").WriteTo(&logWriter{&dump}, 1)
		t.Fatalf("goroutine did not exit on Close: before=%d, afterClose=%d\n%s",
			before, afterClose, dump.String())
	}
}

func TestMonitor_CloseDoesNotHangOnFreshMonitor(t *testing.T) {
	// Verify that Close on a freshly-created Monitor (before any ticker fire)
	// returns promptly and does not block on wg.Wait().
	t.Parallel()

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)

	done := make(chan struct{})
	go func() {
		_ = mon.Close()
		close(done)
	}()

	select {
	case <-done:
		// Good — Close completed
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung even before any ticker fire")
	}
}

func TestMonitor_CloseIsIdempotent(t *testing.T) {
	// Verify that calling Close() multiple times does NOT panic
	// (close-of-closed-channel) and returns nil on every call.
	t.Parallel()

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
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

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
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

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	before := countMonitorGoroutines(t)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		mon := createMonitor(storer, genesisTime, unresponsiveDuration, timer)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mon.Close()
		}()
	}
	wg.Wait()

	// Allow scheduler to clean up
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	runtime.Gosched()

	after := countMonitorGoroutines(t)
	t.Logf("monitor goroutines: before=%d after=%d", before, after)

	if after > before {
		t.Fatalf("goroutine leak detected: %d extra goroutines after 50 create/close cycles", after-before)
	}
}
