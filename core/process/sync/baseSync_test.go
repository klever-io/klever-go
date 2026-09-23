package sync

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
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
	// shadowed is another header for the queued nonce, appended to the pool before
	// the queued one: it is picked once the queued header is removed from the pool
	shadowed *block.Block

	processBlockFailHook func()

	finalNonce         uint64
	processBlockHook   func()
	numProcessBlock    int
	numReverts         int
	numRemovedFromPool int
	numRemovedFromFork int
	numResetProbable   int
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
			if sim.shadowed != nil {
				sim.queued, sim.shadowed = sim.shadowed, nil
			} else {
				sim.queuedInPool = false
			}
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
		hasher:         hasher,
		marshalizer:    marshalizer,
		forkDetector:   forkDetector,
		requestHandler: &mock.RequestHandlerStub{},
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

func TestSyncBlock_FarFutureHeaderShouldBeDroppedWithoutRollBack(t *testing.T) {
	sim, boot := newSyncSim(t)

	// The pool holds the honest nonce 6 and, appended last, a nonce 6 dated far
	// beyond any chronology drift: the far-future one is picked first.
	sim.shadowed = sim.queued
	sim.queued = newHeader(6, uint64(sim.index())+1000, []byte("h5"))

	require.ErrorIs(t, boot.syncBlock(), process.ErrSlotAheadOfChronology)

	sim.mut.Lock()
	assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must not be rolled back")
	assert.Equal(t, 0, sim.numReverts)
	assert.Equal(t, 1, sim.numRemovedFromPool, "the far-future header must be dropped from the pool")
	assert.Equal(t, 1, sim.numRemovedFromFork, "the far-future header must be dropped from the fork detector")
	assert.Equal(t, 0, sim.numResetProbable)
	assert.Equal(t, uint32(0), boot.GetNumSyncedWithErrorsForNonce(6))
	sim.mut.Unlock()

	// The honest header behind it is synced right away, within the same slot.
	require.NoError(t, boot.syncBlock())
	assert.Equal(t, uint64(6), sim.currentHeader().GetNonce())
	assert.Equal(t, 0, sim.numReverts)
}

func TestDoJobOnSyncBlockFail_SlotAheadBoundary(t *testing.T) {
	t.Run("exactly maxSlotsAhead is postponed", func(t *testing.T) {
		sim, boot := newSyncSim(t)
		hdr := newHeader(6, uint64(sim.index())+maxSlotsAheadToPostponeSync, []byte("h5"))

		boot.doJobOnSyncBlockFail(hdr, process.ErrSlotAheadOfChronology, sim.index())

		assert.True(t, boot.isSyncPostponed())
		assertNothingDiscarded(t, sim, boot)
	})
	t.Run("one past maxSlotsAhead is dropped", func(t *testing.T) {
		sim, boot := newSyncSim(t)
		hdr := newHeader(6, uint64(sim.index())+maxSlotsAheadToPostponeSync+1, []byte("h5"))

		boot.doJobOnSyncBlockFail(hdr, process.ErrSlotAheadOfChronology, sim.index())

		assert.False(t, boot.isSyncPostponed())
		sim.mut.Lock()
		defer sim.mut.Unlock()
		assert.Equal(t, uint64(5), sim.current.GetNonce(), "the committed block must not be rolled back")
		assert.Equal(t, 0, sim.numReverts)
		assert.Equal(t, 1, sim.numRemovedFromPool)
		assert.Equal(t, 1, sim.numRemovedFromFork)
		assert.Equal(t, uint32(0), boot.GetNumSyncedWithErrorsForNonce(6))
	})
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
