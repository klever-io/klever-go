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

type kleverDelegate struct {
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

// NewKleverDelegateFunc returns the create asset built-in function component
func NewKleverDelegateFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverDelegate, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverDelegate{
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
func (e *kleverDelegate) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Delegate
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverDelegate) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getDelegateContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	//err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsDelegate)
	//if err != nil {
	//	return nil, err
	//}

	//Using Kapps
	resultCode, err := e.kappController.GetAccountsKApp().Delegate(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("Delegate error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("BucketDelegate error: %s", resultCode.String())
		log.Trace("Delegate error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getDelegateContract convert the arguments to an DelegateContract
func (e *kleverDelegate) getDelegateContract(vmInput *vmcommon.ContractCallInput) (*transaction.DelegateContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsDelegate {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.DelegateContract{
		ToAddress: vmInput.NextArg(),
		BucketID:  vmInput.NextArg(),
	}

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverDelegate) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverDelegate) IsInterfaceNil() bool {
	return e == nil
}
