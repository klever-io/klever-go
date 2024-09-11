package preprocess_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
)

const MaxGasLimitPerBlock = uint64(100000)

func shardedDataCacherNotifier() retriever.ShardedDataCacherNotifier {
	return &commonMock.ShardedDataStub{
		ShardDataStoreCalled: func(id string) (c storage.Cacher) {
			return &commonMock.CacherStub{
				PeekCalled: func(key []byte) (value interface{}, ok bool) {
					if reflect.DeepEqual(key, []byte("tx1_hash")) {
						return &transaction.Transaction{}, true
					}
					if reflect.DeepEqual(key, []byte("tx2_hash")) {
						return &transaction.Transaction{}, true
					}
					return nil, false
				},
				KeysCalled: func() [][]byte {
					return [][]byte{[]byte("key1"), []byte("key2")}
				},
				LenCalled: func() int {
					return 0
				},
			}
		},
		AddDataCalled:                 func(key []byte, data interface{}, sizeInBytes int, cacheId string) {},
		RemoveSetOfDataFromPoolCalled: func(keys [][]byte, id string) {},
		SearchFirstDataCalled: func(key []byte) (value interface{}, ok bool) {
			if reflect.DeepEqual(key, []byte("tx1_hash")) {
				return &transaction.Transaction{}, true
			}
			if reflect.DeepEqual(key, []byte("tx2_hash")) {
				return &transaction.Transaction{}, true
			}
			return nil, false
		},
	}
}

func initDataPool() *commonMock.PoolsHolderStub {
	sdp := &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return shardedDataCacherNotifier()
		},
		UnsignedTransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return shardedDataCacherNotifier()
		},
		MetaBlocksCalled: func() storage.Cacher {
			return &commonMock.CacherStub{
				GetCalled: func(key []byte) (value interface{}, ok bool) {
					if reflect.DeepEqual(key, []byte("tx1_hash")) {
						return &transaction.Transaction{}, true
					}
					return nil, false
				},
				KeysCalled: func() [][]byte {
					return nil
				},
				LenCalled: func() int {
					return 0
				},
				PeekCalled: func(key []byte) (value interface{}, ok bool) {
					if reflect.DeepEqual(key, []byte("tx1_hash")) {
						return &transaction.Transaction{}, true
					}
					return nil, false
				},
				RegisterHandlerCalled: func(i func(key []byte, value interface{})) {},
			}
		},
	}
	return sdp
}

func createMockPubkeyConverter() *mock.PubkeyConverterMock {
	return mock.NewPubkeyConverterMock(32)
}

