// Package warmup precompiles the wasm modules of smart contracts that pending transactions
// call, so the first call after a node start or a compiled-code cache clear restores a
// module instead of compiling it inside the SC execution timeout (KLC-3001).
//
// # Consensus safety
//
// A call served from a precompiled module must behave exactly like a cold call. What a module
// carries from the moment it was compiled decides what the warmer must guard:
//
//   - Opcode costs are baked into Cache() output: a module keeps charging the costs it was
//     compiled under (kvm/wasmer2 TestCompiledModuleKeepsCompileTimeOpcodeCosts). They are the
//     only input known to be baked in.
//   - The fork-dependent options are not. The declared-table cap (FixAuditChangesV4) and the
//     per-element table.init price (FixAuditChangesV5) are handed to the executor on every
//     instantiation, from the restoring call's options (kvm/vmhost/hosttest
//     TestGasUsed_TableInit_PriceIsAppliedAtInstantiationNotCompiledIn, and the comments on
//     the table cap in kvm/vmhost/contexts).
//   - GasLimit is serialized into Cache() output, but the restoring call's limit is the one
//     enforced (kvm/wasmer2 TestCompiledModuleGasLimitIsAppliedOnRestore), which is why the
//     pool is shared by calls with different gas limits.
//
// Every insert goes through BlockChainHookImpl.SaveCompiledCodeIfCurrent. It is refused if the
// cache was cleared since compilation started (generation check) or if the fingerprint
// {opcode costs, V4/V5 flags, compilation options}, recomputed under the cache lock, differs
// from the one the module was compiled with. The two guards protect different inputs:
//
//   - The gas schedule is followed by a clear. gasScheduleNotifier.EpochConfirmed stores the
//     new schedule (changeLatestGasSchedule, under its own lock) before it calls any handler,
//     on any epoch change that changes the schedule, backwards included, and
//     vmContainerFactory.GasScheduleChange clears the compiled codes. A module saved before the
//     clear is wiped by it; one saved after it was compiled against the new schedule or fails
//     the generation check.
//   - The fork flags are guarded by the fingerprint recheck at save time only, not by a clear.
//     CheckEpoch is called only by the block processor, so after a start the flags jump from
//     their epoch-0 values to the current epoch in one step; the hook clears only on an exact
//     activation epoch; and restoreEpochState (core/process/block) moves the epoch back without
//     a clear. The recheck refuses a module whose flags changed before it was saved, but a
//     module saved earlier stays cached across the change. That is harmless only because
//     nothing flag-dependent is baked into a module.
//
// A future compilation input that is baked into Cache() output must therefore get its own
// clear on every change before the warmer, or the compiled-code cache at all, may rely on it.
// The fingerprint recheck alone never removes a module that is already cached.
//
// # Never on the block path
//
// The OnAdded handler runs synchronously in the interceptor goroutines: it only parses the
// transaction and makes a non-blocking send. All reads and compiles happen on one worker
// goroutine with its own executor. The cleared handler and the rescan trigger are
// non-blocking sends on a channel of size one.
//
// # Lock order
//
// The fingerprint recheck runs under the hook's compiled-code read lock and takes the gas
// schedule notifier's read lock. The notifier holds its write lock while it calls a newly
// registered GasScheduleSubscribeHandler, and the vmContainerFactory handler clears the hook.
// The warmer must therefore be started only after the main vmContainerFactory.Create() has
// registered its handler.
package warmup

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/contexts"
	"github.com/klever-io/klever-go/kvm/wasmer2"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("process/smartcontract/warmup")

const (
	// defaultQueueSize bounds the contract addresses waiting to be warmed; more are dropped
	defaultQueueSize = 1024
	// defaultRecentAddressesSize bounds the set used to skip addresses seen recently
	defaultRecentAddressesSize = 4096
	// txPoolCacheID is the only transactions pool shard the node uses
	txPoolCacheID = "0"
	// warmupGasLimit is the GasLimit compilation option used by the warmer. It is a value the
	// cold path itself can pass; the restoring call's own limit is the one enforced.
	warmupGasLimit = uint64(core.MaxGasLimitPerTx)
	// logKeyAddress and logKeyError are the log field names the warmer uses
	logKeyAddress = "address"
	logKeyError   = "error"
)

