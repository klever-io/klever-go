package disabled

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// nodesCoordinator -
type nodesCoordinator struct {
}

// NewNodesCoordinator returns a new instance of nodesCoordinator
func NewNodesCoordinator() *nodesCoordinator {
	return &nodesCoordinator{}
}

// GetChance -
func (n *nodesCoordinator) GetChance(uint32) uint32 {
	return 1
}

// ValidatorsWeights -
func (n *nodesCoordinator) ValidatorsWeights(validators []sharding.Validator) ([]uint32, error) {
	return make([]uint32, len(validators)), nil
}

// GetValidatorsIndexes -
func (n *nodesCoordinator) GetValidatorsIndexes(_ []string, _ uint32) ([]uint64, error) {
	return nil, nil
}

// GetAllElectedValidatorsKeys -
func (n *nodesCoordinator) GetAllElectedValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	return nil, nil
}

// GetAllEligibleValidatorsKeys -
func (n *nodesCoordinator) GetAllEligibleValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	return nil, nil
}

// GetAllWaitingValidatorsKeys -
func (n *nodesCoordinator) GetAllWaitingValidatorsKeys(_ uint32, _ bool) ([][]byte, error) {
	return nil, nil
}

// CheckValidatorSlot -
func (n *nodesCoordinator) CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool {
	return false
}

// GetConsensusValidatorsPublicKeys -
func (n *nodesCoordinator) GetConsensusValidatorsPublicKeys(_ []byte, _ uint64, _ uint32) ([]string, error) {
	return nil, nil
}

// GetOwnPublicKey -
func (n *nodesCoordinator) GetOwnPublicKey() []byte {
	return nil
}

// ComputeConsensusGroup -
func (n *nodesCoordinator) ComputeConsensusGroup(_ []byte, _ uint64, _ uint32) (validatorsGroup []sharding.Validator, err error) {
	return nil, nil
}

// GetValidatorWithPublicKey -
func (n *nodesCoordinator) GetValidatorWithPublicKey(_ []byte) (validator sharding.Validator, err error) {
	return nil, nil
}

// LoadState -
func (n *nodesCoordinator) LoadState(_ []byte) error {
	return nil
}

// SetEpochValidatorsInfo -
func (n *nodesCoordinator) SetEpochValidatorsInfo(_ uint32, _ []*state.ValidatorInfo) error {
	return nil
}

// GetSavedStateKey -
func (n *nodesCoordinator) GetSavedStateKey() []byte {
	return nil
}

// GetConsensusWhitelistedNodes -
func (n *nodesCoordinator) GetConsensusWhitelistedNodes(_ uint32) (map[string]struct{}, error) {
	panic("not implemented")
}

// ConsensusGroupSize -
func (n *nodesCoordinator) ConsensusGroupSize() int {
	return 0
}

// GetNumTotalEligible -
func (n *nodesCoordinator) GetNumTotalEligible() uint64 {
	return 0
}

// IsInterfaceNil -
func (n *nodesCoordinator) IsInterfaceNil() bool {
	return n == nil
}
