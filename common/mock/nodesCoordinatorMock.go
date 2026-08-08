package mock

import (
	"bytes"
	"fmt"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// NodesCoordinatorMock defines the behaviour of a struct able to do validator group selection
type NodesCoordinatorMock struct {
	Validators                          []sharding.Validator
	ConsensusSize                       uint32
	GetSelectedPublicKeysCalled         func(selection []byte, epoch uint32) (publicKeys []string, err error)
	GetValidatorsPublicKeysCalled       func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	GetValidatorsRewardsAddressesCalled func(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	SetNodesCalled                      func(nodes []sharding.Validator, epoch uint32) error
	ComputeValidatorsGroupCalled        func(randomness []byte, slot uint64, epoch uint32) (validatorsGroup []sharding.Validator, err error)
	GetValidatorWithPublicKeyCalled     func(publicKey []byte) (validator sharding.Validator, err error)
	GetAllElectedValidatorsKeysCalled   func() ([][]byte, error)
	// GetAllElectedValidatorsKeysWithEpochCalled takes precedence over
	// GetAllElectedValidatorsKeysCalled when both are set
	GetAllElectedValidatorsKeysWithEpochCalled func(epoch uint32) ([][]byte, error)
	GetAllEligibleValidatorsKeysCalled         func() ([][]byte, error)
	GetAllWaitingValidatorsKeysCalled          func() ([][]byte, error)
	GetAllLeavingValidatorsPublicKeysCalled    func() ([][]byte, error)
	CheckValidatorSlotCalled                   func(epoch uint32, slotIndex int64, pubkey []byte) bool
	ConsensusGroupSizeCalled                   func() int
	LoadValidatorsCalled                       func(validators []*state.ValidatorInfo) error
	SetEpochValidatorsInfoCalled               func(epoch uint32, validatorsInfo []*state.ValidatorInfo) error
	IsReadyCalled                              func() bool
	LoadStateFailedValue                       bool
}

// NewNodesCoordinatorMock -
func NewNodesCoordinatorMock() *NodesCoordinatorMock {
	nodes := 2

	validatorsList := make([]sharding.Validator, nodes)
	for v := 0; v < nodes; v++ {
		validatorsList[v], _ = sharding.NewValidator(
			[]byte(fmt.Sprintf("owner%d", v)),
			[]byte(fmt.Sprintf("pubKey%d", v)),
			1,
			uint32(v), // #nosec G115
		)
	}

	return &NodesCoordinatorMock{
		ConsensusSize: 1,
		Validators:    validatorsList,
	}
}

// GetChance -
func (ncm *NodesCoordinatorMock) GetChance(uint32) uint32 {
	return 1
}

// GetNumTotalEligible -
func (ncm *NodesCoordinatorMock) GetNumTotalEligible() uint64 {
	return 1
}

// GetAllElectedValidatorsKeys -
func (ncm *NodesCoordinatorMock) GetAllElectedValidatorsKeys(epoch uint32, _ bool) ([][]byte, error) {
	if ncm.GetAllElectedValidatorsKeysWithEpochCalled != nil {
		return ncm.GetAllElectedValidatorsKeysWithEpochCalled(epoch)
	}
	if ncm.GetAllElectedValidatorsKeysCalled != nil {
		return ncm.GetAllElectedValidatorsKeysCalled()
	}
	return nil, nil
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
	if ncm.GetAllWaitingValidatorsKeysCalled != nil {
		return ncm.GetAllWaitingValidatorsKeysCalled()
	}
	return nil, nil
}

// CheckValidatorSlot -
func (ncm *NodesCoordinatorMock) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	if ncm.CheckValidatorSlotCalled != nil {
		return ncm.CheckValidatorSlotCalled(epoch, slotIndex, pubkey)
	}
	return true
}

// GetValidatorsIndexes -
func (ncm *NodesCoordinatorMock) GetValidatorsIndexes(_ []string, _ uint32) ([]uint64, error) {
	return nil, nil
}

// GetSelectedPublicKeys -
func (ncm *NodesCoordinatorMock) GetSelectedPublicKeys(selection []byte, epoch uint32) (publicKeys []string, err error) {
	if ncm.GetSelectedPublicKeysCalled != nil {
		return ncm.GetSelectedPublicKeysCalled(selection, epoch)
	}

	if len(ncm.Validators) == 0 {
		return nil, sharding.ErrNilInputNodesMap
	}

	pubKeys := make([]string, 0)

	for _, v := range ncm.Validators {
		pubKeys = append(pubKeys, string(v.PubKey()))
	}

	return pubKeys, nil
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

	valGrStr := make([]string, 0)

	for _, v := range validators {
		valGrStr = append(valGrStr, string(v.PubKey()))
	}

	return valGrStr, nil
}

// SetNodesPerShards -
func (ncm *NodesCoordinatorMock) SetNodesPerShards(
	eligible []sharding.Validator,
	_ []sharding.Validator,
	epoch uint32,
) error {
	if ncm.SetNodesCalled != nil {
		return ncm.SetNodesCalled(eligible, epoch)
	}

	if eligible == nil {
		return sharding.ErrNilInputNodesMap
	}

	ncm.Validators = eligible

	return nil
}

// ComputeAdditionalLeaving -
func (ncm *NodesCoordinatorMock) ComputeAdditionalLeaving([]*state.ValidatorInfo) ([]sharding.Validator, error) {
	return make([]sharding.Validator, 0), nil
}

// ComputeConsensusGroup -
func (ncm *NodesCoordinatorMock) ComputeConsensusGroup(
	randomess []byte,
	slot uint64,
	epoch uint32,
) ([]sharding.Validator, error) {

	if ncm.ComputeValidatorsGroupCalled != nil {
		return ncm.ComputeValidatorsGroupCalled(randomess, slot, epoch)
	}

	if randomess == nil {
		return nil, sharding.ErrNilRandomness
	}

	validatorsGroup := make([]sharding.Validator, 0)

	for i := uint32(0); i < ncm.ConsensusSize; i++ {
		validatorsGroup = append(validatorsGroup, ncm.Validators[i])
	}

	return validatorsGroup, nil
}

// ConsensusGroupSize -
func (ncm *NodesCoordinatorMock) ConsensusGroupSize() int {
	if ncm.ConsensusGroupSizeCalled != nil {
		return ncm.ConsensusGroupSizeCalled()
	}
	return 1
}

// GetValidatorWithPublicKey -
func (ncm *NodesCoordinatorMock) GetValidatorWithPublicKey(publicKey []byte) (sharding.Validator, error) {
	if ncm.GetValidatorWithPublicKeyCalled != nil {
		return ncm.GetValidatorWithPublicKeyCalled(publicKey)
	}

	if publicKey == nil {
		return nil, sharding.ErrNilPubKey
	}

	for i := 0; i < len(ncm.Validators); i++ {
		if bytes.Equal(publicKey, ncm.Validators[i].PubKey()) {
			return ncm.Validators[i], nil
		}
	}

	return nil, sharding.ErrValidatorNotFound
}

// GetAllLeavingValidatorsPublicKeys -
func (ncm *NodesCoordinatorMock) GetAllLeavingValidatorsPublicKeys(_ uint32) ([][]byte, error) {
	return nil, nil
}

// GetOwnPublicKey -
func (ncm *NodesCoordinatorMock) GetOwnPublicKey() []byte {
	return []byte("key")
}

// LoadState -
func (ncm *NodesCoordinatorMock) LoadState(_ []byte) error {
	return nil
}

// IsReady -
func (ncm *NodesCoordinatorMock) IsReady() bool {
	if ncm.IsReadyCalled != nil {
		return ncm.IsReadyCalled()
	}
	return true
}

// LoadStateFailed -
func (ncm *NodesCoordinatorMock) LoadStateFailed() bool {
	return ncm.LoadStateFailedValue
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
) (map[string]struct{}, error) {
	panic("not implemented")
}

// ValidatorsWeights -
func (ncm *NodesCoordinatorMock) ValidatorsWeights(validators []sharding.Validator) ([]uint32, error) {
	weights := make([]uint32, len(validators))
	for i := range validators {
		weights[i] = 1
	}

	return weights, nil
}

func (ncm *NodesCoordinatorMock) LoadValidators(validators []*state.ValidatorInfo) error {

	nodes := len(validators)

	validatorsList := make([]sharding.Validator, nodes)
	for v := 0; v < nodes; v++ {
		validatorsList[v], _ = sharding.NewValidator(
			validators[v].OwnerAddress,
			validators[v].PublicKey,
			1,
			uint32(v), // #nosec G115
		)
	}

	ncm.Validators = validatorsList

	return nil
}

func (ncm *NodesCoordinatorMock) SetEpochValidatorsInfo(epoch uint32, validatorsInfo []*state.ValidatorInfo) error {
	return ncm.SetEpochValidatorsInfoCalled(epoch, validatorsInfo)
}

// IsInterfaceNil -
func (ncm *NodesCoordinatorMock) IsInterfaceNil() bool {
	return ncm == nil
}
