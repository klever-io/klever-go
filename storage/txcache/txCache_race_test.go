package txcache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

// TestTxCache_TOCTOU_RaceCondition_EmptySenderRemoval demonstrates the TOCTOU race condition
// where a sender marked as "empty" during cleanup can have new transactions added before
// the actual removal, causing valid newly-added transactions to be deleted.
func TestTxCache_TOCTOU_RaceCondition_EmptySenderRemoval(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-race",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                1000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 1,
		CleanupInterval:               -1, // Disable periodic cleanup for test
	})
	require.NoError(t, err)
	defer cache.Close()

	sender := []byte("race-sender")

	// Counter to track if new transactions were lost
	var newTxLost atomic.Bool
	var wg sync.WaitGroup

	// Add an expired transaction that will be cleaned up
	expiredTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
		TxHash:   []byte("expired-tx"),
		Size:     100,
		ExpireOn: time.Now().Add(-1 * time.Minute).Unix(), // Already expired
	}
	ok, added := cache.AddTx(expiredTx)
	require.True(t, ok)
	require.True(t, added)

	// Verify the expired transaction exists
	_, exists := cache.GetByTxHash([]byte("expired-tx"))
	require.True(t, exists)

	// Hook into the cleanup process to inject a new transaction at the critical moment
	// We'll use a goroutine that continuously tries to add transactions during cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait a tiny bit to let cleanup start
		time.Sleep(1 * time.Millisecond)

		// Try to add new transactions repeatedly during the cleanup window
		for i := 0; i < 100; i++ {
			newTx := &WrappedTransaction{
				Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(100 + i)}},
				TxHash:   []byte("new-tx"),
				Size:     100,
				ExpireOn: time.Now().Add(5 * time.Minute).Unix(), // Valid for 5 minutes
			}

			// Try to add the new transaction
			ok, added := cache.AddTx(newTx)
			if ok && added {
				// Small delay to check if it stays
				time.Sleep(1 * time.Millisecond)

				// Check if the new transaction still exists after cleanup
				_, exists := cache.GetByTxHash([]byte("new-tx"))
				if !exists {
					newTxLost.Store(true)
					return
				}

				// Clean up for next iteration
				cache.RemoveTxByHash([]byte("new-tx"))
			}

			time.Sleep(100 * time.Microsecond)
		}
	}()

	// Run cleanup which should remove expired transactions
	// but might also remove the newly added valid transaction
	cache.cleanExpiredTransactions()

	wg.Wait()

	// Check results
	_, expiredExists := cache.GetByTxHash([]byte("expired-tx"))
	require.False(t, expiredExists, "Expired transaction should be removed")

	// Check if we detected the race condition
	if newTxLost.Load() {
		t.Error("TOCTOU Race Condition Detected: A newly added valid transaction was deleted during cleanup of an 'empty' sender")
	}
}

