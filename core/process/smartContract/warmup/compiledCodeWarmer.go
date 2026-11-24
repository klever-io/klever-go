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
//   - The fork-dependent option is not. The declared-table cap (FixAuditChangesV4) is handed to
//     the executor on every instantiation, from the restoring call's options, and the executor
//     checks it on restore (kvm/vmhost/contexts
//     TestRuntimeContext_CompiledCodeFromBeforeTheTableCapIsRejectedLikeACold). The runtime
//     checks it again on the compiled-code restore path, for a restore the executor accepts
//     (TestRuntimeContext_CompiledCodeRestoreChecksTheTableCapItself).
//   - GasLimit is serialized into Cache() output, but the restoring call's limit is the one
//     enforced (kvm/wasmer2 TestCompiledModuleGasLimitIsAppliedOnRestore), which is why the
//     pool is shared by calls with different gas limits.
//
// Every insert by the warmer goes through the main block processor's
// BlockChainHookImpl.SaveCompiledCodeIfCurrent. It is refused if the cache was cleared since
// compilation started (generation check) or if the fingerprint {opcode costs, V4 flag,
// compilation options}, recomputed under the cache lock, differs from the one the module was
// compiled with. The two guards protect different inputs:
//
//   - The gas schedule is followed by a clear. gasScheduleNotifier.EpochConfirmed stores the
//     new schedule (changeLatestGasSchedule, under its own lock) before it calls any handler,
//     on any epoch change that changes the schedule, backwards included, and
//     vmContainerFactory.GasScheduleChange clears the compiled codes. A module saved before the
//     clear is wiped by it; one saved after it was compiled against the new schedule or fails
//     the generation check.
//   - The fork flags are guarded by the fingerprint recheck at save time only, not by a clear.
//     CheckEpoch is called only by the block processor, so after a start the flags jump from
//     their epoch-0 values to the current epoch in one step. The hook clears only on an exact
//     activation epoch; the VM host also clears when a confirmed epoch crosses one in either
//     direction, including restoreEpochState (core/process/block) moving the epoch back, but
//     not on its first confirmation, at registration. The recheck refuses a module whose flags
//     changed before it was saved; a module cached before the host's first confirmation can
//     outlive a flag change. That is harmless only because nothing flag-dependent is baked
//     into a module, and the table cap is checked on restore.
//
// The guards order the warmer against the main hook only. The transaction simulator's hook
// shares the same compiled-code pool with its own lock and generation (factory/process.go,
// createMetaTxSimulatorProcessor), so its saves and clears are not ordered against the main
// hook's (KLC-3030 tracks giving it its own pool).
//
// A future compilation input that is baked into Cache() output must therefore get its own
// clear on every change before the warmer, or the compiled-code cache at all, may rely on it.
// The fingerprint recheck alone never removes a module that is already cached.
//
// # Never on the block path
//
// The OnAdded handler runs synchronously in the interceptor goroutines: it only parses the
// transaction and makes a non-blocking send. All reads and compiles happen on one worker
// goroutine with its own executor. The cleared handler is a non-blocking send on a channel of
// size one. Account reads take the accounts adapter's exclusive lock, which the block path
// takes for every account operation, so the worker reads only the account's code hash first,
// reads the code only when its module is missing, and leaves readInterval between reads.
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
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
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
	// defaultReadInterval is the least time the worker leaves between two account reads, each
	// of which holds the accounts adapter's exclusive lock: at most 100 reads per second
	defaultReadInterval = 10 * time.Millisecond
	// txPoolCacheID is the only transactions pool shard the node uses
	txPoolCacheID = "0"
	// warmupGasLimit is the GasLimit compilation option used by the warmer. It is a value the
	// cold path itself can pass; the restoring call's own limit is the one enforced.
	warmupGasLimit = uint64(core.MaxGasLimitPerTx)
	// logKeyAddress and logKeyError are the log field names the warmer uses
	logKeyAddress = "address"
	logKeyError   = "error"
)

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
	Enqueued       uint64
	Deduped        uint64
	Dropped        uint64
	Compiled       uint64
	SkippedPresent uint64
	// SkippedNoCode counts addresses without an account or without code
	SkippedNoCode uint64
	// SkippedUnsupported counts modules declaring a start function
	SkippedUnsupported uint64
	DiscardedStale     uint64
	// Errors counts real failures: unreadable state, undecodable code, compile errors, an
	// unusable gas schedule, and recovered panics
	Errors      uint64
	Rescans     uint64
	CompileTime time.Duration
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
	started   atomic.Bool

	// owned by the worker goroutine
	exec         executor.Executor
	backlog      [][]byte
	readInterval time.Duration
	lastRead     time.Time

	numEnqueued           atomic.Uint64
	numDeduped            atomic.Uint64
	numDropped            atomic.Uint64
	numCompiled           atomic.Uint64
	numSkippedPresent     atomic.Uint64
	numSkippedNoCode      atomic.Uint64
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

	// OnAdded only sees transactions added from now on: queue the invokes already in the pool
	w.TriggerRescan()
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
		readInterval:  defaultReadInterval,
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
	if w.isClosing() {
		return
	}
	w.started.Store(true)
	go w.run()
}

