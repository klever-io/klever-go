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
	CurrentBlockTxCount() uint32
	TotalProcessedTxCount() *big.Int
	LiveTPS() float64
	PeakTPS() float64
	IsInterfaceNil() bool
}
