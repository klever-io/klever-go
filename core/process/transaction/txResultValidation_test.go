package transaction

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
)

// TestLogResultMismatch tests the logResultMismatch function
func TestLogResultMismatch(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{}

	// Test that the function doesn't panic and can be called
	txHash := []byte("test-hash-12345")
	txIndex := 5
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)
	actualResultCode := uint32(transaction.Transaction_Ok)

	// Call with different execution modes
	txProc.logResultMismatch(txHash, txIndex, expectedResultCode, actualResultCode, vmcommon.ExecutionModeValidator)
	txProc.logResultMismatch(txHash, txIndex, expectedResultCode, actualResultCode, vmcommon.ExecutionModeLeader)
	txProc.logResultMismatch(txHash, txIndex, expectedResultCode, actualResultCode, vmcommon.ExecutionModeQuery)

	// Function is for logging only, so we just verify it doesn't panic
}

// TestGetActualResultCode tests the getActualResultCode helper function
func TestGetActualResultCode(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{}

	t.Run("returns Ok when no error", func(t *testing.T) {
		tx := &transaction.Transaction{
			ResultCode: transaction.Transaction_VMExecutionFailed,
		}
		result := txProc.getActualResultCode(tx, nil)
		assert.Equal(t, uint32(transaction.Transaction_Ok), result)
	})

	t.Run("returns transaction ResultCode when error present", func(t *testing.T) {
		tx := &transaction.Transaction{
			ResultCode: transaction.Transaction_VMExecutionFailed,
		}
		result := txProc.getActualResultCode(tx, process.ErrTransactionResultMismatch)
		assert.Equal(t, uint32(transaction.Transaction_VMExecutionFailed), result)
	})
}

// TestValidateToleranceBand_LeaderWeakHardware tests rejection when leader hardware is too weak.
// KLR-63: a malicious proposer marks a locally-successful transaction as VMExecutionFailed in
// TxResults while publishing the *successful* state root, so the downstream verifyBlockTrieRoots
// comparison matches and cannot catch the lie. From FixAuditChangesV5 on, the rejection is an
// explicit non-nil error instead of the (always nil) localErr, so it propagates out of
// ProcessTransaction and rejects the block - and it has to be the sentinel processBlockTxs unwraps,
// or the block is dropped as a "bad tx" instead of rejected as a whole.
func TestValidateToleranceBand_LeaderWeakHardware(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500, // 500ms base timeout
				TimeOutTolerancePercentage:          15,  // 15% tolerance
			},
		},
	}

	// Validator finished in 400ms (well below 425ms lower bound)
	// Lower bound = 500ms - (500ms * 15%) = 425ms
	// Leader should have succeeded -> REJECT block
	validatorTimeNs := int64(400 * time.Millisecond)
	localErr := error(nil) // Validator succeeded
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)
	txHash := []byte("test-hash")

	t.Run("fork enabled: explicit mismatch error", func(t *testing.T) {
		t.Parallel()

		txProc := &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				cfg:            cfg,
				forkController: &forkControllerStub{fixAuditChangesV5: true},
			},
		}
		tx := &transaction.Transaction{}

		err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		// ResultCode must NOT be updated when rejecting
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
		assert.Equal(t, transaction.Transaction_TXResult(0), tx.Result)
	})

	t.Run("fork enabled: far below the bound", func(t *testing.T) {
		t.Parallel()

		txProc := &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				cfg:            cfg,
				forkController: &forkControllerStub{fixAuditChangesV5: true},
			},
		}
		tx := &transaction.Transaction{}

		// 10ms: nowhere near the 425ms lower bound, so no honest leader could have timed out on it
		err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, int64(10*time.Millisecond), localErr)

		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
		assert.Equal(t, transaction.Transaction_TXResult(0), tx.Result)
	})

	t.Run("fork disabled: legacy nil localErr preserved", func(t *testing.T) {
		t.Parallel()

		txProc := &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				cfg:            cfg,
				forkController: &forkControllerStub{fixAuditChangesV5: false},
			},
		}
		tx := &transaction.Transaction{}

		err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

		assert.Equal(t, localErr, err)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
	})
}

