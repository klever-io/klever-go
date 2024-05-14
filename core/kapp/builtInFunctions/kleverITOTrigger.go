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

type kleverITOTrigger struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverITOTriggerFunc returns the create asset built-in function component
func NewKleverITOTriggerFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverITOTrigger, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverITOTrigger{
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
func (e *kleverITOTrigger) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.ITOTrigger
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverITOTrigger) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getITOTriggerContract(vmInput)
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
	resultCode, err := e.kappController.GetITOKApp().Trigger(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("ITOTrigger error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("ITOTrigger error: %s", resultCode.String())
		log.Trace("ITOTrigger error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getITOTriggerContract convert the arguments to an ITOTriggerContract
func (e *kleverITOTrigger) getITOTriggerContract(vmInput *vmcommon.ContractCallInput) (*transaction.ITOTriggerContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsITOTrigger {
		return nil, ErrInvalidArguments
	}

	triggerType := transaction.ITOTriggerContract_EnumITOTriggerType(vmInput.NextArg().Int32())

	contract := &transaction.ITOTriggerContract{
		TriggerType: triggerType,
		AssetID:     vmInput.NextArg(),
	}

	switch triggerType {
	case transaction.ITOTriggerContract_SetITOPrices:
		packs, err := DecodeITOPacks(vmInput.NextArg())
		if err != nil {
			return nil, err
		}
		contract.PackInfo = packs
	case transaction.ITOTriggerContract_UpdateStatus:
		contract.Status = transaction.ITOTriggerContract_EnumITOStatus(vmInput.NextArg().Int32())
	case transaction.ITOTriggerContract_UpdateReceiverAddress:
		contract.ReceiverAddress = vmInput.NextArg()
	case transaction.ITOTriggerContract_UpdateMaxAmount:
		contract.MaxAmount = vmInput.NextArg().Int64()
	case transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress:
		contract.DefaultLimitPerAddress = vmInput.NextArg().Int64()
	case transaction.ITOTriggerContract_UpdateTimes:
		contract.StartTime = vmInput.NextArg().Int64()
		contract.EndTime = vmInput.NextArg().Int64()
	case transaction.ITOTriggerContract_UpdateWhitelistStatus:
		contract.WhitelistStatus = transaction.ITOTriggerContract_EnumITOStatus(vmInput.NextArg().Int32())
	case transaction.ITOTriggerContract_AddToWhitelist,
		transaction.ITOTriggerContract_RemoveFromWhitelist:
		wl, err := DecodeITOWhitelist(vmInput.NextArg())
		if err != nil {
			return nil, err
		}

		contract.WhitelistInfo = wl
	case transaction.ITOTriggerContract_UpdateWhitelistTimes:
		contract.WhitelistStartTime = vmInput.NextArg().Int64()
		contract.WhitelistEndTime = vmInput.NextArg().Int64()
	default:
		return nil, ErrInvalidArguments
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverITOTrigger) IsInterfaceNil() bool {
	return e == nil
}
