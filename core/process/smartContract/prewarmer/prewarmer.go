package prewarmer

import (
	"sync"
	"sync/atomic"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/storage/txcache"
)

var log = logger.GetOrCreate("process/smartcontract/prewarmer")

// Args bundles the dependencies a Prewarmer needs.
type Args struct {
	CodeFetcher    CodeFetcher
	CompiledStore  CompiledCodeStore
	Compiler       Compiler
	CompileOptions executor.CompilationOptions
	Workers        int
	QueueSize      int
}

// Stats is a snapshot of prewarmer activity counters. Returned by (*Prewarmer).Stats.
// All fields are monotonic counters since process start.
type Stats struct {
	// Enqueued: TXs whose target contract address was successfully queued.
	Enqueued uint64
	// Dropped: TXs that arrived while the queue was full.
	Dropped uint64
	// CompileSucceeded: prewarms that produced AOT bytes and saved them.
	CompileSucceeded uint64
	// CompileFailed: prewarms where the compiler returned an error.
	CompileFailed uint64
	// SkippedAlreadyCached: prewarms that found AOT bytes already in the cache.
	SkippedAlreadyCached uint64
	// SkippedDuplicateInFlight: prewarms that bailed because another worker
	// is already compiling the same codeHash.
	SkippedDuplicateInFlight uint64
	// SkippedFetchFailed: prewarms where FetchCode returned an error or empty
	// code/hash (account not found, no contract code).
	SkippedFetchFailed uint64
	// QueueDepth: instantaneous queue depth at the time Stats() is called.
	QueueDepth int
	// QueueCapacity: configured queue size.
	QueueCapacity int
}

// Prewarmer compiles smart contracts in the background as transactions enter
// the TX pool, so the block processor's first call to a given contract can
// hit the AOT cache instead of paying the full Wasmer JIT cost.
//
// It exposes OnTxAdded to be registered as a TX-pool RegisterOnAdded handler
// and GasScheduleChange to be registered as a GasSchedule notify handler.
// Work is dispatched to a bounded worker pool; if the queue is full the
// prewarm is dropped and the block processor falls back to the JIT path —
// the prewarmer is best-effort and never blocks TX-pool insertion.
type Prewarmer struct {
	fetcher  CodeFetcher
	store    CompiledCodeStore
	compiler Compiler
	workers  int

	// opts is read on every prewarm and replaced under optsMu when the gas
	// schedule changes. Held by value (small struct) so reads are cheap and
	// don't need a lock-free trick.
	optsMu sync.RWMutex
	opts   executor.CompilationOptions

	queue chan []byte
	quit  chan struct{}
	wg    sync.WaitGroup

	mu       sync.Mutex
	inFlight map[string]struct{}

	// Counters (atomic, monotonic).
	cntEnqueued                uint64
	cntDropped                 uint64
	cntCompileSucceeded        uint64
	cntCompileFailed           uint64
	cntSkippedAlreadyCached    uint64
	cntSkippedDuplicateInFlt   uint64
	cntSkippedFetchFailed      uint64
}

// New constructs a Prewarmer. Call Start before registering OnTxAdded.
func New(args Args) (*Prewarmer, error) {
	if args.CodeFetcher == nil {
		return nil, ErrNilCodeFetcher
	}
	if args.CompiledStore == nil {
		return nil, ErrNilCompiledStore
	}
	if args.Compiler == nil {
		return nil, ErrNilCompiler
	}
	if args.Workers <= 0 {
		args.Workers = 4
	}
	if args.QueueSize <= 0 {
		args.QueueSize = 256
	}
	return &Prewarmer{
		fetcher:  args.CodeFetcher,
		store:    args.CompiledStore,
		compiler: args.Compiler,
		opts:     args.CompileOptions,
		workers:  args.Workers,
		queue:    make(chan []byte, args.QueueSize),
		quit:     make(chan struct{}),
		inFlight: make(map[string]struct{}),
	}, nil
}

// Start spawns the configured number of worker goroutines.
func (p *Prewarmer) Start() {
	for range p.workers {
		p.wg.Add(1)
		go p.worker()
	}
}

// Stop signals workers to exit and waits for them to drain.
func (p *Prewarmer) Stop() {
	close(p.quit)
	p.wg.Wait()
}

// Close is an io.Closer alias for Stop, for use in shutdown registries.
// Always returns nil; failures from worker goroutines are already logged.
func (p *Prewarmer) Close() error {
	p.Stop()
	return nil
}