// TestValidateToleranceBand_LeaderRightToFail tests acceptance when leader had right to fail
func TestValidateToleranceBand_LeaderRightToFail(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500, // 500ms base timeout
				TimeOutTolerancePercentage:          15,  // 15% tolerance
			},
		},
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// Validator finished in 450ms (above 425ms lower bound)
	// Lower bound = 500ms - (500ms * 15%) = 425ms
	// Leader had right to fail -> ACCEPT block
	validatorTimeNs := int64(450 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
	// ResultCode SHOULD be updated to consensus value
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
}

// TestValidateToleranceBand_ExactlyAtLowerBound tests edge case at lower bound
func TestValidateToleranceBand_ExactlyAtLowerBound(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500, // 500ms base timeout
				TimeOutTolerancePercentage:          15,  // 15% tolerance
			},
		},
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// Validator finished exactly at 425ms (lower bound)
	// Lower bound = 500ms - (500ms * 15%) = 425ms
	// At boundary, leader had right to fail -> ACCEPT
	validatorTimeNs := int64(425 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
}

// TestValidateToleranceBand_DefaultTolerance tests default tolerance when not configured
func TestValidateToleranceBand_DefaultTolerance(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          0, // Not configured, should use default 15%
			},
		},
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// Lower bound should be 500ms - (500ms * 15%) = 425ms (using default)
	validatorTimeNs := int64(450 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
}

// TestValidateToleranceBand_ToleranceOver100 tests capping tolerance at 100%
func TestValidateToleranceBand_ToleranceOver100(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          150, // Invalid: >100%, should cap at 100%
			},
		},
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// With 100% tolerance, lower bound = 500ms - 500ms = 0ms
	// Any execution time should be accepted
	validatorTimeNs := int64(100 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	// Should accept (tolerance capped at 100%)
	assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
}

// TestValidateToleranceBand_UnconfiguredBaseTimeout covers the node that never set
// timeOutForSCExecutionInMilliseconds (or set it below the floor). The VM host clamps that config
// value to core.MinSCExecutionTimeout before executing, so the bound has to be derived from the
// same floor. Taking the raw 0 instead gives a zero lower bound, and since the call site only
// reaches here with validatorExecutionTimeNs > 0, no execution time can ever fall below it: the
// rejection becomes unreachable and the node stamps FAILED onto a transaction that succeeded.
func TestValidateToleranceBand_UnconfiguredBaseTimeout(t *testing.T) {
	t.Parallel()

	// Floored base = 400ms, tolerance 15% -> lower bound = 400ms - 60ms = 340ms
	newCfg := func(baseTimeoutMs uint32) config.Config {
		return config.Config{
			VirtualMachine: config.VirtualMachineServicesConfig{
				Execution: config.VirtualMachineConfig{
					TimeOutForSCExecutionInMilliseconds: baseTimeoutMs,
					TimeOutTolerancePercentage:          15,
				},
			},
		}
	}
	newTxProc := func(cfg config.Config, fixActive bool) *txProcessor {
		return &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				cfg:            cfg,
				forkController: &forkControllerStub{fixAuditChangesV5: fixActive},
			},
		}
	}

	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	t.Run("unset timeout still rejects a fast local success", func(t *testing.T) {
		t.Parallel()

		tx := &transaction.Transaction{}
		err := newTxProc(newCfg(0), true).
			validateToleranceBand(txHash, tx, expectedResultCode, int64(50*time.Millisecond), nil)

		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
	})

	t.Run("timeout below the floor is raised to it", func(t *testing.T) {
		t.Parallel()

		// 100ms configured: without the floor the bound would be 85ms and a 300ms local execution
		// would look like a justified leader timeout.
		tx := &transaction.Transaction{}
		err := newTxProc(newCfg(100), true).
			validateToleranceBand(txHash, tx, expectedResultCode, int64(300*time.Millisecond), nil)

		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
	})

	t.Run("unset timeout still accepts inside the floored band", func(t *testing.T) {
		t.Parallel()

		// 350ms is above the 340ms lower bound, so the leader keeps the right to fail: the floor
		// must not turn into a blanket rejection.
		tx := &transaction.Transaction{}
		err := newTxProc(newCfg(0), true).
			validateToleranceBand(txHash, tx, expectedResultCode, int64(350*time.Millisecond), nil)

		assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
		assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
	})

	t.Run("fork disabled: bound stays on the raw config value", func(t *testing.T) {
		t.Parallel()

		// Legacy behaviour, kept byte-identical for pre-fork replay: base 0 -> lower bound 0, so
		// the 50ms local success lands at or above it and the consensus failure is adopted.
		tx := &transaction.Transaction{}
		err := newTxProc(newCfg(0), false).
			validateToleranceBand(txHash, tx, expectedResultCode, int64(50*time.Millisecond), nil)

		assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
		assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
	})
}

