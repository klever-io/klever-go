package coordinator_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/coordinator"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/stretchr/testify/assert"
)

const MaxGasLimitPerBlock = uint64(100000)

func createShardedDataChacherNotifier(
	handler data.TransactionHandler,
	testHash []byte,
) func() retriever.ShardedDataCacherNotifier {
	return func() retriever.ShardedDataCacherNotifier {
		return &commonMock.ShardedDataStub{
			RegisterOnAddedCalled: func(i func(key []byte, value interface{})) {},
			ShardDataStoreCalled: func(id string) (c storage.Cacher) {
				return &commonMock.CacherStub{
					PeekCalled: func(key []byte) (value interface{}, ok bool) {
						if reflect.DeepEqual(key, testHash) {
							return handler, true
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
			RemoveSetOfDataFromPoolCalled: func(keys [][]byte, id string) {},
			SearchFirstDataCalled: func(key []byte) (value interface{}, ok bool) {
				if reflect.DeepEqual(key, []byte("tx1_hash")) {
					return handler, true
				}
				return nil, false
			},
			AddDataCalled: func(key []byte, data interface{}, sizeInBytes int, cacheId string) {
			},
		}
	}
}

func initDataPool(testHash []byte) *commonMock.PoolsHolderStub {
	tx := &transaction.Transaction{}

	txCalled := createShardedDataChacherNotifier(tx, testHash)

	sdp := &commonMock.PoolsHolderStub{
		TransactionsCalled: txCalled,
		BlocksCalled: func() storage.Cacher {
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
		HeadersCalled: func() retriever.HeadersPool {
			cs := &commonMock.HeadersCacherStub{}
			cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {
			}
			return cs
		},
		CurrBlockTxsCalled: func() retriever.TransactionCacher {
			return &commonMock.TxForCurrentBlockStub{}
		},
	}
	return sdp
}

func generateTestCache() storage.Cacher {
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000, Shards: 1, SizeInBytes: 0})
	return cache
}

func generateTestUnit() storage.Storer {
	storer, _ := storageUnit.NewStorageUnit(
		generateTestCache(),
		memorydb.New(),
	)

	return storer
}

func initStore() *retriever.ChainStorer {
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.TransactionUnit, generateTestUnit())
	store.AddStorer(retriever.BlockUnit, generateTestUnit())
	return store
}

func initAccountsMock() *commonMock.AccountsStub {
	rootHashCalled := func() ([]byte, error) {
		return []byte("rootHash"), nil
	}
	return &commonMock.AccountsStub{
		RootHashCalled: rootHashCalled,
	}
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

func createTxLogsProcessor() process.TransactionLogProcessor {
	txLogsProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Marshalizer:          &commonMock.MarshalizerMock{},
		SaveInStorageEnabled: false,
	})

	return txLogsProcessor
}

func createMockTransactionCoordinator(dataPool retriever.PoolsHolder) (coordinator.TransactionCoordinator, error) {
	forkController := newForkController()
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

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		preprocessor,
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		forkController,
		&commonMock.SCProcessorMock{},
	)

	return coordinator.NewTestTransactionCoordinator(tc), err
}

func TestNewTransactionCoordinator_NilHasher(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		nil,
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)

	assert.Nil(t, tc)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewTransactionCoordinator_NilMarshalizer(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		nil,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)

	assert.Nil(t, tc)
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewTransactionCoordinator_NilAccountsStub(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		nil,
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)

	assert.Nil(t, tc)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestNewTransactionCoordinator_NilAssetsStub(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		nil,
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)

	assert.Nil(t, tc)
	assert.Equal(t, common.ErrNilKAppAccountsAdapter, err)
}

func TestNewTransactionCoordinator_NilRequestHandler(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		nil,
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, tc)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}

func TestNewTransactionCoordinator_NilPreProcessor(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		nil,
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, tc)
	assert.Equal(t, process.ErrNilPreProcessorsContainer, err)
}

