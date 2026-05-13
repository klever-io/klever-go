package statistics_test

import (
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func updateTpsBenchmark(tpsBenchmark *statistics.TpsBenchmark, txCount uint32, nonce uint64) {
	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:   nonce,
			Slot:    nonce,
			TxCount: txCount,
		},
	}
	tpsBenchmark.Update(metaBlock)

	_ = tpsBenchmark.ActiveNodes()
	_ = tpsBenchmark.SlotTime()
	_ = tpsBenchmark.BlockNumber()
	_ = tpsBenchmark.SlotNumber()
	_ = tpsBenchmark.AverageBlockTxCount()
	_ = tpsBenchmark.CurrentBlockTxCount()
	_ = tpsBenchmark.TotalProcessedTxCount()
	_ = tpsBenchmark.LiveTPS()
	_ = tpsBenchmark.PeakTPS()
	_ = tpsBenchmark.AverageTPS()
}

func TestTpsBenchmark_NewTPSBenchmarkReturnsErrorOnInvalidDuration(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(0)
	tpsBenchmark, err := statistics.NewTPSBenchmark(slotInterval)
	assert.Nil(t, tpsBenchmark)
	assert.Equal(t, err, statistics.ErrInvalidSlotInterval)
}

func TestTpsBenchmark_NewTPSBenchmark(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(4)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)

	assert.Equal(t, tpsBenchmark.SlotTime(), slotInterval)
	assert.False(t, check.IfNil(tpsBenchmark))
}

func TestTpsBenchmark_BlockNumber(t *testing.T) {
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(1)
	blockNumber := uint64(1)
	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber,
			Slot:       blockNumber,
			ParentHash: []byte{1},
			TxCount:    10,
		},
	}
	assert.Equal(t, tpsBenchmark.BlockNumber(), uint64(0))
	tpsBenchmark.Update(metaBlock)
	assert.Equal(t, tpsBenchmark.BlockNumber(), blockNumber)
}

func TestTpsBenchmark_UpdateIrrelevantBlock(t *testing.T) {
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(1)

	tpsBenchmark.Update(nil)
	assert.Equal(t, tpsBenchmark.BlockNumber(), uint64(0))
}

func TestTpsBenchmark_UpdateTotalNumberOfTx(t *testing.T) {
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(1)
	slot := uint64(1)
	blockNumber := slot
	totalTxCount := big.NewInt(int64(2000))

	metaBlock1 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber,
			Slot:       slot,
			ParentHash: []byte{1},
			TxCount:    1000,
		},
	}

	metaBlock2 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber + 1,
			Slot:       slot + 1,
			ParentHash: []byte{1},
			TxCount:    1000,
		},
	}

	tpsBenchmark.Update(metaBlock1)
	tpsBenchmark.Update(metaBlock2)
	assert.Equal(t, tpsBenchmark.TotalProcessedTxCount(), totalTxCount)
}

func TestTpsBenchmark_UpdatePeakTps(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(1)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	slot := uint64(1)
	blockNumber := slot
	peakTps := uint32(20)

	metaBlock1 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber,
			Slot:       slot,
			ParentHash: []byte{1},
			TxCount:    20,
		},
	}

	metaBlock2 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber + 1,
			Slot:       slot + 1,
			ParentHash: []byte{1},
			TxCount:    10,
		},
	}

	tpsBenchmark.Update(metaBlock1)
	tpsBenchmark.Update(metaBlock2)
	assert.Equal(t, float64(peakTps), tpsBenchmark.PeakTPS())
}

func TestTPSBenchmark_GettersAndSetters(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(1)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	slot := uint64(1)
	blockNumber := slot

	totalTxs := uint32(10)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber,
			Slot:       slot,
			ParentHash: []byte{1},
			TxCount:    totalTxs,
		},
	}

	tpsBenchmark.Update(metaBlock)

	assert.Equal(t, slotInterval, tpsBenchmark.SlotTime())
	assert.Equal(t, blockNumber, tpsBenchmark.BlockNumber())
	assert.Equal(t, float64(totalTxs), tpsBenchmark.PeakTPS())
	assert.Equal(t, totalTxs, tpsBenchmark.CurrentBlockTxCount())
	assert.Equal(t, big.NewInt(int64(totalTxs)), tpsBenchmark.AverageBlockTxCount())
	assert.Equal(t, big.NewInt(int64(totalTxs)), tpsBenchmark.TotalProcessedTxCount())
}

