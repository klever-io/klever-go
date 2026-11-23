package warmup

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/data/transaction"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost/contexts"
	"github.com/klever-io/klever-go/kvm/wasmer2"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const basicFeaturesPath = "../../../../kvm/test/features/basic-features/output/basic-features.wasm"

var errTestAccountNotFound = errors.New("account not found")

// --- fakes -----------------------------------------------------------------------------

// fakeCompiledCodeCache mirrors BlockChainHookImpl's compiled-code semantics: a generation
// bumped by every clear, the guarded save evaluated under the lock the clear takes
// exclusively, and the cleared handlers called after the lock is released.
type fakeCompiledCodeCache struct {
	mut        sync.RWMutex
	generation uint64
	modules    map[string][]byte

	mutHandlers sync.Mutex
	handlers    []func()
}

func newFakeCompiledCodeCache() *fakeCompiledCodeCache {
	return &fakeCompiledCodeCache{modules: make(map[string][]byte)}
}

func (f *fakeCompiledCodeCache) GetCompiledCode(codeHash []byte) (bool, []byte) {
	f.mut.RLock()
	defer f.mut.RUnlock()

	code, ok := f.modules[string(codeHash)]
	return ok, code
}

func (f *fakeCompiledCodeCache) CompiledCodeGeneration() uint64 {
	f.mut.RLock()
	defer f.mut.RUnlock()

	return f.generation
}

func (f *fakeCompiledCodeCache) SaveCompiledCodeIfCurrent(generation uint64, isCurrent func() bool, codeHash []byte, code []byte) bool {
	f.mut.Lock()
	defer f.mut.Unlock()

	if generation != f.generation {
		return false
	}
	if isCurrent != nil && !isCurrent() {
		return false
	}
	f.modules[string(codeHash)] = code
	return true
}

func (f *fakeCompiledCodeCache) RegisterCompiledCodesClearedHandler(handler func()) {
	f.mutHandlers.Lock()
	f.handlers = append(f.handlers, handler)
	f.mutHandlers.Unlock()
}

func (f *fakeCompiledCodeCache) clear() {
	f.mut.Lock()
	f.generation++
	f.modules = make(map[string][]byte)
	f.mut.Unlock()

	f.mutHandlers.Lock()
	handlers := append([]func(){}, f.handlers...)
	f.mutHandlers.Unlock()
	for _, handler := range handlers {
		handler()
	}
}

func (f *fakeCompiledCodeCache) put(codeHash []byte, code []byte) {
	f.mut.Lock()
	f.modules[string(codeHash)] = code
	f.mut.Unlock()
}

func (f *fakeCompiledCodeCache) len() int {
	f.mut.RLock()
	defer f.mut.RUnlock()

	return len(f.modules)
}

func (f *fakeCompiledCodeCache) IsInterfaceNil() bool {
	return f == nil
}

type fakeAccounts struct {
	mut          sync.Mutex
	codes        map[string][]byte
	numGetCalled atomic.Int64
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{codes: make(map[string][]byte)}
}

// setContract registers an account; a nil code registers an account without code
func (f *fakeAccounts) setContract(address []byte, code []byte) {
	f.mut.Lock()
	defer f.mut.Unlock()

	f.codes[string(address)] = code
}

func (f *fakeAccounts) GetAccountCode(address []byte) ([]byte, error) {
	f.numGetCalled.Add(1)

	f.mut.Lock()
	defer f.mut.Unlock()

	code, ok := f.codes[string(address)]
	if !ok {
		return nil, errTestAccountNotFound
	}
	return code, nil
}

func (f *fakeAccounts) IsInterfaceNil() bool {
	return f == nil
}

type fakeTxPool struct {
	mut         sync.Mutex
	handlers    []func(key []byte, value interface{})
	store       storage.Cacher
	requestedID []string
}

func newFakeTxPool(t *testing.T) *fakeTxPool {
	store, err := lrucache.NewCache(1000)
	require.NoError(t, err)
	return &fakeTxPool{store: store}
}

func (f *fakeTxPool) RegisterOnAdded(handler func(key []byte, value interface{})) {
	f.mut.Lock()
	f.handlers = append(f.handlers, handler)
	f.mut.Unlock()
}

