package sync

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/typeConverters/uint64ByteSlice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSimStorage = errors.New("not found")

// syncSim is a node that is behind by one block: it has committed nonce 5 and
// holds the honest nonce 6 in its headers pool. The chronology and the block
// processor are simulated; ProcessBlock and CommitBlock apply the same upper
// bound as baseProcessor.checkSlotIsNotInTheFuture (header slot <= index + 1).
type syncSim struct {
	mut          sync.Mutex
	slot         int64
	current      *block.Block
	currentHash  []byte
	byHash       map[string]*block.Block
	queued       *block.Block
	queuedHash   []byte
	queuedInPool bool

	processBlockFailHook func()

	finalNonce         uint64
	processBlockHook   func()
	processBlockErr    func(header data.HeaderHandler) error
	numProcessBlock    int
	numReverts         int
	numSetCurrentBlock int
	checkStateErr      error
	checkedStateNonces []uint64
	numRemovedFromPool int
	numRemovedFromFork int
	numResetProbable   int
	numResetFork       int
	numResetRequests   int
	removedRequests    []uint64
	removedRequestHash []byte
}

func newHeader(nonce uint64, slot uint64, parentHash []byte) *block.Block {
	return &block.Block{Header: &block.BlockHeader{Nonce: nonce, Slot: slot, ParentHash: parentHash}}
}

func (sim *syncSim) index() int64 {
	sim.mut.Lock()
	defer sim.mut.Unlock()
	return sim.slot
}

func (sim *syncSim) setSlot(slot int64) {
	sim.mut.Lock()
	sim.slot = slot
	sim.mut.Unlock()
}

func (sim *syncSim) currentHeader() data.HeaderHandler {
	sim.mut.Lock()
	defer sim.mut.Unlock()
	return sim.current
}

func (sim *syncSim) checkSlot(header data.HeaderHandler) error {
	if header.GetSlot() > uint64(sim.index())+1 {
		return process.ErrSlotAheadOfChronology
	}
	return nil
}

func newSyncSim(t *testing.T) (*syncSim, *baseBootstrap) {
	t.Helper()

	sim := &syncSim{slot: 11, finalNonce: 4, byHash: make(map[string]*block.Block)}

	b4 := newHeader(4, 9, []byte("h3"))
	b5 := newHeader(5, 10, []byte("h4"))
	sim.byHash["h4"] = b4
	sim.byHash["h5"] = b5
	sim.current, sim.currentHash = b5, []byte("h5")
	sim.queued, sim.queuedHash, sim.queuedInPool = newHeader(6, 12, []byte("h5")), []byte("h6"), true

	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}

	headers := &mock.HeadersCacherStub{
		RemoveHeaderByHashCalled: func(_ []byte) {
			sim.mut.Lock()
			sim.numRemovedFromPool++
			sim.queuedInPool = false
			sim.mut.Unlock()
		},
	}

	forkDetector := &mock.ForkDetectorMock{
		CheckForkCalled:                 process.NewForkInfo,
		ProbableHighestNonceCalled:      func() uint64 { return 6 },
		GetNotarizedHeaderHashCalled:    func(_ uint64) []byte { return nil },
		GetHighestFinalBlockNonceCalled: func() uint64 { return sim.finalNonce },
		RemoveHeaderCalled: func(_ uint64, _ []byte) {
			sim.mut.Lock()
			sim.numRemovedFromFork++
			sim.mut.Unlock()
		},
		ResetProbableHighestNonceCalled: func() {
			sim.mut.Lock()
			sim.numResetProbable++
			sim.mut.Unlock()
		},
		ResetForkCalled: func() {
			sim.mut.Lock()
			sim.numResetFork++
			sim.mut.Unlock()
		},
	}

	chain := &mock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return newHeader(0, 0, nil) },
		GetCurrentBlockHeaderCalled: sim.currentHeader,
		GetCurrentBlockHeaderHashCalled: func() []byte {
			sim.mut.Lock()
			defer sim.mut.Unlock()
			return sim.currentHash
		},
		SetCurrentBlockHeaderAndHashCalled: func(header data.HeaderHandler, hash []byte) error {
			sim.mut.Lock()
			defer sim.mut.Unlock()
			sim.numSetCurrentBlock++
			sim.current, _ = header.(*block.Block)
			sim.currentHash = hash
			return nil
		},
	}

	blockProcessor := &consensusMock.BlockProcessorMock{
		ProcessBlockCalled: func(header data.HeaderHandler, _ func() time.Duration) error {
			sim.mut.Lock()
			sim.numProcessBlock++
			sim.mut.Unlock()
			if err := sim.checkSlot(header); err != nil {
				if sim.processBlockFailHook != nil {
					sim.processBlockFailHook()
				}
				return err
			}
			if !bytes.Equal(header.GetParentHash(), chain.GetCurrentBlockHeaderHash()) {
				return process.ErrBlockHashDoesNotMatch
			}
			if err := process.CheckBlockBodyMatchesHeader(header, hasher, &mock.ForkControllerStub{}); err != nil {
				return err
			}
			if sim.processBlockErr != nil {
				if err := sim.processBlockErr(header); err != nil {
					return err
				}
			}
			if sim.processBlockHook != nil {
				sim.processBlockHook()
			}
			return nil
		},
		CommitBlockCalled: func(header data.HeaderHandler) error {
			if err := sim.checkSlot(header); err != nil {
				return err
			}
			sim.mut.Lock()
			sim.byHash[string(sim.queuedHash)] = sim.queued
			sim.queuedInPool = false
			sim.mut.Unlock()
			return chain.SetCurrentBlockHeaderAndHash(header, sim.queuedHash)
		},
		RevertStateToBlockCalled: func(_ data.HeaderHandler) error {
			sim.mut.Lock()
			sim.numReverts++
			sim.mut.Unlock()
			return nil
		},
		CheckStateAvailableCalled: func(header data.HeaderHandler) error {
			sim.mut.Lock()
			defer sim.mut.Unlock()
			sim.checkedStateNonces = append(sim.checkedStateNonces, header.GetNonce())
			return sim.checkStateErr
		},
		RestoreBlockIntoPoolsCalled: func(_ data.HeaderHandler) error { return nil },
	}

	storer := &mock.StorerStub{
		GetCalled:    func(_ []byte) ([]byte, error) { return nil, errSimStorage },
		RemoveCalled: func(_ []byte) error { return nil },
	}

	boot := &baseBootstrap{
		headers:        headers,
		chainHandler:   chain,
		blockProcessor: blockProcessor,
		store: &mock.ChainStorerMock{
			GetStorerCalled: func(_ retriever.UnitType) storage.Storer { return storer },
		},
		slotManager: &consensusMock.SlotManagerMock{
			IndexCalled:        sim.index,
			TimeDurationCalled: func() time.Duration { return time.Second },
		},
		hasher:       hasher,
		marshalizer:  marshalizer,
		forkDetector: forkDetector,
		requestHandler: &mock.RequestHandlerStub{
			ResetRequestsCalled: func() {
				sim.mut.Lock()
				sim.numResetRequests++
				sim.mut.Unlock()
			},
			RemoveHeaderRequestsCalled: func(nonce uint64, hash []byte) {
				sim.mut.Lock()
				sim.removedRequests = append(sim.removedRequests, nonce)
				sim.removedRequestHash = hash
				sim.mut.Unlock()
			},
		},
		networkWatcher: &mock.MessengerStub{},
		bootStorer: &mock.BoostrapStorerMock{
			GetCalled:            func(_ int64) (*bootstrapStorage.BootstrapData, error) { return nil, errSimStorage },
			GetHighestSlotCalled: func() int64 { return 0 },
		},
		uint64Converter:      uint64ByteSlice.NewBigEndianConverter(),
		indexer:              &consensusMock.IndexerMock{},
		headerStore:          storer,
		headerNonceHashStore: storer,
		hasStarted:           true,
	}
	boot.blockBootstrapper = &simBlockBootstrapper{sim: sim}
	boot.init()

	return sim, boot
}

type simBlockBootstrapper struct {
	sim *syncSim
}

