package builtInFunctions

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverWithdraw struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverWithdrawFunc returns the create asset built-in function component
func NewKleverWithdrawFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverWithdraw, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverWithdraw{
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
func (e *kleverWithdraw) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Withdraw
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverWithdraw) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getWithdrawContract(vmInput)
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
	var resultCode transaction.Transaction_TXResultCode
	switch contract.WithdrawType {
	case transaction.WithdrawContract_Staking:
		resultCode, err = e.kappController.GetAccountsKApp().Withdraw(vmInput.CallerAddr, contract)
	case transaction.WithdrawContract_KDAPool:
		resultCode, err = e.kappController.GetKDAFeesPoolKApp().Withdraw(vmInput.CallerAddr, contract)
	default:
		resultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrWithdrawTypeInvalid
	}
	if err != nil {
		log.Trace("Withdraw error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("KleverWithdraw error: %s", resultCode.String())
		log.Trace("Withdraw error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getWithdrawContract convert the arguments to an WithdrawContract
func (e *kleverWithdraw) getWithdrawContract(vmInput *vmcommon.ContractCallInput) (*transaction.WithdrawContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsWithdraw {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.WithdrawContract{
		WithdrawType: transaction.WithdrawContract_EnumWithdrawType(vmInput.NextArg().Int32()),
		AssetID:      vmInput.NextArg(),
		Amount:       vmInput.NextArg().Int64(),
		CurrencyID:   vmInput.NextArg(),
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverWithdraw) IsInterfaceNil() bool {
	return e == nil
}
