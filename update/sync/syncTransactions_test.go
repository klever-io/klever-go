package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/require"
)

func createMockArgs() ArgsNewPendingTransactionsSyncer {
	return ArgsNewPendingTransactionsSyncer{
		DataPools: cMock.NewPoolsHolderMock(),
		Storages: &cMock.ChainStorerMock{
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return &mock.StorerStub{}
			},
		},
		Marshalizer:    &cMock.MarshalizerFake{},
		RequestHandler: &cMock.RequestHandlerStub{},
	}
}

func TestNewPendingTransactionsSyncer(t *testing.T) {
	t.Parallel()

	args := createMockArgs()

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, err)
	require.NotNil(t, pendingTxsSyncer)
	require.False(t, pendingTxsSyncer.IsInterfaceNil())
}

func TestNewPendingTransactionsSyncer_NilStorages(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.Storages = nil

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, pendingTxsSyncer)
	require.NotNil(t, common.ErrNilHeadersStorage, err)
}

func TestNewPendingTransactionsSyncer_NilDataPools(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.DataPools = nil

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, pendingTxsSyncer)
	require.NotNil(t, common.ErrNilDataPoolHolder, err)
}

func TestNewPendingTransactionsSyncer_NilMarshalizer(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.Marshalizer = nil

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, pendingTxsSyncer)
	require.NotNil(t, common.ErrNilMarshalizer, err)
}

func TestNewPendingTransactionsSyncer_NilRequestHandler(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.RequestHandler = nil

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, pendingTxsSyncer)
	require.NotNil(t, common.ErrNilRequestHandler, err)
}

func TestSyncPendingTransactionsFor(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.Storages = &cMock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) (bytes []byte, err error) {
					tx := &dataTransaction.Transaction{
						RawData: &dataTransaction.Transaction_Raw{
							Sender: []byte("snd"),
						},
					}
					return json.Marshal(tx)
				},
			}
		},
	}

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, err)

	b := &block.Block{TxHashes: [][]byte{[]byte("txHash")}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = pendingTxsSyncer.SyncPendingTransactions(b, 1, ctx)
	cancel()
	require.Nil(t, err)
}

func TestSyncPendingTransactionsFor_MissingTxFromPool(t *testing.T) {
	t.Parallel()

	args := createMockArgs()
	args.Storages = &cMock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) (bytes []byte, err error) {
					dummy := 10
					return json.Marshal(dummy)
				},
			}
		},
	}

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, err)

	b := &block.Block{TxHashes: [][]byte{[]byte("txHash")}}

	// we need a value larger than the request interval as to also test what happens after the normal request interval has expired
	timeout := time.Second + time.Millisecond*500
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err = pendingTxsSyncer.SyncPendingTransactions(b, 1, ctx)
	cancel()
	require.Equal(t, process.ErrTimeIsOut, err)
}

func TestSyncPendingTransactionsFor_ReceiveMissingTx(t *testing.T) {
	t.Parallel()

	txHash := []byte("txHash")
	args := createMockArgs()
	args.Storages = &cMock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) (bytes []byte, err error) {
					dummy := 10
					return json.Marshal(dummy)
				},
			}
		},
	}

	pendingTxsSyncer, err := NewPendingTransactionsSyncer(args)
	require.Nil(t, err)

	b := &block.Block{TxHashes: [][]byte{txHash}}

	go func() {
		time.Sleep(500 * time.Millisecond)

		tx := &dataTransaction.Transaction{
			RawData: &dataTransaction.Transaction_Raw{
				Sender: []byte("snd"),
			},
		}

		pendingTxsSyncer.receivedTransaction(txHash, tx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = pendingTxsSyncer.SyncPendingTransactions(b, 1, ctx)
	cancel()
	require.Nil(t, err)
}
