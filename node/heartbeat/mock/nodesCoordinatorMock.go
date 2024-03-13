package mock

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// NodesCoordinatorMock -
type NodesCoordinatorMock struct {
	ComputeValidatorsGroupCalled        func(randomness []byte, slot uint64, epoch uint32) ([]sharding.Validator, error)
	GetValidatorsPublicKeysCalled       func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	GetValidatorsRewardsAddressesCalled func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	GetAllEligibleValidatorsKeysCalled  func() ([][]byte, error)
}

// GetAllEligibleValidatorsKeys -
func (ncm *NodesCoordinatorMock) GetAllEligibleValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	if ncm.GetAllEligibleValidatorsKeysCalled != nil {
		return ncm.GetAllEligibleValidatorsKeysCalled()
	}
	return nil, nil
}

// GetAllWaitingValidatorsKeys -
func (ncm *NodesCoordinatorMock) GetAllWaitingValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	return nil, nil
}

// CheckValidatorSlot -
func (ncm *NodesCoordinatorMock) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	return false
}

// ComputeConsensusGroup -
func (ncm *NodesCoordinatorMock) ComputeConsensusGroup(
	randomness []byte,
	slot uint64,
	epoch uint32,
) (validatorsGroup []sharding.Validator, err error) {

	if ncm.ComputeValidatorsGroupCalled != nil {
		return ncm.ComputeValidatorsGroupCalled(randomness, slot, epoch)
	}

	list := []sharding.Validator{
		NewValidatorMock([]byte("A"), []byte("A"), 1, 1),
		NewValidatorMock([]byte("B"), []byte("B"), 1, 1),
		NewValidatorMock([]byte("C"), []byte("C"), 1, 1),
		NewValidatorMock([]byte("D"), []byte("D"), 1, 1),
		NewValidatorMock([]byte("E"), []byte("E"), 1, 1),
		NewValidatorMock([]byte("F"), []byte("F"), 1, 1),
		NewValidatorMock([]byte("G"), []byte("G"), 1, 1),
		NewValidatorMock([]byte("H"), []byte("H"), 1, 1),
		NewValidatorMock([]byte("I"), []byte("I"), 1, 1),
	}

	return list, nil
}

// GetNumTotalEligible -
func (ncm *NodesCoordinatorMock) GetNumTotalEligible() uint64 {
	return 1
}

// ConsensusGroupSize -
func (ncm *NodesCoordinatorMock) ConsensusGroupSize() int {
	return 1
}

// GetConsensusValidatorsPublicKeys -
func (ncm *NodesCoordinatorMock) GetConsensusValidatorsPublicKeys(
	randomness []byte,
	slot uint64,
	epoch uint32,
) ([]string, error) {
	if ncm.GetValidatorsPublicKeysCalled != nil {
		return ncm.GetValidatorsPublicKeysCalled(randomness, slot, epoch)
	}

	validators, err := ncm.ComputeConsensusGroup(randomness, slot, epoch)
	if err != nil {
		return nil, err
	}

	pubKeys := make([]string, 0)

	for _, v := range validators {
		pubKeys = append(pubKeys, string(v.PubKey()))
	}

	return pubKeys, nil
}

// SetNodesPerShards -
func (ncm *NodesCoordinatorMock) SetNodesPerShards(
	_ map[uint32][]sharding.Validator,
	_ map[uint32][]sharding.Validator,
	_ uint32,
) error {
	return nil
}

// ComputeLeaving -
func (ncm *NodesCoordinatorMock) ComputeLeaving(_ []*state.ValidatorInfo) ([]sharding.Validator, error) {
	return make([]sharding.Validator, 0), nil
}

// ComputeAdditionalLeaving -
func (ncm *NodesCoordinatorMock) ComputeAdditionalLeaving(_ []*state.ValidatorInfo) (map[uint32][]sharding.Validator, error) {
	return nil, nil
}

// LoadState -
func (ncm *NodesCoordinatorMock) LoadState(_ []byte) error {
	return nil
}

// GetSavedStateKey -
func (ncm *NodesCoordinatorMock) GetSavedStateKey() []byte {
	return []byte("key")
}

// ShuffleOutForEpoch verifies if the shards changed in the new epoch and calls the shuffleOutHandler
func (ncm *NodesCoordinatorMock) ShuffleOutForEpoch(_ uint32) {
	panic("not implemented")
}

// GetConsensusWhitelistedNodes return the whitelisted nodes allowed to send consensus messages, for each of the shards
func (ncm *NodesCoordinatorMock) GetConsensusWhitelistedNodes(
	_ uint32,
) (struct{}, error) {
	panic("not implemented")
}

// SetConsensusGroupSize -
func (ncm *NodesCoordinatorMock) SetConsensusGroupSize(_ int) error {
	panic("implement me")
}

// GetSelectedPublicKeys -
func (ncm *NodesCoordinatorMock) GetSelectedPublicKeys(_ []byte, _ uint32, _ uint32) (publicKeys []string, err error) {
	panic("implement me")
}

// GetValidatorWithPublicKey -
func (ncm *NodesCoordinatorMock) GetValidatorWithPublicKey(_ []byte) (sharding.Validator, error) {
	panic("implement me")
}

// GetValidatorsIndexes -
func (ncm *NodesCoordinatorMock) GetValidatorsIndexes(_ []string, _ uint32) ([]uint64, error) {
	panic("implement me")
}

// GetOwnPublicKey -
func (ncm *NodesCoordinatorMock) GetOwnPublicKey() []byte {
	panic("implement me")
}

// ValidatorsWeights -
func (ncm *NodesCoordinatorMock) ValidatorsWeights(validators []sharding.Validator) ([]uint32, error) {
	weights := make([]uint32, len(validators))
	for i := range validators {
		weights[i] = 1
	}

	return weights, nil
}

// GetChance -
func (ncm *NodesCoordinatorMock) GetChance(uint32) uint32 {
	return 1
}

// IsInterfaceNil returns true if there is no value under the interface
func (ncm *NodesCoordinatorMock) IsInterfaceNil() bool {
	return ncm == nil
}
