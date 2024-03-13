package presenter

import (
	"github.com/klever-io/klever-go/core"
)

// GetNumTxInBlock will return how many transactions are in block
func (psh *PresenterStatusHandler) GetNumTxInBlock() uint64 {
	return psh.getFromCacheAsUint64(core.MetricNumTxInBlock)
}

// GetConsensusState will return consensus state of node
func (psh *PresenterStatusHandler) GetConsensusState() string {
	return psh.getFromCacheAsString(core.MetricConsensusState)
}

// GetConsensusSlotState will return consensus slot state
func (psh *PresenterStatusHandler) GetConsensusSlotState() string {
	return psh.getFromCacheAsString(core.MetricConsensusSlotState)
}

// GetCurrentBlockHash will return current block hash
func (psh *PresenterStatusHandler) GetCurrentBlockHash() string {
	return psh.getFromCacheAsString(core.MetricCurrentBlockHash)
}

// GetEpochNumber will return current epoch
func (psh *PresenterStatusHandler) GetEpochNumber() uint64 {
	return psh.getFromCacheAsUint64(core.MetricEpochNumber)
}

// GetCurrentSlotTimestamp will return current slot timestamp
func (psh *PresenterStatusHandler) GetCurrentSlotTimestamp() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCurrentSlotTimestamp)
}

// GetBlockSize will return current block size
func (psh *PresenterStatusHandler) GetBlockSize() uint64 {
	bodyBlocksSize := psh.getFromCacheAsUint64(core.MetricBodyBlocksSize)
	headerSize := psh.getFromCacheAsUint64(core.MetricHeaderSize)

	return bodyBlocksSize + headerSize
}

// GetHighestFinalBlock will return highest nonce block notarized by metachain for current shard
func (psh *PresenterStatusHandler) GetHighestFinalBlock() uint64 {
	return psh.getFromCacheAsUint64(core.MetricHighestFinalBlock)
}
