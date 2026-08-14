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
	numRemovedFromPool int
	numRemovedFromFork int
	numResetProbable   int
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

func headerAtSlot(slot uint64) data.HeaderHandler {
	return &block.Block{Header: &block.BlockHeader{Slot: slot}}
}

func headerAt(slot uint64, nonce uint64) data.HeaderHandler {
	return &block.Block{Header: &block.BlockHeader{Slot: slot, Nonce: nonce}}
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
				return headerAtSlot(genesisSlot)
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
		boot := newBootstrapForRequestGating(slotIndex, 0, headerAtSlot(lastCommittedSlot))
		boot.isNodeSynchronized = true

		// #nosec G115 -- slotIndex is a small positive loop bound
		expected := uint64(slotIndex) > process.MaxSlotsWithoutNewBlockReceived

		assert.Equal(t, expected, boot.shouldTryToRequestHeaders(),
			"slot index %d, lag %d", slotIndex, slotIndex)
	}
}

// Import mode replays historical blocks, so the slot lag is unbounded by
// construction and there are no peers to request from. It also lacks the
// once-per-slot latch the branch relies on, since GetNodeState always answers
// NsNotSynchronized while importing, which would put the warning and the request
// goroutine on the 5 ms sync loop instead.
func TestShouldTryToRequestHeaders_ImportModeIgnoresSlotLag(t *testing.T) {
	t.Parallel()

	boot := newBootstrapForRequestGating(100, 0, headerAtSlot(0))
	boot.isNodeSynchronized = true
	boot.isInImportMode = true

	assert.False(t, boot.shouldTryToRequestHeaders())

	// The very same lag outside import mode does trigger a request, so the
	// assertion above cannot pass for the wrong reason.
	boot.isInImportMode = false
	assert.True(t, boot.shouldTryToRequestHeaders())
}

func TestShouldTryToRequestHeaders_NotSynchronizedAlwaysRequests(t *testing.T) {
	t.Parallel()

	// No lag at all, so only the pre-existing unsynchronized branch can be what
	// returns true here.
	boot := newBootstrapForRequestGating(3, 0, headerAtSlot(3))
	boot.isNodeSynchronized = false

	assert.True(t, boot.shouldTryToRequestHeaders())
}

func TestShouldTryToRequestHeaders_GuardsShortCircuitBeforeSlotLag(t *testing.T) {
	t.Parallel()

	// Every case below carries a lag far past the threshold, so a true result
	// would prove the guard no longer runs before the slot-lag check.
	const laggingSlotIndex = int64(100)

	t.Run("before genesis", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAtSlot(0))
		boot.isNodeSynchronized = true
		boot.slotManager = &consensusMock.SlotManagerMock{
			SlotIndex:           laggingSlotIndex,
			BeforeGenesisCalled: func() bool { return true },
		}

		assert.False(t, boot.shouldTryToRequestHeaders())
	})

	t.Run("forced roll back one block", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAtSlot(0))
		boot.isNodeSynchronized = true
		boot.forkInfo = &process.ForkInfo{IsDetected: true, Nonce: math.MaxUint64}

		assert.False(t, boot.shouldTryToRequestHeaders())
	})

	t.Run("forced roll back to nonce", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(laggingSlotIndex, 0, headerAtSlot(0))
		boot.isNodeSynchronized = true
		boot.forkInfo = &process.ForkInfo{IsDetected: true, Slot: math.MaxUint64}

		assert.False(t, boot.shouldTryToRequestHeaders())
	})
}

func TestSlotsSinceLastCommittedBlock(t *testing.T) {
	t.Parallel()

	t.Run("uses the current header slot", func(t *testing.T) {
		t.Parallel()

		boot := newBootstrapForRequestGating(30, 0, headerAtSlot(12))

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

		boot := newBootstrapForRequestGating(50, 0, headerAtSlot(100))

		assert.Equal(t, uint64(0), boot.slotsSinceLastCommittedBlock())

		boot.isNodeSynchronized = true
		assert.False(t, boot.shouldTryToRequestHeaders())
	})
}

