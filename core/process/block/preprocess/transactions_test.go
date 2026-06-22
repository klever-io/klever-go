package preprocess_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestTxsPreprocessor_NewTransactionPreprocessorNilPeers(t *testing.T) {
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
		nil,
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilPeerAccountsAdapter, err)
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

func TestTxsPreprocessor_NewTransactionPreprocessorNilFeeHandler(t *testing.T) {
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
		nil,
		createMockPubkeyConverter(),
		newForkController(),
	)

	assert.Nil(t, txs)
	assert.Equal(t, process.ErrNilEconomicsFeeHandler, err)
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

func TestTxsPreprocessor_NewTransactionPreprocessorNilForkController(t *testing.T) {
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
		nil,
	)

	assert.Nil(t, txs)
	assert.Equal(t, common.ErrNilForkController, err)
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
	return createGoodPreprocessorWithFork(dataPool, newForkController())
}

func createGoodPreprocessorWithFork(dataPool retriever.PoolsHolder, forkController core.ForkController) *preprocess.Transactions {
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
		forkController,
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
	assert.Equal(t, err, process.ErrNilTxBlockHeader)
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

func createTxWithParams(hash []byte, sender []byte, nonce uint64, dataLen int, fees int64) *txcache.WrappedTransaction {
	tx := transaction.NewBaseTransaction(sender, nonce, [][]byte{make([]byte, dataLen)}, fees, 1_000_000)
	if dataLen > 0 {
		tx.GasLimit = uint64(dataLen) * 1000
	}

	return &txcache.WrappedTransaction{
		Tx:     tx,
		TxHash: hash,
	}
}

func createFakeSenderAddress(senderTag int) []byte {
	bytes := make([]byte, 32)
	binary.LittleEndian.PutUint64(bytes, uint64(senderTag))
	binary.LittleEndian.PutUint64(bytes[24:], uint64(senderTag))
	return bytes
}

func createFakeTxHash(fakeSenderAddress []byte, nonce int) []byte {
	bytes := make([]byte, 32)
	copy(bytes, fakeSenderAddress)
	binary.LittleEndian.PutUint64(bytes[8:], uint64(nonce))
	binary.LittleEndian.PutUint64(bytes[16:], uint64(nonce))
	return bytes
}

func TestTransactions_ComputeSortedTxs_TXPoolError(t *testing.T) {
	t.Parallel()

	gasBandwidth := uint64(1000000)
	randomness := make([]byte, 32) // 0x000000..00

	poolHolders := &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &commonMock.ShardedDataStub{
				ShardDataStoreCalled: func(id string) (c storage.Cacher) {
					return nil
				},
			}
		},
	}
	txs := createGoodPreprocessor(poolHolders)
	// nil pool should return an error
	_, err := txs.ComputeSortedTxs(gasBandwidth, randomness)
	assert.Equal(t, common.ErrNilTxDataPool, err)

	poolHolders = &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return shardedDataCacherNotifier()
		},
	}
	txs = createGoodPreprocessor(poolHolders)
	// cast txcache.TxCache error
	_, err = txs.ComputeSortedTxs(gasBandwidth, randomness)
	assert.Equal(t, common.ErrWrongTypeAssertion, err)

}

func TestTransactions_ComputeSortedTxs_NoDataTransactions(t *testing.T) {
	t.Parallel()

	config := txcache.Config{
		Name:                          "untitled",
		NumChunks:                     16,
		CountThreshold:                100_000,
		CountPerSenderThreshold:       math.MaxUint32,
		NumSendersToPreemptivelyEvict: 200,
		NumBytesThreshold:             1_073_741_824,
		NumBytesPerSenderThreshold:    33_554_432,
	}

	cache, err := txcache.NewTxCache(config)
	require.NoError(t, err)

	// add 100 senders with 1000 txs each
	for i := 0; i < 10; i++ {
		sender := createFakeSenderAddress(i)
		for j := 1; j < 10001; j++ {
			hash := createFakeTxHash(sender, j)
			tx := createTxWithParams(hash, sender, uint64(j), 0, 1000)
			cache.AddTx(tx)
		}
	}

	poolHolders := &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &commonMock.ShardedDataStub{
				ShardDataStoreCalled: func(id string) (c storage.Cacher) {
					return cache
				},
			}
		},
	}
	txs := createGoodPreprocessor(poolHolders)

	gasBandwidth := uint64(1000000)
	randomness := make([]byte, 32) // 0x000000..00

	sortedTxs, err := txs.ComputeSortedTxs(gasBandwidth, randomness)
	assert.Nil(t, err)

	// should select 12000 transactions (max block size)
	assert.Len(t, sortedTxs, 12000)

	// check sender order and nonce with randomness 0x000000..00
	// Check if transactions are sorted by sender and nonce with randomness
	validateTxOrder(t, sortedTxs)

	// for 10 sender, it should change every 1200 txs
	sendersOrder := []int{6, 0, 9, 3, 8, 1, 7, 5, 4, 2}
	for i, tx := range sortedTxs {
		sender := tx.Tx.GetRaw().Sender
		senderTag := binary.LittleEndian.Uint64(sender)
		idx := int(i / 1200)
		assert.Equal(t, sendersOrder[idx], int(senderTag), fmt.Sprintf("Senders should be in order: %d(%d/%d)", idx, sendersOrder[idx], senderTag))
	}
}

