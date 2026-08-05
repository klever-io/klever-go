package networksharding_test

import (
	"github.com/klever-io/klever-go/common"
	state "github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

type nodesCoordinatorStub struct {
	GetValidatorsPublicKeysCalled   func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	GetValidatorWithPublicKeyCalled func(publicKey []byte) (validator sharding.Validator, err error)
}

// GetChance -
func (ncm *nodesCoordinatorStub) GetChance(uint32) uint32 {
	return 1
}

// GetAllLeavingValidatorsKeys -
func (ncs *nodesCoordinatorStub) GetAllLeavingValidatorsKeys(_ uint32) ([][]byte, error) {
	return nil, nil
}

// GetAllElectedValidatorsKeys -
func (ncs *nodesCoordinatorStub) GetAllElectedValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	return nil, nil
}

func (ncs *nodesCoordinatorStub) LoadValidators(validators []*state.ValidatorInfo) error {
	return nil
}

// ComputeAdditionalLeaving -
func (ncs *nodesCoordinatorStub) ComputeAdditionalLeaving(_ []*state.ValidatorInfo) ([]sharding.Validator, error) {
	panic("implement me")
}

// GetAllEligibleValidatorsKeys -
func (ncs *nodesCoordinatorStub) GetAllEligibleValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	panic("implement me")
}

// GetAllWaitingValidatorsKeys -
func (ncs *nodesCoordinatorStub) GetAllWaitingValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	panic("implement me")
}

// CheckValidatorSlot -
func (ncs *nodesCoordinatorStub) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	panic("implement me")
}

// GetNumTotalEligible -
func (ncs *nodesCoordinatorStub) GetNumTotalEligible() uint64 {
	panic("implement me")
}

// GetValidatorsIndexes -
func (ncs *nodesCoordinatorStub) GetValidatorsIndexes(_ []string, _ uint32) ([]uint64, error) {
	panic("implement me")
}

// GetConsensusValidatorsKeys -
func (ncs *nodesCoordinatorStub) GetConsensusValidatorsKeys(_ []byte, _ uint64, _ uint32) ([]string, error) {
	panic("implement me")
}

// GetConsensusValidatorsRewardsAddresses -
func (ncs *nodesCoordinatorStub) GetConsensusValidatorsRewardsAddresses(_ []byte, _ uint64, _ uint32, _ uint32) ([]string, error) {
	panic("implement me")
}

// GetOwnPublicKey -
func (ncs *nodesCoordinatorStub) GetOwnPublicKey() []byte {
	panic("implement me")
}

// SetNodesPerShards -
func (ncs *nodesCoordinatorStub) SetNodesPerShards(_ []sharding.Validator, _ []sharding.Validator, _ []sharding.Validator, _ uint32) error {
	panic("implement me")
}

// ComputeConsensusGroup -
func (ncs *nodesCoordinatorStub) ComputeConsensusGroup(_ []byte, _ uint64, _ uint32) (validatorsGroup []sharding.Validator, err error) {
	panic("implement me")
}

// LoadState -
func (ncs *nodesCoordinatorStub) LoadState(_ []byte) error {
	panic("implement me")
}

// IsReady -
func (ncs *nodesCoordinatorStub) IsReady() bool {
	return true
}

// GetSavedStateKey -
func (ncs *nodesCoordinatorStub) GetSavedStateKey() []byte {
	panic("implement me")
}

// ShuffleOutForEpoch verifies if the shards changed in the new epoch and calls the shuffleOutHandler
func (ncm *nodesCoordinatorStub) ShuffleOutForEpoch(_ uint32) {
	panic("not implemented")
}

// GetConsensusWhitelistedNodes -
func (ncs *nodesCoordinatorStub) GetConsensusWhitelistedNodes(_ uint32) (map[string]struct{}, error) {
	panic("implement me")
}

// ConsensusGroupSize -
func (ncs *nodesCoordinatorStub) ConsensusGroupSize() int {
	panic("implement me")
}

// GetValidatorWithPublicKey -
func (ncs *nodesCoordinatorStub) GetValidatorWithPublicKey(publicKey []byte) (sharding.Validator, error) {
	if ncs.GetValidatorWithPublicKeyCalled != nil {
		return ncs.GetValidatorWithPublicKeyCalled(publicKey)
	}

	return nil, common.ErrValidatorNotFound
}

// ValidatorsWeights -
func (ncs *nodesCoordinatorStub) ValidatorsWeights(validators []sharding.Validator) ([]uint32, error) {
	weights := make([]uint32, len(validators))
	for i := range validators {
		weights[i] = 1
	}

	return weights, nil
}

// GetConsensusValidatorsPublicKeys -
func (ncm *nodesCoordinatorStub) GetConsensusValidatorsPublicKeys(
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

// SetEpochValidatorsInfo -
func (ncs *nodesCoordinatorStub) SetEpochValidatorsInfo(_ uint32, _ []*state.ValidatorInfo) error {
	return nil
}

// IsInterfaceNil -
func (ncs *nodesCoordinatorStub) IsInterfaceNil() bool {
	return ncs == nil
}
