package builtInFunctions

import (
	"github.com/klever-io/klever-go/vmcommon"
)

// disabledPayableHandler is a disabled payableCheck handler that implements PayableChecker interface but it is disabled
type disabledPayableHandler struct {
}

// CheckPayable returns error as this is a disabled payableCheck handler
func (d *disabledPayableHandler) CheckPayable(_ *vmcommon.ContractCallInput, _ []byte, _ int) error {
	return ErrAccountNotPayable
}

// IsInterfaceNil returns true if underlying object is nil
func (d *disabledPayableHandler) IsInterfaceNil() bool {
	return d == nil
}
