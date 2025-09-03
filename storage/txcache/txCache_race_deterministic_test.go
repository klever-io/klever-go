package txcache

import (
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

// TestTxCache_TOCTOU_DeterministicRaceCondition uses synchronization to deterministically
// trigger the TOCTOU race condition
func TestTxCache_TOCTOU_DeterministicRaceCondition(t *testing.T) {
	// Create a custom wrapper to intercept critical points
	cache, err := NewTxCache(Config{
		Name:                          "test-deterministic",
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

	sender := []byte("deterministic-sender")

	// Add only expired transactions for a sender
	for i := 0; i < 3; i++ {
		expiredTx := &WrappedTransaction{
			Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: uint64(i + 1)}},
			TxHash:   []byte{byte(i)},
			Size:     100,
			ExpireOn: time.Now().Add(-1 * time.Minute).Unix(), // Already expired
		}
		ok, added := cache.AddTx(expiredTx)
		require.True(t, ok)
		require.True(t, added)
	}

	// Channel to coordinate timing
	addTxSignal := make(chan struct{})
	cleanupDone := make(chan struct{})

	var wg sync.WaitGroup
	var newTxAdded bool
	var newTxExists bool

	// Goroutine that will add a new transaction at the critical moment
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for signal that cleanup has identified sender as empty
		<-addTxSignal

		// Add a new valid transaction for the "empty" sender
		newTx := &WrappedTransaction{
			Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 100}},
			TxHash:   []byte("new-deterministic-tx"),
			Size:     100,
			ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
		}

		ok, added := cache.AddTx(newTx)
		newTxAdded = ok && added

		// Let cleanup continue
		close(cleanupDone)
	}()

	// Manually execute the cleanup steps to control timing
	currentTime := time.Now().Unix()

	// Step 1: Get snapshot of senders
	senders := cache.txListBySender.getSnapshotAscending()
	require.Len(t, senders, 1)

	// Step 2: Process expired transactions (this marks sender as empty)
	result := cache.processExpiredTransactions(senders, currentTime)
	require.Equal(t, 3, result.totalExpiredTxs)
	require.Equal(t, 1, result.totalExpiredSenders)
	require.Len(t, result.sendersToRemove, 1)

	// Signal the goroutine to add new transaction NOW
	// This is the critical race window - sender is marked for removal
	// but not yet removed
	close(addTxSignal)

	// Wait for new transaction to be added
	<-cleanupDone

	// Step 3: Perform bulk removal (this should remove the sender)
	cache.performBulkRemoval(result)

	wg.Wait()

	// Check if the new transaction exists after cleanup
	_, newTxExists = cache.GetByTxHash([]byte("new-deterministic-tx"))

	// Verify the fix works - new transaction should survive
	if newTxAdded {
		require.True(t, newTxExists, "With the fix, new transaction added during cleanup should survive")
	}

	// Check that expired transactions are gone
	for i := 0; i < 3; i++ {
		_, exists := cache.GetByTxHash([]byte{byte(i)})
		require.False(t, exists, "Expired transaction should be removed")
	}

	// Check sender count - with the fix, sender should remain if new tx was added
	if newTxAdded && newTxExists {
		require.Equal(t, uint64(1), cache.CountSenders(), "Sender should remain after re-validation found new transaction")
	} else {
		require.Equal(t, uint64(0), cache.CountSenders(), "Sender should be removed if truly empty")
	}
}

// TestTxCache_VerifySenderNotRemovedWithValidTx verifies that a sender with valid transactions
// is NOT removed even if it has some expired transactions
func TestTxCache_VerifySenderNotRemovedWithValidTx(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-mixed",
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

	sender := []byte("mixed-sender")

	// Add expired transaction
	expiredTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
		TxHash:   []byte("expired-mixed"),
		Size:     100,
		ExpireOn: time.Now().Add(-1 * time.Minute).Unix(),
	}
	cache.AddTx(expiredTx)

	// Add valid transaction
	validTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 2}},
		TxHash:   []byte("valid-mixed"),
		Size:     100,
		ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
	}
	cache.AddTx(validTx)

	// Run cleanup
	cache.cleanExpiredTransactions()

	// Verify expired is gone but valid remains
	_, expiredExists := cache.GetByTxHash([]byte("expired-mixed"))
	require.False(t, expiredExists, "Expired transaction should be removed")

	_, validExists := cache.GetByTxHash([]byte("valid-mixed"))
	require.True(t, validExists, "Valid transaction should remain")

	// Sender should still exist
	require.Equal(t, uint64(1), cache.CountSenders(), "Sender should remain with valid transaction")
}
