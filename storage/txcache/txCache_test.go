package txcache

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"

	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func Test_NewTxCache(t *testing.T) {
	config := Config{
		Name:                       "test",
		NumChunks:                  16,
		NumBytesPerSenderThreshold: maxNumBytesPerSenderUpperBound,
		CountPerSenderThreshold:    math.MaxUint32,
	}

	withEvictionConfig := Config{
		Name:                          "test",
		NumChunks:                     16,
		NumBytesPerSenderThreshold:    maxNumBytesPerSenderUpperBound,
		CountPerSenderThreshold:       math.MaxUint32,
		EvictionEnabled:               true,
		NumBytesThreshold:             maxNumBytesUpperBound,
		CountThreshold:                math.MaxUint32,
		NumSendersToPreemptivelyEvict: 100,
	}

	cache, err := NewTxCache(config)
	require.Nil(t, err)
	require.NotNil(t, cache)

	badConfig := config
	badConfig.Name = ""
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.Name")

	badConfig = config
	badConfig.NumChunks = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.NumChunks")

	badConfig = config
	badConfig.NumBytesPerSenderThreshold = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.NumBytesPerSenderThreshold")

	badConfig = config
	badConfig.CountPerSenderThreshold = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.CountPerSenderThreshold")

	badConfig = withEvictionConfig
	badConfig.NumBytesThreshold = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.NumBytesThreshold")

	badConfig = withEvictionConfig
	badConfig.CountThreshold = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.CountThreshold")

	badConfig = withEvictionConfig
	badConfig.NumSendersToPreemptivelyEvict = 0
	requireErrorOnNewTxCache(t, badConfig, storage.ErrInvalidConfig, "config.NumSendersToPreemptivelyEvict")
}

func requireErrorOnNewTxCache(t *testing.T, config Config, errExpected error, errPartialMessage string) {
	cache, errReceived := NewTxCache(config)
	require.Nil(t, cache)
	require.True(t, errors.Is(errReceived, errExpected))
	require.Contains(t, errReceived.Error(), errPartialMessage)
}

func Test_AddTx(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	tx := createTx([]byte("hash-1"), "alice", 1, 1)

	ok, added := cache.AddTx(tx)
	require.True(t, ok)
	require.True(t, added)
	require.True(t, cache.Has([]byte("hash-1")))

	// Add it again (no-operation)
	ok, added = cache.AddTx(tx)
	require.True(t, ok)
	require.False(t, added)
	require.True(t, cache.Has([]byte("hash-1")))

	foundTx, ok := cache.GetByTxHash([]byte("hash-1"))
	require.True(t, ok)
	require.Equal(t, tx, foundTx)
}

func Test_AddNilTx_DoesNothing(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	txHash := []byte("hash-1")

	ok, added := cache.AddTx(&WrappedTransaction{Tx: nil, TxHash: txHash})
	require.False(t, ok)
	require.False(t, added)

	foundTx, ok := cache.GetByTxHash(txHash)
	require.False(t, ok)
	require.Nil(t, foundTx)
}

func Test_AddTx_AppliesSizeConstraintsPerSenderForNumBytes(t *testing.T) {
	cache := newCacheToTest(1024, math.MaxUint32)

	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-alice-1"), "alice", 1, 128, 42))
	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-alice-2"), "alice", 2, 512, 42))
	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-alice-4"), "alice", 3, 256, 42))
	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-bob-1"), "bob", 1, 512, 42))
	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-bob-2"), "bob", 2, 513, 42))

	require.Equal(t, []string{"tx-alice-1", "tx-alice-2", "tx-alice-4"}, cache.getHashesForSender("alice"))
	require.Equal(t, []string{}, cache.getHashesForSender("bob"))
	require.True(t, cache.areInternalMapsConsistent())

	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-alice-3"), "alice", 3, 256, 42))
	cache.AddTx(createTxWithParamsAndDelay([]byte("tx-bob-2"), "bob", 3, 512, 42))
	require.Equal(t, []string{}, cache.getHashesForSender("alice"))
	require.Equal(t, []string{"tx-bob-2"}, cache.getHashesForSender("bob"))
	require.True(t, cache.areInternalMapsConsistent())
}