func (sbb *simBlockBootstrapper) getCurrHeader() (data.HeaderHandler, error) {
	return sbb.sim.currentHeader(), nil
}

func (sbb *simBlockBootstrapper) getPrevHeader(header data.HeaderHandler, _ storage.Storer) (data.HeaderHandler, error) {
	sbb.sim.mut.Lock()
	defer sbb.sim.mut.Unlock()
	prev, ok := sbb.sim.byHash[string(header.GetParentHash())]
	if !ok {
		return nil, errSimStorage
	}
	return prev, nil
}

func (sbb *simBlockBootstrapper) getHeaderWithHashRequestingIfMissing(_ []byte) (data.HeaderHandler, error) {
	return nil, process.ErrTimeIsOut
}

func (sbb *simBlockBootstrapper) getHeaderWithNonceRequestingIfMissing(nonce uint64) (data.HeaderHandler, error) {
	sbb.sim.mut.Lock()
	defer sbb.sim.mut.Unlock()
	if !sbb.sim.queuedInPool || sbb.sim.queued.GetNonce() != nonce {
		return nil, process.ErrTimeIsOut
	}
	return sbb.sim.queued, nil
}

func (sbb *simBlockBootstrapper) haveHeaderInPoolWithNonce(_ uint64) bool { return true }
func (sbb *simBlockBootstrapper) isForkTriggeredByMeta() bool             { return false }
func (sbb *simBlockBootstrapper) requestHeaderByNonce(_ uint64)           {}

func assertDroppedWithoutRollBack(t *testing.T, sim *syncSim, boot *baseBootstrap) {
	t.Helper()

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must not be rolled back")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.False(t, sim.queuedInPool, "the rejected block must leave the pool")
	assert.Equal(t, 1, sim.numRemovedFromPool)
	assert.Equal(t, 0, sim.numRemovedFromFork, "the header must stay in the fork detector")
	assert.Equal(t, 0, sim.numResetProbable)
	assert.Equal(t, []uint64{6}, sim.removedRequests, "the dropped block must be requested again right away")
	queuedHash, err := tools.CalculateHash(boot.marshalizer, boot.hasher, sim.queued.Header)
	require.NoError(t, err)
	assert.Equal(t, queuedHash, sim.removedRequestHash, "the dropped header must be requestable again by its hash")
	assert.Equal(t, 0, sim.numResetRequests, "unrelated requests must stay in the time cache")
	assert.Equal(t, uint32(0), boot.GetNumSyncedWithErrorsForNonce(6), "no sync failure must be counted")
}

func assertNothingDiscarded(t *testing.T, sim *syncSim, boot *baseBootstrap) {
	t.Helper()

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must not be rolled back")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.True(t, sim.queuedInPool, "the queued header must stay in the pool")
	assert.Equal(t, 0, sim.numRemovedFromPool)
	assert.Equal(t, 0, sim.numRemovedFromFork, "the header must stay in the fork detector")
	assert.Equal(t, 0, sim.numResetProbable)
	assert.Equal(t, uint32(0), boot.GetNumSyncedWithErrorsForNonce(6), "no sync failure must be counted")
}

func TestSyncBlock_ChronologyStepsBackBeforeProcessShouldRetryWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	// The chronology steps back from 11 to 10: slot 12 is now two slots ahead.
	sim.setSlot(10)
	err := boot.syncBlock()
	require.ErrorIs(t, err, process.ErrSlotAheadOfChronology)
	assertNothingDiscarded(t, sim, boot)

	// While neither the chronology nor the chain moves, the sync loop - which spins
	// every few milliseconds - does not rebuild the attempt or its header requests.
	for i := 0; i < 500; i++ {
		require.NoError(t, boot.syncBlock())
	}
	sim.mut.Lock()
	assert.Equal(t, 1, sim.numProcessBlock, "the doomed attempt must not be retried within the same slot")
	sim.mut.Unlock()
	assertNothingDiscarded(t, sim, boot)

	// A chronology that moves - even further back - is retried once.
	sim.setSlot(9)
	require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)
	require.NoError(t, boot.syncBlock())
	sim.mut.Lock()
	assert.Equal(t, 2, sim.numProcessBlock)
	sim.mut.Unlock()
	assertNothingDiscarded(t, sim, boot)

	// Once the chronology catches up the same header is synced on top of nonce 5.
	sim.setSlot(11)
	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestSyncBlock_ChronologyStepsBackBeforeCommitShouldRetryWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	// ProcessBlock accepts slot 12 at index 11, then the chronology steps back
	// before CommitBlock re-runs the validity check.
	var once atomic.Bool
	sim.processBlockHook = func() {
		if once.CompareAndSwap(false, true) {
			sim.setSlot(10)
		}
	}

	err := boot.syncBlock()
	require.ErrorIs(t, err, process.ErrSlotAheadOfChronology)
	assertNothingDiscarded(t, sim, boot)

	sim.setSlot(11)
	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
}

func TestSyncBlock_InvalidHeaderShouldStillRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	// Control case: a header that fails for any other reason keeps the existing
	// behavior - it is dropped, counted and the last non-final block is reverted.
	sim.queued = newHeader(6, 12, []byte("not-h5"))

	err := boot.syncBlock()
	require.ErrorIs(t, err, process.ErrBlockHashDoesNotMatch)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, 1, sim.numReverts)
	assert.False(t, sim.queuedInPool)
	assert.Equal(t, uint32(1), boot.GetNumSyncedWithErrorsForNonce(6))
}

func TestSyncBlock_BodyNotMatchingHeaderShouldBeDroppedWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	honest := sim.queued
	mismatched := newHeader(6, 12, []byte("h5"))
	mismatched.Header.TxCount = 1
	sim.queued = mismatched

	require.ErrorIs(t, boot.syncBlock(), process.ErrInvalidTXCount)
	assertDroppedWithoutRollBack(t, sim, boot)

	sim.mut.Lock()
	sim.queued, sim.queuedInPool = honest, true
	sim.mut.Unlock()

	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestSyncBlock_TxResultMismatchShouldBeDroppedWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	var once atomic.Bool
	sim.processBlockErr = func(_ data.HeaderHandler) error {
		if once.CompareAndSwap(false, true) {
			return process.ErrTransactionResultMismatch
		}
		return nil
	}

	require.ErrorIs(t, boot.syncBlock(), process.ErrTransactionResultMismatch)
	assertDroppedWithoutRollBack(t, sim, boot)

	sim.mut.Lock()
	sim.queuedInPool = true
	sim.mut.Unlock()

	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestDoJobOnSyncBlockFail_RejectedBodyAtErrorLimitShouldNotReset(t *testing.T) {
	bodyErrors := []error{
		fmt.Errorf("%w: %w", process.ErrBlockBodyNotMatchingHeader, process.ErrInvalidTxHashLength),
		fmt.Errorf("%w: %w", process.ErrBlockBodyNotMatchingHeader, process.ErrInvalidTXCount),
		fmt.Errorf("%w: %w", process.ErrBlockBodyNotMatchingHeader, process.ErrInvalidTXResultsCount),
		fmt.Errorf("%w: %w", process.ErrBlockBodyNotMatchingHeader, process.ErrTxRootHashDoesNotMatch),
		process.ErrTransactionResultMismatch,
		process.WrapBlockExecutionNotMatchingHeader(process.ErrInvalidTXFees),
		process.WrapBlockExecutionNotMatchingHeader(process.ErrInvalidKAppsFees),
		process.WrapBlockExecutionNotMatchingHeader(process.ErrRootStateDoesNotMatch),
	}

	for _, bodyErr := range bodyErrors {
		t.Run(bodyErr.Error(), func(t *testing.T) {
			sim, boot := newSyncSim(t)

			sim.setSlot(process.SlotModulusTrigger * 2)
			boot.SetNumSyncedWithErrorsForNonce(6, process.MaxSyncWithErrorsAllowed)

			boot.doJobOnSyncBlockFail(sim.queued, bodyErr, sim.index())

			assert.Equal(t, uint32(process.MaxSyncWithErrorsAllowed), boot.GetNumSyncedWithErrorsForNonce(6))
			sim.mut.Lock()
			defer sim.mut.Unlock()
			assert.Equal(t, uint64(5), sim.current.GetNonce())
			assert.Equal(t, 0, sim.numResetProbable)
			assert.Equal(t, 0, sim.numReverts)
			assert.Equal(t, 0, sim.numRemovedFromFork)
			assert.Equal(t, 1, sim.numRemovedFromPool)
			assert.Equal(t, []uint64{6}, sim.removedRequests)
			queuedHash, err := tools.CalculateHash(boot.marshalizer, boot.hasher, sim.queued.Header)
			require.NoError(t, err)
			assert.Equal(t, queuedHash, sim.removedRequestHash)
			assert.Equal(t, 0, sim.numResetRequests)
		})
	}
}

