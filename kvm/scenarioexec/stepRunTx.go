package scenarioexec

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/vm"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

func (ae *VMTestExecutor) executeTx(txIndex string, tx *scenjsonmodel.Transaction) (*vmcommon.VMOutput, error) {
	ae.World.CreateStateBackup()

	var err error
	defer func() {
		if err != nil {
			errRollback := ae.World.RollbackChanges()
			if errRollback != nil {
				err = errRollback
			}
		} else {
			errCommit := ae.World.CommitChanges()
			if errCommit != nil {
				err = errCommit
			}
		}
	}()

	gasForExecution := uint64(0)

	if tx.Type.HasSender() {
		beforeErr := ae.World.UpdateWorldStateBefore(
			tx.From.Value,
			tx.GasLimit.Value,
			tx.GasPrice.Value)
		if beforeErr != nil {
			err = fmt.Errorf("could not set up tx %s: %w", txIndex, beforeErr)
			return nil, err
		}

		gasForExecution = tx.GasLimit.Value
		if tx.KDAValue != nil || tx.KLVValue.Value.Cmp(big.NewInt(0)) > 0 {
			gasConsumed, err := ae.directKDATransferFromTx(tx)
			if err != nil {
				return nil, err
			}

			gasForExecution -= gasConsumed
		}
	}

	// we also use fake vm outputs for transactions that don't use the VM, just for convenience
	var output *vmcommon.VMOutput

	if !ae.senderHasEnoughBalance(tx) {
		// out of funds is handled by the protocol, so it needs to be mocked here
		output = outOfFundsResult()
	} else {
		switch tx.Type {
		case scenjsonmodel.ScDeploy:
			output, err = ae.scCreate(txIndex, tx, gasForExecution)
			if err != nil {
				return nil, err
			}
			if ae.PeekTraceGas() {
				fmt.Println("\nIn txID:", txIndex, ", step type:Deploy", ", total gas used:", gasForExecution-output.GasRemaining)
			}
		case scenjsonmodel.ScQuery:
			// imitates the behaviour of the protocol
			// the sender is the contract itself during SC queries
			tx.From = tx.To
			// gas restrictions waived during SC queries
			tx.GasLimit.Value = math.MaxInt64
			gasForExecution = math.MaxInt64
			fallthrough
		case scenjsonmodel.ScCall:
			output, err = ae.scCall(txIndex, tx, gasForExecution)
			if err != nil {
				return nil, err
			}
			if ae.PeekTraceGas() {
				fmt.Println("\nIn txID:", txIndex, ", step type:ScCall, function:", tx.Function, ", total gas used:", gasForExecution-output.GasRemaining)
			}
		case scenjsonmodel.Transfer:
			// transfer already processed by directKDATransferFromTx
			output = ae.vmHost.Output().GetVMOutput()
		case scenjsonmodel.ValidatorReward:
			fallthrough
		default:
			return nil, errors.New("unknown transaction type")
		}
	}

	if output.ReturnCode == vmcommon.Ok {
		err := ae.updateStateAfterTx(tx, output)
		if err != nil {
			return nil, err
		}
	} else {
		err = fmt.Errorf(
			"tx step failed: retcode=%d, msg=%s",
			output.ReturnCode, output.ReturnMessage)
	}

	return output, nil
}

func (ae *VMTestExecutor) senderHasEnoughBalance(tx *scenjsonmodel.Transaction) bool {
	if !tx.Type.HasSender() {
		return true
	}
	sender, err := ae.World.AccountsCacher.GetExistingUser(tx.From.Value)
	if err != nil {
		return false
	}
	return sender.GetBalance(nil) >= 0
}

func outOfFundsResult() *vmcommon.VMOutput {
	return &vmcommon.VMOutput{
		ReturnData:      make([][]byte, 0),
		ReturnCode:      vmcommon.VMExecutionFailed,
		ReturnMessage:   "",
		GasRemaining:    0,
		OutputAccounts:  make(map[string]*vmcommon.OutputAccount),
		DeletedAccounts: make([][]byte, 0),
		Logs:            make([]*vmcommon.LogEntry, 0),
	}
}

