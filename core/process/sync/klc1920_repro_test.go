package sync_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

// klc1920_repro_test.go pins down the invariant the KLC-1920 fix relies on:
// under the production failure mode (only BHProposed deliveries arrive, the
// BHReceived path is broken by peer churn after an election), the fork
// detector's HighestNonceReceived must advance with gossip while
// ProbableHighestNonce stays at the last processed nonce. The gap between
// them is what baseBootstrap.computeNodeState uses to force hasLastBlock=false
// and prevent the false isNodeSynchronized=true reported in the Slack-thread
// log at sprint-97/KLC-1920/slack-thread/log.txt.

func newSlotManagerForRepro(slot int64) *consensusMock.SlotManagerMock {
	return &consensusMock.SlotManagerMock{
		SlotIndex:          slot,
		TimeDurationCalled: func() time.Duration { return 0 },
	}
}

// TestKLC1920_HighestNonceReceivedAdvancesUnderBHProposedOnly is the
// regression guard for the gossip-ceiling invariant. Production logs showed
// `setHighestNonceReceived` firing constantly while `forkDetector.AddHeader
// state=0` (BHReceived) never appeared. This test reproduces exactly that
// shape and asserts both sides of the gap are observable.
func TestKLC1920_HighestNonceReceivedAdvancesUnderBHProposedOnly(t *testing.T) {
	t.Parallel()

	bfd, err := sync.NewMetaForkDetector(newSlotManagerForRepro(100), &mock.BlackListHandlerStub{}, 0)
	assert.Nil(t, err)
	assert.NotNil(t, bfd)

	processedHdr := &block.BlockHeader{Nonce: 10, Slot: 10}
	err = bfd.AddHeader(&block.Block{Header: processedHdr}, []byte("processed-10"), process.BHProcessed, nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, uint64(10), bfd.ProbableHighestNonce(),
		"baseline: probable highest after BHProcessed at 10")
	assert.Equal(t, uint64(10), bfd.HighestNonceReceived(),
		"baseline: highest received tracks the same processed nonce")

	for nonce := uint64(11); nonce <= uint64(15); nonce++ {
		hdr := &block.BlockHeader{Nonce: nonce, Slot: nonce}
		hash := []byte(fmt.Sprintf("proposed-%d", nonce))
		err = bfd.AddHeader(&block.Block{Header: hdr}, hash, process.BHProposed, nil, nil)
		assert.Nil(t, err)
	}

	assert.Equal(t, uint64(15), bfd.HighestNonceReceived(),
		"gossip ceiling must reflect every BHProposed delivery")
	assert.Equal(t, uint64(10), bfd.ProbableHighestNonce(),
		"probableHighestNonce intentionally stays at last processed — BHProposed must not advance it (would break consensus during proposal rounds)")

	gap := bfd.HighestNonceReceived() - bfd.ProbableHighestNonce()
	assert.Equal(t, uint64(5), gap,
		"the gap between gossip ceiling and probable is the signal computeNodeState uses to force hasLastBlock=false when it exceeds BlockFinality")
}

// TestKLC1920_GapExceedsBlockFinality demonstrates that the gap threshold
// (HighestNonceReceived - currentBlockNonce > BlockFinality) is the
// condition the fix watches for. BlockFinality is 1, so any gap >= 2
// indicates the node is not really synced.
func TestKLC1920_GapExceedsBlockFinality(t *testing.T) {
	t.Parallel()

	bfd, err := sync.NewMetaForkDetector(newSlotManagerForRepro(100), &mock.BlackListHandlerStub{}, 0)
	assert.Nil(t, err)

	processedHdr := &block.BlockHeader{Nonce: 50, Slot: 50}
	err = bfd.AddHeader(&block.Block{Header: processedHdr}, []byte("p-50"), process.BHProcessed, nil, nil)
	assert.Nil(t, err)

	for nonce := uint64(51); nonce <= uint64(70); nonce++ {
		hdr := &block.BlockHeader{Nonce: nonce, Slot: nonce}
		hash := []byte(fmt.Sprintf("g-%d", nonce))
		err = bfd.AddHeader(&block.Block{Header: hdr}, hash, process.BHProposed, nil, nil)
		assert.Nil(t, err)
	}

	currentBlockNonce := uint64(50)
	gossipGap := bfd.HighestNonceReceived() - currentBlockNonce
	assert.Equal(t, uint64(20), gossipGap,
		"matches Slack-log production amplitude (~70-block gap) at scale")
	assert.True(t, gossipGap > uint64(process.BlockFinality),
		"gap exceeds BlockFinality — computeNodeState must declare not-synced")
}