func TestTransactions_ComputeSortedTxs_GasTransactions(t *testing.T) {
	t.Parallel()

	config := txcache.Config{
		Name:                          "untitled",
		NumChunks:                     16,
		CountThreshold:                100_000,
		CountPerSenderThreshold:       math.MaxUint32,
		NumSendersToPreemptivelyEvict: 200,
		NumBytesThreshold:             1_073_741_824,
		NumBytesPerSenderThreshold:    33_554_432,
	}

	cache, err := txcache.NewTxCache(config)
	require.NoError(t, err)

	// add 100 senders with 1000 txs each
	for i := 0; i < 100; i++ {
		sender := createFakeSenderAddress(i)
		for j := 1; j < 1001; j++ {
			hash := createFakeTxHash(sender, j)
			tx := createTxWithParams(hash, sender, uint64(j), 100, 1000)
			cache.AddTx(tx)
		}
	}

	poolHolders := &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &commonMock.ShardedDataStub{
				ShardDataStoreCalled: func(id string) (c storage.Cacher) {
					return cache
				},
			}
		},
	}
	txs := createGoodPreprocessor(poolHolders)

	gasBandwidth := uint64(100_000_000)
	randomness := make([]byte, 32) // 0x000000..00

	sortedTxs, err := txs.ComputeSortedTxs(gasBandwidth, randomness)
	assert.Nil(t, err)

	// as transaction has data, the gas limit should applied
	// each TX has 100 gas per byte, so 100 * 1000 = 100_000 gas per TX
	// 100_000_000 limit should allow 1000 TXs
	fmt.Println(len(sortedTxs))
	assert.Len(t, sortedTxs, 1000)

	// check sender order and nonce with randomness 0x000000..00
	// Check if transactions are sorted by sender and nonce with randomness
	validateTxOrder(t, sortedTxs)
}

func validateTxOrder(t *testing.T, sortedTxs []*txcache.WrappedTransaction) {
	lastSender := sortedTxs[0].Tx.GetRaw().Sender
	lastNonce := sortedTxs[0].Tx.GetRaw().Nonce
	for i := 1; i < len(sortedTxs); i++ {
		currentSender := sortedTxs[i].Tx.GetRaw().Sender
		currentNonce := sortedTxs[i].Tx.GetRaw().Nonce

		if bytes.Equal(currentSender, lastSender) {
			assert.Equal(t, currentNonce, lastNonce+1, "Nonce should have no gap for the same sender")
		}

		lastSender = currentSender
		lastNonce = currentNonce
	}
}

