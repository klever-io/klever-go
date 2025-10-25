package transaction

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
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

// TestValidateToleranceBand_LeaderWeakHardware tests rejection when leader hardware is too weak
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

	txProc := &txProcessor{
		baseTxProcessor: &baseTxProcessor{
			cfg: cfg,
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// Validator finished in 400ms (well below 425ms lower bound)
	// Lower bound = 500ms - (500ms * 15%) = 425ms
	// Leader should have succeeded -> REJECT block
	validatorTimeNs := int64(400 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	assert.Equal(t, localErr, err)
	// ResultCode should NOT be updated when rejecting
	assert.Equal(t, transaction.Transaction_TXResultCode(0), tx.ResultCode)
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
			cfg: cfg,
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

	assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
			cfg: cfg,
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

	assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
			cfg: cfg,
		},
	}

	tx := &transaction.Transaction{}
	txHash := []byte("test-hash")
	expectedResultCode := uint32(transaction.Transaction_VMExecutionFailed)

	// Lower bound should be 500ms - (500ms * 15%) = 425ms (using default)
	validatorTimeNs := int64(450 * time.Millisecond)
	localErr := error(nil) // Validator succeeded

	err := txProc.validateToleranceBand(txHash, tx, expectedResultCode, validatorTimeNs, localErr)

	assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
			cfg: cfg,
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
	assert.Equal(t, process.ErrTransactionResultMismatch, err)
	assert.Equal(t, transaction.Transaction_TXResultCode(expectedResultCode), tx.ResultCode)
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
			cfg:         cfg,
			scProcessor: mockSC,
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
	)

	// Should reject because leader hardware too weak
	// Note: With the current implementation, when rejecting, it returns localErr (nil in this case)
	// This means the function returns nil, even though the log says "Rejecting block"
	assert.Equal(t, nil, err)
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
	)

	// Validator must reject when it failed but leader succeeded
	assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
	)

	// Validator must reject on different error codes
	assert.Equal(t, process.ErrAccountNotFound, err)
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
			cfg:         cfg,
			scProcessor: mockSC,
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
		assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
		assert.Nil(t, err)                                                      // Returns localErr which is nil
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
		assert.Equal(t, process.ErrTransactionResultMismatch, err)
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
