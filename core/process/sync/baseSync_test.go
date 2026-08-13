package sync

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
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
		// Populated even for the predicate tests: shouldTryToRequestHeaders logs
		// the fork detector's nonce, so a nil here panics instead of failing.
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
		boot.requestHeadersIfSyncIsStuck()

		assert.Empty(t, stub.requestedNonces)
	})

	t.Run("requests lag-1 headers just past the threshold", func(t *testing.T) {
		t.Parallel()

		// Lag 11 -> min(20, 10) = 10 headers, starting at nonce 8.
		boot, stub := newBoot(int64(process.MaxSlotsWithoutNewBlockReceived)+1, headerAt(0, 7))
		boot.requestHeadersIfSyncIsStuck()

		assert.Equal(t, []uint64{8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, stub.requestedNonces)
	})

	t.Run("caps the burst at MaxHeadersToRequestInAdvance", func(t *testing.T) {
		t.Parallel()

		// Lag 500 would ask for 499, the cap holds it to 20.
		boot, stub := newBoot(500, headerAt(0, 7))
		boot.requestHeadersIfSyncIsStuck()

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
		boot.requestHeadersIfSyncIsStuck()

		assert.Empty(t, stub.requestedNonces)
	})
}
