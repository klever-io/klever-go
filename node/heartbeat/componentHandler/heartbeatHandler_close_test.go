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

// countHeartbeatGoroutines counts live goroutines whose stack contains marker.
// debug=2 prints one stack per goroutine; debug=1 would aggregate identical
// stacks into a single entry, hiding new goroutines behind existing ones.
func countHeartbeatGoroutines(t *testing.T, marker string) int {
	t.Helper()
	var buf strings.Builder
	require.NoError(t, pprof.Lookup("goroutine").WriteTo(&buf, 2))
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

	// Wait until the monitor goroutine is visible in pprof.
	require.Eventually(t, func() bool {
		runtime.Gosched()
		return countHeartbeatGoroutines(t, ".runRefreshLoop(") >= 1
	}, 2*time.Second, 10*time.Millisecond,
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

	// Wait until pprof reflects the goroutine exit.
	require.Eventually(t, func() bool {
		runtime.GC()
		runtime.Gosched()
		return countHeartbeatGoroutines(t, ".runRefreshLoop(") == 0
	}, 2*time.Second, 10*time.Millisecond,
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