func TestNewTransactionCoordinator_OK(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)

	assert.Nil(t, err)
	assert.NotNil(t, tc)
	assert.False(t, tc.IsInterfaceNil())
}

func TestTransactionCoordinator_CreateBlockStarted(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	tc.CreateBlockStarted()

	txs := tc.GetAllCurrentUsedTxs()
	assert.Equal(t, 0, len(txs))
}

func TestTransactionCoordinator_CreateMarshalizedDataNilBody(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	mrTxs, err := tc.CreateMarshalizedData(nil)
	assert.Equal(t, err, common.ErrNilHeader)
	assert.Equal(t, 0, len(mrTxs))
}

func createTestBlock() *block.Block {
	txHashes := make([][]byte, 0)

	txHashes = append(txHashes, []byte("tx_hash1"))
	txHashes = append(txHashes, []byte("tx_hash2"))
	txHashes = append(txHashes, []byte("tx_hash3"))
	txHashes = append(txHashes, []byte("tx_hash4"))
	txHashes = append(txHashes, []byte("tx_hash5"))
	txHashes = append(txHashes, []byte("tx_hash6"))

	return &block.Block{TxHashes: txHashes}
}

func TestTransactionCoordinator_CreateMarshalizedData(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	mrTxs, err := tc.CreateMarshalizedData(createTestBlock())
	assert.Nil(t, err)
	assert.Equal(t, 0, len(mrTxs))
}

func TestTransactionCoordinator_GetAllCurrentUsedTxs(t *testing.T) {
	t.Parallel()

	txPool, _ := commonMock.CreateTxPool(0, 0)
	tdp := initDataPool([]byte("tx_hash1"))
	tdp.TransactionsCalled = func() retriever.ShardedDataCacherNotifier {
		return txPool
	}

	_, _ = createMockTransactionCoordinator(tdp)

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	usedTxs := tc.GetAllCurrentUsedTxs()
	assert.Equal(t, 0, len(usedTxs))
}

func TestTransactionCoordinator_RequestBlockTransactionsNilBlock(t *testing.T) {
	t.Parallel()

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		&commonMock.PreProcessorMock{},
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	tc.RequestBlockTransactions(nil)

	txc := coordinator.NewTestTransactionCoordinator(tc)

	assert.Equal(t, 0, txc.RequestedTxs())
}

func TestTransactionCoordinator_RequestBlockTransactionsRequestOne(t *testing.T) {
	t.Parallel()

	txHashInPool := []byte("tx_hash1")
	tdp := initDataPool(txHashInPool)
	tc, err := createMockTransactionCoordinator(tdp)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	txHashToAsk := []byte("tx_hashnotinPool")
	block := &block.Block{TxHashes: [][]byte{txHashInPool, txHashToAsk}}
	tc.RequestBlockTransactions(block)

	assert.Equal(t, 1, tc.RequestedTxs())

	haveTime := func() time.Duration {
		return time.Second
	}
	err = tc.IsDataPreparedForProcessing(haveTime)
	assert.Equal(t, process.ErrTimeIsOut, err)
}

func TestTransactionCoordinator_IsDataPreparedForProcessing(t *testing.T) {
	t.Parallel()

	txHash := []byte("tx_hash1")
	tdp := initDataPool(txHash)
	tc, err := createMockTransactionCoordinator(tdp)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	haveTime := func() time.Duration {
		return time.Second
	}
	err = tc.IsDataPreparedForProcessing(haveTime)
	assert.Nil(t, err)
}

func TestTransactionCoordinator_SaveTxsToStorage(t *testing.T) {
	t.Parallel()

	txHash := []byte("tx_hash1")
	tdp := initDataPool(txHash)
	tc, err := createMockTransactionCoordinator(tdp)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	err = tc.SaveTxsToStorage(nil)
	assert.Nil(t, err)

	body := &block.Block{TxHashes: [][]byte{txHash}}

	tc.RequestBlockTransactions(body)

	err = tc.SaveTxsToStorage(body)
	assert.Nil(t, err)

	txHashToAsk := []byte("tx_hashnotinPool")
	body = &block.Block{TxHashes: [][]byte{txHashToAsk}}

	err = tc.SaveTxsToStorage(body)
	assert.Equal(t, process.ErrMissingTransaction, err)
}

