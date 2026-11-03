package transaction_test

import (
	"errors"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessTransaction_TxResultsValidation_ResultsMatch tests the validation path
// when local SC execution result matches consensus result from block.TxResults.
// This is the happy path - covers the validation logic in ProcessTransaction where block.TxResults
// is compared with local execution results when execution time is set.
func TestProcessTransaction_TxResultsValidation_ResultsMatch(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor to simulate WASM execution that naturally sets execution time
	// The real SC processor (smartContract/process.go) sets execution time after contract execution
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			// Real SC processor sets execution time after executing the contract
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.Ok, nil // Success (result code 0)
		},
	}

	execTx := NewTXProcessor(t, args)

	// Setup accounts - SC contract address and owner
	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	// Create SmartContract invoke transaction
	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")} // Required for SC processing

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Create block with TxResults and TxHashes
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{0} // Consensus: success (matches local execution)

	// Process transaction - triggers validation path in ProcessTransaction
	// Flow: processContracts → invokeSC → ExecuteSmartContractTransaction (sets exec time) → validateTransactionResult
	err = execTx.ProcessTransaction(block, hash, tx)

	// Should succeed - results match consensus
	assert.Nil(t, err, "Transaction should succeed when results match consensus")
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
}

// TestProcessTransaction_TxResultsValidation_LocalFailConsensusSuccess tests
// when local execution fails but consensus says it succeeded.
// Validator should accept consensus result.
func TestProcessTransaction_TxResultsValidation_LocalFailConsensusSuccess(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor to simulate failed execution
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.VMUserError, nil // Local fails (result code 57)
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Consensus says success, but local execution failed
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{0} // Consensus: success

	// Process - local returns 57 (VMUserError), consensus expects 0 → MISMATCH
	err = execTx.ProcessTransaction(block, hash, tx)

	// Should detect mismatch - validator must follow consensus or reject
	assert.NotNil(t, err, "Should return error indicating result mismatch")
}

// TestProcessTransaction_TxResultsValidation_LocalSuccessConsensusFail pins KLR-63 CASE 1b: a
// proposer that marks a locally successful transaction with a *deterministic* failure code instead
// of the timeout code. VMUserError (57), VMOutOfGas (58) and Fail (99) cannot be produced by a slow
// leader, so there is no timing argument to weigh - the block is rejected outright and the forged
// code is never stamped onto the transaction. Before this fix these codes matched neither CASE 1
// (keyed to VMExecutionFailed) nor CASE 2 and fell through to the always-nil localErr, so the block
// was accepted while carrying a result contradicting the state it committed.
func TestProcessTransaction_TxResultsValidation_LocalSuccessConsensusFail(t *testing.T) {
	t.Parallel()

	// Local execution succeeds; the execution time is irrelevant on this path, so it is set well
	// inside the tolerance band to prove the rejection carries no timing precondition.
	newProcessor := func(t *testing.T, mode vmcommon.ExecutionMode, enableEpochs *config.EnableEpochs) (process.TransactionProcessor, *transaction.Transaction, []byte) {
		args := createArgsForTxProcessor()
		args.ScProcessor = &commonMock.SCProcessorMock{
			ExecutionMode: mode,
			ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
				ctx.SetExecutionTime(100 * time.Millisecond)
				return vmcommon.Ok, nil // Local succeeds (result code 0)
			},
		}
		if enableEpochs != nil {
			forkController, err := fork.NewForkController(*enableEpochs, &commonMock.EpochNotifierStub{})
			require.Nil(t, err)
			args.ForkController = forkController
		}

		execTx := NewTXProcessor(t, args)

		AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
		AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
		_ = args.AccountsCacher.SaveAll()

		scContract := transaction.SmartContract{
			Type:    transaction.SmartContract_SCInvoke,
			Address: testToAddress,
		}
		tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
		tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

		_, hash, err := execTx.PreProcessTransaction(tx)
		require.Nil(t, err)

		return execTx, tx, hash
	}

	deterministicFailures := []struct {
		name string
		code transaction.Transaction_TXResultCode
	}{
		{name: "VMUserError", code: transaction.Transaction_VMUserError},
		{name: "VMOutOfGas", code: transaction.Transaction_VMOutOfGas},
		{name: "Fail", code: transaction.Transaction_Fail},
	}

	for _, tc := range deterministicFailures {
		t.Run(tc.name+" rejects the block", func(t *testing.T) {
			t.Parallel()

			execTx, tx, hash := newProcessor(t, vmcommon.ExecutionModeValidator, nil)

			blk := createBlockHeader()
			blk.TxHashes = [][]byte{hash}
			blk.TxResults = []uint32{uint32(tc.code)}

			err := execTx.ProcessTransaction(blk, hash, tx)

			require.Error(t, err, "a forged non-timeout failure must not report success to processBlockTxs")
			assert.True(t, errors.Is(err, process.ErrTransactionResultMismatch),
				"the block is only rejected as a whole when the sentinel survives ProcessTransaction, got: %v", err)
			// The forged consensus result must never be adopted
			assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
		})
	}

	// The replay short-circuit is scoped to the timeout code: a deterministic failure that local
	// re-execution contradicts is a real determinism break, not agreed history to reproduce.
	t.Run("Observer rejects too", func(t *testing.T) {
		t.Parallel()

		execTx, tx, hash := newProcessor(t, vmcommon.ExecutionModeReplay, nil)

		blk := createBlockHeader()
		blk.TxHashes = [][]byte{hash}
		blk.TxResults = []uint32{uint32(transaction.Transaction_VMUserError)}

		err := execTx.ProcessTransaction(blk, hash, tx)

		require.Error(t, err)
		assert.True(t, errors.Is(err, process.ErrTransactionResultMismatch), "got: %v", err)
		assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
	})

	// Pre-fork the legacy always-nil localErr is returned, so replay of historical blocks is
	// byte-identical. Deleting the gate would break it silently without this case.
	t.Run("fork disabled: legacy acceptance preserved", func(t *testing.T) {
		t.Parallel()

		execTx, tx, hash := newProcessor(t, vmcommon.ExecutionModeValidator, &config.EnableEpochs{FixAuditChangesV5: 1000})

		blk := createBlockHeader()
		blk.TxHashes = [][]byte{hash}
		blk.TxResults = []uint32{uint32(transaction.Transaction_VMUserError)}

		err := execTx.ProcessTransaction(blk, hash, tx)

		assert.Nil(t, err, "pre-fork behaviour must be unchanged")
		assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
	})
}

