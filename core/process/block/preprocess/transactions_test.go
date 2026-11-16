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
	"google.golang.org/protobuf/proto"
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
		//txReceived should be all txs in the same order, as candidate-local copies
		//rather than the pooled objects themselves
		assert.NotSame(t, tx, txsRetrieved[idx])
		assert.True(t, proto.Equal(tx, txsRetrieved[idx]))
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

func TestTransactions_ProcessBlockTransactions_AcceptLeaderNotRejected(t *testing.T) {
	t.Parallel()

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000}},
		{TxHash: []byte("TX3"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000}},
	})

	txs := createGoodPreprocessor(poolHolders)

	blk := &block.Block{
		TxHashes: [][]byte{
			[]byte("TX1"),
			[]byte("TX2"),
			[]byte("TX3"),
		},
	}

	haveTime := func() bool { return true }

	callCount := 0
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		callCount++
		if callCount == 2 {
			return process.ErrTransactionResultMismatchAcceptLeader
		}
		return nil
	}

	processResult, err := txs.ProcessBlockTransactions(blk, haveTime)

	// Block accepted (not rejected), and the reproduced tx stays in the result.
	assert.Nil(t, err)
	assert.Equal(t, 3, processResult.Length())
	assert.Equal(t, blk.TxHashes, processResult.Hashes())
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

	forkStatus := &commonMock.ForkControllerStub{FixMarketBuyOverflowValue: true}

	frozenSender, err := hex.DecodeString("54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b")
	require.NoError(t, err)
	require.True(t, common.IsAccountFrozen(frozenSender, forkStatus))

	normalSender, err := hex.DecodeString("11111111111111111111111111111111111111111111111111111111111111ff")
	require.NoError(t, err)
	require.False(t, common.IsAccountFrozen(normalSender, forkStatus))

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX-FROZEN"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 1, Sender: frozenSender, Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX-NORMAL"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 1, Sender: normalSender, Data: [][]byte{}}, GasLimit: 50000}},
	})

	txs := createGoodPreprocessorWithFork(poolHolders, forkStatus)

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

func TestTransactions_CreateAndProcessBlock_SkipsSenderAfterHigherNonce(t *testing.T) {
	t.Parallel()

	gappedSender := []byte("addr-gapped")
	healthySender := []byte("addr-healthy")

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX-GAP-1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 1, Sender: gappedSender, Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX-GAP-2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 2, Sender: gappedSender, Data: [][]byte{}}, GasLimit: 50000}},
		{TxHash: []byte("TX-OK"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: healthySender, Data: [][]byte{}}, GasLimit: 50000}},
	})

	txs := createGoodPreprocessor(poolHolders)

	preProcessedPerSender := map[string]int{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
		preProcessedPerSender[string(tx.GetSender())]++
		if bytes.Equal(tx.GetSender(), gappedSender) {
			return nil, nil, process.ErrHigherNonceInTransaction
		}

		return nil, nil, nil
	}

	processed := map[string]bool{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, _ []byte, tx *transaction.Transaction) error {
		processed[string(tx.GetSender())] = true
		return nil
	}
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 300_000

	blk := &block.Block{Header: &block.BlockHeader{Nonce: 1, RandSeed: []byte("rand_seed")}}
	processResult, err := txs.CreateAndProcessBlockTransactions(blk, func() bool { return true })
	require.NoError(t, err)

	require.Equal(t, 1, preProcessedPerSender[string(gappedSender)], "once a sender hits a higher nonce its remaining txs must be skipped, not re-processed")
	require.False(t, processed[string(gappedSender)], "no tx from the gapped sender may reach ProcessTransaction")
	require.True(t, processed[string(healthySender)], "the skip must be scoped to the gapped sender")
	require.Equal(t, 1, processResult.Length(), "only the healthy sender's tx should be included in the proposed block")
}

