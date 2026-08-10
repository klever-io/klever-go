package consensus

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/integrationTest"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
)

func TestConsensus_InsertDupTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	numOfNodes := 3

	nodes, wallets, err := initTest(t, numOfNodes)
	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()
	require.Nil(t, err)
	assert.Len(t, wallets, 2)

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

	log.Info("*********************** SEND TX ***********************")
	// send transaction
	tx, txHash := commonTxTest.SendTx(t, nodes, wallets, 1)
	log.Info("TX Sent", "hash", txHash, "sender", tx.GetSender(), "nonce", tx.GetNonce())

	// create with transaction
	slot = currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)
	txInBlock := uint64(slot)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txInBlock)
	require.Nil(t, err)

	// force push TX to pool
	for _, n := range nodes {
		n.DataPool.Transactions().AddData([]byte(txHash), tx, 100, "0")
	}

	// create another block
	slot = currentSlot.Add(1)
	WaitForBlockSlotOrTimeOut(t, nodes, uint64(slot), blockWaitTimeoutSeconds)

	// get block
	header, _ := nodes[0].GetCurrentBlockHeaderAndHash()
	// check if TX is in block
	assert.Equal(t, uint32(0), header.GetTxCount())
}
