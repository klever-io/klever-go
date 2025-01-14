package mock

import (
	"math/big"
	"sync"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// TpsBenchmarkMock will calculate statistics for the network activity
type TpsBenchmarkMock struct {
	mut                   sync.RWMutex
	activeNodes           uint32
	slotTime              uint64
	blockNumber           uint64
	slotNumber            uint64
	peakTPS               float64
	averageBlockTxCount   *big.Int
	currentBlockTxCount   uint32
	totalProcessedTxCount *big.Int
}

// ActiveNodes returns the number of active nodes
func (s *TpsBenchmarkMock) ActiveNodes() uint32 {
	return s.activeNodes
}

// SlotTime returns the slot duration in seconds
func (s *TpsBenchmarkMock) SlotTime() uint64 {
	return s.slotTime
}

// BlockNumber returns the last processed block number
func (s *TpsBenchmarkMock) BlockNumber() uint64 {
	return s.blockNumber
}

// SlotNumber returns the slot index for this benchmark object
func (s *TpsBenchmarkMock) SlotNumber() uint64 {
	return s.slotNumber
}

// AverageBlockTxCount returns an average of the tx/block
func (s *TpsBenchmarkMock) AverageBlockTxCount() *big.Int {
	return s.averageBlockTxCount
}

// CurrentBlockTxCount returns the number of transactions processed in the current block
func (s *TpsBenchmarkMock) CurrentBlockTxCount() uint32 {
	return s.currentBlockTxCount
}

// TotalProcessedTxCount returns the total number of processed transactions
func (s *TpsBenchmarkMock) TotalProcessedTxCount() *big.Int {
	return s.totalProcessedTxCount
}

// LiveTPS returns tps for the current block
func (s *TpsBenchmarkMock) LiveTPS() float64 {
	return float64(uint64(s.currentBlockTxCount) / s.slotTime)
}

// PeakTPS returns tps for the last block
func (s *TpsBenchmarkMock) PeakTPS() float64 {
	return s.peakTPS
}

// Update receives a metablock and updates all fields accordingly for each shard available in the meta block
func (s *TpsBenchmarkMock) Update(mblock data.HeaderHandler) {
	if mblock == nil || mblock.IsInterfaceNil() {
		return
	}

	mb, ok := mblock.(*block.Block)
	if !ok {
		return
	}

	s.mut.Lock()
	_ = s.updateStatistics(mb)
	s.mut.Unlock()
}

func (s *TpsBenchmarkMock) updateStatistics(b *block.Block) error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *TpsBenchmarkMock) IsInterfaceNil() bool {
	return s == nil
}
