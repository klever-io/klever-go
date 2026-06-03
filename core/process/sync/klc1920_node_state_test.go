package sync_test

import (
	"sync"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	syncpkg "github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

// klc1920_node_state_test.go covers the new branch in
// baseBootstrap.computeNodeState: when HighestNonceReceived is more than
// BlockFinality blocks ahead of currentBlockNonce, hasLastBlock is forced
// to false so isNodeSynchronized correctly reports the node is behind.
//
// Without this branch, a fallback whose BHReceived path is broken (peer
// churn after election) would have probableHighestNonce == currentBlockNonce
// and falsely declare itself synced — the production failure mode KLC-1920
// and KLC-2389 describe.

type observableStatusHandler struct {
	mu        sync.Mutex
	isSyncing uint64
}

func (o *observableStatusHandler) Increment(_ string)             {}
func (o *observableStatusHandler) AddUint64(_ string, _ uint64)   {}
func (o *observableStatusHandler) Decrement(_ string)             {}
func (o *observableStatusHandler) SetInt64Value(_ string, _ int64) {}
func (o *observableStatusHandler) SetUInt64Value(key string, value uint64) {
	if key != core.MetricIsSyncing {
		return
	}
	o.mu.Lock()
	o.isSyncing = value
	o.mu.Unlock()
}
func (o *observableStatusHandler) SetStringValue(_ string, _ string) {}
func (o *observableStatusHandler) Close()                            {}
func (o *observableStatusHandler) IsInterfaceNil() bool              { return o == nil }

func (o *observableStatusHandler) IsSyncing() uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.isSyncing
}

func buildKLC1920Bootstrap(probable, highest, currentBlockNonce uint64) (*syncpkg.BaseBootstrap, *observableStatusHandler) {
	forkDetector := &commonMock.ForkDetectorMock{
		CheckForkCalled:                 func() *process.ForkInfo { return &process.ForkInfo{} },
		ProbableHighestNonceCalled:      func() uint64 { return probable },
		HighestNonceReceivedCalled:      func() uint64 { return highest },
		GetHighestFinalBlockNonceCalled: func() uint64 { return 0 },
	}

	genesisHeader := &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 0}}
	currentHeader := &block.Block{Header: &block.BlockHeader{Nonce: currentBlockNonce, Slot: currentBlockNonce}}

	chainHandler := &commonMock.BlockChainMock{
		GetGenesisHeaderCalled:      func() data.HeaderHandler { return genesisHeader },
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return currentHeader },
	}

	slotManager := &consensusMock.SlotManagerMock{
		SlotIndex:           int64(currentBlockNonce + 5),
		TimeDurationCalled:  func() time.Duration { return 0 },
		BeforeGenesisCalled: func() bool { return true }, // suppress requestHeadersIfSyncIsStuck path
	}

	networkWatcher := &commonMock.MessengerStub{
		IsConnectedToTheNetworkCalled: func() bool { return true },
	}
	statusHandler := &observableStatusHandler{}

	boot := syncpkg.NewBaseBootstrapForKLC1920Test(
		forkDetector,
		chainHandler,
		slotManager,
		networkWatcher,
		statusHandler,
	)

	return boot, statusHandler
}

// TestKLC1920_ComputeNodeState_GossipAheadForcesNotSynced is the regression
// guard for the synced-state gate. Pre-fix: with probable == current the
// node declared itself synced even when HighestNonceReceived was 20 blocks
// ahead. Post-fix: any gossip-vs-current gap > BlockFinality forces
// hasLastBlock=false and isNodeSynchronized=false.
func TestKLC1920_ComputeNodeState_GossipAheadForcesNotSynced(t *testing.T) {
	t.Parallel()

	// Production failure shape: probable matches current (fork detector
	// thinks it's caught up) but gossip has reported headers 20 ahead.
	boot, statusHandler := buildKLC1920Bootstrap(
		uint64(50), // probable
		uint64(70), // highest received from gossip
		uint64(50), // current block nonce
	)

	boot.ComputeNodeState()

	assert.False(t, boot.HasLastBlock(),
		"KLC-1920 fix: gossip-ahead gap must force hasLastBlock=false")
	assert.False(t, boot.IsNodeSynchronized(),
		"KLC-1920 fix: node must not declare synced when gossip is ahead")
	assert.Equal(t, uint64(1), statusHandler.IsSyncing(),
		"KLC-1920 fix: klv_is_syncing must report 1 — the production-bug metric was 0 (false-synced)")
}

// TestKLC1920_ComputeNodeState_GossipWithinFinalityStaysSynced confirms the
// gate does NOT spuriously fire during normal proposal rounds where gossip
// is briefly one block ahead of the just-committed block.
func TestKLC1920_ComputeNodeState_GossipWithinFinalityStaysSynced(t *testing.T) {
	t.Parallel()

	// Normal cycle: a BHProposed for nonce N+1 has arrived but the block
	// hasn't committed yet. gap = 1 = BlockFinality — must NOT fire.
	boot, statusHandler := buildKLC1920Bootstrap(
		uint64(50), // probable
		uint64(51), // highest received (one proposal ahead — normal)
		uint64(50), // current block nonce
	)

	boot.ComputeNodeState()

	assert.True(t, boot.HasLastBlock(),
		"normal proposal cycle: gap == BlockFinality must NOT force not-synced")
	assert.True(t, boot.IsNodeSynchronized(),
		"normal proposal cycle: node remains synced; consensus must not be gated")
	assert.Equal(t, uint64(0), statusHandler.IsSyncing(),
		"normal proposal cycle: klv_is_syncing stays 0")
}
