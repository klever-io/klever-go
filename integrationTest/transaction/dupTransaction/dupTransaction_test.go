package transfer

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var log = logger.GetOrCreate("integrationTest/transaction/checkDup")

func initTest(t *testing.T, numOfNodes int) ([]*processorNode.ProcessorNode, []*processorNode.NodeAccount, error) {
	initialBalance := int64(1_000_000_000_000)
	numWallets := 2
	mainConfig := commonTxTest.LoadDefaultConfigs(t, "../../../integrationTest/config/config.yaml")
	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numOfNodes, numOfNodes, numWallets, mainConfig)
	require.Nil(t, err)
	// wait initializations
	time.Sleep(1000 * time.Millisecond)

	return nodes, wallets, err
}

func TestCheckDupTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	require.NoError(t, logger.SetLogLevel("*:INFO,process/block:DEBUG,process/sync:DEBUG"))
	xidProposerBlock := 0

	numOfNodes := 4
	nodes, wallets, err := initTest(t, numOfNodes)
	require.Nil(t, err)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	var txMap = make(map[string]*transaction.Transaction)

	// update processor request
	for _, n := range nodes {
		n.OnRequestTransactionsHandler = func(hashes [][]byte) {
			for _, hash := range hashes {
				fmt.Println("Requesting transaction", hex.EncodeToString(hash))
				tx, ok := txMap[string(hash)]
				if ok {
					fmt.Println("Transaction found in map", hex.EncodeToString(hash))
					// send TX to node pool (transcation send may fail if duplicated)
					_, _ = n.SendTransaction(tx)
				}
			}
		}
	}

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	// send transaction
	tx, txHash := commonTxTest.SendTx(t, nodes, wallets, 1)
	log.Info("TX Sent", "hash", txHash, "sender", tx.GetSender(), "nonce", tx.GetNonce())
	txMap[string(txHash)] = tx

	txInBlock := nonce
	slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, txHash, txInBlock)
	require.Nil(t, err)

	// create and push transaction to producer node only
	tx2, tx2Hash, err := commonTxTest.CreateTransactionOnly(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		struct {
			Receiver string
			Amount   int64
			Asset    string
		}{
			Receiver: processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
			Amount:   10,
			Asset:    "KLV",
		},
	)
	require.Nil(t, err)
	txMap[string(tx2Hash)] = tx2

	// push to producer node pool
	integrationTest.PushToProposerPool[string(tx2Hash)] = tx2
	tx2InBlock := nonce
	slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, tx2Hash, tx2InBlock)
	require.Nil(t, err)

	// push duplicate transaction to producer pool
	// transaction should be removed and not processed by leader
	integrationTest.PushToProposerPool[string(tx2Hash)] = tx2
	slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// check tx in block
	err = commonTxTest.CheckTXInBlock(nodes, tx2Hash, tx2InBlock)
	require.Nil(t, err)

	// create a new transaction
	tx3, tx3Hash, err := commonTxTest.CreateTransactionOnly(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		struct {
			Receiver string
			Amount   int64
			Asset    string
		}{
			Receiver: processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
			Amount:   10,
			Asset:    "KLV",
		},
	)
	require.Nil(t, err)
	txMap[string(tx3Hash)] = tx3

	// push to producer node pool but with wrong hash
	// so we force TX to be added into the block
	integrationTest.PushToProposerPool[string(tx2Hash)] = tx3 // tx2Hash is been used here, tx3 is valid and might be added to the block
	slot, nonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	finalNonce := nonce - 1 // Final block nonce produced
	// block should be reverted as TX is duplicated
	// sync will timeout blocked by checkDup
	// and block will be reverted
	// check all nodes block header,
	for idx, n := range nodes {
		header, _ := n.GetCurrentBlockHeaderAndHash()
		if idx == xidProposerBlock {
			// last block should be saved
			assert.Equal(t, finalNonce, header.GetNonce())
			continue
		}
		assert.Equal(t, int64(slot), n.SlotManager.SlotIndex.Load()+1, "Node should be at previous slot")
		// other nodes then proposed should be reverted
		// current block nonce should be equal to the last block nonce -1
		assert.Equal(t, finalNonce-1, header.GetNonce())
	}
}
