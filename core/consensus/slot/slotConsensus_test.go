package slot_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/stretchr/testify/assert"
)

func initSlotConsensus() *slot.SlotConsensus {
	pubKeys := []string{"1", "2", "3"}
	eligibleNodes := make(map[string]struct{})

	for i := range pubKeys {
		eligibleNodes[pubKeys[i]] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(
		eligibleNodes,
		len(eligibleNodes),
		"2")

	rcns.SetConsensusGroup(pubKeys)

	rcns.ResetSlotState()

	return slot.NewSlotConsensusWrapper(rcns)
}

func TestSlotConsensus_NewSlotConsensusShouldWork(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	assert.NotNil(t, rcns)
	assert.Equal(t, 3, len(rcns.ConsensusGroup()))
	assert.Equal(t, "3", rcns.ConsensusGroup()[2])
	assert.Equal(t, "2", rcns.SelfPubKey())
}

func TestSlotConsensus_ConsensusGroupIndexFound(t *testing.T) {
	t.Parallel()

	pubKeys := []string{"key1", "key2", "key3"}
	eligibleNodes := make(map[string]struct{})

	for i := range pubKeys {
		eligibleNodes[pubKeys[i]] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(eligibleNodes, 3, "key3")
	rcns.SetConsensusGroup(pubKeys)
	index, err := rcns.ConsensusGroupIndex("key3")

	assert.Equal(t, 2, index)
	assert.Nil(t, err)
}

func TestSlotConsensus_ConsensusGroupIndexNotFound(t *testing.T) {
	t.Parallel()

	pubKeys := []string{"key1", "key2", "key3"}
	eligibleNodes := make(map[string]struct{})

	for i := range pubKeys {
		eligibleNodes[pubKeys[i]] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(eligibleNodes, 3, "key4")
	rcns.SetConsensusGroup(pubKeys)
	index, err := rcns.ConsensusGroupIndex("key4")

	assert.Zero(t, index)
	assert.Equal(t, slot.ErrNotFoundInConsensus, err)
}

func TestSlotConsensus_IndexSelfConsensusGroupInConsesus(t *testing.T) {
	t.Parallel()

	pubKeys := []string{"key1", "key2", "key3"}
	eligibleNodes := make(map[string]struct{})

	for i := range pubKeys {
		eligibleNodes[pubKeys[i]] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(eligibleNodes, 3, "key2")
	rcns.SetConsensusGroup(pubKeys)
	index, err := rcns.SelfConsensusGroupIndex()

	assert.Equal(t, 1, index)
	assert.Nil(t, err)
}

func TestSlotConsensus_IndexSelfConsensusGroupNotFound(t *testing.T) {
	t.Parallel()

	pubKeys := []string{"key1", "key2", "key3"}
	eligibleNodes := make(map[string]struct{})

	for i := range pubKeys {
		eligibleNodes[pubKeys[i]] = struct{}{}
	}

	rcns := slot.NewSlotConsensus(eligibleNodes, 3, "key4")
	rcns.SetConsensusGroup(pubKeys)
	index, err := rcns.SelfConsensusGroupIndex()

	assert.Zero(t, index)
	assert.Equal(t, slot.ErrNotFoundInConsensus, err)
}

func TestSlotConsensus_SetConsensusGroupShouldChangeTheConsensusGroup(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	rcns.SetConsensusGroup([]string{"4", "5", "6"})

	assert.Equal(t, "4", rcns.ConsensusGroup()[0])
	assert.Equal(t, "5", rcns.ConsensusGroup()[1])
	assert.Equal(t, "6", rcns.ConsensusGroup()[2])
}

func TestSlotConsensus_SetConsensusGroupSizeShouldChangeTheConsensusGroupSize(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	assert.Equal(t, len(rcns.ConsensusGroup()), rcns.ConsensusGroupSize())
	rcns.SetConsensusGroupSize(99999)
	assert.Equal(t, 99999, rcns.ConsensusGroupSize())
}

func TestSlotConsensus_SetSelfPubKeyShouldChangeTheSelfPubKey(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	rcns.SetSelfPubKey("X")
	assert.Equal(t, "X", rcns.SelfPubKey())
}

func TestSlotConsensus_GetJobDoneShouldReturnsFalseWhenValidatorIsNotInTheConsensusGroup(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetJobDone("3", bls.SrBlock, true)
	rcns.SetConsensusGroup([]string{"1", "2"})
	isJobDone, _ := rcns.JobDone("3", bls.SrBlock)
	assert.False(t, isJobDone)
}

func TestSlotConsensus_SetJobDoneShouldNotBeSetWhenValidatorIsNotInTheConsensusGroup(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetJobDone("4", bls.SrBlock, true)
	isJobDone, _ := rcns.JobDone("4", bls.SrBlock)
	assert.False(t, isJobDone)
}

func TestSlotConsensus_GetSelfJobDoneShouldReturnFalse(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	for i := 0; i < len(rcns.ConsensusGroup()); i++ {
		if rcns.ConsensusGroup()[i] == rcns.SelfPubKey() {
			continue
		}

		_ = rcns.SetJobDone(rcns.ConsensusGroup()[i], bls.SrBlock, true)
	}

	jobDone, _ := rcns.SelfJobDone(bls.SrBlock)
	assert.False(t, jobDone)
}

func TestSlotConsensus_GetSelfJobDoneShouldReturnTrue(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetJobDone("2", bls.SrBlock, true)

	jobDone, _ := rcns.SelfJobDone(bls.SrBlock)
	assert.True(t, jobDone)
}

func TestSlotConsensus_SetSelfJobDoneShouldWork(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetSelfJobDone(bls.SrBlock, true)

	jobDone, _ := rcns.JobDone("2", bls.SrBlock)
	assert.True(t, jobDone)
}

func TestSlotConsensus_IsNodeInConsensusGroup(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	assert.Equal(t, false, rcns.IsNodeInConsensusGroup("4"))
	assert.Equal(t, true, rcns.IsNodeInConsensusGroup(rcns.SelfPubKey()))
}

func TestSlotConsensus_ComputeSize(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetJobDone("1", bls.SrBlock, true)
	assert.Equal(t, 1, rcns.ComputeSize(bls.SrBlock))
}

func TestSlotConsensus_ResetValidationMap(t *testing.T) {
	t.Parallel()

	rcns := *initSlotConsensus()

	_ = rcns.SetJobDone("1", bls.SrBlock, true)
	jobDone, _ := rcns.JobDone("1", bls.SrBlock)
	assert.Equal(t, true, jobDone)

	rcns.ConsensusGroup()[1] = "X"

	rcns.ResetSlotState()

	jobDone, err := rcns.JobDone("1", bls.SrBlock)
	assert.Equal(t, false, jobDone)
	assert.Nil(t, err)
}
