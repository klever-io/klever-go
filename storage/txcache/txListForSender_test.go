package txcache

import (
	"math"
	"testing"

	"github.com/klever-io/klever-go/common/mock/txfeesmocks"
	"github.com/stretchr/testify/require"
)

func TestListForSender_AddTx_Sorts(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	list.AddTx(createTxWithDelay([]byte("a"), ".", 1, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("c"), ".", 3, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("d"), ".", 4, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("b"), ".", 2, 1), txFeeHelper)

	require.Equal(t, []string{"a", "b", "c", "d"}, list.getTxHashesAsStrings())
}

func TestListForSender_AddTx_GivesPriorityToHigherFees(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	list.AddTx(createTxWithParams([]byte("a"), ".", 1, 128, 42), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("b"), ".", 3, 128, 100), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("c"), ".", 3, 128, 99), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("d"), ".", 2, 128, 42), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("e"), ".", 3, 128, 101), txFeeHelper)

	require.Equal(t, []string{"a", "d", "e", "b", "c"}, list.getTxHashesAsStrings())
}

func TestListForSender_AddTx_SortsCorrectlyWhenSameNonceSamePrice(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	list.AddTx(createTxWithParams([]byte("a"), ".", 1, 128, 42), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("b"), ".", 3, 128, 100), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("c"), ".", 3, 128, 100), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("d"), ".", 3, 128, 98), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("e"), ".", 3, 128, 101), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("f"), ".", 2, 128, 42), txFeeHelper)
	list.AddTx(createTxWithParams([]byte("g"), ".", 3, 128, 99), txFeeHelper)

	// In case of same-nonce, same-price transactions, the newer one has priority
	require.Equal(t, []string{"a", "f", "e", "c", "b", "g", "d"}, list.getTxHashesAsStrings())
}

func TestListForSender_AddTx_IgnoresDuplicates(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	added, _ := list.AddTx(createTx([]byte("tx1"), ".", 1, 1), txFeeHelper)
	require.True(t, added)
	added, _ = list.AddTx(createTx([]byte("tx2"), ".", 2, 1), txFeeHelper)
	require.True(t, added)
	added, _ = list.AddTx(createTx([]byte("tx3"), ".", 3, 1), txFeeHelper)
	require.True(t, added)
	//Transaction is not a duplicate anymore because of timestamp
	added, _ = list.AddTx(createTx([]byte("tx2"), ".", 2, 1), txFeeHelper)
	require.False(t, added)
}

func TestListForSender_AddTx_AppliesSizeConstraintsForNumTransactions(t *testing.T) {
	list := newListToTest(math.MaxUint32, 3)
	_, txFeeHelper := dummyParams()

	list.AddTx(createTxWithDelay([]byte("tx1"), ".", 1, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("tx5"), ".", 5, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("tx4"), ".", 4, 1), txFeeHelper)
	list.AddTx(createTxWithDelay([]byte("tx2"), ".", 2, 1), txFeeHelper)
	require.Equal(t, []string{}, list.getTxHashesAsStrings())

	_, evicted := list.AddTx(createTxWithDelay([]byte("tx3"), ".", 3, 1), txFeeHelper)
	require.Equal(t, []string{"tx3"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))

	// Gives priority to higher gas - though undesirably to some extent, "tx3" is evicted
	_, evicted = list.AddTx(createTxWithParams([]byte("tx2++"), ".", 2, 128, 42), txFeeHelper)
	require.Equal(t, []string{"tx2++", "tx3"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))

	// Though Undesirably to some extent, "tx3++"" is added, then evicted
	_, evicted = list.AddTx(createTxWithParamsAndDelay([]byte("tx3++"), ".", 3, 128, 42), txFeeHelper)
	require.Equal(t, []string{"tx2++", "tx3++", "tx3"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))
}