const (
	metricEnqueued       = core.MetricSCWarmupEnqueued
	metricDropped        = core.MetricSCWarmupDropped
	metricCompiled       = core.MetricSCWarmupCompiled
	metricSkippedPresent = core.MetricSCWarmupSkippedPresent
	metricDiscardedStale = core.MetricSCWarmupDiscardedStale
	metricErrors         = core.MetricSCWarmupErrors
	metricCompileTimeMs  = core.MetricSCWarmupCompileTimeMs
)

var errNoCode = errors.New("account has no code")

// ArgsCompiledCodeWarmer holds the arguments of NewCompiledCodeWarmer
type ArgsCompiledCodeWarmer struct {
	TxPool        TxPool
	Accounts      AccountsReader
	CompiledCode  CompiledCodeCache
	GasSchedule   GasScheduleProvider
	ForkFlags     ForkFlags
	Hasher        hashing.Hasher
	StatusHandler core.AppStatusHandler
	// Executor is optional; when nil the warmer creates its own wasmer2 executor
	Executor executor.Executor
}

// Counters is a snapshot of the warmer activity counters
type Counters struct {
	Enqueued           uint64
	Deduped            uint64
	Dropped            uint64
	Compiled           uint64
	SkippedPresent     uint64
	SkippedUnsupported uint64
	DiscardedStale     uint64
	Errors             uint64
	Rescans            uint64
	CompileTime        time.Duration
}

type fingerprint struct {
	opcodes executor.WASMOpcodeCost
	options executor.CompilationOptions
}

type compiledCodeWarmer struct {
	txPool        TxPool
	accounts      AccountsReader
	compiledCode  CompiledCodeCache
	gasSchedule   GasScheduleProvider
	forkFlags     ForkFlags
	hasher        hashing.Hasher
	statusHandler core.AppStatusHandler

	queue    chan []byte
	rescanCh chan struct{}
	recent   storage.Cacher

	closing   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
	started   atomic.Bool

	// owned by the worker goroutine
	exec         executor.Executor
	execOpcodes  *executor.WASMOpcodeCost
	memoSchedule map[string]map[string]uint64
	memoGasCost  *config.GasCost

	numEnqueued           atomic.Uint64
	numDeduped            atomic.Uint64
	numDropped            atomic.Uint64
	numCompiled           atomic.Uint64
	numSkippedPresent     atomic.Uint64
	numSkippedUnsupported atomic.Uint64
	numDiscardedStale     atomic.Uint64
	numErrors             atomic.Uint64
	numRescans            atomic.Uint64
	compileTimeNs         atomic.Int64
}

// NewCompiledCodeWarmer creates the warmer, registers it on the transactions pool and on the
// compiled-code cache, and starts its worker. It must be called only after the main
// vmContainerFactory.Create(), see the package documentation.
func NewCompiledCodeWarmer(args ArgsCompiledCodeWarmer) (*compiledCodeWarmer, error) {
	w, err := newCompiledCodeWarmer(args, defaultQueueSize, defaultRecentAddressesSize)
	if err != nil {
		return nil, err
	}

	w.start()
	return w, nil
}

func newCompiledCodeWarmer(args ArgsCompiledCodeWarmer, queueSize int, recentSize int) (*compiledCodeWarmer, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	recent, err := lrucache.NewCache(recentSize)
	if err != nil {
		return nil, err
	}

	exec := args.Executor
	if check.IfNil(exec) {
		// Opcode costs are set lazily, from the gas schedule in force when a module is compiled.
		// No VM hooks: the worker only compiles and serializes, it never calls a function.
		exec, err = wasmer2.CreateExecutor()
		if err != nil {
			return nil, fmt.Errorf("%w: cannot create the warm-up executor", err)
		}
	}

	w := &compiledCodeWarmer{
		txPool:        args.TxPool,
		accounts:      args.Accounts,
		compiledCode:  args.CompiledCode,
		gasSchedule:   args.GasSchedule,
		forkFlags:     args.ForkFlags,
		hasher:        args.Hasher,
		statusHandler: args.StatusHandler,
		queue:         make(chan []byte, queueSize),
		rescanCh:      make(chan struct{}, 1),
		recent:        recent,
		closing:       make(chan struct{}),
		done:          make(chan struct{}),
		exec:          exec,
	}

	w.txPool.RegisterOnAdded(w.onTxAdded)
	w.compiledCode.RegisterCompiledCodesClearedHandler(w.TriggerRescan)

	return w, nil
}