// TestProcessTransaction_TxResultsValidation_BothFailDifferentErrors tests
// when both local and consensus fail but with different error codes.
func TestProcessTransaction_TxResultsValidation_BothFailDifferentErrors(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor to simulate OutOfGas error
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.VMOutOfGas, nil // Local: OutOfGas (result code 58)
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Consensus says VMUserError, local gets VMOutOfGas
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{57} // Consensus: VMUserError (different from local VMOutOfGas)

	// Process - local returns 58 (VMOutOfGas), consensus expects 57 (VMUserError) → MISMATCH
	err = execTx.ProcessTransaction(block, hash, tx)

	// Both failed, but with different errors - should detect mismatch
	assert.NotNil(t, err, "Should detect mismatch when error codes differ")
}

// TestProcessTransaction_TxResultsValidation_NoTxResults tests
// when block has no TxResults - validation should be skipped.
func TestProcessTransaction_TxResultsValidation_NoTxResults(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.Ok, nil
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Block has NO TxResults - validation should be skipped
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{} // Empty - validation skipped (len(block.TxResults) == 0)

	err = execTx.ProcessTransaction(block, hash, tx)

	// Should succeed without validation
	assert.Nil(t, err, "Should succeed when no TxResults to validate against")
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
}

// TestProcessTransaction_TxResultsValidation_NoExecutionTime tests
// when block has TxResults but execution time is 0 - validation should be skipped.
func TestProcessTransaction_TxResultsValidation_NoExecutionTime(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor that does NOT set execution time
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			// Note: NOT calling ctx.SetExecutionTime()
			return vmcommon.Ok, nil
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Block has TxResults but execution time is 0
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{0}

	err = execTx.ProcessTransaction(block, hash, tx)

	// Should succeed - validation skipped because execution time was not set (validatorExecTime == 0)
	assert.Nil(t, err, "Should succeed when execution time is not set")
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
}

