package main

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nodesSetterStub struct {
	setNodesCalled func(elected []sharding.Validator, eligible []sharding.Validator, waiting []sharding.Validator, epoch uint32) error
}

func (s *nodesSetterStub) SetNodes(elected []sharding.Validator, eligible []sharding.Validator, waiting []sharding.Validator, epoch uint32) error {
	if s.setNodesCalled != nil {
		return s.setNodesCalled(elected, eligible, waiting, epoch)
	}
	return nil
}

func serializableValidators(pubKeys ...string) []*sharding.SerializableValidator {
	validators := make([]*sharding.SerializableValidator, 0, len(pubKeys))
	for i, pubKey := range pubKeys {
		validators = append(validators, &sharding.SerializableValidator{
			OwnerAddress: []byte(pubKey),
			PubKey:       []byte(pubKey),
			Index:        uint32(i), // #nosec G115
		})
	}
	return validators
}

func TestRestorePreviousEpochNodes_EpochZeroSkipsWrappedLookup(t *testing.T) {
	t.Parallel()

	// a registry entry under the wrapped key must never be applied at epoch 0
	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"4294967295": {ElectedValidators: serializableValidators("elected0")},
		},
	}
	setNodesCalls := 0
	stub := &nodesSetterStub{
		setNodesCalled: func(_ []sharding.Validator, _ []sharding.Validator, _ []sharding.Validator, _ uint32) error {
			setNodesCalls++
			return nil
		},
	}

	err := restorePreviousEpochNodes(stub, nodeRegistry, 0)

	require.Nil(t, err)
	assert.Equal(t, 0, setNodesCalls)
}

func TestRestorePreviousEpochNodes_MissingPreviousEpochIsNoOp(t *testing.T) {
	t.Parallel()

	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{},
	}
	setNodesCalls := 0
	stub := &nodesSetterStub{
		setNodesCalled: func(_ []sharding.Validator, _ []sharding.Validator, _ []sharding.Validator, _ uint32) error {
			setNodesCalls++
			return nil
		},
	}

	err := restorePreviousEpochNodes(stub, nodeRegistry, 5)

	require.Nil(t, err)
	assert.Equal(t, 0, setNodesCalls)
}

func TestRestorePreviousEpochNodes_RestoresPreviousEpochLists(t *testing.T) {
	t.Parallel()

	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"4": {
				ElectedValidators:  serializableValidators("elected0", "elected1"),
				EligibleValidators: serializableValidators("eligible0"),
				WaitingValidators:  serializableValidators("waiting0"),
			},
		},
	}
	var gotElected, gotEligible, gotWaiting []sharding.Validator
	var gotEpoch uint32
	stub := &nodesSetterStub{
		setNodesCalled: func(elected []sharding.Validator, eligible []sharding.Validator, waiting []sharding.Validator, epoch uint32) error {
			gotElected, gotEligible, gotWaiting, gotEpoch = elected, eligible, waiting, epoch
			return nil
		},
	}

	err := restorePreviousEpochNodes(stub, nodeRegistry, 5)

	require.Nil(t, err)
	assert.Equal(t, uint32(4), gotEpoch)
	require.Equal(t, 2, len(gotElected))
	assert.Equal(t, []byte("elected0"), gotElected[0].PubKey())
	require.Equal(t, 1, len(gotEligible))
	assert.Equal(t, []byte("eligible0"), gotEligible[0].PubKey())
	require.Equal(t, 1, len(gotWaiting))
	assert.Equal(t, []byte("waiting0"), gotWaiting[0].PubKey())
}

func TestRestorePreviousEpochNodes_PropagatesSetNodesError(t *testing.T) {
	t.Parallel()

	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"4": {ElectedValidators: serializableValidators("elected0")},
		},
	}
	expectedErr := errors.New("set nodes failed")
	stub := &nodesSetterStub{
		setNodesCalled: func(_ []sharding.Validator, _ []sharding.Validator, _ []sharding.Validator, _ uint32) error {
			return expectedErr
		},
	}

	err := restorePreviousEpochNodes(stub, nodeRegistry, 5)

	require.Equal(t, expectedErr, err)
}

func TestRegistryValidatorsForEpoch_MissingEpochReturnsDefaults(t *testing.T) {
	t.Parallel()

	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{},
	}
	defaultElected := []sharding.Validator{nil}
	defaultEligible := []sharding.Validator{nil, nil}

	elected, eligible, err := registryValidatorsForEpoch(nodeRegistry, 7, defaultElected, defaultEligible)

	require.Nil(t, err)
	assert.Equal(t, 1, len(elected))
	assert.Equal(t, 2, len(eligible))
}

func TestRegistryValidatorsForEpoch_FoundEpochReturnsRegistryLists(t *testing.T) {
	t.Parallel()

	nodeRegistry := &sharding.NodesCoordinatorRegistry{
		EpochsConfig: map[string]*sharding.EpochValidators{
			"7": {
				ElectedValidators:  serializableValidators("elected0"),
				EligibleValidators: serializableValidators("eligible0", "eligible1"),
			},
		},
	}

	elected, eligible, err := registryValidatorsForEpoch(nodeRegistry, 7, nil, nil)

	require.Nil(t, err)
	require.Equal(t, 1, len(elected))
	assert.Equal(t, []byte("elected0"), elected[0].PubKey())
	require.Equal(t, 2, len(eligible))
	assert.Equal(t, []byte("eligible0"), eligible[0].PubKey())
}