func TestTransactionCoordinator_RestoreBlockDataFromStorage(t *testing.T) {
	t.Parallel()

	txHash := []byte("tx_hash1")
	tdp := initDataPool(txHash)
	tc, err := createMockTransactionCoordinator(tdp)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	nrTxs, err := tc.RestoreBlockDataFromStorage(nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, nrTxs)

	body := &block.Block{TxHashes: [][]byte{txHash}}

	tc.RequestBlockTransactions(body)
	err = tc.SaveTxsToStorage(body)
	assert.Nil(t, err)
	nrTxs, err = tc.RestoreBlockDataFromStorage(body)
	assert.Equal(t, 1, nrTxs)
	assert.Nil(t, err)

	txHashToAsk := []byte("tx_hashnotinPool")
	body = &block.Block{TxHashes: [][]byte{txHashToAsk}}

	err = tc.SaveTxsToStorage(body)
	assert.Equal(t, process.ErrMissingTransaction, err)

	nrTxs, err = tc.RestoreBlockDataFromStorage(body)
	assert.Equal(t, 1, nrTxs)
	assert.Nil(t, err)
}

func TestTransactionCoordinator_RemoveTxsFromPool(t *testing.T) {
	t.Parallel()

	txHash := []byte("tx_hash1")
	tdp := initDataPool(txHash)
	tc, err := createMockTransactionCoordinator(tdp)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	err = tc.RemoveTxsFromPool(nil)
	assert.Nil(t, err)

	body := &block.Block{TxHashes: [][]byte{txHash}}

	tc.RequestBlockTransactions(body)
	err = tc.RemoveTxsFromPool(body)
	assert.Nil(t, err)
}

func TestTransactionCoordinator_ProcessBlockTransaction(t *testing.T) {
	t.Parallel()

	txHash := []byte("tx_hash1")
	dataPool := initDataPool(txHash)
	requestTransaction := func(txHashes [][]byte) {}

	preprocessor, _ := preprocess.NewTransactionPreprocessor(
		dataPool.Transactions(),
		initStore(),
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{
			ProcessTransactionCalled: func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
				return nil
			},
		},
		initAccountsMock(),
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		newForkController(),
	)

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		&commonMock.RequestHandlerStub{},
		preprocessor,
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		newForkController(),
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	haveTime := func() time.Duration {
		return time.Second
	}

	// validate nil block
	_, err = tc.ProcessBlockTransactions(nil, haveTime)
	assert.Equal(t, process.ErrNilBlockHeader, err)

	_, err = tc.ProcessBlockTransactions(&block.Block{}, haveTime)
	assert.Nil(t, err)

	body := &block.Block{TxHashes: [][]byte{txHash}, Header: &block.BlockHeader{Timestamp: time.Now().Unix()}}

	tc.RequestBlockTransactions(body)
	_, err = tc.ProcessBlockTransactions(body, haveTime)
	assert.Nil(t, err)

	noTime := func() time.Duration {
		return -1
	}
	_, err = tc.ProcessBlockTransactions(body, noTime)
	assert.Equal(t, process.ErrTimeIsOut, err)

	txHashToAsk := []byte("tx_hashnotinPool")
	body = &block.Block{TxHashes: [][]byte{txHashToAsk}, Header: &block.BlockHeader{Timestamp: time.Now().Unix()}}
	_, err = tc.ProcessBlockTransactions(body, haveTime)
	assert.Equal(t, process.ErrMissingTransaction, err)
}

