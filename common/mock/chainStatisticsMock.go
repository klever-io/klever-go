package mock

import "math/big"

// ChainStatisticsMock will hold the tps statistics for each shard
type ChainStatisticsMock struct {
	slotTime              uint64
	averageTPS            *big.Int
	peakTPS               float64
	lastBlockTxCount      uint32
	averageBlockTxCount   uint32
	currentBlockNonce     uint64
	totalProcessedTxCount *big.Int
}

// AverageTPS returns an average tps for all processed blocks in a shard
func (ss *ChainStatisticsMock) AverageTPS() *big.Int {
	return ss.averageTPS
}

// AverageBlockTxCount returns an average transaction count for
func (ss *ChainStatisticsMock) AverageBlockTxCount() uint32 {
	return ss.averageBlockTxCount
}

// CurrentBlockNonce returns the block nounce of the last processed block in a shard
func (ss *ChainStatisticsMock) CurrentBlockNonce() uint64 {
	return ss.currentBlockNonce
}

// LiveTPS returns tps for the last block
func (ss *ChainStatisticsMock) LiveTPS() float64 {
	return float64(uint64(ss.lastBlockTxCount) / ss.slotTime)
}

// PeakTPS returns peak tps for for all the blocks of the current shard
func (ss *ChainStatisticsMock) PeakTPS() float64 {
	return ss.peakTPS
}

// LastBlockTxCount returns the number of transactions included in the last block
func (ss *ChainStatisticsMock) LastBlockTxCount() uint32 {
	return ss.lastBlockTxCount
}

// TotalProcessedTxCount returns the total number of processed transactions for this shard
func (ss *ChainStatisticsMock) TotalProcessedTxCount() *big.Int {
	return ss.totalProcessedTxCount
}

// IsInterfaceNil returns true if there is no value under the interface
func (ss *ChainStatisticsMock) IsInterfaceNil() bool {
	return ss == nil
}