func TestListForSender_AddTx_AppliesSizeConstraintsForNumBytes(t *testing.T) {
	list := newListToTest(1024, math.MaxUint32)
	_, txFeeHelper := dummyParams()

	list.AddTx(createTxWithParamsAndDelay([]byte("tx1"), ".", 1, 128, 42), txFeeHelper)
	list.AddTx(createTxWithParamsAndDelay([]byte("tx2"), ".", 2, 512, 42), txFeeHelper)
	list.AddTx(createTxWithParamsAndDelay([]byte("tx3"), ".", 3, 256, 42), txFeeHelper)

	_, evicted := list.AddTx(createTxWithParamsAndDelay([]byte("tx5"), ".", 4, 256, 42), txFeeHelper)
	require.Equal(t, []string{}, list.getTxHashesAsStrings())
	require.Equal(t, []string{"tx1", "tx2", "tx3", "tx5"}, hashesAsStrings(evicted))

	_, evicted = list.AddTx(createTxWithParamsAndDelay([]byte("tx5--"), ".", 4, 128, 42), txFeeHelper)
	require.Equal(t, []string{"tx5--"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))

	_, evicted = list.AddTx(createTxWithParamsAndDelay([]byte("tx4"), ".", 4, 128, 42), txFeeHelper)
	require.Equal(t, []string{"tx4", "tx5--"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))

	// Gives priority to higher gas - though undesirably to some extent, "tx4" is evicted
	_, evicted = list.AddTx(createTxWithParams([]byte("tx3++"), ".", 3, 256, 100), txFeeHelper)
	require.Equal(t, []string{"tx3++", "tx4", "tx5--"}, list.getTxHashesAsStrings())
	require.Equal(t, []string{}, hashesAsStrings(evicted))

}

func TestListForSender_findTx(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	txA := createTx([]byte("A"), ".", 41, 1)
	txANewer := createTx([]byte("ANewer"), ".", 41, 1)
	txB := createTx([]byte("B"), ".", 42, 1)
	txD := createTx([]byte("none"), ".", 43, 1)
	list.AddTx(txA, txFeeHelper)
	list.AddTx(txANewer, txFeeHelper)
	list.AddTx(txB, txFeeHelper)

	elementWithA := list.findListElementWithTx(txA)
	elementWithANewer := list.findListElementWithTx(txANewer)
	elementWithB := list.findListElementWithTx(txB)
	noElementWithD := list.findListElementWithTx(txD)

	require.NotNil(t, elementWithA)
	require.NotNil(t, elementWithANewer)
	require.NotNil(t, elementWithB)

	require.Equal(t, txA, elementWithA.Value.(*WrappedTransaction))
	require.Equal(t, txANewer, elementWithANewer.Value.(*WrappedTransaction))
	require.Equal(t, txB, elementWithB.Value.(*WrappedTransaction))
	require.Nil(t, noElementWithD)
}

func TestListForSender_findTx_CoverNonceComparisonOptimization(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()
	list.AddTx(createTx([]byte("A"), ".", 42, 1), txFeeHelper)

	// Find one with a lower nonce, not added to cache
	noElement := list.findListElementWithTx(createTx(nil, ".", 41, 1))
	require.Nil(t, noElement)
}

func TestListForSender_RemoveTransaction(t *testing.T) {
	list := newUnconstrainedListToTest()
	tx := createTx([]byte("a"), ".", 1, 1)
	_, txFeeHelper := dummyParams()

	list.AddTx(tx, txFeeHelper)
	require.Equal(t, 1, list.items.Len())

	list.RemoveTx(tx)
	require.Equal(t, 0, list.items.Len())
}

func TestListForSender_RemoveTransaction_NoPanicWhenTxMissing(t *testing.T) {
	list := newUnconstrainedListToTest()
	tx := createTx([]byte(""), ".", 1, 1)

	list.RemoveTx(tx)
	require.Equal(t, 0, list.items.Len())
}

