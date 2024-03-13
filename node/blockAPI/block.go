package blockAPI

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

type apiBlockProcessor struct {
	hasDbLookupExtensions    bool
	store                    retriever.StorageService
	marshalizer              marshal.Marshalizer
	uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	// historyRepo              dblookupext.HistoryRepository // TODO:
	unmarshalTx func(txBytes []byte) (*api.Transaction, error)
}

// NewAPIBlockProcessor will create a new instance of meta api block processor
func NewAPIBlockProcessor(arg *APIBlockProcessorArg) *apiBlockProcessor {
	//hasDbLookupExtensions := arg.HistoryRepo.IsEnabled() // TODO:
	return &apiBlockProcessor{
		hasDbLookupExtensions:    false,
		store:                    arg.Store,
		marshalizer:              arg.Marshalizer,
		uint64ByteSliceConverter: arg.Uint64ByteSliceConverter,
		//historyRepo:              arg.HistoryRepo,
		unmarshalTx: arg.UnmarshalTx,
	}
}

// GetBlockByNonce wil return a meta APIBlock by nonce
func (mbp *apiBlockProcessor) GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error) {

	nonceToByteSlice := mbp.uint64ByteSliceConverter.ToByteSlice(nonce)
	headerHash, err := mbp.store.Get(retriever.HdrNonceHashDataUnit, nonceToByteSlice)
	if err != nil {
		return nil, err
	}

	blockBytes, err := mbp.getFromStorer(retriever.BlockUnit, headerHash)
	if err != nil {
		return nil, err
	}

	return mbp.convertBlockBytesToAPIBlock(headerHash, blockBytes, withTxs)
}

// GetBlockByHash will return a shard APIBlock by hash
func (mbp *apiBlockProcessor) GetBlockByHash(hash []byte, withTxs bool) (*api.Block, error) {
	blockBytes, err := mbp.getFromStorer(retriever.BlockUnit, hash)
	if err != nil {
		return nil, err
	}

	blockAPI, err := mbp.convertBlockBytesToAPIBlock(hash, blockBytes, withTxs)
	if err != nil {
		return nil, err
	}

	return mbp.computeStatusAndPutInBlock(blockAPI, retriever.HdrNonceHashDataUnit)
}

func (mbp *apiBlockProcessor) convertBlockBytesToAPIBlock(hash []byte, blockBytes []byte, withTxs bool) (*api.Block, error) {
	blockHeader := &block.Block{}
	err := mbp.marshalizer.Unmarshal(blockHeader, blockBytes)
	if err != nil {
		return nil, err
	}

	var txs []*api.Transaction
	if withTxs {
		txs = mbp.getTxsByMb(blockHeader)
	}

	return &api.Block{
		Block:        blockHeader,
		Hash:         hex.EncodeToString(hash),
		Transactions: txs,
		Status:       BlockStatusOnChain,
	}, nil
}