// TestTransactions_RejectedCandidateLeavesPooledTxUntouched covers CertiK KLR-64 and
// KLR-69, which are the same defect seen from two sides: block validation used to
// execute against the transaction objects the pool stores, and no rejection path undid
// those writes.
//
// Candidate A fails T during execution, which stamps Result = FAILED. A is then
// rejected, so T never became ledger state. The pooled object must carry none of that:
// a leftover FAILED made the next candidate count T as processed although it never
// executed (KLR-64), and a leftover Block made the API report T as on-chain with
// receipts from the discarded block (KLR-69).
func TestTransactions_RejectedCandidateLeavesPooledTxUntouched(t *testing.T) {
	t.Parallel()

	pooledTx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}},
	}
	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX_T"), Tx: pooledTx},
	})

	txs := createGoodPreprocessor(poolHolders)
	blkA := &block.Block{Header: &block.BlockHeader{Nonce: 42}, TxHashes: [][]byte{[]byte("TX_T")}}
	haveTime := func() bool { return true }

	// Candidate A: T fails during execution. ProcessTransaction writes the execution
	// metadata a real processor would before reporting the failure.
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		tx.Block = blk.GetNonce()
		tx.ResultCode = transaction.Transaction_FeeInvalid
		tx.Receipts = []*transaction.Transaction_Receipt{{Data: [][]byte{[]byte("receipt")}}}
		return errors.New("execution failed")
	}

	// An executed failure is legitimately counted - it still pays fees and belongs in
	// the block. A is rejected afterwards, by the fee-total or processed-count check in
	// the coordinator, which is where the leak used to survive.
	processResult, err := txs.ProcessBlockTransactions(blkA, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, 1, processResult.Length())

	// Candidate A was rejected. The pooled object must be pristine.
	assert.Equal(t, transaction.Transaction_SUCCESS, pooledTx.Result)
	assert.Equal(t, transaction.Transaction_Ok, pooledTx.ResultCode)
	assert.Equal(t, uint64(0), pooledTx.Block)
	assert.Empty(t, pooledTx.Receipts)
}

// TestTransactions_PrecheckRejectionAfterFailedCandidateIsNotCounted is the
// two-candidate scenario from KLR-64. Candidate A stamps T as FAILED and is rejected.
// In candidate B a valid same-sender transaction precedes T, so T is rejected by the
// nonce precheck before it ever executes. With the stale FAILED on the shared object,
// the bad-tx branch was skipped and T's hash was still counted as processed, letting
// this node accept a block a clean validator rejects.
func TestTransactions_PrecheckRejectionAfterFailedCandidateIsNotCounted(t *testing.T) {
	t.Parallel()

	pooledTx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}},
	}
	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX_T"), Tx: pooledTx},
	})

	txs := createGoodPreprocessor(poolHolders)
	haveTime := func() bool { return true }
	txProc := txs.GetTXProcessor().(*mock.TxProcessorMock)

	// Candidate A: T fails during execution and is stamped FAILED, then A is rejected.
	txProc.ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		tx.Result = transaction.Transaction_FAILED
		return errors.New("execution failed")
	}
	_, err := txs.ProcessBlockTransactions(&block.Block{Header: &block.BlockHeader{Nonce: 42}, TxHashes: [][]byte{[]byte("TX_T")}}, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, transaction.Transaction_SUCCESS, pooledTx.Result)

	// Candidate B: T is rejected by the precheck, before any execution.
	txs.CreateBlockStarted()
	txProc.PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
		return nil, nil, process.ErrLowerNonceInTransaction
	}
	txProc.ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		assert.Fail(t, "a precheck-rejected transaction must not be executed")
		return nil
	}

	processResult, err := txs.ProcessBlockTransactions(&block.Block{Header: &block.BlockHeader{Nonce: 43}, TxHashes: [][]byte{[]byte("TX_T")}}, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, 0, processResult.Length())
}

// A hash repeated in one block whose first run fails: the second run is nonce-rejected.
// Before FixAuditChangesV6 both entries share the pooled pointer, so the second inherits
// FAILED and is counted, as on older binaries; the gate keeps a mixed fleet in agreement.
func TestTransactions_DuplicateHashCountingIsForkGated(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, fixAuditChangesV6 bool) int {
		poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
			{TxHash: []byte("TX_T"), Tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}},
			}},
		})
		forkController := commonMock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixAuditChangesV6)
		txs := createGoodPreprocessorWithFork(poolHolders, forkController)
		txProc := txs.GetTXProcessor().(*mock.TxProcessorMock)

		preProcessed := 0
		txProc.PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
			preProcessed++
			if preProcessed > 1 {
				return nil, nil, process.ErrLowerNonceInTransaction
			}
			return nil, nil, nil
		}
		txProc.ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
			return errors.New("execution failed")
		}

		blk := &block.Block{Header: &block.BlockHeader{Nonce: 42}, TxHashes: [][]byte{[]byte("TX_T"), []byte("TX_T")}}
		processResult, err := txs.ProcessBlockTransactions(blk, func() bool { return true })
		require.NoError(t, err)

		return processResult.Length()
	}

	t.Run("before FixAuditChangesV6", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 2, run(t, false))
	})
	t.Run("from FixAuditChangesV6", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 1, run(t, true))
	})
}

