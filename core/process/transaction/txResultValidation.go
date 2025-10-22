package transaction

import (
	"time"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

// validateTransactionResult validates local execution result against consensus result
// Returns an error if validation fails (block should be rejected)
// Returns the consensus error if local succeeded but consensus says it should fail
func (txProc *txProcessor) validateTransactionResult(
	block *block.Block,
	txHash []byte,
	tx *transaction.Transaction,
	localErr error,
	validatorExecutionTimeNs int64,
) error {
	// Find transaction index and get expected result from consensus
	txIndex := txProc.findTxIndexInBlock(block, txHash)
	if txIndex < 0 || txIndex >= len(block.TxResults) {
		log.Debug("validateTransactionResult: transaction not found in block or no result available",
			"txHash", txHash,
			"txIndex", txIndex,
			"blockTxResultsLen", len(block.TxResults))
		return localErr
	}

	expectedResultCode := block.TxResults[txIndex]
	actualResultCode := txProc.getActualResultCode(tx, localErr)

	// If results match, no validation needed
	if expectedResultCode == actualResultCode {
		log.Trace("validateTransactionResult: results match consensus",
			"txHash", txHash,
			"txIndex", txIndex,
			"resultCode", expectedResultCode)
		return localErr
	}

	// Results differ - validate based on case
	executionMode := txProc.scProcessor.GetVMExecutionMode()
	txProc.logResultMismatch(txHash, txIndex, expectedResultCode, actualResultCode, executionMode)

	// Handle different mismatch scenarios
	return txProc.handleResultMismatch(
		txHash,
		txIndex,
		tx,
		localErr,
		expectedResultCode,
		actualResultCode,
		executionMode,
		validatorExecutionTimeNs,
	)
}

// getActualResultCode determines the actual result code from local execution
func (txProc *txProcessor) getActualResultCode(tx *transaction.Transaction, localErr error) uint32 {
	if localErr != nil {
		return uint32(tx.ResultCode)
	}
	return uint32(transaction.Transaction_Ok)
}

// logResultMismatch logs when transaction results don't match consensus
func (txProc *txProcessor) logResultMismatch(
	txHash []byte,
	txIndex int,
	expectedResultCode, actualResultCode uint32,
	executionMode vmcommon.ExecutionMode,
) {
	log.Warn("Transaction result mismatch",
		"txHash", txHash,
		"txIndex", txIndex,
		"expected", expectedResultCode,
		"actual", actualResultCode,
		"mode", executionMode)
}

// handleResultMismatch processes different result mismatch scenarios
func (txProc *txProcessor) handleResultMismatch(
	txHash []byte,
	txIndex int,
	tx *transaction.Transaction,
	localErr error,
	expectedResultCode, actualResultCode uint32,
	executionMode vmcommon.ExecutionMode,
	validatorExecutionTimeNs int64,
) error {
	okCode := uint32(transaction.Transaction_Ok)

	// CASE 1: Validator succeeded, Leader failed - check tolerance band
	if actualResultCode == okCode && expectedResultCode != okCode {
		return txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorExecutionTimeNs)
	}

	// CASE 2: Leader succeeded, but validator failed - reject
	if expectedResultCode == okCode && actualResultCode != okCode {
		return txProc.handleLocalFailureConsensusSuccess(txHash, txIndex, expectedResultCode, actualResultCode, executionMode)
	}

	// CASE 3: Both failed with different error codes
	return txProc.handleDifferentErrorCodes(txHash, txIndex, tx, localErr, expectedResultCode, actualResultCode, executionMode)
}

// validateToleranceBand checks if leader's timeout failure was justified
func (txProc *txProcessor) validateToleranceBand(
	txHash []byte,
	tx *transaction.Transaction,
	expectedResultCode uint32,
	validatorExecutionTimeNs int64,
) error {
	baseTimeout := time.Duration(txProc.cfg.VirtualMachine.Execution.TimeOutForSCExecutionInMilliseconds) * time.Millisecond
	tolerancePercentage := txProc.cfg.VirtualMachine.Execution.TimeOutTolerancePercentage
	if tolerancePercentage == 0 {
		tolerancePercentage = 15 // Default 15% tolerance
	}

	// Calculate lower bound: base - tolerance%
	toleranceAmount := (baseTimeout * time.Duration(tolerancePercentage)) / 100
	lowerBound := baseTimeout - toleranceAmount
	validatorExecTime := time.Duration(validatorExecutionTimeNs)

	log.Warn("Validator succeeded, leader failed - checking tolerance",
		"txHash", txHash,
		"validatorTime", validatorExecTime,
		"baseTimeout", baseTimeout,
		"lowerBound", lowerBound)

	// If validator finished BEFORE lower bound, leader hardware is too weak
	if validatorExecTime < lowerBound {
		log.Warn("Rejecting block: leader hardware too weak",
			"txHash", txHash,
			"validatorTime", validatorExecTime,
			"lowerBound", lowerBound,
			"leaderShouldHaveSucceeded", true)
		return process.ErrTransactionResultMismatch
	}

	// Leader had right to fail - accept block
	log.Warn("Accepting leader failure - within tolerance",
		"txHash", txHash,
		"validatorTime", validatorExecTime,
		"lowerBound", lowerBound)
	tx.ResultCode = transaction.Transaction_TXResultCode(expectedResultCode)
	return process.ErrTransactionResultMismatch
}

// handleLocalFailureConsensusSuccess handles case where validator failed but consensus succeeded
func (txProc *txProcessor) handleLocalFailureConsensusSuccess(
	txHash []byte,
	txIndex int,
	expectedResultCode, actualResultCode uint32,
	executionMode vmcommon.ExecutionMode,
) error {
	if executionMode == vmcommon.ExecutionModeObserver {
		log.Warn("Observer: cannot validate leader's success - local execution failed, rejecting block",
			"txHash", txHash,
			"expectedResult", expectedResultCode,
			"actualResult", actualResultCode,
			"txIndex", txIndex)
		return process.ErrTransactionResultMismatch
	}

	log.Warn("Validator: transaction result mismatch - rejecting block",
		"txHash", txHash,
		"expectedResult", expectedResultCode,
		"actualResult", actualResultCode,
		"txIndex", txIndex)
	return process.ErrTransactionResultMismatch
}

// handleDifferentErrorCodes handles case where both failed with different error codes
func (txProc *txProcessor) handleDifferentErrorCodes(
	txHash []byte,
	txIndex int,
	tx *transaction.Transaction,
	localErr error,
	expectedResultCode, actualResultCode uint32,
	executionMode vmcommon.ExecutionMode,
) error {
	if executionMode == vmcommon.ExecutionModeObserver {
		log.Warn("Observer: accepting consensus decision despite different error codes",
			"txHash", txHash,
			"consensusResult", expectedResultCode,
			"localResult", actualResultCode,
			"txIndex", txIndex)
		tx.ResultCode = transaction.Transaction_TXResultCode(expectedResultCode)
		return localErr // Return local error to maintain state consistency
	}

	log.Warn("Validator: transaction error code mismatch - rejecting block",
		"txHash", txHash,
		"expectedResult", expectedResultCode,
		"actualResult", actualResultCode,
		"txIndex", txIndex)
	return process.ErrTransactionResultMismatch
}
