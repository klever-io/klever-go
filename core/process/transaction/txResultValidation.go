package transaction

import (
	"encoding/hex"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

// validateTransactionResult validates local execution result against consensus result from block.TxResults.
// This function implements tolerance band validation for smart contract execution timeouts to prevent
// unfair block rejections when validators have slightly better hardware than leaders.
//
// The validation behaves differently based on the VM execution mode:
//   - Validator (live): re-judges a timeout mismatch via the tolerance band (KLC-1894)
//   - Observer (import-db replay): treats a recorded *timeout* as ground truth and reproduces it,
//     never re-deriving pass/fail from local wall-clock timing (see CASE 1a in handleResultMismatch)
//
// A consensus failure code that is not the timeout code is deterministic, so a local success
// contradicting it is rejected in every mode (CASE 1b).
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
//   - an error matching process.ErrTransactionResultMismatch (errors.Is): Block should be rejected
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
		executionMode,
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
//   - executionMode: Current VM execution mode (Leader/Validator/Query/Observer)
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
// whether to accept or reject the block based on the mismatch type and the execution mode.
//
// Scenarios handled:
//  1. Local succeeded, consensus recorded a failure:
//     a. the failure is VMExecutionFailed (the timeout code), which a slow leader can legitimately
//     produce:
//     - Observer (import-db replay): reproduce the recorded failure and revert the local success
//     - Validator (live): check the tolerance band to decide if the leader's timeout was justified
//     b. the failure is any other code (VMUserError, VMOutOfGas, Fail, ...), which is deterministic
//     and cannot be explained by leader slowness: reject the block outright, in every mode
//     (KLR-63, from FixAuditChangesV5 on)
//  2. Consensus succeeded, but local execution failed with the timeout code: reject the block.
//     Validator (live): ErrLocalExecutionTimeoutAbstain, so consensus can report the abstention.
//     Replay (agreed history): plain ErrTransactionResultMismatch, the state cannot be mended.
//  3. Both failed with different errors: bubble up the local error
//
// Parameters:
//   - txHash: Hash of the transaction with mismatched results
//   - txIndex: Index of the transaction in the block
//   - tx: Transaction object (ResultCode may be updated)
//   - localErr: Error from local execution
//   - expectedResultCode: Result code from consensus
//   - actualResultCode: Result code from local execution
//   - validatorExecutionTimeNs: Execution time for tolerance band validation
//   - executionMode: Current VM execution mode (Leader/Validator/Query/Observer)
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
	executionMode vmcommon.ExecutionMode,
) error {
	okCode := uint32(transaction.Transaction_Ok)
	vmFailedCode := uint32(transaction.Transaction_VMExecutionFailed)

	// CASE 1: local succeeded, consensus recorded a failure of any kind.
	if actualResultCode == okCode && expectedResultCode != okCode {
		// CASE 1a: the consensus failure is the timeout code, so the leader may have a legitimate
		// hardware excuse and the tolerance band gets to judge it.
		if expectedResultCode == vmFailedCode {
			// Observer (import-db replay): the recorded VMExecutionFailed is agreed history, not a
			// live leader to judge. Reproduce it and let the preprocessor revert the completed local
			// execution so the account state root matches the recorded root - never re-derive
			// pass/fail from local wall-clock timing (the tolerance band, KLC-1894, is
			// Validator/live-only).
			if executionMode == vmcommon.ExecutionModeReplay {
				log.Warn("Observer: transaction result mismatch - Accepting leader failure (agreed history: import-db or sync)",
					"txHash", txHash,
					"expectedResult", expectedResultCode,
					"actualResult", actualResultCode,
					"txIndex", txIndex)
				return acceptConsensusFailure(tx, expectedResultCode)
			}
			return txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorExecutionTimeNs, localErr)
		}

		// CASE 1b (KLR-63): the consensus failure is a deterministic one - VMUserError (57),
		// VMOutOfGas (58), Fail (99), anything that is not the timeout code. Local execution
		// provably succeeded, and no amount of leader slowness produces those codes, so there is
		// no timing argument to weigh and validateToleranceBand must not be reached: accepting
		// would stamp a non-timeout failure onto a transaction that succeeded. Reject outright.
		//
		// This holds in Observer mode too. The replay short-circuit above exists because timeouts
		// are environmental; a deterministic failure code that local re-execution contradicts is a
		// genuine determinism break, and reproducing it would paper over it. Pre-fork blocks are
		// unaffected - the flag is toggled per epoch, so replaying them takes the legacy branch.
		log.Warn("Transaction result mismatch - consensus recorded a non-timeout failure for a locally successful transaction",
			"txHash", txHash,
			"expectedResult", expectedResultCode,
			"actualResult", actualResultCode,
			"txIndex", txIndex,
			"mode", executionMode)

		if txProc.forkController.FixAuditChangesV5() {
			return process.ErrTransactionResultMismatch
		}

		return localErr
	}

	// CASE 2: Leader succeeded, but the local execution failed with the timeout code - block reject
	// (even on replay mode, we cant mend the state here as we dont have the actual result)
	if expectedResultCode == okCode && actualResultCode == vmFailedCode {
		return txProc.rejectLocalTimeout(txHash, txIndex, tx, validatorExecutionTimeNs, executionMode)
	}

	// CASE 3: Both failed with different error codes, continue based on execution mode - block rejected
	return localErr
}

