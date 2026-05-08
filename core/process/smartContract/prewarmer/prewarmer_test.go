package prewarmer

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

// --- mocks ---

type fakeFetcher struct {
	mu       sync.Mutex
	contents map[string]struct{ hash, code []byte }
	err      error
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		contents: make(map[string]struct{ hash, code []byte }),
	}
}

func (f *fakeFetcher) register(address, hash, code []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contents[string(address)] = struct{ hash, code []byte }{hash: hash, code: code}
}

func (f *fakeFetcher) FetchCode(address []byte) ([]byte, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, nil, f.err
	}
	c, ok := f.contents[string(address)]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	return c.hash, c.code, nil
}

type fakeStore struct {
	mu        sync.Mutex
	cache     map[string][]byte
	hasCalls  int
	saveCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{cache: make(map[string][]byte)}
}

func (s *fakeStore) HasCompiledCode(codeHash []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCalls++
	_, ok := s.cache[string(codeHash)]
	return ok
}

func (s *fakeStore) SaveCompiledCode(codeHash, compiled []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCalls++
	s.cache[string(codeHash)] = compiled
}

type fakeCompiler struct {
	calls   int32
	delay   time.Duration
	failErr error
}

func (c *fakeCompiler) CompileToAOT(code []byte, _ executor.CompilationOptions) ([]byte, error) {
	atomic.AddInt32(&c.calls, 1)
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	if c.failErr != nil {
		return nil, c.failErr
	}
	return append([]byte("AOT:"), code...), nil
}

// capturingCompiler records the most recent CompilationOptions it was called with.
type capturingCompiler struct {
	mu       sync.Mutex
	lastOpts executor.CompilationOptions
}

func (c *capturingCompiler) CompileToAOT(code []byte, opts executor.CompilationOptions) ([]byte, error) {
	c.mu.Lock()
	c.lastOpts = opts
	c.mu.Unlock()
	return append([]byte("AOT:"), code...), nil
}

// fakeMixedCompiler returns an error for any code containing the byte
// sequence "fail"; everything else compiles successfully. This lets a single
// stub stand in for both success and failure paths in stats tests.
type fakeMixedCompiler struct{}

func (c *fakeMixedCompiler) CompileToAOT(code []byte, _ executor.CompilationOptions) ([]byte, error) {
	if bytes.Contains(code, []byte("fail")) {
		return nil, errors.New("synthetic compile error")
	}
	return append([]byte("AOT:"), code...), nil
}

// --- test helpers ---

func wrapSC(t *testing.T, address []byte, scType transaction.SmartContract_SCType) *txcache.WrappedTransaction {
	sc := &transaction.SmartContract{Type: scType, Address: address}
	param, err := anypb.New(sc)
	require.NoError(t, err)
	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Contract: []*transaction.TXContract{
				{Type: transaction.TXContract_SmartContractType, Parameter: param},
			},
		},
	}
	return &txcache.WrappedTransaction{Tx: tx}
}

func wrapNonSC() *txcache.WrappedTransaction {
	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Contract: []*transaction.TXContract{
				{Type: transaction.TXContract_TransferContractType},
			},
		},
	}
	return &txcache.WrappedTransaction{Tx: tx}
}

func waitForCalls(t *testing.T, c *fakeCompiler, want int32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&c.calls) >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	require.Equal(t, want, atomic.LoadInt32(&c.calls), "compiler call count")
}

func mustNew(t *testing.T, args Args) *Prewarmer {
	t.Helper()
	pw, err := New(args)
	require.NoError(t, err)
	return pw
}

// --- tests ---

func TestPrewarmer_New_ValidatesArgs(t *testing.T) {
	t.Parallel()
	_, err := New(Args{CompiledStore: newFakeStore(), Compiler: &fakeCompiler{}})
	require.ErrorIs(t, err, ErrNilCodeFetcher)

	_, err = New(Args{CodeFetcher: newFakeFetcher(), Compiler: &fakeCompiler{}})
	require.ErrorIs(t, err, ErrNilCompiledStore)

	_, err = New(Args{CodeFetcher: newFakeFetcher(), CompiledStore: newFakeStore()})
	require.ErrorIs(t, err, ErrNilCompiler)
}

func TestPrewarmer_HappyPath_CompilesAndCaches(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm-bytes"))
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 2, QueueSize: 8})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))

	waitForCalls(t, cmp, 1, time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()
	require.Equal(t, []byte("AOT:wasm-bytes"), s.cache["hash1"])
	require.Equal(t, 1, s.saveCalls)
}

func TestPrewarmer_SkipsAlreadyCached(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	s.cache["hash1"] = []byte("already")
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&cmp.calls))
}

func TestPrewarmer_SkipsDeploy(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCDeploy))

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&cmp.calls))
}

func TestPrewarmer_SkipsNonSC(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapNonSC())

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&cmp.calls))
}

func TestPrewarmer_SingleFlightPerCodeHash(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	f.register([]byte("addr2"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{delay: 30 * time.Millisecond}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 4, QueueSize: 16})
	pw.Start()
	defer pw.Stop()

	// Two TXs to two addresses that share the same codeHash. The second
	// worker should bail in the in-flight check after the first wins.
	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
	pw.OnTxAdded(nil, wrapSC(t, []byte("addr2"), transaction.SmartContract_SCInvoke))

	time.Sleep(150 * time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&cmp.calls), "should compile only once for shared codeHash")
}

