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

type kleverDeposit struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverDepositFunc returns the create asset built-in function component
func NewKleverDepositFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverDeposit, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverDeposit{
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
func (e *kleverDeposit) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Deposit
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverDeposit) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getDepositContract(vmInput)
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
	switch contract.GetDepositType() {
	case transaction.DepositContract_FPRDeposit:
		resultCode, err = e.kappController.GetKDAKApp().Deposit(vmInput.CallerAddr, contract)
	case transaction.DepositContract_KDAPool:
		resultCode, err = e.kappController.GetKDAFeesPoolKApp().Deposit(vmInput.CallerAddr, contract)
	default:
		resultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrDepositTypeInvalid
	}
	if err != nil {
		log.Trace("Deposit error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("KleverDeposit error: %s", resultCode.String())
		log.Trace("Deposit error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getDepositContract convert the arguments to an DepositContract
func (e *kleverDeposit) getDepositContract(vmInput *vmcommon.ContractCallInput) (*transaction.DepositContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsDeposit {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.DepositContract{
		DepositType: transaction.DepositContract_EnumDepositType(vmInput.NextArg().Int32()),
		ID:          vmInput.NextArg(),
		CurrencyID:  vmInput.NextArg(),
		Amount:      vmInput.NextArg().Int64(),
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverDeposit) IsInterfaceNil() bool {
	return e == nil
}