func checkArgs(args ArgsCompiledCodeWarmer) error {
	if check.IfNil(args.TxPool) {
		return process.ErrNilTransactionPool
	}
	if check.IfNil(args.Accounts) {
		return process.ErrNilAccountsAdapter
	}
	if check.IfNil(args.CompiledCode) {
		return process.ErrNilBlockChainHook
	}
	if check.IfNil(args.GasSchedule) {
		return process.ErrNilGasSchedule
	}
	if check.IfNil(args.ForkFlags) {
		return process.ErrNilEnableEpochsHandler
	}
	if check.IfNil(args.Hasher) {
		return process.ErrNilHasher
	}
	if check.IfNil(args.StatusHandler) {
		return process.ErrNilAppStatusHandler
	}

	return nil
}

func (w *compiledCodeWarmer) start() {
	if w.closed.Load() {
		return
	}
	w.started.Store(true)
	go w.run()
}

// onTxAdded is the transactions pool OnAdded handler. It runs synchronously in the
// interceptor goroutines, so it must never block, compile or panic.
func (w *compiledCodeWarmer) onTxAdded(_ []byte, value interface{}) {
	if w.closed.Load() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Debug("compiledCodeWarmer.onTxAdded: recovered", "panic", r)
		}
	}()

	w.enqueueInvokesOf(value)
}

func txFromValue(value interface{}) *transaction.Transaction {
	switch v := value.(type) {
	case *txcache.WrappedTransaction:
		if v == nil {
			return nil
		}
		tx, _ := v.Tx.(*transaction.Transaction)
		return tx
	case *transaction.Transaction:
		return v
	default:
		return nil
	}
}

func (w *compiledCodeWarmer) enqueueInvokesOf(value interface{}) {
	tx := txFromValue(value)
	if tx == nil {
		return
	}

	for _, contract := range tx.GetRawData().GetContract() {
		if contract == nil || contract.Type != transaction.TXContract_SmartContractType {
			continue
		}
		sc, err := contract.GetSmartContract()
		if err != nil {
			continue
		}
		// deploys are not in state yet; only already-deployed code can be warmed
		if sc.GetType() != transaction.SmartContract_SCInvoke {
			continue
		}
		address := sc.GetAddress()
		if len(address) == 0 {
			continue
		}

		w.enqueue(address)
	}
}

func (w *compiledCodeWarmer) enqueue(address []byte) {
	has, _ := w.recent.HasOrAdd(address, struct{}{}, 0)
	if has {
		w.numDeduped.Add(1)
		return
	}

	addressCopy := make([]byte, len(address))
	copy(addressCopy, address)

	select {
	case w.queue <- addressCopy:
		w.numEnqueued.Add(1)
	default:
		w.numDropped.Add(1)
		// forget it, so a later transaction to the same contract can still be warmed
		w.recent.Remove(address)
	}
}

// TriggerRescan asks the worker to re-enqueue the contracts called by the transactions
// currently in the pool. Requests are coalesced and the call never blocks.
func (w *compiledCodeWarmer) TriggerRescan() {
	if w.closed.Load() {
		return
	}

	select {
	case w.rescanCh <- struct{}{}:
	default:
	}
}

func (w *compiledCodeWarmer) run() {
	defer close(w.done)

	for {
		select {
		case <-w.closing:
			return
		default:
		}

		select {
		case <-w.closing:
			return
		case <-w.rescanCh:
			w.safely(w.rescan)
		case address := <-w.queue:
			w.safely(func() { w.warm(address) })
		}
	}
}

func (w *compiledCodeWarmer) safely(f func()) {
	defer func() {
		if r := recover(); r != nil {
			w.numErrors.Add(1)
			log.Error("compiledCodeWarmer: recovered", "panic", r)
		}
	}()

	f()
}