func Test_RemoveByTxHash(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("hash-1"), "alice", 1, 1))
	cache.AddTx(createTx([]byte("hash-2"), "alice", 2, 1))

	_, removed := cache.RemoveTxByHash([]byte("hash-1"))
	require.True(t, removed)
	cache.Remove([]byte("hash-2"))

	foundTx, ok := cache.GetByTxHash([]byte("hash-1"))
	require.False(t, ok)
	require.Nil(t, foundTx)

	foundTx, ok = cache.GetByTxHash([]byte("hash-2"))
	require.False(t, ok)
	require.Nil(t, foundTx)
}

func Test_CountTx_And_Len(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("hash-1"), "alice", 1, 1))
	cache.AddTx(createTx([]byte("hash-2"), "alice", 2, 1))
	cache.AddTx(createTx([]byte("hash-3"), "alice", 3, 1))

	require.Equal(t, uint64(3), cache.CountTx())
	require.Equal(t, 3, cache.Len())
}

func Test_GetByTxHash_And_Peek_And_Get(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	txHash := []byte("hash-1")
	tx := createTx(txHash, "alice", 1, 1)
	cache.AddTx(tx)

	foundTx, ok := cache.GetByTxHash(txHash)
	require.True(t, ok)
	require.Equal(t, tx, foundTx)

	foundTxPeek, okPeek := cache.Peek(txHash)
	require.True(t, okPeek)
	require.Equal(t, tx.Tx, foundTxPeek)

	foundTxPeek, okPeek = cache.Peek([]byte("missing"))
	require.False(t, okPeek)
	require.Nil(t, foundTxPeek)

	foundTxGet, okGet := cache.Get(txHash)
	require.True(t, okGet)
	require.Equal(t, tx.Tx, foundTxGet)

	foundTxGet, okGet = cache.Get([]byte("missing"))
	require.False(t, okGet)
	require.Nil(t, foundTxGet)
}

func Test_RemoveByTxHash_WhenMissing(t *testing.T) {
	cache := newUnconstrainedCacheToTest()
	_, removed := cache.RemoveTxByHash([]byte("missing"))
	require.False(t, removed)
}

func Test_RemoveByTxHash_RemovesFromByHash_WhenMapsInconsistency(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	txHash := []byte("hash-1")
	tx := createTx(txHash, "alice", 1, 1)
	cache.AddTx(tx)

	// Cause an inconsistency between the two internal maps (theoretically possible in case of misbehaving eviction)
	cache.txListBySender.removeTx(tx)

	_, _ = cache.RemoveTxByHash(txHash)
	require.Equal(t, 0, cache.txByHash.backingMap.Count())
}

func Test_Clear(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("hash-alice-1"), "alice", 1, 1))
	cache.AddTx(createTx([]byte("hash-bob-7"), "bob", 7, 1))
	cache.AddTx(createTx([]byte("hash-alice-42"), "alice", 42, 1))
	require.Equal(t, uint64(3), cache.CountTx())

	cache.Clear()
	require.Equal(t, uint64(0), cache.CountTx())
}

func Test_ForEachTransaction(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("hash-alice-1"), "alice", 1, 1))
	cache.AddTx(createTx([]byte("hash-bob-7"), "bob", 7, 1))

	counter := 0
	cache.ForEachTransaction(func(txHash []byte, value *WrappedTransaction) {
		counter++
	})
	require.Equal(t, 2, counter)
}

