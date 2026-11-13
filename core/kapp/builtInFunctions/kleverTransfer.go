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

const kdaTransferErrorMessage = "KDA Transfer error"

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
	return kdaTransferError(e.kappController.GetAccountsKApp().Transfer(
		transaction.TXContract_TransferContractType,
		callerAddr,
		contract,
	))
}

// newTransferContract builds the account transfer that settles one KDA transfer of the call
func newTransferContract(recipient []byte, kdaTransfer *vmcommon.KDATransfer, value int64) *transaction.TransferContract {
	return &transaction.TransferContract{
		ToAddress:    recipient,
		AssetID:      GetTokenIdentifier(kdaTransfer),
		Amount:       value,
		KDARoyalties: 0, // No royalties for SC transfers
		KLVRoyalties: 0, // No royalties for SC transfers
	}
}

// transferAll settles every pending transfer as one accounts TransferBatch, so a transfer that fails
// its checks leaves every other transfer of the call unapplied. Transfers are marked executed and
// added to vmOutput only once the whole batch has settled.
//
// If moving value fails after earlier transfers of the batch were applied, none of them is marked
// executed, where the legacy loop marked the ones it paid. That is safe only because the error
// aborts and resets the whole transaction: nothing retries the builtin or reads IsExecuted after
// it fails.
//
// An invalid value is reported after the transfers before it are checked, which is the error the
// legacy loop reports, since it settles those transfers before it reads the invalid value.
func (e *kdaTransfer) transferAll(vmInput *vmcommon.ContractCallInput, vmOutput *vmcommon.VMOutput) error {
	pending, contracts, err := e.pendingTransferContracts(vmInput)
	if err != nil {
		return err
	}

	if toSettle := contractsToSettle(contracts); len(toSettle) > 0 {
		if err := kdaTransferError(e.kappController.GetAccountsKApp().TransferBatch(
			transaction.TXContract_TransferContractType,
			vmInput.CallerAddr,
			toSettle,
		)); err != nil {
			return err
		}
	}

	for i, kdaTransfer := range pending {
		// mark as executed to avoid double spending
		kdaTransfer.SetExecuted()
		e.logTransfer(vmOutput, vmInput.CallerAddr, contracts[i])
	}

	return nil
}

// pendingTransferContracts returns the pending transfers of the call and the account transfer that
// settles each of them. On an invalid value it reports the error the legacy loop would: that of an
// earlier transfer failing its checks, or else the value error.
func (e *kdaTransfer) pendingTransferContracts(vmInput *vmcommon.ContractCallInput) ([]*vmcommon.KDATransfer, []*transaction.TransferContract, error) {
	pending := make([]*vmcommon.KDATransfer, 0, len(vmInput.KDATransfers))
	contracts := make([]*transaction.TransferContract, 0, len(vmInput.KDATransfers))

	for _, kdaTransfer := range vmInput.KDATransfers {
		if kdaTransfer.IsExecuted() {
			continue
		}

		value, err := e.extractTransferValue(kdaTransfer)
		if err != nil {
			return nil, nil, e.validateBeforeValueError(vmInput.CallerAddr, contracts, err)
		}

		pending = append(pending, kdaTransfer)
		contracts = append(contracts, newTransferContract(vmInput.RecipientAddr, kdaTransfer, value))
	}

	return pending, contracts, nil
}

// validateBeforeValueError returns the error of the first of contracts that fails its checks, or
// valueErr when they all pass
func (e *kdaTransfer) validateBeforeValueError(callerAddr []byte, contracts []*transaction.TransferContract, valueErr error) error {
	toSettle := contractsToSettle(contracts)
	if len(toSettle) == 0 {
		return valueErr
	}

	if err := kdaTransferError(e.kappController.GetAccountsKApp().ValidateTransfers(
		transaction.TXContract_TransferContractType,
		callerAddr,
		toSettle,
	)); err != nil {
		return err
	}

	return valueErr
}

// contractsToSettle drops the zero-value transfers, which move nothing
func contractsToSettle(contracts []*transaction.TransferContract) []*transaction.TransferContract {
	toSettle := make([]*transaction.TransferContract, 0, len(contracts))
	for _, contract := range contracts {
		if contract.Amount > 0 {
			toSettle = append(toSettle, contract)
		}
	}

	return toSettle
}