func TestTpsBenchmark_EmptyBlocksShouldNotUpdateMultipleTimes(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(6)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	txCount := uint32(5205)

	metaBlock1 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			Slot:       2,
			ParentHash: []byte{1},
			TxCount:    5205,
		},
	}

	tpsBenchmark.Update(metaBlock1)

	metaBlock2 := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			Slot:       2,
			ParentHash: []byte{1},
			TxCount:    5205,
		},
	}
	tpsBenchmark.Update(metaBlock2)

	bigTxCount := big.NewInt(int64(txCount))
	assert.Equal(t, bigTxCount, tpsBenchmark.TotalProcessedTxCount())
}

func TestTpsBenchmark_TpsShouldUpdateButTxsCountShouldNot(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(6)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	txCount := uint32(60)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			Slot:       2,
			ParentHash: []byte{1},
			TxCount:    txCount,
		},
	}
	tpsBenchmark.Update(metaBlock)

	assert.Equal(t, big.NewInt(int64(txCount)), tpsBenchmark.TotalProcessedTxCount())
	assert.Equal(t, float64(txCount)/float64(slotInterval), tpsBenchmark.LiveTPS())
}

func TestTpsBenchmark_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	t.Parallel()

	numOfTests := 25
	for i := 0; i < numOfTests; i++ {
		testTpsBenchmarkConcurrent(t)
	}
}

func testTpsBenchmarkConcurrent(t *testing.T) {
	slotInterval := uint64(6)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	txCount := uint32(10)
	nrGoroutines := 8000

	wg := sync.WaitGroup{}
	wg.Add(nrGoroutines)

	for i := 1; i <= nrGoroutines; i++ {
		go func(nonce int) {
			time.Sleep(time.Millisecond)
			updateTpsBenchmark(tpsBenchmark, txCount, uint64(nonce))
			wg.Done()
		}(i)
	}
	wg.Wait()

	bigTxCount := big.NewInt(int64(txCount))
	bigTxCount.Mul(bigTxCount, big.NewInt(int64(nrGoroutines)))
	assert.Equal(t, bigTxCount, tpsBenchmark.TotalProcessedTxCount())
}

func TestTpsBenchmark_ZeroTxMetaBlockAndShardHeader(t *testing.T) {
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(4)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			Slot:       2,
			ParentHash: []byte{1},
			TxCount:    0,
		},
	}
	tpsBenchmark.Update(metaBlock)

	bigTxCount := big.NewInt(0)
	assert.Equal(t, bigTxCount, tpsBenchmark.TotalProcessedTxCount())
}

func TestTpsBenchmark_ZeroTxMetaBlockAndEmptyShardHeader(t *testing.T) {
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(4)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			Slot:       2,
			ParentHash: []byte{1},
			TxCount:    0,
		},
	}
	tpsBenchmark.Update(metaBlock)

	bigTxCount := big.NewInt(0)
	assert.Equal(t, bigTxCount, tpsBenchmark.TotalProcessedTxCount())
}