// Block processing only gas-stamps copies now, so a restored tx must keep its stored gas:
// the proposer's tx selection reads GasLimit from the pooled object.
func TestTransactions_RestoreBlockDataIntoPoolsKeepsGas(t *testing.T) {
	t.Parallel()

	marshalizer := &commonMock.MarshalizerMock{}
	storedTx := &transaction.Transaction{
		RawData:       &transaction.Transaction_Raw{Nonce: 1, Sender: []byte("addr1")},
		GasLimit:      123,
		GasMultiplier: 4,
		Block:         7,
		Result:        transaction.Transaction_FAILED,
	}
	txBuff, err := marshalizer.Marshal(storedTx)
	require.NoError(t, err)

	var restored *transaction.Transaction
	preprocessor, err := preprocess.NewTransactionPreprocessor(
		&commonMock.ShardedDataStub{
			AddDataCalled: func(_ []byte, data interface{}, _ int, _ string) {
				restored = data.(*transaction.Transaction)
			},
		},
		&commonMock.ChainStorerMock{
			GetAllCalled: func(_ retriever.UnitType, _ [][]byte) (map[string][]byte, error) {
				return map[string][]byte{"TX_T": txBuff}, nil
			},
			GetStorerCalled: func(_ retriever.UnitType) storage.Storer {
				return &commonMock.StorerStub{RemoveCalled: func(_ []byte) error { return nil }}
			},
		},
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

	_, err = preprocessor.RestoreBlockDataIntoPools(&block.Block{Header: &block.BlockHeader{}, TxHashes: [][]byte{[]byte("TX_T")}})
	require.NoError(t, err)

	require.NotNil(t, restored)
	assert.Equal(t, uint64(123), restored.GasLimit)
	assert.Equal(t, uint64(4), restored.GasMultiplier)
	assert.Equal(t, uint64(0), restored.Block)
	assert.Equal(t, transaction.Transaction_SUCCESS, restored.Result)
}

// The proposer path must execute against copies too: its own execution leaves no
// metadata on the pooled object (KLR-69). The second half is a guard only - the old
// proposer path also cleared a stale FAILED, via PrepareForProcessing on the pooled tx.
func TestTransactions_CreateAndProcessBlockLeavesPooledTxUntouched(t *testing.T) {
	t.Parallel()

	pooledTx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}},
	}
	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX_T"), Tx: pooledTx},
	})

	txs := createGoodPreprocessor(poolHolders)
	haveTime := func() bool { return true }
	txProc := txs.GetTXProcessor().(*mock.TxProcessorMock)

	txProc.ProcessTransactionCalled = func(blk *block.Block, txHash []byte, tx *transaction.Transaction) error {
		tx.Block = blk.GetNonce()
		tx.ResultCode = transaction.Transaction_FeeInvalid
		tx.Receipts = []*transaction.Transaction_Receipt{{Data: [][]byte{[]byte("receipt")}}}
		return errors.New("execution failed")
	}
	processResult, err := txs.CreateAndProcessBlockTransactions(&block.Block{Header: &block.BlockHeader{Nonce: 42}}, haveTime)
	require.NoError(t, err)
	require.Equal(t, 1, processResult.Length())
	require.True(t, proto.Equal(&transaction.Transaction{RawData: pooledTx.RawData}, pooledTx), "pooled tx must keep its proto defaults")

	pooledTx.Result = transaction.Transaction_FAILED
	txs.CreateBlockStarted()
	txProc.PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
		return nil, nil, process.ErrLowerNonceInTransaction
	}
	processResult, err = txs.CreateAndProcessBlockTransactions(&block.Block{Header: &block.BlockHeader{Nonce: 43}}, haveTime)
	require.NoError(t, err)
	require.Equal(t, 0, processResult.Length())
}