func newForkController() core.ForkController {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	return forkController
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilPool(t *testing.T) {
	t.Parallel()

	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		nil,
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, process.ErrNilTransactionPool, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilStore(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		nil,
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilTxStorage, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilHasher(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		nil,
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilMarsalizer(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		nil,
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilTxProce(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		nil,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, process.ErrNilTxProcessor, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilAccounts(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		nil,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilKApps(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		nil,
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilKAppAccountsAdapter, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilRequestFunc(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		nil,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorNilPubkeyConverter(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		nil,
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestTxsPreprocessor_NewTransactionPreprocessorOkValsShouldWork(t *testing.T) {
	t.Parallel()

	tdp := initDataPool()
	requestTransaction := func(txHashes [][]byte) {}
	txs, err := preprocess.NewTransactionPreprocessor(
		tdp.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, err)
	assert.NotNil(t, txs)
	assert.False(t, txs.IsInterfaceNil())
}

func createGoodPreprocessor(dataPool retriever.PoolsHolder) *preprocess.Transactions {
	requestTransaction := func(txHashes [][]byte) {}

	preprocessor, _ := preprocess.NewTransactionPreprocessor(
		dataPool.Transactions(),
		&commonMock.ChainStorerMock{},
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	return preprocess.NewTransactionsTest(preprocessor)
}

func TestTxsPreProcessor_GetTransactionFromPool(t *testing.T) {
	t.Parallel()
	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)
	txHash := []byte("tx2_hash")
	tx, _ := process.GetTransactionHandlerFromPool(txHash, dataPool.Transactions())
	assert.NotNil(t, txs)
	assert.NotNil(t, tx)
}

func TestTransactionPreprocessor_ReceivedTransactionShouldEraseRequested(t *testing.T) {
	t.Parallel()

	dataPool := commonMock.NewPoolsHolderMock()

	shardedDataStub := &commonMock.ShardedDataStub{
		ShardDataStoreCalled: func(cacheId string) (c storage.Cacher) {
			return &commonMock.CacherStub{
				PeekCalled: func(key []byte) (value interface{}, ok bool) {
					return &transaction.Transaction{}, true
				},
			}
		},
	}

	dataPool.SetTransactions(shardedDataStub)

	txs := createGoodPreprocessor(dataPool)

	//add 3 tx hashes on requested list
	txHash1 := []byte("tx hash 1")
	txHash2 := []byte("tx hash 2")
	txHash3 := []byte("tx hash 3")

	txs.AddTxHashToRequestedList(txHash1)
	txs.AddTxHashToRequestedList(txHash2)
	txs.AddTxHashToRequestedList(txHash3)

	txs.SetMissingTxs(3)

	//received txHash2
	txs.ReceivedTransaction(txHash2, &txcache.WrappedTransaction{Tx: &transaction.Transaction{}})

	assert.True(t, txs.IsTxHashRequested(txHash1))
	assert.False(t, txs.IsTxHashRequested(txHash2))
	assert.True(t, txs.IsTxHashRequested(txHash3))
}

func computeHash(data interface{}, marshalizer marshal.Marshalizer, hasher hashing.Hasher) []byte {
	buff, _ := marshalizer.Marshal(data)
	return hasher.Compute(string(buff))
}

func TestTransactionPreprocessor_GetAllTxsFromBlockShouldWork(t *testing.T) {
	t.Parallel()

	hasher := commonMock.HasherMock{}
	marshalizer := &commonMock.MarshalizerMock{}
	dataPool := commonMock.NewPoolsHolderMock()

	txsSlice := []*transaction.Transaction{
		{},
		{},
		{},
	}
	transactionsHashes := make([][]byte, len(txsSlice))

	//add defined transactions to sender-destination cacher
	for idx, tx := range txsSlice {
		transactionsHashes[idx] = computeHash(tx, marshalizer, hasher)

		dataPool.Transactions().AddData(
			transactionsHashes[idx],
			tx,
			tx.GetSize(),
			"0",
		)
	}

	//add some random data
	txRandom := &transaction.Transaction{}
	dataPool.Transactions().AddData(
		computeHash(txRandom, marshalizer, hasher),
		txRandom,
		txRandom.GetSize(),
		"0",
	)

	txs := createGoodPreprocessor(dataPool)

	bl := &block.Block{
		TxHashes: transactionsHashes,
	}

	txsRetrieved, txHashesRetrieved, err := txs.GetAllTxsFromBlock(bl)

	assert.Nil(t, err)
	assert.Equal(t, len(txsSlice), len(txsRetrieved))
	assert.Equal(t, len(txsSlice), len(txHashesRetrieved))
	for idx, tx := range txsSlice {
		//txReceived should be all txs in the same order
		assert.Equal(t, txsRetrieved[idx], tx)
		//verify corresponding transaction hashes
		assert.Equal(t, txHashesRetrieved[idx], computeHash(tx, marshalizer, hasher))
	}
}

func haveTime() time.Duration {
	return 2000 * time.Millisecond
}

func TestTransactionPreprocessor_RemoveTxsDataFromPoolsNilBlockShouldErr(t *testing.T) {
	t.Parallel()
	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)
	err := txs.RemoveTxsFromPools(nil)
	assert.NotNil(t, err)
	assert.Equal(t, err, process.ErrNilTxBlockBody)
}

func TestTransactionPreprocessor_RemoveTxsDataFromPoolsOK(t *testing.T) {
	t.Parallel()
	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)
	txHash := []byte("txHash")
	txHashes := make([][]byte, 0)
	txHashes = append(txHashes, txHash)
	block := &block.Block{
		TxHashes: txHashes,
	}
	err := txs.RemoveTxsFromPools(block)
	assert.Nil(t, err)
}

func TestTransactions_IsDataPrepared_NumMissingTxsZeroShouldWork(t *testing.T) {
	t.Parallel()

	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)

	err := txs.IsDataPrepared(0, haveTime)
	assert.Nil(t, err)
}

func TestTransactions_IsDataPrepared_NumMissingTxsGreaterThanZeroTxNotReceivedShouldTimeout(t *testing.T) {
	t.Parallel()

	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)

	haveTimeShorter := func() time.Duration {
		return time.Millisecond
	}
	err := txs.IsDataPrepared(2, haveTimeShorter)
	assert.Equal(t, process.ErrTimeIsOut, err)
}

func TestTransactions_IsDataPrepared_NumMissingTxsGreaterThanZeroShouldWork(t *testing.T) {
	t.Parallel()

	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)

	go func() {
		txs.SetRcvdTxChan()
	}()

	err := txs.IsDataPrepared(2, haveTime)
	assert.Nil(t, err)
}