// KLC-2722: a successful deploy reaches validateTransactionResult only with an execution time (set from V7).
func TestProcessTransaction_TxResultsValidation_DeployTimeout(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, mode vmcommon.ExecutionMode, execTime time.Duration, recorded transaction.Transaction_TXResultCode) (error, *transaction.Transaction) {
		args := createArgsForTxProcessor()
		deployCalled := false
		args.ScProcessor = &commonMock.SCProcessorMock{
			ExecutionMode: mode,
			DeploySmartContractCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
				deployCalled = true
				if execTime > 0 {
					ctx.SetExecutionTime(execTime)
				}
				return vmcommon.Ok, nil // local deploy succeeds
			},
		}

		execTx := NewTXProcessor(t, args)

		AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
		_ = args.AccountsCacher.SaveAll()

		scContract := transaction.SmartContract{Type: transaction.SmartContract_SCDeploy}
		tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
		tx.RawData.Data = [][]byte{[]byte("code@0500@0000")}

		_, hash, err := execTx.PreProcessTransaction(tx)
		require.Nil(t, err)

		blk := createBlockHeader()
		blk.TxHashes = [][]byte{hash}
		blk.TxResults = []uint32{uint32(recorded)}

		err = execTx.ProcessTransaction(blk, hash, tx)
		require.True(t, deployCalled, "the deploy path must run")

		return err, tx
	}

	// The tolerance band itself is pinned in txResultValidation_test.go; these cases pin that a
	// deploy reaches the check, and which way a recorded 62 and a recorded 57 go once it does.
	t.Run("a recorded timeout (62) is reproduced on replay", func(t *testing.T) {
		t.Parallel()

		err, tx := run(t, vmcommon.ExecutionModeReplay, 50*time.Millisecond, transaction.Transaction_VMExecutionFailed)

		assert.True(t, errors.Is(err, process.ErrTransactionResultMismatchAcceptLeader), "got: %v", err)
		assert.Equal(t, transaction.Transaction_VMExecutionFailed, tx.ResultCode)
	})

	for name, mode := range map[string]vmcommon.ExecutionMode{"replay": vmcommon.ExecutionModeReplay, "validator": vmcommon.ExecutionModeValidator} {
		t.Run("a recorded VMUserError (57) rejects the block on "+name, func(t *testing.T) {
			t.Parallel()

			err, tx := run(t, mode, 450*time.Millisecond, transaction.Transaction_VMUserError)

			assert.True(t, errors.Is(err, process.ErrTransactionResultMismatch), "got: %v", err)
			assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
		})
	}

	t.Run("without an execution time (pre-V7) the check is skipped", func(t *testing.T) {
		t.Parallel()

		err, tx := run(t, vmcommon.ExecutionModeReplay, 0, transaction.Transaction_VMExecutionFailed)

		assert.Nil(t, err, "no execution time, no result check: the state root decides")
		assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
	})
}

// TestProcessTransaction_TxResultsValidation_TxNotInBlock tests
// when transaction hash is not in block's TxHashes - validation should be skipped.
func TestProcessTransaction_TxResultsValidation_TxNotInBlock(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.Ok, nil
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	// Block has TxResults but transaction hash is NOT in TxHashes
	block := createBlockHeader()
	block.TxHashes = [][]byte{[]byte("different-hash")} // Different hash
	block.TxResults = []uint32{0}

	err = execTx.ProcessTransaction(block, hash, tx)

	// Should succeed - validation skipped because tx not found in block (findTxIndexInBlock returns -1)
	assert.Nil(t, err, "Should succeed when transaction not found in block")
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
}

// TestProcessTransaction_TxResultsValidation_FastLocalSuccessRejectsBlock pins the end of the
// KLR-63 path: a local success far below the tolerance band contradicts a consensus
// VMExecutionFailed, and the rejection has to survive ProcessTransaction as the sentinel
// processBlockTxs unwraps. The sentinel travels through the shared kAppFeeErr variable and an error
// path that resets receipts and reverts fees, so wrapping it anywhere in between silently disables
// the fix - the unit tests on validateToleranceBand would stay green.
func TestProcessTransaction_TxResultsValidation_FastLocalSuccessRejectsBlock(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	// Without a configured timeout the lower bound is zero and no execution time can fall below it
	args.Cfg = config.Config{
		VirtualMachine: config.VirtualMachineServicesConfig{
			Execution: config.VirtualMachineConfig{
				TimeOutForSCExecutionInMilliseconds: 500, // lower bound = 500ms - 15% = 425ms
				TimeOutTolerancePercentage:          15,
			},
		},
	}

	// Live validation path, and 10ms is nowhere near the 425ms bound: no honest leader could have
	// timed out on this transaction
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecutionMode: vmcommon.ExecutionModeValidator,
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(10 * time.Millisecond)
			return vmcommon.Ok, nil
		},
	}

	execTx := NewTXProcessor(t, args)

	AddBalanceAccount(args.AccountsCacher, 10_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 0, nil, testToAddress)
	_ = args.AccountsCacher.SaveAll()

	scContract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: testToAddress,
	}
	tx, _ := createTransactionMock(&scContract, transaction.TXContract_SmartContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("invokeFunction")}

	_, hash, err := execTx.PreProcessTransaction(tx)
	require.Nil(t, err)

	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{uint32(transaction.Transaction_VMExecutionFailed)}

	err = execTx.ProcessTransaction(block, hash, tx)

	require.Error(t, err, "a fast local success must not report success to processBlockTxs")
	assert.True(t, errors.Is(err, process.ErrTransactionResultMismatch),
		"the block is only rejected as a whole when the sentinel survives ProcessTransaction, got: %v", err)
	// The forged consensus result must never be adopted
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
}