func (w *compiledCodeWarmer) isClosing() bool {
	select {
	case <-w.closing:
		return true
	default:
		return false
	}
}

// rescan re-enqueues the invoked contracts of every transaction in the pool. The dedup set
// is cleared first: after a cache clear every contract is cold again.
func (w *compiledCodeWarmer) rescan() {
	defer w.publishMetrics()

	w.numRescans.Add(1)
	w.recent.Clear()

	store := w.txPool.ShardDataStore(txPoolCacheID)
	if check.IfNil(store) {
		return
	}

	for _, key := range store.Keys() {
		if w.isClosing() {
			return
		}
		value, ok := store.Peek(key)
		if !ok {
			continue
		}
		w.enqueueInvokesOf(value)
	}
}

// warm compiles the module of the contract at address and stores it, unless it is cached
// already or the cache inputs changed while it compiled. Every failure only skips the
// contract: its next call compiles cold, exactly as without the warmer.
func (w *compiledCodeWarmer) warm(address []byte) {
	defer w.publishMetrics()

	code, err := w.readCode(address)
	if err != nil {
		w.numErrors.Add(1)
		log.Debug("compiledCodeWarmer: cannot read contract code", logKeyAddress, address, logKeyError, err)
		return
	}

	// the same key the runtime computes for this code (runtime.go startWasmerInstance)
	codeHash := w.hasher.Compute(string(code))
	found, _ := w.compiledCode.GetCompiledCode(codeHash)
	if found {
		w.numSkippedPresent.Add(1)
		return
	}

	err = contexts.VerifyNoStartSection(code)
	if err != nil {
		if errors.Is(err, vmhost.ErrContractHasStartSection) {
			// the start function would run at instantiation, on an executor without VM hooks
			w.numSkippedUnsupported.Add(1)
		} else {
			w.numErrors.Add(1)
		}
		log.Debug("compiledCodeWarmer: contract not warmed", logKeyAddress, address, logKeyError, err)
		return
	}

	generation := w.compiledCode.CompiledCodeGeneration()
	compiledFor, err := w.currentFingerprint()
	if err != nil {
		w.numErrors.Add(1)
		log.Debug("compiledCodeWarmer: cannot build the compilation inputs", logKeyError, err)
		return
	}
	w.ensureExecutorOpcodeCosts(&compiledFor.opcodes)

	startTime := time.Now()
	compiled, err := w.compile(code, compiledFor.options)
	duration := time.Since(startTime)
	w.compileTimeNs.Add(int64(duration))
	if err != nil {
		w.numErrors.Add(1)
		log.Debug("compiledCodeWarmer: compile failed", logKeyAddress, address, logKeyError, err)
		return
	}

	isCurrent := func() bool {
		current, errFingerprint := w.currentFingerprint()
		return errFingerprint == nil && current == compiledFor
	}
	saved := w.compiledCode.SaveCompiledCodeIfCurrent(generation, isCurrent, codeHash, compiled)
	if !saved {
		w.numDiscardedStale.Add(1)
		log.Debug("compiledCodeWarmer: discarded module compiled for a stale cache", logKeyAddress, address)
		return
	}

	w.numCompiled.Add(1)
	log.Debug("compiledCodeWarmer: module precompiled", logKeyAddress, address, "codeHash", codeHash, "duration", duration)
}

func (w *compiledCodeWarmer) readCode(address []byte) ([]byte, error) {
	code, err := w.accounts.GetAccountCode(address)
	if err != nil {
		return nil, err
	}
	if len(code) == 0 {
		return nil, errNoCode
	}

	return code, nil
}

func (w *compiledCodeWarmer) compile(code []byte, options executor.CompilationOptions) ([]byte, error) {
	instance, err := w.exec.NewInstanceWithOptions(code, options)
	if err != nil {
		return nil, err
	}
	defer instance.Clean()

	return instance.Cache()
}

func (w *compiledCodeWarmer) ensureExecutorOpcodeCosts(opcodes *executor.WASMOpcodeCost) {
	if w.execOpcodes != nil && *w.execOpcodes == *opcodes {
		return
	}

	costs := *opcodes
	w.exec.SetOpcodeCosts(&costs)
	w.execOpcodes = &costs
}

