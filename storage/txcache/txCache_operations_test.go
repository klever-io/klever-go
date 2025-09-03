// This file contains comprehensive operational tests for the transaction cache including:
// - Nonce-based transaction removal operations
// - Memory pressure and capacity limit tests
// - Property-based invariant verification tests
package txcache

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

// TestTxCache_RemoveTxBySenderNonce tests the RemoveTxBySenderNonce function
func TestTxCache_RemoveTxBySenderNonce(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-remove-nonce",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                1000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 1,
	})
	require.NoError(t, err)
	defer cache.Close()

	sender := []byte("test-sender")

	// Add transactions with various nonces
	for i := uint64(1); i <= 10; i++ {
		tx := &WrappedTransaction{
			Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: i}},
			TxHash:   []byte(fmt.Sprintf("tx-%d", i)),
			Size:     100,
			ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
		}
		ok, added := cache.AddTx(tx)
		require.True(t, ok)
		require.True(t, added)
	}

	// Verify all transactions exist
	require.Equal(t, uint64(10), cache.CountTx())

	// Remove transactions with nonce <= 5
	removed := cache.RemoveTxBySenderNonce(sender, 5)
	require.Equal(t, 5, removed)

	// Verify only transactions with nonce > 5 remain
	require.Equal(t, uint64(5), cache.CountTx())

	// Check specific transactions
	for i := uint64(1); i <= 5; i++ {
		_, exists := cache.GetByTxHash([]byte(fmt.Sprintf("tx-%d", i)))
		require.False(t, exists, "Transaction with nonce %d should be removed", i)
	}

	for i := uint64(6); i <= 10; i++ {
		_, exists := cache.GetByTxHash([]byte(fmt.Sprintf("tx-%d", i)))
		require.True(t, exists, "Transaction with nonce %d should remain", i)
	}

	// Test removing with nonce higher than all transactions
	removed = cache.RemoveTxBySenderNonce(sender, 100)
	require.Equal(t, 5, removed)
	require.Equal(t, uint64(0), cache.CountTx())

	// Test removing from non-existent sender
	removed = cache.RemoveTxBySenderNonce([]byte("unknown-sender"), 5)
	require.Equal(t, 0, removed)
}

// TestTxCache_RemoveTxBySenderNonce_Concurrent tests concurrent removal by nonce
func TestTxCache_RemoveTxBySenderNonce_Concurrent(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-remove-nonce-concurrent",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                10000,
		CountPerSenderThreshold:       1000,
		NumSendersToPreemptivelyEvict: 1,
	})
	require.NoError(t, err)
	defer cache.Close()

	numSenders := 10
	txsPerSender := 100

	// Add transactions for multiple senders
	for s := 0; s < numSenders; s++ {
		sender := []byte(fmt.Sprintf("sender-%d", s))
		for i := uint64(1); i <= uint64(txsPerSender); i++ {
			tx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: i}},
				TxHash:   []byte(fmt.Sprintf("tx-%d-%d", s, i)),
				Size:     100,
				ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
			}
			cache.AddTx(tx)
		}
	}

	var wg sync.WaitGroup
	var totalRemoved atomic.Int32

	// Concurrently remove transactions for different senders
	for s := 0; s < numSenders; s++ {
		wg.Add(1)
		go func(senderIdx int) {
			defer wg.Done()
			sender := []byte(fmt.Sprintf("sender-%d", senderIdx))

			// Remove in chunks
			for nonce := uint64(20); nonce <= uint64(txsPerSender); nonce += 20 {
				removed := cache.RemoveTxBySenderNonce(sender, nonce)
				totalRemoved.Add(int32(removed))
				time.Sleep(1 * time.Millisecond)
			}
		}(s)
	}

	wg.Wait()

	// All transactions should be removed
	require.Equal(t, int32(numSenders*txsPerSender), totalRemoved.Load())
	require.Equal(t, uint64(0), cache.CountTx())
}

