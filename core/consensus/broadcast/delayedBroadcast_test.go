package broadcast_test

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/alarm"
	"github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/klever-io/klever-go/core/fallback/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/atomic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baseDelay = 100 * time.Millisecond
const orderDelay = 1 * time.Second

func newDefaultDelayedBroadcasterArgs() *broadcast.ArgsDelayedBlockBroadcaster {
	return &broadcast.ArgsDelayedBlockBroadcaster{
		InterceptorsContainer: &commonMock.InterceptorsContainerStub{},
		HeadersSubscriber:     &mock.HeadersCacherStub{},
		LeaderCacheSize:       2,
		ValidatorCacheSize:    2,
		AlarmScheduler:        alarm.NewAlarmScheduler(),
	}
}

func createBlock(transactions [][]byte) *block.Block {
	block := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 0,
			Slot:  0,
		},
	}

	if transactions != nil || len(transactions) > 0 {
		block.TxHashes = transactions
		block.Header.TxRootHash, _ = block.ComputeRootHash(commonMock.HasherMock{})
	}

	return block
}

func createDelayData(prefix string) ([]byte, [][]byte) {
	return []byte(fmt.Sprintf("%s header hash", prefix)), [][]byte{
		[]byte(fmt.Sprintf("%s tx0", prefix)),
		[]byte(fmt.Sprintf("%s tx1", prefix)),
	}
}

func TestNewDelayedBlockBroadcaster_NilInterceptorsContainerShouldErr(t *testing.T) {
	t.Parallel()

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	delayBroadcastArgs.InterceptorsContainer = nil
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.NotNil(t, err)
	assert.Equal(t, common.ErrNilInterceptorsContainer, err)
	assert.Nil(t, d)
}

func TestNewDelayedBlockBroadcaster_NilHeadersSubscriberShouldErr(t *testing.T) {
	t.Parallel()

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	delayBroadcastArgs.HeadersSubscriber = nil
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.NotNil(t, err)
	assert.Equal(t, common.ErrNilHeadersSubscriber, err)
	assert.Nil(t, d)
}

func TestNewDelayedBlockBroadcaster_NilAlarmSchedulerShouldErr(t *testing.T) {
	t.Parallel()

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	delayBroadcastArgs.AlarmScheduler = nil
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.NotNil(t, err)
	assert.Equal(t, common.ErrNilAlarmScheduler, err)
	assert.Nil(t, d)
}

func TestNewDelayedBlockBroadcaster_ShouldOK(t *testing.T) {
	t.Parallel()

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)
}

func TestDelayedBlockBroadcaster_HeaderReceivedNoDelayedDataRegistered(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := atomic.Flag{}

	broadcastTransactions := func(txData [][]byte) error {
		_ = txBroadcastCalled.Set()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	block := createBlock(nil)

	d.HeaderReceived(block, []byte("block hash"))
	time.Sleep(baseDelay)
	assert.False(t, txBroadcastCalled.IsSet())
}

func TestDelayedBlockBroadcaster_HeaderReceivedForRegisteredDelayedDataShouldBroadcastTheData(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Flag{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Set()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	))
	assert.Nil(t, err)
	time.Sleep(baseDelay)
	assert.False(t, txBroadcastCalled.IsSet())

	d.HeaderReceived(block, headerHash)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)
	assert.True(t, txBroadcastCalled.IsSet())

	vbd := d.GetLeaderBroadcastData()
	assert.Equal(t, 0, len(vbd))
}

func TestDelayedBlockBroadcaster_HeaderReceivedForNotRegisteredDelayedDataShouldNotBroadcast(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Flag{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Set()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	))
	assert.Nil(t, err)
	assert.False(t, txBroadcastCalled.IsSet())

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash[1:],
		block,
		block.GetTxHashes(),
	))
	time.Sleep(baseDelay)
	assert.Nil(t, err)
	assert.False(t, txBroadcastCalled.IsSet())

	otherBlock := createBlock(nil)

	d.HeaderReceived(otherBlock, []byte("other block hash"))
	time.Sleep(baseDelay)
	assert.False(t, txBroadcastCalled.IsSet())
}

func TestDelayedBlockBroadcaster_HeaderReceivedForNextRegisteredDelayedDataShouldBroadcastBoth(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		_ = txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block1 := createBlock(transactions)

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block1,
		block1.GetTxHashes(),
	))
	assert.Nil(t, err)
	time.Sleep(baseDelay)
	assert.Equal(t, txBroadcastCalled.Get(), int64(0))

	headerHash2, transactions2 := createDelayData("2")

	block2 := createBlock(transactions2)

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash2,
		block2,
		block2.GetTxHashes(),
	))
	assert.Nil(t, err)
	time.Sleep(baseDelay)
	assert.Equal(t, int64(0), txBroadcastCalled.Get())

	d.HeaderReceived(block2, headerHash2)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)
	assert.Equal(t, int64(2), txBroadcastCalled.Get())

	vbd := d.GetLeaderBroadcastData()
	assert.Equal(t, 0, len(vbd))
}

