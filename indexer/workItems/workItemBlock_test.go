package workItems_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemBlock_SaveNilHeaderShouldRetNil(t *testing.T) {
	itemBlock := workItems.NewItemBlock(
		&mock.ElasticProcessorStub{},
		&mock.MarshalizerMock{},
		&indexer.ArgsSaveBlockData{},
	)
	require.False(t, itemBlock.IsInterfaceNil())

	err := itemBlock.Save()
	assert.Nil(t, err)
}

func TestItemBlock_SaveHeaderShouldErr(t *testing.T) {
	localErr := errors.New("local err")
	itemBlock := workItems.NewItemBlock(
		&mock.ElasticProcessorStub{
			SaveHeaderCalled: func(header data.HeaderHandler, signer []byte, txsSize int, validators []string) error {
				return localErr
			},
		},
		&mock.MarshalizerMock{},
		&indexer.ArgsSaveBlockData{
			HeaderHash:       []byte("header hash"),
			Header:           &block.Block{Header: &block.BlockHeader{}},
			Signer:           []byte("signature"),
			TransactionsPool: &indexer.Pool{},
		},
	)
	require.False(t, itemBlock.IsInterfaceNil())

	err := itemBlock.Save()
	require.True(t, errors.Is(err, localErr))
}

func TestItemBlock_SaveTransactionsShouldErr(t *testing.T) {
	localErr := errors.New("local err")
	itemBlock := workItems.NewItemBlock(
		&mock.ElasticProcessorStub{
			SaveTransactionsCalled: func(header data.HeaderHandler, pool *indexer.Pool) error {
				return localErr
			},
		},
		&mock.MarshalizerMock{},
		&indexer.ArgsSaveBlockData{
			HeaderHash:       []byte("header hash"),
			Header:           &block.Block{Header: &block.BlockHeader{}},
			Signer:           []byte("signature"),
			TransactionsPool: &indexer.Pool{},
		},
	)
	require.False(t, itemBlock.IsInterfaceNil())

	err := itemBlock.Save()
	require.True(t, errors.Is(err, localErr))
}

func TestItemBlock_SaveShouldWork(t *testing.T) {
	countCalled := 0
	itemBlock := workItems.NewItemBlock(
		&mock.ElasticProcessorStub{
			SaveHeaderCalled: func(header data.HeaderHandler, signer []byte, txsSize int, validators []string) error {
				countCalled++
				return nil
			},
			SaveTransactionsCalled: func(header data.HeaderHandler, pool *indexer.Pool) error {
				countCalled++
				return nil
			},
		},
		&mock.MarshalizerMock{},
		&indexer.ArgsSaveBlockData{
			HeaderHash:       []byte("header hash"),
			Header:           &block.Block{Header: &block.BlockHeader{}},
			Signer:           []byte("signature"),
			TransactionsPool: &indexer.Pool{},
		},
	)
	require.False(t, itemBlock.IsInterfaceNil())

	err := itemBlock.Save()
	require.NoError(t, err)
	require.Equal(t, 2, countCalled)
}
