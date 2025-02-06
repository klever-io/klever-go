package bls_test

import (
	"fmt"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

var chainID = []byte("chain ID")

const currentPid = core.PeerID("pid")

const slotTimeDuration = 100 * time.Millisecond

func displayStatistics() {
}

func extend(subslotId int) {
	fmt.Println(subslotId)
}

// executeStoredMessages tries to execute all the messages received which are valid for execution
func executeStoredMessages() {
}

// resetConsensusMessages resets at the start of each slot, all the previous consensus messages received
func resetConsensusMessages() {
}

func initSlotManagerMock() *mock.SlotManagerMock {
	return &mock.SlotManagerMock{
		SlotIndex: 0,
		TimestampCalled: func() time.Time {
			return time.Unix(0, 0)
		},
		TimeDurationCalled: func() time.Duration {
			return slotTimeDuration
		},
	}
}

func initWorker() slot.WorkerHandler {
	slotWorker := &mock.SlotWorkerMock{}
	slotWorker.GetConsensusStateChangedChannelsCalled = func() chan bool {
		return make(chan bool)
	}
	slotWorker.RemoveAllReceivedMessagesCallsCalled = func() {}

	slotWorker.AddReceivedMessageCallCalled =
		func(messageType consensus.MessageType, receivedMessageCall func(cnsDta *consensus.Message) bool) {}

	return slotWorker
}

func initFactoryWithContainer(container *mock.ConsensusCoreMock) bls.Factory {
	worker := initWorker()
	consensusState := initConsensusState()

	fct, _ := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	return fct
}

func initFactory() bls.Factory {
	container := mock.InitConsensusCore()
	return initFactoryWithContainer(container)
}

func TestFactory_GetMessageTypeName(t *testing.T) {
	t.Parallel()

	r := bls.GetStringValue(bls.MtBlockHeader)
	assert.Equal(t, "(BLOCK_HEADER)", r)

	r = bls.GetStringValue(bls.MtSignature)
	assert.Equal(t, "(SIGNATURE)", r)

	r = bls.GetStringValue(bls.MtBlockHeaderFinalInfo)
	assert.Equal(t, "(FINAL_INFO)", r)

	r = bls.GetStringValue(bls.MtUnknown)
	assert.Equal(t, "(UNKNOWN)", r)

	r = bls.GetStringValue(consensus.MessageType(-1))
	assert.Equal(t, "Undefined message type", r)
}

func TestFactory_NewFactoryNilContainerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	worker := initWorker()

	fct, err := bls.NewSubslotsFactory(
		nil,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilConsensusCore, err)
}

func TestFactory_NewFactoryNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	worker := initWorker()

	fct, err := bls.NewSubslotsFactory(
		container,
		nil,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestFactory_NewFactoryNilBlockchainShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetBlockchain(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestFactory_NewFactoryNilBlockProcessorShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetBlockProcessor(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestFactory_NewFactoryNilBootstrapperShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetBootStrapper(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilBootstrapper, err)
}

func TestFactory_NewFactoryNilChronologyHandlerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetChronology(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilChronologyHandler, err)
}

func TestFactory_NewFactoryNilHasherShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetHasher(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestFactory_NewFactoryNilMarshalizerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetMarshalizer(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilMarshalizer, err)
}

func TestFactory_NewFactoryNilMultiSignerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetMultiSigner(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestFactory_NewFactoryNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetSlotManager(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestFactory_NewFactoryNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetSyncTimer(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestFactory_NewFactoryNilValidatorGroupSelectorShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()
	container.SetValidatorGroupSelector(nil)

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilNodesCoordinator, err)
}

func TestFactory_NewFactoryNilWorkerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		nil,
		chainID,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrNilWorker, err)
}

func TestFactory_NewFactoryShouldWork(t *testing.T) {
	t.Parallel()

	fct := *initFactory()

	assert.False(t, check.IfNil(&fct))
}