// currentFingerprint builds the compilation inputs in force now. It is also called under the
// compiled-code lock, so the costly gas config derivation is memoized by schedule instance
// (see gasCostFor): the notifier replaces its schedule map on every change and never mutates
// it in place.
func (w *compiledCodeWarmer) currentFingerprint() (fingerprint, error) {
	schedule := w.gasSchedule.LatestGasSchedule()
	gasCost, err := w.gasCostFor(schedule)
	if err != nil {
		return fingerprint{}, err
	}

	options := contexts.BuildCompilationOptions(
		gasCost,
		w.forkFlags.FixAuditChangesV4(),
		w.forkFlags.FixAuditChangesV5(),
		warmupGasLimit,
	)

	return fingerprint{
		opcodes: *gasCost.WASMOpcodeCost,
		options: options,
	}, nil
}

// gasCostFor derives the gas costs of schedule, reusing the previous derivation when schedule
// is the very map it was made from. The memo holds that map, not just its address: while it is
// referenced the garbage collector cannot free it, so no later schedule can be allocated at the
// same address and be mistaken for it.
func (w *compiledCodeWarmer) gasCostFor(schedule map[string]map[string]uint64) (*config.GasCost, error) {
	if w.memoGasCost != nil && sameMap(schedule, w.memoSchedule) {
		return w.memoGasCost, nil
	}

	gasCost, err := config.CreateGasConfig(schedule)
	if err != nil {
		return nil, err
	}
	if gasCost.WASMOpcodeCost == nil {
		return nil, errors.New("gas schedule without WASM opcode costs")
	}

	w.memoSchedule = schedule
	w.memoGasCost = gasCost
	return gasCost, nil
}

// sameMap reports whether a and b are the same map instance. Sound only while both are
// referenced, which gasCostFor guarantees by holding the memoized map.
func sameMap(a map[string]map[string]uint64, b map[string]map[string]uint64) bool {
	if a == nil || b == nil {
		return false
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// Counters returns a snapshot of the activity counters
func (w *compiledCodeWarmer) Counters() Counters {
	return Counters{
		Enqueued:           w.numEnqueued.Load(),
		Deduped:            w.numDeduped.Load(),
		Dropped:            w.numDropped.Load(),
		Compiled:           w.numCompiled.Load(),
		SkippedPresent:     w.numSkippedPresent.Load(),
		SkippedUnsupported: w.numSkippedUnsupported.Load(),
		DiscardedStale:     w.numDiscardedStale.Load(),
		Errors:             w.numErrors.Load(),
		Rescans:            w.numRescans.Load(),
		CompileTime:        time.Duration(w.compileTimeNs.Load()),
	}
}

func (w *compiledCodeWarmer) publishMetrics() {
	counters := w.Counters()
	w.statusHandler.SetUInt64Value(metricEnqueued, counters.Enqueued)
	w.statusHandler.SetUInt64Value(metricDropped, counters.Dropped)
	w.statusHandler.SetUInt64Value(metricCompiled, counters.Compiled)
	w.statusHandler.SetUInt64Value(metricSkippedPresent, counters.SkippedPresent)
	w.statusHandler.SetUInt64Value(metricDiscardedStale, counters.DiscardedStale)
	w.statusHandler.SetUInt64Value(metricErrors, counters.Errors+counters.SkippedUnsupported)
	w.statusHandler.SetUInt64Value(metricCompileTimeMs, uint64(counters.CompileTime.Milliseconds())) // #nosec G115 - durations are non-negative
}

// Close stops the worker and waits for it to exit. It is idempotent.
func (w *compiledCodeWarmer) Close() error {
	w.closeOnce.Do(func() {
		w.closed.Store(true)
		close(w.closing)
		if w.started.Load() {
			<-w.done
		}
		// wasmer2 executors hold no resources beyond their instances, which are cleaned
		// after every compile; dropping the reference is all there is to release.
	})

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (w *compiledCodeWarmer) IsInterfaceNil() bool {
	return w == nil
}