// TestHandleResultMismatch_ValidatorSucceededLeaderFailed tests CASE 1
func TestHandleResultMismatch_ValidatorSucceededLeaderFailed(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	// Mock smart contract processor
	mockSC := &mockSmartContractProcessor{
		executionMode: vmcommon.ExecutionModeValidator,
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			scProcessor:    mockSC,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed) // Leader failed
	actualResultCode := uint32(transaction.Transaction_Ok)                  // Validator succeeded

	// Validator finished quickly (300ms < 425ms lower bound)
	validatorTimeNs := int64(300 * time.Millisecond)

	err := txProc.handleResultMismatch(
		txHash,
		0,
		tx,
		nil,
		expectedResultCode,
		actualResultCode,
		validatorTimeNs,
		vmcommon.ExecutionModeValidator,
	)

	// Rejects because leader hardware too weak, with an error that actually propagates (KLR-63)
	assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
	assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
}

// TestHandleResultMismatch_LeaderSucceededValidatorFailed tests CASE 2 for Validator
func TestHandleResultMismatch_LeaderSucceededValidatorFailed_Validator(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			scProcessor: &mockSmartContractProcessor{
				executionMode: vmcommon.ExecutionModeValidator,
			},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_Ok)              // Leader succeeded
	actualResultCode := uint32(transaction.Transaction_VMExecutionFailed) // Validator failed

	err := txProc.handleResultMismatch(
		txHash,
		0,
		tx,
		errors.New("contract invalid"),
		expectedResultCode,
		actualResultCode,
		0,
		vmcommon.ExecutionModeValidator,
	)

	// Validator must reject when it failed but leader succeeded
	assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
}

// TestHandleResultMismatch_BothFailedDifferentErrors_Validator tests CASE 3 for Validator
func TestHandleResultMismatch_BothFailedDifferentErrors_Validator(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			scProcessor: &mockSmartContractProcessor{
				executionMode: vmcommon.ExecutionModeValidator,
			},
		},
	}

	tx := &transaction.Transaction{
		ResultCode: transaction.Transaction_VMExecutionFailed,
	}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed) // Leader: timeout
	actualResultCode := uint32(transaction.Transaction_VMExecutionFailed)   // Validator: contract invalid

	err := txProc.handleResultMismatch(
		txHash,
		0,
		tx,
		process.ErrAccountNotFound,
		expectedResultCode,
		actualResultCode,
		0,
		vmcommon.ExecutionModeValidator,
	)

	// Validator must reject on different error codes
	assert.Equal(t, process.ErrAccountNotFound, err)
}

func TestHandleResultMismatch_ImportDB_ReproducesConsensusFailure(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:         cfg,
			scProcessor: &mockSmartContractProcessor{executionMode: vmcommon.ExecutionModeReplay},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("574536fff31288af9400c68bc6a28fc2366bce243ae1953b7b12600445a70034")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed) // network-agreed result
	actualResultCode := uint32(transaction.Transaction_Ok)                  // local re-execution succeeded

	// Local execution finished fast (~50ms, well below the 425ms lower bound). Under Observer
	// (import-db replay) the recorded failure is reproduced instead of re-judged by local timing.
	validatorTimeNs := int64(50 * time.Millisecond)

	err := txProc.handleResultMismatch(txHash, 0, tx, nil, expectedResultCode, actualResultCode, validatorTimeNs, vmcommon.ExecutionModeReplay)

	// Reproduces consensus: accept-leader + ResultCode stamped to the recorded failure, with no
	// dependence on local timing.
	assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
	assert.Equal(t, transaction.Transaction_FAILED, tx.Result)
}

