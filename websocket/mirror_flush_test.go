package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runHubServer runs StartServer on hub against a private event queue and returns a stop
// function that cancels it and waits for it to return — the path on which the mirror
// warn counters are flushed.
func runHubServer(t *testing.T, hub *SocketHub) (stop func()) {
	t.Helper()
	origQueue := indexer.EventQueue
	indexer.EventQueue = make(chan indexer.Event, 10)
	t.Cleanup(func() { indexer.EventQueue = origQueue })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hub.StartServer(ctx)
		close(done)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			assertReturnsQuickly(t, 5*time.Second, "StartServer did not return after cancellation", func() { <-done })
		})
	}
	// Registered after the queue restore above, so it runs before it (cleanups run last-in
	// first-out): a test that fails before calling stop still joins StartServer first.
	t.Cleanup(stop)
	return stop
}

// TestStartServer_FlushesPendingMirrorFailuresOnShutdown pins the shutdown flush of the
// post-failure counter: a failure inside the warn window after one was already logged
// stays pending, and shutdown reports it instead of leaving it in the counter.
func TestStartServer_FlushesPendingMirrorFailuresOnShutdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	stop := runHubServer(t, hub)

	// First failure: the window is open, so fire() logs and resets the counter.
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	require.Eventually(t, func() bool { return atomic.LoadInt64(&hub.postFailWarn.lastLogged) != 0 }, 2*time.Second, 10*time.Millisecond, "first failure must log")

	// Second failure inside the window: counted, not logged — pending until the next window
	// or the shutdown flush.
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	require.Eventually(t, func() bool { return hub.postFailWarn.count.Get() >= 1 }, 2*time.Second, 10*time.Millisecond, "second failure must stay pending")

	stop()
	assert.Zero(t, hub.postFailWarn.count.Get(), "shutdown must flush the pending failure count")
}

// TestStartServer_FlushesPendingQueueDropsOnShutdown pins the same flush for the queue-drop
// counter: with one worker parked in a slow mirror and a one-slot queue, every further send
// is dropped; the first drop logs, the next stays pending, and shutdown reports it.
func TestStartServer_FlushesPendingQueueDropsOnShutdown(t *testing.T) {
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer ts.Close()
	// Deferred after ts.Close so it runs first: the parked handler is released before the
	// server waits for its handlers to finish.
	defer close(release)

	hub := NewHub(ts.URL, "key", nil, Limits{PostWorkers: 1, PostQueueSize: 1})
	stop := runHubServer(t, hub)

	// One send parks the worker in the handler, one fills the queue; everything after is a
	// drop. Four sends therefore produce at least two drops: one logged, one pending.
	for i := 0; i < 4; i++ {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	}
	require.Eventually(t, func() bool { return atomic.LoadInt64(&hub.queueDropWarn.lastLogged) != 0 }, 2*time.Second, 10*time.Millisecond, "first drop must log")
	require.GreaterOrEqual(t, hub.queueDropWarn.count.Get(), int64(1), "a drop inside the window must stay pending")

	stop()
	assert.Zero(t, hub.queueDropWarn.count.Get(), "shutdown must flush the pending drop count")
}