// onTxAdded is the transactions pool OnAdded handler. It runs synchronously in the
// interceptor goroutines, so it must never block, compile or panic.
func (w *compiledCodeWarmer) onTxAdded(_ []byte, value interface{}) {
	if w.isClosing() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Debug("compiledCodeWarmer.onTxAdded: recovered", "panic", r)
		}
	}()

	for _, address := range invokedContracts(value) {
		w.enqueue(address)
	}
}

// txFromValue returns the transaction a pool entry holds, or nil for anything else, including
// a wrapped transaction of another type: such an entry simply has nothing to warm.
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

// invokedContracts returns the smart-contract addresses the invokes of a pool entry call
func invokedContracts(value interface{}) [][]byte {
	tx := txFromValue(value)
	if tx == nil {
		return nil
	}

	var addresses [][]byte
	for _, contract := range tx.GetRawData().GetContract() {
		if contract == nil || contract.Type != transaction.TXContract_SmartContractType {
			continue
		}
		sc, err := contract.GetSmartContract()
		if err != nil {
			log.Debug("compiledCodeWarmer: cannot read the smart contract of a pool transaction", logKeyError, err)
			continue
		}
		// deploys are not in state yet; only already-deployed code can be warmed
		if sc.GetType() != transaction.SmartContract_SCInvoke {
			continue
		}
		// any other address would cost a state read and cannot hold code
		address := sc.GetAddress()
		if !core.IsSmartContractAddress(address) {
			continue
		}

		addresses = append(addresses, address)
	}

	return addresses
}

func (w *compiledCodeWarmer) enqueue(address []byte) {
	has, _ := w.recent.HasOrAdd(address, struct{}{}, 0)
	if has {
		w.numDeduped.Add(1)
		return
	}

	select {
	case w.queue <- bytes.Clone(address):
		w.numEnqueued.Add(1)
	default:
		w.numDropped.Add(1)
		// forget it, so a later transaction to the same contract can still be warmed
		w.forget(address)
	}
}

// addToBacklog adds address to the worker's own backlog, which a rescan fills: the rescan runs
// on the queue's only consumer, so it cannot wait for room in the queue. Worker only.
func (w *compiledCodeWarmer) addToBacklog(address []byte) {
	has, _ := w.recent.HasOrAdd(address, struct{}{}, 0)
	if has {
		w.numDeduped.Add(1)
		return
	}

	w.backlog = append(w.backlog, bytes.Clone(address))
	w.numEnqueued.Add(1)
}

// forget removes address from the dedup set, so a later transaction or rescan warms it again
func (w *compiledCodeWarmer) forget(address []byte) {
	w.recent.Remove(address)
}

// TriggerRescan asks the worker to re-enqueue the contracts called by the transactions
// currently in the pool. Requests are coalesced and the call never blocks.
func (w *compiledCodeWarmer) TriggerRescan() {
	if w.isClosing() {
		return
	}

	select {
	case w.rescanCh <- struct{}{}:
	default:
	}
}

func (w *compiledCodeWarmer) run() {
	defer close(w.done)

	for !w.isClosing() {
		w.step()
	}
}

