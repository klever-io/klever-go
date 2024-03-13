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

type kleverSell struct {
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

// NewKleverSellFunc returns the create asset built-in function component
func NewKleverSellFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverSell, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverSell{
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
func (e *kleverSell) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Sell
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverSell) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getSellContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsSell)
	if err != nil {
		return nil, err
	}

	//Using Kapps
	resultCode, err := e.kappController.GetMarketKApp().Sell(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("MarketSell error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("MarketSell error: %s", resultCode.String())
		log.Trace("MarketSell error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getSellContract convert the arguments to an SellContract
func (e *kleverSell) getSellContract(vmInput *vmcommon.ContractCallInput) (*transaction.SellContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsSell {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.SellContract{
		MarketType:    transaction.SellContract_EnumMarketType(vmInput.NextArg().Int32()),
		MarketplaceID: vmInput.NextArg(),
		AssetID:       []byte(fmt.Sprintf("%s/%d", vmInput.NextArg(), vmInput.NextArg().Int64())),
		CurrencyID:    vmInput.NextArg(),
		Price:         vmInput.NextArg().Int64(),
		ReservePrice:  vmInput.NextArg().Int64(),
		EndTime:       vmInput.NextArg().Int64(),
	}

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverSell) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverSell) IsInterfaceNil() bool {
	return e == nil
}