func TestDoJobOnSyncBlockFail_HeaderLevelTxErrorsShouldStillRollBack(t *testing.T) {
	headerErrors := []error{
		fmt.Errorf("%w, metachain, computed: %s, received: %s, meta header nonce: %d",
			process.ErrValidatorStatsRootHashDoesNotMatch, "computed root", "received root", 6),
		process.ErrInvalidTXCount,
		process.ErrTxRootHashDoesNotMatch,
		process.ErrInvalidTXFees,
		process.ErrRootStateDoesNotMatch,
	}

	for _, headerErr := range headerErrors {
		t.Run(headerErr.Error(), func(t *testing.T) {
			sim, boot := newSyncSim(t)

			boot.doJobOnSyncBlockFail(sim.queued, headerErr, sim.index())

			sim.mut.Lock()
			defer sim.mut.Unlock()
			assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
			assert.Equal(t, 1, sim.numReverts)
			assert.False(t, sim.queuedInPool)
			assert.Equal(t, uint32(1), boot.GetNumSyncedWithErrorsForNonce(6))
		})
	}
}

func TestDoJobOnSyncBlockFail_SlotAheadOfChronologyAtErrorLimitShouldNotReset(t *testing.T) {
	sim, boot := newSyncSim(t)

	// Even at the sync-with-errors limit in a proper slot the error is not
	// treated as a failure: no probable-nonce reset and no pool purge.
	sim.setSlot(process.SlotModulusTrigger * 2)
	boot.SetNumSyncedWithErrorsForNonce(6, process.MaxSyncWithErrorsAllowed)

	boot.doJobOnSyncBlockFail(sim.queued, process.ErrSlotAheadOfChronology, sim.index())

	assert.Equal(t, uint32(process.MaxSyncWithErrorsAllowed), boot.GetNumSyncedWithErrorsForNonce(6))
	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, 0, sim.numResetProbable)
	assert.Equal(t, 0, sim.numReverts)
	assert.True(t, sim.queuedInPool)
}

func TestSyncBlock_SlotTicksOverAfterFailedAttemptShouldStillRetry(t *testing.T) {
	sim, boot := newSyncSim(t)

	// The attempt fails at index 10 and the slot ticks over to 11 before the
	// failure is handled: index 11 was never tried, so it must not be postponed.
	sim.setSlot(10)
	var once atomic.Bool
	sim.processBlockFailHook = func() {
		if once.CompareAndSwap(false, true) {
			sim.setSlot(11)
		}
	}

	require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)
	assertNothingDiscarded(t, sim, boot)

	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestSyncBlock_FarFutureHeaderShouldBePostponedWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	// A nonce 6 dated far beyond any chronology drift stays queued like any other
	// slot-ahead header: nothing is dropped, counted or rolled back.
	honest := sim.queued
	sim.queued = newHeader(6, uint64(sim.index())+1000, []byte("h5"))

	require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)
	assert.True(t, boot.isSyncPostponed())
	assertNothingDiscarded(t, sim, boot)

	// Repeated failures across slots are never counted towards the error limit.
	for range process.MaxSyncWithErrorsAllowed + 1 {
		sim.setSlot(sim.index() + 1)
		require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)
	}
	assertNothingDiscarded(t, sim, boot)

	// The honest header arriving later is the one picked from the pool, and is
	// synced on the next slot.
	sim.mut.Lock()
	sim.queued = honest
	sim.mut.Unlock()
	sim.setSlot(sim.index() + 1)
	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestIsSyncPostponed_ShouldReleaseWhenNonceChangesInSameSlot(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.setSlot(10)
	require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)
	require.True(t, boot.isSyncPostponed())

	// The chain moves to nonce 6 while the chronology stays on the same slot.
	require.NoError(t, boot.chainHandler.SetCurrentBlockHeaderAndHash(newHeader(6, 10, []byte("h5")), []byte("h6")))

	assert.False(t, boot.isSyncPostponed())
}

func forceOneBlockRollBack(boot *baseBootstrap) {
	forkDetector := boot.forkDetector.(*mock.ForkDetectorMock)
	forkDetector.CheckForkCalled = func() *process.ForkInfo {
		forkInfo := process.NewForkInfo()
		forkInfo.IsDetected = true
		return forkInfo
	}
}

func TestSyncBlock_ForcedForkShouldRollBackOneBlock(t *testing.T) {
	sim, boot := newSyncSim(t)

	forceOneBlockRollBack(boot)

	err := boot.syncBlock()
	require.NoError(t, err)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, 1, sim.numResetFork)
}

const syncLoggerName = "process/sync"

type warnRecorder struct {
	message  string
	level    logger.LogLevel
	mut      sync.Mutex
	lines    [][]string
	messages []string
}

func (r *warnRecorder) Write(p []byte) (int, error) { return len(p), nil }

func (r *warnRecorder) Output(line logger.LogLineHandler) []byte {
	isOtherMessage := r.message != "" && line.GetMessage() != r.message
	if isOtherMessage || line.GetLoggerName() != syncLoggerName || line.GetLogLevel() != int32(r.level) {
		return nil
	}

	r.mut.Lock()
	r.lines = append(r.lines, line.GetArgs())
	r.messages = append(r.messages, line.GetMessage())
	r.mut.Unlock()

	return nil
}

func (r *warnRecorder) IsInterfaceNil() bool { return r == nil }

func (r *warnRecorder) snapshot() [][]string {
	r.mut.Lock()
	defer r.mut.Unlock()

	return append([][]string(nil), r.lines...)
}

func (r *warnRecorder) loggedMessages() []string {
	r.mut.Lock()
	defer r.mut.Unlock()

	return append([]string(nil), r.messages...)
}

func observeWarn(t *testing.T, message string) *warnRecorder {
	t.Helper()

	return observeLog(t, message, logger.LogWarning)
}

func observeAllWarn(t *testing.T) *warnRecorder {
	t.Helper()

	return observeLog(t, "", logger.LogWarning)
}

func observeLog(t *testing.T, message string, level logger.LogLevel) *warnRecorder {
	t.Helper()

	previous := log.GetLevel()
	if previous > level {
		log.SetLevel(level)
	}
	recorder := &warnRecorder{message: message, level: level}
	require.NoError(t, logger.AddLogObserver(recorder, recorder))
	t.Cleanup(func() {
		_ = logger.RemoveLogObserver(recorder)
		log.SetLevel(previous)
	})

	return recorder
}

const warnRollBackRefused = "roll back refused, state of the target block is not available"

func TestRollBack_TargetStateNotAvailableShouldRefuseWithoutMovingHead(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	recorder := observeWarn(t, warnRollBackRefused)

	err := boot.rollBack(false)
	require.ErrorIs(t, err, process.ErrStateNotAvailable)

	lines := recorder.snapshot()
	require.Len(t, lines, 1, "the refusal must be logged once at warn level")
	assert.Contains(t, lines[0], "nonce")
	assert.Contains(t, lines[0], "error")

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, []byte("h5"), sim.currentHash)
	assert.Equal(t, 0, sim.numSetCurrentBlock, "the head must not be moved and restored")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces)
}