func TestDelayedBlockBroadcaster_SetLeaderDataNilDataShouldErr(t *testing.T) {
	t.Parallel()

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetLeaderData(nil)
	require.Equal(t, common.ErrNilParameter, err)
}

func TestDelayedBlockBroadcaster_SetLeaderData(t *testing.T) {
	t.Parallel()

	broadcastTransactions := func(txData [][]byte) error {
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	err = d.SetLeaderData(broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	))
	assert.Nil(t, err)

	vbd := d.GetLeaderBroadcastData()
	assert.Equal(t, 1, len(vbd))
}

func TestDelayedBlockBroadcaster_SetValidatorDataNilDataShouldErr(t *testing.T) {
	t.Parallel()

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetValidatorData(nil)
	require.Equal(t, common.ErrNilParameter, err)

	vld := d.GetLeaderBroadcastData()
	assert.Equal(t, 0, len(vld))
}

func TestDelayedBlockBroadcaster_SetValidatorData(t *testing.T) {
	t.Parallel()

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbb := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbb))
}

func TestDelayedBlockBroadcaster_SetHeaderForValidatorShouldSetAlarmAndBroadcastHeader(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.SetSignature([]byte("signature"))

	err = d.SetHeaderForValidator(
		broadcast.CreateValidatorHeaderBroadcastData(
			headerHash,
			block,
			block.GetTxHashes(),
			1,
		),
	)
	require.Nil(t, err)

	vbb := d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 1, len(vbb))
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)
	require.Equal(t, int64(1), headerBroadcastCalled.Get())
	require.Equal(t, int64(1), txBroadcastCalled.Get())

	vbb = d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 0, len(vbb))
}

func TestDelayedBlockBroadcaster_InterceptedHeaderShouldCancelAlarm(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.SetSignature([]byte("signature"))

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbb := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbb))
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	d.InterceptedHeaderData(block.GetTxRootHash(), block)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	vbb = d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbb))
}

func TestDelayedBlockBroadcaster_InterceptedHeaderShouldCancelAlarmForHeaderBroadcast(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.SetSignature([]byte("signature"))

	err = d.SetHeaderForValidator(broadcast.CreateValidatorHeaderBroadcastData(
		headerHash,
		block,
		block.GetTxHashes(),
		1,
	))
	require.Nil(t, err)

	vbb := d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 1, len(vbb))
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	d.InterceptedHeaderData(block.GetTxRootHash(), block)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	vbb = d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 0, len(vbb))
}

func TestDelayedBlockBroadcaster_InterceptedHeaderInvalidOrDifferentShouldIgnore(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcasterArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcasterArgs)
	require.Nil(t, err)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.SetSignature([]byte("signature"))

	err = d.SetHeaderForValidator(broadcast.CreateValidatorHeaderBroadcastData(
		headerHash,
		block,
		block.GetTxHashes(),
		1,
	))
	require.Nil(t, err)

	vbb := d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 1, len(vbb))
	require.Equal(t, int64(0), headerBroadcastCalled.Get())
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	differentBlock := createBlock(nil)
	differentBlock.Header.TxRootHash = []byte("different")
	differentBlock.Header.Slot = 1

	// should not cancel alarm
	d.InterceptedHeaderData(differentBlock.GetTxRootHash(), differentBlock)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)

	// alarm expired and sent header
	require.Equal(t, int64(1), headerBroadcastCalled.Get())
	require.Equal(t, int64(1), txBroadcastCalled.Get())

	vbb = d.GetValidatorHeaderBroadcastData()
	require.Equal(t, 0, len(vbb))
}

func TestDelayedBlockBroadcaster_SetValidatorDelayBroadcastAccumulatedDataBounded(t *testing.T) {
	t.Parallel()

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	vbd := d.GetValidatorBroadcastData()
	expectedLen := 0
	require.Equal(t, expectedLen, len(vbd))

	for i := 1; i < 100; i++ {
		headerHash, transactions := createDelayData(fmt.Sprintf("%d", i))

		block := createBlock(transactions)

		validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
			headerHash,
			block,
			block.GetTxHashes(),
		)

		err = d.SetValidatorData(validatorDelay)
		require.Nil(t, err)

		vbd = d.GetValidatorBroadcastData()
		expectedLen := i
		if i > int(delayBroadcastArgs.ValidatorCacheSize) {
			expectedLen = int(delayBroadcastArgs.ValidatorCacheSize)
		}
		require.Equal(t, expectedLen, len(vbd))
	}

}

func TestDelayedBlockBroadcaster_ScheduleValidatorBroadcastDifferentHeaderSlotShouldDoNothing(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	dfv := &broadcast.HeaderDataForValidator{
		Slot:         block.GetNonce() + 1,
		PrevRandSeed: block.GetPrevRandSeed(),
	}
	d.ScheduleValidatorBroadcast(dfv)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))
}

