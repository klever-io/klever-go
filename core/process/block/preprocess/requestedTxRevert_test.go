package preprocess_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/interceptors/processor"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestedTxNode wires one pool to the real preprocessor and the real tx interceptor, the way a
// node does, with the tx of a committed block kept in storage until a revert restores it
type requestedTxNode struct {
	pool         retriever.ShardedDataCacherNotifier
	txs          *preprocess.Transactions
	txip         *processor.TxInterceptorProcessor
	tx           *transaction.Transaction
	hash         []byte
	inStorage    atomic.Bool
	numValidated atomic.Int32
}

func newRequestedTxNode(t *testing.T) *requestedTxNode {
	sender := createFakeSenderAddress(1)
	wrapped := createTxWithParams(createFakeTxHash(sender, 1), sender, 1, 0, 1)
	tx, ok := wrapped.Tx.(*transaction.Transaction)
	require.True(t, ok)

	n := &requestedTxNode{
		pool: commonMock.NewPoolsHolderMock().Transactions(),
		tx:   tx,
		hash: wrapped.TxHash,
	}
	n.inStorage.Store(true)

	marshalizer := &commonMock.MarshalizerMock{}
	buff, err := marshalizer.Marshal(tx)
	require.NoError(t, err)

	store := &commonMock.ChainStorerMock{
		GetAllCalled: func(_ retriever.UnitType, _ [][]byte) (map[string][]byte, error) {
			return map[string][]byte{string(n.hash): buff}, nil
		},
		GetStorerCalled: func(_ retriever.UnitType) storage.Storer {
			return &commonMock.StorerStub{
				RemoveCalled: func(_ []byte) error {
					n.inStorage.Store(false)
					return nil
				},
			}
		},
	}

	preprocessor, err := preprocess.NewTransactionPreprocessor(
		n.pool,
		store,
		&commonMock.HasherMock{},
		marshalizer,
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		func(txHashes [][]byte) {},
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)
	require.NoError(t, err)
	n.txs = preprocess.NewTransactionsTest(preprocessor)

	n.txip, err = processor.NewTxInterceptorProcessor(&processor.ArgTxInterceptorProcessor{
		TxDataCache: n.pool,
		TxValidator: &mock.TxValidatorStub{
			CheckTxValidityCalled: func(_ process.TxValidatorHandler) error {
				n.numValidated.Add(1)
				return nil
			},
			CheckDupCalled: func(_ []byte) error {
				return common.ErrDupTransaction
			},
		},
		RequestedItemsHandler: &commonMock.RequestedItemsHandlerStub{
			HasCalled:    func(key string) bool { return key == string(n.hash) },
			RemoveCalled: func(_ string) {},
		},
	})
	require.NoError(t, err)

	return n
}

// answer delivers a peer's answer to the request for the tx through the interceptor
func (n *requestedTxNode) answer() error {
	answer := &struct {
		mock.InterceptedDataStub
		mock.InterceptedTxHandlerStub
	}{
		InterceptedDataStub: mock.InterceptedDataStub{
			HashCalled: func() []byte { return n.hash },
		},
		InterceptedTxHandlerStub: mock.InterceptedTxHandlerStub{
			TransactionCalled: func() data.TransactionHandler {
				return &transaction.Transaction{RawData: n.tx.RawData, Signature: n.tx.Signature}
			},
		},
	}

	err := n.txip.Validate(answer, "")
	if err != nil {
		return err
	}

	return n.txip.Save(answer, "", "")
}

// waitForBlock requests the tx for a block listing it and waits for it while arrive runs
func (n *requestedTxNode) waitForBlock(t *testing.T, arrive func()) error {
	n.txs.CreateBlockStarted()
	requested := n.txs.RequestBlockTransactions(&block.Block{TxHashes: [][]byte{n.hash}})
	require.Equal(t, 1, requested)

	go arrive()

	return n.txs.IsDataPrepared(requested, func() time.Duration { return 500 * time.Millisecond })
}

func (n *requestedTxNode) revert() error {
	_, err := n.txs.RestoreBlockDataIntoPools(&block.Block{TxHashes: [][]byte{n.hash}})
	return err
}

func (n *requestedTxNode) numInPool() int {
	return n.pool.ShardDataStore("0").Len()
}

func TestTransactions_RequestedTxRacingARevert(t *testing.T) {
	t.Parallel()

	t.Run("the revert restores the tx while the block waits, later answers are dups", func(t *testing.T) {
		t.Parallel()

		n := newRequestedTxNode(t)
		errRevert := make(chan error, 1)
		err := n.waitForBlock(t, func() { errRevert <- n.revert() })
		require.NoError(t, err)
		require.NoError(t, <-errRevert)

		assert.Equal(t, common.ErrDupTransaction, n.answer())
		assert.Equal(t, common.ErrDupTransaction, n.answer())
		assert.Zero(t, n.numValidated.Load())
		assert.Equal(t, 1, n.numInPool())
		assert.False(t, n.inStorage.Load())
	})
	t.Run("an answer for a tx only in storage releases the block before the revert", func(t *testing.T) {
		t.Parallel()

		n := newRequestedTxNode(t)
		errAnswer := make(chan error, 1)
		err := n.waitForBlock(t, func() { errAnswer <- n.answer() })
		require.NoError(t, err)
		require.NoError(t, <-errAnswer)
		assert.Equal(t, int32(1), n.numValidated.Load())

		require.NoError(t, n.revert())
		assert.Equal(t, common.ErrDupTransaction, n.answer())
		assert.Equal(t, int32(1), n.numValidated.Load())
		assert.Equal(t, 1, n.numInPool())
	})
	t.Run("a tx that leaves the pool after a dup answer is accepted when requested again", func(t *testing.T) {
		t.Parallel()

		n := newRequestedTxNode(t)
		errRevert := make(chan error, 1)
		require.NoError(t, n.waitForBlock(t, func() { errRevert <- n.revert() }))
		require.NoError(t, <-errRevert)
		require.Equal(t, common.ErrDupTransaction, n.answer())

		n.pool.RemoveData(n.hash, "0")
		require.Zero(t, n.numInPool())

		errAnswer := make(chan error, 1)
		err := n.waitForBlock(t, func() { errAnswer <- n.answer() })
		require.NoError(t, err)
		require.NoError(t, <-errAnswer)
		assert.Equal(t, int32(1), n.numValidated.Load())
		assert.Equal(t, 1, n.numInPool())
	})
}