// validateToleranceBand checks if a leader's smart contract execution timeout was justified
// based on the validator's execution time and configured tolerance band.
//
// Tolerance Band Logic:
//   - baseTimeout: Configured timeout (e.g., 500ms), floored at core.MinSCExecutionTimeout to match
//     the clamp the VM host applies to the same config value
//   - tolerance: Percentage tolerance (e.g., 15%)
//   - lowerBound: baseTimeout - (baseTimeout * tolerance%) = 425ms with 15% tolerance
//
// Decision Rules:
//   - If validator finishes BEFORE lower bound: leader hardware too weak, the leader should have
//     succeeded. The ResultCode is NOT updated and the block is rejected (KLR-63: explicit from
//     FixAuditChangesV5 on; before the fork the always-nil localErr was returned instead, leaving
//     the rejection to the downstream verifyBlockTrieRoots state-root comparison).
//   - If validator finishes AT OR AFTER lower bound: leader had the right to fail. The ResultCode
//     is updated to the consensus value and ErrTransactionResultMismatchAcceptLeader is returned so
//     the preprocessor reverts the local success and accepts the block with the consensus result.
//
// Parameters:
//   - txHash: Transaction hash for logging
//   - tx: Transaction to potentially update ResultCode
//   - expectedResultCode: Consensus result code (leader's failure code)
//   - validatorExecutionTimeNs: Validator's execution time in nanoseconds
//
// Returns:
//   - an error matching process.ErrLeaderTimeoutRefused and process.ErrTransactionResultMismatch
//     (pre-FixAuditChangesV5: localErr, always nil): leader too weak, reject the block; no
//     ResultCode update
//   - process.ErrTransactionResultMismatchAcceptLeader: accept, with ResultCode set to consensus
func (txProc *txProcessor) validateToleranceBand(
	txHash []byte,
	tx *transaction.Transaction,
	expectedResultCode uint32,
	validatorExecutionTimeNs int64,
	localErr error,
) error {
	// The bound has to be derived from the timeout the VM actually enforced, not from the raw
	// config value. hostCore.configureTimeouts clamps to core.MinSCExecutionTimeout, so on a node
	// where timeOutForSCExecutionInMilliseconds is unset or below the floor the raw value is 0
	// while contracts still execute with a 400ms budget. Judging the leader against a zero base
	// puts every possible execution time at or above a zero lower bound, which makes the rejection
	// below unreachable and silently turns the fix inert exactly where it matters (the call site
	// only reaches here when validatorExecutionTimeNs > 0). Gated so that pre-fork replay keeps
	// deriving the bound from the raw config value.
	fixAuditChangesV5 := txProc.forkController.FixAuditChangesV5()

	// Calculate lower bound: base - tolerance%
	baseTimeout, toleranceAmount := txProc.scTimeoutBand(fixAuditChangesV5)
	lowerBound := baseTimeout - toleranceAmount
	validatorExecTime := time.Duration(validatorExecutionTimeNs)

	log.Warn("Validator succeeded, leader failed - checking tolerance",
		"txHash", txHash,
		"validatorTime", validatorExecTime,
		"baseTimeout", baseTimeout,
		"lowerBound", lowerBound)

	// If validator finished BEFORE lower bound, leader hardware is too weak: the leader should
	// have succeeded, so the block is invalid and must be rejected.
	if validatorExecTime < lowerBound {
		log.Warn("Rejecting block: leader hardware too weak",
			"txHash", txHash,
			"validatorTime", validatorExecTime,
			"lowerBound", lowerBound,
			"leaderShouldHaveSucceeded", true)

		if fixAuditChangesV5 {
			return txProc.newResultMismatchError(process.ErrLeaderTimeoutRefused, txHash, tx)
		}

		return localErr
	}

	// Leader had right to fail - accept block
	log.Warn("Accepting leader failure - within tolerance",
		"txHash", txHash,
		"validatorTime", validatorExecTime,
		"lowerBound", lowerBound)
	return acceptConsensusFailure(tx, expectedResultCode)
}

func acceptConsensusFailure(tx *transaction.Transaction, expectedResultCode uint32) error {
	tx.Result = transaction.Transaction_FAILED
	//nolint:gosec // G115: expectedResultCode is provably vmFailedCode at this point, which is a valid enum value
	tx.ResultCode = transaction.Transaction_TXResultCode(expectedResultCode)
	return process.ErrTransactionResultMismatchAcceptLeader
}