func (f *fakeTxPool) ShardDataStore(cacheID string) storage.Cacher {
	f.mut.Lock()
	f.requestedID = append(f.requestedID, cacheID)
	f.mut.Unlock()
	return f.store
}

func (f *fakeTxPool) addTx(hash []byte, tx *transaction.Transaction) {
	f.store.Put(hash, tx, 0)

	f.mut.Lock()
	handlers := append([]func([]byte, interface{}){}, f.handlers...)
	f.mut.Unlock()
	for _, handler := range handlers {
		handler(hash, &txcache.WrappedTransaction{Tx: tx, TxHash: hash})
	}
}

func (f *fakeTxPool) IsInterfaceNil() bool {
	return f == nil
}

type fakeForkFlags struct {
	v4 atomic.Bool
}

func newFakeForkFlags(v4 bool) *fakeForkFlags {
	f := &fakeForkFlags{}
	f.v4.Store(v4)
	return f
}

func (f *fakeForkFlags) FixAuditChangesV4() bool { return f.v4.Load() }
func (f *fakeForkFlags) IsInterfaceNil() bool    { return f == nil }

// hookedExecutor wraps a real executor so a test can act in the middle of a compile.
type hookedExecutor struct {
	executor.Executor
	beforeCompile     func()
	numCompiles       atomic.Int64
	numSetOpcodeCosts atomic.Int64
}

func (h *hookedExecutor) NewInstanceWithOptions(code []byte, options executor.CompilationOptions) (executor.Instance, error) {
	h.numCompiles.Add(1)
	if h.beforeCompile != nil {
		h.beforeCompile()
	}
	return h.Executor.NewInstanceWithOptions(code, options)
}

func (h *hookedExecutor) SetOpcodeCosts(costs *executor.WASMOpcodeCost) {
	h.numSetOpcodeCosts.Add(1)
	h.Executor.SetOpcodeCosts(costs)
}

type warmerFixture struct {
	args     ArgsCompiledCodeWarmer
	cache    *fakeCompiledCodeCache
	accounts *fakeAccounts
	pool     *fakeTxPool
	gas      *notifierMock.GasScheduleNotifierMock
	flags    *fakeForkFlags
	exec     *hookedExecutor
	hasher   *blake2b.Blake2b
}

func newWarmerFixture(t *testing.T) *warmerFixture {
	t.Helper()

	realExec, err := wasmer2.CreateExecutor()
	require.NoError(t, err)

	f := &warmerFixture{
		cache:    newFakeCompiledCodeCache(),
		accounts: newFakeAccounts(),
		pool:     newFakeTxPool(t),
		gas:      notifierMock.NewGasScheduleNotifierMock(kvmConfig.MakeGasMapForTests()),
		flags:    newFakeForkFlags(true),
		exec:     &hookedExecutor{Executor: realExec},
		hasher:   &blake2b.Blake2b{},
	}
	f.args = ArgsCompiledCodeWarmer{
		TxPool:        f.pool,
		Accounts:      f.accounts,
		CompiledCode:  f.cache,
		GasSchedule:   f.gas,
		ForkFlags:     f.flags,
		Hasher:        f.hasher,
		StatusHandler: statusHandler.NewNilStatusHandler(),
		Executor:      f.exec,
	}
	return f
}

// newStopped builds a warmer whose worker is not running, so tests drive it directly.
func (f *warmerFixture) newStopped(t *testing.T, queueSize int) *compiledCodeWarmer {
	t.Helper()

	w, err := newCompiledCodeWarmer(f.args, queueSize, defaultRecentAddressesSize)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	return w
}

func readBasicFeatures(t *testing.T) []byte {
	t.Helper()

	code, err := os.ReadFile(basicFeaturesPath)
	require.NoError(t, err)
	return code
}

func scAddress(id byte) []byte {
	address := make([]byte, 32)
	address[31] = id
	return address
}

