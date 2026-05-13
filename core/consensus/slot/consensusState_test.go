package slot_test

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
)

func internalInitConsensusState() *slot.ConsensusState {
	eligibleList := []string{"1", "2", "3"}

	eligibleNodesPubKeys := make(map[string]struct{})
	for _, key := range eligibleList {
		eligibleNodesPubKeys[key] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(
		eligibleNodesPubKeys,
		3,
		"2")

	rcns.SetConsensusGroup(eligibleList)
	rcns.ResetSlotState()

	rthr := slot.NewSlotThreshold()

	rthr.SetThreshold(bls.SrBlock, 1)
	rthr.SetThreshold(bls.SrSignature, 3)
	rthr.SetFallbackThreshold(bls.SrBlock, 1)
	rthr.SetFallbackThreshold(bls.SrSignature, 2)

	rstatus := slot.NewSlotStatus()
	rstatus.ResetSlotStatus()

	cns := slot.NewConsensusState(
		rcns,
		rthr,
		rstatus,
	)

	return cns
}

func TestConsensusState__NewConsensusStateShouldWork(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	assert.NotNil(t, cns)
}

func TestConsensusState_ResetConsensusStateShouldWork(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.SlotCanceled = true
	cns.ExtendedCalled = true
	cns.WaitingAllSignaturesTimeOut = true
	cns.ResetConsensusState()
	assert.False(t, cns.SlotCanceled)
	assert.False(t, cns.ExtendedCalled)
	assert.False(t, cns.WaitingAllSignaturesTimeOut)
}

func TestConsensusState_IsNodeLeaderInCurrentSlotShouldReturnFalseWhenGetLeaderErr(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.SetConsensusGroup(nil)
	assert.Equal(t, false, cns.IsNodeLeaderInCurrentSlot("1"))
}

func TestConsensusState_IsNodeLeaderInCurrentSlotShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.Equal(t, false, cns.IsNodeLeaderInCurrentSlot("2"))
}

func TestConsensusState_IsNodeLeaderInCurrentSlotShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.Equal(t, true, cns.IsNodeLeaderInCurrentSlot("1"))
}

func TestConsensusState_IsSelfLeaderInCurrentSlotShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.False(t, cns.IsSelfLeaderInCurrentSlot())
}

func TestConsensusState_IsSelfLeaderInCurrentSlotShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.False(t, cns.IsSelfLeaderInCurrentSlot())
}

func TestConsensusState_GetLeaderShoudErrNilConsensusGroup(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.SetConsensusGroup(nil)

	_, err := cns.GetLeader()
	assert.Equal(t, slot.ErrNilConsensusGroup, err)
}

func TestConsensusState_GetLeaderShouldErrEmptyConsensusGroup(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.SetConsensusGroup(make([]string, 0))

	_, err := cns.GetLeader()
	assert.Equal(t, slot.ErrEmptyConsensusGroup, err)
}

func TestConsensusState_GetLeaderShouldWork(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	leader, err := cns.GetLeader()
	assert.Nil(t, err)
	assert.Equal(t, cns.ConsensusGroup()[0], leader)
}

func TestConsensusState_GetNextConsensusGroupShouldFailWhenComputeValidatorsGroupErr(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	nodesCoordinator := &mock.NodesCoordinatorMock{}
	err := errors.New("error")
	nodesCoordinator.ComputeValidatorsGroupCalled = func(
		randomness []byte,
		slot uint64,
		epoch uint32,
	) ([]sharding.Validator, error) {
		return nil, err
	}

	_, err2 := cns.GetNextConsensusGroup([]byte(""), 0, nodesCoordinator, 0)
	assert.Equal(t, err, err2)
}

func TestConsensusState_GetNextConsensusGroupShouldWork(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	nodesCoordinator := &mock.NodesCoordinatorMock{}

	nextConsensusGroup, err := cns.GetNextConsensusGroup([]byte(""), 0, nodesCoordinator, 0)
	assert.Nil(t, err)
	assert.NotNil(t, nextConsensusGroup)
}

func TestConsensusState_IsConsensusDataSetShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = make([]byte, 0)

	assert.True(t, cns.IsConsensusDataSet())
}

func TestConsensusState_IsConsensusDataSetShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = nil

	assert.False(t, cns.IsConsensusDataSet())
}

func TestConsensusState_IsConsensusDataEqualShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	data := []byte("consensus data")

	cns.Data = data

	assert.True(t, cns.IsConsensusDataEqual(data))
}

func TestConsensusState_IsConsensusDataEqualShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	data := []byte("consensus data")

	cns.Data = data

	assert.False(t, cns.IsConsensusDataEqual([]byte("X")))
}

