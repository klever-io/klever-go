package disabled

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
)

//var _ process.ValidityAttester = (*validityAttester)(nil)

type validityAttester struct {
}

// NewValidityAttester returns a new instance of validityAttester
func NewValidityAttester() *validityAttester {
	return &validityAttester{}
}

// CheckBlockAgainstFinal -
func (v *validityAttester) CheckBlockAgainstFinal(_ data.HeaderHandler) error {
	return nil
}

// CheckBlockAgainstSlotManager -
func (v *validityAttester) CheckBlockAgainstSlotManager(_ data.HeaderHandler) error {
	return nil
}

// CheckBlockAgainstWhitelist -
func (v *validityAttester) CheckBlockAgainstWhitelist(_ process.InterceptedData) bool {
	return false
}

// IsInterfaceNil -
func (v *validityAttester) IsInterfaceNil() bool {
	return v == nil
}