func invokeTx(t *testing.T, addresses ...[]byte) *transaction.Transaction {
	t.Helper()

	tx := transaction.NewBaseTransaction([]byte("sender"), 1, nil, 0, 0)
	for _, address := range addresses {
		require.NoError(t, tx.PushContract(transaction.TXContract_SmartContractType, &transaction.SmartContract{
			Type:    transaction.SmartContract_SCInvoke,
			Address: address,
		}))
	}
	return tx
}

func queued(w *compiledCodeWarmer) [][]byte {
	var out [][]byte
	for {
		select {
		case address := <-w.queue:
			out = append(out, address)
		default:
			return out
		}
	}
}

// startSectionModule is the smallest valid module declaring a start function.
func startSectionModule() []byte {
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // header
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type section: () -> ()
		0x03, 0x02, 0x01, 0x00, // function section: one func of type 0
		0x08, 0x01, 0x00, // start section: func 0
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code section: empty body
	}
}

// --- constructor ----------------------------------------------------------------------

func TestNewCompiledCodeWarmer_NilArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(args *ArgsCompiledCodeWarmer)
		err    error
	}{
		{"tx pool", func(a *ArgsCompiledCodeWarmer) { a.TxPool = nil }, process.ErrNilTransactionPool},
		{"accounts", func(a *ArgsCompiledCodeWarmer) { a.Accounts = nil }, process.ErrNilAccountsAdapter},
		{"compiled code", func(a *ArgsCompiledCodeWarmer) { a.CompiledCode = nil }, process.ErrNilBlockChainHook},
		{"gas schedule", func(a *ArgsCompiledCodeWarmer) { a.GasSchedule = nil }, process.ErrNilGasSchedule},
		{"fork flags", func(a *ArgsCompiledCodeWarmer) { a.ForkFlags = nil }, process.ErrNilEnableEpochsHandler},
		{"hasher", func(a *ArgsCompiledCodeWarmer) { a.Hasher = nil }, process.ErrNilHasher},
		{"status handler", func(a *ArgsCompiledCodeWarmer) { a.StatusHandler = nil }, process.ErrNilAppStatusHandler},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newWarmerFixture(t)
			tt.modify(&f.args)

			w, err := NewCompiledCodeWarmer(f.args)
			assert.Nil(t, w)
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestNewCompiledCodeWarmer_RegistersHandlersAndCreatesOwnExecutor(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	f.args.Executor = nil

	w, err := NewCompiledCodeWarmer(f.args)
	require.NoError(t, err)
	require.False(t, w.IsInterfaceNil())
	defer func() { _ = w.Close() }()

	assert.NotNil(t, w.exec)
	assert.Len(t, f.pool.handlers, 1)
	assert.Len(t, f.cache.handlers, 1)
}

func TestNewCompiledCodeWarmer_WarmsInvokesAlreadyInPool(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	key := f.hasher.Compute(string(code))

	// added before the warmer exists, so no OnAdded handler sees it
	f.pool.addTx([]byte("h1"), invokeTx(t, scAddress(1)))

	w, err := NewCompiledCodeWarmer(f.args)
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	require.Eventually(t, func() bool { return w.Counters().Compiled == 1 }, 10*time.Second, 5*time.Millisecond)
	found, _ := f.cache.GetCompiledCode(key)
	assert.True(t, found)
	assert.Equal(t, uint64(1), w.Counters().Rescans)
}

// --- OnAdded handler -----------------------------------------------------------------

func TestCompiledCodeWarmer_HandlerIgnoresAnythingButInvokes(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	transfer := transaction.NewBaseTransaction([]byte("sender"), 1, nil, 0, 0)
	require.NoError(t, transfer.PushContract(transaction.TXContract_TransferContractType, &transaction.TransferContract{
		ToAddress: scAddress(1),
		Amount:    1,
	}))

	deploy := transaction.NewBaseTransaction([]byte("sender"), 1, nil, 0, 0)
	require.NoError(t, deploy.PushContract(transaction.TXContract_SmartContractType, &transaction.SmartContract{
		Type:    transaction.SmartContract_SCDeploy,
		Address: scAddress(2),
	}))

	brokenParam := transaction.NewBaseTransaction([]byte("sender"), 1, nil, 0, 0)
	brokenParam.RawData.Contract = []*transaction.TXContract{
		nil,
		{Type: transaction.TXContract_SmartContractType, Parameter: nil},
	}

	emptyAddress := invokeTx(t, nil)

	values := []interface{}{
		nil,
		"not a transaction",
		(*txcache.WrappedTransaction)(nil),
		&txcache.WrappedTransaction{},
		&txcache.WrappedTransaction{Tx: &transaction.Transaction{}},
		&txcache.WrappedTransaction{Tx: transfer},
		&txcache.WrappedTransaction{Tx: deploy},
		&txcache.WrappedTransaction{Tx: brokenParam},
		&txcache.WrappedTransaction{Tx: emptyAddress},
		&txcache.WrappedTransaction{Tx: invokeTx(t)},
	}
	for _, value := range values {
		require.NotPanics(t, func() { w.onTxAdded([]byte("hash"), value) })
	}

	assert.Empty(t, queued(w))
	assert.Equal(t, uint64(0), w.Counters().Enqueued)
}

func TestCompiledCodeWarmer_HandlerEnqueuesEveryInvokeOfAMultiContractTx(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	tx := invokeTx(t, scAddress(1), scAddress(2))
	require.NoError(t, tx.PushContract(transaction.TXContract_SmartContractType, &transaction.SmartContract{
		Type:    transaction.SmartContract_SCDeploy,
		Address: scAddress(3),
	}))
	require.NoError(t, tx.PushContract(transaction.TXContract_TransferContractType, &transaction.TransferContract{
		ToAddress: scAddress(4),
		Amount:    1,
	}))

	f.pool.addTx([]byte("hash"), tx)

	assert.Equal(t, [][]byte{scAddress(1), scAddress(2)}, queued(w))
	assert.Equal(t, uint64(2), w.Counters().Enqueued)
}

func TestCompiledCodeWarmer_HandlerDedupsRecentAddresses(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	f.pool.addTx([]byte("h1"), invokeTx(t, scAddress(1)))
	f.pool.addTx([]byte("h2"), invokeTx(t, scAddress(1), scAddress(1)))
	f.pool.addTx([]byte("h3"), invokeTx(t, scAddress(2)))

	assert.Equal(t, [][]byte{scAddress(1), scAddress(2)}, queued(w))
	counters := w.Counters()
	assert.Equal(t, uint64(2), counters.Enqueued)
	assert.Equal(t, uint64(2), counters.Deduped)
}

func TestCompiledCodeWarmer_HandlerNeverBlocksWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 2)

	txs := make([]*transaction.Transaction, 0, 50)
	for i := byte(1); i <= 50; i++ {
		txs = append(txs, invokeTx(t, scAddress(i)))
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i, tx := range txs {
			f.pool.addTx([]byte{byte(i)}, tx)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the OnAdded handler blocked on a full queue")
	}

	counters := w.Counters()
	assert.Equal(t, uint64(2), counters.Enqueued)
	assert.Equal(t, uint64(48), counters.Dropped)

	// a dropped address is forgotten by the dedup set, so it is accepted once there is room
	_ = queued(w)
	f.pool.addTx([]byte("again"), invokeTx(t, scAddress(50)))
	assert.Equal(t, [][]byte{scAddress(50)}, queued(w))
}