// A fast local success in Validator mode routes through the tolerance band, not the Observer
// short-circuit, so the block is rejected and the consensus ResultCode is never stamped.
func TestHandleResultMismatch_ValidatorMode_FastLocalSuccessRejects(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}
	// Validator mode (live validation path)

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			scProcessor:    &mockSmartContractProcessor{executionMode: vmcommon.ExecutionModeValidator},
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)
	actualResultCode := uint32(transaction.Transaction_Ok)
	validatorTimeNs := int64(50 * time.Millisecond)

	err := txProc.handleResultMismatch(txHash, 0, tx, nil, expectedResultCode, actualResultCode, validatorTimeNs, vmcommon.ExecutionModeValidator)

	// Live path still routes through the tolerance band (not the Observer short-circuit): the fast
	// local success rejects the block and never stamps the consensus ResultCode.
	assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
	assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
}

// TestHandleResultMismatch_ImportDB_Case2And3Untouched asserts the Observer guard is scoped to
// CASE 1: even in Observer (import-db) mode, a recorded success with a local failure (CASE 2) still
// rejects, and two differing failures (CASE 3) still bubble up the local error.
func TestHandleResultMismatch_ImportDB_Case2And3Untouched(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			scProcessor: &mockSmartContractProcessor{executionMode: vmcommon.ExecutionModeReplay},
		},
	}

	okCode := uint32(transaction.Transaction_Ok)
	failCode := uint32(transaction.Transaction_VMExecutionFailed)

	// CASE 2: consensus Ok, local failed -> still rejects
	tx2 := &transaction.Transaction{ResultCode: transaction.Transaction_VMExecutionFailed}
	err2 := txProc.handleResultMismatch([]byte("h2"), 0, tx2, errors.New("contract invalid"), okCode, failCode, 0, vmcommon.ExecutionModeReplay)
	assert.Equal(t, process.ErrTransactionResultMismatch, err2)

	// CASE 3: both failed, different codes -> still bubbles up local error
	tx3 := &transaction.Transaction{ResultCode: transaction.Transaction_VMExecutionFailed}
	err3 := txProc.handleResultMismatch([]byte("h3"), 0, tx3, process.ErrAccountNotFound, failCode, failCode, 0, vmcommon.ExecutionModeReplay)
	assert.Equal(t, process.ErrAccountNotFound, err3)
}

// TestValidateTransactionResult_NoTxResults tests skip validation when TxResults empty
func TestValidateTransactionResult_NoTxResults(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{}

	blk := &block.Block{
		TxHashes:  [][]byte{[]byte("hash1")},
		TxResults: []uint32{}, // Empty TxResults
	}

	tx := &transaction.Transaction{}
	localErr := errors.New("contract invalid")

	err := txProc.validateTransactionResult(blk, []byte("hash1"), tx, localErr, 100000000)

	// Should return local error unchanged (skip validation)
	assert.Equal(t, localErr, err)
}

// TestValidateTransactionResult_TxNotFound tests skip validation when tx not in block
func TestValidateTransactionResult_TxNotFound(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{}

	blk := &block.Block{
		TxHashes:  [][]byte{[]byte("hash1")},
		TxResults: []uint32{uint32(transaction.Transaction_Ok)},
	}

	tx := &transaction.Transaction{}
	localErr := errors.New("contract invalid")

	// Request validation for tx not in block
	err := txProc.validateTransactionResult(blk, []byte("different-hash"), tx, localErr, 100000000)

	// Should return local error unchanged (skip validation)
	assert.Equal(t, localErr, err)
}