func TestRollBack_RepeatedRefusalForSameTargetShouldWarnOncePerSlotTrigger(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeLog(t, warnRollBackRefused, logger.LogWarning)
	repeats := observeLog(t, warnRollBackRefused, logger.LogDebug)

	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 1, "a repeated refusal in the same slot must not warn again")
	require.Len(t, repeats.snapshot(), 1, "a repeated refusal must still be logged at debug level")
	assert.Equal(t, warnings.snapshot()[0], repeats.snapshot()[0], "the repeat must carry the same fields")

	sim.setSlot(sim.index() + process.SlotModulusTrigger - 1)
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 1, "the refusal must stay quiet until the slot trigger has passed")
	require.Len(t, repeats.snapshot(), 2)

	sim.setSlot(sim.index() + 1)
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 2, "the refusal must warn again once the slot trigger has passed")
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 2)
	require.Len(t, repeats.snapshot(), 3)

	sim.mut.Lock()
	sim.byHash["other-h4"] = newHeader(4, 9, []byte("h3"))
	sim.current, sim.currentHash = newHeader(5, 10, []byte("other-h4")), []byte("other-h5")
	sim.mut.Unlock()

	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 3, "a refusal for a different target must warn right away")
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 3)
	require.Len(t, repeats.snapshot(), 4)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, []byte("other-h5"), sim.currentHash)
	assert.Equal(t, 0, sim.numSetCurrentBlock, "the head must not be moved and restored")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
}

func TestRollBack_RefusalAfterChronologyStepsBackShouldWarnAgain(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeLog(t, warnRollBackRefused, logger.LogWarning)

	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 1)

	sim.setSlot(sim.index() - 1)
	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 2, "a refusal at an earlier slot than the last warning must warn again")

	require.ErrorIs(t, boot.rollBack(false), process.ErrStateNotAvailable)
	require.Len(t, warnings.snapshot(), 2, "a repeated refusal in the same slot must not warn again")
}

func TestRollBack_TargetStateAvailableShouldRollBackOneBlock(t *testing.T) {
	sim, boot := newSyncSim(t)

	recorder := observeWarn(t, warnRollBackRefused)

	err := boot.rollBack(false)
	require.NoError(t, err)

	assert.Empty(t, recorder.snapshot())

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, []byte("h4"), sim.currentHash)
	assert.Equal(t, 1, sim.numSetCurrentBlock)
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces, "the target block state is checked before the roll back")
}

func TestSyncBlock_ForcedForkWithUnavailableTargetStateShouldKeepHead(t *testing.T) {
	sim, boot := newSyncSim(t)

	forceOneBlockRollBack(boot)
	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeAllWarn(t)

	err := boot.syncBlock()
	require.NoError(t, err)

	assert.Equal(t, []string{warnRollBackRefused}, warnings.loggedMessages(),
		"the refusal must be logged once at warn level")

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, []byte("h5"), sim.currentHash)
	assert.Equal(t, 0, sim.numSetCurrentBlock, "the head must not be moved and restored")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces)
	assert.Equal(t, 1, sim.numResetFork)
}

func forkAtHead(boot *baseBootstrap) {
	forkDetector := boot.forkDetector.(*mock.ForkDetectorMock)
	forkDetector.CheckForkCalled = func() *process.ForkInfo {
		forkInfo := process.NewForkInfo()
		forkInfo.IsDetected = true
		forkInfo.Nonce = 5
		forkInfo.Slot = 10
		forkInfo.Hash = []byte("other-h5")
		return forkInfo
	}
}

func TestSyncBlock_ForkWithUnavailableTargetStateShouldWarnOnceForRepeatedAttempts(t *testing.T) {
	sim, boot := newSyncSim(t)

	forkAtHead(boot)
	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeAllWarn(t)

	for i := 0; i < 3; i++ {
		err := boot.syncBlock()
		require.ErrorIs(t, err, process.ErrStateNotAvailable)
	}

	assert.Equal(t, []string{warnRollBackRefused}, warnings.loggedMessages(),
		"attempts repeated in the same slot must warn once")
	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.Equal(t, []uint64{4, 4, 4}, sim.checkedStateNonces)
}

func TestRollBackOneBlockForced_RepeatedRefusalInTheSameSlotShouldWarnOnce(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeAllWarn(t)

	boot.rollBackOneBlockForced()
	boot.rollBackOneBlockForced()

	assert.Equal(t, []string{warnRollBackRefused}, warnings.loggedMessages())
	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
}

func TestSyncBlock_ForcedForkBehindFinalBlockShouldNotWarn(t *testing.T) {
	sim, boot := newSyncSim(t)

	forceOneBlockRollBack(boot)
	sim.finalNonce = 5
	warnings := observeAllWarn(t)
	refusals := observeLog(t, "rollBackOneBlockForced", logger.LogDebug)

	err := boot.syncBlock()
	require.NoError(t, err)

	assert.Empty(t, warnings.loggedMessages(), "an expected refusal must not warn")
	lines := refusals.snapshot()
	require.Len(t, lines, 1, "the refusal must still be logged at debug level")
	assert.Equal(t, []string{"error", ErrRollBackBehindFinalHeader.Error()}, lines[0])

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the final block must stay the head")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.Empty(t, sim.checkedStateNonces)
}

func TestDoJobOnSyncBlockFail_RollBackBehindFinalBlockShouldNotWarn(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.finalNonce = 5
	warnings := observeAllWarn(t)
	refusals := observeLog(t, "rollBack", logger.LogDebug)

	boot.doJobOnSyncBlockFail(sim.queued, process.ErrInvalidTXCount, sim.index())
	boot.doJobOnSyncBlockFail(sim.queued, process.ErrInvalidTXCount, sim.index())

	assert.Empty(t, warnings.loggedMessages(), "an expected refusal must not warn, however often a block fails")
	assert.Len(t, refusals.snapshot(), 2, "each refusal must still be logged at debug level")

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the final block must stay the head")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
}

func TestRollBackErrorLogLevel_OnlyUnexpectedFailuresShouldWarn(t *testing.T) {
	assert.Equal(t, logger.LogDebug, rollBackErrorLogLevel(ErrRollBackBehindFinalHeader))
	assert.Equal(t, logger.LogDebug, rollBackErrorLogLevel(process.ErrNilBlockHeader))
	assert.Equal(t, logger.LogDebug,
		rollBackErrorLogLevel(fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)))
	assert.Equal(t, logger.LogWarning, rollBackErrorLogLevel(ErrRollBackBehindForkNonce))
	assert.Equal(t, logger.LogWarning, rollBackErrorLogLevel(errSimStorage))
}

func TestDoJobOnSyncBlockFail_TimeoutAtErrorLimitShouldRollBackOneBlock(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.setSlot(process.SlotModulusTrigger * 2)
	boot.SetNumSyncedWithErrorsForNonce(6, process.MaxSyncWithErrorsAllowed)

	boot.doJobOnSyncBlockFail(nil, process.ErrTimeIsOut, sim.index())

	assert.Equal(t, uint32(process.MaxSyncWithErrorsAllowed+1), boot.GetNumSyncedWithErrorsForNonce(6))
	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces, "the target block state is checked before the roll back")
	assert.Equal(t, 1, sim.numResetProbable)
}

func TestDoJobOnSyncBlockFail_TimeoutAtErrorLimitWithUnavailableTargetStateShouldKeepHead(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.setSlot(process.SlotModulusTrigger * 2)
	boot.SetNumSyncedWithErrorsForNonce(6, process.MaxSyncWithErrorsAllowed)
	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	warnings := observeAllWarn(t)

	boot.doJobOnSyncBlockFail(nil, process.ErrTimeIsOut, sim.index())

	assert.Equal(t, []string{warnRollBackRefused}, warnings.loggedMessages(),
		"the refusal must be logged once at warn level")
	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must stay the head")
	assert.Equal(t, []byte("h5"), sim.currentHash)
	assert.Equal(t, 0, sim.numSetCurrentBlock, "the head must not be moved and restored")
	assert.Equal(t, 0, sim.numReverts, "state must not be reverted")
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces)
	assert.Equal(t, 1, sim.numResetProbable)
}

const warnFinalNonceNotSaved = "roll back: highest final block nonce not saved in boot storage"

