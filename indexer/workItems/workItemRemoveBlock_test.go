package workItems_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	imock "github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewItemRemoveBlock(t *testing.T) {
	t.Parallel()

	indexer := &imock.ElasticProcessorStub{}
	header := &block.Block{
		Header: &block.BlockHeader{},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)

	require.NotNil(t, itemRemoveBlock)
	require.False(t, itemRemoveBlock.IsInterfaceNil())
}

func TestItemRemoveBlock_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var itemRemoveBlock *workItems.WorkItemHandler
	require.True(t, itemRemoveBlock == nil)

	indexer := &imock.ElasticProcessorStub{}
	header := &block.Block{
		Header: &block.BlockHeader{},
	}
	itemRemoveBlockImpl := workItems.NewItemRemoveBlock(indexer, header)
	require.False(t, itemRemoveBlockImpl.IsInterfaceNil())
}

func TestItemRemoveBlock_SaveWithInvalidHeaderType(t *testing.T) {
	t.Parallel()

	indexer := &imock.ElasticProcessorStub{}
	// Use a mock header that is not a *block.Block
	header := &mock.HeaderHandlerStub{}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.Equal(t, workItems.ErrBodyTypeAssertion, err)
}

func TestItemRemoveBlock_SaveRevertAccountBalancesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("revert account balances error")
	revertCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			return expectedErr
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.True(t, revertCalled)
	require.Equal(t, expectedErr, err)
}

func TestItemRemoveBlock_SaveRemoveAccountsHistoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("remove accounts history error")
	revertCalled := false
	removeHistoryCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			assert.Equal(t, int64(123456), blockTimestamp)
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			assert.Equal(t, int64(123456), blockTimestamp)
			return expectedErr
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.Equal(t, expectedErr, err)
}

func TestItemRemoveBlock_SaveRemoveTransactionsError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("remove transactions error")
	revertCalled := false
	removeHistoryCalled := false
	removeTransactionsCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			removeTransactionsCalled = true
			return expectedErr
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
		TxHashes: [][]byte{
			[]byte("txHash1"),
			[]byte("txHash2"),
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.True(t, removeTransactionsCalled)
	require.Equal(t, expectedErr, err)
}

func TestItemRemoveBlock_SaveSkipRemoveTransactionsWhenNoTxHashes(t *testing.T) {
	t.Parallel()

	revertCalled := false
	removeHistoryCalled := false
	removeTransactionsCalled := false
	removeHeaderCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			removeTransactionsCalled = true
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			removeHeaderCalled = true
			return nil
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
		TxHashes: [][]byte{}, // Empty tx hashes
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.NoError(t, err)
	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.False(t, removeTransactionsCalled)
	require.True(t, removeHeaderCalled)
}

func TestItemRemoveBlock_SaveRemoveHeaderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("remove header error")
	revertCalled := false
	removeHistoryCalled := false
	removeTransactionsCalled := false
	removeHeaderCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			removeTransactionsCalled = true
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			removeHeaderCalled = true
			return expectedErr
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
		TxHashes: [][]byte{
			[]byte("txHash1"),
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.True(t, removeTransactionsCalled)
	require.True(t, removeHeaderCalled)
	require.Equal(t, expectedErr, err)
}

func TestItemRemoveBlock_SaveSuccessWithTransactions(t *testing.T) {
	t.Parallel()

	revertCalled := false
	removeHistoryCalled := false
	removeTransactionsCalled := false
	removeHeaderCalled := false
	var capturedTimestamp int64

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			capturedTimestamp = blockTimestamp
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			assert.Equal(t, capturedTimestamp, blockTimestamp)
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			removeTransactionsCalled = true
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			removeHeaderCalled = true
			return nil
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
		TxHashes: [][]byte{
			[]byte("txHash1"),
			[]byte("txHash2"),
			[]byte("txHash3"),
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.NoError(t, err)
	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.True(t, removeTransactionsCalled)
	require.True(t, removeHeaderCalled)
	require.Equal(t, int64(123456), capturedTimestamp)
}

func TestItemRemoveBlock_SaveSuccessWithoutTransactions(t *testing.T) {
	t.Parallel()

	revertCalled := false
	removeHistoryCalled := false
	removeTransactionsCalled := false
	removeHeaderCalled := false

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			revertCalled = true
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			removeHistoryCalled = true
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			removeTransactionsCalled = true
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			removeHeaderCalled = true
			return nil
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     200,
			Timestamp: 789012,
		},
		TxHashes: nil, // No transactions
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.NoError(t, err)
	require.True(t, revertCalled)
	require.True(t, removeHistoryCalled)
	require.False(t, removeTransactionsCalled)
	require.True(t, removeHeaderCalled)
}

func TestItemRemoveBlock_SaveExecutionOrder(t *testing.T) {
	t.Parallel()

	executionOrder := make([]string, 0)

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			executionOrder = append(executionOrder, "RevertAccountBalances")
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			executionOrder = append(executionOrder, "RemoveAccountsHistory")
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			executionOrder = append(executionOrder, "RemoveTransactions")
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			executionOrder = append(executionOrder, "RemoveHeader")
			return nil
		},
	}

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:     100,
			Timestamp: 123456,
		},
		TxHashes: [][]byte{
			[]byte("txHash1"),
		},
	}

	itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
	err := itemRemoveBlock.Save()

	require.NoError(t, err)
	require.Equal(t, []string{
		"RevertAccountBalances",
		"RemoveAccountsHistory",
		"RemoveTransactions",
		"RemoveHeader",
	}, executionOrder)
}

func TestItemRemoveBlock_SaveWithDifferentBlockData(t *testing.T) {
	t.Parallel()

	var capturedNonce uint64
	var capturedTimestamp int64

	indexer := &imock.ElasticProcessorStub{
		RevertAccountBalancesCalled: func(blockTimestamp int64) error {
			capturedTimestamp = blockTimestamp
			return nil
		},
		RemoveAccountsHistoryCalled: func(blockTimestamp int64) error {
			return nil
		},
		RemoveTransactionsCalled: func(blk data.HeaderHandler) error {
			capturedNonce = blk.GetNonce()
			return nil
		},
		RemoveHeaderCalled: func(header data.HeaderHandler) error {
			return nil
		},
	}

	testCases := []struct {
		name      string
		nonce     uint64
		timestamp int64
		txHashes  [][]byte
	}{
		{
			name:      "Block with nonce 0",
			nonce:     0,
			timestamp: 100000,
			txHashes:  [][]byte{[]byte("tx1")},
		},
		{
			name:      "Block with high nonce",
			nonce:     999999,
			timestamp: 999999999,
			txHashes:  [][]byte{[]byte("tx1"), []byte("tx2")},
		},
		{
			name:      "Block with multiple transactions",
			nonce:     500,
			timestamp: 500000,
			txHashes:  [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			header := &block.Block{
				Header: &block.BlockHeader{
					Nonce:     tc.nonce,
					Timestamp: tc.timestamp,
				},
				TxHashes: tc.txHashes,
			}

			itemRemoveBlock := workItems.NewItemRemoveBlock(indexer, header)
			err := itemRemoveBlock.Save()

			require.NoError(t, err)
			require.Equal(t, tc.timestamp, capturedTimestamp)
			require.Equal(t, tc.nonce, capturedNonce)
		})
	}
}