func (ae *VMTestExecutor) scCreate(txIndex string, tx *scenjsonmodel.Transaction, gasLimit uint64) (*vmcommon.VMOutput, error) {
	txHash := generateTxHash(txIndex)
	vmInput := vmcommon.VMInput{
		CallerAddr:     tx.From.Value,
		Arguments:      scenjsonmodel.JSONBytesFromTreeValues(tx.Arguments),
		GasProvided:    gasLimit,
		OriginalTxHash: txHash,
		CurrentTxHash:  txHash,
		KDATransfers:   make([]*vmcommon.KDATransfer, 0),
	}
	addKDAToVMInput(tx.KLVValue, tx.KDAValue, &vmInput)
	input := &vmcommon.ContractCreateInput{
		ContractCode: tx.Code.Value,
		VMInput:      vmInput,
	}

	return ae.vm.RunSmartContractCreate(input)
}

func (ae *VMTestExecutor) scCall(txIndex string, tx *scenjsonmodel.Transaction, gasLimit uint64) (*vmcommon.VMOutput, error) {
	recipient, err := ae.World.AccountsCacher.GetExistingUser(tx.To.Value)
	if err != nil {
		return nil, fmt.Errorf("tx recipient (address: %s) does not exist", hex.EncodeToString(tx.To.Value))
	}
	if len(ae.World.GetCode(recipient)) == 0 {
		return nil, fmt.Errorf("tx recipient (address: %s) is not a smart contract", hex.EncodeToString(tx.To.Value))
	}
	txHash := generateTxHash(txIndex)

	vmInput := vmcommon.VMInput{
		CallerAddr:     tx.From.Value,
		Arguments:      scenjsonmodel.JSONBytesFromTreeValues(tx.Arguments),
		GasProvided:    gasLimit,
		OriginalTxHash: txHash,
		CurrentTxHash:  txHash,
		KDATransfers:   make([]*vmcommon.KDATransfer, 0),
	}
	addKDAToVMInput(tx.KLVValue, tx.KDAValue, &vmInput)
	input := &vmcommon.ContractCallInput{
		RecipientAddr: tx.To.Value,
		Function:      tx.Function,
		VMInput:       vmInput,
	}

	return ae.vm.RunSmartContractCall(input)
}

func (ae *VMTestExecutor) directKDATransferFromTx(tx *scenjsonmodel.Transaction) (uint64, error) {
	transfers := make([]*vmcommon.KDATransfer, 0)

	for _, kda := range tx.KDAValue {
		transfer := &vmcommon.KDATransfer{
			KDAValue:      kda.Value.Value,
			KDATokenName:  kda.TokenIdentifier.Value,
			KDATokenType:  0,
			KDATokenNonce: kda.Nonce.Value,
		}
		if kda.Nonce.Value != 0 {
			transfer.KDATokenType = uint32(core.NonFungible)
		} else {
			transfer.KDATokenType = uint32(core.Fungible)
		}
		transfers = append(transfers, transfer)
	}

	if tx.KLVValue.Value.Cmp(big.NewInt(0)) > 0 {
		transfer := &vmcommon.KDATransfer{
			KDAValue:      tx.KLVValue.Value,
			KDATokenName:  kdautils.KLVIdentifier,
			KDATokenNonce: 0,
			KDATokenType:  uint32(core.Fungible),
		}
		transfers = append(transfers, transfer)
	}

	args := vmhost.KDATransfersArgs{
		Destination:    tx.To.Value,
		OriginalCaller: tx.From.Value,
		Sender:         tx.From.Value,
		Transfers:      transfers,
	}
	_, gasConsumed, err := ae.vmHost.ExecuteKDATransfer(&args, vm.DirectCall)
	if err != nil {
		return 0, err
	}
	return gasConsumed, nil
}

func (ae *VMTestExecutor) updateStateAfterTx(
	tx *scenjsonmodel.Transaction,
	output *vmcommon.VMOutput) error {
	// update accounts based on deltas
	updErr := ae.World.UpdateAccounts(output.OutputAccounts, output.DeletedAccounts)
	if updErr != nil {
		return updErr
	}
	return ae.World.CommitChanges()
}
