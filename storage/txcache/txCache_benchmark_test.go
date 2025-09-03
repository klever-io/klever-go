package txcache

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

// BenchmarkTxCache_CleanExpiredTransactions benchmarks the cleanup performance
func BenchmarkTxCache_CleanExpiredTransactions(b *testing.B) {
	sizes := []struct {
		name           string
		senders        int
		txsPerSender   int
		percentExpired int
	}{
		{"Small_100x10_50%", 100, 10, 50},
		{"Medium_1000x10_50%", 1000, 10, 50},
		{"Large_5000x20_50%", 5000, 20, 50},
		{"VeryLarge_10000x10_30%", 10000, 10, 30},
		{"MostlyExpired_1000x10_90%", 1000, 10, 90},
		{"FewExpired_1000x10_10%", 1000, 10, 10},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			cache, err := NewTxCache(Config{
				Name:                          "bench",
				NumChunks:                     16,
				EvictionEnabled:               false,
				NumBytesThreshold:             1000000000,
				NumBytesPerSenderThreshold:    500000,
				CountThreshold:                1000000,
				CountPerSenderThreshold:       1000,
				NumSendersToPreemptivelyEvict: 100,
			})
			require.NoError(b, err)
			defer cache.Close()

			// Populate cache with mixed expired/valid transactions
			now := time.Now().Unix()
			for i := 0; i < size.senders; i++ {
				sender := []byte(fmt.Sprintf("sender-%d", i))
				for j := 0; j < size.txsPerSender; j++ {
					var expireTime int64
					if (j * 100 / size.txsPerSender) < size.percentExpired {
						expireTime = now - 120 // Expired 2 minutes ago
					} else {
						expireTime = now + 300 // Expires in 5 minutes
					}

					tx := &WrappedTransaction{
						Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(j)}},
						TxHash:   []byte(fmt.Sprintf("tx-%d-%d", i, j)),
						Size:     100,
						ExpireOn: expireTime,
					}
					cache.AddTx(tx)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cache.cleanExpiredTransactions()
			}

			stats := cache.GetCleanupStats()
			b.ReportMetric(float64(stats["lastCleanupDurationUs"].(uint64)), "us/op")
			b.ReportMetric(float64(stats["totalExpiredRemoved"].(uint64))/float64(b.N), "expired/op")
		})
	}
}