func TestListForSender_SelectBatchTo(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	for index := 0; index < 100; index++ {
		list.AddTx(createTx([]byte{byte(index)}, ".", uint64(index), 1), txFeeHelper)
	}

	destination := make([]*WrappedTransaction, 1000)

	// First batch
	journal := list.selectBatchTo(true, destination, 50)
	require.Equal(t, 50, journal.copied)
	require.NotNil(t, destination[49])
	require.Nil(t, destination[50])

	// Second batch
	journal = list.selectBatchTo(false, destination[50:], 50)
	require.Equal(t, 50, journal.copied)
	require.NotNil(t, destination[99])

	// No third batch
	journal = list.selectBatchTo(false, destination, 50)
	require.Equal(t, 0, journal.copied)

	// Restart copy
	journal = list.selectBatchTo(true, destination, 12345)
	require.Equal(t, 100, journal.copied)
}

func TestListForSender_SelectBatchTo_NoPanicWhenCornerCases(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	for index := 0; index < 100; index++ {
		list.AddTx(createTx([]byte{byte(index)}, ".", uint64(index), 1), txFeeHelper)
	}

	// When empty destination
	destination := make([]*WrappedTransaction, 0)
	journal := list.selectBatchTo(true, destination, 10)
	require.Equal(t, 0, journal.copied)

	// When small destination
	destination = make([]*WrappedTransaction, 5)
	journal = list.selectBatchTo(false, destination, 10)
	require.Equal(t, 5, journal.copied)
}

func TestListForSender_SelectBatchTo_WhenInitialGap(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()
	list.notifyAccountNonce(1)

	for index := 10; index < 20; index++ {
		list.AddTx(createTx([]byte{byte(index)}, ".", uint64(index), 1_500_000), txFeeHelper)
	}

	destination := make([]*WrappedTransaction, 1000)

	// First batch of selection, first failure
	journal := list.selectBatchTo(true, destination, 50)
	require.Equal(t, 0, journal.copied)
	require.Nil(t, destination[0])
	require.Equal(t, int64(1), list.numFailedSelections.Get())

	// Second batch of selection, don't count failure again
	journal = list.selectBatchTo(false, destination, 50)
	require.Equal(t, 0, journal.copied)
	require.Nil(t, destination[0])
	require.Equal(t, int64(1), list.numFailedSelections.Get())

	// First batch of another selection, second failure, enters grace period
	journal = list.selectBatchTo(true, destination, 50)
	require.Equal(t, 1, journal.copied)
	require.NotNil(t, destination[0])
	require.Nil(t, destination[1])
	require.Equal(t, int64(2), list.numFailedSelections.Get())
}

func TestListForSender_SelectBatchTo_WhenGracePeriodWithGapResolve(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()
	list.notifyAccountNonce(1)

	for index := 2; index < 20; index++ {
		list.AddTx(createTx([]byte{byte(index)}, ".", uint64(index), 1_500_000), txFeeHelper)
	}

	destination := make([]*WrappedTransaction, 1000)

	// Try a number of selections with failure, reach close to grace period
	for i := 1; i < senderGracePeriodLowerBound; i++ {
		journal := list.selectBatchTo(true, destination, math.MaxInt32)
		require.Equal(t, 0, journal.copied)
		require.Equal(t, int64(i), list.numFailedSelections.Get())
	}

	// Try selection again. Failure will move the sender to grace period and return 1 transaction
	journal := list.selectBatchTo(true, destination, math.MaxInt32)
	require.Equal(t, 1, journal.copied)
	require.Equal(t, int64(senderGracePeriodLowerBound), list.numFailedSelections.Get())
	require.False(t, list.sweepable.IsSet())

	// Now resolve the gap
	list.AddTx(createTx([]byte("resolving-tx"), ".", 1, 1_500_000), txFeeHelper)
	// Selection will be successful
	journal = list.selectBatchTo(true, destination, math.MaxInt32)
	require.Equal(t, 19, journal.copied)
	require.Equal(t, int64(0), list.numFailedSelections.Get())
	require.False(t, list.sweepable.IsSet())
}

