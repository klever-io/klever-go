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

type kleverConfigMarketplace struct {
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

// NewKleverConfigMarketplaceFunc returns the create asset built-in function component
func NewKleverConfigMarketplaceFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverConfigMarketplace, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverConfigMarketplace{
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
func (e *kleverConfigMarketplace) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.ConfigMarketplace
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverConfigMarketplace) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getConfigMarketplaceContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsConfigMarketplace)
	if err != nil {
		return nil, err
	}

	//Using Kapps
	resultCode, err := e.kappController.GetMarketKApp().ConfigMarketplace(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("ConfigMarketplace error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("ConfigMarketplace error: %s", resultCode.String())
		log.Trace("ConfigMarketplace error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getConfigMarketplaceContract convert the arguments to an ConfigMarketplaceContract
func (e *kleverConfigMarketplace) getConfigMarketplaceContract(vmInput *vmcommon.ContractCallInput) (*transaction.ConfigMarketplaceContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsConfigMarketplace {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.ConfigMarketplaceContract{
		MarketplaceID:      vmInput.NextArg(),
		Name:               vmInput.NextArg(),
		ReferralAddress:    vmInput.NextArg(),
		ReferralPercentage: vmInput.NextArg().Uint32(),
	}

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverConfigMarketplace) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverConfigMarketplace) IsInterfaceNil() bool {
	return e == nil
}
