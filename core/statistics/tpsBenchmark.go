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
	AverageTPS            *big.Int
	AverageBlockTxCount   *big.Int
	TotalProcessedTxCount *big.Int
	CurrentBlockTxCount   uint32
}

// TpsBenchmarkSnapshot is a consistent point-in-time copy of a TpsBenchmark.
// All *big.Int fields are defensive copies — safe to read or mutate without affecting the source.
type TpsBenchmarkSnapshot struct {
	BlockNumber           uint64
	SlotNumber            uint64
	SlotTime              uint64
	PeakTPS               float64
	LiveTPS               float64
	CurrentBlockTxCount   uint32
	AverageTPS            *big.Int
	AverageBlockTxCount   *big.Int
	TotalProcessedTxCount *big.Int
}

// copyBigInt returns a defensive deep copy of b, or a fresh zero when b is nil.
func copyBigInt(b *big.Int) *big.Int {
	if b == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(b)
}

// TpsBenchmark will calculate statistics for the network activity
type TpsBenchmark struct {
	mut         sync.RWMutex
	activeNodes uint32
	slotTime    uint64
	blockNumber uint64
	slotNumber  uint64
	averageTPS  *big.Int

	peakTPS               float64
	averageBlockTxCount   *big.Int
	currentBlockTxCount   uint32
	totalProcessedTxCount *big.Int
	statusHandler         core.AppStatusHandler
	initialBlockNumber    int64
}

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

	return &TpsBenchmark{
		slotTime:              slotInterval,
		peakTPS:               initialTpsBenchmark.PeakTPS,
		averageTPS:            copyBigInt(initialTpsBenchmark.AverageTPS),
		currentBlockTxCount:   initialTpsBenchmark.CurrentBlockTxCount,
		blockNumber:           initialTpsBenchmark.BlockNumber,
		slotNumber:            initialTpsBenchmark.SlotNumber,
		totalProcessedTxCount: copyBigInt(initialTpsBenchmark.TotalProcessedTxCount),
		averageBlockTxCount:   copyBigInt(initialTpsBenchmark.AverageBlockTxCount),
		statusHandler:         appStatusHandler,
		initialBlockNumber:    int64(initialTpsBenchmark.BlockNumber), // #nosec G115
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

	return &TpsBenchmark{
		slotTime:              slotInterval,
		statusHandler:         statusHandler.NewNilStatusHandler(),
		totalProcessedTxCount: big.NewInt(0),
		averageTPS:            big.NewInt(0),
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
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.blockNumber
}

// SlotNumber returns the slot index for this benchmark object
func (s *TpsBenchmark) SlotNumber() uint64 {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.slotNumber
}

// AverageBlockTxCount returns an average of the tx/block
func (s *TpsBenchmark) AverageBlockTxCount() *big.Int {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return copyBigInt(s.averageBlockTxCount)
}

// CurrentBlockTxCount returns the number of transactions processed in the current block
func (s *TpsBenchmark) CurrentBlockTxCount() uint32 {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.currentBlockTxCount
}

// TotalProcessedTxCount returns the total number of processed transactions
func (s *TpsBenchmark) TotalProcessedTxCount() *big.Int {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return copyBigInt(s.totalProcessedTxCount)
}

// LiveTPS returns tps for the current block
func (s *TpsBenchmark) LiveTPS() float64 {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return float64(uint64(s.currentBlockTxCount) / s.slotTime)
}

// PeakTPS returns tps for the last block
func (s *TpsBenchmark) PeakTPS() float64 {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.peakTPS
}

// AverageTPS returns the average tps for the last block
func (s *TpsBenchmark) AverageTPS() *big.Int {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return copyBigInt(s.averageTPS)
}

// Snapshot returns a consistent, defensively-copied view of all benchmark fields under a single read lock.
// Prefer this over per-getter calls when consuming multiple fields together (e.g. for API responses) —
// it guarantees the returned values all come from the same block.
func (s *TpsBenchmark) Snapshot() TpsBenchmarkSnapshot {
	s.mut.RLock()
	defer s.mut.RUnlock()

	return TpsBenchmarkSnapshot{
		BlockNumber:           s.blockNumber,
		SlotNumber:            s.slotNumber,
		SlotTime:              s.slotTime,
		PeakTPS:               s.peakTPS,
		LiveTPS:               float64(uint64(s.currentBlockTxCount) / s.slotTime),
		CurrentBlockTxCount:   s.currentBlockTxCount,
		AverageTPS:            copyBigInt(s.averageTPS),
		AverageBlockTxCount:   copyBigInt(s.averageBlockTxCount),
		TotalProcessedTxCount: copyBigInt(s.totalProcessedTxCount),
	}
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

	totalTxs := uint64(b.Header.TxCount)
	s.currentBlockTxCount = b.Header.TxCount

	currentTPS := float64(totalTxs / s.slotTime)

	if currentTPS > s.peakTPS {
		s.peakTPS = currentTPS
	}

	s.totalProcessedTxCount.Add(s.totalProcessedTxCount, big.NewInt(0).SetUint64(totalTxs))
	s.averageBlockTxCount.Quo(s.totalProcessedTxCount, big.NewInt(0).SetUint64(b.Header.Nonce))
	s.averageTPS = big.NewInt(0).SetUint64(s.totalProcessedTxCount.Uint64() / b.Header.Slot)

	s.statusHandler.AddUint64(core.MetricNumProcessedTxsTPSBenchmark, totalTxs)
	s.statusHandler.SetUInt64Value(core.MetricNonceForTPS, b.Header.Nonce)
	s.statusHandler.SetUInt64Value(core.MetricCurrentBlockTxCount, totalTxs)
	s.statusHandler.SetUInt64Value(core.MetricPeakTPS, uint64(s.peakTPS))
	s.statusHandler.SetStringValue(core.MetricAverageTPS, s.averageTPS.String())
	s.statusHandler.SetStringValue(core.MetricAverageBlockTxCount, s.averageBlockTxCount.String())

	log.Debug("TPS benchmark updated",
		"peakTPS", s.peakTPS,
		"averageTPS", s.averageTPS,
		"liveTPS", currentTPS,
		"currentBlockTxCount", s.currentBlockTxCount,
		"avgBlockTxCount", s.averageBlockTxCount,
		"totalProcessedTxCount", s.totalProcessedTxCount,
	)

	return nil
}

func (s *TpsBenchmark) IsInterfaceNil() bool {
	return s == nil
}