// TestTxCache_ConcurrentCleanupAndOperations tests concurrent cleanup with add/remove operations
func TestTxCache_ConcurrentCleanupAndOperations(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "concurrent-test",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             10000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                10000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 10,
	})
	require.NoError(t, err)
	defer cache.Close()

	stopFlag := atomic.Bool{}
	wg := sync.WaitGroup{}

	// Track operations
	addCount := atomic.Int64{}
	removeCount := atomic.Int64{}
	selectCount := atomic.Int64{}
	cleanupCount := atomic.Int64{}

	// Goroutine 1: Continuously add transactions with varying expiration times
	wg.Add(1)
	go func() {
		defer wg.Done()
		nonce := uint64(0)
		for !stopFlag.Load() {
			for i := 0; i < 10; i++ {
				sender := []byte(fmt.Sprintf("sender-%d", i))
				// Mix of expired and valid transactions
				expireTime := time.Now().Unix()
				if rand.Intn(100) < 30 { // 30% expired
					expireTime -= 120
				} else {
					expireTime += 300
				}

				tx := &WrappedTransaction{
					Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: nonce}},
					TxHash:   []byte(fmt.Sprintf("tx-%d-%d", i, nonce)),
					Size:     100 + rand.Int63n(900),
					ExpireOn: expireTime,
				}
				cache.AddTx(tx)
				addCount.Add(1)
				nonce++
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Goroutine 2: Continuously remove transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stopFlag.Load() {
			for i := 0; i < 5; i++ {
				txHash := []byte(fmt.Sprintf("tx-%d-%d", rand.Intn(10), rand.Intn(1000)))
				cache.RemoveTxByHash(txHash)
				removeCount.Add(1)
			}
			time.Sleep(15 * time.Millisecond)
		}
	}()

	// Goroutine 3: Continuously select transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stopFlag.Load() {
			cache.SelectTransactions(100, 10, 100000)
			selectCount.Add(1)
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Goroutine 4: Manual cleanup calls
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stopFlag.Load() {
			cache.cleanExpiredTransactions()
			cleanupCount.Add(1)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Goroutine 5: Read operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stopFlag.Load() {
			_ = cache.CountTx()
			_ = cache.CountSenders()
			_ = cache.NumBytes()

			// Try to get some transactions
			for i := 0; i < 5; i++ {
				txHash := []byte(fmt.Sprintf("tx-%d-%d", rand.Intn(10), rand.Intn(1000)))
				cache.GetByTxHash(txHash)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Let it run for 2 seconds
	time.Sleep(2 * time.Second)
	stopFlag.Store(true)
	wg.Wait()

	// Check stats
	stats := cache.GetCleanupStats()
	t.Logf("Operations completed - Adds: %d, Removes: %d, Selects: %d, Manual Cleanups: %d",
		addCount.Load(), removeCount.Load(), selectCount.Load(), cleanupCount.Load())
	t.Logf("Cleanup Stats: %+v", stats)

	// Verify no panics occurred and cache is still functional
	require.NotPanics(t, func() {
		cache.CountTx()
		cache.CountSenders()
		cache.cleanExpiredTransactions()
	})
}

// TestTxCache_CleanupEfficiency tests that cleanup actually removes expired transactions efficiently
func TestTxCache_CleanupEfficiency(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "efficiency-test",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             100000000,
		NumBytesPerSenderThreshold:    500000,
		CountThreshold:                100000,
		CountPerSenderThreshold:       1000,
		NumSendersToPreemptivelyEvict: 100,
	})
	require.NoError(t, err)
	defer cache.Close()

	// Add 1000 expired and 1000 valid transactions
	now := time.Now().Unix()
	expiredCount := 0
	validCount := 0

	for i := 0; i < 100; i++ {
		sender := []byte(fmt.Sprintf("sender-%d", i))
		for j := 0; j < 20; j++ {
			var expireTime int64
			if j < 10 {
				expireTime = now - 120 // Expired
				expiredCount++
			} else {
				expireTime = now + 300 // Valid
				validCount++
			}

			tx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(j)}},
				TxHash:   []byte(fmt.Sprintf("tx-%d-%d", i, j)),
				Size:     100,
				ExpireOn: expireTime,
			}
			cache.AddTx(tx)
		}
	}

	// Verify initial state
	require.Equal(t, uint64(expiredCount+validCount), cache.CountTx())

	// Run cleanup
	startTime := time.Now()
	cache.cleanExpiredTransactions()
	duration := time.Since(startTime)

	// Verify cleanup results
	require.Equal(t, uint64(validCount), cache.CountTx())

	stats := cache.GetCleanupStats()
	require.Equal(t, uint64(expiredCount), stats["totalExpiredRemoved"])
	require.Equal(t, uint64(1), stats["totalCleanupRuns"])

	t.Logf("Cleaned %d expired transactions in %v (%.2f tx/ms)",
		expiredCount, duration, float64(expiredCount)/float64(duration.Milliseconds()))
	t.Logf("Stats: %+v", stats)
}