// TestTxCache_TOCTOU_RaceCondition_ConcurrentOperations runs a more aggressive test
// with multiple goroutines adding transactions while cleanup is happening
func TestTxCache_TOCTOU_RaceCondition_ConcurrentOperations(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-concurrent-race",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                10000,
		CountPerSenderThreshold:       1000,
		NumSendersToPreemptivelyEvict: 1,
		CleanupInterval:               -1, // Disable periodic cleanup for test
	})
	require.NoError(t, err)
	defer cache.Close()

	numSenders := 10
	var lostTransactions atomic.Int32
	var wg sync.WaitGroup

	// Create senders with expired transactions
	for i := 0; i < numSenders; i++ {
		sender := []byte(fmt.Sprintf("sender-%d", i))

		// Add an expired transaction for each sender
		expiredTx := &WrappedTransaction{
			Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
			TxHash:   []byte(fmt.Sprintf("expired-%d", i)),
			Size:     100,
			ExpireOn: time.Now().Add(-1 * time.Minute).Unix(),
		}
		cache.AddTx(expiredTx)
	}

	// Start multiple goroutines that add new transactions
	for i := 0; i < numSenders; i++ {
		wg.Add(1)
		go func(senderIdx int) {
			defer wg.Done()
			sender := []byte(fmt.Sprintf("sender-%d", senderIdx))

			// Continuously add new transactions during cleanup
			for j := 0; j < 50; j++ {
				txHash := []byte(fmt.Sprintf("new-tx-%d-%d", senderIdx, j))
				newTx := &WrappedTransaction{
					Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(100 + j)}},
					TxHash:   txHash,
					Size:     100,
					ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
				}

				ok, added := cache.AddTx(newTx)
				if ok && added {
					// Small delay to let cleanup potentially interfere
					time.Sleep(100 * time.Microsecond)

					// Check if transaction still exists
					_, exists := cache.GetByTxHash(txHash)
					if !exists {
						lostTransactions.Add(1)
					}
				}

				time.Sleep(200 * time.Microsecond)
			}
		}(i)
	}

	// Run cleanup multiple times while transactions are being added
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Millisecond)
		cache.cleanExpiredTransactions()
	}

	wg.Wait()

	// Report findings
	lost := lostTransactions.Load()
	if lost > 0 {
		t.Errorf("TOCTOU Race Condition: %d valid transactions were lost due to race condition in empty sender removal", lost)
	}

	// Verify all expired transactions are gone
	for i := 0; i < numSenders; i++ {
		_, exists := cache.GetByTxHash([]byte(fmt.Sprintf("expired-%d", i)))
		require.False(t, exists, "Expired transaction should be removed")
	}
}

// TestTxCache_TOCTOU_RaceCondition_DelayedAddition tests the race condition
// where a transaction is added to a sender during the cleanup window
func TestTxCache_TOCTOU_RaceCondition_DelayedAddition(t *testing.T) {
	// This test simulates adding a transaction after cleanup has identified
	// a sender as empty but before it's actually removed

	cache, err := NewTxCache(Config{
		Name:                          "test-delayed-add",
		NumChunks:                     16,
		EvictionEnabled:               false,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                1000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 1,
		CleanupInterval:               -1, // Disable periodic cleanup for test
	})
	require.NoError(t, err)
	defer cache.Close()

	sender := []byte("delayed-sender")

	// Add an expired transaction
	expiredTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
		TxHash:   []byte("expired-timed-tx"),
		Size:     100,
		ExpireOn: time.Now().Add(-1 * time.Minute).Unix(),
	}
	cache.AddTx(expiredTx)

	var wg sync.WaitGroup
	var txAddedDuringCleanup atomic.Bool
	var txLostAfterCleanup atomic.Bool

	// Goroutine to add a new transaction at the critical moment
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for cleanup to process the sender and mark it as empty
		// This simulates the window between processExpiredForSender and performBulkRemoval
		time.Sleep(5 * time.Millisecond)

		// Add a new valid transaction for the "empty" sender
		newTx := &WrappedTransaction{
			Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 100}},
			TxHash:   []byte("new-timed-tx"),
			Size:     100,
			ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
		}

		ok, added := cache.AddTx(newTx)
		if ok && added {
			txAddedDuringCleanup.Store(true)

			// Wait for cleanup to finish
			time.Sleep(10 * time.Millisecond)

			// Check if our new transaction survived
			_, exists := cache.GetByTxHash([]byte("new-timed-tx"))
			if !exists {
				txLostAfterCleanup.Store(true)
			}
		}
	}()

	// Run cleanup
	cache.cleanExpiredTransactions()

	wg.Wait()

	// Analyze results
	if txAddedDuringCleanup.Load() && txLostAfterCleanup.Load() {
		t.Error("TOCTOU Race Condition Confirmed: New valid transaction was added during cleanup but was deleted when the 'empty' sender was removed")
	}

	// The expired transaction should definitely be gone
	_, exists := cache.GetByTxHash([]byte("expired-timed-tx"))
	require.False(t, exists, "Expired transaction should be removed")
}
