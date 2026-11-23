package consensus

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/stretchr/testify/require"
)

// rollbackForkDetector makes a forced cluster-wide rollback final for this
// harness. An already dispatched header callback can otherwise reinsert the
// discarded block after RemoveHeader, making sync wait for a block that no peer
// has anymore. Only hashes explicitly discarded by the test are suppressed;
// normal sync removals and replacement blocks retain their original behavior.
type rollbackForkDetector struct {
	process.ForkDetector
	mu        sync.Mutex
	discarded map[string]struct{}
}

// AddHeader ignores explicitly discarded hashes and forwards every other header.
func (fd *rollbackForkDetector) AddHeader(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, notarized []data.HeaderHandler, notarizedHashes [][]byte) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if _, discarded := fd.discarded[string(hash)]; discarded {
		return nil
	}
	return fd.ForkDetector.AddHeader(header, hash, state, notarized, notarizedHashes)
}

// RemoveHeader preserves normal sync error recovery: a removed hash may be retried.
func (fd *rollbackForkDetector) RemoveHeader(nonce uint64, hash []byte) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.ForkDetector.RemoveHeader(nonce, hash)
}

// DiscardHeader excludes this exact hash from late delivery during a forced rollback.
func (fd *rollbackForkDetector) DiscardHeader(hash []byte) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.discarded[string(hash)] = struct{}{}
}

// discardBlockForRollback marks the current block on every node before any node
// reverts, so late callbacks cannot resurrect it during the cluster-wide rollback.
func discardBlockForRollback(t *testing.T, nodes []*processorNode.ProcessorNode, nonce uint64) {
	t.Helper()
	for _, n := range nodes {
		header, hash := n.GetCurrentBlockHeaderAndHash()
		require.NotNil(t, header)
		require.Equal(t, nonce, header.GetNonce())
		fd, ok := n.ForkDetector.(*rollbackForkDetector)
		require.True(t, ok)
		fd.DiscardHeader(hash)
	}
}

// TestConsensus_LateRevertedHeaderDoesNotBlockReplacement verifies that explicit
// rollback rejects the discarded hash while allowing a replacement at the same nonce.
func TestConsensus_LateRevertedHeaderDoesNotBlockReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	nodes, _, err := initTest(t, 3)
	require.NoError(t, err)
	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	currentSlot := startConsensus(t, nodes)
	for slot := uint64(1); slot <= 2; slot++ {
		currentSlot.Add(1)
		WaitForBlockSlotOrTimeOut(t, nodes, slot, blockWaitTimeoutSeconds)
	}
	revertedHeader, revertedHash := nodes[0].GetCurrentBlockHeaderAndHash()
	discardBlockForRollback(t, nodes, 2)
	require.NoError(t, integrationTest.RevertOneBlock(nodes, 2))

	// Reproduce an in-flight header callback arriving after rollback removed it.
	// It must not make sync wait for the block the entire cluster just discarded.
	for _, n := range nodes {
		require.NoError(t, n.ForkDetector.AddHeader(revertedHeader, revertedHash, process.BHReceived, nil, nil))
		require.Equal(t, uint64(1), n.ForkDetector.ProbableHighestNonce())
	}

	currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, 3, blockWaitTimeoutSeconds)
	for _, n := range nodes {
		header, hash := n.GetCurrentBlockHeaderAndHash()
		require.Equal(t, uint64(2), header.GetNonce())
		require.NotEqual(t, revertedHash, hash)
	}
}
