package builtInFunctions

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

var zero = big.NewInt(0)

type kdaTransfer struct {
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

// NewKDATransferFunc returns the kda transfer built-in function component
func NewKDATransferFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kdaTransfer, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kdaTransfer{
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
func (e *kdaTransfer) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Transfer
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kdaTransfer) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	// Builtin cost by number of TX executed
	totalCost := e.funcGasCost * uint64(len(vmInput.KDATransfers))
	gasRemaining := computeGasRemaining(vmInput.GasProvided, totalCost)
	vmOutput := &vmcommon.VMOutput{
		GasRemaining:   gasRemaining,
		ReturnCode:     vmcommon.Ok,
		OutputAccounts: make(map[string]*vmcommon.OutputAccount),
	}

	err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsKDATransfer)
	if err != nil {
		return nil, err
	}

	for _, kdaTransfer := range vmInput.KDATransfers {
		// Check if the transfer was already executed
		if kdaTransfer.IsExecuted() {
			continue
		}
		tokenIdentifier := kdaTransfer.KDATokenName
		if kdaTransfer.KDATokenNonce > 0 {
			tokenIdentifier = []byte(fmt.Sprintf("%s%s%d", kdaTransfer.KDATokenName, kapps.Sp, kdaTransfer.KDATokenNonce))
		}

		value := int64(0)
		if kdaTransfer.KDAValue != nil {
			value = kdaTransfer.KDAValue.Int64()
		}

		contract := &transaction.TransferContract{
			ToAddress:    vmInput.RecipientAddr,
			AssetID:      tokenIdentifier,
			Amount:       value,
			KDARoyalties: 0, // No royalties for SC transfers
			KLVRoyalties: 0, // No royalties for SC transfers
		}

		// Using Kapps, transfer the KDA without transfer fixed/percentage royalties
		// royalties are only processed if the contract is a TXContract_TransferContractType
		resultCode, err := e.kappController.GetAccountsKApp().Transfer(
			transaction.TXContract_SmartContractType,
			vmInput.CallerAddr,
			contract,
		)
		if err != nil {
			log.Trace("KDA Transfer error", "resultCode", resultCode, "err", err.Error())
			return nil, err
		}

		if resultCode != transaction.Transaction_Ok {
			err = fmt.Errorf("KDA Transfer error: %s", resultCode.String())
			log.Trace("KDA Transfer error", "resultCode", resultCode, "err", err.Error())
			return nil, err
		}
		// mark as executed to avoid double spending
		kdaTransfer.SetExecuted()

		addKDAEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionTransfer), contract.AssetID, 0, big.NewInt(contract.Amount), vmInput.CallerAddr, contract.ToAddress)
	}

	return vmOutput, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kdaTransfer) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kdaTransfer) IsInterfaceNil() bool {
	return e == nil
}

func checkBasicKDAArguments(vmInput *vmcommon.ContractCallInput) error {
	if vmInput == nil {
		return ErrNilVmInput
	}

	return nil
}

func computeGasRemaining(gasProvided uint64, gasToUse uint64) uint64 {
	if gasProvided < gasToUse {
		return 0
	}

	return gasProvided - gasToUse
}
