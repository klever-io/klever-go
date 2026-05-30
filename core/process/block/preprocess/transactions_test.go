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
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/vmhost"
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

// TestTransactions_CreateAndProcessBlock_LeaderSCTimeout pins down the leader-side
// behavior when an SC TX times out during block-build.
//
// Required behavior (KLC-2397): on local timeout, the leader must NOT include the
// TX in the block AND must NOT charge the user. State changes from
// ProcessBandwidthFee (BW fee debit, nonce++, feeHandler accumulator entry) must
// all be reverted; the TX stays in mempool for the next leader (or eventually
// times out of the pool via TTL). The sender is marked for skip in this slot so
// their dependent-nonce follow-up TXs are also deferred.
//
// Backward-compat: validator path (ProcessBlockTransactions) intentionally does
// NOT take pre-fee snapshots and does NOT have this skip path — replaying a
// block from an OLD leader that included a timed-out TX as FAILED with fee
// debited still works, with cross-version mismatches handled by KLC-1894's
// tolerance-band check in handleResultMismatch.
func TestTransactions_CreateAndProcessBlock_LeaderSCTimeout(t *testing.T) {
	t.Parallel()

	// TX2 will time out during ProcessTransaction (simulating SC timeout on leader).
	// TX3 is from the SAME sender as TX2 — must also be skipped (dependent nonce).
	// TX4 is from a different sender — must NOT be affected.
	tx1 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}
	tx2 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000, Result: transaction.Transaction_SUCCESS}
	tx3 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}
	tx4 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: tx1},
		{TxHash: []byte("TX2"), Tx: tx2},
		{TxHash: []byte("TX3"), Tx: tx3},
		{TxHash: []byte("TX4"), Tx: tx4},
	})

	txs := createGoodPreprocessor(poolHolders)

	blk := &block.Block{Header: &block.BlockHeader{Nonce: 1, RandSeed: []byte("rand_seed")}}
	haveTime := func() bool { return true }

	// Track which TXs had BW fee processed (line 194 of txProcess.go reached).
	bwFeeProcessed := map[string]bool{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessBandwidthFeeCalled = func(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error) {
		bwFeeProcessed[string(txHash)] = true
		return tx.GetBandwidthFee(), nil
	}

	// Track BW fee accumulator reverts (the partial-revert-bug guard).
	bwFeeReverted := map[string]int64{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).RevertBandwidthFeeCalled = func(txHash []byte, bwFee int64) error {
		bwFeeReverted[string(txHash)] = bwFee
		return nil
	}

	// TX2 times out; everything else succeeds.
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, txHash []byte, _ *transaction.Transaction) error {
		if string(txHash) == "TX2" {
			return vmhost.ErrExecutionFailedWithTimeout
		}
		return nil
	}
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 300_000

	processResult, err := txs.CreateAndProcessBlockTransactions(blk, haveTime)
	require.Nil(t, err)
	require.NotNil(t, processResult)

	hashesInBlock := map[string]bool{}
	for _, h := range processResult.Hashes() {
		hashesInBlock[string(h)] = true
	}

	// (1) BW fee WAS processed for the timed-out TX (line 194 reached BEFORE the
	//     VM call timed out).
	assert.True(t, bwFeeProcessed["TX2"],
		"ProcessBandwidthFee must have been called for TX2 (timeout happens AFTER fee in ProcessTransaction)")

	// (2) BW fee accumulator WAS reverted (the consistency-restoring step).
	assert.Equal(t, tx2.GetBandwidthFee(), bwFeeReverted["TX2"],
		"BW fee must be reverted from feeHandler accumulator so block header TxFees stays consistent")

	// (3) The timed-out TX MUST NOT be in the block (no charge to user).
	assert.False(t, hashesInBlock["TX2"],
		"timed-out TX must NOT be in block — user is not charged for failed leader execution")

	// (4) TX3 (same sender, dependent nonce) is also skipped — its nonce depends on
	//     TX2 having executed, which didn't happen.
	assert.False(t, hashesInBlock["TX3"],
		"follow-up TX from same sender must be skipped (dependent nonce)")

	// (5) TX1 is addr1's nonce-1 TX — processed and committed BEFORE TX2's timeout, so
	//     its inclusion isn't affected by senderAddressToSkip being set on TX2.
	//     TX4 is from a different sender, so the sender-skip on addr1 doesn't apply.
	assert.True(t, hashesInBlock["TX1"], "TX1 (success, executed before timeout) must be in block")
	assert.True(t, hashesInBlock["TX4"], "TX4 (success, different sender) must be in block")

	// (6) tx2.Result remains FAILED in the in-memory pooled object after the leader
	//     skip path returns. The mempool's next selection (or the next leader's
	//     createAndProcessBlock) calls tx.PrepareForProcessing() at the top of its
	//     loop (transactions.go:708), which resets Result/ResultCode/Receipts/GasLimit
	//     before any processing — so no explicit reset is needed here. This assertion
	//     just pins that we do NOT redundantly reset Result in the skip path itself
	//     (avoids zeroing GasLimit on the mempool-shared pointer).
	assert.Equal(t, transaction.Transaction_FAILED, tx2.Result,
		"leader skip path must not reset tx state — next iteration's PrepareForProcessing handles it")
}

