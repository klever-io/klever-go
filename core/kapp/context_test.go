package kapp_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/kapp"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

// TestKappContext_ReceiptsAccumulation tests that receipts are accumulated during contract processing
// In real scenarios (txProcess.go:processContracts), when a transaction has multiple contracts,
// each contract can generate receipts that are accumulated in the context and later attached
// to the transaction (txProcess.go:370)
func TestKappContext_ReceiptsAccumulation(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender1"),
		ContractID:     0,
		ContractType:   0,
		Block:          &block.Block{},
	})

	// Simulate first contract execution generating a receipt
	receipt1 := txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		[]byte("sender1"),
	)
	ctx.Receipts().Add(receipt1)

	assert.Equal(t, 1, len(ctx.Receipts().Get()), "should have 1 receipt after first contract")

	// Simulate second contract execution generating another receipt
	ctx.SetContractID(1) // Move to next contract (like txProcess.go:392)
	receipt2 := txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		[]byte("receiver1"),
	)
	ctx.Receipts().Add(receipt2)

	receipts := ctx.Receipts().Get()
	assert.Equal(t, 2, len(receipts), "should accumulate receipts from multiple contracts")

	// Verify we got both receipts back
	assert.Equal(t, receipt1, receipts[0])
	assert.Equal(t, receipt2, receipts[1])

	// In real code, these receipts would be appended to tx.Receipts (txProcess.go:370)
}

// TestKappContext_ExecutionTimeStorage tests that execution time can be stored and retrieved.
// In production: smartContract/process.go:266 stores SC execution duration, then
// txProcess.go:337 retrieves it for tolerance band validation.
func TestKappContext_ExecutionTimeStorage(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender"),
		ContractID:     0,
		ContractType:   0,
		Block:          &block.Block{},
		TxHash:         []byte("tx-hash"),
	})

	// Simulate smart contract execution storing time (process.go:266)
	executionTime := 250 * time.Millisecond
	ctx.SetExecutionTime(executionTime)

	// Verify it can be retrieved for validation (txProcess.go:337)
	retrieved := ctx.GetExecutionTime()
	assert.Equal(t, executionTime, retrieved)
}

// TestReceiptSlice_GetByType tests filtering receipts by type
func TestReceiptSlice_GetByType(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender"),
		ContractID:     0,
		Block:          &block.Block{},
	})

	// Add receipts of different types
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Transfer, 0, []byte("data1")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Transfer, 0, []byte("data2")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Freeze, 0, []byte("data3")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Debug, 0, []byte("debug1")))

	// Filter by Transfer type
	transfers := ctx.Receipts().GetByType(int8(txProcess.Transfer))
	assert.Equal(t, 2, len(transfers), "should have 2 Transfer receipts")

	// Filter by Freeze type
	freezes := ctx.Receipts().GetByType(int8(txProcess.Freeze))
	assert.Equal(t, 1, len(freezes), "should have 1 Freeze receipt")

	// Filter by Debug type
	debugs := ctx.Receipts().GetByType(int8(txProcess.Debug))
	assert.Equal(t, 1, len(debugs), "should have 1 Debug receipt")

	// Filter by non-existent type
	none := ctx.Receipts().GetByType(int8(txProcess.Claim))
	assert.Equal(t, 0, len(none), "should have 0 Claim receipts")
}

// TestReceiptSlice_GetPreserved tests retrieving system receipts (type >= 120)
func TestReceiptSlice_GetPreserved(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender"),
		ContractID:     0,
		Block:          &block.Block{},
	})

	// Add regular receipts
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Transfer, 0, []byte("data1")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Freeze, 0, []byte("data2")))

	// Add system receipts (Debug=120, Warning=121, Error=122)
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Debug, 0, []byte("debug")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Warning, 0, []byte("warning")))
	ctx.Receipts().Add(txProcess.NewReceipt(txProcess.Error, 0, []byte("error")))

	// GetPreserved should only return system receipts
	preserved := ctx.Receipts().GetPreserved()
	assert.Equal(t, 3, len(preserved), "should have 3 preserved (system) receipts")

	// Verify all preserved receipts have type >= 120
	for _, r := range preserved {
		assert.GreaterOrEqual(t, r.Data[0][0], byte(kapp.SystemReceiptTypeStart),
			"preserved receipt type should be >= SystemReceiptTypeStart")
	}
}

// TestReceiptSlice_AddError tests the AddError convenience method
func TestReceiptSlice_AddError(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender"),
		ContractID:     5,
		Block:          &block.Block{},
	})

	// Add error using convenience method
	ctx.Receipts().AddError(ctx.ContractID(), "InvalidAmount", "amount must be positive")

	receipts := ctx.Receipts().Get()
	assert.Equal(t, 1, len(receipts), "should have 1 receipt")

	receipt := receipts[0]
	// Verify receipt structure: Data[0] = {receiptType, contractID}
	assert.Equal(t, byte(kapp.ReceiptTypeError), receipt.Data[0][0], "receipt type should be Error")
	assert.Equal(t, byte(5), receipt.Data[0][1], "contract ID should be 5")

	// Verify params
	assert.Equal(t, "InvalidAmount", string(receipt.Data[1]), "first param should be field name")
	assert.Equal(t, "amount must be positive", string(receipt.Data[2]), "second param should be reason")

	// Verify it's preserved on GetPreserved
	preserved := ctx.Receipts().GetPreserved()
	assert.Equal(t, 1, len(preserved), "error receipt should be preserved")
}
