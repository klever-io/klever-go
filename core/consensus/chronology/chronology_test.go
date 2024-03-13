package chronology_test

import (
	"encoding/hex"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/chronology"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

var (
	nodesCoordinator sharding.NodesCoordinator
	pub              []byte
)

func initSubslotHandlerMock() *mock.SubslotHandlerMock {
	srm := &mock.SubslotHandlerMock{}
	srm.CurrentCalled = func() int {
		return 0
	}
	srm.NextCalled = func() int {
		return 1
	}
	srm.DoWorkCalled = func(slotHandler consensus.SlotManager) bool {
		return false
	}
	srm.NameCalled = func() string {
		return "(TEST)"
	}
	return srm
}

func init() {
	nodesCoordinator = commonMock.NewNodesCoordinatorMock()
	pub, _ = hex.DecodeString("2eec1bf505a88ab7a5a372d3d21281bb27b38f35e15e1a2b071fb5ff3d8d080650757aeb75779edd3abc6ed20b04f91882be2a016a9b0830f337ed27bd8d7e7c6e20383e26c7861f2a176059c3498b77ee02a248e56c71c21f9009cd4a8f8382")
	// TODO: push validator to initial node
}

func TestChronology_NewChronologyNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()
	syncTimerMock := &mock.SyncTimerMock{}
	chr, err := chronology.NewChronology(
		nil,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)

	assert.Nil(t, chr)
	assert.Equal(t, err, chronology.ErrNilSlotManager)
}

func TestChronology_NewChronologyNilSyncerShouldFail(t *testing.T) {
	t.Parallel()
	smMock := &mock.SlotManagerMock{}
	chr, err := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		nil,
		&mock.WatchdogMock{},
	)

	assert.Nil(t, chr)
	assert.Equal(t, err, chronology.ErrNilSyncTimer)
}

func TestChronology_NewChronologyNilWatchdogShouldFail(t *testing.T) {
	t.Parallel()
	smMock := &mock.SlotManagerMock{}
	chr, err := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		&mock.SyncTimerMock{},
		nil,
	)

	assert.Nil(t, chr)
	assert.Equal(t, err, chronology.ErrNilWatchdog)
}

func TestChronology_NewChronologyShouldWork(t *testing.T) {
	t.Parallel()
	smMock := &mock.SlotManagerMock{}
	syncTimerMock := &mock.SyncTimerMock{}
	chr, err := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(chr))
}

func TestChronology_StartSlotShouldWork(t *testing.T) {
	t.Parallel()
	syncTimerMock := &mock.SyncTimerMock{
		CurrentTimeCalled: func() time.Time {
			return time.Now()
		},
	}
	genesisTime := time.Now()
	smMock := &mock.SlotManagerMock{
		GenesisTimestampCalled: func() time.Time {
			return genesisTime
		},
		TimestampCalled: func() time.Time {
			return time.Now()
		},
	}

	blckChain := blockchain.NewBlockChain()
	_ = blckChain.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{
			Epoch:     0,
			Slot:      0,
			Timestamp: genesisTime.Unix(),
		},
	})

	smMock.UpdateSlot(smMock.Timestamp().Add(smMock.TimeDuration()))
	chr, _ := chronology.NewChronology(
		smMock,
		blckChain,
		nodesCoordinator,
		&mock.BlockProcessorMock{
			CommitBlockCalled: func(header data.HeaderHandler) error {
				return nil
			},
		},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)
	sh := &mock.SlotHandlerMock{
		IsMinerCalled: func(blsPubKey []byte) bool {
			return true
		},
		ProduceBlockCalled: func(blkc data.ChainHandler, sm consensus.SlotManager) (data.HeaderHandler, []byte, error) {
			return &block.Block{
				Header: &block.BlockHeader{
					Nonce:      0,
					ParentHash: make([]byte, 0),
					Timestamp:  time.Now().Unix(),
					Slot:       0,
					Epoch:      0,
					TrieRoot:   make([]byte, 0),
					TxCount:    0,
				},
			}, make([]byte, 32), nil
		},
	}
	// TODO: chr.SetSlotHandler(sh)
	_ = sh

	ash := &commonMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {

		},
	}
	_ = chr.SetAppStatusHandler(ash)

	ssm := initSubslotHandlerMock()
	ssm.DoWorkCalled = func(slotHandler consensus.SlotManager) bool {
		return true
	}
	chr.AddSubslot(ssm)
	chr.SetSubslotID(0)
	chr.StartSlot()

	assert.Equal(t, ssm.Next(), chr.SubslotID())
}

func TestChronology_UpdateSlotShouldInitSlot(t *testing.T) {
	t.Parallel()
	smMock := &mock.SlotManagerMock{}
	syncTimerMock := &mock.SyncTimerMock{}
	chr, _ := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)

	ash := &commonMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {

		},
	}
	err := chr.SetAppStatusHandler(ash)
	assert.Nil(t, err)

	srm := initSubslotHandlerMock()
	chr.AddSubslot(srm)
	chr.UpdateSlot()

	assert.Equal(t, srm.Current(), chr.SubslotID())
}