// --- worker --------------------------------------------------------------------------

func TestCompiledCodeWarmer_WarmCompilesAndStoresModule(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))

	key := f.hasher.Compute(string(code))
	found, compiled := f.cache.GetCompiledCode(key)
	require.True(t, found, "the module must be stored under hasher(code), the key the runtime computes")
	require.NotEmpty(t, compiled)

	counters := w.Counters()
	assert.Equal(t, uint64(1), counters.Compiled)
	assert.Equal(t, uint64(0), counters.Errors)
	assert.Greater(t, counters.CompileTime, time.Duration(0))

	// the stored module restores with the options the runtime would build
	gasCost, err := kvmConfig.CreateGasConfig(f.gas.LatestGasSchedule())
	require.NoError(t, err)
	restoreExec, err := wasmer2.CreateExecutor()
	require.NoError(t, err)
	restoreExec.SetOpcodeCosts(gasCost.WASMOpcodeCost)
	inst, err := restoreExec.NewInstanceFromCompiledCodeWithOptions(compiled, contexts.BuildCompilationOptions(gasCost, true, 1_000_000))
	require.NoError(t, err)
	inst.Clean()
}

func TestCompiledCodeWarmer_SkipsModuleAlreadyPresent(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	key := f.hasher.Compute(string(code))
	f.cache.put(key, []byte("module from an earlier cold call"))
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))

	assert.Equal(t, int64(0), f.exec.numCompiles.Load())
	_, stored := f.cache.GetCompiledCode(key)
	assert.Equal(t, []byte("module from an earlier cold call"), stored)
	assert.Equal(t, uint64(1), w.Counters().SkippedPresent)
}

