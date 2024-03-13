package statistics

import (
	"math/big"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("statistics")

// defaultBlockNumber is used to identify the default value of the value representing the block number fetched from storage.
// it is used to signal that no value was read from storage and the check for not updating total number of processed
// transactions should be skipped.
const defaultBlockNumber = -1

// TpsPersistentData holds the tps benchmark data which is stored between node restarts
type TpsPersistentData struct {
	BlockNumber           uint64
	SlotNumber            uint64
	PeakTPS               float64
	AverageBlockTxCount   *big.Int
	TotalProcessedTxCount *big.Int
	LastBlockTxCount      uint32
}

// TpsBenchmark will calculate statistics for the network activity
type TpsBenchmark struct {
	mut                   sync.RWMutex
	activeNodes           uint32
	slotTime              uint64
	blockNumber           uint64
	slotNumber            uint64
	peakTPS               float64
	averageBlockTxCount   *big.Int
	lastBlockTxCount      uint32
	totalProcessedTxCount *big.Int
	statistics            ChainStatistic
	statusHandler         core.AppStatusHandler
	initialBlockNumber    int64
}

// ChainStatistics will hold the tps statistics
type ChainStatistics struct {
	slotTime              uint64
	averageTPS            *big.Int
	peakTPS               float64
	lastBlockTxCount      uint32
	averageBlockTxCount   uint32
	currentBlockNonce     uint64
	totalProcessedTxCount *big.Int
}

// NewTPSBenchmarkWithInitialData instantiates a new object responsible with calculating statistics for each shard tps
// starting with initial data
func NewTPSBenchmarkWithInitialData(
	appStatusHandler core.AppStatusHandler,
	initialTpsBenchmark *TpsPersistentData,
	slotInterval uint64,
) (*TpsBenchmark, error) {
	if slotInterval == 0 {
		return nil, ErrInvalidSlotInterval
	}
	if initialTpsBenchmark == nil {
		return nil, ErrNilInitialTPSBenchmarks
	}
	if check.IfNil(appStatusHandler) {
		return nil, ErrNilStatusHandler
	}

	stats := &ChainStatistics{
		slotTime:              slotInterval,
		totalProcessedTxCount: big.NewInt(0),
	}
	return &TpsBenchmark{
		slotTime:              slotInterval,
		statistics:            stats,
		peakTPS:               initialTpsBenchmark.PeakTPS,
		lastBlockTxCount:      initialTpsBenchmark.LastBlockTxCount,
		blockNumber:           initialTpsBenchmark.BlockNumber,
		slotNumber:            initialTpsBenchmark.SlotNumber,
		totalProcessedTxCount: initialTpsBenchmark.TotalProcessedTxCount,
		averageBlockTxCount:   initialTpsBenchmark.AverageBlockTxCount,
		statusHandler:         appStatusHandler,
		initialBlockNumber:    int64(initialTpsBenchmark.BlockNumber),
	}, nil
}

// NewTPSBenchmark instantiates a new object responsible with calculating statistics for each shard tps.
// represents the total number of shards, slotInterval is the duration for a slot in seconds
func NewTPSBenchmark(
	slotInterval uint64,
) (*TpsBenchmark, error) {
	if slotInterval == 0 {
		return nil, ErrInvalidSlotInterval
	}

	stats := &ChainStatistics{
		slotTime:              slotInterval,
		totalProcessedTxCount: big.NewInt(0),
	}

	return &TpsBenchmark{
		slotTime:              slotInterval,
		statistics:            stats,
		statusHandler:         statusHandler.NewNilStatusHandler(),
		totalProcessedTxCount: big.NewInt(0),
		averageBlockTxCount:   big.NewInt(0),
		initialBlockNumber:    defaultBlockNumber,
	}, nil
}

// ActiveNodes returns the number of active nodes
func (s *TpsBenchmark) ActiveNodes() uint32 {
	return s.activeNodes
}

// SlotTime returns the slot duration in seconds
func (s *TpsBenchmark) SlotTime() uint64 {
	return s.slotTime
}

// BlockNumber returns the last processed block number
func (s *TpsBenchmark) BlockNumber() uint64 {
	return s.blockNumber
}

// SlotNumber returns the slot index for this benchmark object
func (s *TpsBenchmark) SlotNumber() uint64 {
	return s.slotNumber
}

// AverageBlockTxCount returns an average of the tx/block
func (s *TpsBenchmark) AverageBlockTxCount() *big.Int {
	return s.averageBlockTxCount
}

// LastBlockTxCount returns the number of transactions processed in the last block
func (s *TpsBenchmark) LastBlockTxCount() uint32 {
	return s.lastBlockTxCount
}

// TotalProcessedTxCount returns the total number of processed transactions
func (s *TpsBenchmark) TotalProcessedTxCount() *big.Int {
	return s.totalProcessedTxCount
}

// LiveTPS returns tps for the last block
func (s *TpsBenchmark) LiveTPS() float64 {
	return float64(uint64(s.lastBlockTxCount) / s.slotTime)
}

// PeakTPS returns tps for the last block
func (s *TpsBenchmark) PeakTPS() float64 {
	return s.peakTPS
}

// Statistic returns the current statistical state
func (s *TpsBenchmark) Statistic() ChainStatistic {
	s.mut.RLock()
	defer s.mut.RUnlock()

	return s.statistics
}

// Update receives a metablock and updates all fields accordingly for each shard available in the meta block
func (s *TpsBenchmark) Update(mblock data.HeaderHandler) {
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

func (s *TpsBenchmark) updateStatistics(b *block.Block) error {
	if s.blockNumber == b.Header.Nonce {
		return nil
	}

	s.blockNumber = b.Header.Nonce
	s.slotNumber = b.Header.Slot

	// same value (one shard)
	totalTxsForTPS := uint64(b.Header.TxCount)
	totalTxsForCount := uint64(b.Header.TxCount)

	s.lastBlockTxCount = uint32(totalTxsForTPS)

	s.totalProcessedTxCount.Add(s.totalProcessedTxCount, big.NewInt(0).SetUint64(totalTxsForCount))
	s.statusHandler.AddUint64(core.MetricNumProcessedTxsTPSBenchmark, totalTxsForCount)

	s.averageBlockTxCount.Quo(s.totalProcessedTxCount, big.NewInt(int64(b.Header.Nonce)))

	currentTPS := float64(totalTxsForTPS / s.slotTime)
	if currentTPS > s.peakTPS {
		s.peakTPS = currentTPS
	}

	s.statusHandler.SetUInt64Value(core.MetricNonceForTPS, b.Header.Nonce)
	s.statusHandler.SetUInt64Value(core.MetricLastBlockTxCount, totalTxsForTPS)
	s.statusHandler.SetUInt64Value(core.MetricPeakTPS, uint64(s.peakTPS))
	s.statusHandler.SetStringValue(core.MetricAverageBlockTxCount, s.averageBlockTxCount.String())

	// one shard only...
	shardTotalTxsForTPS := uint64(b.Header.TxCount)
	shardTotalTxsForCount := uint64(b.Header.TxCount)

	shardPeakTPS := s.statistics.PeakTPS()
	currentShardTPS := float64(shardTotalTxsForTPS / s.slotTime)
	if currentShardTPS > s.statistics.PeakTPS() {
		shardPeakTPS = currentShardTPS
	}

	bigTxCount := big.NewInt(0).SetUint64(shardTotalTxsForCount)
	newTotalProcessedTxCount := big.NewInt(0).Add(s.statistics.TotalProcessedTxCount(), bigTxCount)
	slotsPassed := big.NewInt(int64(b.Header.Slot))
	newAverageTPS := big.NewInt(0).Quo(newTotalProcessedTxCount, slotsPassed)

	updatedChainStats := &ChainStatistics{
		slotTime:              s.slotTime,
		currentBlockNonce:     b.Header.Nonce,
		totalProcessedTxCount: newTotalProcessedTxCount,

		averageTPS:       newAverageTPS,
		peakTPS:          shardPeakTPS,
		lastBlockTxCount: uint32(shardTotalTxsForTPS),
	}

	log.Debug("TpsBenchmark.updateStatistics",
		"block", updatedChainStats.currentBlockNonce,
		"avgTPS", updatedChainStats.averageTPS,
		"peakTPS", updatedChainStats.peakTPS,
		"lastBlockTxCount", updatedChainStats.lastBlockTxCount,
		"avgBlockTxCount", updatedChainStats.averageBlockTxCount,
		"totalProcessedTxCount", updatedChainStats.totalProcessedTxCount,
	)

	s.statistics = updatedChainStats

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *TpsBenchmark) IsInterfaceNil() bool {
	return s == nil
}

// AverageTPS returns an average tps for all processed blocks in a shard
func (ss *ChainStatistics) AverageTPS() *big.Int {
	return ss.averageTPS
}

// AverageBlockTxCount returns an average transaction count for
func (ss *ChainStatistics) AverageBlockTxCount() uint32 {
	return ss.averageBlockTxCount
}

// CurrentBlockNonce returns the block nounce of the last processed block in a shard
func (ss *ChainStatistics) CurrentBlockNonce() uint64 {
	return ss.currentBlockNonce
}

// LiveTPS returns tps for the last block
func (ss *ChainStatistics) LiveTPS() float64 {
	return float64(uint64(ss.lastBlockTxCount) / ss.slotTime)
}

// PeakTPS returns peak tps for for all the blocks of the current shard
func (ss *ChainStatistics) PeakTPS() float64 {
	return ss.peakTPS
}

// LastBlockTxCount returns the number of transactions included in the last block
func (ss *ChainStatistics) LastBlockTxCount() uint32 {
	return ss.lastBlockTxCount
}

// TotalProcessedTxCount returns the total number of processed transactions for this shard
func (ss *ChainStatistics) TotalProcessedTxCount() *big.Int {
	return ss.totalProcessedTxCount
}

// IsInterfaceNil returns true if there is no value under the interface
func (ss *ChainStatistics) IsInterfaceNil() bool {
	return ss == nil
}