func Test_SelectTransactions_Dummy(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("hash-alice-4"), "alice", 4, 1))
	cache.AddTx(createTx([]byte("hash-alice-3"), "alice", 3, 1))
	cache.AddTx(createTx([]byte("hash-alice-2"), "alice", 2, 1))
	cache.AddTx(createTx([]byte("hash-alice-1"), "alice", 1, 1))
	cache.AddTx(createTx([]byte("hash-bob-7"), "bob", 7, 1))
	cache.AddTx(createTx([]byte("hash-bob-6"), "bob", 6, 1))
	cache.AddTx(createTx([]byte("hash-bob-5"), "bob", 5, 1))
	cache.AddTx(createTx([]byte("hash-carol-1"), "carol", 1, 1))

	sorted := cache.SelectTransactions(10, 2)
	require.Len(t, sorted, 8)
}

func Test_SelectTransactions(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	// Add "nSenders" * "nTransactionsPerSender" transactions in the cache (in reversed nonce order)
	nSenders := 1000
	nTransactionsPerSender := 100
	nTotalTransactions := nSenders * nTransactionsPerSender
	nRequestedTransactions := math.MaxInt16

	for senderTag := 0; senderTag < nSenders; senderTag++ {
		sender := fmt.Sprintf("sender:%d", senderTag)

		for txNonce := nTransactionsPerSender; txNonce > 0; txNonce-- {
			txHash := fmt.Sprintf("hash:%d:%d", senderTag, txNonce)
			tx := createTx([]byte(txHash), sender, uint64(txNonce), int64(txNonce))
			cache.AddTx(tx)
		}
	}

	require.Equal(t, uint64(nTotalTransactions), cache.CountTx())

	sorted := cache.SelectTransactions(nRequestedTransactions, 2)

	require.Len(t, sorted, tools.MinInt(nRequestedTransactions, nTotalTransactions))

	// Check order
	nonces := make(map[string]uint64, nSenders)
	for _, tx := range sorted {
		nonce := tx.Tx.GetNonce()
		sender := string(tx.Tx.GetRaw().GetSender())
		previousNonce := nonces[sender]

		require.LessOrEqual(t, previousNonce, nonce)
		nonces[sender] = nonce
	}
}

func Test_Keys(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	cache.AddTx(createTx([]byte("alice-x"), "alice", 42, 1))
	cache.AddTx(createTx([]byte("alice-y"), "alice", 43, 1))
	cache.AddTx(createTx([]byte("bob-x"), "bob", 42, 1))
	cache.AddTx(createTx([]byte("bob-y"), "bob", 43, 1))

	keys := cache.Keys()
	require.Equal(t, 4, len(keys))
	require.Contains(t, keys, []byte("alice-x"))
	require.Contains(t, keys, []byte("alice-y"))
	require.Contains(t, keys, []byte("bob-x"))
	require.Contains(t, keys, []byte("bob-y"))
}

func Test_AddWithEviction_UniformDistributionOfTxsPerSender(t *testing.T) {
	config := Config{
		Name:                          "untitled",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             maxNumBytesUpperBound,
		CountThreshold:                100, // countThreshold +1
		NumSendersToPreemptivelyEvict: 1,
		NumBytesPerSenderThreshold:    maxNumBytesPerSenderUpperBound,
		CountPerSenderThreshold:       math.MaxUint32,
	}

	// 11 * 10
	cache, err := NewTxCache(config)
	require.Nil(t, err)
	require.NotNil(t, cache)

	addManyTransactionsWithUniformDistribution(cache, 11, 10)
	require.LessOrEqual(t, cache.CountTx(), uint64(101))

	config = Config{
		Name:                          "untitled",
		NumChunks:                     16,
		EvictionEnabled:               true,
		NumBytesThreshold:             maxNumBytesUpperBound,
		CountThreshold:                250000,
		NumSendersToPreemptivelyEvict: 1,
		NumBytesPerSenderThreshold:    maxNumBytesPerSenderUpperBound,
		CountPerSenderThreshold:       math.MaxUint32,
	}

	// 100 * 1000
	cache, err = NewTxCache(config)
	require.Nil(t, err)
	require.NotNil(t, cache)

	addManyTransactionsWithUniformDistribution(cache, 100, 1000)
	require.LessOrEqual(t, cache.CountTx(), uint64(250000))
}

