package indexer

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/klever-io/klever-go/crypto/hashing"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/tools/marshal"
)

type dataParser struct {
	hasher      hashing.Hasher
	marshalizer marshal.Marshalizer
}

func (dp *dataParser) getSerializedElasticBlockAndHeaderHash(
	block nodeData.HeaderHandler,
	signer []byte,
	sizeTxs int,
	validators []string,
) ([]byte, []byte, error) {
	headerBytes, err := dp.marshalizer.Marshal(block.GetBlockHeader())
	if err != nil {
		return nil, nil, err
	}
	bodyBytes, err := dp.marshalizer.Marshal(block)
	if err != nil {
		return nil, nil, err
	}

	blockSizeInBytes := len(bodyBytes)

	txHashes := make([]string, 0)
	for _, txHash := range block.GetTxHashes() {
		txHashString := hex.EncodeToString(txHash)
		txHashes = append(txHashes, txHashString)
	}

	headerHash := dp.hasher.Compute(string(headerBytes))

	producerBls := ""
	if len(validators) > 0 {
		producerBls = validators[0]
	}

	elasticBlock := data.Block{
		Hash:               hex.EncodeToString(headerHash),
		Nonce:              block.GetNonce(),
		ParentHash:         hex.EncodeToString(block.GetParentHash()),
		Timestamp:          time.Duration(block.GetTimestamp() * 1000),
		Slot:               block.GetSlot(),
		Epoch:              block.GetEpoch(),
		IsEpochStart:       block.GetIsEpochStart(),
		Size:               int64(blockSizeInBytes),
		SizeTxs:            int64(sizeTxs),
		TxRootHash:         hex.EncodeToString(block.GetTxRootHash()),
		TrieRoot:           hex.EncodeToString(block.GetTrieRoot()),
		ValidatorsTrieRoot: hex.EncodeToString(block.GetValidatorsTrieRoot()),
		KAppsTrieRoot:      hex.EncodeToString(block.GetKAppsTrieRoot()),
		ProducerSignature:  hex.EncodeToString(signer),
		Signature:          hex.EncodeToString(block.GetSignature()),
		PrevRandSeed:       hex.EncodeToString(block.GetPrevRandSeed()),
		RandSeed:           hex.EncodeToString(block.GetRandSeed()),
		TxCount:            block.GetTxCount(),
		TxFees:             block.GetTxFees(),
		TxBurnedFees:       block.GetTxBurnedFees(),
		KAppFees:           block.GetKAppFees(),
		BlockRewards:       block.GetBlockRewards(),
		BurnedUnclaimed:    block.GetBurnedUnclaimed(),
		StakingRewards:     block.GetStakingRewards(),
		TxHashes:           txHashes,
		Validators:         validators,
		SoftwareVersion:    string(block.GetSoftwareVersion()),
		ChainID:            string(block.GetChainID()),
		Reserved:           hex.EncodeToString(block.GetReserved()),
		ProducerBLS:        producerBls,
		PrevEpochStartSlot: block.GetPrevEpochStartSlot(),
	}

	serializedBlock, err := json.Marshal(elasticBlock)
	if err != nil {
		return nil, nil, err
	}

	return serializedBlock, headerHash, nil
}
