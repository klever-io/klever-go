package builtInFunctions

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type payableCheck struct {
	payableHandler vmcommon.PayableHandler
	forkController core.ForkController
}

// NewPayableCheckFunc returns a new component which checks if destination is payableCheck when needed
func NewPayableCheckFunc(
	payable vmcommon.PayableHandler,
	forkController core.ForkController,
) (*payableCheck, error) {
	if check.IfNil(payable) {
		return nil, ErrNilPayableHandler
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	return &payableCheck{
		payableHandler: payable,
		forkController: forkController,
	}, nil
}

func (p *payableCheck) mustVerifyPayable(vmInput *vmcommon.ContractCallInput, minLenArguments int) bool {
	if vmInput.ReturnCallAfterError {
		return false
	}
	if vmInput.CallType == vm.KDATransferAndExecute {
		return false
	}
	if len(vmInput.Arguments) > minLenArguments {
		if len(vmInput.Arguments[minLenArguments]) > 0 {
			return false
		}
	}

	return true
}

// CheckPayable returns error if the destination account a non-payable smart contract and there is no sc call after transfer
func (p *payableCheck) CheckPayable(vmInput *vmcommon.ContractCallInput, dstAddress []byte, minLenArguments int) error {
	if !p.mustVerifyPayable(vmInput, minLenArguments) {
		return nil
	}

	isPayable, errIsPayable := p.payableHandler.IsPayable(vmInput.CallerAddr, dstAddress)
	if errIsPayable != nil {
		return errIsPayable
	}
	if !isPayable {
		return ErrAccountNotPayable
	}

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (p *payableCheck) IsInterfaceNil() bool {
	return p == nil
}
