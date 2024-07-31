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

type kleverUnjail struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverUnjailFunc returns the create asset built-in function component
func NewKleverUnjailFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverUnjail, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverUnjail{
		funcGasCost:    funcGasCost,
		marshaller:     marshaller,
		keyPrefix:      []byte(""),
		accountsCacher: accountsCacher,
		forkController: forkController,
		kappController: kappController,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *kleverUnjail) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Unjail
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverUnjail) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getUnjailContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}

	//Using Kapps
	resultCode, err := e.kappController.GetValidatorsKApp().Unjail(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("Unjail error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("ValidatorUnjail error: %s", resultCode.String())
		log.Trace("Unjail error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getUnjailContract convert the arguments to an UnjailContract
func (e *kleverUnjail) getUnjailContract(vmInput *vmcommon.ContractCallInput) (*transaction.UnjailContract, error) {
	return &transaction.UnjailContract{}, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverUnjail) IsInterfaceNil() bool {
	return e == nil
}
