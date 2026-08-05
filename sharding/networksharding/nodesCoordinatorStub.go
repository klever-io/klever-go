package networksharding

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// NodesCoordinatorStub can not be moved inside mock package as it generates cyclic imports.
// TODO refactor mock package & sharding package & remove this file. Put tests in sharding_test package
type NodesCoordinatorStub struct {
	Validators                              []sharding.Validator
	ConsensusSize                           uint32
	GetSelectedPublicKeysCalled             func(selection []byte, epoch uint32) (publicKeys []string, err error)
	GetValidatorsPublicKeysCalled           func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	GetValidatorsRewardsAddressesCalled     func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	SetNodesCalled                          func(nodes []sharding.Validator, epoch uint32) error
	ComputeValidatorsGroupCalled            func(randomness []byte, slot uint64, epoch uint32) (validatorsGroup []sharding.Validator, err error)
	GetValidatorWithPublicKeyCalled         func(publicKey []byte) (validator sharding.Validator, err error)
	GetAllElectedValidatorsKeysCalled       func() ([][]byte, error)
	GetAllEligibleValidatorsKeysCalled      func() ([][]byte, error)
	GetAllWaitingValidatorsKeysCalled       func() ([][]byte, error)
	GetAllLeavingValidatorsPublicKeysCalled func() ([][]byte, error)
	CheckValidatorSlotCalled                func(epoch uint32, slotIndex int64, pubkey []byte) bool
	ConsensusGroupSizeCalled                func() int
	LoadValidatorsCalled                    func(validators []*state.ValidatorInfo) error
	SetEpochValidatorsInfoCalled            func(epoch uint32, validatorsInfo []*state.ValidatorInfo) error
	GetConsensusWhitelistedNodesCalled      func(epoch uint32) (map[string]struct{}, error)
	GetOwnPublicKeyCalled                   func() []byte
	GetSavedStateKeyCalled                  func() []byte
	LoadStateCalled                         func(key []byte) error
}

// GetChance -
func (ncm *NodesCoordinatorStub) GetChance(uint32) uint32 {
	return 1
}

// GetAllElectedValidatorsKeys -
func (ncm *NodesCoordinatorStub) GetAllElectedValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	if ncm.GetAllElectedValidatorsKeysCalled != nil {
		return ncm.GetAllElectedValidatorsKeysCalled()
	}
	return nil, nil
}

// GetAllEligibleValidatorsKeys -
func (ncm *NodesCoordinatorStub) GetAllEligibleValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	if ncm.GetAllEligibleValidatorsKeysCalled != nil {
		return ncm.GetAllEligibleValidatorsKeysCalled()
	}
	return nil, nil
}

// GetAllWaitingValidatorsKeys -
func (ncm *NodesCoordinatorStub) GetAllWaitingValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	if ncm.GetAllWaitingValidatorsKeysCalled != nil {
		return ncm.GetAllWaitingValidatorsKeysCalled()
	}
	return nil, nil
}

// GetValidatorWithPublicKey -
func (ncm *NodesCoordinatorStub) GetValidatorWithPublicKey(publicKey []byte) (sharding.Validator, error) {
	if ncm.GetValidatorWithPublicKeyCalled != nil {
		return ncm.GetValidatorWithPublicKeyCalled(publicKey)
	}
	return nil, nil
}

// ComputeConsensusGroup -
func (ncm *NodesCoordinatorStub) ComputeConsensusGroup(
	randomess []byte,
	slot uint64,
	epoch uint32,
) ([]sharding.Validator, error) {
	if ncm.ComputeValidatorsGroupCalled != nil {
		return ncm.ComputeValidatorsGroupCalled(randomess, slot, epoch)
	}
	return nil, nil
}

// GetConsensusValidatorsPublicKeys -
func (ncm *NodesCoordinatorStub) GetConsensusValidatorsPublicKeys(
	randomness []byte,
	slot uint64,
	epoch uint32,
) ([]string, error) {
	if ncm.GetValidatorsPublicKeysCalled != nil {
		return ncm.GetValidatorsPublicKeysCalled(randomness, slot, epoch)
	}

	return nil, nil
}

// ConsensusGroupSize -
func (ncm *NodesCoordinatorStub) ConsensusGroupSize() int {
	if ncm.ConsensusGroupSizeCalled != nil {
		return ncm.ConsensusGroupSizeCalled()
	}
	return 1
}

// CheckValidatorSlot -
func (ncm *NodesCoordinatorStub) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	if ncm.CheckValidatorSlotCalled != nil {
		return ncm.CheckValidatorSlotCalled(epoch, slotIndex, pubkey)
	}
	return true
}

// GetConsensusWhitelistedNodes return the whitelisted nodes allowed to send consensus messages, for each of the shards
func (ncm *NodesCoordinatorStub) GetConsensusWhitelistedNodes(epoch uint32) (map[string]struct{}, error) {
	if ncm.GetConsensusWhitelistedNodesCalled != nil {
		return ncm.GetConsensusWhitelistedNodesCalled(epoch)
	}
	return nil, nil
}

// GetOwnPublicKey -
func (ncm *NodesCoordinatorStub) GetOwnPublicKey() []byte {
	if ncm.GetOwnPublicKeyCalled != nil {
		return ncm.GetOwnPublicKeyCalled()
	}
	return nil
}

// GetSavedStateKey -
func (ncm *NodesCoordinatorStub) GetSavedStateKey() []byte {
	if ncm.GetSavedStateKeyCalled != nil {
		return ncm.GetSavedStateKeyCalled()
	}
	return nil
}

// LoadState -
func (ncm *NodesCoordinatorStub) LoadState(key []byte) error {
	if ncm.LoadStateCalled != nil {
		return ncm.LoadStateCalled(key)
	}
	return nil
}

// IsReady -
func (ncm *NodesCoordinatorStub) IsReady() bool {
	return true
}

// SetEpochValidatorsInfo
func (ncm *NodesCoordinatorStub) SetEpochValidatorsInfo(epoch uint32, validatorsInfo []*state.ValidatorInfo) error {
	if ncm.SetEpochValidatorsInfoCalled != nil {
		return ncm.SetEpochValidatorsInfoCalled(epoch, validatorsInfo)
	}
	return nil
}

// IsInterfaceNil -
func (ncs *NodesCoordinatorStub) IsInterfaceNil() bool {
	return ncs == nil
}