// TestValidateTransactionResult_ResultsMatch tests happy path when results match
func TestValidateTransactionResult_ResultsMatch(t *testing.T) {
	t.Parallel()

	txProc := &txProcessor{}

	blk := &block.Block{
		TxHashes:  [][]byte{[]byte("hash1")},
		TxResults: []uint32{uint32(transaction.Transaction_Ok)},
	}

	tx := &transaction.Transaction{}
	localErr := error(nil) // Local execution succeeded

	err := txProc.validateTransactionResult(blk, []byte("hash1"), tx, localErr, 100000000)

	// Should return nil (no validation needed, results match)
	assert.Nil(t, err)
}

// TestValidateTransactionResult_CallsHandleResultMismatch tests the full flow triggering handleResultMismatch
func TestValidateTransactionResult_CallsHandleResultMismatch(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	mockSC := &mockSmartContractProcessor{
		executionMode: vmcommon.ExecutionModeValidator,
	}

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg:            cfg,
			scProcessor:    mockSC,
			forkController: &forkControllerStub{fixAuditChangesV5: true},
		},
	}

	t.Run("validator succeeded, leader failed - within tolerance", func(t *testing.T) {
		txHash := []byte("tx-hash-123")
		blk := &block.Block{
			TxHashes:  [][]byte{txHash},
			TxResults: []uint32{uint32(transaction.Transaction_VMExecutionFailed)}, // Leader failed
		}

		tx := &transaction.Transaction{}
		localErr := error(nil)                           // Validator succeeded
		validatorTimeNs := int64(450 * time.Millisecond) // Within tolerance band (>425ms)

		err := txProc.validateTransactionResult(blk, txHash, tx, localErr, validatorTimeNs)

		// Should accept (leader had right to fail) and update tx.ResultCode
		assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
		assert.Equal(t, transaction.Transaction_VMExecutionFailed, tx.ResultCode)
	})

	t.Run("validator succeeded, leader failed - outside tolerance", func(t *testing.T) {
		txHash := []byte("tx-hash-456")
		blk := &block.Block{
			TxHashes:  [][]byte{txHash},
			TxResults: []uint32{uint32(transaction.Transaction_VMExecutionFailed)}, // Leader failed
		}

		tx := &transaction.Transaction{}
		localErr := error(nil)                           // Validator succeeded
		validatorTimeNs := int64(300 * time.Millisecond) // Below tolerance band (<425ms)

		err := txProc.validateTransactionResult(blk, txHash, tx, localErr, validatorTimeNs)

		// Should reject (leader hardware too weak)
		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode) // Not updated
	})

	t.Run("leader succeeded, validator failed", func(t *testing.T) {
		txHash := []byte("tx-hash-789")
		blk := &block.Block{
			TxHashes:  [][]byte{txHash},
			TxResults: []uint32{uint32(transaction.Transaction_Ok)}, // Leader succeeded
		}

		tx := &transaction.Transaction{
			ResultCode: transaction.Transaction_VMExecutionFailed, // Validator failed
		}
		localErr := errors.New("validator execution failed")
		validatorTimeNs := int64(300 * time.Millisecond)

		err := txProc.validateTransactionResult(blk, txHash, tx, localErr, validatorTimeNs)

		// Should reject (validator must reject when it fails but leader succeeded)
		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
	})

	t.Run("both failed with different errors", func(t *testing.T) {
		txHash := []byte("tx-hash-abc")
		blk := &block.Block{
			TxHashes:  [][]byte{txHash},
			TxResults: []uint32{uint32(transaction.Transaction_VMExecutionFailed)}, // Leader failed
		}

		tx := &transaction.Transaction{
			ResultCode: transaction.Transaction_VMExecutionFailed, // Validator also failed
		}
		localErr := process.ErrAccountNotFound // Different error
		validatorTimeNs := int64(300 * time.Millisecond)

		err := txProc.validateTransactionResult(blk, txHash, tx, localErr, validatorTimeNs)

		// Should return the local error (validator rejects)
		assert.Equal(t, process.ErrAccountNotFound, err)
	})
}