// step handles one rescan request or warms one address. Queued addresses, from transactions
// added since, go before a rescan's backlog, which is worked only while the queue is empty.
func (w *compiledCodeWarmer) step() {
	if len(w.backlog) == 0 {
		select {
		case <-w.closing:
		case <-w.rescanCh:
			w.safely(w.rescan)
		case address := <-w.queue:
			w.safely(func() { w.warm(address) })
		}
		return
	}

	select {
	case <-w.closing:
	case <-w.rescanCh:
		w.safely(w.rescan)
	case address := <-w.queue:
		w.safely(func() { w.warm(address) })
	default:
		address := w.backlog[0]
		w.backlog[0] = nil
		w.backlog = w.backlog[1:]
		w.safely(func() { w.warm(address) })
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

// rescan replaces the backlog with the invoked contracts of every transaction in the pool.
// The dedup set is cleared first: after a cache clear every contract is cold again.
func (w *compiledCodeWarmer) rescan() {
	defer w.publishMetrics()

	w.numRescans.Add(1)
	w.recent.Clear()
	w.backlog = nil

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
		for _, address := range invokedContracts(value) {
			w.addToBacklog(address)
		}
	}
}

// warm compiles the module of the contract at address and stores it, unless it is cached
// already or the cache inputs changed while it compiled. Every failure only skips the
// contract: its next call compiles cold, exactly as without the warmer. Failures that can
// pass (unreadable state, a stale cache) forget the address, so a later transaction or rescan
// retries it; those that are a property of the address or the code (no account or no code
// yet, a start function, undecodable code, a compile error) do not.
func (w *compiledCodeWarmer) warm(address []byte) {
	defer w.publishMetrics()

	codeHash, code, ok := w.loadUncachedCode(address)
	if !ok {
		return
	}

	err := contexts.VerifyNoStartSection(code)
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
		w.forget(address)
		log.Debug("compiledCodeWarmer: cannot build the compilation inputs", logKeyError, err)
		return
	}
	w.exec.SetOpcodeCosts(&compiledFor.opcodes)

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
		w.forget(address)
		log.Debug("compiledCodeWarmer: discarded module compiled for a stale cache", logKeyAddress, address)
		return
	}

	w.numCompiled.Add(1)
	log.Debug("compiledCodeWarmer: module precompiled", logKeyAddress, address, "codeHash", codeHash, "duration", duration)
}

// loadUncachedCode returns the hash the runtime keys the contract's module on and its code,
// unless the module is cached already or the code cannot be read. It reads the account's code
// hash first and the code only when that module is missing, so an address whose module is
// cached costs one small read under the accounts lock.
func (w *compiledCodeWarmer) loadUncachedCode(address []byte) ([]byte, []byte, bool) {
	if !w.waitForRead() {
		return nil, nil, false
	}
	accountCodeHash, err := w.accounts.GetAccountCodeHash(address)
	if err != nil || len(accountCodeHash) == 0 {
		w.skipUnreadable(address, err)
		return nil, nil, false
	}
	if w.isCached(accountCodeHash) {
		return nil, nil, false
	}

	if !w.waitForRead() {
		return nil, nil, false
	}
	code, err := w.accounts.GetAccountCode(address)
	if err != nil || len(code) == 0 {
		w.skipUnreadable(address, err)
		return nil, nil, false
	}

	// The account's code hash is the hash of its code under the same hasher (accountsDB.go
	// saveCode, runtime.go startWasmerInstance). The code can still have changed between the
	// two reads, so the module is keyed on the hash of the code actually read.
	codeHash := w.hasher.Compute(string(code))
	if !bytes.Equal(codeHash, accountCodeHash) && w.isCached(codeHash) {
		return nil, nil, false
	}

	return codeHash, code, true
}

// skipUnreadable counts an address whose code could not be read. No account and no code are
// ordinary inputs: the address stays in the dedup set, so pool transactions to an address
// without a contract do not cost an accounts read each. A contract deployed there later is
// cached by its deploy, and a rescan clears the set. Any other error is a failure, and the
// address is forgotten so a later transaction retries it.
func (w *compiledCodeWarmer) skipUnreadable(address []byte, err error) {
	if err == nil || errors.Is(err, common.ErrAccNotFound) {
		w.numSkippedNoCode.Add(1)
		log.Trace("compiledCodeWarmer: no code to warm", logKeyAddress, address, logKeyError, err)
		return
	}

	w.forget(address)
	w.numErrors.Add(1)
	log.Debug("compiledCodeWarmer: cannot read contract code", logKeyAddress, address, logKeyError, err)
}

