package sharding

import "fmt"

// SerializableValidator holds the minimal data required for marshalling and un-marshalling a validator
type SerializableValidator struct {
	OwnerAddress []byte `json:"ownerAddress"`
	PubKey       []byte `json:"pubKey"`
	Index        uint32 `json:"index"`
}

// EpochValidators holds one epoch configuration for a nodes coordinator
type EpochValidators struct {
	ElectedValidators  []*SerializableValidator `json:"electedValidators"`
	EligibleValidators []*SerializableValidator `json:"eligibleValidators"`
	WaitingValidators  []*SerializableValidator `json:"waitingValidators"`
	LeavingValidators  []*SerializableValidator `json:"leavingValidators"`
}

// NodesCoordinatorRegistry holds the data that can be used to initialize a nodes coordinator
type NodesCoordinatorRegistry struct {
	EpochsConfig map[string]*EpochValidators `json:"epochConfigs"`
	CurrentEpoch uint32                      `json:"currentEpoch"`
}

// NodesCoordinatorToRegistry will export the nodesCoordinator data to the registry
func (ihgs *indexHashedNodesCoordinator) NodesCoordinatorToRegistry() *NodesCoordinatorRegistry {
	ihgs.mutNodesConfig.RLock()
	defer ihgs.mutNodesConfig.RUnlock()

	registry := &NodesCoordinatorRegistry{
		CurrentEpoch: ihgs.currentEpoch.Load(),
		EpochsConfig: make(map[string]*EpochValidators, len(ihgs.nodesConfig)),
	}

	for epoch, epochNodesData := range ihgs.nodesConfig {
		registry.EpochsConfig[fmt.Sprint(epoch)] = epochNodesConfigToEpochValidators(epochNodesData)
	}

	return registry
}

func epochNodesConfigToEpochValidators(config *epochNodesConfig) *EpochValidators {
	result := &EpochValidators{
		ElectedValidators:  ValidatorArrayToSerializableValidatorArray(config.electedList),
		EligibleValidators: ValidatorArrayToSerializableValidatorArray(config.eligibleList),
		WaitingValidators:  ValidatorArrayToSerializableValidatorArray(config.waitingList),
		LeavingValidators:  ValidatorArrayToSerializableValidatorArray(config.leavingList),
	}

	return result
}

// ValidatorArrayToSerializableValidatorArray -
func ValidatorArrayToSerializableValidatorArray(validators []Validator) []*SerializableValidator {
	result := make([]*SerializableValidator, len(validators))

	for i, v := range validators {
		result[i] = &SerializableValidator{
			OwnerAddress: v.OwnerAddress(),
			PubKey:       v.PubKey(),
			//Chances: v.Chances(),
			Index: v.Index(),
		}
	}

	return result
}

// NodesInfoToValidators maps nodeInfo to validator interface
func NodesInfoToValidators(nodesInfo []GenesisNodeInfoHandler) ([]Validator, error) {

	validators := make([]Validator, 0, len(nodesInfo))
	for index, nodeInfo := range nodesInfo {
		// #nosec G115
		validatorObj, err := NewValidator(nodeInfo.AddressBytes(), nodeInfo.PubKeyBytes(), DefaultSelectionChances, uint32(index))
		if err != nil {
			return nil, err
		}

		validators = append(validators, validatorObj)
	}

	return validators, nil
}
