package preprocess

import (
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type txInfo struct {
	tx data.TransactionHandler
}

type txsForBlock struct {
	missingTxs     int
	mutTxsForBlock sync.RWMutex
	txHashAndInfo  map[string]*txInfo
}

type basePreProcess struct {
	hasher          hashing.Hasher
	marshalizer     marshal.Marshalizer
	economicsFee    process.EconomicsDataHandler
	accounts        state.AccountsAdapter
	kapps           state.AccountsAdapter
	peers           state.AccountsAdapter
	pubkeyConverter core.PubkeyConverter
	forkController  core.ForkController
}

func (bpp *basePreProcess) removeTxsFromPools(
	block *block.Block,
	txPool retriever.ShardedDataCacherNotifier,
) error {
	if check.IfNil(block) {
		return process.ErrNilTxBlockHeader
	}
	if check.IfNil(txPool) {
		return process.ErrNilTransactionPool
	}

	txPool.RemoveSetOfDataFromPool(block.TxHashes, "0")

	return nil
}

func (bpp *basePreProcess) createMarshalizedData(txHashes [][]byte, forBlock *txsForBlock) ([][]byte, error) {
	mrsTxs := make([][]byte, 0, len(txHashes))
	for _, txHash := range txHashes {
		forBlock.mutTxsForBlock.RLock()
		txInfoFromMap := forBlock.txHashAndInfo[string(txHash)]
		forBlock.mutTxsForBlock.RUnlock()

		if txInfoFromMap == nil || check.IfNil(txInfoFromMap.tx) {
			log.Warn("basePreProcess.createMarshalizedData: tx not found", "hash", txHash)
			continue
		}

		txMrs, err := bpp.marshalizer.Marshal(txInfoFromMap.tx)
		if err != nil {
			return nil, process.ErrMarshalWithoutSuccess
		}
		mrsTxs = append(mrsTxs, txMrs)
	}

	log.Trace("basePreProcess.createMarshalizedData",
		"num txs", len(mrsTxs),
	)

	return mrsTxs, nil
}

func (bpp *basePreProcess) baseReceivedTransaction(
	txHash []byte,
	tx data.TransactionHandler,
	forBlock *txsForBlock,
) bool {

	forBlock.mutTxsForBlock.Lock()
	defer forBlock.mutTxsForBlock.Unlock()

	if forBlock.missingTxs > 0 {
		txInfoForHash := forBlock.txHashAndInfo[string(txHash)]
		if txInfoForHash != nil &&
			(txInfoForHash.tx == nil || txInfoForHash.tx.IsInterfaceNil()) {
			forBlock.txHashAndInfo[string(txHash)].tx = tx
			forBlock.missingTxs--
		}

		return forBlock.missingTxs == 0
	}

	return false
}

func (bpp *basePreProcess) computeExistingAndRequestMissing(
	block *block.Block,
	forBlock *txsForBlock,
	_ chan bool,
	txPool retriever.ShardedDataCacherNotifier,
	onRequestTxs func(txHashes [][]byte),
) int {
	if check.IfNil(block) {
		return 0
	}

	forBlock.mutTxsForBlock.Lock()
	defer forBlock.mutTxsForBlock.Unlock()

	txHashes := make([][]byte, 0)
	missingTxs := make([][]byte, 0)

	for i := 0; i < len(block.TxHashes); i++ {
		txHash := block.TxHashes[i]
		tx, err := process.GetTransactionHandlerFromPool(txHash, txPool)
		if err != nil {
			txHashes = append(txHashes, txHash)
			forBlock.missingTxs++
			log.Trace("missing tx",
				"block", block.GetNonce(),
				"hash", txHash,
				"error", err.Error(),
			)
			continue
		}

		forBlock.txHashAndInfo[string(txHash)] = &txInfo{tx: tx}
	}

	if len(txHashes) > 0 {
		bpp.setMissingTxs(txHashes, forBlock)
		missingTxs = append(missingTxs, txHashes...)
	}

	return bpp.requestMissingTxs(missingTxs, onRequestTxs)
}

// this method should be called only under the mutex protection: forBlock.mutTxsForBlock
func (bpp *basePreProcess) setMissingTxs(
	txHashes [][]byte,
	forBlock *txsForBlock,
) {
	for _, txHash := range txHashes {
		forBlock.txHashAndInfo[string(txHash)] = &txInfo{tx: nil}
	}
}

// this method should be called only under the mutex protection: forBlock.mutTxsForBlock
func (bpp *basePreProcess) requestMissingTxs(
	missingTxs [][]byte,
	onRequestTxs func(txHashes [][]byte),
) int {
	requestedTxs := len(missingTxs)
	go onRequestTxs(missingTxs)

	return requestedTxs
}
