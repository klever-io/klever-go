package builtInFunctions

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kdaTransfer struct {
	baseAlwaysActiveHandler
	kappController kapp.KAppController
	forkController core.ForkController
	funcGasCost    uint64
	payableHandler vmcommon.PayableChecker
	mutExecution   sync.RWMutex
}

// NewKDATransferFunc returns the kda transfer built-in function component
func NewKDATransferFunc(
	funcGasCost uint64,
	kappController kapp.KAppController,
	forkController core.ForkController,
) *kdaTransfer {
	e := &kdaTransfer{
		funcGasCost:    funcGasCost,
		payableHandler: &disabledPayableHandler{},
		kappController: kappController,
		forkController: forkController,
	}

	return e
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

// Extract token identifier
func GetTokenIdentifier(kdaTransfer *vmcommon.KDATransfer) []byte {
	tokenIdentifier := kdaTransfer.KDATokenName
	if kdaTransfer.KDATokenNonce > 0 {
		tokenIdentifier = []byte(
			fmt.Sprintf("%s%s%d", kdaTransfer.KDATokenName, kapps.Sp, kdaTransfer.KDATokenNonce),
		)
	}
	return tokenIdentifier
}

// GetTransferValue extracts the transfer value using the legacy (pre-FixAuditChangesV3) semantics:
// the arbitrary-size value is narrowed with Int64() and a nil value maps to zero. It is kept to
// reproduce the historical behaviour when replaying blocks before the fork activation epoch.
func GetTransferValue(kdaTransfer *vmcommon.KDATransfer) int64 {
	if kdaTransfer.KDAValue != nil {
		return kdaTransfer.KDAValue.Int64()
	}
	return 0
}

// GetValidatedTransferValue extracts the transfer value under the FixAuditChangesV3 fork.
//
// The value must be non-nil, non-negative and representable as int64. Oversized values are rejected
// instead of being silently truncated by Int64(): otherwise a value such as 2^64 would narrow to 0,
// the real account transfer would be skipped, yet the original oversized amount would still reach the
// recipient contract as call value, letting it credit assets that were never paid (KLR-02).
func GetValidatedTransferValue(kdaTransfer *vmcommon.KDATransfer) (int64, error) {
	if kdaTransfer == nil || kdaTransfer.KDAValue == nil {
		return 0, common.ErrInvalidValue
	}
	if kdaTransfer.KDAValue.Sign() < 0 || !kdaTransfer.KDAValue.IsInt64() {
		return 0, common.ErrInvalidValue
	}
	return kdaTransfer.KDAValue.Int64(), nil
}

// extractTransferValue returns the transfer value to apply, rejecting oversized/invalid amounts once
// the FixAuditChangesV3 fork is active and falling back to the legacy truncating behaviour before it.
func (e *kdaTransfer) extractTransferValue(kdaTransfer *vmcommon.KDATransfer) (int64, error) {
	if e.forkController.FixAuditChangesV3() {
		return GetValidatedTransferValue(kdaTransfer)
	}

	value := GetTransferValue(kdaTransfer)
	if value < 0 {
		return 0, common.ErrInvalidValue
	}
	return value, nil
}

// Perform the KDA transfer
func (e *kdaTransfer) performKDATransfer(
	callerAddr []byte,
	contract *transaction.TransferContract,
) error {
	// Using Kapps, transfer the KDA without transfer fixed/percentage royalties
	// royalties are only processed if the contract is a TXContract_TransferContractType
	resultCode, err := e.kappController.GetAccountsKApp().Transfer(
		transaction.TXContract_TransferContractType,
		callerAddr,
		contract,
	)
	if err != nil {
		log.Trace("KDA Transfer error", "resultCode", resultCode, "err", err.Error())
		return err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("KDA Transfer error: %s", resultCode.String())
		log.Trace("KDA Transfer error", "resultCode", resultCode, "err", err.Error())
		return err
	}

	return nil
}

func (e *kdaTransfer) getPendingTransfersCount(vmInput *vmcommon.ContractCallInput) uint64 {
	kdaTransfersLen := uint64(0)

	for _, v := range vmInput.KDATransfers {
		if !v.IsExecuted() {
			kdaTransfersLen++
		}
	}

	return kdaTransfersLen
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kdaTransfer) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	if err := checkBasicKDAArguments(vmInput); err != nil {
		return nil, err
	}

	kdaTransfersLen := e.getPendingTransfersCount(vmInput)

	// Builtin cost by number of TX to be executed (pending transfers)
	totalCost := e.funcGasCost * kdaTransfersLen
	if totalCost > vmInput.GasProvided {
		return nil, common.ErrNotEnoughGas
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, totalCost)
	vmOutput := &vmcommon.VMOutput{
		GasRemaining:   gasRemaining,
		ReturnCode:     vmcommon.Ok,
		OutputAccounts: make(map[string]*vmcommon.OutputAccount),
	}

	if err := e.payableHandler.CheckPayable(
		vmInput,
		vmInput.RecipientAddr,
		core.MinLenArgumentsKDATransfer,
	); err != nil {
		return nil, err
	}

	for _, kdaTransfer := range vmInput.KDATransfers {
		// Check if the transfer was already executed
		if kdaTransfer.IsExecuted() {
			continue
		}

		tokenIdentifier := GetTokenIdentifier(kdaTransfer)
		value, err := e.extractTransferValue(kdaTransfer)
		if err != nil {
			return nil, err
		}

		contract := &transaction.TransferContract{
			ToAddress:    vmInput.RecipientAddr,
			AssetID:      tokenIdentifier,
			Amount:       value,
			KDARoyalties: 0, // No royalties for SC transfers
			KLVRoyalties: 0, // No royalties for SC transfers
		}

		if value > 0 {
			if err := e.performKDATransfer(vmInput.CallerAddr, contract); err != nil {
				return nil, err
			}
		}

		// mark as executed to avoid double spending
		kdaTransfer.SetExecuted()

		addKDAEntryInVMOutput(
			vmOutput,
			[]byte(core.BuiltInFunctionTransfer),
			contract.AssetID,
			0,
			big.NewInt(contract.Amount),
			vmInput.CallerAddr,
			contract.ToAddress,
		)
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