// transferEach is the loop used before FixAuditChangesV7: each transfer settles before the next one
// is read.
func (e *kdaTransfer) transferEach(vmInput *vmcommon.ContractCallInput, vmOutput *vmcommon.VMOutput) error {
	for _, kdaTransfer := range vmInput.KDATransfers {
		// Check if the transfer was already executed
		if kdaTransfer.IsExecuted() {
			continue
		}

		if err := e.settleTransfer(vmInput, vmOutput, kdaTransfer); err != nil {
			return err
		}
	}

	return nil
}

// logTransfer adds the transfer log of contract to vmOutput. From FixAuditChangesV6 a zero-value
// transfer leaves no transfer log: it moved nothing, and the log would name any asset id or NFT
// nonce as transferred, held or not (KLC-2830).
func (e *kdaTransfer) logTransfer(vmOutput *vmcommon.VMOutput, callerAddr []byte, contract *transaction.TransferContract) {
	if contract.Amount == 0 && e.forkController.FixAuditChangesV6() {
		return
	}

	addTransferEntryInVMOutput(vmOutput, callerAddr, contract)
}

// addTransferEntryInVMOutput adds the transfer log of contract, sent by callerAddr, to vmOutput.
func addTransferEntryInVMOutput(vmOutput *vmcommon.VMOutput, callerAddr []byte, contract *transaction.TransferContract) {
	addKDAEntryInVMOutput(
		vmOutput,
		[]byte(core.BuiltInFunctionTransfer),
		contract.AssetID,
		0,
		big.NewInt(contract.Amount),
		callerAddr,
		contract.ToAddress,
	)
}

// kdaTransferError turns the outcome of an accounts transfer into the error the builtin reports
func kdaTransferError(resultCode transaction.Transaction_TXResultCode, err error) error {
	if err == nil && resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("%s: %s", kdaTransferErrorMessage, resultCode.String())
	}

	if err != nil {
		log.Trace(kdaTransferErrorMessage, "resultCode", resultCode, "err", err.Error())
	}

	return err
}

// ProcessBuiltinFunction resolves KDA transfer function calls.
//
// Each transfer settles straight into the accounts cacher, and nothing inside the builtin undoes
// one (there is no sub-transaction rollback, see BlockChainHookImpl.TransferValueOnly). Once
// FixAuditChangesV7 is active the pending transfers settle as one accounts TransferBatch, which
// checks all of them before moving any: a transfer that fails its checks leaves no other transfer
// of the call applied. Failures the checks cannot foresee (a receiver balance overflow) are still
// contained by the invariant that every VM error aborts the transaction.
func (e *kdaTransfer) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	if err := checkBasicKDAArguments(vmInput); err != nil {
		return nil, err
	}

	kdaTransfersLen := vmcommon.CountPendingKDATransfers(vmInput.KDATransfers)

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

	transfer := e.transferEach
	if e.forkController.FixAuditChangesV7() {
		transfer = e.transferAll
	}

	if err := transfer(vmInput, vmOutput); err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// settleTransfer settles one pending transfer from the caller to the recipient, marks it executed
// and logs it in vmOutput.
func (e *kdaTransfer) settleTransfer(
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
	kdaTransfer *vmcommon.KDATransfer,
) error {
	value, err := e.extractTransferValue(kdaTransfer)
	if err != nil {
		return err
	}

	contract := newTransferContract(vmInput.RecipientAddr, kdaTransfer, value)

	if value > 0 {
		if err := e.performKDATransfer(vmInput.CallerAddr, contract); err != nil {
			return err
		}
	}

	// mark as executed to avoid double spending
	kdaTransfer.SetExecuted()
	e.logTransfer(vmOutput, vmInput.CallerAddr, contract)

	return nil
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

// computeBuiltinGasRemaining enforces the built-in cost from V7 onward and
// preserves the legacy zero remainder for underfunded calls before activation.
func computeBuiltinGasRemaining(gasProvided, gasCost uint64, forks core.ForkController) (uint64, error) {
	if forks.FixAuditChangesV7() && gasProvided < gasCost {
		return 0, common.ErrNotEnoughGas
	}

	return computeGasRemaining(gasProvided, gasCost), nil
}
