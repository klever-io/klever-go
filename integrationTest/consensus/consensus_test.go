package consensus

import (
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

	currentSlot := int64(0)

	integrationTest.UpdateSlot(nodes, 0)
	for _, n := range nodes {
		n.SlotManager.TimeDurationField = 4 // 4 seconds simulation
		n.SlotManager.UpdateSlotCalled = func(t time.Time) {
			n.SlotManager.SlotIndex = currentSlot
		}
		n.SlotManager.TimestampCalled = func() time.Time {
			// compute Timestamp based on slot
			return n.SlotManager.GenesisTimeField.Add(time.Duration(currentSlot*int64(n.SlotManager.TimeDurationField)) * time.Second) // 4 seconds simulation
		}
		n.SlotManager.RemainingTimeCalled = func(startTime time.Time, maxTime time.Duration) time.Duration {
			return maxTime // always return maxTime
		}
		n.Node.StartConsensus()
	}

	// inc slot and wait for block proposal
	currentSlot++
	// wait for block proposal
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	// create another block
	currentSlot++
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	log.Info("*********************** SEND TX ***********************")
	// send transaction
	tx, txHash := commonTxTest.SendTx(t, nodes, wallets, 1)
	log.Info("TX Sent", "hash", txHash, "sender", tx.GetSender(), "nonce", tx.GetNonce())

	// create another block
	currentSlot++
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, txHash, uint64(currentSlot))
	require.Nil(t, err)

	// revert last block
	err = integrationTest.RevertOneBlock(nodes, uint64(currentSlot))
	require.Nil(t, err)

	// check tx in block
	err = commonTxTest.CheckTXIsPending(nodes, txHash)
	require.Nil(t, err)

	// create another block
	currentSlot++
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	// TX should be processed on next blok after revert
	txBlockNonce := uint64(currentSlot) - 1
	txBlockSlot := uint64(currentSlot)

	// check TX in block again
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txBlockNonce)
	assert.Nil(t, err)

	// produce 12 should create 2 epochs (6 slots each)
	for i := 0; i < 12; i++ {
		currentSlot++
		WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)
	}

	// wait for block sync
	time.Sleep(time.Millisecond * 500)
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txBlockNonce)
	assert.Nil(t, err)

	// Check TX block
	b, err := nodes[0].GetBlockByNonce(txBlockNonce)
	assert.Nil(t, err)
	assert.Equal(t, txBlockNonce, b.GetNonce())
	assert.Equal(t, txBlockSlot, b.GetSlot())
	require.Len(t, b.TxHashes, 1)
	assert.Equal(t, txHash, b.TxHashes[0])

	// get current block header and check slot/nonce/epoch
	header, _ := nodes[0].GetCurrentBlockHeaderAndHash()
	assert.Equal(t, uint64(currentSlot), header.GetSlot())
	assert.Equal(t, uint64(currentSlot-1), header.GetNonce())
	// Since slots start at 1, we subtract 1 to align with zero-based indexing.
	// in this tests we have 6 slots per epoch
	assert.Equal(t, uint32(currentSlot-1)/uint32(6), header.GetEpoch())
}

func WaitForBlockNonceOrTimeOut(nodes []*processorNode.ProcessorNode, nonce uint64, timeout int) {
	waitForBlockConditionOrTimeOut(nodes, blockWaitCondition{
		check: func(n *processorNode.ProcessorNode) bool {
			// get current block header, check if slot is in
			header, _ := n.GetCurrentBlockHeaderAndHash()
			return header != nil && header.GetNonce() == nonce
		},
		errorMsg: "timeout waiting for nonce",
		value:    nonce,
	}, timeout)
}

func WaitForBlockSlotOrTimeOut(nodes []*processorNode.ProcessorNode, slot uint64, timeout int) {
	waitForBlockConditionOrTimeOut(nodes, blockWaitCondition{
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

func waitForBlockConditionOrTimeOut(nodes []*processorNode.ProcessorNode, condition blockWaitCondition, timeout int) {
	nodesComplete := make([]bool, len(nodes))
	maxTime := time.After(time.Duration(timeout) * time.Second)
	backoff := 100 * time.Millisecond
	maxBackoff := time.Second
	for {
		select {
		case <-maxTime:
			log.Error(condition.errorMsg, condition.value, "recMap", nodesComplete)
			return
		default:
			for i, n := range nodes {
				if nodesComplete[i] {
					continue
				}
				if condition.check(n) {
					nodesComplete[i] = true
				}
			}
			allComplete := true
			for _, c := range nodesComplete {
				if !c {
					allComplete = false
					break
				}
			}
			if allComplete {
				return
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