func recordBootStorerCalls(boot *baseBootstrap) *[]string {
	calls := &[]string{}
	bootStorer := boot.bootStorer.(*mock.BoostrapStorerMock)
	bootStorer.RaiseHighestFinalBlockNonceCalled = func(slot int64, nonce uint64, headerHash []byte) error {
		*calls = append(*calls, fmt.Sprintf("raise final nonce of slot %d to %d for %s", slot, nonce, headerHash))
		return nil
	}
	bootStorer.SaveLastSlotCalled = func(slot int64) error {
		*calls = append(*calls, fmt.Sprintf("save last slot %d", slot))
		return nil
	}

	return calls
}

func useBootStorerWithCommittedBlocks(t *testing.T, boot *baseBootstrap) process.BootStorer {
	t.Helper()

	bootStorer, err := bootstrapStorage.NewBootstrapStorer(boot.marshalizer, mock.NewStorerMock("", 0))
	require.NoError(t, err)
	for nonce := uint64(3); nonce <= 5; nonce++ {
		err = bootStorer.Put(int64(nonce)+5, &bootstrapStorage.BootstrapData{
			LastHeader:             &bootstrapStorage.BootstrapHeaderInfo{Nonce: nonce, Hash: []byte(fmt.Sprintf("h%d", nonce))},
			HighestFinalBlockNonce: nonce - 1,
		})
		require.NoError(t, err)
	}
	boot.bootStorer = bootStorer

	return bootStorer
}

func TestRollBack_ShouldPersistNewHeadAsFinalBlock(t *testing.T) {
	sim, boot := newSyncSim(t)

	bootStorer := useBootStorerWithCommittedBlocks(t, boot)

	headBefore, err := bootStorer.Get(9)
	require.NoError(t, err)
	require.Equal(t, uint64(3), headBefore.HighestFinalBlockNonce)
	require.Equal(t, int64(8), headBefore.LastSlot)
	require.Equal(t, int64(10), bootStorer.GetHighestSlot())

	err = boot.rollBack(false)
	require.NoError(t, err)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, int64(9), bootStorer.GetHighestSlot())

	head, err := bootStorer.Get(9)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), head.LastHeader.Nonce)
	assert.Equal(t, head.LastHeader.Nonce, head.HighestFinalBlockNonce, "a restart must restore the new head as the final block")
	assert.Equal(t, headBefore.LastSlot, head.LastSlot, "the link to the previous record must be kept")
	assert.Equal(t, headBefore.LastHeader, head.LastHeader)

	rolledBack, err := bootStorer.Get(10)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), rolledBack.HighestFinalBlockNonce)
	assert.Equal(t, int64(9), rolledBack.LastSlot)
}

func TestRollBack_ShouldNotPersistAFinalBlockAboveTheForkDetector(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.finalNonce = 3
	calls := recordBootStorerCalls(boot)

	err := boot.rollBack(false)
	require.NoError(t, err)

	assert.Equal(t, []string{"raise final nonce of slot 9 to 3 for h4", "save last slot 9"}, *calls,
		"the record is updated before the highest slot is moved to it")
}

func TestRollBack_ShouldNotPersistAFinalBlockAboveTheNewHead(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.finalNonce = 5
	boot.forkInfo.Nonce = 5
	calls := recordBootStorerCalls(boot)

	err := boot.rollBack(true)
	require.NoError(t, err)

	assert.Equal(t, []string{"raise final nonce of slot 9 to 4 for h4", "save last slot 9"}, *calls)
}

func TestRollBack_ToGenesisShouldNotRaiseFinalNonceInBootStorage(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.finalNonce = 0
	sim.byHash["h0"] = newHeader(0, 0, nil)
	sim.current, sim.currentHash = newHeader(1, 1, []byte("h0")), []byte("h1")
	calls := recordBootStorerCalls(boot)

	err := boot.rollBack(false)
	require.NoError(t, err)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, []string{"save last slot 0"}, *calls)
}

func TestRollBack_RefusedRollBackShouldNotTouchBootStorage(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.checkStateErr = fmt.Errorf("%w: validators trie, nonce 4", process.ErrStateNotAvailable)
	calls := recordBootStorerCalls(boot)

	err := boot.rollBack(false)
	require.ErrorIs(t, err, process.ErrStateNotAvailable)

	assert.Empty(t, *calls)

	sim.checkStateErr = nil

	err = boot.rollBack(false)
	require.NoError(t, err)

	assert.Equal(t, []string{"raise final nonce of slot 9 to 4 for h4", "save last slot 9"}, *calls,
		"an accepted roll back of the same node does reach the boot storage")
}

func TestRollBack_BootStorageFailureShouldNotAbortRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	calls := recordBootStorerCalls(boot)
	boot.bootStorer.(*mock.BoostrapStorerMock).RaiseHighestFinalBlockNonceCalled = func(_ int64, _ uint64, _ []byte) error {
		return errSimStorage
	}
	recorder := observeWarn(t, warnFinalNonceNotSaved)

	err := boot.rollBack(false)
	require.NoError(t, err)

	lines := recorder.snapshot()
	require.Len(t, lines, 1, "the failure must be logged once at warn level")
	assert.Contains(t, lines[0], "slot")
	assert.Contains(t, lines[0], "nonce")
	assert.Contains(t, lines[0], "error")

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "nonce 5 is rolled back")
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, []string{"save last slot 9"}, *calls, "the highest slot must still be moved")
}

func restoreForkDetectorFromStorage(t *testing.T, sim *syncSim, boot *baseBootstrap) *metaForkDetector {
	t.Helper()

	record, err := boot.bootStorer.Get(boot.bootStorer.GetHighestSlot())
	require.NoError(t, err)

	forkDetector, err := NewMetaForkDetector(boot.slotManager, &mock.BlackListHandlerStub{}, 0)
	require.NoError(t, err)

	slotSeconds := int64(boot.slotManager.TimeDuration().Seconds())
	for nonce := record.HighestFinalBlockNonce; nonce <= record.LastHeader.Nonce; nonce++ {
		hash := []byte(fmt.Sprintf("h%d", nonce))
		sim.mut.Lock()
		header := sim.byHash[string(hash)]
		sim.mut.Unlock()
		require.NotNil(t, header)
		header.Header.Timestamp = tools.SafeU64ToI64(header.GetSlot()) * slotSeconds
		require.NoError(t, forkDetector.AddHeader(header, hash, process.BHProcessed, nil, nil))
	}

	if record.HighestFinalBlockNonce == record.LastHeader.Nonce {
		forkDetector.SetFinalToLastCheckpoint()
	}
	require.Equal(t, record.HighestFinalBlockNonce, forkDetector.GetHighestFinalBlockNonce())

	return forkDetector
}

func TestRollBack_HeadRestoredAsFinalBlockShouldRefuseAnotherRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.byHash["h3"] = newHeader(3, 8, []byte("h2"))
	useBootStorerWithCommittedBlocks(t, boot)

	require.NoError(t, boot.rollBack(false))
	require.Equal(t, uint64(4), sim.currentHeader().GetNonce())

	forkDetector := restoreForkDetectorFromStorage(t, sim, boot)
	require.Equal(t, uint64(4), forkDetector.GetHighestFinalBlockNonce(),
		"the record saved by the roll back must restore the head as the final block")
	boot.forkDetector = forkDetector

	err := boot.rollBack(false)
	require.ErrorIs(t, err, ErrRollBackBehindFinalHeader)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(4), sim.current.GetNonce(), "the restored head must not be rolled back")
	assert.Equal(t, []byte("h4"), sim.currentHash)
	assert.Equal(t, 1, sim.numReverts, "state must not be reverted again")
	assert.Equal(t, 1, sim.numSetCurrentBlock)
	assert.Equal(t, []uint64{4}, sim.checkedStateNonces)
}