func Test_GetBySenderPaginated_NewSender(t *testing.T) {
	sender := []byte("alice")
	cache := newUnconstrainedCacheToTest()

	// new sender must return empty list
	paginated, i := cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 0, i)
	require.Len(t, paginated, 0)
}

func Test_GetBySenderPaginated_CachePreserved(t *testing.T) {
	sender := []byte("alice")
	cache := newUnconstrainedCacheToTest()

	// new sender must return empty list
	paginated, i := cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 0, i)
	require.Len(t, paginated, 0)

	// ensure that the sender is not inserted in cache as a side effect
	_, ok := cache.txListBySender.backingMap.Get(string(sender))
	require.False(t, ok)
}

func Test_GetBySenderPaginated_ExistingSender(t *testing.T) {
	sender := []byte("alice")
	cache := newUnconstrainedCacheToTest()

	// if manually added, sender must exist
	_ = cache.txListBySender.getOrAddListForSender(string(sender))
	_, ok := cache.txListBySender.backingMap.Get(string(sender))
	require.True(t, ok)

	// existing sender with no transactions
	paginated, i := cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 0, i)
	require.Len(t, paginated, 0)

	// add transaction
	cache.AddTx(createTx([]byte("hash-1"), string(sender), 1, 1))

	// existing sender with 1 transaction
	paginated, i = cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 1, i)
	require.Len(t, paginated, 1)
}

func Test_GetBySenderPaginated_TxsOnlyFromSender(t *testing.T) {
	sender := []byte("alice")
	sender2 := []byte("not-alice")
	cache := newUnconstrainedCacheToTest()

	// add transaction
	cache.AddTx(createTx([]byte("hash-1"), string(sender), 1, 1))

	// existing sender with 1 transaction
	paginated, i := cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 1, i)
	require.Len(t, paginated, 1)

	// add transaction from another sender
	cache.AddTx(createTx([]byte("hash-2"), string(sender2), 1, 1))
	cache.AddTx(createTx([]byte("hash-3"), string(sender2), 1, 1))
	cache.AddTx(createTx([]byte("hash-4"), string(sender2), 1, 1))
	cache.AddTx(createTx([]byte("hash-5"), string(sender2), 1, 1))

	// transactions exists in cache
	count := len(cache.txByHash.keys())
	require.Equal(t, 5, count)

	// existing sender still returns only 1 transaction
	paginated, i = cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 1, i)
	require.Len(t, paginated, 1)
}

func Test_GetBySenderPaginated_Pagination(t *testing.T) {
	sender := []byte("alice")
	cache := newUnconstrainedCacheToTest()

	// add transactions
	cache.AddTx(createTx([]byte("hash-1"), string(sender), 1, 1))
	cache.AddTx(createTx([]byte("hash-2"), string(sender), 2, 1))
	cache.AddTx(createTx([]byte("hash-3"), string(sender), 3, 1))
	cache.AddTx(createTx([]byte("hash-4"), string(sender), 4, 1))

	// transactions exists in cache
	count := len(cache.txByHash.keys())
	require.Equal(t, 4, count)

	// retrieve all on same page
	paginated, i := cache.GetBySenderPaginated(sender, 0, 10)
	require.Equal(t, 4, i)
	require.Len(t, paginated, 4)

	// paginate
	page01, i := cache.GetBySenderPaginated(sender, 0, 2)
	require.Equal(t, 4, i)
	require.Len(t, page01, 2)

	page02, i := cache.GetBySenderPaginated(sender, 1, 2)
	require.Equal(t, 4, i)
	require.Len(t, page02, 2)

	// out of bounds
	page03, i := cache.GetBySenderPaginated(sender, 2, 2)
	require.Equal(t, 4, i)    // total count must be correct even if out of bounds
	require.Len(t, page03, 0) // beyond bounds returns empty list

	// check if all transactions are returned ordered
	items := append(page01, page02...)
	for i, tx := range items {
		wrapped := tx.(*transaction.Transaction)
		require.Equal(t, uint64(i+1), wrapped.GetNonce())
	}
}