// Mock smart contract processor for testing
// TestValidateTransactionResult_ObserverModePlumbing pins the full path
// validateTransactionResult -> scProcessor.GetVMExecutionMode() -> handleResultMismatch CASE 1.
func TestValidateTransactionResult_ObserverModePlumbing(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	newTxProc := func(mode vmcommon.ExecutionMode) *txProcessor {
		return &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				cfg:            cfg,
				scProcessor:    &mockSmartContractProcessor{executionMode: mode},
				forkController: &forkControllerStub{fixAuditChangesV5: true},
			},
		}
	}

	txHash := []byte("574536fff31288af9400c68bc6a28fc2366bce243ae1953b7b12600445a70034")
	newBlock := func() *block.Block {
		return &block.Block{
			TxHashes:  [][]byte{txHash},
			TxResults: []uint32{uint32(transaction.Transaction_VMExecutionFailed)}, // network recorded a timeout fail
		}
	}
	fastLocal := int64(50 * time.Millisecond) // deep inside the 425ms lower bound

	t.Run("Observer reproduces the recorded failure", func(t *testing.T) {
		tx := &transaction.Transaction{} // local execution succeeded (localErr nil -> Ok)
		err := newTxProc(vmcommon.ExecutionModeReplay).
			validateTransactionResult(newBlock(), txHash, tx, nil, fastLocal)

		// Only holds if GetVMExecutionMode()==Observer reaches CASE 1.
		assert.Equal(t, process.ErrTransactionResultMismatchAcceptLeader, err)
		assert.Equal(t, transaction.Transaction_VMExecutionFailed, tx.ResultCode)
	})

	t.Run("Validator at the same 50ms rejects via the tolerance band", func(t *testing.T) {
		tx := &transaction.Transaction{}
		err := newTxProc(vmcommon.ExecutionModeValidator).
			validateTransactionResult(newBlock(), txHash, tx, nil, fastLocal)

		// Same input, opposite outcome from Observer: the leader had no right to fail this fast,
		// so the block is rejected and the consensus ResultCode is never adopted.
		assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
		assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
	})
}

// forkControllerStub overrides only the flag under test. Every other core.ForkController method is
// promoted from the embedded nil interface, so an unexpected call panics instead of passing silently.
type forkControllerStub struct {
	core.ForkController
	fixAuditChangesV5 bool
}

func (f *forkControllerStub) FixAuditChangesV5() bool {
	return f.fixAuditChangesV5
}

type mockSmartContractProcessor struct {
	executionMode vmcommon.ExecutionMode
}

func (m *mockSmartContractProcessor) GetVMExecutionMode() vmcommon.ExecutionMode {
	return m.executionMode
}

func (m *mockSmartContractProcessor) SetVMExecutionMode(mode vmcommon.ExecutionMode) {
	m.executionMode = mode
}

func (m *mockSmartContractProcessor) ExecuteSmartContractTransaction(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
	return vmcommon.Ok, nil
}

func (m *mockSmartContractProcessor) DeploySmartContract(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
	return vmcommon.Ok, nil
}