func TestConsensusState_IsJobDoneShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	_ = cns.SetJobDone("1", bls.SrBlock, false)
	assert.False(t, cns.IsJobDone("1", bls.SrBlock))

	_ = cns.SetJobDone("1", bls.SrSignature, true)
	assert.False(t, cns.IsJobDone("1", bls.SrBlock))

	_ = cns.SetJobDone("2", bls.SrBlock, true)
	assert.False(t, cns.IsJobDone("1", bls.SrBlock))
}

func TestConsensusState_IsJobDoneShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	_ = cns.SetJobDone("1", bls.SrBlock, true)

	assert.True(t, cns.IsJobDone("1", bls.SrBlock))
}

func TestConsensusState_IsSelfJobDoneShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, false)
	assert.False(t, cns.IsSelfJobDone(bls.SrBlock))

	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrSignature, true)
	assert.False(t, cns.IsSelfJobDone(bls.SrBlock))

	_ = cns.SetJobDone(cns.SelfPubKey()+"X", bls.SrBlock, true)
	assert.False(t, cns.IsSelfJobDone(bls.SrBlock))
}

func TestConsensusState_IsSelfJobDoneShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, true)

	assert.True(t, cns.IsSelfJobDone(bls.SrBlock))
}

func TestConsensusState_IsCurrentSubslotFinishedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.SetStatus(bls.SrBlock, slot.SsNotFinished)
	assert.False(t, cns.IsSubslotFinished(bls.SrBlock))

	cns.SetStatus(bls.SrSignature, slot.SsFinished)
	assert.False(t, cns.IsSubslotFinished(bls.SrBlock))

}

func TestConsensusState_IsCurrentSubslotFinishedShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.SetStatus(bls.SrBlock, slot.SsFinished)
	assert.True(t, cns.IsSubslotFinished(bls.SrBlock))
}

func TestConsensusState_IsNodeSelfShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.False(t, cns.IsNodeSelf(cns.SelfPubKey()+"X"))
}

func TestConsensusState_IsNodeSelfShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	assert.True(t, cns.IsNodeSelf(cns.SelfPubKey()))
}

func TestConsensusState_IsHeaderAlreadyReceivedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Header = nil

	assert.False(t, cns.IsHeaderAlreadyReceived())
}

func TestConsensusState_IsHeaderAlreadyReceivedShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Header = &block.Block{}

	assert.True(t, cns.IsHeaderAlreadyReceived())
}

func TestConsensusState_CanDoSubslotJobShouldReturnFalseWhenConsensusDataNotSet(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = nil

	assert.False(t, cns.CanDoSubslotJob(bls.SrBlock))
}

func TestConsensusState_CanDoSubslotJobShouldReturnFalseWhenSelfJobIsDone(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = make([]byte, 0)
	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, true)

	assert.False(t, cns.CanDoSubslotJob(bls.SrBlock))
}

func TestConsensusState_CanDoSubslotJobShouldReturnFalseWhenCurrentSlotIsFinished(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = make([]byte, 0)
	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, false)
	cns.SetStatus(bls.SrBlock, slot.SsFinished)

	assert.False(t, cns.CanDoSubslotJob(bls.SrBlock))
}

func TestConsensusState_CanDoSubslotJobShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.Data = make([]byte, 0)
	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, false)
	cns.SetStatus(bls.SrBlock, slot.SsNotFinished)

	assert.True(t, cns.CanDoSubslotJob(bls.SrBlock))
}

func TestConsensusState_CanProcessReceivedMessageShouldReturnFalseWhenMessageIsReceivedFromItself(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cnsDta := &consensus.Message{
		SlotIndex: 0,
		PubKey:    []byte(cns.SelfPubKey()),
	}

	assert.False(t, cns.CanProcessReceivedMessage(cnsDta, 0, bls.SrBlock))
}

func TestConsensusState_CanProcessReceivedMessageShouldReturnFalseWhenMessageIsReceivedForOtherSlot(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cnsDta := &consensus.Message{
		SlotIndex: 0,
		PubKey:    []byte("1"),
	}

	assert.False(t, cns.CanProcessReceivedMessage(cnsDta, 1, bls.SrBlock))
}

func TestConsensusState_CanProcessReceivedMessageShouldReturnFalseWhenJobIsDone(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cnsDta := &consensus.Message{
		SlotIndex: 0,
		PubKey:    []byte("1"),
	}

	_ = cns.SetJobDone("1", bls.SrBlock, true)

	assert.False(t, cns.CanProcessReceivedMessage(cnsDta, 0, bls.SrBlock))
}

