package dataPool_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/retriever/dataPool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//------- NewDataPool

func TestNewDataPool_NilTransactionsShouldErr(t *testing.T) {
	t.Parallel()

	tdp, err := dataPool.NewDataPool(
		nil,
		&mock.HeadersCacherStub{},
		mock.NewCacherStub(),
		mock.NewCacherStub(),
		&mock.TxForCurrentBlockStub{},
	)

	assert.Equal(t, common.ErrNilTxDataPool, err)
	assert.Nil(t, tdp)
}

func TestNewDataPool_NilHeadersShouldErr(t *testing.T) {
	t.Parallel()

	tdp, err := dataPool.NewDataPool(
		mock.NewShardedDataStub(),
		nil,
		mock.NewCacherStub(),
		mock.NewCacherStub(),
		&mock.TxForCurrentBlockStub{},
	)

	assert.Equal(t, common.ErrNilHeadersDataPool, err)
	assert.Nil(t, tdp)
}

func TestNewDataPool_NilTrieNodesShouldErr(t *testing.T) {
	t.Parallel()

	tdp, err := dataPool.NewDataPool(
		mock.NewShardedDataStub(),
		&mock.HeadersCacherStub{},
		nil,
		mock.NewCacherStub(),
		&mock.TxForCurrentBlockStub{},
	)

	assert.Equal(t, common.ErrNilTrieNodesPool, err)
	assert.Nil(t, tdp)
}

func TestNewDataPool_NilSmartContractsShouldErr(t *testing.T) {
	t.Parallel()

	tdp, err := dataPool.NewDataPool(
		mock.NewShardedDataStub(),
		&mock.HeadersCacherStub{},
		mock.NewCacherStub(),
		nil,
		&mock.TxForCurrentBlockStub{},
	)

	assert.Equal(t, common.ErrNilSmartContractsPool, err)
	assert.Nil(t, tdp)
}

func TestNewDataPool_NilCurrBlockShouldErr(t *testing.T) {
	transactions := mock.NewShardedDataStub()
	headers := &mock.HeadersCacherStub{}
	trieNodes := mock.NewCacherStub()

	tdp, err := dataPool.NewDataPool(
		transactions,
		headers,
		trieNodes,
		mock.NewCacherStub(),
		nil,
	)

	require.Nil(t, tdp)
	require.Equal(t, common.ErrNilCurrBlockTxs, err)
}

func TestNewDataPool_OkValsShouldWork(t *testing.T) {
	transactions := mock.NewShardedDataStub()
	headers := &mock.HeadersCacherStub{}
	trieNodes := mock.NewCacherStub()
	currBlock := &mock.TxForCurrentBlockStub{}

	tdp, err := dataPool.NewDataPool(
		transactions,
		headers,
		trieNodes,
		mock.NewCacherStub(),
		currBlock,
	)

	assert.Nil(t, err)
	require.False(t, tdp.IsInterfaceNil())
	//pointer checking
	assert.True(t, transactions == tdp.Transactions())
	assert.True(t, headers == tdp.Headers())
	assert.True(t, currBlock == tdp.CurrentBlockTxs())
	assert.True(t, trieNodes == tdp.TrieNodes())
}
