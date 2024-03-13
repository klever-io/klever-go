package sharding

import (
	logger "github.com/klever-io/klever-go-logger"
)

var log = logger.GetOrCreate("sharding")

// SerializableValidatorsToValidators creates the validator map from serializable validator map
func SerializableValidatorsToValidators(nodeRegistryValidators []*SerializableValidator) ([]Validator, error) {
	newValidators := make([]Validator, len(nodeRegistryValidators))
	for i, shardValidator := range nodeRegistryValidators {
		v, err := NewValidator(shardValidator.OwnerAddress, shardValidator.PubKey, DefaultSelectionChances, shardValidator.Index)
		if err != nil {
			return nil, err
		}
		newValidators[i] = v
	}
	return newValidators, nil
}

func computeStartIndexAndNumAppearancesForValidator(expElectedList []uint32, idx int64) (int64, int64) {
	val := expElectedList[idx]
	startIdx := int64(0)
	listLen := int64(len(expElectedList))

	for i := idx - 1; i >= 0; i-- {
		if expElectedList[i] != val {
			startIdx = i + 1
			break
		}
	}

	endIdx := listLen - 1
	for i := idx + 1; i < listLen; i++ {
		if expElectedList[i] != val {
			endIdx = i - 1
			break
		}
	}

	return startIdx, endIdx - startIdx + 1
}

func displayNodesConfiguration(
	elected []Validator,
	eligible []Validator,
	waiting []Validator,
	leaving []Validator,
) {
	for _, v := range elected {
		pk := v.PubKey()
		log.Info("elected", "pk", pk)
	}
	for _, v := range eligible {
		pk := v.PubKey()
		log.Info("eligible", "pk", pk)
	}
	for _, v := range waiting {
		pk := v.PubKey()
		log.Info("waiting", "pk", pk)
	}
	for _, v := range leaving {
		pk := v.PubKey()
		log.Info("leaving", "pk", pk)
	}
}
