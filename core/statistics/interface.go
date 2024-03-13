package statistics

import (
	"math/big"

	"github.com/klever-io/klever-go/data"
)

// TPSBenchmark is an interface used to calculate statistics for the network activity
type TPSBenchmark interface {
	Update(mb data.HeaderHandler)
	ActiveNodes() uint32
	SlotTime() uint64
	BlockNumber() uint64
	SlotNumber() uint64
	AverageBlockTxCount() *big.Int
	LastBlockTxCount() uint32
	TotalProcessedTxCount() *big.Int
	LiveTPS() float64
	PeakTPS() float64
	Statistic() ChainStatistic
	IsInterfaceNil() bool
}

// ChainStatistic is an interface used to calculate statistics for the network activity of a specific shard
type ChainStatistic interface {
	AverageTPS() *big.Int
	AverageBlockTxCount() uint32
	CurrentBlockNonce() uint64
	LiveTPS() float64
	PeakTPS() float64
	LastBlockTxCount() uint32
	TotalProcessedTxCount() *big.Int
	IsInterfaceNil() bool
}