func TestCompiledCodeWarmer_DiscardsModuleWhenClearedDuringCompile(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	f.accounts.setContract(scAddress(1), readBasicFeatures(t))
	f.exec.beforeCompile = f.cache.clear
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))

	assert.Equal(t, 0, f.cache.len())
	counters := w.Counters()
	assert.Equal(t, uint64(1), counters.DiscardedStale)
	assert.Equal(t, uint64(0), counters.Compiled)
	assert.Len(t, w.rescanCh, 1, "the clear must have requested a rescan")
}

func TestCompiledCodeWarmer_DiscardsModuleWhenForkFlagFlipsDuringCompile(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	f.flags.v4.Store(false)
	f.accounts.setContract(scAddress(1), readBasicFeatures(t))
	f.exec.beforeCompile = func() { f.flags.v4.Store(true) }
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))

	assert.Equal(t, 0, f.cache.len())
	assert.Equal(t, uint64(1), w.Counters().DiscardedStale)
}

func TestCompiledCodeWarmer_DiscardsModuleWhenGasScheduleChangesDuringCompile(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	newSchedule := kvmConfig.MakeGasMapForTests()
	newSchedule["WASMOpcodeCost"]["I64Add"]++
	f.exec.beforeCompile = func() { f.gas.GasSchedule = newSchedule }
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))
	assert.Equal(t, 0, f.cache.len())
	assert.Equal(t, uint64(1), w.Counters().DiscardedStale)
	assert.Equal(t, int64(1), f.exec.numSetOpcodeCosts.Load())

	// the next attempt picks up the new schedule, moves the executor to it and saves
	f.exec.beforeCompile = nil
	w.warm(scAddress(1))
	assert.Equal(t, 1, f.cache.len())
	assert.Equal(t, uint64(1), w.Counters().Compiled)
	assert.Equal(t, int64(2), f.exec.numSetOpcodeCosts.Load())
}

func TestCompiledCodeWarmer_KeepsExecutorCostsWhileScheduleIsUnchanged(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	f.accounts.setContract(scAddress(2), append(append([]byte{}, code...), []byte{}...))
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))
	f.cache.clear()
	w.warm(scAddress(2))

	assert.Equal(t, uint64(2), w.Counters().Compiled)
	assert.Equal(t, int64(1), f.exec.numSetOpcodeCosts.Load())
}

func TestCompiledCodeWarmer_SkipsCodeWithStartSection(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	f.accounts.setContract(scAddress(1), startSectionModule())
	w := f.newStopped(t, 16)

	w.warm(scAddress(1))

	assert.Equal(t, int64(0), f.exec.numCompiles.Load(), "a start function would run without VM hooks")
	assert.Equal(t, 0, f.cache.len())
	assert.Equal(t, uint64(1), w.Counters().SkippedUnsupported)
}

func TestCompiledCodeWarmer_CountsErrors(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	f.accounts.setContract(scAddress(2), nil)
	f.accounts.setContract(scAddress(3), []byte("definitely not wasm"))
	w := f.newStopped(t, 16)

	w.warm(scAddress(1)) // unknown account
	w.warm(scAddress(2)) // no code
	w.warm(scAddress(3)) // does not compile

	assert.Equal(t, 0, f.cache.len())
	assert.Equal(t, uint64(3), w.Counters().Errors+w.Counters().SkippedUnsupported)
	assert.Equal(t, uint64(0), w.Counters().Compiled)
}

