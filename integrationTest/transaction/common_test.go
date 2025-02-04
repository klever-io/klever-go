package transaction

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	tdata "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetAndCheckTransaction(t *testing.T) {
	node, err := processorNode.NewProcessorNode()
	require.Nil(t, err)

	node.InternalMarshalizer = &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) ([]byte, error) {
			return obj.(*tdata.Transaction_Raw).ChainID, nil
		},
	}
	node.Hasher = &mock.HasherStub{
		ComputeCalled: func(data string) []byte {
			return []byte(data)
		},
	}
	node.DataPool = mock.NewPoolsHolderMock()

	tests := []struct {
		name        string
		tx          *tdata.Transaction
		expectedErr error
	}{
		{
			name: "transaction not in block",
			tx: &tdata.Transaction{
				RawData: &tdata.Transaction_Raw{
					Nonce:   1,
					ChainID: []byte("0x1"),
				},
			},
			expectedErr: ErrNotInBlock,
		},
		{
			name: "successful transaction",
			tx: &tdata.Transaction{
				RawData: &tdata.Transaction_Raw{
					Nonce:   1,
					ChainID: []byte("0x2"),
				},
				Block: 1,
			},
			expectedErr: nil,
		},
		{
			name: "failed transaction status",
			tx: &tdata.Transaction{
				RawData: &tdata.Transaction_Raw{
					Nonce:   1,
					ChainID: []byte("0x3"),
				},
				Block:  1,
				Result: tdata.Transaction_FAILED,
			},
			expectedErr: ErrStatusNotSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node.DataPool.Transactions().AddData(
				tt.tx.RawData.ChainID,
				tt.tx,
				10,
				"meta",
			)
			_, err := GetAndCheckTransaction(node, tt.tx.RawData.ChainID)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}