func TestChronology_InitSlotShouldNotSetSlotWhenSlotIndexIsNegative(t *testing.T) {
	t.Parallel()
	syncTimerMock := &mock.SyncTimerMock{}
	smMock := &mock.SlotManagerMock{
		GenesisTimestampCalled: func() time.Time {
			return syncTimerMock.CurrentTime()
		},
	}
	chr, _ := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)

	smMock.IndexCalled = func() int64 {
		return -1
	}
	smMock.BeforeGenesisCalled = func() bool {
		return true
	}
	chr.InitSlot()

	assert.Equal(t, int64(-1), chr.CurrentSlot())
}

func TestChronology_InitSlotShouldSetSlotWhenSlotIndexIsPositive(t *testing.T) {
	t.Parallel()
	syncTimerMock := &mock.SyncTimerMock{}
	smMock := &mock.SlotManagerMock{
		GenesisTimestampCalled: func() time.Time {
			return syncTimerMock.CurrentTime()
		},
	}
	smMock.UpdateSlot(smMock.Timestamp().Add(smMock.TimeDuration()))
	chr, _ := chronology.NewChronology(
		smMock,
		blockchain.NewBlockChain(),
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)
	ash := &commonMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {

		},
	}
	err := chr.SetAppStatusHandler(ash)
	assert.Nil(t, err)

	ss := initSubslotHandlerMock()
	chr.AddSubslot(ss)
	chr.InitSlot()

	assert.Equal(t, ss.Current(), chr.SubslotID())
}

func TestChronology_StartSlotShouldNotUpdateSlotWhenCurrentSlotIsNotFinished(t *testing.T) {
	t.Parallel()
	syncTimerMock := &mock.SyncTimerMock{}
	smMock := &mock.SlotManagerMock{
		GenesisTimestampCalled: func() time.Time {
			return syncTimerMock.CurrentTime()
		},
	}

	blckChain := blockchain.NewBlockChain()
	_ = blckChain.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{
			Epoch: 0,
			Slot:  0,
		},
	})

	chr, _ := chronology.NewChronology(
		smMock,
		blckChain,
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)
	sh := &mock.SlotHandlerMock{
		IsMinerCalled: func(blsPubKey []byte) bool {
			return true
		},
		ProduceBlockCalled: func(blkc data.ChainHandler, sm consensus.SlotManager) (data.HeaderHandler, []byte, error) {
			return &block.Block{
				Header: &block.BlockHeader{
					Nonce:      0,
					ParentHash: make([]byte, 0),
					Timestamp:  time.Now().Unix(),
					Slot:       0,
					Epoch:      0,
					TrieRoot:   make([]byte, 0),
					TxCount:    0,
				},
			}, make([]byte, 32), nil
		},
	}
	// TODO: chr.SetSlotHandler(sh)
	_ = sh

	ash := &commonMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {

		},
	}
	err := chr.SetAppStatusHandler(ash)
	assert.Nil(t, err)

	chr.SetCurrentSlot(0)
	chr.StartSlot()

	assert.Equal(t, int64(1), smMock.Index())
}

func TestChronology_StartSlotShouldUpdateSlotWhenCurrentSlotIsFinished(t *testing.T) {
	t.Parallel()

	syncTimerMock := &mock.SyncTimerMock{}
	smMock := &mock.SlotManagerMock{
		GenesisTimestampCalled: func() time.Time {
			return syncTimerMock.CurrentTime()
		},
	}

	blckChain := blockchain.NewBlockChain()
	_ = blckChain.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{
			Epoch: 0,
			Slot:  0,
		},
	})

	chr, _ := chronology.NewChronology(
		smMock,
		blckChain,
		nodesCoordinator,
		&mock.BlockProcessorMock{},
		&mock.BroadcastMessengerMock{},
		syncTimerMock,
		&mock.WatchdogMock{},
	)
	sh := &mock.SlotHandlerMock{
		IsMinerCalled: func(blsPubKey []byte) bool {
			return true
		},
		ProduceBlockCalled: func(blkc data.ChainHandler, sm consensus.SlotManager) (data.HeaderHandler, []byte, error) {
			return &block.Block{
				Header: &block.BlockHeader{
					Nonce:      0,
					ParentHash: make([]byte, 0),
					Timestamp:  time.Now().Unix(),
					Slot:       0,
					Epoch:      0,
					TrieRoot:   make([]byte, 0),
					TxCount:    0,
				},
			}, make([]byte, 32), nil
		},
	}
	// TODO: chr.SetSlotHandler(sh)
	_ = sh
	ash := &commonMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {

		},
	}
	err := chr.SetAppStatusHandler(ash)
	assert.Nil(t, err)

	chr.SetCurrentSlot(-1)
	chr.StartSlot()

	assert.Equal(t, int64(1), smMock.Index())
}
