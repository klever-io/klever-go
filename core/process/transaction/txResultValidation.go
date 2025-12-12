package transaction

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

// validateTransactionResult validates local execution result against consensus result from block.TxResults.
// This function implements tolerance band validation for smart contract execution timeouts to prevent
// unfair block rejections when validators have slightly better hardware than leaders.
//
// The validation behaves differently based on execution mode:
//   - Validator: Strictly validates results, rejects blocks on unjustified mismatches
//   - Observer: Follows consensus decision, accepts blocks even with local execution differences
//
// Parameters:
//   - block: The block containing consensus results in TxResults field
//   - txHash: Hash of the transaction being validated
//   - tx: The transaction object (ResultCode may be updated based on consensus)
//   - localErr: Error from local execution (nil if succeeded)
//   - validatorExecutionTimeNs: Execution time in nanoseconds from local execution
//
// Returns:
//   - nil: Results match consensus, no further action needed
//   - localErr: Transaction not found in block or no validation needed
//   - process.ErrTransactionResultMismatch: Block should be rejected (validator mode)
//   - consensus error: Local succeeded but consensus failed (accept block with consensus result)
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
		validatorExecutionTimeNs,
	)
}

// getActualResultCode determines the actual result code from local transaction execution.
// If local execution failed, returns the transaction's ResultCode. If succeeded, returns Transaction_Ok.
//
// Parameters:
//   - tx: Transaction with ResultCode set by local execution
//   - localErr: Error from local execution (nil if succeeded)
//
// Returns:
//   - uint32: Transaction_Ok if no error, otherwise tx.ResultCode
func (txProc *txProcessor) getActualResultCode(tx *transaction.Transaction, localErr error) uint32 {
	if localErr != nil {
		//nolint:gosec // G115: ResultCode enum values are always within uint32 range
		return uint32(tx.ResultCode)
	}
	return uint32(transaction.Transaction_Ok)
}

// logResultMismatch logs a warning when transaction execution results don't match consensus.
// This is a diagnostic helper function to track result mismatches for monitoring and debugging.
//
// Parameters:
//   - txHash: Hash of the transaction with mismatched results
//   - txIndex: Index of the transaction in the block
//   - expectedResultCode: Result code from consensus (block.TxResults)
//   - actualResultCode: Result code from local execution
//   - executionMode: Current VM execution mode (Validator/Observer)
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

// handleResultMismatch processes different transaction result mismatch scenarios and determines
// whether to accept or reject the block based on the mismatch type and execution mode.
//
// Three main scenarios are handled:
//  1. Validator succeeded, Leader failed: Check tolerance band to determine if leader's failure was justified
//  2. Leader succeeded, Validator failed: Always reject (both Validators and Observers)
//  3. Both failed with different errors: Validators reject block
//
// Parameters:
//   - txHash: Hash of the transaction with mismatched results
//   - txIndex: Index of the transaction in the block
//   - tx: Transaction object (ResultCode may be updated)
//   - localErr: Error from local execution
//   - expectedResultCode: Result code from consensus
//   - actualResultCode: Result code from local execution
//   - executionMode: Validator or Observer mode
//   - validatorExecutionTimeNs: Execution time for tolerance band validation
//
// Returns:
//   - error: Appropriate error based on scenario and execution mode
func (txProc *txProcessor) handleResultMismatch(
	txHash []byte,
	txIndex int,
	tx *transaction.Transaction,
	localErr error,
	expectedResultCode, actualResultCode uint32,
	validatorExecutionTimeNs int64,
) error {
	okCode := uint32(transaction.Transaction_Ok)
	timeoutCode := uint32(transaction.Transaction_VMExecutionFailed)

	// CASE 1: Validator succeeded, Leader failed - check tolerance band
	if actualResultCode == okCode && expectedResultCode == timeoutCode {
		return txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorExecutionTimeNs, localErr)
	}

	// CASE 2: Leader succeeded, but validator failed - block reject
	if expectedResultCode == okCode && actualResultCode == timeoutCode {
		log.Warn("Validator: transaction result mismatch - rejecting block",
			"txHash", txHash,
			"expectedResult", expectedResultCode,
			"actualResult", actualResultCode,
			"txIndex", txIndex)
		return process.ErrTransactionResultMismatch
	}

	// CASE 3: Both failed with different error codes, constinue based on execution mode - block rejected
	return localErr
}

// validateToleranceBand checks if a leader's smart contract execution timeout was justified
// based on the validator's execution time and configured tolerance band.
//
// Tolerance Band Logic:
//   - baseTimeout: Configured timeout (e.g., 500ms)
//   - tolerance: Percentage tolerance (e.g., 15%)
//   - lowerBound: baseTimeout - (baseTimeout * tolerance%) = 425ms with 15% tolerance
//
// Decision Rules:
//   - If validator finishes BEFORE lower bound: Leader hardware too weak → REJECT block
//   - If validator finishes AT OR AFTER lower bound: Leader had right to fail → ACCEPT block
//
// When accepting, the transaction's ResultCode is updated to the consensus value and
// ErrTransactionResultMismatch is returned to signal acceptance with consensus result.
//
// Parameters:
//   - txHash: Transaction hash for logging
//   - tx: Transaction to potentially update ResultCode
//   - expectedResultCode: Consensus result code (leader's failure code)
//   - validatorExecutionTimeNs: Validator's execution time in nanoseconds
//
// Returns:
//   - process.ErrTransactionResultMismatch: Either rejection (no ResultCode update) or
//     acceptance (with ResultCode updated to consensus)
func (txProc *txProcessor) validateToleranceBand(
	txHash []byte,
	tx *transaction.Transaction,
	expectedResultCode uint32,
	validatorExecutionTimeNs int64,
	localErr error,
) error {
	baseTimeout := time.Duration(txProc.cfg.VirtualMachine.Execution.TimeOutForSCExecutionInMilliseconds) * time.Millisecond
	tolerancePercentage := txProc.cfg.VirtualMachine.Execution.TimeOutTolerancePercentage
	if tolerancePercentage == 0 {
		tolerancePercentage = core.DefaultTolerancePercentage
	}
	if tolerancePercentage > 100 {
		log.Warn("TimeOutTolerancePercentage exceeds 100%, using 100%",
			"configured", tolerancePercentage)
		tolerancePercentage = 100
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
	// cant update result and should reutnr local error to reject block
	if validatorExecTime < lowerBound {
		log.Warn("Rejecting block: leader hardware too weak",
			"txHash", txHash,
			"validatorTime", validatorExecTime,
			"lowerBound", lowerBound,
			"leaderShouldHaveSucceeded", true)
		return localErr
	}

	// Leader had right to fail - accept block
	log.Warn("Accepting leader failure - within tolerance",
		"txHash", txHash,
		"validatorTime", validatorExecTime,
		"lowerBound", lowerBound)
	tx.Result = transaction.Transaction_FAILED
	//nolint:gosec // G115: expectedResultCode comes from block.TxResults which are valid enum values
	tx.ResultCode = transaction.Transaction_TXResultCode(expectedResultCode)
	return process.ErrTransactionResultMismatchAcceptLeader
}
