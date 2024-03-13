package disabled

import (
	"errors"
)

var errCannotReturnTotalStakedFromShardNode = errors.New("total staked value cannot be returned by a shard node")

type disabledStaleValuesProcessor struct{}

// NewDisabledStakeValuesProcessor -
func NewDisabledStakeValuesProcessor() (*disabledStaleValuesProcessor, error) {
	return new(disabledStaleValuesProcessor), nil
}

// GetTotalStakedValue -
func (d *disabledStaleValuesProcessor) GetTotalStakedValue() (int64, error) {
	return 0, errCannotReturnTotalStakedFromShardNode
}

// IsInterfaceNil returns true if there is no value under the interface
func (d *disabledStaleValuesProcessor) IsInterfaceNil() bool {
	return d == nil
}