func TestListForSender_SelectBatchTo_WhenGracePeriodWithNoGapResolve(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()
	list.notifyAccountNonce(1)

	for index := 2; index < 20; index++ {
		list.AddTx(createTx([]byte{byte(index)}, ".", uint64(index), 1_500_000), txFeeHelper)
	}

	destination := make([]*WrappedTransaction, 1000)

	// Try a number of selections with failure, reach close to grace period
	for i := 1; i < senderGracePeriodLowerBound; i++ {
		journal := list.selectBatchTo(true, destination, math.MaxInt32)
		require.Equal(t, 0, journal.copied)
		require.Equal(t, int64(i), list.numFailedSelections.Get())
	}

	// Try a number of selections with failure, within the grace period
	for i := senderGracePeriodLowerBound; i <= senderGracePeriodUpperBound; i++ {
		journal := list.selectBatchTo(true, destination, math.MaxInt32)
		require.Equal(t, 1, journal.copied)
		require.Equal(t, int64(i), list.numFailedSelections.Get())
	}

	// Grace period exceeded now
	journal := list.selectBatchTo(true, destination, math.MaxInt32)
	require.Equal(t, 0, journal.copied)
	require.Equal(t, int64(senderGracePeriodUpperBound+1), list.numFailedSelections.Get())
	require.True(t, list.sweepable.IsSet())
}

func TestListForSender_NotifyAccountNonce(t *testing.T) {
	list := newUnconstrainedListToTest()

	require.Equal(t, uint64(0), list.accountNonce.Get())
	require.False(t, list.accountNonceKnown.IsSet())

	list.notifyAccountNonce(42)

	require.Equal(t, uint64(42), list.accountNonce.Get())
	require.True(t, list.accountNonceKnown.IsSet())
}

func TestListForSender_hasInitialGap(t *testing.T) {
	list := newUnconstrainedListToTest()
	list.notifyAccountNonce(42)
	_, txFeeHelper := dummyParams()

	// No transaction, no gap
	require.False(t, list.hasInitialGap())
	// One gap
	list.AddTx(createTx([]byte("tx-43"), ".", 43, 1_500_000), txFeeHelper)
	require.True(t, list.hasInitialGap())
	// Resolve gap
	list.AddTx(createTx([]byte("tx-42"), ".", 42, 1_500_000), txFeeHelper)
	require.False(t, list.hasInitialGap())
}

func TestListForSender_getTxHashes(t *testing.T) {
	list := newUnconstrainedListToTest()
	require.Len(t, list.getTxHashes(), 0)
	_, txFeeHelper := dummyParams()

	list.AddTx(createTx([]byte("A"), ".", 1, 1), txFeeHelper)
	require.Len(t, list.getTxHashes(), 1)

	list.AddTx(createTx([]byte("B"), ".", 2, 1), txFeeHelper)
	list.AddTx(createTx([]byte("C"), ".", 3, 1), txFeeHelper)
	require.Len(t, list.getTxHashes(), 3)
}

func TestListForSender_DetectRaceConditions(t *testing.T) {
	list := newUnconstrainedListToTest()
	_, txFeeHelper := dummyParams()

	go func() {
		// These are called concurrently with addition: during eviction, during removal etc.
		approximatelyCountTxInLists([]*txListForSender{list})
		list.IsEmpty()
	}()

	go func() {
		list.AddTx(createTx([]byte("test"), ".", 42, 1), txFeeHelper)
	}()
}

func dummyParamsWithGasPrice(minGasPrice uint64) (TxFeesHandler, feeHelper) {
	txFeesHandler := &txfeesmocks.TxFeesHandlerMock{}
	txFeeHelper := newFeeComputationHelper()
	return txFeesHandler, txFeeHelper
}

func dummyParams() (TxFeesHandler, feeHelper) {
	minPrice := uint64(1000000000)
	return dummyParamsWithGasPrice(minPrice)
}

func newUnconstrainedListToTest() *txListForSender {
	return newTxListForSender(".", &senderConstraints{
		maxNumBytes: math.MaxUint32,
		maxNumTxs:   math.MaxUint32,
	}, func(_ *txListForSender, _ senderScoreParams) {})
}

func newListToTest(maxNumBytes uint32, maxNumTxs uint32) *txListForSender {
	return newTxListForSender(".", &senderConstraints{
		maxNumBytes: maxNumBytes,
		maxNumTxs:   maxNumTxs,
	}, func(_ *txListForSender, _ senderScoreParams) {})
}