// rejectLocalTimeout handles CASE 2: the block records a success for a transaction whose local
// execution failed with the timeout code. The block is rejected in every mode, the difference is
// only in what is reported:
//   - Validator (live consensus), local execution ran up to the Validator-mode VM timeout: this
//     node cannot vouch for the leader's result, so it abstains from signing. The returned error
//     matches ErrLocalExecutionTimeoutAbstain and carries the transaction and contract so
//     consensus can report it; it still matches ErrTransactionResultMismatch, so the block is
//     rejected and the local execution reverted.
//   - Validator, local execution failed before that timeout: the code is also produced by VM
//     failures other than a timeout, so this is reported as a plain mismatch.
//   - Replay (agreed history: sync and import-db): local execution failed even with the replay
//     budget, so there is no recorded result to reproduce and the state cannot be mended.
func (txProc *txProcessor) rejectLocalTimeout(
	txHash []byte,
	txIndex int,
	tx *transaction.Transaction,
	validatorExecutionTimeNs int64,
	executionMode vmcommon.ExecutionMode,
) error {
	contract := txProc.smartContractAddress(tx)

	if executionMode == vmcommon.ExecutionModeReplay {
		log.Warn("Replay: local smart contract execution failed with the timeout code even with the replay budget "+
			"on a transaction the network recorded as successful - rejecting block, the local state cannot be mended",
			"txHash", txHash,
			"txIndex", txIndex,
			"contract", contract)
		return process.ErrTransactionResultMismatch
	}

	validatorTimeout := txProc.validatorSCTimeout()
	localExecTime := time.Duration(validatorExecutionTimeNs)
	if localExecTime < validatorTimeout {
		log.Warn("Validator: local smart contract execution failed before the timeout where the leader succeeded - rejecting block",
			"txHash", txHash,
			"txIndex", txIndex,
			"contract", contract,
			"localExecTime", localExecTime,
			"validatorTimeout", validatorTimeout)
		return process.ErrTransactionResultMismatch
	}

	log.Warn("Validator: local smart contract execution timed out where the leader succeeded - rejecting block, abstaining from signing",
		"txHash", txHash,
		"txIndex", txIndex,
		"contract", contract,
		"localExecTime", localExecTime,
		"validatorTimeout", validatorTimeout)
	return &process.TxResultMismatchError{
		Reason:   process.ErrLocalExecutionTimeoutAbstain,
		TxHash:   txHash,
		Contract: contract,
	}
}

func (txProc *txProcessor) newResultMismatchError(reason error, txHash []byte, tx *transaction.Transaction) error {
	return &process.TxResultMismatchError{
		Reason:   reason,
		TxHash:   txHash,
		Contract: txProc.smartContractAddress(tx),
	}
}

// smartContractAddress returns the encoded address of the first smart contract the transaction
// calls, or an empty string when it calls none (e.g. a deploy, which has no address yet)
func (txProc *txProcessor) smartContractAddress(tx *transaction.Transaction) string {
	for _, contract := range tx.GetRawData().GetContract() {
		if contract.GetType() != transaction.TXContract_SmartContractType {
			continue
		}

		sc, err := contract.GetSmartContract()
		if err != nil || len(sc.GetAddress()) == 0 {
			continue
		}

		if check.IfNil(txProc.pubkeyConv) {
			return hex.EncodeToString(sc.GetAddress())
		}

		return txProc.pubkeyConv.Encode(sc.GetAddress())
	}

	return ""
}

// scTimeoutBand returns the base smart contract execution timeout and the tolerance amount around
// it, normalised the way hostCore.configureTimeouts normalises them: a zero tolerance percentage
// takes the default and one above 100 is capped at 100. floorBase raises a base below
// core.MinSCExecutionTimeout to that floor, as the VM host does.
func (txProc *txProcessor) scTimeoutBand(floorBase bool) (time.Duration, time.Duration) {
	baseTimeout := time.Duration(txProc.cfg.VirtualMachine.Execution.TimeOutForSCExecutionInMilliseconds) * time.Millisecond
	if floorBase && baseTimeout < core.MinSCExecutionTimeout {
		baseTimeout = core.MinSCExecutionTimeout
	}

	tolerancePercentage := txProc.cfg.VirtualMachine.Execution.TimeOutTolerancePercentage
	if tolerancePercentage == 0 {
		tolerancePercentage = core.DefaultTolerancePercentage
	}
	if tolerancePercentage > 100 {
		log.Warn("TimeOutTolerancePercentage exceeds 100%, using 100%",
			"configured", tolerancePercentage)
		tolerancePercentage = 100
	}

	return baseTimeout, (baseTimeout * time.Duration(tolerancePercentage)) / 100
}

// validatorSCTimeout returns the timeout the VM host enforces in Validator mode (base + tolerance,
// hostCore.getEffectiveTimeout). The host floors the base unconditionally, so this does too. The
// execution time is measured around the whole VM call, which only returns after the deadline when
// it was cancelled, so a local timeout always measures at least this long.
func (txProc *txProcessor) validatorSCTimeout() time.Duration {
	baseTimeout, toleranceAmount := txProc.scTimeoutBand(true)
	return baseTimeout + toleranceAmount
}