func TestShardProcessor_ProcessBlockCompleteWithOkTxsShouldExecuteThemAndNotRevertAccntState(t *testing.T) {
	t.Parallel()

	dataPool := commonMock.NewPoolsHolderMock()
	requestTransaction := func(txHashes [][]byte) {}

	//we will have a block that will have 3 tx hashes
	//all txs will be in datapool and none of them will return err when processed
	//so, tx processor will return nil on processing tx
	txHash1 := []byte("tx hash 1")
	txHash2 := []byte("tx hash 2")
	txHash3 := []byte("tx hash 3")

	body := block.Block{
		TxHashes: [][]byte{txHash1, txHash2, txHash3},
		Header:   &block.BlockHeader{Timestamp: time.Now().Unix()},
	}

	//put the existing tx inside datapool
	dataPool.Transactions().AddData(txHash1, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash1},
	}, 0, "0")
	dataPool.Transactions().AddData(txHash2, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash2},
	}, 0, "0")
	dataPool.Transactions().AddData(txHash3, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash3},
	}, 0, "0")

	tx1ExecutionResult := 0
	tx2ExecutionResult := 0
	tx3ExecutionResult := 0

	accounts := &commonMock.AccountsStub{
		RevertToSnapshotCalled: func(snapshot int) error {
			assert.Fail(t, "revert should have not been called")
			return nil
		},
		JournalLenCalled: func() int {
			return 0
		},
	}

	kapps := &commonMock.AccountsStub{
		RevertToSnapshotCalled: func(snapshot int) error {
			assert.Fail(t, "revert should have not been called")
			return nil
		},
		JournalLenCalled: func() int {
			return 0
		},
	}

	forkController := newForkController()

	preprocessor, _ := preprocess.NewTransactionPreprocessor(
		dataPool.Transactions(),
		initStore(),
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{
			ProcessTransactionCalled: func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
				//execution, in this context, means moving the tx number to its corresponding execution result variable
				if bytes.Equal(transaction.RawData.Sender, txHash1) {
					tx1ExecutionResult = 1
				}
				if bytes.Equal(transaction.RawData.Sender, txHash2) {
					tx2ExecutionResult = 2
				}
				if bytes.Equal(transaction.RawData.Sender, txHash3) {
					tx3ExecutionResult = 3
				}

				return nil
			},
		},
		accounts,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		forkController,
	)

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		accounts,
		kapps,
		&commonMock.RequestHandlerStub{},
		preprocessor,
		&commonMock.FeeAccumulatorStub{},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		forkController,
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	// tc.RequestBlockTransactions(&body)
	haveTime := func() time.Duration {
		return time.Second
	}
	processResult, err := tc.ProcessBlockTransactions(&body, haveTime)
	assert.Nil(t, err)
	assert.Equal(t, 1, tx1ExecutionResult)
	assert.Equal(t, 2, tx2ExecutionResult)
	assert.Equal(t, 3, tx3ExecutionResult)

	expectedSize := int64(len(body.TxHashes) * 13) // 3 txs * 13 bytes
	assert.Equal(t, len(body.TxHashes), processResult.Length())
	assert.Equal(t, expectedSize, processResult.Size())
}

