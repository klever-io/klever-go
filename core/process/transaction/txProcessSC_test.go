package transaction_test

import (
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
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

// TestProcessTransaction_TxResultsValidation_LocalSuccessConsensusFail tests
// when local execution succeeds but consensus says it failed.
func TestProcessTransaction_TxResultsValidation_LocalSuccessConsensusFail(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()

	// Mock SC processor to simulate successful execution
	args.ScProcessor = &commonMock.SCProcessorMock{
		ExecuteSmartContractTransactionCalled: func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			ctx.SetExecutionTime(100 * time.Millisecond)
			return vmcommon.Ok, nil // Local succeeds (result code 0)
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

	// Consensus says failure, but local execution succeeded
	block := createBlockHeader()
	block.TxHashes = [][]byte{hash}
	block.TxResults = []uint32{57} // Consensus: VMUserError

	// Process - local returns 0 (Ok), consensus expects 57 → MISMATCH
	err = execTx.ProcessTransaction(block, hash, tx)

	// When local succeeds but consensus fails, the validation detects the mismatch
	// but the transaction still succeeds locally with result code 0
	// The mismatch is logged for monitoring purposes
	assert.Nil(t, err, "Transaction completes successfully despite mismatch")
	// Local execution succeeded, so ResultCode remains 0
	assert.Equal(t, transaction.Transaction_Ok, tx.ResultCode)
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