func TestTransactions_PreFilterTransactionsWithPriority(t *testing.T) {
	t.Parallel()

	dataPool := initDataPool()
	txs := createGoodPreprocessor(dataPool)

	// Create a set of test transactions
	transactions := []*txcache.WrappedTransaction{
		{Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000}},
		{Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}}, GasLimit: 50000}},
		{Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 4, Nonce: 2, Sender: []byte("addr2"), Data: [][]byte{[]byte("data")}}, GasLimit: 125000}},
	}

	gasBandwidth := uint64(200000)

	selectedTxs, skippedTxs := txs.PreFilterTransactionsWithPriority(transactions, gasBandwidth)

	// Assert the number of selected and skipped transactions
	assert.Len(t, selectedTxs, 3, "Expected 3 selected transactions")
	assert.Len(t, skippedTxs, 2, "Expected 2 skipped transactions")

	shouldSelect := []uint32{0, 3, 1}
	shouldSkip := []uint32{2, 4}
	for i, tx := range selectedTxs {
		assert.Equal(t, shouldSelect[i], tx.Tx.GetRaw().Version, "Unexpected transaction selected")
	}

	for i, tx := range skippedTxs {
		assert.Equal(t, shouldSkip[i], tx.Tx.GetRaw().Version, "Unexpected transaction skipped")
	}

	// Check that selected transactions respect the gas bandwidth
	totalGasSelected := uint64(0)
	for _, tx := range selectedTxs {
		totalGasSelected += tx.Tx.GetGasLimit()
	}
	assert.LessOrEqual(t, totalGasSelected, gasBandwidth, "Total gas of selected transactions should not exceed gasBandwidth")

	// Test with a smaller gasBandwidth
	smallerGasBandwidth := uint64(50000)
	selectedTxsSmall, skippedTxsSmall := txs.PreFilterTransactionsWithPriority(transactions, smallerGasBandwidth)

	// even tho the we have a small gas bandwidth, we should still select the first 2 transactions
	// as they have no data field and are priority transactions
	assert.Len(t, selectedTxsSmall, 2, "Expected 2 selected transactions with smaller gasBandwidth")
	assert.Len(t, skippedTxsSmall, 3, "Expected 3 skipped transactions with smaller gasBandwidth")

	// check that selected transaction has no data field
	for _, tx := range selectedTxsSmall {
		assert.Len(t, tx.Tx.GetRaw().Data, 0, "Selected transaction should have no data field")
	}
}

func createCacheWithTransactions(t *testing.T, transactions []*txcache.WrappedTransaction) *commonMock.PoolsHolderStub {
	config := txcache.Config{
		Name:                          "untitled",
		NumChunks:                     16,
		CountThreshold:                100_000,
		CountPerSenderThreshold:       math.MaxUint32,
		NumSendersToPreemptivelyEvict: 200,
		NumBytesThreshold:             1_073_741_824,
		NumBytesPerSenderThreshold:    33_554_432,
	}

	cache, err := txcache.NewTxCache(config)
	require.NoError(t, err)

	for _, tx := range transactions {
		cache.AddTx(tx)
	}

	return &commonMock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &commonMock.ShardedDataStub{
				ShardDataStoreCalled: func(id string) (c storage.Cacher) {
					return cache
				},
				RemoveDataCalled: func(key []byte, cacheID string) {
					cache.RemoveTxByHash(key)
				},
			}
		},
	}
}

func TestTransactions_ProcessBlockTransactions(t *testing.T) {
	t.Parallel()

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000}},
		{TxHash: []byte("TX3"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX4"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX5"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 4, Nonce: 2, Sender: []byte("addr2"), Data: [][]byte{[]byte("data")}}, GasLimit: 125000}},
	})

	txs := createGoodPreprocessor(poolHolders)

	// Create a mock block with transactions
	blk := &block.Block{
		TxHashes: [][]byte{
			[]byte("TX1"),
			[]byte("TX2"),
			[]byte("TX5"),
		},
	}

	haveTime := func() bool { return true }

	// mock TXProcessor
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
		return nil
	}

	processResult, err := txs.ProcessBlockTransactions(blk, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, blk.TxHashes, processResult.Hashes())
	assert.Equal(t, 3, processResult.Length())
}

func TestTransactions_ProcessBlockTransactions_TransactionResultMismatch(t *testing.T) {
	t.Parallel()

	// Create transactions with Result field NOT set to FAILED
	// This ensures the error handling path is triggered
	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}},
		{TxHash: []byte("TX2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000, Result: transaction.Transaction_SUCCESS}},
		{TxHash: []byte("TX3"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}},
	})

	txs := createGoodPreprocessor(poolHolders)

	// Create a mock block with transactions
	blk := &block.Block{
		TxHashes: [][]byte{
			[]byte("TX1"),
			[]byte("TX2"),
			[]byte("TX3"),
		},
	}

	haveTime := func() bool { return true }

	// Mock TXProcessor to return ErrTransactionResultMismatch on second transaction
	// This simulates a consensus mismatch scenario where the transaction result
	// doesn't match what was expected from the block leader
	callCount := 0
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		callCount++
		if callCount == 2 {
			// Return ErrTransactionResultMismatch on second transaction
			// This should trigger the error handling path that logs the error
			// and returns immediately, stopping block processing
			return process.ErrTransactionResultMismatch
		}
		return nil
	}

	processResult, err := txs.ProcessBlockTransactions(blk, haveTime)

	// Should return error when ErrTransactionResultMismatch occurs
	// This validates that the block processing is aborted when consensus mismatch is detected
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, process.ErrTransactionResultMismatch))

	// Result contains only transactions processed before the mismatch (TX1)
	assert.NotNil(t, processResult)
	assert.Equal(t, 1, processResult.Length())
	assert.Equal(t, []byte("TX1"), processResult.Hashes()[0])
}