func TestShardProcessor_ProcessBlockInvalidFees_ShouldFail(t *testing.T) {
	t.Parallel()

	dataPool := commonMock.NewPoolsHolderMock()
	requestTransaction := func(txHashes [][]byte) {}

	txHash1 := []byte("tx hash 1")
	//put the existing tx inside datapool
	dataPool.Transactions().AddData(txHash1, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash1, BandwidthFee: 1, KAppFee: 1},
	}, 0, "0")

	accounts := &commonMock.AccountsStub{}
	kapps := &commonMock.AccountsStub{}

	forkController := newForkController()
	preprocessor, _ := preprocess.NewTransactionPreprocessor(
		dataPool.Transactions(),
		initStore(),
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{
			ProcessTransactionCalled: func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
				return nil
			},
		},
		accounts,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		forkController,
	)

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		accounts,
		kapps,
		&commonMock.RequestHandlerStub{},
		preprocessor,
		&commonMock.FeeAccumulatorStub{
			GetAccumulatedTxFeesCalled: func() int64 {
				return 1
			},
			GetAccumulatedKAppFeesCalled: func() int64 {
				return 1
			},
		},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		forkController,
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	// tc.RequestBlockTransactions(&body)
	haveTime := func() time.Duration {
		return time.Second
	}

	t.Run("Should fail due to invalid bandwith fees", func(t *testing.T) {
		body := block.Block{
			TxHashes: [][]byte{txHash1},
			Header: &block.BlockHeader{
				Timestamp: time.Now().Unix(),
				TxFees:    2,
				KAppFees:  1,
			},
		}

		processResult, err := tc.ProcessBlockTransactions(&body, haveTime)
		assert.Equal(t, process.ErrInvalidTXFees, err)
		assert.Nil(t, processResult)
	})

	t.Run("Should fail due to invalid kapp fees", func(t *testing.T) {
		body := block.Block{
			TxHashes: [][]byte{txHash1},
			Header: &block.BlockHeader{
				Timestamp: time.Now().Unix(),
				TxFees:    1,
				KAppFees:  2,
			},
		}

		processResult, err := tc.ProcessBlockTransactions(&body, haveTime)
		assert.Equal(t, process.ErrInvalidKAppsFees, err)
		assert.Nil(t, processResult)
	})

}

func TestShardProcessor_ProcessBlockInvalidNumberOfBlockTxs_ShouldFail(t *testing.T) {
	t.Parallel()

	dataPool := commonMock.NewPoolsHolderMock()
	requestTransaction := func(txHashes [][]byte) {}

	txHash1 := []byte("tx hash 1")
	txHash2 := []byte("tx hash 2")
	//put the existing tx inside datapool
	dataPool.Transactions().AddData(txHash1, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash1},
	}, 0, "0")
	dataPool.Transactions().AddData(txHash2, &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{Sender: txHash2},
	}, 0, "0")

	accounts := &commonMock.AccountsStub{}
	kapps := &commonMock.AccountsStub{}

	forkController := newForkController()
	preprocessor, _ := preprocess.NewTransactionPreprocessor(
		dataPool.Transactions(),
		initStore(),
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		&mock.TxProcessorMock{
			PreProcessTransactionCalled: func(transaction *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
				if bytes.Equal(transaction.RawData.Sender, txHash1) {
					return nil, nil, nil
				}

				return nil, nil, process.ErrInvalidTXFees
			},
			ProcessTransactionCalled: func(blk *block.Block, txHash []byte, transaction *transaction.Transaction) error {
				return nil
			},
		},
		accounts,
		&commonMock.AccountsStub{},
		&commonMock.AccountsStub{},
		requestTransaction,
		&commonMock.FeeHandlerStub{},
		createMockPubkeyConverter(),
		forkController,
	)

	tc, err := coordinator.NewTransactionCoordinator(
		&commonMock.HasherMock{},
		&commonMock.MarshalizerMock{},
		accounts,
		kapps,
		&commonMock.RequestHandlerStub{},
		preprocessor,
		&commonMock.FeeAccumulatorStub{
			GetAccumulatedTxFeesCalled: func() int64 {
				return 1
			},
			GetAccumulatedKAppFeesCalled: func() int64 {
				return 1
			},
		},
		&commonMock.FeeHandlerStub{},
		createTxLogsProcessor(),
		forkController,
		&commonMock.SCProcessorMock{},
	)
	assert.Nil(t, err)
	assert.NotNil(t, tc)

	body := block.Block{
		TxHashes: [][]byte{txHash1, txHash2},
		Header: &block.BlockHeader{
			Timestamp: time.Now().Unix(),
			TxFees:    1,
			KAppFees:  1,
		},
	}

	// tc.RequestBlockTransactions(&body)
	haveTime := func() time.Duration {
		return time.Second
	}
	processResult, err := tc.ProcessBlockTransactions(&body, haveTime)
	assert.Equal(t, process.ErrInvalidNumberOfBlockTxs, err)
	assert.Nil(t, processResult)
}
