package componentHandler

import (
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type strBuf struct{ b *strings.Builder }

func (w *strBuf) Write(p []byte) (int, error) { w.b.Write(p); return len(p), nil }

func countHeartbeatGoroutines(t *testing.T, marker string) int {
	t.Helper()
	var buf strings.Builder
	require.NoError(t, pprof.Lookup("goroutine").WriteTo(&strBuf{&buf}, 1))
	return strings.Count(buf.String(), marker)
}

// Verifies the full production shutdown path:
// HeartbeatHandler.Close() -> monitor.Close() -> stops background goroutine cleanly.
func TestHeartbeatHandler_CloseStopsMonitorGoroutine(t *testing.T) {
	// No t.Parallel: depends on global goroutine count.

	arg := createMockArgument()
	hbh, err := NewHeartbeatHandler(arg)
	assert.Nil(t, err)
	require.NotNil(t, hbh.Monitor())

	// Let the monitor goroutine actually start and park in its select.
	time.Sleep(100 * time.Millisecond)
	runtime.Gosched()

	monGoroutinesBefore := countHeartbeatGoroutines(t, "startValidatorProcessing")
	t.Logf("monitor goroutines BEFORE close: %d", monGoroutinesBefore)
	require.GreaterOrEqual(t, monGoroutinesBefore, 1,
		"expected at least one monitor goroutine after construction")

	// Call Close with a watchdog so we fail loudly if Wait() blocks forever.
	closeDone := make(chan error, 1)
	go func() { closeDone <- hbh.Close() }()

	select {
	case err := <-closeDone:
		assert.Nil(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("hbh.Close did not return within 5s — monitor.Close() blocked on wg.Wait")
	}

	// Allow scheduler to reflect the exit in pprof.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	runtime.Gosched()

	monGoroutinesAfter := countHeartbeatGoroutines(t, "startValidatorProcessing")
	t.Logf("monitor goroutines AFTER close: %d", monGoroutinesAfter)

	assert.Equal(t, 0, monGoroutinesAfter,
		"monitor goroutine should be gone after HeartbeatHandler.Close")
}

// Locks in that HeartbeatHandler.Close() is idempotent. The implementation has no
// explicit guard — it relies on cancelFunc, <-senderDone (closed channel), and
// monitor.Close (sync.Once) all being safe to repeat. Regressions in any of those
// would surface here as a panic.
func TestHeartbeatHandler_CloseIsIdempotent(t *testing.T) {
	arg := createMockArgument()
	hbh, err := NewHeartbeatHandler(arg)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		require.NoError(t, hbh.Close())
		require.NoError(t, hbh.Close())
		require.NoError(t, hbh.Close())
	})
}

// Locks in safety under concurrent shutdown — e.g. signal handler and closer
// loop racing to Close the same handler.
func TestHeartbeatHandler_ConcurrentClose(t *testing.T) {
	arg := createMockArgument()
	hbh, err := NewHeartbeatHandler(arg)
	require.NoError(t, err)

	const concurrency = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- hbh.Close()
		}()
	}
	close(start)

	doneAll := make(chan struct{})
	go func() { wg.Wait(); close(doneAll) }()

	select {
	case <-doneAll:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent HeartbeatHandler.Close did not converge within 5s")
	}

	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