func (w *compiledCodeWarmer) isCached(codeHash []byte) bool {
	found, _ := w.compiledCode.GetCompiledCode(codeHash)
	if found {
		w.numSkippedPresent.Add(1)
	}

	return found
}

// waitForRead spaces account reads by readInterval and reports false, without waiting out the
// interval, once the warmer is closing. Worker only.
func (w *compiledCodeWarmer) waitForRead() bool {
	wait := w.readInterval - time.Since(w.lastRead)
	if wait > 0 && !w.lastRead.IsZero() {
		timer := time.NewTimer(wait)
		select {
		case <-w.closing:
			timer.Stop()
			return false
		case <-timer.C:
		}
	}

	w.lastRead = time.Now()
	return true
}

func (w *compiledCodeWarmer) compile(code []byte, options executor.CompilationOptions) ([]byte, error) {
	instance, err := w.exec.NewInstanceWithOptions(code, options)
	if err != nil {
		return nil, err
	}
	defer instance.Clean()

	return instance.Cache()
}

// currentFingerprint builds the compilation inputs in force now. The save's recheck also calls
// it, under the compiled-code read lock.
func (w *compiledCodeWarmer) currentFingerprint() (fingerprint, error) {
	gasCost, err := config.CreateGasConfig(w.gasSchedule.LatestGasSchedule())
	if err != nil {
		return fingerprint{}, err
	}
	if gasCost.WASMOpcodeCost == nil {
		return fingerprint{}, errors.New("gas schedule without WASM opcode costs")
	}

	options := contexts.BuildCompilationOptions(
		gasCost,
		w.forkFlags.FixAuditChangesV4(),
		warmupGasLimit,
	)

	return fingerprint{
		opcodes: *gasCost.WASMOpcodeCost,
		options: options,
	}, nil
}

// Counters returns a snapshot of the activity counters
func (w *compiledCodeWarmer) Counters() Counters {
	return Counters{
		Enqueued:           w.numEnqueued.Load(),
		Deduped:            w.numDeduped.Load(),
		Dropped:            w.numDropped.Load(),
		Compiled:           w.numCompiled.Load(),
		SkippedPresent:     w.numSkippedPresent.Load(),
		SkippedNoCode:      w.numSkippedNoCode.Load(),
		SkippedUnsupported: w.numSkippedUnsupported.Load(),
		DiscardedStale:     w.numDiscardedStale.Load(),
		Errors:             w.numErrors.Load(),
		Rescans:            w.numRescans.Load(),
		CompileTime:        time.Duration(w.compileTimeNs.Load()),
	}
}

func (w *compiledCodeWarmer) publishMetrics() {
	counters := w.Counters()
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupEnqueued, counters.Enqueued)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupDropped, counters.Dropped)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupCompiled, counters.Compiled)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupSkippedPresent, counters.SkippedPresent)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupSkippedNoCode, counters.SkippedNoCode)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupSkippedUnsupported, counters.SkippedUnsupported)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupDiscardedStale, counters.DiscardedStale)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupErrors, counters.Errors)
	w.statusHandler.SetUInt64Value(core.MetricSCWarmupCompileTimeMs, uint64(counters.CompileTime.Milliseconds())) // #nosec G115 - durations are non-negative
}

// Close stops the worker and waits for it to exit. It is idempotent.
func (w *compiledCodeWarmer) Close() error {
	w.closeOnce.Do(func() {
		close(w.closing)
		if w.started.Load() {
			<-w.done
		}
		// The instances are cleaned after every compile. The executor's own native allocation
		// is not freed: there is no Go binding for vm_exec_executor_destroy, so one small
		// allocation per warmer outlives it.
	})

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (w *compiledCodeWarmer) IsInterfaceNil() bool {
	return w == nil
}
