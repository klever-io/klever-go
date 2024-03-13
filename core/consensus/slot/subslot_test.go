package slot_test

import (
	"sync"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/stretchr/testify/assert"
)

var chainID = []byte("chain ID")
var wrongChainID = []byte("wrong chain ID")

const currentPid = core.PeerID("pid")

// executeStoredMessages tries to execute all the messages received which are valid for execution
func executeStoredMessages() {
}

func createEligibleList(size int) []string {
	eligibleList := make([]string, 0)
	for i := 0; i < size; i++ {
		var value string
		for j := 0; j < PublicKeySize; j++ {
			value += string([]byte{byte(i + 65)})
		}

		eligibleList = append(eligibleList, value)
	}

	return eligibleList
}

func initConsensusState() *slot.ConsensusState {
	consensusGroupSize := 9
	eligibleList := createEligibleList(consensusGroupSize)

	eligibleNodesKeys := make(map[string]struct{}, len(eligibleList))
	for _, key := range eligibleList {
		eligibleNodesKeys[key] = struct{}{}
	}

	indexLeader := 1
	rcns := slot.NewSlotConsensus(
		eligibleNodesKeys,
		consensusGroupSize,
		eligibleList[indexLeader])

	rcns.SetConsensusGroup(eligibleList)
	rcns.ResetSlotState()

	pBFTThreshold := consensusGroupSize*2/3 + 1
	pBFTFallbackThreshold := consensusGroupSize*1/2 + 1

	rthr := slot.NewSlotThreshold()
	rthr.SetThreshold(1, 1)
	rthr.SetThreshold(2, pBFTThreshold)
	rthr.SetThreshold(3, pBFTThreshold)
	rthr.SetThreshold(4, pBFTThreshold)
	rthr.SetThreshold(5, pBFTThreshold)
	rthr.SetFallbackThreshold(1, 1)
	rthr.SetFallbackThreshold(2, pBFTFallbackThreshold)
	rthr.SetFallbackThreshold(3, pBFTFallbackThreshold)
	rthr.SetFallbackThreshold(4, pBFTFallbackThreshold)
	rthr.SetFallbackThreshold(5, pBFTFallbackThreshold)

	rstatus := slot.NewSlotStatus()
	rstatus.ResetSlotStatus()

	cns := slot.NewConsensusState(
		rcns,
		rthr,
		rstatus,
	)

	cns.Data = []byte("X")
	cns.SlotIndex = 0
	return cns
}

func TestSubslot_NewSubslotNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	ch := make(chan bool, 1)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		nil,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Equal(t, slot.ErrNilConsensusState, err)
	assert.Nil(t, sr)
}

func TestSubslot_NewSubslotNilChannelShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		nil,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Equal(t, slot.ErrNilChannel, err)
	assert.Nil(t, sr)
}

func TestSubslot_NewSubslotNilExecuteStoredMessagesShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	container := mock.InitConsensusCore()
	ch := make(chan bool, 1)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		nil,
		container,
		chainID,
		currentPid,
	)

	assert.Equal(t, slot.ErrNilExecuteStoredMessages, err)
	assert.Nil(t, sr)
}

func TestSubslot_NewSubslotNilContainerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		nil,
		chainID,
		currentPid,
	)

	assert.Equal(t, slot.ErrNilConsensusCore, err)
	assert.Nil(t, sr)
}

func TestSubslot_NilContainerBlockchainShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetBlockchain(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestSubslot_NilContainerBlockprocessorShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetBlockProcessor(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestSubslot_NilContainerBootstrapperShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetBootStrapper(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilBootstrapper, err)
}

func TestSubslot_NilContainerChronologyShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetChronology(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilChronologyHandler, err)
}

func TestSubslot_NilContainerHasherShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetHasher(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestSubslot_NilContainerMarshalizerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetMarshalizer(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilMarshalizer, err)
}

func TestSubslot_NilContainerMultisignerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetMultiSigner(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestSubslot_NilContainerSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetSlotManager(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestSubslot_NilContainerSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetSyncTimer(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestSubslot_NilContainerValidatorGroupSelectorShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(nil)

	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, sr)
	assert.Equal(t, slot.ErrNilNodesCoordinator, err)
}

func TestSubslot_EmptyChainIDShouldFail(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		nil,
		currentPid,
	)

	assert.Equal(t, slot.ErrInvalidChainID, err)
	assert.Nil(t, sr)
}