// requestHeadersIfSyncIsStuck is what actually drives recovery once the node
// notices it is behind, and the size of each burst is what operators observe in
// the logs. This pins that formula: min(MaxHeadersToRequestInAdvance, lag-1)
// starting at the nonce after the last committed block (issue #90).
func TestRequestHeadersIfSyncIsStuck(t *testing.T) {
	t.Parallel()

	newBoot := func(slotIndex int64, currentHeader data.HeaderHandler) (*baseBootstrap, *blockBootstrapperStub) {
		stub := &blockBootstrapperStub{}
		boot := newBootstrapForRequestGating(slotIndex, 0, currentHeader)
		boot.blockBootstrapper = stub

		return boot, stub
	}

	t.Run("does nothing at or below the threshold", func(t *testing.T) {
		t.Parallel()

		// Lag is exactly MaxSlotsWithoutNewBlockReceived, so the guard holds.
		boot, stub := newBoot(int64(process.MaxSlotsWithoutNewBlockReceived), headerAt(0, 7))
		boot.requestHeadersIfSyncIsStuck(false)

		assert.Empty(t, stub.requestedNonces)
	})

	t.Run("requests lag-1 headers just past the threshold", func(t *testing.T) {
		t.Parallel()

		// Lag 11 -> min(20, 10) = 10 headers, starting at nonce 8.
		boot, stub := newBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))
		boot.requestHeadersIfSyncIsStuck(false)

		assert.Equal(t, []uint64{8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, stub.requestedNonces)
	})

	t.Run("caps the burst at MaxHeadersToRequestInAdvance", func(t *testing.T) {
		t.Parallel()

		// Lag 500 would ask for 499, the cap holds it to 20.
		boot, stub := newBoot(500, headerAt(0, 7))
		boot.requestHeadersIfSyncIsStuck(false)

		assert.Len(t, stub.requestedNonces, process.MaxHeadersToRequestInAdvance)
		assert.Equal(t, uint64(8), stub.requestedNonces[0])
		assert.Equal(t, uint64(27), stub.requestedNonces[len(stub.requestedNonces)-1])
	})

	// Without the underflow guard in slotsSinceLastCommittedBlock the subtraction
	// wraps to a huge uint64, the threshold is trivially cleared, and the node
	// requests a full burst of nonces that cannot exist yet.
	t.Run("requests nothing when the committed block is ahead of the slot index", func(t *testing.T) {
		t.Parallel()

		boot, stub := newBoot(50, headerAt(100, 7))
		boot.requestHeadersIfSyncIsStuck(false)

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

// The once-per-slot cap must be enforced by the function itself, not by how
// often callers happen to invoke it: before lastStuckRequestSlot existed the
// bound depended on computeNodeState's memoization, which a refactor in
// syncBlock could silently remove with every test still green.
func TestRequestHeadersIfSyncIsStuck_FiresAtMostOncePerSlot(t *testing.T) {
	t.Parallel()

	boot, stub := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

	boot.requestHeadersIfSyncIsStuck(false)
	firstBurst := len(stub.requestedNonces)
	assert.Equal(t, 10, firstBurst)

	// Same slot: a second invocation must not produce a second burst.
	boot.requestHeadersIfSyncIsStuck(false)
	assert.Len(t, stub.requestedNonces, firstBurst)

	// Next slot: the cap releases and the burst fires again.
	boot.slotManager.(*consensusMock.SlotManagerMock).SlotIndex++
	boot.requestHeadersIfSyncIsStuck(false)
	assert.Greater(t, len(stub.requestedNonces), firstBurst)
}

type stallWarnFormatter struct {
	logger.PlainFormatter
}

func (f *stallWarnFormatter) Output(line logger.LogLineHandler) []byte {
	if line.GetMessage() != "node believes it is synchronized but has not committed a block for a while" {
		return nil
	}

	return f.PlainFormatter.Output(line)
}

// The warning must fire exactly when the burst was decided while the node
// believed it was synchronized: that state advertises NsSynchronized with
// MetricIsSyncing at 0, so this line is its only signal. An honestly syncing
// node passing through the same code path is just catching up and must stay
// quiet. Not parallel: it registers a global log observer.
func TestRequestHeadersIfSyncIsStuck_WarnsOnlyWhenStalledWhileSynced(t *testing.T) {
	buff := &bytes.Buffer{}
	require.Nil(t, logger.AddLogObserver(buff, &stallWarnFormatter{}))
	t.Cleanup(func() {
		require.Nil(t, logger.RemoveLogObserver(buff))
	})

	t.Run("warns when the node believed it was synchronized", func(t *testing.T) {
		buff.Reset()
		boot, _ := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

		boot.requestHeadersIfSyncIsStuck(true)

		require.Contains(t, buff.String(), "node believes it is synchronized")
		require.Contains(t, buff.String(), "slots since last committed block")
	})

	t.Run("stays silent while honestly syncing", func(t *testing.T) {
		buff.Reset()
		boot, stub := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

		boot.requestHeadersIfSyncIsStuck(false)

		require.NotEmpty(t, stub.requestedNonces, "the burst itself must still fire")
		require.Empty(t, buff.String())
	})

	t.Run("warning shares the once-per-slot cap", func(t *testing.T) {
		buff.Reset()
		boot, _ := newStuckRequestBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))

		boot.requestHeadersIfSyncIsStuck(true)
		firstLen := buff.Len()
		require.Greater(t, firstLen, 0)

		boot.requestHeadersIfSyncIsStuck(true)
		require.Equal(t, firstLen, buff.Len())
	})
}