func Test_NotImplementedFunctions(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	evicted := cache.Put(nil, nil, 0)
	require.False(t, evicted)

	has, added := cache.HasOrAdd(nil, nil, 0)
	require.False(t, has)
	require.False(t, added)

	require.NotPanics(t, func() { cache.RegisterHandler(nil, "") })
	require.Zero(t, cache.MaxSize())
}

func Test_IsInterfaceNil(t *testing.T) {
	cache := newUnconstrainedCacheToTest()
	require.False(t, check.IfNil(cache))

	makeNil := func() storage.Cacher {
		return nil
	}

	thisIsNil := makeNil()
	require.True(t, check.IfNil(thisIsNil))
}

func TestTxCache_ConcurrentMutationAndSelection(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	// Alice will quickly move between two score buckets (chunks)
	cheapTransaction := createTxWithParams([]byte("alice-x-o"), "alice", 0, 128, 50000)
	expensiveTransaction := createTxWithParams([]byte("alice-x-1"), "alice", 1, 128, 80000)
	cache.AddTx(cheapTransaction)
	cache.AddTx(expensiveTransaction)

	wg := sync.WaitGroup{}

	// Simulate selection
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			fmt.Println("Selection", i)
			cache.SelectTransactions(100, 100)
		}

		wg.Done()
	}()

	// Simulate add / remove transactions
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			fmt.Println("Add / remove", i)
			cache.Remove([]byte("alice-x-1"))
			cache.AddTx(expensiveTransaction)
		}

		wg.Done()
	}()

	timedOut := waitTimeout(&wg, 1*time.Second)
	require.False(t, timedOut, "Timed out. Perhaps deadlock?")
}

func TestTxCache_TransactionIsAdded_EvenWhenInternalMapsAreInconsistent(t *testing.T) {
	cache := newUnconstrainedCacheToTest()

	// Setup inconsistency: transaction already exists in map by hash, but not in map by sender
	cache.txByHash.addTx(createTx([]byte("alice-x"), "alice", 42, 1))

	require.Equal(t, 1, cache.txByHash.backingMap.Count())
	require.True(t, cache.Has([]byte("alice-x")))
	ok, added := cache.AddTx(createTx([]byte("alice-x"), "alice", 42, 1))
	require.True(t, ok)
	require.True(t, added)
	require.Equal(t, uint64(1), cache.CountSenders())
	require.Equal(t, []string{"alice-x"}, cache.getHashesForSender("alice"))
	cache.Clear()

	// Setup inconsistency: transaction already exists in map by sender, but not in map by hash
	cache.txListBySender.addTx(createTx([]byte("alice-x"), "alice", 42, 1))

	require.False(t, cache.Has([]byte("alice-x")))
	ok, added = cache.AddTx(createTx([]byte("alice-x"), "alice", 42, 1))
	require.True(t, ok)
	require.True(t, added)
	require.Equal(t, uint64(1), cache.CountSenders())
	require.Equal(t, []string{"alice-x"}, cache.getHashesForSender("alice"))
	cache.Clear()
}

func newUnconstrainedCacheToTest() *TxCache {
	cache, err := NewTxCache(Config{
		Name:                       "test",
		NumChunks:                  16,
		NumBytesPerSenderThreshold: maxNumBytesPerSenderUpperBound,
		CountPerSenderThreshold:    math.MaxUint32,
	})
	if err != nil {
		panic(fmt.Sprintf("newUnconstrainedCacheToTest(): %s", err))
	}

	return cache
}

func newCacheToTest(numBytesPerSenderThreshold uint32, countPerSenderThreshold uint32) *TxCache {
	cache, err := NewTxCache(Config{
		Name:                       "test",
		NumChunks:                  16,
		NumBytesPerSenderThreshold: numBytesPerSenderThreshold,
		CountPerSenderThreshold:    countPerSenderThreshold,
	})
	if err != nil {
		panic(fmt.Sprintf("newCacheToTest(): %s", err))
	}

	return cache
}