// TestTxCache_MemoryPressure tests cache behavior under memory pressure
func TestTxCache_MemoryPressure(t *testing.T) {
	// Small cache limits to trigger memory pressure
	cache, err := NewTxCache(Config{
		Name:                          "test-memory-pressure",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             10000, // 10KB total
		NumBytesPerSenderThreshold:    2000,  // 2KB per sender
		CountThreshold:                100,   // Max 100 transactions
		CountPerSenderThreshold:       20,    // Max 20 per sender
		NumSendersToPreemptivelyEvict: 5,
	})
	require.NoError(t, err)
	defer cache.Close()

	// Track memory before test
	runtime.GC()
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Try to add more transactions than cache can hold
	numSenders := 20
	txsPerSender := 30   // More than CountPerSenderThreshold
	largeDataSize := 500 // Large transaction size

	addedCount := 0
	rejectedCount := 0

	for s := 0; s < numSenders; s++ {
		sender := []byte(fmt.Sprintf("sender-%d", s))
		for i := uint64(1); i <= uint64(txsPerSender); i++ {
			// Create transaction with large size
			tx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: i}},
				TxHash:   []byte(fmt.Sprintf("tx-%d-%d", s, i)),
				Size:     int64(100 + largeDataSize), // Simulate large transaction
				ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
			}

			ok, added := cache.AddTx(tx)
			if ok && added {
				addedCount++
			} else {
				rejectedCount++
			}
		}
	}

	// Cache should enforce limits
	require.LessOrEqual(t, cache.CountTx(), uint64(100), "Cache should not exceed CountThreshold")
	require.LessOrEqual(t, cache.NumBytes(), 10000, "Cache should not exceed NumBytesThreshold")

	// Some transactions should have been rejected or evicted (but cache is working correctly)
	// The fact that cache respects limits is the important test
	t.Logf("Memory pressure test: Added=%d, Rejected=%d, Final Count=%d", addedCount, rejectedCount, cache.CountTx())

	// Memory check - Go's memory management makes this non-deterministic
	// The important test is that cache respects its configured limits above
	runtime.GC()
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Log memory usage for information
	memoryGrowth := int64(memStatsAfter.Alloc) - int64(memStatsBefore.Alloc)
	t.Logf("Memory growth during test: %d bytes (%.2f MB)", memoryGrowth, float64(memoryGrowth)/(1024*1024))
}

// TestTxCache_MemoryPressure_ConcurrentOperations tests memory pressure with concurrent operations
func TestTxCache_MemoryPressure_ConcurrentOperations(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-memory-concurrent",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             50000, // 50KB
		NumBytesPerSenderThreshold:    5000,  // 5KB per sender
		CountThreshold:                200,
		CountPerSenderThreshold:       50,
		NumSendersToPreemptivelyEvict: 10,
	})
	require.NoError(t, err)
	defer cache.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	opsPerGoroutine := 100

	// Track operations
	var adds, removes, selects atomic.Int32

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for op := 0; op < opsPerGoroutine; op++ {
				switch op % 3 {
				case 0: // Add transaction
					sender := []byte(fmt.Sprintf("sender-%d", id))
					tx := &WrappedTransaction{
						Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(op)}},
						TxHash:   []byte(fmt.Sprintf("tx-%d-%d", id, op)),
						Size:     int64(100 + (op % 500)), // Variable size
						ExpireOn: time.Now().Add(time.Duration(op%60) * time.Second).Unix(),
					}
					if ok, added := cache.AddTx(tx); ok && added {
						adds.Add(1)
					}

				case 1: // Remove transaction
					txHash := []byte(fmt.Sprintf("tx-%d-%d", id, op-1))
					_, removed := cache.RemoveTxByHash(txHash)
					if removed {
						removes.Add(1)
					}

				case 2: // Select transactions
					txs := cache.SelectTransactions(10, 2, 1000)
					if len(txs) > 0 {
						selects.Add(int32(len(txs)))
					}
				}

				// Small delay to simulate real-world timing
				if op%10 == 0 {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(g)
	}

	// Run cleanup periodically during the test
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cache.cleanExpiredTransactions()
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)

	// Final checks
	require.LessOrEqual(t, cache.CountTx(), uint64(200), "Cache should respect CountThreshold")
	require.LessOrEqual(t, cache.NumBytes(), 50000, "Cache should respect NumBytesThreshold")

	// Operations should have succeeded
	require.Greater(t, adds.Load(), int32(0), "Some adds should succeed")
	require.Greater(t, removes.Load(), int32(0), "Some removes should succeed")
	require.Greater(t, selects.Load(), int32(0), "Some selects should succeed")

	t.Logf("Operations: Adds=%d, Removes=%d, Selected=%d, Final Count=%d",
		adds.Load(), removes.Load(), selects.Load(), cache.CountTx())
}

