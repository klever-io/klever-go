package epochStart

import (
	"testing"
	"time"

	mclsinglesig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-2814: the epoch start block of slot S is processed by every node but never committed, so
// the slot times out and Worker.Extend reverts it. The leader proposes the epoch start again at
// S+1 and the other nodes take it over the blocks topic. Their interceptor must accept it: a
// trigger left in epoch start at S rejected it, and those nodes stayed behind until restart.
func TestEpochStart_WithheldEpochStartSlotIsRetriedOnTheNextSlot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	const xidProposer = 0

	nodes, _, err := commonTxTest.CreateStandardSetupForTxTests(1)
	require.NoError(t, err)
	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()
	time.Sleep(300 * time.Millisecond)

	// the processor nodes run 6 slots per epoch, so slot 6 is the first slot of epoch 1
	slot, nonce := uint64(1), uint64(1)
	for slot < 6 {
		slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposer, slot, nonce)
		require.NoError(t, err)
	}

	withholdEpochStartBlock(t, nodes, xidProposer, slot, nonce)

	slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposer, slot+1, nonce)
	require.NoError(t, err)
	for i, n := range nodes {
		head := n.Blkc.GetCurrentBlockHeader()
		require.NotNil(t, head, "node %d", i)
		assert.Equal(t, nonce-1, head.GetNonce(), "node %d did not commit the retried epoch start block", i)
		assert.True(t, head.GetIsEpochStart(), "node %d", i)
		assert.Equal(t, uint32(1), head.GetEpoch(), "node %d", i)
	}

	// the chain runs on in the new epoch
	for i := 0; i < 2; i++ {
		slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposer, slot, nonce)
		require.NoError(t, err)
	}
	for i, n := range nodes {
		head := n.Blkc.GetCurrentBlockHeader()
		require.NotNil(t, head, "node %d", i)
		assert.Equal(t, nonce-1, head.GetNonce(), "node %d", i)
		assert.Equal(t, uint32(1), head.GetEpoch(), "node %d", i)
	}
}

// withholdEpochStartBlock has the leader create and sign the epoch start block of slot, and
// every other node process it as the consensus participants do. Nobody commits it: each node
// then reverts it the way Worker.Extend does when the slot times out.
func withholdEpochStartBlock(t *testing.T, nodes []*processorNode.ProcessorNode, xidProposer int, slot uint64, nonce uint64) {
	t.Helper()

	currHdr := nodes[xidProposer].Blkc.GetCurrentBlockHeader()
	pubKeys, err := nodes[xidProposer].NodesCoordinator.GetConsensusValidatorsPublicKeys(currHdr.GetRandSeed(), slot, currHdr.GetEpoch())
	require.NoError(t, err)
	consensusNodes := processorNode.SelectTestNodesForPubKeys(nodes, pubKeys)
	integrationTest.UpdateSlot(consensusNodes, slot)

	header, err := processorNode.ProposeBlockWithConsensusSigs(consensusNodes, pubKeys, xidProposer, slot, nonce)
	require.NoError(t, err)
	header, err = consensusNodes[xidProposer].FillHeaderFields(consensusNodes[xidProposer], header, &mclsinglesig.BlsSingleSigner{})
	require.NoError(t, err)
	require.True(t, header.GetIsEpochStart(), "slot %d must start the epoch", slot)

	for i, n := range consensusNodes {
		if i == xidProposer {
			continue
		}
		// each node decodes its own copy of a proposal, and processing writes into it
		err = n.BlockProcessor.ProcessBlock(header.Clone(), func() time.Duration { return 4 * time.Second })
		require.NoError(t, err, "node %d", i)
	}

	for _, n := range consensusNodes {
		n.BlockProcessor.RevertStateToSnapshot(header)
	}
}