func TestTransactions_CreateAndProcessBlockTransactions(t *testing.T) {
	t.Parallel()

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000}},
		{TxHash: []byte("TX3"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX4"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX5"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 4, Nonce: 2, Sender: []byte("addr2"), Data: [][]byte{[]byte("data")}}, GasLimit: 125000}},
		{TxHash: []byte("TX6"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 5, Nonce: 1, Sender: []byte("addr3"), Data: [][]byte{}}, GasLimit: 50000}},
	})

	txs := createGoodPreprocessor(poolHolders)

	// Create a mock block with transactions
	blk := &block.Block{Header: &block.BlockHeader{Nonce: 1, RandSeed: []byte("rand_seed")}}

	haveTime := func() bool { return true }

	// mock TXProcessor
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
		return nil
	}
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 300_000

	processResult, err := txs.CreateAndProcessBlockTransactions(blk, haveTime)
	assert.Nil(t, err)
	assert.LessOrEqual(t, processResult.Length(), 5)

	// Remove Bad TXs TX4
	// mock TXProcessor
	txs.GetTXProcessor().(*mock.TxProcessorMock).PreProcessTransactionCalled = func(transaction *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
		if transaction.RawData.Version == 3 {
			return nil, nil, errors.New("bad tx")
		}

		return nil, nil, nil
	}

	processResult, err = txs.CreateAndProcessBlockTransactions(blk, haveTime)
	assert.Nil(t, err)
	assert.LessOrEqual(t, processResult.Length(), 5)

	// error on `ComputeSortedTxs` should return with no selections, only happen if pool is corrupted
	dataPool := initDataPool()
	txs = createGoodPreprocessor(dataPool)
	processResult, err = txs.CreateAndProcessBlockTransactions(blk, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, 0, processResult.Length())

	// no error but no transactions to process
	poolHolders = createCacheWithTransactions(t, []*txcache.WrappedTransaction{})
	txs = createGoodPreprocessor(poolHolders)
	processResult, err = txs.CreateAndProcessBlockTransactions(blk, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, 0, processResult.Length())

	// select TX, but no time to process
	poolHolders = createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
	})
	txs = createGoodPreprocessor(poolHolders)
	haveNoTime := func() bool { return false }
	processResult, err = txs.CreateAndProcessBlockTransactions(blk, haveNoTime)
	assert.Nil(t, err)
	assert.Equal(t, 0, processResult.Length())
}

// TestTransactions_CreateAndProcessBlock_ExcludesFrozenSender pins the proposer
// guarantee: a frozen sender's tx is never proposed (skipped before ProcessTransaction),
// a normal sender's is. The exclusion comes from the shared emergency guard today and
// from the FixMarketBuyOverflow check once the guard is retired — the test holds for both.
func TestTransactions_CreateAndProcessBlock_ExcludesFrozenSender(t *testing.T) {
	t.Parallel()

	frozenSender, err := hex.DecodeString("54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b")
	require.NoError(t, err)
	require.True(t, common.IsAccountFrozen(frozenSender))

	normalSender, err := hex.DecodeString("11111111111111111111111111111111111111111111111111111111111111ff")
	require.NoError(t, err)
	require.False(t, common.IsAccountFrozen(normalSender))

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX-FROZEN"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 1, Sender: frozenSender, Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX-NORMAL"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 1, Sender: normalSender, Data: [][]byte{}}, GasLimit: 50000}},
	})

	txs := createGoodPreprocessorWithFork(poolHolders, &commonMock.ForkControllerStub{FixMarketBuyOverflowValue: true})

	processed := map[string]bool{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, _ []byte, tx *transaction.Transaction) error {
		processed[string(tx.GetSender())] = true
		return nil
	}
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 300_000

	blk := &block.Block{Header: &block.BlockHeader{Nonce: 1, RandSeed: []byte("rand_seed")}}
	processResult, err := txs.CreateAndProcessBlockTransactions(blk, func() bool { return true })
	require.NoError(t, err)

	require.False(t, processed[string(frozenSender)], "a frozen sender must never reach ProcessTransaction on the proposer path")
	require.True(t, processed[string(normalSender)], "a normal sender must be processed")
	require.Equal(t, 1, processResult.Length(), "only the normal tx should be included in the proposed block")
}
