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

type kleverConfigITO struct {
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

// NewKleverConfigITOFunc returns the create asset built-in function component
func NewKleverConfigITOFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverConfigITO, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverConfigITO{
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
func (e *kleverConfigITO) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.ConfigITO
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverConfigITO) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getConfigITOContract(vmInput)
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
	resultCode, err := e.kappController.GetITOKApp().Config(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("ConfigITO error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("ConfigITO error: %s", resultCode.String())
		log.Trace("ConfigITO error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getConfigITOContract convert the arguments to an ConfigITOContract
func (e *kleverConfigITO) getConfigITOContract(vmInput *vmcommon.ContractCallInput) (*transaction.ConfigITOContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsITOConfig {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.ConfigITOContract{
		AssetID:                vmInput.NextArg(),
		ReceiverAddress:        vmInput.NextArg(),
		Status:                 transaction.ConfigITOContract_EnumITOStatus(vmInput.NextArg().Int32()),
		MaxAmount:              vmInput.NextArg().Int64(),
		DefaultLimitPerAddress: vmInput.NextArg().Int64(),
		StartTime:              vmInput.NextArg().Int64(),
		EndTime:                vmInput.NextArg().Int64(),
		PackInfo:               make(map[string]*transaction.PackInfo),
		WhitelistStatus:        transaction.ConfigITOContract_EnumITOStatus(vmInput.NextArg().Int32()),
		WhitelistStartTime:     vmInput.NextArg().Int64(),
		WhitelistEndTime:       vmInput.NextArg().Int64(),
		WhitelistInfo:          make(map[string]*transaction.WhitelistInfo),
	}

	wl, err := DecodeITOWhitelist(vmInput.NextArg())
	if err != nil {
		return nil, err
	}

	contract.WhitelistInfo = wl

	packs, err := DecodeITOPacks(vmInput.NextArg())
	if err != nil {
		return nil, err
	}

	contract.PackInfo = packs

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverConfigITO) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverConfigITO) IsInterfaceNil() bool {
	return e == nil
}
