package kapp_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/kapp"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

func TestSubContext(t *testing.T) {
	t.Parallel()

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender1"),
		ContractID:     -1,
		ContractType:   -1,
		Block:          &block.Block{},
	})

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		[]byte("sender1"),
	))

	assert.Equal(t, len(ctx.Receipts().Get()), 1)
	assert.Equal(t, ctx.ContractID(), -1)

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		[]byte("kapps.ITOKAppAddress"),
	))

	assert.Equal(t, len(ctx.Receipts().Get()), 2)
	assert.Equal(t, len(ctx.Receipts().Get()), 2)

	assert.Equal(t, ctx.ContractID(), -1)
	assert.Equal(t, ctx.ContractID(), -1)
}
