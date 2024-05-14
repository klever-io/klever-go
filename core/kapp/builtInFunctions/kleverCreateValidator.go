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

type kleverCreateValidator struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverCreateValidatorFunc returns the create asset built-in function component
func NewKleverCreateValidatorFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverCreateValidator, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverCreateValidator{
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
func (e *kleverCreateValidator) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.CreateValidator
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverCreateValidator) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getCreateValidatorContract(vmInput)
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
	resultCode, err := e.kappController.GetValidatorsKApp().Register(contract)
	if err != nil {
		log.Trace("CreateValidator error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("CreateValidator error: %s", resultCode.String())
		log.Trace("CreateValidator error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getCreateValidatorContract convert the arguments to an CreateValidatorContract
func (e *kleverCreateValidator) getCreateValidatorContract(vmInput *vmcommon.ContractCallInput) (*transaction.CreateValidatorContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsCreateValidator {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.CreateValidatorContract{
		OwnerAddress: vmInput.NextArg(),
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        vmInput.NextArg(),
			RewardAddress:       vmInput.NextArg(),
			Name:                vmInput.NextArg().String(),
			CanDelegate:         vmInput.NextArg().Bool(),
			Commission:          vmInput.NextArg().Uint32(),
			MaxDelegationAmount: vmInput.NextArg().Int64(),
			Logo:                vmInput.NextArg().String(),
			URIs:                make(map[string]string),
		},
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverCreateValidator) IsInterfaceNil() bool {
	return e == nil
}
