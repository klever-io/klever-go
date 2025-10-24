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
