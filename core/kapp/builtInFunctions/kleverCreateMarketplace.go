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

type kleverCreateMarketplace struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverCreateMarketplaceFunc returns the create asset built-in function component
func NewKleverCreateMarketplaceFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverCreateMarketplace, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverCreateMarketplace{
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
func (e *kleverCreateMarketplace) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.CreateMarketplace
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverCreateMarketplace) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getCreateMarketplaceContract(vmInput)
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
	resultCode, err := e.kappController.GetMarketKApp().CreateMarketplace(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("CreateMarketplace error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("CreateMarketplace error: %s", resultCode.String())
		log.Trace("CreateMarketplace error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getCreateMarketplaceContract convert the arguments to an CreateMarketplaceContract
func (e *kleverCreateMarketplace) getCreateMarketplaceContract(vmInput *vmcommon.ContractCallInput) (*transaction.CreateMarketplaceContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsCreateMarketplace {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.CreateMarketplaceContract{
		Name:               vmInput.NextArg(),
		ReferralAddress:    vmInput.NextArg(),
		ReferralPercentage: vmInput.NextArg().Uint32(),
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverCreateMarketplace) IsInterfaceNil() bool {
	return e == nil
}