func TestRollBack_HeadRestoredAboveFinalBlockShouldAllowAnotherRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	sim.byHash["h3"] = newHeader(3, 8, []byte("h2"))
	bootStorer := useBootStorerWithCommittedBlocks(t, boot)
	require.NoError(t, bootStorer.SaveLastSlot(9))
	sim.mut.Lock()
	sim.current, sim.currentHash = sim.byHash["h4"], []byte("h4")
	sim.mut.Unlock()

	forkDetector := restoreForkDetectorFromStorage(t, sim, boot)
	require.Equal(t, uint64(3), forkDetector.GetHighestFinalBlockNonce(),
		"a record left with the final block below the head must restore it there")
	boot.forkDetector = forkDetector

	err := boot.rollBack(false)
	require.NoError(t, err)

	sim.mut.Lock()
	defer sim.mut.Unlock()
	assert.Equal(t, uint64(3), sim.current.GetNonce(), "nonce 4 is rolled back")
	assert.Equal(t, []byte("h3"), sim.currentHash)
	assert.Equal(t, 1, sim.numReverts)
	assert.Equal(t, []uint64{3}, sim.checkedStateNonces)
}

func headerAt(slot uint64, nonce uint64) data.HeaderHandler {
	return &block.Block{Header: &block.BlockHeader{Slot: slot, Nonce: nonce}}
}

func requestsHeaders(boot *baseBootstrap) bool {
	return boot.shouldTryToRequestHeaders(boot.slotsSinceLastCommittedBlock())
}

func fireStuck(boot *baseBootstrap, stalledWhileSynced bool) {
	boot.requestHeadersIfSyncIsStuck(stalledWhileSynced)
}

// blockBootstrapperStub is the smallest blockBootstrapper that
// requestHeadersIfSyncIsStuck exercises: it records the nonces requested and
// reports nothing as already present in the pool.
type blockBootstrapperStub struct {
	requestedNonces []uint64
}

func (s *blockBootstrapperStub) getCurrHeader() (data.HeaderHandler, error) { return nil, nil }

func (s *blockBootstrapperStub) getPrevHeader(data.HeaderHandler, storage.Storer) (data.HeaderHandler, error) {
	return nil, nil
}

func (s *blockBootstrapperStub) getHeaderWithHashRequestingIfMissing([]byte) (data.HeaderHandler, error) {
	return nil, nil
}

func (s *blockBootstrapperStub) getHeaderWithNonceRequestingIfMissing(uint64) (data.HeaderHandler, error) {
	return nil, nil
}

func (s *blockBootstrapperStub) haveHeaderInPoolWithNonce(uint64) bool { return false }

func (s *blockBootstrapperStub) isForkTriggeredByMeta() bool { return false }

func (s *blockBootstrapperStub) requestHeaderByNonce(nonce uint64) {
	s.requestedNonces = append(s.requestedNonces, nonce)
}

// newBootstrapForRequestGating builds the minimal baseBootstrap that
// shouldTryToRequestHeaders and slotsSinceLastCommittedBlock touch. Every other
// field stays zero-valued because neither path reads it.
func newBootstrapForRequestGating(
	slotIndex int64,
	genesisSlot uint64,
	currentHeader data.HeaderHandler,
) *baseBootstrap {
	return &baseBootstrap{
		slotManager: &consensusMock.SlotManagerMock{SlotIndex: slotIndex},
		chainHandler: &mock.BlockChainMock{
			GetGenesisHeaderCalled: func() data.HeaderHandler {
				return headerAt(genesisSlot, 0)
			},
			GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
				return currentHeader
			},
		},
		forkInfo: &process.ForkInfo{},
		// Needed by requestHeadersIfSyncIsStuck, whose logging reads the fork
		// detector's nonce; the predicate itself is side-effect-free.
		forkDetector: &mock.ForkDetectorMock{
			ProbableHighestNonceCalled: func() uint64 { return 0 },
		},
	}
}

// A node whose header intake is blocked keeps a frozen fork detector, so it
// reports itself synchronized while the chain moves on (issue #90). It must then
// react to the slot lag on every slot, not only on the every-20th-slot cadence
// this replaced.
func TestShouldTryToRequestHeaders_SyncedNodeReactsToSlotLagOnEverySlot(t *testing.T) {
	t.Parallel()

	const lastCommittedSlot = uint64(0)

	for slotIndex := int64(1); slotIndex <= 40; slotIndex++ {
		boot := newBootstrapForRequestGating(slotIndex, 0, headerAt(lastCommittedSlot, 0))
		boot.isNodeSynchronized = true

		// #nosec G115 -- slotIndex is a small positive loop bound
		expected := uint64(slotIndex) > process.MaxSlotsWithoutNewBlockReceived

		assert.Equal(t, expected, requestsHeaders(boot),
			"slot index %d, lag %d", slotIndex, slotIndex)
	}
}

// Import mode replays historical blocks, so the slot lag is unbounded and there
// are no peers. GetNodeState stays NsNotSynchronized, so the guard has to hold
// for a node that does not report itself synchronized too.
func TestShouldTryToRequestHeaders_ImportModeIgnoresSlotLag(t *testing.T) {
	t.Parallel()

	boot := newBootstrapForRequestGating(100, 0, headerAt(0, 0))
	boot.isInImportMode = true

	boot.isNodeSynchronized = true
	assert.False(t, requestsHeaders(boot))

	boot.isNodeSynchronized = false
	assert.False(t, requestsHeaders(boot))

	// The same lag outside import mode does trigger a request.
	boot.isInImportMode = false
	boot.isNodeSynchronized = true
	assert.True(t, requestsHeaders(boot))
}

func TestShouldTryToRequestHeaders_NotSynchronizedAlwaysRequests(t *testing.T) {
	t.Parallel()

	// No lag at all, so only the pre-existing unsynchronized branch can be what
	// returns true here.
	boot := newBootstrapForRequestGating(3, 0, headerAt(3, 0))
	boot.isNodeSynchronized = false

	assert.True(t, requestsHeaders(boot))
}

func TestShouldTryToRequestHeaders_GuardsShortCircuitBeforeSlotLag(t *testing.T) {
	t.Parallel()

	// Every case below carries a lag far past the threshold, so a true result
	// would prove the guard no longer runs before the slot-lag check.
	const laggingSlotIndex = int64(100)

	t.Run("before genesis", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAt(0, 0))
		boot.isNodeSynchronized = true
		boot.slotManager = &consensusMock.SlotManagerMock{
			SlotIndex:           laggingSlotIndex,
			BeforeGenesisCalled: func() bool { return true },
		}

		assert.False(t, requestsHeaders(boot))
	})

	t.Run("forced roll back one block", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAt(0, 0))
		boot.isNodeSynchronized = true
		boot.forkInfo = &process.ForkInfo{IsDetected: true, Nonce: math.MaxUint64}

		assert.False(t, requestsHeaders(boot))
	})

	t.Run("forced roll back to nonce", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAt(0, 0))
		boot.isNodeSynchronized = true
		boot.forkInfo = &process.ForkInfo{IsDetected: true, Slot: math.MaxUint64}

		assert.False(t, requestsHeaders(boot))
	})
}

func TestSlotsSinceLastCommittedBlock(t *testing.T) {
	t.Parallel()

	t.Run("uses the current header slot", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(30, 0, headerAt(12, 0))

		assert.Equal(t, uint64(18), boot.slotsSinceLastCommittedBlock())
	})

	t.Run("falls back to the genesis slot without a current header", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(30, 5, nil)

		assert.Equal(t, uint64(25), boot.slotsSinceLastCommittedBlock())
	})

	// Subtracting without this guard wraps around on uint64 and reports an
	// enormous lag, which would make the node request headers every single slot.
	t.Run("reports no lag when the committed block is ahead of the slot index", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(50, 0, headerAt(100, 0))

		assert.Equal(t, uint64(0), boot.slotsSinceLastCommittedBlock())

		boot.isNodeSynchronized = true
		assert.False(t, requestsHeaders(boot))
	})
}

