package preprocess

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage/txcache"
)

func NewTransactionsTest(txs *transactions) *Transactions {
	return &Transactions{txs}
}

// Transactions -
type Transactions struct {
	*transactions
}

func (txs *Transactions) GetAllTxsFromBlock(blk *block.Block) ([]*transaction.Transaction, [][]byte, error) {
	return txs.getAllTxsFromBlock(blk)
}

func (txs *Transactions) ReceivedTransaction(txHash []byte, value interface{}) {
	txs.receivedTransaction(txHash, value)
}

func (txs *Transactions) AddTxHashToRequestedList(txHash []byte) {
	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	defer txs.txsForCurrBlock.mutTxsForBlock.Unlock()

	if txs.txsForCurrBlock.txHashAndInfo == nil {
		txs.txsForCurrBlock.txHashAndInfo = make(map[string]*txInfo)
	}
	txs.txsForCurrBlock.txHashAndInfo[string(txHash)] = &txInfo{}
}

func (txs *Transactions) IsTxHashRequested(txHash []byte) bool {
	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	defer txs.txsForCurrBlock.mutTxsForBlock.Unlock()

	return txs.txsForCurrBlock.txHashAndInfo[string(txHash)].tx == nil ||
		txs.txsForCurrBlock.txHashAndInfo[string(txHash)].tx.IsInterfaceNil()
}

func (txs *Transactions) SetMissingTxs(missingTxs int) {
	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	txs.txsForCurrBlock.missingTxs = missingTxs
	txs.txsForCurrBlock.mutTxsForBlock.Unlock()
}

func (txs *Transactions) SetRcvdTxChan() {
	txs.chRcvAllTxs <- true
}

func (txs *Transactions) AddTxForCurrentBlock(
	txHash []byte,
	txHandler data.TransactionHandler,
	senderShardID uint32,
	receiverShardID uint32,
) {
	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	defer txs.txsForCurrBlock.mutTxsForBlock.Unlock()

	if txs.txsForCurrBlock.txHashAndInfo == nil {
		txs.txsForCurrBlock.txHashAndInfo = make(map[string]*txInfo)
	}

	txs.txsForCurrBlock.txHashAndInfo[string(txHash)] = &txInfo{
		tx: txHandler,
	}
}

func (txs *Transactions) GetTxInfoForCurrentBlock(txHash []byte) data.TransactionHandler {
	txs.txsForCurrBlock.mutTxsForBlock.RLock()
	defer txs.txsForCurrBlock.mutTxsForBlock.RUnlock()

	txInfo, ok := txs.txsForCurrBlock.txHashAndInfo[string(txHash)]
	if !ok {
		return nil
	}

	return txInfo.tx
}

func (txs *Transactions) PreFilterTransactionsWithPriority(
	transactions []*txcache.WrappedTransaction,
	gasBandwidth uint64,
) ([]*txcache.WrappedTransaction, []*txcache.WrappedTransaction) {
	return txs.preFilterTransactionsWithPriority(transactions, gasBandwidth)
}

func (txs *Transactions) GetTXProcessor() process.TransactionProcessor {
	return txs.txProcessor
}

func (txs *Transactions) GetEconomicsFee() process.EconomicsDataHandler {
	return txs.economicsFee
}