func TestSubslot_NewSubslotShouldWork(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	sr, err := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	assert.Nil(t, err)

	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.NotNil(t, sr)
}

func TestSubslot_DoWorkShouldReturnFalseWhenJobFunctionIsNotSet(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = nil
	sr.Check = func() bool {
		return true
	}

	maxTime := time.Now().Add(100 * time.Millisecond)
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.RemainingTimeCalled = func(time.Time, time.Duration) time.Duration {
		return time.Until(maxTime)
	}

	r := sr.DoWork(slotManagerMock)

	assert.False(t, r)
}

func TestSubslot_DoWorkShouldReturnFalseWhenCheckFunctionIsNotSet(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = nil

	maxTime := time.Now().Add(100 * time.Millisecond)
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.RemainingTimeCalled = func(time.Time, time.Duration) time.Duration {
		return time.Until(maxTime)
	}

	r := sr.DoWork(slotManagerMock)
	assert.False(t, r)
}

func TestSubslot_DoWorkShouldReturnFalseWhenConsensusIsNotDone(t *testing.T) {
	t.Parallel()

	testDoWork(t, false, false)
}

func TestSubslot_DoWorkShouldReturnTrueWhenJobAndConsensusAreDone(t *testing.T) {
	t.Parallel()

	testDoWork(t, true, true)
}

func testDoWork(t *testing.T, checkDone bool, shouldWork bool) {
	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return checkDone
	}

	maxTime := time.Now().Add(100 * time.Millisecond)
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.RemainingTimeCalled = func(time.Time, time.Duration) time.Duration {
		return time.Until(maxTime)
	}

	r := sr.DoWork(slotManagerMock)
	assert.Equal(t, shouldWork, r)
}

func TestSubslot_DoWorkShouldReturnTrueWhenJobIsDoneAndConsensusIsDoneAfterAWhile(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		-1,
		bls.SrStartSlot,
		bls.SrBlock,
		int64(0*slotTimeDuration/100),
		int64(5*slotTimeDuration/100),
		"(START_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	var mut sync.RWMutex
	mut.Lock()
	checkSuccess := false
	mut.Unlock()

	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		mut.RLock()
		defer mut.RUnlock()
		return checkSuccess
	}

	maxTime := time.Now().Add(2000 * time.Millisecond)
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.RemainingTimeCalled = func(time.Time, time.Duration) time.Duration {
		return time.Until(maxTime)
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)

		mut.Lock()
		checkSuccess = true
		mut.Unlock()

		ch <- true
	}()

	r := sr.DoWork(slotManagerMock)

	assert.True(t, r)
}

func TestSubslot_Previous(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, bls.SrStartSlot, sr.Previous())
}

func TestSubslot_Current(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, bls.SrBlock, sr.Current())
}

func TestSubslot_Next(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, bls.SrSignature, sr.Next())
}

func TestSubslot_StartTime(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetSlotManager(initSlotManagerMock())
	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(25*slotTimeDuration/100),
		int64(40*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, int64(25*slotTimeDuration/100), sr.StartTime())
}

func TestSubslot_EndTime(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()
	container.SetSlotManager(initSlotManagerMock())
	sr, _ := slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, int64(25*slotTimeDuration/100), sr.EndTime())
}

func TestSubslot_Name(t *testing.T) {
	t.Parallel()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	container := mock.InitConsensusCore()

	sr, _ := slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	sr.Job = func() bool {
		return true
	}
	sr.Check = func() bool {
		return false
	}

	assert.Equal(t, "(BLOCK)", sr.Name())
}

func TestSubslot_AppStatusHandlerNilShouldErr(t *testing.T) {
	t.Parallel()

	sr := &slot.Subslot{}
	err := sr.SetAppStatusHandler(nil)

	assert.Equal(t, slot.ErrNilAppStatusHandler, err)
}

func TestSubslot_AppStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	sr := &slot.Subslot{}
	ash := &cMock.AppStatusHandlerStub{}
	err := sr.SetAppStatusHandler(ash)

	assert.Nil(t, err)
	assert.True(t, ash == sr.AppStatusHandler())
}