func TestConsensusState_CanProcessReceivedMessageShouldReturnFalseWhenCurrentSlotIsFinished(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cnsDta := &consensus.Message{
		SlotIndex: 0,
		PubKey:    []byte("1"),
	}

	cns.SetStatus(bls.SrBlock, slot.SsFinished)

	assert.False(t, cns.CanProcessReceivedMessage(cnsDta, 0, bls.SrBlock))
}

func TestConsensusState_CanProcessReceivedMessageShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cnsDta := &consensus.Message{
		SlotIndex: 0,
		PubKey:    []byte("1"),
	}

	assert.True(t, cns.CanProcessReceivedMessage(cnsDta, 0, bls.SrBlock))
}

func TestConsensusState_GenerateBitmapShouldWork(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	bitmapExpected := make([]byte, cns.ConsensusGroupSize()/8+1)
	selfIndexInConsensusGroup, _ := cns.SelfConsensusGroupIndex()
	bitmapExpected[selfIndexInConsensusGroup/8] |= 1 << (uint16(selfIndexInConsensusGroup) % 8)

	_ = cns.SetJobDone(cns.SelfPubKey(), bls.SrBlock, true)
	bitmap := cns.GenerateBitmap(bls.SrBlock)

	assert.Equal(t, bitmapExpected, bitmap)
}

func TestConsensusState_SetAndGetProcessingBlockShouldWork(t *testing.T) {
	t.Parallel()
	cns := internalInitConsensusState()
	cns.SetProcessingBlock(true)

	assert.Equal(t, true, cns.ProcessingBlock())
}

func TestConsensusState_LockUnlockSlotStateAllowsReentryAfterRelease(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.LockSlotState()
	cns.SlotIndex = 7
	cns.UnlockSlotState()

	cns.RLockSlotState()
	got := cns.SlotIndex
	cns.RUnlockSlotState()

	assert.Equal(t, int64(7), got)
}

func TestConsensusState_SetSlotCanceledTogglesFlag(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.SetSlotCanceled(true)
	assert.True(t, cns.SlotCanceled)

	cns.SetSlotCanceled(false)
	assert.False(t, cns.SlotCanceled)
}

func TestConsensusState_SetExtendedCalledTogglesFlag(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.SetExtendedCalled(true)
	assert.True(t, cns.ExtendedCalled)

	cns.SetExtendedCalled(false)
	assert.False(t, cns.ExtendedCalled)
}

func TestConsensusState_SetWaitingAllSignaturesTimeOutTogglesFlag(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.SetWaitingAllSignaturesTimeOut(true)
	assert.True(t, cns.WaitingAllSignaturesTimeOut)

	cns.SetWaitingAllSignaturesTimeOut(false)
	assert.False(t, cns.WaitingAllSignaturesTimeOut)
}

func TestConsensusState_SetWaitingAllSignaturesTimeOutIfSlot_AppliesOnSameSlot(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.BeginNewSlot(42, time.Unix(1700000000, 0))

	applied := cns.SetWaitingAllSignaturesTimeOutIfSlot(42, true)
	assert.True(t, applied)
	assert.True(t, cns.WaitingAllSignaturesTimeOut)
}

func TestConsensusState_SetWaitingAllSignaturesTimeOutIfSlot_SkipsOnSlotMismatch(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.BeginNewSlot(42, time.Unix(1700000000, 0))

	applied := cns.SetWaitingAllSignaturesTimeOutIfSlot(41, true)
	assert.False(t, applied)
	assert.False(t, cns.WaitingAllSignaturesTimeOut)
}

func TestConsensusState_GetDataReadsUnderLock(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()

	cns.LockSlotState()
	cns.Data = []byte("payload")
	cns.UnlockSlotState()

	assert.Equal(t, []byte("payload"), cns.GetData())
}

func TestConsensusState_BeginNewSlotResetsFlagsAndUpdatesIndex(t *testing.T) {
	t.Parallel()

	cns := internalInitConsensusState()
	cns.SetSlotCanceled(true)
	cns.SetExtendedCalled(true)
	cns.SetWaitingAllSignaturesTimeOut(true)
	cns.LockSlotState()
	cns.Data = []byte("stale")
	cns.Header = &block.Block{}
	cns.UnlockSlotState()

	ts := time.Unix(1700000000, 0)
	cns.BeginNewSlot(99, ts)

	cns.RLockSlotState()
	idx := cns.SlotIndex
	stamp := cns.SlotTimestamp
	data := cns.Data
	header := cns.Header
	cns.RUnlockSlotState()

	assert.Equal(t, int64(99), idx)
	assert.Equal(t, ts, stamp)
	assert.Nil(t, data)
	assert.Nil(t, header)
	assert.False(t, cns.SlotCanceled)
	assert.False(t, cns.ExtendedCalled)
	assert.False(t, cns.WaitingAllSignaturesTimeOut)
}
