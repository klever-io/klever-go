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
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
)

var log = logger.GetOrCreate("integrationTest/consensus")

// simulatedSlotDuration shapes the synthetic timestamps fed to consensus via
// TimestampCalled; slots advance only when the test increments currentSlot.
const simulatedSlotDuration = 4 * time.Second

// blockWaitTimeoutSeconds bounds each wait for cluster convergence, pinned to
// the mainnet slot duration. Raising it does not deflake loaded runs — the
// dominant failure is a parked straggler no timeout rescues (measured 4/10
// failures at both 4s and 30s bounds under host load) — so contended runners
// are excluded via the -short skip instead (issue #106).
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

	// currentSlot is mutated by the test goroutine (via Add) and read by the
	// per-node UpdateSlot/Timestamp closures invoked from the chronology
	// goroutine, so it must be accessed atomically.
	var currentSlot atomic.Int64

	integrationTest.UpdateSlot(nodes, 0)
	for _, n := range nodes {
		n.SlotManager.TimeDurationField = simulatedSlotDuration
		n.SlotManager.UpdateSlotCalled = func(_ time.Time) {
			n.SlotManager.SlotIndex.Store(currentSlot.Load())
		}
		n.SlotManager.TimestampCalled = func() time.Time {
			return n.SlotManager.GenesisTimeField.Add(
				time.Duration(currentSlot.Load()) * simulatedSlotDuration,
			)
		}
		n.SlotManager.RemainingTimeCalled = func(_ time.Time, maxTime time.Duration) time.Duration {
			return maxTime // always return maxTime
		}

		require.NoError(t, n.Node.StartConsensus())
	}

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
			var state strings.Builder
			for i, n := range nodes {
				var headerSlot, headerNonce uint64
				if header, _ := n.GetCurrentBlockHeaderAndHash(); header != nil {
					headerSlot, headerNonce = header.GetSlot(), header.GetNonce()
				}
				fmt.Fprintf(&state, " node%d{done=%v headerSlot=%d headerNonce=%d slotIndex=%d}",
					i, nodesComplete[i], headerSlot, headerNonce, n.SlotManager.SlotIndex.Load())
			}
			t.Fatalf("%s: value=%d nodesComplete=%v state:%s", condition.errorMsg, condition.value, nodesComplete, state.String())
			return
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
