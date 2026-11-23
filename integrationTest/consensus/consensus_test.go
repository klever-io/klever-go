package consensus

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/node"
)

var log = logger.GetOrCreate("integrationTest/consensus")

// simulatedSlotDuration shapes the synthetic timestamps fed to consensus via
// TimestampCalled; slots advance only when the test increments currentSlot.
const simulatedSlotDuration = 4 * time.Second

// blockWaitTimeoutSeconds bounds convergence independently of the synthetic clock.
// Slot startup waits for every node's sync state to be calculated before allowing
// a proposal, so the worker cannot drop it in the NsNotCalculated window.
const blockWaitTimeoutSeconds = int(simulatedSlotDuration / time.Second)

func initTest(t *testing.T, numOfNodes int) ([]*processorNode.ProcessorNode, []*processorNode.NodeAccount, error) {
	initialBalance := int64(1_000_000_000_000)
	numWallets := 2
	mainConfig := commonTxTest.LoadDefaultConfigs(t, "../../integrationTest/config/config.yaml")
	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numOfNodes, numOfNodes, numWallets, mainConfig)
	require.Nil(t, err)
	// wait initializations
	time.Sleep(1000 * time.Millisecond)

	return nodes, wallets, err
}

// startConsensus keeps the synthetic slot transition ahead of proposal delivery.
// Updating SlotIndex invalidates the bootstrapper's cached sync state. Until its
// next tick, Worker.ProcessReceivedMessage drops consensus messages, including a
// fast leader's proposal. With the clock frozen that lost proposal leaves a node
// parked forever after its BLOCK timer expires. Wait for all nodes to observe the
// slot and recalculate their sync state before any chronology starts the slot.
func startConsensus(t *testing.T, nodes []*processorNode.ProcessorNode) *atomic.Int64 {
	t.Helper()
	currentSlot := new(atomic.Int64)
	integrationTest.UpdateSlot(nodes, 0)
	for _, n := range nodes {
		n.ForkDetector = &rollbackForkDetector{
			ForkDetector: n.ForkDetector,
			discarded:    make(map[string]struct{}),
		}
		require.NoError(t, n.Node.ApplyOptions(node.WithForkDetector(n.ForkDetector)))
		n.SlotManager.TimeDurationCalled = func() time.Duration {
			return simulatedSlotDuration
		}
		n.SlotManager.UpdateSlotCalled = func(_ time.Time) {
			slot := currentSlot.Load()
			if n.SlotManager.SlotIndex.Swap(slot) == slot {
				return
			}

			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				ready := true
				for _, peer := range nodes {
					if peer.SlotManager.Index() != slot || peer.Node.GetNodeState() != core.NsSynchronized {
						ready = false
						break
					}
				}
				if ready {
					return
				}
				select {
				case <-t.Context().Done():
					return
				case <-ticker.C:
				}
			}
		}
		n.SlotManager.TimestampCalled = func() time.Time {
			return n.SlotManager.GenesisTimeField.Add(
				time.Duration(n.SlotManager.Index()) * simulatedSlotDuration,
			)
		}
		n.SlotManager.RemainingTimeCalled = func(_ time.Time, maxTime time.Duration) time.Duration {
			return maxTime
		}
		require.NoError(t, n.Node.StartConsensus())
	}
	return currentSlot
}