// TestTxCache_CleanupUnderLoad tests cleanup behavior under heavy load
func TestTxCache_CleanupUnderLoad(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "load-test",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             10000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                5000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 10,
	})
	require.NoError(t, err)
	defer cache.Close()

	// Add transactions with different expiry times
	currentTime := time.Now()
	expiredTxCount := 0
	validTxCount := 0

	// Add transactions that will expire soon (use unique nonces starting from 1000)
	// Since Unix() truncates to seconds, we need to add at least 1 second to ensure expiry
	for s := range 3 {
		sender := fmt.Appendf(nil, "expired-sender-%d", s)
		for i := range 20 {
			tx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(1000 + i)}},
				TxHash:   fmt.Appendf(nil, "expired-tx-%d-%d", s, i),
				Size:     100,
				ExpireOn: currentTime.Unix(), // Expire immediately (current time)
			}
			ok, _ := cache.AddTx(tx)
			if ok {
				expiredTxCount++
			}
		}
	}

	// Add transactions that won't expire (use unique nonces starting from 2000)
	for s := range 2 {
		sender := fmt.Appendf(nil, "valid-sender-%d", s)
		for i := range 20 {
			tx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(2000 + i)}},
				TxHash:   fmt.Appendf(nil, "valid-tx-%d-%d", s, i),
				Size:     100,
				ExpireOn: currentTime.Add(10 * time.Second).Unix(), // Won't expire during test
			}
			ok, _ := cache.AddTx(tx)
			if ok {
				validTxCount++
			}
		}
	}

	t.Logf("Added %d transactions to expire, %d to remain valid", expiredTxCount, validTxCount)

	// Simulate ongoing load with new transactions (use different nonces starting from 0)
	done := make(chan struct{})
	errorCount := atomic.Int64{}

	// Producer goroutines add new transactions continuously
	for p := range 2 {
		go func(producerId int) {
			iteration := 0
			for {
				select {
				case <-done:
					return
				default:
					// Use different sender names to avoid conflicts
					sender := fmt.Appendf(nil, "load-producer-%d", producerId)
					for i := range 5 {
						tx := &WrappedTransaction{
							Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(iteration*5 + i)}},
							TxHash:   fmt.Appendf(nil, "load-tx-%d-%d-%d", producerId, iteration*5+i, time.Now().UnixNano()),
							Size:     100,
							ExpireOn: time.Now().Add(5 * time.Second).Unix(), // Won't expire during test
						}
						ok, _ := cache.AddTx(tx)
						if !ok {
							errorCount.Add(1)
						}
					}
					iteration++
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(p)
	}

	// Wait a bit to ensure time has advanced
	// Since we set ExpireOn to current time, any advancement will expire them
	time.Sleep(1100 * time.Millisecond) // Wait just over 1 second to ensure Unix() time advances

	// Debug: Check some transactions before cleanup
	nowTime := time.Now().Unix()
	t.Logf("Time elapsed since adding expired txs: %dms", (nowTime-currentTime.Unix())*1000)

	// Check if any transactions are actually expired
	senders := cache.txListBySender.getSnapshotAscending()
	var sampleExpiredCount, sampleNotExpiredCount int
	for _, listForSender := range senders {
		listForSender.mutex.Lock()
		for element := listForSender.items.Front(); element != nil; element = element.Next() {
			if tx, ok := element.Value.(*WrappedTransaction); ok {
				if tx.ExpireOn < nowTime {
					sampleExpiredCount++
					if sampleExpiredCount <= 3 {
						t.Logf("Found expired tx: ExpireOn=%d, Now=%d, Diff=%d", tx.ExpireOn, nowTime, nowTime-tx.ExpireOn)
					}
				} else {
					sampleNotExpiredCount++
				}
			}
		}
		listForSender.mutex.Unlock()
	}
	t.Logf("Pre-cleanup check: %d expired, %d not expired", sampleExpiredCount, sampleNotExpiredCount)

	// Now trigger cleanup while still under load
	beforeCleanup := cache.CountTx()
	cache.cleanExpiredTransactions()
	afterCleanup := cache.CountTx()

	close(done)
	time.Sleep(500 * time.Millisecond) // Let goroutines finish

	stats := cache.GetCleanupStats()
	t.Logf("Under load - Before: %d, After: %d, Removed: %d",
		beforeCleanup, afterCleanup, stats["totalExpiredRemoved"])
	t.Logf("Error count: %d", errorCount.Load())
	t.Logf("Final stats: %+v", stats)

	// Verify cleanup worked - we should have removed the expired transactions
	require.Greater(t, stats["totalExpiredRemoved"].(uint64), uint64(0), "Should have removed at least some expired transactions")
	require.Less(t, afterCleanup, beforeCleanup, "Transaction count should decrease after cleanup")
}
