package websocket

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartServer_FlushesPendingMirrorFailuresOnShutdown pins the shutdown flush of the
// post-failure counter: a failure inside the warn window after one was already logged
// stays pending, and shutdown logs it as the "(final)" summary instead of leaving it in
// the counter.
func TestStartServer_FlushesPendingMirrorFailuresOnShutdown(t *testing.T) {
	finalCount := captureMirrorField(t, "failed to post to mirror (final)", "failedCount")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	env := startServerEnvWith(t, hub)

	// First failure: the window is open, so fire() logs and resets the counter.
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	require.Eventually(t, func() bool { return atomic.LoadInt64(&hub.postFailWarn.lastLogged) != 0 }, 2*time.Second, 10*time.Millisecond, "first failure must log")

	// Second failure inside the window: counted, not logged — pending until the next window
	// or the shutdown flush.
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	require.Eventually(t, func() bool { return hub.postFailWarn.count.Get() >= 1 }, 2*time.Second, 10*time.Millisecond, "second failure must stay pending")

	env.teardown()
	assert.Zero(t, hub.postFailWarn.count.Get(), "shutdown must flush the pending failure count")
	count, logged := finalCount()
	require.True(t, logged, "shutdown must log the pending failure count")
	assert.Equal(t, "1", count)
}

// TestStartServer_FlushesPendingQueueDropsOnShutdown pins the same flush for the queue-drop
// counter: with one worker parked in a slow mirror and a one-slot queue, every further send
// is dropped; the first drop logs, the next stays pending, and shutdown logs it.
func TestStartServer_FlushesPendingQueueDropsOnShutdown(t *testing.T) {
	finalCount := captureMirrorField(t, "mirror queue full, dropping events (final)", "droppedCount")

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
	env := startServerEnvWith(t, hub)

	// One send parks the worker in the handler, one fills the queue; everything after is a
	// drop. Four sends therefore produce at least two drops: one logged, one pending.
	for i := 0; i < 4; i++ {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	}
	require.Eventually(t, func() bool { return atomic.LoadInt64(&hub.queueDropWarn.lastLogged) != 0 }, 2*time.Second, 10*time.Millisecond, "first drop must log")
	require.GreaterOrEqual(t, hub.queueDropWarn.count.Get(), int64(1), "a drop inside the window must stay pending")

	env.teardown()
	assert.Zero(t, hub.queueDropWarn.count.Get(), "shutdown must flush the pending drop count")
	count, logged := finalCount()
	require.True(t, logged, "shutdown must log the pending drop count")
	assert.NotEqual(t, "0", count)
	assert.NotEmpty(t, count)
}