func TestDelayedBlockBroadcaster_ScheduleValidatorBroadcastDifferentPrevRandShouldDoNothing(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	differentPrevRandSeed := make([]byte, len(block.GetPrevRandSeed()))
	copy(differentPrevRandSeed, block.GetPrevRandSeed())
	differentPrevRandSeed[0] = ^differentPrevRandSeed[0]
	dfv := &broadcast.HeaderDataForValidator{
		Slot:         block.GetNonce() + 1,
		PrevRandSeed: differentPrevRandSeed,
	}
	d.ScheduleValidatorBroadcast(dfv)
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))
}

func TestDelayedBlockBroadcaster_ScheduleValidatorBroadcastSameSlotAndPrevRandShouldBroadcast(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	dfv := &broadcast.HeaderDataForValidator{
		Slot:         block.GetNonce(),
		PrevRandSeed: block.GetPrevRandSeed(),
	}
	d.ScheduleValidatorBroadcast(dfv)
	time.Sleep(2*common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay) // validatorDelayPerOrder + extra delay + order delay + base delay
	require.Equal(t, int64(1), txBroadcastCalled.Get())

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 0, len(vbd))
}

func TestDelayedBlockBroadcaster_AlarmExpiredShouldBroadcastTheDataForRegisteredDelayedData(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	d.AlarmExpired(hex.EncodeToString(headerHash))
	time.Sleep(common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay)

	require.Equal(t, int64(1), txBroadcastCalled.Get())

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 0, len(vbd))
}

func TestDelayedBlockBroadcaster_AlarmExpiredShouldDoNothingForNotRegisteredData(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	delayedData := broadcast.CreateDelayBroadcastDataForValidator(
		headerHash,
		block,
		block.GetTxHashes(),
		1,
	)
	err = d.SetValidatorData(delayedData)

	require.Nil(t, err)
	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	differentHeaderHash := make([]byte, len(block.GetTxRootHash()))
	copy(differentHeaderHash, block.GetTxRootHash())
	differentHeaderHash[0] = ^differentHeaderHash[0]
	d.AlarmExpired(hex.EncodeToString(differentHeaderHash))
	time.Sleep(baseDelay)

	// check there was no broadcast and validator delay data still present
	require.Equal(t, int64(0), txBroadcastCalled.Get())

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))
}

func TestDelayedBlockBroadcaster_RegisterInterceptorCallback(t *testing.T) {
	t.Parallel()

	// setup
	var cbsHeader []func(topic string, hash []byte, data interface{})
	mutCbs := &sync.Mutex{}

	registerHandlerHeaders := func(handler func(topic string, hash []byte, data interface{})) {
		mutCbs.Lock()
		cbsHeader = append(cbsHeader, handler)
		mutCbs.Unlock()
	}

	// mock intercept container
	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	delayBroadcastArgs.InterceptorsContainer = &commonMock.InterceptorsContainerStub{
		GetCalled: func(topic string) (process.Interceptor, error) {
			var hdl func(handler func(topic string, hash []byte, data interface{}))
			switch topic {
			case common.BlocksTopic:
				hdl = registerHandlerHeaders
			default:
				return nil, errors.New("unexpected topic")
			}

			return &commonMock.InterceptorStub{
				RegisterHandlerCalled: hdl,
			}, nil
		},
	}

	// execute
	_, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)

	// assert
	mutCbs.Lock()
	nbRegisteredHeaderHandlers := len(cbsHeader)
	mutCbs.Unlock()
	require.Equal(t, 1, nbRegisteredHeaderHandlers)
}

func TestDelayedBlockBroadcaster_Close(t *testing.T) {
	t.Parallel()

	txBroadcastCalled := &atomic.Counter{}
	headerBroadcastCalled := &atomic.Counter{}

	broadcastTransactions := func(txData [][]byte) error {
		txBroadcastCalled.Increment()
		return nil
	}
	broadcastHeader := func(header data.HeaderHandler) error {
		headerBroadcastCalled.Increment()
		return nil
	}

	delayBroadcastArgs := newDefaultDelayedBroadcasterArgs()
	d, err := broadcast.NewDelayedBlockBroadcaster(delayBroadcastArgs)
	require.Nil(t, err)
	assert.NotNil(t, d)

	err = d.SetBroadcastHandlers(broadcastTransactions, broadcastHeader)
	require.Nil(t, err)

	headerHash, transactions := createDelayData("1")

	block := createBlock(transactions)

	block.Header.PrevRandSeed = []byte("prev")

	validatorDelay := broadcast.CreateDelayBroadcastDataForLeader(
		headerHash,
		block,
		block.GetTxHashes(),
	)

	err = d.SetValidatorData(validatorDelay)
	require.Nil(t, err)

	vbd := d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))

	dfv := &broadcast.HeaderDataForValidator{
		Slot:         block.GetNonce(),
		PrevRandSeed: block.GetPrevRandSeed(),
	}
	d.ScheduleValidatorBroadcast(dfv)
	d.Close()
	time.Sleep(2*common.ExtraDelayForBroadcastBlockInfo + orderDelay + baseDelay) // validatorDelayPerOrder + extra delay + order delay + base delay

	require.Equal(t, int64(0), txBroadcastCalled.Get())

	vbd = d.GetValidatorBroadcastData()
	require.Equal(t, 1, len(vbd))
}