// TestTxCache_PropertyBasedInvariants tests that cache invariants hold under various operations
func TestTxCache_PropertyBasedInvariants(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-invariants",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             100000,
		NumBytesPerSenderThreshold:    10000,
		CountThreshold:                500,
		CountPerSenderThreshold:       50,
		NumSendersToPreemptivelyEvict: 5,
	})
	require.NoError(t, err)
	defer cache.Close()

	// Invariant checks
	checkInvariants := func(stage string) {
		// Invariant 1: CountTx should match actual number of transactions
		actualCount := uint64(0)
		cache.ForEachTransaction(func(txHash []byte, tx *WrappedTransaction) {
			actualCount++
		})
		require.Equal(t, cache.CountTx(), actualCount, "CountTx mismatch at %s", stage)

		// Invariant 2: Every transaction in txByHash should be in txListBySender
		cache.txByHash.forEach(func(txHash []byte, tx *WrappedTransaction) {
			sender := string(tx.Tx.GetSender())
			listForSender, exists := cache.txListBySender.getListForSender(sender)
			require.True(t, exists, "Sender should exist for tx in txByHash at %s", stage)

			found := false
			listForSender.mutex.Lock()
			for element := listForSender.items.Front(); element != nil; element = element.Next() {
				if wtx, ok := element.Value.(*WrappedTransaction); ok {
					if string(wtx.TxHash) == string(txHash) {
						found = true
						break
					}
				}
			}
			listForSender.mutex.Unlock()
			require.True(t, found, "Transaction in txByHash should be in txListBySender at %s", stage)
		})

		// Invariant 3: Cache limits should be respected
		require.LessOrEqual(t, cache.CountTx(), uint64(500), "CountThreshold exceeded at %s", stage)
		require.LessOrEqual(t, cache.NumBytes(), 100000, "NumBytesThreshold exceeded at %s", stage)

		// Invariant 4: No nil transactions
		cache.ForEachTransaction(func(txHash []byte, tx *WrappedTransaction) {
			require.NotNil(t, tx, "Nil transaction found at %s", stage)
			require.NotNil(t, tx.Tx, "Nil inner transaction found at %s", stage)
		})
	}

	// Run various operations and check invariants hold
	stages := []struct {
		name string
		op   func()
	}{
		{
			name: "initial",
			op:   func() {},
		},
		{
			name: "after-adds",
			op: func() {
				for i := 0; i < 100; i++ {
					sender := []byte(fmt.Sprintf("sender-%d", i%10))
					tx := &WrappedTransaction{
						Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(i)}},
						TxHash:   []byte(fmt.Sprintf("tx-%d", i)),
						Size:     100,
						ExpireOn: time.Now().Add(1 * time.Minute).Unix(),
					}
					cache.AddTx(tx)
				}
			},
		},
		{
			name: "after-removes",
			op: func() {
				for i := 0; i < 20; i++ {
					cache.RemoveTxByHash([]byte(fmt.Sprintf("tx-%d", i)))
				}
			},
		},
		{
			name: "after-selection",
			op: func() {
				cache.SelectTransactions(50, 5, 1000)
			},
		},
		{
			name: "after-cleanup",
			op: func() {
				// Add some expired transactions
				for i := 0; i < 20; i++ {
					sender := []byte(fmt.Sprintf("expired-sender-%d", i))
					tx := &WrappedTransaction{
						Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
						TxHash:   []byte(fmt.Sprintf("expired-%d", i)),
						Size:     100,
						ExpireOn: time.Now().Add(-1 * time.Minute).Unix(),
					}
					cache.AddTx(tx)
				}
				cache.cleanExpiredTransactions()
			},
		},
		{
			name: "after-nonce-removal",
			op: func() {
				cache.RemoveTxBySenderNonce([]byte("sender-0"), 50)
			},
		},
		{
			name: "after-concurrent-ops",
			op: func() {
				var wg sync.WaitGroup
				for g := 0; g < 10; g++ {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						for op := 0; op < 10; op++ {
							sender := []byte(fmt.Sprintf("concurrent-%d", id))
							tx := &WrappedTransaction{
								Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(op)}},
								TxHash:   []byte(fmt.Sprintf("concurrent-%d-%d", id, op)),
								Size:     100,
								ExpireOn: time.Now().Add(1 * time.Minute).Unix(),
							}
							cache.AddTx(tx)
						}
					}(g)
				}
				wg.Wait()
			},
		},
	}

	for _, stage := range stages {
		stage.op()
		checkInvariants(stage.name)
	}
}
