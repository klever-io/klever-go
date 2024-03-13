package blockAPI

import (
	"encoding/hex"
	"fmt"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
)

// BlockStatus is the status of a block
type BlockStatus string

const (
	// BlockStatusPending represents the identifier for an pending block
	BlockStatusPending = "pending"
	// BlockStatusOnChain represents the identifier for an on-chain block
	BlockStatusOnChain = "on-chain"
	// BlockStatusReverted represent the identifier for a reverted block
	BlockStatusReverted = "reverted"
)

var log = logger.GetOrCreate("node/blockAPI")

func (mbp *apiBlockProcessor) getTxsByMb(blck *block.Block) []*api.Transaction {

	storer := mbp.store.GetStorer(retriever.TransactionUnit)
	start := time.Now()
	marshalizedTxs, err := storer.GetBulkFromEpoch(blck.TxHashes, blck.GetEpoch())
	if err != nil {
		log.Warn("cannot get from storage transactions",
			"error", err.Error())
		return []*api.Transaction{}
	}
	log.Debug(fmt.Sprintf("GetBulkFromEpoch took %s", time.Since(start)))

	start = time.Now()
	txs := make([]*api.Transaction, 0)
	for txHash, txBytes := range marshalizedTxs {
		tx, errUnmarshalTx := mbp.unmarshalTx(txBytes)
		if errUnmarshalTx != nil {
			log.Warn("cannot unmarshal transaction",
				"hash", hex.EncodeToString([]byte(txHash)),
				"error", errUnmarshalTx.Error())
			continue
		}
		// TODO: add Status

		txs = append(txs, tx)
	}
	log.Debug(fmt.Sprintf("UnmarshalTransactions took %s", time.Since(start)))

	return txs
}

func (mbp *apiBlockProcessor) getFromStorer(unit retriever.UnitType, key []byte) ([]byte, error) {
	return mbp.store.Get(unit, key)
}

func (mbp *apiBlockProcessor) computeBlockStatus(storerUnit retriever.UnitType, blockAPI *api.Block) (string, error) {
	nonceToByteSlice := mbp.uint64ByteSliceConverter.ToByteSlice(blockAPI.GetNonce())
	headerHash, err := mbp.store.Get(storerUnit, nonceToByteSlice)
	if err != nil {
		return "", err
	}

	if hex.EncodeToString(headerHash) != blockAPI.Hash {
		return BlockStatusReverted, err
	}

	return BlockStatusOnChain, nil
}

func (mbp *apiBlockProcessor) computeStatusAndPutInBlock(blockAPI *api.Block, storerUnit retriever.UnitType) (*api.Block, error) {
	blockStatus, err := mbp.computeBlockStatus(storerUnit, blockAPI)
	if err != nil {
		return nil, err
	}

	if blockStatus == BlockStatusOnChain &&
		(blockAPI.ProducerSignature == nil ||
			blockAPI.Signature == nil ||
			blockAPI.PubKeysBitmap == nil) {
		blockAPI.Status = BlockStatusPending
	} else {
		blockAPI.Status = blockStatus
	}

	return blockAPI, nil
}