// requestHeadersIfSyncIsStuck is what actually drives recovery once the node
// notices it is behind, and the size of each burst is what operators observe in
// the logs. This pins that formula: min(MaxHeadersToRequestInAdvance, lag-1)
// starting at the nonce after the last committed block (issue #90).
func TestRequestHeadersIfSyncIsStuck(t *testing.T) {
	t.Parallel()

	t.Run("does nothing at or below the threshold", func(t *testing.T) {
		t.Parallel()

		// Lag is exactly MaxSlotsWithoutNewBlockReceived, so the guard holds.
		boot, stub := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived), headerAt(0, 7))
		fireStuck(boot, false)

		assert.Empty(t, stub.requestedNonces)
	})

	t.Run("requests lag-1 headers just past the threshold", func(t *testing.T) {
		t.Parallel()

		// Lag 11 -> min(20, 10) = 10 headers, starting at nonce 8.
		boot, stub := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))
		fireStuck(boot, false)

		assert.Equal(t, []uint64{8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, stub.requestedNonces)
	})

	t.Run("caps the burst at MaxHeadersToRequestInAdvance", func(t *testing.T) {
		t.Parallel()

		// Lag 500 would ask for 499, the cap holds it to 20.
		boot, stub := newStuckRequestBoot(500, headerAt(0, 7))
		fireStuck(boot, false)

		assert.Len(t, stub.requestedNonces, process.MaxHeadersToRequestInAdvance)
		assert.Equal(t, uint64(8), stub.requestedNonces[0])
		assert.Equal(t, uint64(27), stub.requestedNonces[len(stub.requestedNonces)-1])
	})

	// The not-synced path reaches the burst without a lag check, so this guard
	// alone stops a full burst while the clock trails the committed tip.
	t.Run("no burst when the committed block is ahead of the slot index", func(t *testing.T) {
		t.Parallel()

		boot, stub := newStuckRequestBoot(50, headerAt(100, 7))
		fireStuck(boot, false)

		assert.Empty(t, stub.requestedNonces)
	})
}

// doJobOnSyncBlockFail decides whether a failed sync attempt rolls the chain
// back. The decision hinges on comparing the failure against
// process.ErrTimeIsOut: a timeout is expected and must not roll back, anything
// else must. Comparing with != instead of errors.Is silently rolls back on a
// wrapped timeout, so both forms are pinned here.
func TestDoJobOnSyncBlockFail(t *testing.T) {
	t.Parallel()

	// Not a multiple of process.SlotModulusTrigger, so the synced-with-errors
	// limit branch cannot reach the rollback and only the error decides.
	const notAProperSlot = int64(3)

	newBoot := func() (*baseBootstrap, *bool) {
		rolledBack := false
		boot := newBootstrapForRequestGating(notAProperSlot, 0, headerAt(1, 1))
		boot.mapNonceSyncedWithErrors = make(map[uint64]uint32)
		boot.marshalizer = &mock.MarshalizerMock{}
		boot.hasher = &mock.HasherMock{}
		boot.headers = &mock.HeadersCacherStub{}
		boot.forkDetector = &mock.ForkDetectorMock{
			ProbableHighestNonceCalled: func() uint64 { return 0 },
			RemoveHeaderCalled: func(_ uint64, _ []byte) {
				// Reached only from inside the rollback branch, and before
				// rollBack itself, which bails out on the nil header store.
				rolledBack = true
			},
		}

		return boot, &rolledBack
	}

	header := headerAt(1, 1)

	t.Run("plain timeout does not roll back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(header, process.ErrTimeIsOut, notAProperSlot)

		assert.False(t, *rolledBack)
	})

	t.Run("wrapped timeout does not roll back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(header, fmt.Errorf("process block: %w", process.ErrTimeIsOut), notAProperSlot)

		assert.False(t, *rolledBack)
	})

	t.Run("any other error rolls back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(header, errors.New("something else"), notAProperSlot)

		assert.True(t, *rolledBack)
	})

	t.Run("no header means processing never started, so no roll back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(nil, errors.New("something else"), notAProperSlot)

		assert.False(t, *rolledBack)
	})
}

// newStuckRequestBoot builds the fixture requestHeadersIfSyncIsStuck needs: the
// gating fixture plus a recording blockBootstrapper.
func newStuckRequestBoot(slotIndex int64, currentHeader data.HeaderHandler) (*baseBootstrap, *blockBootstrapperStub) {
	stub := &blockBootstrapperStub{}
	boot := newBootstrapForRequestGating(slotIndex, 0, currentHeader)
	boot.blockBootstrapper = stub

	return boot, stub
}

// Slot 0 must be claimable. Storing the index itself would treat the zero
// value as "already claimed at slot 0" and drop the first burst there.
func TestClaimSyncedStallSlot_IncludesSlotZero(t *testing.T) {
	t.Parallel()

	boot, _ := newStuckRequestBoot(0, headerAt(0, 7))

	require.True(t, boot.claimSyncedStallSlot(0))
	require.False(t, boot.claimSyncedStallSlot(0))
	require.True(t, boot.claimSyncedStallSlot(1))
}

const stallWarnMessage = "node believes it is synchronized but has not committed a block for a while"

// The warning fires only when computeNodeState asked for it; an honestly
// syncing node on the same path stays quiet. Not parallel: it registers a
// global log observer.
func TestRequestHeadersIfSyncIsStuck_WarnsOnlyWhenStalledWhileSynced(t *testing.T) {
	t.Run("warns when the node believed it was synchronized", func(t *testing.T) {
		recorder := observeWarn(t, stallWarnMessage)
		boot, _ := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

		fireStuck(boot, true)

		lines := recorder.snapshot()
		require.Len(t, lines, 1)
		require.Contains(t, lines[0], "slots since last committed block")
	})

	t.Run("stays silent while honestly syncing", func(t *testing.T) {
		counter := CountLogLines(t, stallWarnMessage)
		boot, stub := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

		fireStuck(boot, false)

		require.NotEmpty(t, stub.requestedNonces, "the burst itself must still fire")
		require.Zero(t, counter.Count())
	})
}

// signalingBlockBootstrapperStub closes done once target requests have been
// counted, giving the wiring tests a race-free point to observe the goroutine.
type signalingBlockBootstrapperStub struct {
	blockBootstrapperStub
	counted atomic.Int32
	target  int32
	done    chan struct{}
}

func (s *signalingBlockBootstrapperStub) requestHeaderByNonce(nonce uint64) {
	s.blockBootstrapperStub.requestHeaderByNonce(nonce)
	if s.counted.Add(1) == s.target {
		close(s.done)
	}
}

func (s *signalingBlockBootstrapperStub) arm(target int32) {
	s.target = target
	s.done = make(chan struct{})
}

func waitForRequests(t *testing.T, stub *signalingBlockBootstrapperStub) {
	t.Helper()

	select {
	case <-stub.done:
	case <-time.After(5 * time.Second):
		require.Failf(t, "burst did not finish", "counted %d, target %d", stub.counted.Load(), stub.target)
	}
}

func newComputeNodeStateBoot(slotIndex int64, probableHighest uint64, stub blockBootstrapper) *baseBootstrap {
	boot := newBootstrapForRequestGating(slotIndex, 0, headerAt(0, 7))
	boot.blockBootstrapper = stub
	boot.hasStarted = true
	boot.statusHandler = statusHandler.NewNilStatusHandler()
	boot.networkWatcher = &mock.MessengerStub{
		IsConnectedToTheNetworkCalled: func() bool { return true },
	}
	boot.forkDetector = &mock.ForkDetectorMock{
		CheckForkCalled:            func() *process.ForkInfo { return process.NewForkInfo() },
		ProbableHighestNonceCalled: func() uint64 { return probableHighest },
	}

	return boot
}

// Re-entering computeNodeState in the same slot must not send a second synced
// burst. The claim happens before the spawn, so a cleared
// isNodeStateCalculated flag is not enough to fire again.
func TestComputeNodeState_SyncedStallClaimsSlotAcrossReentry(t *testing.T) {
	t.Parallel()

	boot := newComputeNodeStateBoot(30, 7, &blockBootstrapperStub{})
	spawns := 0
	boot.spawnRequest = func(func()) { spawns++ }

	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	require.Equal(t, 1, spawns)

	boot.isNodeStateCalculated = false
	boot.computeNodeState()
	require.Equal(t, 1, spawns)

	boot.isNodeStateCalculated = false
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex++
	boot.computeNodeState()
	require.Equal(t, 2, spawns)
}

