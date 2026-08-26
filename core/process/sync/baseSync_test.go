package sync

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage"
)

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
		boot.doJobOnSyncBlockFail(header, process.ErrTimeIsOut)

		assert.False(t, *rolledBack)
	})

	t.Run("wrapped timeout does not roll back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(header, fmt.Errorf("process block: %w", process.ErrTimeIsOut))

		assert.False(t, *rolledBack)
	})

	t.Run("any other error rolls back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(header, errors.New("something else"))

		assert.True(t, *rolledBack)
	})

	t.Run("no header means processing never started, so no roll back", func(t *testing.T) {
		t.Parallel()

		boot, rolledBack := newBoot()
		boot.doJobOnSyncBlockFail(nil, errors.New("something else"))

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