func (m *mockSmartContractProcessor) ProcessIfError(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error {
	return nil
}

func (m *mockSmartContractProcessor) IsPayable(sndAddress []byte, recvAddress []byte) (bool, error) {
	return true, nil
}

func (m *mockSmartContractProcessor) LastBlock() data.HeaderHandler {
	return nil
}

func (m *mockSmartContractProcessor) IsInterfaceNil() bool {
	return m == nil
}

// TestHandleResultMismatch_TimeoutOutcomesByMode pins the outcome of every timeout-related mismatch
// per execution mode. Only the two live (Validator) timeout disagreements carry the distinct
// sentinels consensus reports on; both still match ErrTransactionResultMismatch, so every caller
// keeps rejecting the block. Replay (agreed history) and the deterministic cases are unchanged.
func TestHandleResultMismatch_TimeoutOutcomesByMode(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500,
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	okCode := uint32(transaction.Transaction_Ok)
	timeoutCode := uint32(transaction.Transaction_VMExecutionFailed)
	userErrCode := uint32(transaction.Transaction_VMUserError)
	contractAddr := []byte("contract-address-0123456789abcde")
	txHash := []byte("tx-hash")

	newSCTx := func(resultCode transaction.Transaction_TXResultCode) *transaction.Transaction {
		param, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCInvoke, Address: contractAddr})
		if err != nil {
			t.Fatal(err)
		}
		return &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: param}},
			},
			ResultCode: resultCode,
		}
	}

	localTimeoutErr := errors.New("execution timed out")
	fast := int64(50 * time.Millisecond)    // far below the 425ms lower bound
	inBand := int64(450 * time.Millisecond) // inside the tolerance band
	// the VM cancels a Validator-mode call at base + tolerance: 500ms + 15% = 575ms, and with an
	// unset timeout at the 400ms floor + 15% = 460ms; a timed-out call measures at least that
	atValidatorTimeout := int64(575 * time.Millisecond)
	atFlooredValidatorTimeout := int64(460 * time.Millisecond)

	tests := []struct {
		name          string
		mode          vmcommon.ExecutionMode
		expected      uint32
		actual        uint32
		localErr      error
		localResult   transaction.Transaction_TXResultCode
		execTimeNs    int64
		unsetTimeout  bool  // TimeOutForSCExecutionInMilliseconds left at 0
		wantExact     error // compared with assert.Equal when set
		wantIs        []error
		wantNotIs     []error
		wantContract  bool
		wantStampedRC bool
	}{
		{
			name: "CASE 2 validator: local timeout at the validator timeout, leader ok -> abstain", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			execTimeNs:   atValidatorTimeout,
			wantIs:       []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrTransactionResultMismatch},
			wantNotIs:    []error{process.ErrLeaderTimeoutRefused},
			wantContract: true,
		},
		{
			name: "CASE 2 validator: local timeout past the validator timeout -> abstain", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			execTimeNs:   int64(600 * time.Millisecond),
			wantIs:       []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrTransactionResultMismatch},
			wantContract: true,
		},
		{
			name: "CASE 2 validator: VMExecutionFailed before the validator timeout is not a timeout -> plain mismatch", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			execTimeNs: atValidatorTimeout - 1,
			wantExact:  process.ErrTransactionResultMismatch,
			wantNotIs:  []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrLeaderTimeoutRefused},
		},
		{
			name: "CASE 2 validator: unset timeout uses the VM floor, at the floored validator timeout -> abstain", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			unsetTimeout: true, execTimeNs: atFlooredValidatorTimeout,
			wantIs:       []error{process.ErrLocalExecutionTimeoutAbstain},
			wantContract: true,
		},
		{
			name: "CASE 2 validator: unset timeout uses the VM floor, just before it -> plain mismatch", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			unsetTimeout: true, execTimeNs: atFlooredValidatorTimeout - 1,
			wantExact: process.ErrTransactionResultMismatch,
			wantNotIs: []error{process.ErrLocalExecutionTimeoutAbstain},
		},
		{
			name: "CASE 2 replay: unchanged plain mismatch", mode: vmcommon.ExecutionModeReplay,
			expected: okCode, actual: timeoutCode, localErr: localTimeoutErr, localResult: transaction.Transaction_VMExecutionFailed,
			execTimeNs: int64(5 * time.Second),
			wantExact:  process.ErrTransactionResultMismatch,
			wantNotIs:  []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrLeaderTimeoutRefused},
		},
		{
			name: "CASE 1a validator below lower bound -> refused", mode: vmcommon.ExecutionModeValidator,
			expected: timeoutCode, actual: okCode, execTimeNs: fast,
			wantIs:       []error{process.ErrLeaderTimeoutRefused, process.ErrTransactionResultMismatch},
			wantNotIs:    []error{process.ErrLocalExecutionTimeoutAbstain},
			wantContract: true,
		},
		{
			name: "CASE 1a validator inside band -> accept leader", mode: vmcommon.ExecutionModeValidator,
			expected: timeoutCode, actual: okCode, execTimeNs: inBand,
			wantExact: process.ErrTransactionResultMismatchAcceptLeader, wantStampedRC: true,
		},
		{
			name: "CASE 1a replay fast -> accept recorded timeout", mode: vmcommon.ExecutionModeReplay,
			expected: timeoutCode, actual: okCode, execTimeNs: fast,
			wantExact: process.ErrTransactionResultMismatchAcceptLeader, wantStampedRC: true,
		},
		{
			name: "CASE 1b validator: deterministic failure code rejects", mode: vmcommon.ExecutionModeValidator,
			expected: userErrCode, actual: okCode, execTimeNs: inBand,
			wantExact: process.ErrTransactionResultMismatch,
			wantNotIs: []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrLeaderTimeoutRefused},
		},
		{
			name: "CASE 1b replay: deterministic failure code rejects", mode: vmcommon.ExecutionModeReplay,
			expected: userErrCode, actual: okCode, execTimeNs: inBand,
			wantExact: process.ErrTransactionResultMismatch,
			wantNotIs: []error{process.ErrLocalExecutionTimeoutAbstain, process.ErrLeaderTimeoutRefused},
		},
		{
			name: "CASE 3 validator: both failed differently -> local error", mode: vmcommon.ExecutionModeValidator,
			expected: timeoutCode, actual: userErrCode, localErr: process.ErrAccountNotFound, localResult: transaction.Transaction_VMUserError,
			wantExact: process.ErrAccountNotFound,
		},
		{
			name: "non-timeout local failure, leader ok -> local error", mode: vmcommon.ExecutionModeValidator,
			expected: okCode, actual: userErrCode, localErr: process.ErrAccountNotFound, localResult: transaction.Transaction_VMUserError,
			wantExact: process.ErrAccountNotFound,
			wantNotIs: []error{process.ErrLocalExecutionTimeoutAbstain},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rowCfg := cfg
			if tt.unsetTimeout {
				rowCfg.VirtualMachine.Execution.TimeOutForSCExecutionInMilliseconds = 0
			}
			txProc := &txProcessor{
				baseTxProcessor: &baseTxProcessor{
					cfg:            rowCfg,
					scProcessor:    &mockSmartContractProcessor{executionMode: tt.mode},
					forkController: &forkControllerStub{fixAuditChangesV5: true},
				},
			}
			tx := newSCTx(tt.localResult)

			err := txProc.handleResultMismatch(txHash, 0, tx, tt.localErr, tt.expected, tt.actual, tt.execTimeNs, tt.mode)

			assertMismatchOutcome(t, err, tx, mismatchOutcome{
				exact: tt.wantExact, is: tt.wantIs, notIs: tt.wantNotIs,
				contract: tt.wantContract, stampedRC: tt.wantStampedRC, localResult: tt.localResult,
				txHash: txHash, contractAddr: contractAddr,
			})
		})
	}
}

type mismatchOutcome struct {
	exact        error // compared with assert.Equal when set
	is           []error
	notIs        []error
	contract     bool
	stampedRC    bool
	localResult  transaction.Transaction_TXResultCode
	txHash       []byte
	contractAddr []byte
}

func assertMismatchOutcome(t *testing.T, err error, tx *transaction.Transaction, want mismatchOutcome) {
	t.Helper()

	if want.exact != nil {
		assert.Equal(t, want.exact, err)
	}
	for _, target := range want.is {
		assert.ErrorIs(t, err, target)
	}
	for _, target := range want.notIs {
		assert.NotErrorIs(t, err, target)
	}

	var mismatch *process.TxResultMismatchError
	if want.contract {
		if assert.ErrorAs(t, err, &mismatch) {
			assert.Equal(t, want.txHash, mismatch.TxHash)
			assert.Equal(t, hex.EncodeToString(want.contractAddr), mismatch.Contract)
		}
	} else {
		assert.False(t, errors.As(err, &mismatch), "only live timeout disagreements carry the contract")
	}

	if want.stampedRC {
		assert.Equal(t, transaction.Transaction_VMExecutionFailed, tx.ResultCode)
	} else {
		assert.Equal(t, want.localResult, tx.ResultCode)
	}
}
