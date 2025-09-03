package txcache

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestTxCache_CleanExpiredTransactions(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                100,
		CountPerSenderThreshold:       10,
		NumSendersToPreemptivelyEvict: 1,
	})
	require.NoError(t, err)
	defer cache.Close()

	// Add transactions with different expiration times
	sender1 := []byte("sender1")
	sender2 := []byte("sender2")

	// Add expired transaction
	expiredTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender1, Nonce: 1}},
		TxHash:   []byte("expired-tx-1"),
		Size:     100,
		ExpireOn: time.Now().Add(-2 * time.Minute).Unix(), // Expired 2 minutes ago
	}
	ok, added := cache.AddTx(expiredTx)
	require.True(t, ok)
	require.True(t, added)

	// Add valid transaction for same sender
	validTx1 := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender1, Nonce: 2}},
		TxHash:   []byte("valid-tx-1"),
		Size:     100,
		ExpireOn: time.Now().Add(5 * time.Minute).Unix(), // Expires in 5 minutes
	}
	ok, added = cache.AddTx(validTx1)
	require.True(t, ok)
	require.True(t, added)

	// Add another expired transaction for different sender
	expiredTx2 := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender2, Nonce: 1}},
		TxHash:   []byte("expired-tx-2"),
		Size:     100,
		ExpireOn: time.Now().Add(-1 * time.Minute).Unix(), // Expired 1 minute ago
	}
	ok, added = cache.AddTx(expiredTx2)
	require.True(t, ok)
	require.True(t, added)

	// Verify all transactions are in cache
	require.Equal(t, uint64(3), cache.CountTx())
	require.Equal(t, uint64(2), cache.CountSenders())

	// Clean expired transactions
	cache.cleanExpiredTransactions()

	// Verify only valid transaction remains
	require.Equal(t, uint64(1), cache.CountTx())
	require.Equal(t, uint64(1), cache.CountSenders())

	// Verify the valid transaction is still accessible
	tx, found := cache.GetByTxHash([]byte("valid-tx-1"))
	require.True(t, found)
	require.Equal(t, validTx1.TxHash, tx.TxHash)

	// Verify expired transactions are gone
	_, found = cache.GetByTxHash([]byte("expired-tx-1"))
	require.False(t, found)
	_, found = cache.GetByTxHash([]byte("expired-tx-2"))
	require.False(t, found)
}

func TestTxCache_PeriodicCleanup(t *testing.T) {
	// This test verifies that the periodic cleanup goroutine starts and stops properly
	cache, err := NewTxCache(Config{
		Name:                          "test",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    50000,
		CountThreshold:                100,
		CountPerSenderThreshold:       10,
		NumSendersToPreemptivelyEvict: 1,
	})
	require.NoError(t, err)

	// Verify cleanup ticker was created
	require.NotNil(t, cache.cleanupTicker)
	require.NotNil(t, cache.cleanupStop)

	// Close should stop the cleanup goroutine
	err = cache.Close()
	require.NoError(t, err)

	// Give goroutine time to stop
	time.Sleep(10 * time.Millisecond)

	// Verify we can close multiple times without panic
	err = cache.Close()
	require.NoError(t, err)
}

func TestTxCache_ExpiredTransactionsAreRemovedOnCapacityExceeded(t *testing.T) {
	// This test verifies that expired transactions are removed when capacity is exceeded
	// (existing behavior that only works when sender capacity limits are hit)
	cache, err := NewTxCache(Config{
		Name:                          "test",
		NumChunks:                     16,
		EvictionEnabled:               false, // Disable regular eviction
		NumBytesThreshold:             1000000,
		NumBytesPerSenderThreshold:    150, // Low limit to trigger capacity exceeded
		CountThreshold:                100,
		CountPerSenderThreshold:       2, // Only allow 2 transactions per sender
		NumSendersToPreemptivelyEvict: 1,
	})
	require.NoError(t, err)
	defer cache.Close()

	sender := []byte("sender1")

	// Add expired transaction
	expiredTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 1}},
		TxHash:   []byte("expired-tx"),
		Size:     50,
		ExpireOn: time.Now().Add(-1 * time.Minute).Unix(), // Expired
	}
	ok, added := cache.AddTx(expiredTx)
	require.True(t, ok)
	require.True(t, added)

	// Add valid transaction
	validTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 2}},
		TxHash:   []byte("valid-tx"),
		Size:     50,
		ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
	}
	ok, added = cache.AddTx(validTx)
	require.True(t, ok)
	require.True(t, added)

	// Both transactions should be in cache (capacity not exceeded yet)
	require.Equal(t, uint64(2), cache.CountTx())

	// Add third transaction - should trigger capacity check and remove expired one
	newTx := &WrappedTransaction{
		Tx:       &transaction.Transaction{RawData: &transaction.Transaction_Raw{Sender: sender, Nonce: 3}},
		TxHash:   []byte("new-tx"),
		Size:     50,
		ExpireOn: time.Now().Add(5 * time.Minute).Unix(),
	}
	ok, added = cache.AddTx(newTx)
	require.True(t, ok)
	require.True(t, added)

	// Expired transaction should be removed due to capacity constraints
	require.Equal(t, uint64(2), cache.CountTx())

	_, found := cache.GetByTxHash([]byte("expired-tx"))
	require.False(t, found)

	tx, found := cache.GetByTxHash([]byte("valid-tx"))
	require.True(t, found)
	require.Equal(t, validTx.TxHash, tx.TxHash)

	tx, found = cache.GetByTxHash([]byte("new-tx"))
	require.True(t, found)
	require.Equal(t, newTx.TxHash, tx.TxHash)
}
