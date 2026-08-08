package bls_test

import (
	"errors"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultSubslotStartSlotFromSubslot(sr *slot.Subslot) (bls.SubslotStartSlot, error) {
	startSlot, err := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		resetConsensusMessages,
	)

	return startSlot, err
}

func defaultWithoutErrorSubslotStartSlotFromSubslot(sr *slot.Subslot) bls.SubslotStartSlot {
	startSlot, _ := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		resetConsensusMessages,
	)

	return startSlot
}

func defaultSubslot(
	consensusState *slot.ConsensusState,
	ch chan bool,
	container slot.ConsensusCoreHandler,
) (*slot.Subslot, error) {

	return slot.NewSubslot(
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
}

func initSubslotStartSlotWithContainer(container slot.ConsensusCoreHandler) bls.SubslotStartSlot {
	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, _ := defaultSubslot(consensusState, ch, container)
	srStartSlot, _ := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		resetConsensusMessages,
	)

	return srStartSlot
}

func initSubslotStartSlot() bls.SubslotStartSlot {
	container := mock.InitConsensusCore()
	return initSubslotStartSlotWithContainer(container)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilSubslotShouldFail(t *testing.T) {
	t.Parallel()

	srStartSlot, err := bls.NewSubslotStartSlot(
		nil,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		resetConsensusMessages,
	)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilSubslot, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilBlockChainShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetBlockchain(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilBootstrapperShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetBootStrapper(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilBootstrapper, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)

	sr.ConsensusState = nil
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilMultiSignerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetMultiSigner(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetSlotManager(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetSyncTimer(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotNilValidatorGroupSelectorShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)
	container.SetValidatorGroupSelector(nil)
	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.Nil(t, srStartSlot)
	assert.Equal(t, slot.ErrNilNodesCoordinator, err)
}

func TestSubslotStartSlot_NewSubslotStartSlotShouldWork(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)

	srStartSlot, err := defaultSubslotStartSlotFromSubslot(sr)

	assert.NotNil(t, srStartSlot)
	assert.Nil(t, err)
}

func TestSubslotStartSlot_DoStartSlotShouldReturnTrue(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)

	srStartSlot := *defaultWithoutErrorSubslotStartSlotFromSubslot(sr)

	r := srStartSlot.DoStartSlotJob()
	assert.True(t, r)
}

func TestSubslotStartSlot_DoStartSlotConsensusCheckShouldReturnFalseWhenSlotIsCanceled(t *testing.T) {
	t.Parallel()

	sr := *initSubslotStartSlot()

	sr.SlotCanceled = true

	ok := sr.DoStartSlotConsensusCheck()
	assert.False(t, ok)
}

func TestSubslotStartSlot_DoStartSlotConsensusCheckShouldReturnTrueWhenSlotIsFinished(t *testing.T) {
	t.Parallel()

	sr := *initSubslotStartSlot()

	sr.SetStatus(bls.SrStartSlot, slot.SsFinished)

	ok := sr.DoStartSlotConsensusCheck()
	assert.True(t, ok)
}

func TestSubslotStartSlot_DoStartSlotConsensusCheckShouldReturnTrueWhenInitCurrentSlotReturnTrue(t *testing.T) {
	t.Parallel()

	bootstrapperMock := &mock.BootstrapperMock{GetNodeStateCalled: func() core.NodeState {
		return core.NsSynchronized
	}}

	container := mock.InitConsensusCore()
	container.SetBootStrapper(bootstrapperMock)

	sr := *initSubslotStartSlotWithContainer(container)

	ok := sr.DoStartSlotConsensusCheck()
	assert.True(t, ok)
}

func TestSubslotStartSlot_DoStartSlotConsensusCheckShouldReturnFalseWhenInitCurrentSlotReturnFalse(t *testing.T) {
	t.Parallel()

	bootstrapperMock := &mock.BootstrapperMock{GetNodeStateCalled: func() core.NodeState {
		return core.NsNotSynchronized
	}}

	container := mock.InitConsensusCore()
	container.SetBootStrapper(bootstrapperMock)
	container.SetSlotManager(initSlotManagerMock())

	sr := *initSubslotStartSlotWithContainer(container)

	ok := sr.DoStartSlotConsensusCheck()
	assert.False(t, ok)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenGetNodeStateNotReturnSynchronized(t *testing.T) {
	t.Parallel()

	bootstrapperMock := &mock.BootstrapperMock{}

	bootstrapperMock.GetNodeStateCalled = func() core.NodeState {
		return core.NsNotSynchronized
	}
	container := mock.InitConsensusCore()
	container.SetBootStrapper(bootstrapperMock)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenGenerateNextConsensusGroupErr(t *testing.T) {
	t.Parallel()

	validatorGroupSelector := &cMock.NodesCoordinatorMock{}
	err := errors.New("error")
	validatorGroupSelector.ComputeValidatorsGroupCalled = func(bytes []byte, slot uint64, epoch uint32) ([]sharding.Validator, error) {
		return nil, err
	}
	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(validatorGroupSelector)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenMainMachineIsActive(t *testing.T) {
	t.Parallel()

	nodeRedundancyMock := &mock.NodeRedundancyHandlerStub{
		IsRedundancyNodeCalled: func() bool {
			return true
		},
	}
	container := mock.InitConsensusCore()
	container.SetNodeRedundancyHandler(nodeRedundancyMock)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenGetLeaderErr(t *testing.T) {
	t.Parallel()

	validatorGroupSelector := &cMock.NodesCoordinatorMock{}
	validatorGroupSelector.ComputeValidatorsGroupCalled = func(
		bytes []byte,
		slot uint64,
		epoch uint32,
	) ([]sharding.Validator, error) {
		return make([]sharding.Validator, 0), nil
	}

	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(validatorGroupSelector)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnTrueWhenIsNotInTheConsensusGroup(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	consensusState.SetSelfPubKey(consensusState.SelfPubKey() + "X")
	ch := make(chan bool, 1)

	sr, _ := defaultSubslot(consensusState, ch, container)

	srStartSlot := *defaultWithoutErrorSubslotStartSlotFromSubslot(sr)

	r := srStartSlot.InitCurrentSlot()
	assert.True(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenCreateErr(t *testing.T) {
	t.Parallel()

	multiSignerMock := mock.InitMultiSignerMock()
	err := errors.New("error")
	multiSignerMock.ResetCalled = func(pubKeys []string, index uint16) error {
		return err
	}

	container := mock.InitConsensusCore()
	container.SetMultiSigner(multiSignerMock)

	srStartSlot := *initSubslotStartSlotWithContainer(container)
	srStartSlot.SetSelfPubKey("pubKey0")

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnFalseWhenTimeIsOut(t *testing.T) {
	t.Parallel()

	sloterMock := initSlotManagerMock()

	sloterMock.RemainingTimeCalled = func(time.Time, time.Duration) time.Duration {
		return time.Duration(-1)
	}

	container := mock.InitConsensusCore()
	container.SetSlotManager(sloterMock)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.False(t, r)
}

func TestSubslotStartSlot_InitCurrentSlotShouldReturnTrue(t *testing.T) {
	t.Parallel()

	bootstrapperMock := &mock.BootstrapperMock{}

	bootstrapperMock.GetNodeStateCalled = func() core.NodeState {
		return core.NsSynchronized
	}

	container := mock.InitConsensusCore()
	container.SetBootStrapper(bootstrapperMock)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	r := srStartSlot.InitCurrentSlot()
	assert.True(t, r)
}

func TestSubslotStartSlot_GenerateNextConsensusGroupShouldReturnErr(t *testing.T) {
	t.Parallel()

	validatorGroupSelector := &cMock.NodesCoordinatorMock{}

	err := errors.New("error")
	validatorGroupSelector.ComputeValidatorsGroupCalled = func(
		bytes []byte,
		slot uint64,
		epoch uint32,
	) ([]sharding.Validator, error) {
		return nil, err
	}
	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(validatorGroupSelector)

	srStartSlot := *initSubslotStartSlotWithContainer(container)

	err2 := srStartSlot.GenerateNextConsensusGroup(0)

	assert.Equal(t, err, err2)
}

// whitelistNodesCoordinatorMock overrides GetConsensusWhitelistedNodes, which
// the embedded common mock leaves unimplemented (it panics)
type whitelistNodesCoordinatorMock struct {
	*cMock.NodesCoordinatorMock
	whitelisted    map[string]struct{}
	err            error
	requestedEpoch uint32
}

func (w *whitelistNodesCoordinatorMock) GetConsensusWhitelistedNodes(epoch uint32) (map[string]struct{}, error) {
	w.requestedEpoch = epoch
	if w.err != nil {
		return nil, w.err
	}
	return w.whitelisted, nil
}

func TestSubslotStartSlot_EpochStartActionRefreshesElectedNodes(t *testing.T) {
	t.Parallel()

	newEpochKey := "newEpochValidator"
	coordinator := &whitelistNodesCoordinatorMock{
		NodesCoordinatorMock: cMock.NewNodesCoordinatorMock(),
		whitelisted:          map[string]struct{}{newEpochKey: {}},
	}
	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(coordinator)
	container.SetBootStrapper(&mock.BootstrapperMock{GetNodeStateCalled: func() core.NodeState {
		return core.NsNotSynchronized
	}})

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, err := defaultSubslot(consensusState, ch, container)
	require.Nil(t, err)
	resetCount := 0
	srStartSlotPtr, err := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		func() { resetCount++ },
	)
	require.Nil(t, err)
	srStartSlot := *srStartSlotPtr

	oldKey := consensusState.ConsensusGroup()[0]
	require.True(t, consensusState.IsNodeInConsensusGroup(oldKey))
	require.False(t, consensusState.IsNodeInConsensusGroup(newEpochKey))

	srStartSlot.EpochStartAction(&block.Block{Header: &block.BlockHeader{Epoch: 1}})

	// the whitelist must be requested for the epoch of the triggering header
	require.Equal(t, uint32(1), coordinator.requestedEpoch)

	// the elected set is replaced wholesale with the new epoch's whitelisted
	// nodes, so the new-epoch validator is accepted and the old one is not
	require.True(t, consensusState.IsNodeInConsensusGroup(newEpochKey))
	require.False(t, consensusState.IsNodeInConsensusGroup(oldKey))

	// the per-slot message budgets of the previous epoch are cleared exactly once
	require.Equal(t, 1, resetCount)
}

func TestSubslotStartSlot_EpochStartActionKeepsElectedNodesOnCoordinatorError(t *testing.T) {
	t.Parallel()

	coordinator := &whitelistNodesCoordinatorMock{
		NodesCoordinatorMock: cMock.NewNodesCoordinatorMock(),
		err:                  errors.New("coordinator has no config for the epoch"),
	}
	container := mock.InitConsensusCore()
	container.SetValidatorGroupSelector(coordinator)
	container.SetBootStrapper(&mock.BootstrapperMock{GetNodeStateCalled: func() core.NodeState {
		return core.NsNotSynchronized
	}})

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, err := defaultSubslot(consensusState, ch, container)
	require.Nil(t, err)
	resetCount := 0
	srStartSlotPtr, err := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		func() { resetCount++ },
	)
	require.Nil(t, err)
	srStartSlot := *srStartSlotPtr

	oldKey := consensusState.ConsensusGroup()[0]
	require.True(t, consensusState.IsNodeInConsensusGroup(oldKey))

	srStartSlot.EpochStartAction(&block.Block{Header: &block.BlockHeader{Epoch: 1}})

	require.Equal(t, uint32(1), coordinator.requestedEpoch)

	// on a coordinator error the previous elected set is kept unchanged and the
	// message budgets are not cleared, since no epoch transition was applied
	require.True(t, consensusState.IsNodeInConsensusGroup(oldKey))
	require.Equal(t, 0, resetCount)
}

func TestSubslotStartSlot_EpochStartActionDoesNotWidenOnUncalculatedNodeState(t *testing.T) {
	t.Parallel()

	// GetNodeState reports NsNotCalculated for a few milliseconds after every
	// slot boundary, including on a fully synchronized validator. Treating that
	// as "syncing" would widen the elected set mid-slot on a healthy validator,
	// which is exactly what the synchronized skip exists to prevent
	for _, nodeState := range []core.NodeState{core.NsSynchronized, core.NsNotCalculated} {
		newEpochKey := "newEpochValidator"
		coordinator := &whitelistNodesCoordinatorMock{
			NodesCoordinatorMock: cMock.NewNodesCoordinatorMock(),
			whitelisted:          map[string]struct{}{newEpochKey: {}},
		}
		container := mock.InitConsensusCore()
		container.SetValidatorGroupSelector(coordinator)
		container.SetBootStrapper(&mock.BootstrapperMock{GetNodeStateCalled: func() core.NodeState {
			return nodeState
		}})

		consensusState := initConsensusState()
		ch := make(chan bool, 1)
		sr, err := defaultSubslot(consensusState, ch, container)
		require.Nil(t, err)
		resetCount := 0
		srStartSlotPtr, err := bls.NewSubslotStartSlot(
			sr,
			extend,
			bls.ProcessingThresholdPercent,
			executeStoredMessages,
			func() { resetCount++ },
		)
		require.Nil(t, err)
		srStartSlot := *srStartSlotPtr

		oldKey := consensusState.ConsensusGroup()[0]

		srStartSlot.EpochStartAction(&block.Block{Header: &block.BlockHeader{Epoch: 1}})

		require.True(t, consensusState.IsNodeInConsensusGroup(oldKey))
		require.False(t, consensusState.IsNodeInConsensusGroup(newEpochKey))
		require.Equal(t, 0, resetCount)
	}
}

func TestSubslotStartSlot_NewSubslotStartSlotNilResetConsensusMessagesShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, err := defaultSubslot(consensusState, ch, container)
	require.Nil(t, err)

	// doStartSlotJob and EpochStartAction call it unconditionally, so a nil
	// callback must be rejected at construction instead of panicking later
	srStartSlot, err := bls.NewSubslotStartSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		executeStoredMessages,
		nil,
	)

	require.Nil(t, srStartSlot)
	require.Equal(t, slot.ErrNilResetConsensusMessages, err)
}