func TestPrewarmer_QueueFullDoesNotBlock(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{delay: 50 * time.Millisecond}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 1})
	pw.Start()
	defer pw.Stop()

	done := make(chan struct{})
	go func() {
		for range 100 {
			pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnTxAdded blocked when queue was full")
	}
}

func TestPrewarmer_StopDrainsWorkers(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 2, QueueSize: 8})
	pw.Start()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
	waitForCalls(t, cmp, 1, time.Second)

	stopped := make(chan struct{})
	go func() {
		pw.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return")
	}
}

func TestPrewarmer_CompileFailureSwallowed(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{failErr: errors.New("boom")}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
	waitForCalls(t, cmp, 1, time.Second)

	s.mu.Lock()
	require.Equal(t, 0, s.saveCalls)
	s.mu.Unlock()
}

func TestPrewarmer_GasScheduleChange_UpdatesOptions(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	captured := &capturingCompiler{}

	pw := mustNew(t, Args{
		CodeFetcher:    f,
		CompiledStore:  s,
		Compiler:       captured,
		Workers:        1,
		QueueSize:      4,
		CompileOptions: executor.CompilationOptions{UnmeteredLocals: 1, MaxMemoryGrow: 1, MaxMemoryGrowDelta: 1},
	})
	pw.Start()
	defer pw.Stop()

	pw.GasScheduleChange(map[string]map[string]uint64{
		"WASMOpcodeCost": {
			"LocalsUnmetered":    99,
			"MaxMemoryGrow":      77,
			"MaxMemoryGrowDelta": 55,
		},
	})

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		captured.mu.Lock()
		got := captured.lastOpts
		captured.mu.Unlock()
		if got.UnmeteredLocals == 99 {
			require.Equal(t, uint64(99), got.UnmeteredLocals)
			require.Equal(t, uint64(77), got.MaxMemoryGrow)
			require.Equal(t, uint64(55), got.MaxMemoryGrowDelta)
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("compiler did not see updated options")
}

func TestPrewarmer_GasScheduleChange_NilWasmCostIsNoop(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{
		CodeFetcher:    f,
		CompiledStore:  s,
		Compiler:       cmp,
		Workers:        1,
		QueueSize:      4,
		CompileOptions: executor.CompilationOptions{UnmeteredLocals: 42},
	})

	pw.GasScheduleChange(map[string]map[string]uint64{}) // no WASMOpcodeCost key

	pw.optsMu.RLock()
	require.Equal(t, uint64(42), pw.opts.UnmeteredLocals)
	pw.optsMu.RUnlock()
}

func TestPrewarmer_Stats_Counters(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr-ok"), []byte("hash-ok"), []byte("wasm-ok"))
	f.register([]byte("addr-fail"), []byte("hash-fail"), []byte("wasm-fail"))
	s := newFakeStore()
	cmp := &fakeMixedCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 2, QueueSize: 16})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr-ok"), transaction.SmartContract_SCInvoke))
	pw.OnTxAdded(nil, wrapSC(t, []byte("addr-fail"), transaction.SmartContract_SCInvoke))
	pw.OnTxAdded(nil, wrapSC(t, []byte("addr-missing"), transaction.SmartContract_SCInvoke))
	pw.OnTxAdded(nil, wrapNonSC()) // not enqueued

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		st := pw.Stats()
		if st.CompileSucceeded == 1 && st.CompileFailed == 1 && st.SkippedFetchFailed == 1 {
			require.Equal(t, uint64(3), st.Enqueued, "non-SC TXs should not enqueue")
			require.Equal(t, uint64(0), st.Dropped)
			require.Equal(t, 16, st.QueueCapacity)
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("stats did not converge: %+v", pw.Stats())
}

func TestPrewarmer_Stats_QueueDropped(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	f.register([]byte("addr1"), []byte("hash1"), []byte("wasm"))
	s := newFakeStore()
	cmp := &fakeCompiler{delay: 50 * time.Millisecond}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 1})
	pw.Start()
	defer pw.Stop()

	for range 50 {
		pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))
	}

	st := pw.Stats()
	require.Greater(t, st.Dropped, uint64(0), "expected drops with queue size 1 + slow compiler + 50 enqueues")
}

func TestPrewarmer_FetchErrorSkipsCompile(t *testing.T) {
	t.Parallel()
	f := newFakeFetcher()
	// No register call → FetchCode returns "not found".
	s := newFakeStore()
	cmp := &fakeCompiler{}

	pw := mustNew(t, Args{CodeFetcher: f, CompiledStore: s, Compiler: cmp, Workers: 1, QueueSize: 4})
	pw.Start()
	defer pw.Stop()

	pw.OnTxAdded(nil, wrapSC(t, []byte("addr1"), transaction.SmartContract_SCInvoke))

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&cmp.calls))
	s.mu.Lock()
	require.Equal(t, 0, s.saveCalls)
	s.mu.Unlock()
}
