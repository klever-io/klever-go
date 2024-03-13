package blockAPI

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
)

func createMockArgumentsWithTx(
	blockBytes []byte,
	txHash string,
	txBytes []byte,
	marshalizer *mock.MarshalizerFake,
) apiBlockProcessor {
	return apiBlockProcessor{
		marshalizer: marshalizer,
		store: &mock.ChainStorerMock{
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return &mock.StorerStub{
					GetBulkFromEpochCalled: func(keys [][]byte, epoch uint32) (map[string][]byte, error) {
						return map[string][]byte{txHash: txBytes}, nil
					},
					GetFromEpochCalled: func(key []byte, epoch uint32) ([]byte, error) {
						return blockBytes, nil
					},
				}
			},
		},
		unmarshalTx: func(txBytes []byte) (*api.Transaction, error) {
			var unmarshalledTx transaction.Transaction
			_ = marshalizer.Unmarshal(&unmarshalledTx, txBytes)
			return &api.Transaction{
				Transaction: &unmarshalledTx,
				Hash:        txHash,
			}, nil
		},
	}
}

func TestBaseApiBlockProcessor_GetNormalTxFromMiniBlock(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerFake{}

	txHash := "d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00"

	tx := transaction.Transaction{}

	mb := block.Block{
		Header: &block.BlockHeader{
			Epoch: 0,
		},
		TxHashes: [][]byte{[]byte(txHash)},
	}

	txBytes, _ := marshalizer.Marshal(&tx)
	mbBytes, _ := marshalizer.Marshal(&mb)

	baseAPIBlock := createMockArgumentsWithTx(
		mbBytes,
		txHash,
		txBytes,
		marshalizer,
	)

	mbTxs := baseAPIBlock.getTxsByMb(&mb)

	assert.Equal(t, mbTxs[0].Hash, txHash)
}