// signalingBlockBootstrapperStub closes done once the full burst has been
// requested, giving the wiring test below a race-free point to observe the
// goroutine's output.
type signalingBlockBootstrapperStub struct {
	blockBootstrapperStub
	remaining int
	done      chan struct{}
}

func (s *signalingBlockBootstrapperStub) requestHeaderByNonce(nonce uint64) {
	s.blockBootstrapperStub.requestHeaderByNonce(nonce)
	s.remaining--
	if s.remaining == 0 {
		close(s.done)
	}
}

// Pins the spawn-site wiring in computeNodeState: the goroutine must receive
// the freshly computed isNodeSynchronized. Every other test injects that flag
// directly, so with the argument hard-wired to false at the spawn site the
// package would stay green while the stalled state loses its only signal.
// Not parallel: it registers a global log observer.
func TestComputeNodeState_StalledWhileSyncedWiresWarnIntoBurst(t *testing.T) {
	buff := &bytes.Buffer{}
	require.Nil(t, logger.AddLogObserver(buff, &stallWarnFormatter{}))
	t.Cleanup(func() {
		require.Nil(t, logger.RemoveLogObserver(buff))
	})

	stub := &signalingBlockBootstrapperStub{
		remaining: process.MaxHeadersToRequestInAdvance,
		done:      make(chan struct{}),
	}

	// Last committed block at slot 0 / nonce 7 while the slot index is 30: a
	// stalled node. The fork detector reports no fork and a probable highest
	// nonce equal to the committed one, which is exactly the frozen state that
	// computes isNodeSynchronized as true.
	boot := newBootstrapForRequestGating(30, 0, headerAt(0, 7))
	boot.blockBootstrapper = stub
	boot.hasStarted = true
	boot.statusHandler = statusHandler.NewNilStatusHandler()
	boot.networkWatcher = &mock.MessengerStub{
		IsConnectedToTheNetworkCalled: func() bool { return true },
	}
	boot.forkDetector = &mock.ForkDetectorMock{
		CheckForkCalled:            func() *process.ForkInfo { return process.NewForkInfo() },
		ProbableHighestNonceCalled: func() uint64 { return 7 },
	}

	boot.computeNodeState()
	require.True(t, boot.isNodeSynchronized)

	select {
	case <-stub.done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the stuck-recovery burst never fired")
	}

	require.Contains(t, buff.String(), "node believes it is synchronized but has not committed a block for a while")
}