// Stats returns a snapshot of activity counters plus the instantaneous queue
// depth. Safe to call concurrently with Start, Stop, OnTxAdded.
func (p *Prewarmer) Stats() Stats {
	return Stats{
		Enqueued:                 atomic.LoadUint64(&p.cntEnqueued),
		Dropped:                  atomic.LoadUint64(&p.cntDropped),
		CompileSucceeded:         atomic.LoadUint64(&p.cntCompileSucceeded),
		CompileFailed:            atomic.LoadUint64(&p.cntCompileFailed),
		SkippedAlreadyCached:     atomic.LoadUint64(&p.cntSkippedAlreadyCached),
		SkippedDuplicateInFlight: atomic.LoadUint64(&p.cntSkippedDuplicateInFlt),
		SkippedFetchFailed:       atomic.LoadUint64(&p.cntSkippedFetchFailed),
		QueueDepth:               len(p.queue),
		QueueCapacity:            cap(p.queue),
	}
}

// GasScheduleChange implements core.GasScheduleSubscribeHandler. The new
// schedule's WASMOpcodeCost values become the compile options for subsequent
// prewarms; in-flight compiles finish with the previous options. The block
// processor independently wipes compiledScPool / compiledScStorage on
// activation epochs, so any AOT bytes produced under the old options are
// invalidated upstream — we only need to make sure we don't keep producing
// stale bytes after the change.
func (p *Prewarmer) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	wasmCost := gasSchedule["WASMOpcodeCost"]
	if wasmCost == nil {
		return
	}
	p.optsMu.Lock()
	p.opts.UnmeteredLocals = wasmCost["LocalsUnmetered"]
	p.opts.MaxMemoryGrow = wasmCost["MaxMemoryGrow"]
	p.opts.MaxMemoryGrowDelta = wasmCost["MaxMemoryGrowDelta"]
	p.optsMu.Unlock()
}

// IsInterfaceNil supports the project's check.IfNil pattern.
func (p *Prewarmer) IsInterfaceNil() bool {
	return p == nil
}

// OnTxAdded is the txPool.RegisterOnAdded callback. It filters for SC-invoke
// contracts and enqueues each unique recipient address for background
// prewarm. Returns immediately; never blocks on the queue.
func (p *Prewarmer) OnTxAdded(_ []byte, value any) {
	wrapped, ok := value.(*txcache.WrappedTransaction)
	if !ok || wrapped == nil || wrapped.Tx == nil {
		return
	}
	tx, ok := wrapped.Tx.(*transaction.Transaction)
	if !ok {
		return
	}
	for _, c := range tx.GetContracts() {
		if c.GetType() != transaction.TXContract_SmartContractType {
			continue
		}
		sc, err := c.GetSmartContract()
		if err != nil {
			continue
		}
		// Only prewarm invokes. Deploys create the contract so there is no
		// cached code yet, and upgrades change the bytecode mid-call.
		if sc.GetType() != transaction.SmartContract_SCInvoke {
			continue
		}
		address := sc.GetAddress()
		if len(address) == 0 {
			continue
		}
		select {
		case p.queue <- address:
			atomic.AddUint64(&p.cntEnqueued, 1)
		default:
			atomic.AddUint64(&p.cntDropped, 1)
			log.Trace("prewarmer queue full", "address", address)
		}
	}
}

func (p *Prewarmer) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.quit:
			return
		case address := <-p.queue:
			p.prewarm(address)
		}
	}
}

func (p *Prewarmer) prewarm(address []byte) {
	codeHash, code, err := p.fetcher.FetchCode(address)
	if err != nil || len(codeHash) == 0 || len(code) == 0 {
		atomic.AddUint64(&p.cntSkippedFetchFailed, 1)
		return
	}
	if p.store.HasCompiledCode(codeHash) {
		atomic.AddUint64(&p.cntSkippedAlreadyCached, 1)
		return
	}
	// single-flight: avoid double-compiling the same contract from two TXs.
	key := string(codeHash)
	p.mu.Lock()
	if _, busy := p.inFlight[key]; busy {
		p.mu.Unlock()
		atomic.AddUint64(&p.cntSkippedDuplicateInFlt, 1)
		return
	}
	p.inFlight[key] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.inFlight, key)
		p.mu.Unlock()
	}()

	// Re-check: another worker may have just finished while we were queued.
	if p.store.HasCompiledCode(codeHash) {
		atomic.AddUint64(&p.cntSkippedAlreadyCached, 1)
		return
	}

	p.optsMu.RLock()
	opts := p.opts
	p.optsMu.RUnlock()

	cached, err := p.compiler.CompileToAOT(code, opts)
	if err != nil {
		atomic.AddUint64(&p.cntCompileFailed, 1)
		log.Trace("prewarmer compile failed", "err", err, "address", address)
		return
	}
	p.store.SaveCompiledCode(codeHash, cached)
	atomic.AddUint64(&p.cntCompileSucceeded, 1)
}
