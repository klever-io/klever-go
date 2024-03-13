package txcache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSweeping_SweepSweepable(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("alice-42"), "alice", 42, 123))
	cache.AddTx(createTx([]byte("bob-42"), "bob", 42, 123))
	cache.AddTx(createTx([]byte("carol-42"), "carol", 42, 123))

	// Fake "Alice" and "Bob" as sweepable
	cache.sweepingListOfSenders = []*txListForSender{
		cache.getListForSender("alice"),
		cache.getListForSender("bob"),
	}

	require.Equal(t, uint64(3), cache.CountTx())
	require.Equal(t, uint64(3), cache.CountSenders())

	cache.sweepSweepable()

	require.Equal(t, uint64(1), cache.CountTx())
	require.Equal(t, uint64(1), cache.CountSenders())
}