// A stalled synced node gets a forced-rollback pass on every proper slot,
// where tryRequest is false. That pass must not end the episode, or the warning
// repeats every SlotModulusTrigger slots. Not parallel: log observer.
func TestComputeNodeState_ForcedRollbackDoesNotRearmStallWarning(t *testing.T) {
	counter := CountLogLines(t, stallWarnMessage)

	stub := &signalingBlockBootstrapperStub{}
	stub.arm(process.MaxHeadersToRequestInAdvance)
	boot := newComputeNodeStateBoot(31, 7, stub)
	forcedRollback := false
	boot.forkDetector.(*mock.ForkDetectorMock).CheckForkCalled = func() *process.ForkInfo {
		forkInfo := process.NewForkInfo()
		forkInfo.IsDetected = forcedRollback
		return forkInfo
	}

	boot.computeNodeState()
	waitForRequests(t, stub)
	require.Equal(t, 1, counter.Count())

	// Proper slot: the fork detector forces a rollback, no burst is tried.
	forcedRollback = true
	boot.isNodeStateCalculated = false
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex = 35
	boot.computeNodeState()
	require.False(t, boot.isNodeSynchronized)

	// Same slot after the reset: synced and stalled again, no second line.
	forcedRollback = false
	boot.isNodeStateCalculated = false
	stub.arm(int32(process.MaxHeadersToRequestInAdvance) * 2)
	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	waitForRequests(t, stub)
	require.Equal(t, 1, counter.Count())
}

// A not-synced node still requests on every re-entry. The slot claim belongs
// to the synced stall path only.
func TestComputeNodeState_NotSyncedReentryStillRequests(t *testing.T) {
	t.Parallel()

	stub := &signalingBlockBootstrapperStub{}
	stub.arm(process.MaxHeadersToRequestInAdvance)
	// Probable highest nonce above the committed nonce keeps the node unsynced.
	boot := newComputeNodeStateBoot(30, 100, stub)

	boot.computeNodeState()
	require.False(t, boot.isNodeSynchronized)
	waitForRequests(t, stub)

	boot.isNodeStateCalculated = false
	stub.arm(int32(process.MaxHeadersToRequestInAdvance) * 2)
	boot.computeNodeState()
	waitForRequests(t, stub)
	require.Equal(t, int32(process.MaxHeadersToRequestInAdvance)*2, stub.counted.Load())
}

// The synced stall warning is one line for the whole stretch, then fires again
// after the lag drops and the stall returns. Not parallel: log observer.
func TestComputeNodeState_SyncedStallWarnsOncePerEpisode(t *testing.T) {
	counter := CountLogLines(t, stallWarnMessage)

	stub := &signalingBlockBootstrapperStub{}
	stub.arm(process.MaxHeadersToRequestInAdvance)
	boot := newComputeNodeStateBoot(30, 7, stub)

	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	waitForRequests(t, stub)
	require.Equal(t, 1, counter.Count())

	boot.isNodeStateCalculated = false
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex++
	stub.arm(int32(process.MaxHeadersToRequestInAdvance) * 2)
	boot.computeNodeState()
	waitForRequests(t, stub)
	require.Equal(t, 1, counter.Count())

	// Caught up: the next pass clears the latch and does not request.
	boot.isNodeStateCalculated = false
	boot.chainHandler = &mock.BlockChainMock{
		GetGenesisHeaderCalled: func() data.HeaderHandler { return headerAt(0, 0) },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return headerAt(uint64(boot.slotManager.Index()), 7)
		},
	}
	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	require.Equal(t, int32(process.MaxHeadersToRequestInAdvance)*2, stub.counted.Load())

	// Stale again. A new episode warns.
	boot.isNodeStateCalculated = false
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex += 20
	boot.chainHandler = &mock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return headerAt(0, 0) },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return headerAt(0, 7) },
	}
	stub.arm(int32(process.MaxHeadersToRequestInAdvance) * 3)
	boot.computeNodeState()
	waitForRequests(t, stub)
	require.Equal(t, 2, counter.Count())
}

// The burst runs after computeNodeState decided it. If the node commits the
// missing blocks in between, the burst must see the fresh tip and stand down
// instead of requesting nonces that do not exist yet. Not parallel: log observer.
func TestComputeNodeState_DelayedBurstRevalidatesLag(t *testing.T) {
	counter := CountLogLines(t, stallWarnMessage)

	stub := &blockBootstrapperStub{}
	boot := newComputeNodeStateBoot(30, 7, stub)
	var held func()
	boot.spawnRequest = func(run func()) { held = run }

	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	require.NotNil(t, held)

	// Caught up before the burst runs.
	boot.chainHandler = &mock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return headerAt(0, 0) },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return headerAt(30, 32) },
	}
	held()

	require.Empty(t, stub.requestedNonces)
	require.Zero(t, counter.Count())
}

// A pass that straddles a slot boundary must claim the slot it snapshotted.
// Claiming the next slot would suppress that slot's own burst.
func TestComputeNodeState_SyncedStallClaimsSnapshottedSlot(t *testing.T) {
	t.Parallel()

	boot := newComputeNodeStateBoot(30, 7, &blockBootstrapperStub{})
	spawns := 0
	boot.spawnRequest = func(func()) { spawns++ }
	slotManager := boot.slotManager.(*consensusMock.SlotManagerMock)
	// Published right after the snapshot: the chronology ticks mid-pass.
	boot.statusHandler = &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, _ uint64) {
			if key == core.MetricIsSyncing {
				slotManager.SlotIndex = 31
			}
		},
	}

	boot.computeNodeState()
	require.Equal(t, int64(30), boot.slotIndex)
	require.Equal(t, 1, spawns)

	boot.isNodeStateCalculated = false
	boot.computeNodeState()
	require.Equal(t, int64(31), boot.slotIndex)
	require.Equal(t, 2, spawns, "slot 31 must still get its burst")
}

// A burst that stands down because one block landed in between must hand its
// warning back. Otherwise no pass sees the lag clear, the latch stays held and
// the rest of the stall is silent. Not parallel: log observer.
func TestComputeNodeState_StoodDownBurstReleasesStallWarning(t *testing.T) {
	counter := CountLogLines(t, stallWarnMessage)

	stub := &blockBootstrapperStub{}
	boot := newComputeNodeStateBoot(30, 7, stub)
	tip := headerAt(19, 7)
	boot.chainHandler = &mock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return headerAt(0, 0) },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return tip },
	}
	var held func()
	boot.spawnRequest = func(run func()) { held = run }

	// Slot 30, tip slot 19: lag 11 claims the warning.
	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)
	require.NotNil(t, held)

	// One block lands before the burst runs: lag 10, it stands down.
	tip = headerAt(20, 8)
	held()
	require.Empty(t, stub.requestedNonces)
	require.Zero(t, counter.Count())

	// Slot 31: lag 11 again on the same stall, and it must warn.
	held = nil
	boot.isNodeStateCalculated = false
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex = 31
	boot.computeNodeState()
	require.NotNil(t, held)
	held()

	require.Equal(t, 1, counter.Count())
	require.Equal(t, uint64(9), stub.requestedNonces[0])
}

// The lag must use the slot snapshot too. Tip slot 20 puts slot 30 at the
// threshold and slot 31 past it, so a tick mid-pass must not start a burst.
func TestComputeNodeState_LagUsesSnapshottedSlot(t *testing.T) {
	t.Parallel()

	boot := newComputeNodeStateBoot(30, 7, &blockBootstrapperStub{})
	boot.chainHandler = &mock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return headerAt(0, 0) },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return headerAt(20, 7) },
	}
	spawns := 0
	boot.spawnRequest = func(func()) { spawns++ }
	slotManager := boot.slotManager.(*consensusMock.SlotManagerMock)
	boot.statusHandler = &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, _ uint64) {
			if key == core.MetricIsSyncing {
				slotManager.SlotIndex = 31
			}
		},
	}

	boot.computeNodeState()
	require.Equal(t, int64(30), boot.slotIndex)
	require.Zero(t, spawns)

	boot.isNodeStateCalculated = false
	boot.computeNodeState()
	require.Equal(t, int64(31), boot.slotIndex)
	require.Equal(t, 1, spawns)
}
