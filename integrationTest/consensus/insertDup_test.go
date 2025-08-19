package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/integrationTest"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
)

func TestConsensus_InsertDupTransaction(t *testing.T) {
	numOfNodes := 3

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
		require.NoError(t, n.Node.StartConsensus())
	}

	// inc slot and wait for block proposal
	currentSlot++
	// wait for block proposal
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	log.Info("*********************** SEND TX ***********************")
	// send transaction
	tx, txHash := commonTxTest.SendTx(t, nodes, wallets, 1)
	log.Info("TX Sent", "hash", txHash, "sender", tx.GetSender(), "nonce", tx.GetNonce())

	// create with transaction
	currentSlot++
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)
	txInBlock := uint64(currentSlot)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txInBlock)
	require.Nil(t, err)

	// force push TX to pool
	for _, n := range nodes {
		n.DataPool.Transactions().AddData([]byte(txHash), tx, 100, "0")
	}

	// create another block
	currentSlot++
	WaitForBlockSlotOrTimeOut(nodes, uint64(currentSlot), 4)

	// get block
	header, _ := nodes[0].GetCurrentBlockHeaderAndHash()
	// check if TX is in block
	assert.Equal(t, uint32(0), header.GetTxCount())
}