// TestTransactions_ProcessBlockTransactions_ValidatorSCTimeout pins down the
// validator-side behavior on local SC timeout — intentionally DIFFERENT from the
// leader path. The validator must include the TX as FAILED with fee debited
// (develop behavior) so it can validate blocks produced by OLD leaders that did
// the same. Cross-version mismatches with NEW-leader blocks are handled by the
// tolerance-band check in handleResultMismatch (KLC-1894), not by skipping here.
//
// Regression guard: if anyone ever moves the leader-side skip logic from
// createAndProcessBlock into processAndRemoveBadTransaction (the original PR
// design that was rejected for replay-unsafety), this test breaks.
func TestTransactions_ProcessBlockTransactions_ValidatorSCTimeout(t *testing.T) {
	t.Parallel()

	tx1 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}
	tx2 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}}, GasLimit: 100000, Result: transaction.Transaction_SUCCESS}
	tx3 := &transaction.Transaction{RawData: &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}}, GasLimit: 50000, Result: transaction.Transaction_SUCCESS}

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: tx1},
		{TxHash: []byte("TX2"), Tx: tx2},
		{TxHash: []byte("TX3"), Tx: tx3},
	})

	txs := createGoodPreprocessor(poolHolders)

	// Block from an old-version leader that included TX2 as FAILED on its own
	// timeout. Validator must accept this shape.
	blk := &block.Block{
		TxHashes: [][]byte{[]byte("TX1"), []byte("TX2"), []byte("TX3")},
	}
	haveTime := func() bool { return true }

	bwFeeProcessed := map[string]bool{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessBandwidthFeeCalled = func(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error) {
		bwFeeProcessed[string(txHash)] = true
		return tx.GetBandwidthFee(), nil
	}

	// Validator-side: BW fee revert must NOT be called (regression guard against
	// accidentally adding the leader-skip path into the validator flow).
	bwFeeReverted := map[string]bool{}
	txs.GetTXProcessor().(*mock.TxProcessorMock).RevertBandwidthFeeCalled = func(txHash []byte, bwFee int64) error {
		bwFeeReverted[string(txHash)] = true
		return nil
	}

	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, txHash []byte, _ *transaction.Transaction) error {
		if string(txHash) == "TX2" {
			return vmhost.ErrExecutionFailedWithTimeout
		}
		return nil
	}

	processResult, err := txs.ProcessBlockTransactions(blk, haveTime)
	require.Nil(t, err)
	require.NotNil(t, processResult)

	hashesInBlock := map[string]bool{}
	for _, h := range processResult.Hashes() {
		hashesInBlock[string(h)] = true
	}

	// (1) BW fee processed (same as leader path).
	assert.True(t, bwFeeProcessed["TX2"], "validator must process BW fee on timeout same as leader did originally")

	// (2) BW fee NOT reverted (validator preserves develop behavior — TX stays in
	//     block as FAILED, fee charged, matches old-leader chain entry).
	assert.False(t, bwFeeReverted["TX2"],
		"validator MUST NOT revert BW fee on timeout — would break replay of pre-PR blocks")

	// (3) TX2 IS in result (TX matches the on-chain entry from old leader).
	assert.True(t, hashesInBlock["TX2"],
		"validator must include timed-out TX in result to match on-chain FAILED entry from old leader")

	// (4) tx2.Result is FAILED (matches what old leader recorded).
	assert.Equal(t, transaction.Transaction_FAILED, tx2.Result,
		"validator's local result for timeout matches old-leader chain entry (FAILED)")
}

