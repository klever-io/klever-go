package node_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/node"
	"github.com/stretchr/testify/require"
)

func TestNode_TXPool_SkipsEntriesThatAreNotTransactions(t *testing.T) {
	t.Parallel()

	tx1 := transaction.NewBaseTransaction([]byte("alice"), 42, [][]byte{[]byte("data")}, 0, 0)
	tx2 := transaction.NewBaseTransaction([]byte("alice"), 43, [][]byte{[]byte("data")}, 0, 0)
	var nilTx *transaction.Transaction

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{
				GetPaginatedCalled: func(cacheID string, page int, pageSize int) ([]interface{}, int) {
					return []interface{}{tx1, nil, "not a transaction", nilTx, tx2}, 5
				},
			}
		},
	}

	n, err := node.NewNode(
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(getMarshalizer()),
		node.WithHasher(getHasher()),
	)
	require.Nil(t, err)

	txs, total, err := n.TXPool("", 0, 10)
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Len(t, txs, 2)
	for _, tx := range txs {
		require.NotNil(t, tx)
	}
	require.Equal(t, uint64(42), txs[0].GetNonce())
	require.Equal(t, uint64(43), txs[1].GetNonce())
}

func TestNode_TXPool_SkipsTransactionsThatCannotBePrepared(t *testing.T) {
	t.Parallel()

	tx1 := transaction.NewBaseTransaction([]byte("alice"), 42, [][]byte{[]byte("data")}, 0, 0)
	tx2 := transaction.NewBaseTransaction([]byte("alice"), 43, [][]byte{[]byte("data")}, 0, 0)
	tx3 := transaction.NewBaseTransaction([]byte("alice"), 44, [][]byte{[]byte("data")}, 0, 0)

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{
				GetPaginatedCalled: func(cacheID string, page int, pageSize int) ([]interface{}, int) {
					return []interface{}{tx1, tx2, tx3}, 3
				},
			}
		},
	}

	// hashing the middle transaction fails, so it cannot be turned into an API transaction
	realMarshalizer := getMarshalizer()
	marshalizer := &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) ([]byte, error) {
			if obj == tx2.RawData {
				return nil, errors.New("marshal failed")
			}
			return realMarshalizer.Marshal(obj)
		},
	}

	n, err := node.NewNode(
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(marshalizer),
		node.WithHasher(getHasher()),
	)
	require.Nil(t, err)

	txs, total, err := n.TXPool("", 0, 10)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Len(t, txs, 2)
	require.Equal(t, uint64(42), txs[0].GetNonce())
	require.Equal(t, uint64(44), txs[1].GetNonce())
}
