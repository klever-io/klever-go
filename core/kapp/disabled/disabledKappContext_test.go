package disabled_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/kapp/disabled"
	"github.com/stretchr/testify/assert"
)

// TestDisabledKappContext_ExecutionTime ensures coverage for execution time methods.
// These are no-op implementations - SetExecutionTime does nothing, GetExecutionTime returns 0.
// Used in production at txProcess.go:372 for cleanup after transaction processing.
func TestDisabledKappContext_ExecutionTime(t *testing.T) {
	t.Parallel()

	ctx := disabled.NewDisabledKappContext()

	// SetExecutionTime is a no-op (intentionally empty)
	ctx.SetExecutionTime(100 * time.Millisecond)
	ctx.SetExecutionTime(500 * time.Millisecond)

	// GetExecutionTime always returns 0
	executionTime := ctx.GetExecutionTime()
	assert.Equal(t, time.Duration(0), executionTime)
}

// TestDisabledKappContext_AllMethods ensures coverage for all no-op methods.
// This disabled context is used for cleanup to prevent accessing stale transaction data.
func TestDisabledKappContext_AllMethods(t *testing.T) {
	t.Parallel()

	ctx := disabled.NewDisabledKappContext()

	// All setters are no-ops
	ctx.SetContractID(999)
	ctx.AddReturnData([]byte("ignored"))
	ctx.SetReturnData([][]byte{[]byte("ignored")})

	// All getters return zero/empty values
	assert.Equal(t, 0, ctx.ContractID())
	assert.Equal(t, []byte{}, ctx.OriginalSender())
	assert.Equal(t, []byte{}, ctx.TxHash())
	assert.Equal(t, uint64(0), ctx.TxNonce())
	assert.Equal(t, uint64(0), ctx.GetGasLimit())
	assert.Nil(t, ctx.GetExecData())
	assert.Equal(t, [][]byte{}, ctx.GetAndClearReturnData())
	assert.False(t, ctx.IsScSimulation())
	assert.NotNil(t, ctx.Block())
	assert.NotNil(t, ctx.Receipts())

	// SubGasUsed returns nil error
	err := ctx.SubGasUsed(1000)
	assert.Nil(t, err)
}