// TestTransactions_CreateAndProcessBlock_LeaderSCTimeout_Integration is the
// higher-confidence variant of LeaderSCTimeout. Instead of mocking the BW-fee
// revert via TxProcessorMock.RevertBandwidthFeeCalled, it wires a real
// postprocess.feeHandler underneath so the test exercises the actual
// in-memory accumulator behavior end-to-end.
//
// Assertions are on the REAL feeHandler state after CreateAndProcessBlockTransactions:
//   - accumulator (GetAccumulatedTxFees) includes BW fees for the TXs successfully included
//   - accumulator does NOT include BW fee for the timed-out TX
//   - double-revert is a no-op (RevertTransactionFee clamps revert amount to the
//     remaining stored fee; the mapHashFee entry is zeroed but kept in the map until
//     CreateBlockStarted clears the map at next block — see feeHandler.go:115-128)
//
// This is the test that protects against silent regressions in the leader-skip
// path's interaction with the real fee accumulator.
func TestTransactions_CreateAndProcessBlock_LeaderSCTimeout_Integration(t *testing.T) {
	t.Parallel()

	const tx1BWFee = int64(100)
	const tx2BWFee = int64(250) // the one that times out — must NOT remain in accumulator
	const tx4BWFee = int64(175)

	tx1 := &transaction.Transaction{
		RawData:  &transaction.Transaction_Raw{Version: 0, Nonce: 1, Sender: []byte("addr1"), Data: [][]byte{}, BandwidthFee: tx1BWFee},
		GasLimit: 50000, Result: transaction.Transaction_SUCCESS,
	}
	tx2 := &transaction.Transaction{
		RawData:  &transaction.Transaction_Raw{Version: 1, Nonce: 2, Sender: []byte("addr1"), Data: [][]byte{[]byte("data")}, BandwidthFee: tx2BWFee},
		GasLimit: 100000, Result: transaction.Transaction_SUCCESS,
	}
	tx3 := &transaction.Transaction{
		RawData:  &transaction.Transaction_Raw{Version: 2, Nonce: 3, Sender: []byte("addr1"), Data: [][]byte{}, BandwidthFee: 50},
		GasLimit: 50000, Result: transaction.Transaction_SUCCESS,
	}
	tx4 := &transaction.Transaction{
		RawData:  &transaction.Transaction_Raw{Version: 3, Nonce: 1, Sender: []byte("addr2"), Data: [][]byte{}, BandwidthFee: tx4BWFee},
		GasLimit: 50000, Result: transaction.Transaction_SUCCESS,
	}

	poolHolders := createCacheWithTransactions(t, []*txcache.WrappedTransaction{
		{TxHash: []byte("TX1"), Tx: tx1},
		{TxHash: []byte("TX2"), Tx: tx2},
		{TxHash: []byte("TX3"), Tx: tx3},
		{TxHash: []byte("TX4"), Tx: tx4},
	})

	txs := createGoodPreprocessor(poolHolders)

	// Real fee accumulator — the actual production type, not a mock.
	realFeeHandler, err := postprocess.NewFeeAccumulator()
	require.NoError(t, err)

	// Wire the TxProcessorMock so its ProcessBandwidthFee and RevertBandwidthFee
	// forward to the REAL feeHandler — the rest of the txProcessor flow stays
	// mocked but the in-memory fee state is real.
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessBandwidthFeeCalled = func(txHash []byte, tx *transaction.Transaction, _ state.UserAccountHandler) (int64, error) {
		realFeeHandler.ProcessTransactionFee(tx.GetBandwidthFee(), 0, txHash)
		return tx.GetBandwidthFee(), nil
	}
	txs.GetTXProcessor().(*mock.TxProcessorMock).RevertBandwidthFeeCalled = func(txHash []byte, bwFee int64) error {
		realFeeHandler.RevertTransactionFee(txHash, bwFee, 0)
		return nil
	}
	txs.GetTXProcessor().(*mock.TxProcessorMock).ProcessTransactionCalled = func(_ *block.Block, txHash []byte, _ *transaction.Transaction) error {
		if string(txHash) == "TX2" {
			return vmhost.ErrExecutionFailedWithTimeout
		}
		return nil
	}
	txs.GetEconomicsFee().(*commonMock.FeeHandlerStub).MaxGasLimitPerBlockValue = 300_000

	processResult, err := txs.CreateAndProcessBlockTransactions(blk(), func() bool { return true })
	require.Nil(t, err)
	require.NotNil(t, processResult)

	hashesInBlock := map[string]bool{}
	for _, h := range processResult.Hashes() {
		hashesInBlock[string(h)] = true
	}

	// Block contents: TX1 and TX4 are in. TX2 (timed out) and TX3 (dependent nonce)
	// are NOT.
	assert.True(t, hashesInBlock["TX1"], "TX1 (success) in block")
	assert.False(t, hashesInBlock["TX2"], "TX2 (timed out) NOT in block")
	assert.False(t, hashesInBlock["TX3"], "TX3 (same sender, dependent nonce) NOT in block")
	assert.True(t, hashesInBlock["TX4"], "TX4 (different sender) in block")

	// THE INTEGRATION ASSERTION: the real feeHandler's accumulator must agree with
	// what's in the block. TX2's BW fee must NOT be in the accumulator.
	expectedAccumulated := tx1BWFee + tx4BWFee // TX1 + TX4 only; TX2 reverted, TX3 never processed
	assert.Equal(t, expectedAccumulated, realFeeHandler.GetAccumulatedTxFees(),
		"feeHandler accumulator must equal sum of BW fees for ONLY the TXs that are in the block")

	// Additional belt-and-suspenders: explicit re-revert of TX2's fee. RevertTransactionFee
	// (feeHandler.go:115-128) clamps the revert amount to the remaining stored fee, so
	// even though the mapHashFee entry persists (zeroed) after the leader's revert, a
	// second revert finds fee.TxFee=0 and decrements by min(0, tx2BWFee)=0. Net change
	// to the accumulator is zero — idempotent.
	balanceBefore := realFeeHandler.GetAccumulatedTxFees()
	realFeeHandler.RevertTransactionFee([]byte("TX2"), tx2BWFee, 0)
	assert.Equal(t, balanceBefore, realFeeHandler.GetAccumulatedTxFees(),
		"double-revert of TX2 must be a no-op (clamp to remaining fee=0, not because the map entry is gone)")
}

// blk returns a fresh test block. Helper to avoid sharing block state across tests.
func blk() *block.Block {
	return &block.Block{Header: &block.BlockHeader{Nonce: 1, RandSeed: []byte("rand_seed")}}
}