// --- rescan --------------------------------------------------------------------------

func TestCompiledCodeWarmer_RescanIsCoalesced(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	for i := 0; i < 5; i++ {
		w.TriggerRescan()
	}
	assert.Len(t, w.rescanCh, 1)
}

func TestCompiledCodeWarmer_ClearRequestsRescan(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	f.cache.clear()
	f.cache.clear()
	assert.Len(t, w.rescanCh, 1)
}

func TestCompiledCodeWarmer_RescanReenqueuesPoolContents(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 16)

	f.pool.addTx([]byte("a"), invokeTx(t, scAddress(1)))
	_ = queued(w) // drained: scAddress(1) stays in the dedup set
	f.pool.store.Put([]byte("b"), invokeTx(t, scAddress(2)), 0)
	f.pool.store.Put([]byte("c"), "not a transaction", 0)
	f.pool.store.Put([]byte("d"), &txcache.WrappedTransaction{Tx: invokeTx(t, scAddress(3))}, 0)

	w.rescan()

	assert.ElementsMatch(t, [][]byte{scAddress(1), scAddress(2), scAddress(3)}, queued(w))
	assert.Contains(t, f.pool.requestedID, txPoolCacheID)
	assert.Equal(t, uint64(1), w.Counters().Rescans)
}

// --- lifecycle -----------------------------------------------------------------------

func TestCompiledCodeWarmer_EndToEndAndCleanShutdown(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	code := readBasicFeatures(t)
	f.accounts.setContract(scAddress(1), code)
	key := f.hasher.Compute(string(code))

	w, err := NewCompiledCodeWarmer(f.args)
	require.NoError(t, err)

	f.pool.addTx([]byte("h1"), invokeTx(t, scAddress(1)))
	require.Eventually(t, func() bool { return w.Counters().Compiled == 1 }, 10*time.Second, 5*time.Millisecond)
	found, _ := f.cache.GetCompiledCode(key)
	require.True(t, found)

	// a clear wipes the module; the rescan it triggers finds the tx still in the pool
	f.cache.clear()
	require.Eventually(t, func() bool { return w.Counters().Compiled == 2 }, 10*time.Second, 5*time.Millisecond)
	found, _ = f.cache.GetCompiledCode(key)
	require.True(t, found)

	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
	select {
	case <-w.done:
	default:
		require.Fail(t, "the worker goroutine must have exited")
	}

	// after Close the handler still never blocks or panics, and nothing is processed
	require.NotPanics(t, func() {
		for i := byte(10); i < 40; i++ {
			f.pool.addTx([]byte{i}, invokeTx(t, scAddress(i)))
		}
		w.TriggerRescan()
	})
	assert.Equal(t, uint64(2), w.Counters().Compiled)
}

func TestCompiledCodeWarmer_CloseWithoutStartIsSafe(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w, err := newCompiledCodeWarmer(f.args, 4, 4)
	require.NoError(t, err)

	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
}

func TestCompiledCodeWarmer_PublishesMetrics(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	var mut sync.Mutex
	metrics := make(map[string]uint64)
	f.args.StatusHandler = &recordingStatusHandler{set: func(key string, value uint64) {
		mut.Lock()
		metrics[key] = value
		mut.Unlock()
	}}
	f.accounts.setContract(scAddress(1), readBasicFeatures(t))
	w := f.newStopped(t, 16)

	w.onTxAdded([]byte("h"), &txcache.WrappedTransaction{Tx: invokeTx(t, scAddress(1))})
	w.warm(<-w.queue)

	mut.Lock()
	defer mut.Unlock()
	assert.Equal(t, uint64(1), metrics[metricEnqueued])
	assert.Equal(t, uint64(1), metrics[metricCompiled])
	assert.Contains(t, metrics, metricCompileTimeMs)
}

type recordingStatusHandler struct {
	statusHandler.NilStatusHandler
	set func(key string, value uint64)
}

func (r *recordingStatusHandler) SetUInt64Value(key string, value uint64) {
	r.set(key, value)
}

func (r *recordingStatusHandler) IsInterfaceNil() bool {
	return r == nil
}
