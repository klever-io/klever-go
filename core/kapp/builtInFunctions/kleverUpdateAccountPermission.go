package builtInFunctions

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverUpdateAccountPermission struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	payableHandler vmcommon.PayableChecker
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverUpdateAccountPermissionFunc returns the create asset built-in function component
func NewKleverUpdateAccountPermissionFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverUpdateAccountPermission, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverUpdateAccountPermission{
		funcGasCost:    funcGasCost,
		marshaller:     marshaller,
		keyPrefix:      []byte(""),
		payableHandler: &disabledPayableHandler{},
		accountsCacher: accountsCacher,
		forkController: forkController,
		kappController: kappController,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *kleverUpdateAccountPermission) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.UpdateAccountPermission
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverUpdateAccountPermission) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getUpdateAccountPermissionContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsUpdateAccountPermission)
	if err != nil {
		return nil, err
	}

	//Using Kapps
	resultCode, err := e.kappController.GetAccountsKApp().UpdatePermission(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("UpdatePermission error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("UpdatePermission error: %s", resultCode.String())
		log.Trace("UpdatePermission error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getUpdateAccountPermissionContract convert the arguments to an UpdateAccountPermissionContract
func (e *kleverUpdateAccountPermission) getUpdateAccountPermissionContract(vmInput *vmcommon.ContractCallInput) (*transaction.UpdateAccountPermissionContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsUpdateAccountPermission {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.UpdateAccountPermissionContract{
		Permissions: []*transaction.AccPermission{},
	}

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverUpdateAccountPermission) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverUpdateAccountPermission) IsInterfaceNil() bool {
	return e == nil
}
