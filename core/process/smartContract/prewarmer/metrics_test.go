package prewarmer

import (
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

type recordingSink struct {
	mu     sync.Mutex
	values map[string]uint64
}

func newRecordingSink() *recordingSink {
	return &recordingSink{values: make(map[string]uint64)}
}

func (s *recordingSink) SetUInt64Value(key string, value uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

func (s *recordingSink) get(key string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[key]
}

func testKeys() MetricKeys {
	return MetricKeys{
		Enqueued:                 "enqueued",
		Dropped:                  "dropped",
		CompileSucceeded:         "compile_succeeded",
		CompileFailed:            "compile_failed",
		SkippedAlreadyCached:     "skipped_already_cached",
		SkippedDuplicateInFlight: "skipped_duplicate_inflight",
		SkippedFetchFailed:       "skipped_fetch_failed",
		QueueDepth:               "queue_depth",
		QueueCapacity:            "queue_capacity",
	}
}

func TestMetricsReporter_PublishesCapacityImmediately(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 32})
	pw.Start()
	defer pw.Stop()

	sink := newRecordingSink()
	r := NewMetricsReporter(pw, sink, testKeys(), 10*time.Millisecond)
	r.Start()
	defer func() { _ = r.Close() }()

	// Capacity is published synchronously in Start; no need to wait.
	require.Equal(t, uint64(32), sink.get("queue_capacity"))
}

func TestMetricsReporter_SamplesCounters(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 8})
	pw.Start()
	defer pw.Stop()

	sink := newRecordingSink()
	r := NewMetricsReporter(pw, sink, testKeys(), 5*time.Millisecond)
	r.Start()
	defer func() { _ = r.Close() }()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
	pw.OnTxAdded(nil, wrapSC(t, []byte("addr-missing"), transaction.SmartContract_SCInvoke))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sink.get("compile_succeeded") == 1 && sink.get("skipped_fetch_failed") == 1 {
			require.Equal(t, uint64(2), sink.get("enqueued"))
			require.Equal(t, uint64(0), sink.get("dropped"))
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("metrics did not converge: %+v", sink.values)
}

func TestMetricsReporter_CloseStopsSampling(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	sink := newRecordingSink()
	r := NewMetricsReporter(pw, sink, testKeys(), 5*time.Millisecond)
	r.Start()

	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Close did not return within timeout")
	}

	// Idempotent second close.
	require.NoError(t, r.Close())
}

func TestMetricsReporter_NilGuards(t *testing.T) {
	t.Parallel()

	// nil reporter: Start and Close must be safe.
	var r *MetricsReporter
	r.Start()
	require.NoError(t, r.Close())

	// non-nil reporter with nil sink: Start is a no-op (no panic).
	pw := mustNew(t, Args{
		CodeFetcher:   newFakeFetcher(),
		CompiledStore: newFakeStore(),
		Compiler:      &fakeCompiler{},
		Workers:       1,
		QueueSize:     1,
	})
	r = NewMetricsReporter(pw, nil, testKeys(), 5*time.Millisecond)
	r.Start()
	require.NoError(t, r.Close())
}
