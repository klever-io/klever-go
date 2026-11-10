package txcache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// KLC-2808: block commit removes a sender's lower nonces while the expiry cleanup removes the
// same sender's expired ones. Both walk and mutate one linked list; unserialized, the list
// tears, and the commit path panics, spins holding mutTxOperation, or leaves counts behind.
func TestTxCache_CommitRemovalRacesExpiryCleanup(t *testing.T) {
	cache, err := NewTxCache(Config{
		Name:                          "test-commit-expiry-race",
		NumChunks:                     16,
		NumBytesThreshold:             10_000_000,
		NumBytesPerSenderThreshold:    maxNumBytesPerSenderUpperBound,
		CountThreshold:                100_000,
		CountPerSenderThreshold:       100,
		NumSendersToPreemptivelyEvict: 1,
		CleanupInterval:               -1,
	})
	require.NoError(t, err)
	defer cache.Close()

	const (
		iterations = 300
		committed  = 32 // nonces 0..31 are fresh and removed by the commit
		expired    = 32 // nonces 32..63 are expired and removed by the cleanup
	)
	fresh := time.Now().Add(time.Hour).Unix()
	stale := time.Now().Add(-time.Hour).Unix()

	for i := 0; i < iterations; i++ {
		sender := fmt.Sprintf("sender-%d", i)
		for nonce := uint64(0); nonce < committed+expired; nonce++ {
			tx := createTx([]byte(fmt.Sprintf("%s-%d", sender, nonce)), sender, nonce, 1)
			tx.ExpireOn = fresh
			if nonce >= committed {
				tx.ExpireOn = stale
			}
			_, added := cache.AddTx(tx)
			require.True(t, added)
		}
		list := cache.getListForSender(sender)

		var panics atomic.Int32
		var removedByCommit, removedByCleanup int
		start := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer func() {
				if recover() != nil {
					panics.Add(1)
				}
			}()
			<-start
			removedByCommit = cache.RemoveTxBySenderNonce([]byte(sender), committed-1)
		}()
		go func() {
			defer wg.Done()
			defer func() {
				if recover() != nil {
					panics.Add(1)
				}
			}()
			<-start
			removedByCleanup = cache.processExpiredForSender(list, time.Now().Unix()).expiredCount
		}()
		close(start)

		require.False(t, waitTimeout(&wg, 5*time.Second), "iteration %d hung", i)
		require.Zero(t, panics.Load(), "iteration %d panicked", i)
		require.Equal(t, committed, removedByCommit, "iteration %d", i)
		require.Equal(t, expired, removedByCleanup, "iteration %d", i)
		require.Zero(t, list.items.Len(), "iteration %d", i)
		require.Zero(t, list.totalBytes.GetUint64(), "iteration %d", i)
	}
}
