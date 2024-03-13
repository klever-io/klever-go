package statistics_test

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/block"
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
	_ = tpsBenchmark.LastBlockTxCount()
	_ = tpsBenchmark.TotalProcessedTxCount()
	_ = tpsBenchmark.LiveTPS()
	_ = tpsBenchmark.PeakTPS()
	_ = tpsBenchmark.Statistic()
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
	assert.False(t, tpsBenchmark.Statistic().IsInterfaceNil())
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
	assert.Equal(t, blockNumber, tpsBenchmark.BlockNumber())
	assert.Equal(t, float64(totalTxs), tpsBenchmark.PeakTPS())
	assert.Equal(t, totalTxs, tpsBenchmark.LastBlockTxCount())
	assert.Equal(t, big.NewInt(int64(totalTxs)), tpsBenchmark.AverageBlockTxCount())
	assert.Equal(t, big.NewInt(int64(totalTxs)), tpsBenchmark.TotalProcessedTxCount())
	assert.Equal(t, totalTxs, tpsBenchmark.Statistic().LastBlockTxCount())
}

func TestTPSBenchmarkChainStatistics_GettersAndSetters(t *testing.T) {
	t.Parallel()

	slotInterval := uint64(1)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)
	slot := uint64(1)
	blockNumber := slot
	txCount := uint32(5)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:      blockNumber,
			Slot:       slot,
			ParentHash: []byte{1},
			TxCount:    txCount,
		},
	}

	tpsBenchmark.Update(metaBlock)

	statistics := tpsBenchmark.Statistic()
	assert.NotNil(t, statistics)

	assert.Equal(t, 1, statistics.AverageTPS().Cmp(big.NewInt(0)))
	assert.True(t, statistics.LiveTPS() > 0)
	assert.Equal(t, slot, statistics.CurrentBlockNonce())
	assert.Equal(t, float64(txCount), statistics.PeakTPS())
	assert.Equal(t, txCount, statistics.LastBlockTxCount())
	assert.Equal(t, big.NewInt(int64(txCount)), statistics.TotalProcessedTxCount())

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
