package workItems

import (
	"encoding/hex"
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data"

	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("core/indexer/workItems")

type itemBlock struct {
	indexer       saveBlockIndexer
	marshalizer   marshal.Marshalizer
	argsSaveBlock *indexer.ArgsSaveBlockData
}

// NewItemBlock will create a new instance of ItemBlock
func NewItemBlock(
	indexer saveBlockIndexer,
	marshalizer marshal.Marshalizer,
	args *indexer.ArgsSaveBlockData,
) WorkItemHandler {
	return &itemBlock{
		indexer:       indexer,
		marshalizer:   marshalizer,
		argsSaveBlock: args,
	}
}

// Save will prepare and save a block item in elasticsearch database
func (wib *itemBlock) Save() error {
	if check.IfNil(wib.argsSaveBlock.Header) {
		log.Warn("nil header provided when trying to index block, will skip")
		return nil
	}

	log.Debug("indexer: starting indexing block",
		"hash", wib.argsSaveBlock.HeaderHash,
		"nonce", wib.argsSaveBlock.Header.GetNonce())

	if wib.argsSaveBlock.TransactionsPool == nil {
		wib.argsSaveBlock.TransactionsPool = &indexer.Pool{
			Txs:  make(map[string]data.TransactionHandler),
			Logs: make([]*data.LogData, 0),
		}
	}

	txsSizeInBytes := ComputeSizeOfTxs(wib.argsSaveBlock.TransactionsPool)
	err := wib.indexer.SaveHeader(wib.argsSaveBlock.Header, wib.argsSaveBlock.Signer, txsSizeInBytes, wib.argsSaveBlock.Validators)
	if err != nil {
		return fmt.Errorf("%w when saving header block, hash %s, nonce %d",
			err, hex.EncodeToString(wib.argsSaveBlock.HeaderHash), wib.argsSaveBlock.Header.GetNonce())
	}

	err = wib.indexer.SaveTransactions(wib.argsSaveBlock.Header, wib.argsSaveBlock.TransactionsPool)
	if err != nil {
		return fmt.Errorf("%w when saving transactions, block hash %s, nonce %d",
			err, hex.EncodeToString(wib.argsSaveBlock.HeaderHash), wib.argsSaveBlock.Header.GetNonce())
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wib *itemBlock) IsInterfaceNil() bool {
	return wib == nil
}

// ComputeSizeOfTxs will compute size of transactions in bytes
func ComputeSizeOfTxs(pool *indexer.Pool) int {
	sizeTxs := 0
	sizeTxs += computeSizeOfMap(pool.Txs)

	return sizeTxs
}

func computeSizeOfMap(mapTxs map[string]data.TransactionHandler) int {
	txsSize := 0
	for _, tx := range mapTxs {
		txsSize += tx.Size()
	}

	return txsSize
}