// TestConsensus_RevertBlockAndTransactions verifies that a reverted transaction
// returns to the pool, is included again, and remains committed across epochs.
func TestConsensus_RevertBlockAndTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	// change log level to debug "process/block"
	// logger.SetLogLevel("*:INFO,process/block:DEBUG,consensus/chronology:DEBUG,consensus/spos/bls:DEBUG,process/sync:DEBUG")

	numOfNodes := 7

	nodes, wallets, err := initTest(t, numOfNodes)
	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()
	require.Nil(t, err)
	assert.Len(t, wallets, 2)

	currentSlot := startConsensus(t, nodes)

	// inc slot and wait for block proposal
	slot := currentSlot.Add(1)
	// wait for block proposal
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)

	// create another block
	slot = currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)

	log.Info("*********************** SEND TX ***********************")
	// send transaction
	tx, txHash := commonTxTest.SendTx(t, nodes, wallets, 1)
	log.Info("TX Sent", "hash", txHash, "sender", tx.GetSender(), "nonce", tx.GetNonce())

	// create another block
	slot = currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, txHash, uint64(slot))
	require.Nil(t, err)

	// revert last block
	discardBlockForRollback(t, nodes, uint64(slot))
	err = integrationTest.RevertOneBlock(nodes, uint64(slot))
	require.Nil(t, err)

	// check tx in block
	err = commonTxTest.CheckTXIsPending(nodes, txHash)
	require.Nil(t, err)

	// create another block
	slot = currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)

	// TX should be processed on next blok after revert
	txBlockNonce := uint64(slot) - 1
	txBlockSlot := uint64(slot)

	// check TX in block again
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txBlockNonce)
	assert.Nil(t, err)

	// produce 12 should create 2 epochs (6 slots each)
	for i := 0; i < 12; i++ {
		slot = currentSlot.Add(1)
		WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)
	}

	// wait for block sync: poll instead of a fixed sleep so slower machines
	// are not misreported as sync failures
	waitForBlockConditionOrTimeOut(t, nodes, blockWaitCondition{
		check: func(n *processorNode.ProcessorNode) bool {
			return commonTxTest.CheckTXInBlock([]*processorNode.ProcessorNode{n}, txHash, txBlockNonce) == nil
		},
		errorMsg: "timeout waiting for tx block sync",
		value:    txBlockNonce,
	}, blockWaitTimeoutSeconds)

	// Check TX block
	b, err := nodes[0].GetBlockByNonce(txBlockNonce)
	assert.Nil(t, err)
	assert.Equal(t, txBlockNonce, b.GetNonce())
	assert.Equal(t, txBlockSlot, b.GetSlot())
	require.Len(t, b.TxHashes, 1)
	assert.Equal(t, txHash, b.TxHashes[0])

	// get current block header and check slot/nonce/epoch
	finalSlot := currentSlot.Load()
	header, _ := nodes[0].GetCurrentBlockHeaderAndHash()
	assert.Equal(t, uint64(finalSlot), header.GetSlot())
	assert.Equal(t, uint64(finalSlot-1), header.GetNonce())
	// Since slots start at 1, we subtract 1 to align with zero-based indexing.
	// in this tests we have 6 slots per epoch
	assert.Equal(t, uint32(finalSlot-1)/uint32(6), header.GetEpoch())
}

func WaitForBlockSlotOrTimeOut(t *testing.T, nodes []*processorNode.ProcessorNode, slot uint64, timeout int) {
	t.Helper()
	waitForBlockConditionOrTimeOut(t, nodes, blockWaitCondition{
		check: func(n *processorNode.ProcessorNode) bool {
			header, _ := n.GetCurrentBlockHeaderAndHash()
			return header != nil && header.GetSlot() == slot
		},
		errorMsg: "timeout waiting for slot",
		value:    slot,
	}, timeout)
}

type blockWaitCondition struct {
	check    func(*processorNode.ProcessorNode) bool
	errorMsg string
	value    uint64
}

// clusterStateString renders each node's chain head and slot index so a
// timeout log identifies the failure mode without reproduction (issue #106).
func clusterStateString(nodes []*processorNode.ProcessorNode, nodesComplete []bool) string {
	var state strings.Builder
	for i, n := range nodes {
		var headerSlot, headerNonce uint64
		if header, _ := n.GetCurrentBlockHeaderAndHash(); header != nil {
			headerSlot, headerNonce = header.GetSlot(), header.GetNonce()
		}
		fmt.Fprintf(&state, " node%d{done=%v headerSlot=%d headerNonce=%d slotIndex=%d syncState=%d}",
			i, nodesComplete[i], headerSlot, headerNonce, n.SlotManager.SlotIndex.Load(), n.Node.GetNodeState())
	}
	return state.String()
}

func waitForBlockConditionOrTimeOut(t *testing.T, nodes []*processorNode.ProcessorNode, condition blockWaitCondition, timeout int) {
	t.Helper()
	nodesComplete := make([]bool, len(nodes))
	maxTime := time.After(time.Duration(timeout) * time.Second)
	backoff := 100 * time.Millisecond
	maxBackoff := time.Second

	check := func() bool {
		for i, n := range nodes {
			if nodesComplete[i] {
				continue
			}
			if condition.check(n) {
				nodesComplete[i] = true
			}
		}
		for _, c := range nodesComplete {
			if !c {
				return false
			}
		}
		return true
	}

	for {
		if check() {
			return
		}
		select {
		case <-maxTime:
			// One final check right at the deadline: the previous sleep could
			// have run up to maxBackoff (1s) and a node that finished between
			// the prior poll and now would otherwise be misreported as a
			// timeout. Timeouts used to be silently logged, which made later
			// assertions fire against an out-of-sync cluster — fail at the
			// source instead.
			if check() {
				return
			}
			t.Fatalf("%s: value=%d nodesComplete=%v state:%s",
				condition.errorMsg, condition.value, nodesComplete, clusterStateString(nodes, nodesComplete))
			return
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
