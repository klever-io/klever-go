package consensus

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/stretchr/testify/require"
)

// rollbackForkDetector makes a forced cluster-wide rollback final for this
// harness. An already dispatched header callback can otherwise reinsert the
// discarded block after RemoveHeader, making sync wait for a block that no peer
// has anymore. Serialize removal with delivery and ignore that exact hash;
// replacement blocks at the same nonce still reach the real fork detector.
type rollbackForkDetector struct {
	process.ForkDetector
	mu        sync.Mutex
	discarded map[string]struct{}
}

func (fd *rollbackForkDetector) AddHeader(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, notarized []data.HeaderHandler, notarizedHashes [][]byte) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if _, discarded := fd.discarded[string(hash)]; discarded {
		return nil
	}
	return fd.ForkDetector.AddHeader(header, hash, state, notarized, notarizedHashes)
}

func (fd *rollbackForkDetector) RemoveHeader(nonce uint64, hash []byte) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.discarded[string(hash)] = struct{}{}
	fd.ForkDetector.RemoveHeader(nonce, hash)
}

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