// KLC-2806: verifiers bound a block's declared gas from FixAuditChangesV6 and accept it unchanged before.
func TestTransactions_ProcessBlockTransactionsGasBoundIsForkGated(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, fixAuditChangesV6 bool) (int, error) {
		poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
			{TxHash: []byte("TX_A"), Tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}},
			}},
			{TxHash: []byte("TX_B"), Tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}},
			}},
		})
		forkController := commonMock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixAuditChangesV6)
		txs := createGoodPreprocessorWithFork(poolHolders, forkController)
		txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 100
		txProc := txs.GetTXProcessor().(*mock.TxProcessorMock)
		txProc.PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
			tx.GasLimit = 60
			return nil, nil, nil
		}
		txProc.ProcessTransactionCalled = func(_ *block.Block, _ []byte, _ *transaction.Transaction) error {
			return nil
		}

		blk := &block.Block{Header: &block.BlockHeader{Nonce: 42}, TxHashes: [][]byte{[]byte("TX_A"), []byte("TX_B")}}
		processResult, err := txs.ProcessBlockTransactions(blk, func() bool { return true })

		return processResult.Length(), err
	}

	t.Run("before FixAuditChangesV6", func(t *testing.T) {
		t.Parallel()
		numTxs, err := run(t, false)
		require.NoError(t, err)
		assert.Equal(t, 2, numTxs)
	})
	t.Run("from FixAuditChangesV6", func(t *testing.T) {
		t.Parallel()
		_, err := run(t, true)
		assert.ErrorIs(t, err, process.ErrMaxGasLimitPerBlockExceeded)
	})
}

// KLC-2806: a tx over the block gas left is skipped with its sender's next nonces, and the
// skipped txs stay pooled with their gas, so the next slot's selection still counts it.
func TestTransactions_CreateAndProcessBlockSkipsTxOverGasBound(t *testing.T) {
	t.Parallel()

	first := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 7}
	second := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 7}
	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX_1"), Tx: first},
		{TxHash: []byte("TX_2"), Tx: second},
	})
	forkController := commonMock.NewForkControllerStub().SetFork("FixAuditChangesV6", true)
	txs := createGoodPreprocessorWithFork(poolHolders, forkController)
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 100

	preProcessed := 0
	txs.GetTXProcessor().(*mock.TxProcessorMock).PreProcessTransactionCalled = func(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
		preProcessed++
		tx.GasLimit = 150
		return nil, nil, nil
	}

	processResult, err := txs.CreateAndProcessBlockTransactions(&block.Block{Header: &block.BlockHeader{Nonce: 42}}, func() bool { return true })
	require.NoError(t, err)
	assert.Equal(t, 0, processResult.Length())
	assert.Equal(t, 1, preProcessed, "the sender's next nonce must not run")

	cache := poolHolders.Transactions().ShardDataStore("0").(*txcache.TxCache)
	assert.Equal(t, uint64(2), cache.CountTx())
	assert.Equal(t, uint64(7), first.GasLimit)
	assert.Equal(t, uint64(7), second.GasLimit)
}

// The live timeout sentinels must abort block processing exactly like a plain result mismatch,
// otherwise a validator that abstains on a local timeout would keep executing the block. The
// control row wraps an unrelated reason in the same error type and must not abort, which shows the
// short-circuit is reached through the reason and not through the error type.
func TestTransactions_ProcessBlockTransactions_TimeoutSentinelsShortCircuit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		wantAbort     bool
		wantProcessed int
	}{
		{name: "plain mismatch", err: process.ErrTransactionResultMismatch, wantAbort: true, wantProcessed: 1},
		{
			name:      "local timeout abstain",
			err:       &process.TxResultMismatchError{Reason: process.ErrLocalExecutionTimeoutAbstain, TxHash: []byte("TX2"), Contract: "sc"},
			wantAbort: true, wantProcessed: 1,
		},
		{
			name:      "leader timeout refused",
			err:       &process.TxResultMismatchError{Reason: process.ErrLeaderTimeoutRefused, TxHash: []byte("TX2"), Contract: "sc"},
			wantAbort: true, wantProcessed: 1,
		},
		{
			name:      "control: unrelated reason in the same type does not abort",
			err:       &process.TxResultMismatchError{Reason: errors.New("unrelated"), TxHash: []byte("TX2")},
			wantAbort: false, wantProcessed: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
				{TxHash: []byte("TX1"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}},
				{TxHash: []byte("TX2"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000, Result: transaction.Transaction_SUCCESS}},
				{TxHash: []byte("TX3"), Tx: &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}},
			})
			txs := createGoodPreprocessor(poolHolders)
			blk := &block.Block{TxHashes: [][]byte{[]byte("TX1"), []byte("TX2"), []byte("TX3")}}

			callCount := 0
			txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, _ []byte, _ *transaction.Transaction) error {
				callCount++
				if callCount == 2 {
					return tt.err
				}
				return nil
			}

			processResult, err := txs.ProcessBlockTransactions(blk, func() bool { return true })

			if tt.wantAbort {
				assert.ErrorIs(t, err, process.ErrTransactionResultMismatch)
				assert.Equal(t, 2, callCount, "TX3 must not be executed once the block is rejected")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 3, callCount)
			}
			assert.Equal(t, tt.wantProcessed, processResult.Length())
		})
	}
}