func TestTpsBenchmark_NewTPSBenchmarkWithInitialData(t *testing.T) {
	t.Parallel()

	t.Run("zero slotInterval returns ErrInvalidSlotInterval", func(t *testing.T) {
		t.Parallel()
		tps, err := statistics.NewTPSBenchmarkWithInitialData(
			statusHandler.NewNilStatusHandler(),
			&statistics.TpsPersistentData{},
			0,
		)
		assert.Nil(t, tps)
		assert.Equal(t, statistics.ErrInvalidSlotInterval, err)
	})

	t.Run("nil initial benchmark returns ErrNilInitialTPSBenchmarks", func(t *testing.T) {
		t.Parallel()
		tps, err := statistics.NewTPSBenchmarkWithInitialData(
			statusHandler.NewNilStatusHandler(),
			nil,
			6,
		)
		assert.Nil(t, tps)
		assert.Equal(t, statistics.ErrNilInitialTPSBenchmarks, err)
	})

	t.Run("nil status handler returns ErrNilStatusHandler", func(t *testing.T) {
		t.Parallel()
		tps, err := statistics.NewTPSBenchmarkWithInitialData(
			nil,
			&statistics.TpsPersistentData{
				AverageTPS:            big.NewInt(0),
				AverageBlockTxCount:   big.NewInt(0),
				TotalProcessedTxCount: big.NewInt(0),
			},
			6,
		)
		assert.Nil(t, tps)
		assert.Equal(t, statistics.ErrNilStatusHandler, err)
	})

	t.Run("happy path with full data and aliasing protection", func(t *testing.T) {
		t.Parallel()
		// The constructor must defensively copy *big.Int inputs so callers that mutate
		// the source after construction cannot race the writer.
		totalTxs := big.NewInt(1234)
		avgBlockTxs := big.NewInt(5)
		avgTPS := big.NewInt(2)

		tps, err := statistics.NewTPSBenchmarkWithInitialData(
			statusHandler.NewNilStatusHandler(),
			&statistics.TpsPersistentData{
				BlockNumber:           42,
				SlotNumber:            100,
				PeakTPS:               7.5,
				CurrentBlockTxCount:   30,
				AverageTPS:            avgTPS,
				AverageBlockTxCount:   avgBlockTxs,
				TotalProcessedTxCount: totalTxs,
			},
			6,
		)
		assert.NoError(t, err)
		assert.NotNil(t, tps)
		assert.Equal(t, uint64(42), tps.BlockNumber())
		assert.Equal(t, uint64(100), tps.SlotNumber())
		assert.Equal(t, big.NewInt(1234), tps.TotalProcessedTxCount())
		assert.Equal(t, big.NewInt(5), tps.AverageBlockTxCount())
		assert.Equal(t, big.NewInt(2), tps.AverageTPS())

		// Mutate the source *big.Int values — the benchmark must be unaffected.
		totalTxs.SetInt64(9999)
		avgBlockTxs.SetInt64(9999)
		avgTPS.SetInt64(9999)

		assert.Equal(t, big.NewInt(1234), tps.TotalProcessedTxCount())
		assert.Equal(t, big.NewInt(5), tps.AverageBlockTxCount())
		assert.Equal(t, big.NewInt(2), tps.AverageTPS())
	})

	t.Run("nil *big.Int fields are tolerated and exposed as zero", func(t *testing.T) {
		t.Parallel()
		tps, err := statistics.NewTPSBenchmarkWithInitialData(
			statusHandler.NewNilStatusHandler(),
			&statistics.TpsPersistentData{
				BlockNumber: 1,
				// AverageTPS, AverageBlockTxCount, TotalProcessedTxCount left nil
			},
			6,
		)
		assert.NoError(t, err)
		assert.NotNil(t, tps)
		assert.Equal(t, big.NewInt(0), tps.TotalProcessedTxCount())
		assert.Equal(t, big.NewInt(0), tps.AverageBlockTxCount())
		assert.Equal(t, big.NewInt(0), tps.AverageTPS())
	})
}

// TestTpsBenchmark_SnapshotConsistencyUnderConcurrentUpdates verifies that Snapshot()
// returns a cross-field-consistent view: every snapshot must satisfy the invariant
// enforced by updateStatistics — averageBlockTxCount = totalProcessedTxCount / blockNumber
// (integer quotient). Per-getter calls would tear this invariant when Update() interleaves
// between calls; Snapshot() takes a single RLock so all three fields advance together.
func TestTpsBenchmark_SnapshotConsistencyUnderConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	t.Parallel()

	tpsBenchmark, _ := statistics.NewTPSBenchmark(uint64(6))
	txCount := uint32(10)
	nrUpdates := 2000

	done := make(chan struct{})
	var inconsistent atomic.Uint64
	var observed atomic.Uint64

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			snap := tpsBenchmark.Snapshot()
			if snap.BlockNumber == 0 {
				continue
			}
			observed.Add(1)
			blockNumber := new(big.Int).SetUint64(snap.BlockNumber)
			lo := new(big.Int).Mul(snap.AverageBlockTxCount, blockNumber)
			hi := new(big.Int).Add(lo, blockNumber)
			if snap.TotalProcessedTxCount.Cmp(lo) < 0 || snap.TotalProcessedTxCount.Cmp(hi) >= 0 {
				inconsistent.Add(1)
			}
		}
	}()

	for i := 1; i <= nrUpdates; i++ {
		updateTpsBenchmark(tpsBenchmark, txCount, uint64(i))
	}
	close(done)
	wg.Wait()

	assert.Equal(t, uint64(0), inconsistent.Load(),
		"Snapshot returned %d inconsistent views out of %d observations",
		inconsistent.Load(), observed.Load())
	assert.Greater(t, observed.Load(), uint64(0), "reader should have observed at least one non-zero snapshot")
}
