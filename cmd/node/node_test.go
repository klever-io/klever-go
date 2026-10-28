package main

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEpochNodesSetter struct {
	called   bool
	err      error
	elected  []sharding.Validator
	eligible []sharding.Validator
	waiting  []sharding.Validator
	leaving  []sharding.Validator
	epoch    uint32
}

func (rens *recordingEpochNodesSetter) SetNodes(
	elected, eligible, waiting, leaving []sharding.Validator,
	epoch uint32,
) error {
	rens.called = true
	rens.elected = elected
	rens.eligible = eligible
	rens.waiting = waiting
	rens.leaving = leaving
	rens.epoch = epoch

	return rens.err
}

func TestSeedPreviousEpochFromRegistry_RestoresEveryValidatorList(t *testing.T) {
	t.Parallel()

	registry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"6": {
				ElectedValidators:  []*sharding.SerializableValidator{serializableValidator("elected", 1)},
				EligibleValidators: []*sharding.SerializableValidator{serializableValidator("eligible", 2)},
				WaitingValidators:  []*sharding.SerializableValidator{serializableValidator("waiting", 3)},
				LeavingValidators:  []*sharding.SerializableValidator{serializableValidator("leaving", 4)},
			},
		},
	}
	setter := &recordingEpochNodesSetter{}

	err := seedPreviousEpochFromRegistry(setter, registry, 7)

	require.NoError(t, err)
	require.True(t, setter.called)
	assert.Equal(t, uint32(6), setter.epoch)
	require.Len(t, setter.elected, 1)
	require.Len(t, setter.eligible, 1)
	require.Len(t, setter.waiting, 1)
	require.Len(t, setter.leaving, 1)
	assert.Equal(t, []byte("elected"), setter.elected[0].PubKey())
	assert.Equal(t, []byte("eligible"), setter.eligible[0].PubKey())
	assert.Equal(t, []byte("waiting"), setter.waiting[0].PubKey())
	assert.Equal(t, []byte("leaving"), setter.leaving[0].PubKey())
}

func TestSeedPreviousEpochFromRegistry_AtEpochZeroDoesNotUnderflow(t *testing.T) {
	t.Parallel()

	registry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"4294967295": {
				ElectedValidators:  []*sharding.SerializableValidator{serializableValidator("wrapped", 1)},
				EligibleValidators: []*sharding.SerializableValidator{},
			},
		},
	}
	setter := &recordingEpochNodesSetter{}

	err := seedPreviousEpochFromRegistry(setter, registry, 0)

	require.NoError(t, err)
	assert.False(t, setter.called)
}

func TestRegistryEpochValidators_AcceptsLegacyEntryWithoutLeavingList(t *testing.T) {
	t.Parallel()

	epochConfig := &sharding.EpochValidators{
		ElectedValidators:  []*sharding.SerializableValidator{serializableValidator("elected", 1)},
		EligibleValidators: []*sharding.SerializableValidator{serializableValidator("eligible", 2)},
		WaitingValidators:  []*sharding.SerializableValidator{serializableValidator("waiting", 3)},
	}

	elected, eligible, waiting, leaving, err := registryEpochValidators(epochConfig)

	require.NoError(t, err)
	assert.Len(t, elected, 1)
	assert.Len(t, eligible, 1)
	assert.Len(t, waiting, 1)
	assert.Empty(t, leaving)
}

func TestRegistryEpochValidators_RejectsNullEntry(t *testing.T) {
	t.Parallel()

	_, _, _, _, err := registryEpochValidators(nil)

	require.EqualError(t, err, "nil registry epoch validators")
}

func TestSeedPreviousEpochFromRegistry_LeavingNilPubKeyFailsBeforeSetNodes(t *testing.T) {
	t.Parallel()

	leaving := serializableValidator("leaving", 4)
	leaving.PubKey = nil
	registry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"6": {
				ElectedValidators: []*sharding.SerializableValidator{serializableValidator("elected", 1)},
				LeavingValidators: []*sharding.SerializableValidator{leaving},
			},
		},
	}
	setter := &recordingEpochNodesSetter{}

	err := seedPreviousEpochFromRegistry(setter, registry, 7)

	require.ErrorIs(t, err, sharding.ErrNilPubKey)
	assert.False(t, setter.called)
}

func TestSeedPreviousEpochFromRegistry_PropagatesSetNodesError(t *testing.T) {
	t.Parallel()

	setErr := errors.New("set nodes failed")
	registry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"6": {
				ElectedValidators: []*sharding.SerializableValidator{serializableValidator("elected", 1)},
			},
		},
	}
	setter := &recordingEpochNodesSetter{err: setErr}

	err := seedPreviousEpochFromRegistry(setter, registry, 7)

	require.ErrorIs(t, err, setErr)
	assert.True(t, setter.called)
}

func serializableValidator(publicKey string, index uint32) *sharding.SerializableValidator {
	return &sharding.SerializableValidator{
		OwnerAddress: []byte("owner-" + publicKey),
		PubKey:       []byte(publicKey),
		Index:        index,
	}
}