func TestFactory_NewFactoryEmptyChainIDShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	worker := initWorker()

	fct, err := bls.NewSubslotsFactory(
		container,
		consensusState,
		worker,
		nil,
		currentPid,
	)

	assert.Nil(t, fct)
	assert.Equal(t, slot.ErrInvalidChainID, err)
}

func TestFactory_GenerateSubslotStartSlotShouldFailWhenNewSubslotFail(t *testing.T) {
	t.Parallel()

	fct := *initFactory()
	fct.Worker().(*mock.SlotWorkerMock).GetConsensusStateChangedChannelsCalled = func() chan bool {
		return nil
	}

	err := fct.GenerateStartSlotSubslot()

	assert.Equal(t, slot.ErrNilChannel, err)
}

func TestFactory_GenerateSubslotStartSlotShouldFailWhenNewSubslotStartSlotFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)
	container.SetSyncTimer(nil)

	err := fct.GenerateStartSlotSubslot()

	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestFactory_GenerateSubslotBlockShouldFailWhenNewSubslotFail(t *testing.T) {
	t.Parallel()

	fct := *initFactory()
	fct.Worker().(*mock.SlotWorkerMock).GetConsensusStateChangedChannelsCalled = func() chan bool {
		return nil
	}

	err := fct.GenerateBlockSubslot()

	assert.Equal(t, slot.ErrNilChannel, err)
}

func TestFactory_GenerateSubslotBlockShouldFailWhenNewSubslotBlockFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)
	container.SetSyncTimer(nil)

	err := fct.GenerateBlockSubslot()

	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestFactory_GenerateSubslotSignatureShouldFailWhenNewSubslotFail(t *testing.T) {
	t.Parallel()

	fct := *initFactory()
	fct.Worker().(*mock.SlotWorkerMock).GetConsensusStateChangedChannelsCalled = func() chan bool {
		return nil
	}

	err := fct.GenerateSignatureSubslot()

	assert.Equal(t, slot.ErrNilChannel, err)
}

func TestFactory_GenerateSubslotSignatureShouldFailWhenNewSubslotSignatureFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)
	container.SetSyncTimer(nil)

	err := fct.GenerateSignatureSubslot()

	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestFactory_GenerateSubslotEndSlotShouldFailWhenNewSubslotFail(t *testing.T) {
	t.Parallel()

	fct := *initFactory()
	fct.Worker().(*mock.SlotWorkerMock).GetConsensusStateChangedChannelsCalled = func() chan bool {
		return nil
	}

	err := fct.GenerateEndSlotSubslot()

	assert.Equal(t, slot.ErrNilChannel, err)
}

func TestFactory_GenerateSubslotEndSlotShouldFailWhenNewSubslotEndSlotFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)
	container.SetSyncTimer(nil)

	err := fct.GenerateEndSlotSubslot()

	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestFactory_GenerateSubslotsShouldWork(t *testing.T) {
	t.Parallel()

	subslotHandlers := 0

	chrm := &mock.ChronologyHandlerMock{}
	chrm.AddSubslotCalled = func(subslotHandler consensus.SubslotHandler) {
		subslotHandlers++
	}
	container := mock.InitConsensusCore()
	container.SetChronology(chrm)
	fct := *initFactoryWithContainer(container)

	_ = fct.GenerateSubslots()

	assert.Equal(t, 4, subslotHandlers)
}

func TestFactory_SetAppStatusHandlerNilStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)

	err := fct.SetAppStatusHandler(nil)
	assert.Equal(t, slot.ErrNilAppStatusHandler, err)
}

func TestFactory_SetAppStatusHandlerOkStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)

	ash := &cMock.AppStatusHandlerMock{}
	err := fct.SetAppStatusHandler(ash)

	assert.Nil(t, err)
	assert.Equal(t, ash, fct.AppStatusHandler())
}

func TestFactory_SetIndexerShouldWork(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	fct := *initFactoryWithContainer(container)

	indexer := &mock.IndexerMock{}
	fct.SetIndexer(indexer)

	assert.Equal(t, indexer, fct.Indexer())
}
