package mock

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"golang.org/x/net/context"
)

// PendingTransactionsSyncHandlerMock -
type PendingTransactionsSyncHandlerMock struct {
	SyncPendingTransactionsCalled func(blk *block.Block, epoch uint32, ctx context.Context) error
	GetTransactionsCalled         func() (map[string]data.TransactionHandler, error)
}

// SyncPendingTransactionsFor -
func (et *PendingTransactionsSyncHandlerMock) SyncPendingTransactions(blk *block.Block, epoch uint32, ctx context.Context) error {
	if et.SyncPendingTransactionsCalled != nil {
		return et.SyncPendingTransactionsCalled(blk, epoch, ctx)
	}
	return nil
}

// GetTransactions -
func (et *PendingTransactionsSyncHandlerMock) GetTransactions() (map[string]data.TransactionHandler, error) {
	if et.GetTransactionsCalled != nil {
		return et.GetTransactionsCalled()
	}
	return nil, nil
}

// IsInterfaceNil -
func (et *PendingTransactionsSyncHandlerMock) IsInterfaceNil() bool {
	return et == nil
}
