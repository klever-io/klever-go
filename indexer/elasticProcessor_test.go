package indexer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	imock "github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

func newTestElasticSearchDatabase(elasticsearchWriter DatabaseClientHandler, arguments ArgElasticProcessor) *elasticProcessor {
	argsLogsAndEventsProc := logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: arguments.AddressPubkeyConverter,
		Marshalizer:     arguments.Marshalizer,
		Hasher:          arguments.Hasher,
	}

	logsAndEventsProc, _ := logsevents.NewLogsAndEventsProcessor(argsLogsAndEventsProc)

	return &elasticProcessor{
		txDatabaseProcessor: newTxDatabaseProcessor(
			arguments.Hasher,
			arguments.Marshalizer,
			arguments.AddressPubkeyConverter,
			arguments.ValidatorPubkeyConverter,
			arguments.IsInImportDBMode,
		),
		elasticClient: elasticsearchWriter,
		parser: &dataParser{
			marshalizer: arguments.Marshalizer,
			hasher:      arguments.Hasher,
		},
		enabledIndexes:    arguments.EnabledIndexes,
		accountsDB:        arguments.AccountsDB,
		kappsDB:           arguments.KappsDB,
		logsAndEventsProc: logsAndEventsProc,
	}
}

func createMockElasticProcessorArgs() ArgElasticProcessor {
	return ArgElasticProcessor{
		AddressPubkeyConverter: &mock.PubkeyConverterStub{
			DecodeCalled: func(humanReadable string) ([]byte, error) {
				// Simple mock that accepts klv1 addresses and returns the address as bytes
				if len(humanReadable) > 4 && humanReadable[:4] == "klv1" {
					// Return a deterministic byte array based on the address
					return []byte(humanReadable), nil
				}
				return hex.DecodeString(humanReadable)
			},
			EncodeCalled: func(pkBytes []byte) string {
				// Return as-is for our test addresses
				return string(pkBytes)
			},
			LenCalled: func() int {
				return 32
			},
		},
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Hasher:                   &mock.HasherMock{},
		Marshalizer:              &mock.MarshalizerMock{},
		DBClient:                 &imock.DatabaseWriterStub{},
		IsInImportDBMode:         false,
		EnabledIndexes: map[string]struct{}{
			blockIndex: {}, txIndex: {}, accountsIndex: {}, proposalsIndex: {}, accountsHistoryIndex: {}, peersAccountsIndex: {}, kdaPoolsIndex: {},
		},
		KappsDB:              &mock.KappsDBMock{},
		KAppController:       &mock.KappsControllerMock{},
		AccountsDB:           &mock.AccountsStub{},
		CacheExpirationTime:  time.Hour,
		CacheCleanUpInterval: 2 * time.Hour,
	}
}

func TestNewElasticProcessorWithKibana(t *testing.T) {
	args := createMockElasticProcessorArgs()
	args.UseKibana = true
	args.DBClient = &imock.DatabaseWriterStub{}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)
	require.NotNil(t, elasticProc)
}

func TestElasticProcessor_RemoveHeader(t *testing.T) {
	called := false

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoBulkRemoveCalled: func(index string, hashes []string) error {
			called = true
			return nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RemoveHeader(&dataBlock.Block{})
	require.Nil(t, err)
	require.True(t, called)
}

func TestElasticseachDatabaseSaveHeader_RequestError(t *testing.T) {
	localErr := errors.New("localErr")
	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1},
	}
	signer := []byte("signer")
	arguments := createMockElasticProcessorArgs()
	dbWriter := &imock.DatabaseWriterStub{
		DoRequestCalled: func(req *esapi.IndexRequest) error {
			return localErr
		},
	}
	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)

	err := elasticDatabase.SaveHeader(header, signer, 1, []string{})
	require.Equal(t, localErr, err)
}

func TestElasticseachDatabaseSaveHeader_CheckRequestBody(t *testing.T) {
	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1},
	}
	signer := []byte("signer")
	arguments := createMockElasticProcessorArgs()

	dbWriter := &imock.DatabaseWriterStub{
		DoRequestCalled: func(req *esapi.IndexRequest) error {
			require.Equal(t, blockIndex, req.Index)

			var block data.Block
			blockBytes, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(blockBytes, &block)
			require.Equal(t, header.Header.Nonce, block.Nonce)
			require.Equal(t, hex.EncodeToString(signer), block.ProducerSignature)

			return nil
		},
	}

	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	err := elasticDatabase.SaveHeader(header, signer, 1, []string{})
	require.Nil(t, err)
}

// Fallback: when Prepared is nil, SaveTransactions must run prep itself.
func TestElasticProcessor_SaveTransactions_FallbackPrepWhenPreparedNil(t *testing.T) {
	var bulkInvocations int32
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(_ *bytes.Buffer, _ string) error {
			atomic.AddInt32(&bulkInvocations, 1)
			return nil
		},
	}

	contract := transaction.TransferContract{
		ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		Amount:    45,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType,
		[]byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
	require.NoError(t, err)

	header := &dataBlock.Block{
		Header:   &dataBlock.BlockHeader{Nonce: 7, Timestamp: 100},
		TxHashes: [][]byte{[]byte("h1")},
	}
	pool := &indexer.Pool{
		Txs:  map[string]nodeData.TransactionHandler{"h1": tx},
		Logs: []*nodeData.LogData{},
	}

	ep := newTestElasticSearchDatabase(dbWriter, createMockElasticProcessorArgs())
	require.NoError(t, ep.SaveTransactions(header, pool, nil))
	require.Greater(t, atomic.LoadInt32(&bulkInvocations), int32(0),
		"fallback must run the full indexing pipeline when prepared is nil")
}

func TestElasticseachSaveTransactions_ShouldReturnErr(t *testing.T) {
	localErr := errors.New("localErr")
	arguments := createMockElasticProcessorArgs()
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, index string) error {
			return localErr
		},
	}

	contract := transaction.TransferContract{
		ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		Amount:    45,
	}

	txHash1 := []byte("txHash1")
	tx1, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
	txHash2 := []byte("txHash2")
	tx2, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
	txHash3 := []byte("txHash3")
	tx3, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))

	header := &dataBlock.Block{
		Header:   &dataBlock.BlockHeader{},
		TxHashes: [][]byte{txHash1, txHash2, txHash3},
	}

	txPool := map[string]nodeData.TransactionHandler{
		string(txHash1): tx1,
		string(txHash2): tx2,
		string(txHash3): tx3,
	}

	logsPool := []*nodeData.LogData{}

	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	err := elasticDatabase.SaveTransactions(header, &indexer.Pool{Txs: txPool, Logs: logsPool}, nil)
	require.Equal(t, localErr, err)
}

func TestUpdateTransaction(t *testing.T) {
	t.Skip("test must run only if you have an elasticsearch server on address http://localhost:9200")

	indexTemplates, indexPolicies := getIndexTemplateAndPolicies()
	dbClient, _ := NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})

	args := ArgElasticProcessor{
		DBClient:                 dbClient,
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		IndexTemplates:           indexTemplates,
		IndexPolicies:            indexPolicies,
		IsInImportDBMode:         false,
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		AccountsDB:               &mock.AccountsStub{},
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(96),
		EnabledIndexes: map[string]struct{}{
			"transactions": {},
		},
	}

	esDatabase, err := NewElasticProcessor(args)
	require.Nil(t, err)

	contract := transaction.TransferContract{
		ToAddress: []byte("sender_address1"),
		Amount:    int64(10),
	}
	tx1, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, []byte("receiver_address1"))
	txHash1 := []byte("tx1")

	contract2 := transaction.TransferContract{
		ToAddress: []byte("sender_address2"),
		Amount:    int64(20),
	}
	tx2, _ := createTransactionMock(&contract2, transaction.TXContract_TransferContractType, []byte("receiver_address2"))
	txHash2 := []byte("tx2")

	contract3 := transaction.TransferContract{
		ToAddress: []byte("sender_address3"),
		Amount:    int64(30),
	}
	tx3, _ := createTransactionMock(&contract3, transaction.TXContract_TransferContractType, []byte("receiver_address3"))
	txHash3 := []byte("tx3")

	body := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{},
		TxHashes: [][]byte{
			[]byte("tx1"),
			[]byte("tx2"),
			[]byte("tx3"),
		},
		ProducerSignature: []byte("producer signature"),
	}

	txPool := map[string]nodeData.TransactionHandler{
		string(txHash1): tx1,
		string(txHash2): tx2,
		string(txHash3): tx3,
	}

	// insert
	err = esDatabase.SaveTransactions(body, &indexer.Pool{Txs: txPool}, nil)
	require.Nil(t, err)

	fmt.Println(hex.EncodeToString(txHash1))

	// header.TimeStamp = 1234
	txPool = map[string]nodeData.TransactionHandler{
		string(txHash1): tx1,
		string(txHash2): tx2,
	}

	body.TxHashes = append(body.TxHashes, txHash3)

	// update
	err = esDatabase.SaveTransactions(body, &indexer.Pool{Txs: txPool}, nil)
	require.Nil(t, err)
}

func TestDoBulkRequestLimit(t *testing.T) {
	t.Skip("test must run only if you have an elasticsearch server on address http://localhost:9200")

	indexTemplates, indexPolicies := getIndexTemplateAndPolicies()
	dbClient, _ := NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})

	args := ArgElasticProcessor{
		DBClient:                 dbClient,
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   &mock.PubkeyConverterMock{},
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		IndexTemplates:           indexTemplates,
		IndexPolicies:            indexPolicies,
		AccountsDB:               &mock.AccountsStub{},
		KappsDB:                  &mock.KappsDBMock{},
		KAppController:           &mock.KappsControllerMock{},
	}

	esDatabase, err := NewElasticProcessor(args)
	require.Nil(t, err)
	//Generate transaction and hashes
	numTransactions := 1
	dataSize := 900001
	for i := 0; i < 1000; i++ {
		txs, hashes := generateTransactions(numTransactions, dataSize)

		txsPool := make(map[string]nodeData.TransactionHandler)
		for j := 0; j < numTransactions; j++ {
			txsPool[hashes[j]] = &txs[j]
		}

		header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1}}

		err := esDatabase.SaveTransactions(header, &indexer.Pool{Txs: txsPool}, nil)
		require.Nil(t, err)
	}
}

func TestRevertAccountBalances_IndexNotEnabled(t *testing.T) {
	t.Parallel()

	args := createMockElasticProcessorArgs()
	// Disable accounts index
	args.EnabledIndexes = map[string]struct{}{
		blockIndex: {},
		txIndex:    {},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(12345)
	require.NoError(t, err)
}

func TestRevertAccountBalances_OnlyAccountsIndexNeeded(t *testing.T) {
	t.Parallel()

	searchCalled := false

	args := createMockElasticProcessorArgs()
	// Enable only accounts index (accounts-history index no longer required)
	args.EnabledIndexes = map[string]struct{}{
		accountsIndex: {},
	}

	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			require.Equal(t, accountsIndex, index)
			// Return empty results - no accounts to revert
			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{},
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(12345)
	require.NoError(t, err)
	require.True(t, searchCalled)
}

func TestRevertAccountBalances_SearchError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("search error")
	searchCalled := false
	timestamp := int64(12345)

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			return nil, expectedErr
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.True(t, searchCalled)
	require.Equal(t, expectedErr, err)
}

func TestRevertAccountBalances_InvalidResponseFormat(t *testing.T) {
	t.Parallel()

	searchCalled := false
	timestamp := int64(12345)

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			// Return invalid response structure
			return templates.Object{
				"invalid": "response",
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.True(t, searchCalled)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response format")
}

func TestRevertAccountBalances_NoAccountsToRevert(t *testing.T) {
	t.Parallel()

	searchCalled := false
	timestamp := int64(12345)

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			// Return empty hits
			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{},
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.True(t, searchCalled)
	require.NoError(t, err) // Should not error when no accounts to revert
}

func TestRevertAccountBalances_SingleAccountCreatedInBlock(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	address := "klv1test123456789012345678901234567890123456789012345678901234"
	searchCalled := false
	updateCalled := false

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			// Query should be against accounts index, not accounts-history
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address": address,
							},
						},
					},
				},
			}, nil
		},
		DoUpdateCalled: func(index string, id string, body *bytes.Buffer) error {
			updateCalled = true
			require.Equal(t, accountsIndex, index)
			require.Equal(t, address, id)

			// Verify the update sets balance to 0 (account not found in trie)
			var updateDoc map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &updateDoc)
			require.NoError(t, err)

			script, ok := updateDoc["script"].(map[string]interface{})
			require.True(t, ok)

			params, ok := script["params"].(map[string]interface{})
			require.True(t, ok)

			require.Equal(t, float64(0), params["balance"])
			require.Equal(t, float64(0), params["frozenBalance"])

			return nil
		},
	}

	// Mock AccountsDB to return error (account not found)
	args.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(container []byte) (state.AccountHandler, error) {
			return nil, fmt.Errorf("account not found")
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.NoError(t, err)
	require.True(t, searchCalled)
	require.True(t, updateCalled)
}

func TestRevertAccountBalances_AccountWithPreviousState(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	address := "klv1previousstate0123456789012345678901234567890123456789012"
	balanceInTrie := int64(1500)
	frozenBalanceInTrie := int64(750)

	searchCalled := false
	updateCalled := false

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			// Return one account history entry for this block
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address":       address,
								"balance":       float64(2000),
								"frozenBalance": float64(1000),
								"timestamp":     float64(timestamp),
							},
						},
					},
				},
			}, nil
		},
		DoUpdateCalled: func(index string, id string, body *bytes.Buffer) error {
			updateCalled = true
			require.Equal(t, accountsIndex, index)
			require.Equal(t, address, id)

			// Verify the update restores balance from trie
			var updateDoc map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &updateDoc)
			require.NoError(t, err)

			script, ok := updateDoc["script"].(map[string]interface{})
			require.True(t, ok)

			params, ok := script["params"].(map[string]interface{})
			require.True(t, ok)

			require.Equal(t, float64(balanceInTrie), params["balance"])
			require.Equal(t, float64(frozenBalanceInTrie), params["frozenBalance"])

			return nil
		},
	}

	// Mock AccountsDB to return an account with specific balance
	args.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(container []byte) (state.AccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balanceInTrie
				},
				GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
					return &kapps.UserKDA{
						FrozenBalance: frozenBalanceInTrie,
					}, nil
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.NoError(t, err)
	require.True(t, searchCalled)
	require.True(t, updateCalled)
}

func TestRevertAccountBalances_MultipleAccounts(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	address1 := "klv1account1000000000000000000000000000000000000000000001"
	address2 := "klv1account2000000000000000000000000000000000000000000002"
	address3 := "klv1account3000000000000000000000000000000000000000000003"

	searchCalled := false
	updateCallCount := 0
	updatedAddresses := make(map[string]bool)

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			searchCalled = true
			// Return all 3 accounts from the current block
			require.Equal(t, accountsIndex, index)

			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address":       address1,
								"balance":       float64(1000),
								"frozenBalance": float64(100),
								"timestamp":     float64(timestamp),
							},
						},
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address":       address2,
								"balance":       float64(2000),
								"frozenBalance": float64(200),
								"timestamp":     float64(timestamp),
							},
						},
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address":       address3,
								"balance":       float64(3000),
								"frozenBalance": float64(300),
								"timestamp":     float64(timestamp),
							},
						},
					},
				},
			}, nil
		},
		DoUpdateCalled: func(index string, id string, body *bytes.Buffer) error {
			updateCallCount++
			updatedAddresses[id] = true
			return nil
		},
	}

	// Mock AccountsDB to return accounts with balances
	args.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(container []byte) (state.AccountHandler, error) {
			// Return different balances for each account
			balance := int64(500)
			frozenBalance := int64(50)
			return &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
					return &kapps.UserKDA{
						FrozenBalance: frozenBalance,
					}, nil
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.NoError(t, err)
	require.True(t, searchCalled)
	require.Equal(t, 3, updateCallCount)
	require.True(t, updatedAddresses[address1])
	require.True(t, updatedAddresses[address2])
	require.True(t, updatedAddresses[address3])
}

func TestRevertAccountBalances_UpdateError(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	address := "klv1testerror00000000000000000000000000000000000000000789"
	expectedErr := errors.New("update error")

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			// Verify search query contains correct timestamp
			var query map[string]interface{}
			err := json.Unmarshal(body.Bytes(), &query)
			require.NoError(t, err)

			queryObj := query["query"].(map[string]interface{})
			termObj := queryObj["term"].(map[string]interface{})
			require.Equal(t, float64(timestamp*1000), termObj["updatedAt"])

			// Return a simple account entry
			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{
						map[string]interface{}{
							"_source": map[string]interface{}{
								"address":       address,
								"balance":       float64(1000),
								"frozenBalance": float64(100),
								"timestamp":     float64(timestamp),
							},
						},
					},
				},
			}, nil
		},
		DoUpdateCalled: func(index string, id string, body *bytes.Buffer) error {
			return expectedErr
		},
	}

	// Mock AccountsDB
	args.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(container []byte) (state.AccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 500
				},
				GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
					return &kapps.UserKDA{
						FrozenBalance: 50,
					}, nil
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	// Should not return error even if individual update fails (it logs warning)
	err = elasticProc.RevertAccountBalances(timestamp)
	require.NoError(t, err)
}

func TestRevertAccountBalances_AccountHistorySearchError(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	expectedErr := errors.New("history search error")

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			return nil, expectedErr
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RevertAccountBalances(timestamp)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func TestRevertAccountBalances_InvalidAccountHistoryData(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoSearchCalled: func(index string, body *bytes.Buffer) (templates.Object, error) {
			// Return account with missing address field
			return templates.Object{
				"hits": map[string]interface{}{
					"hits": []interface{}{
						map[string]interface{}{
							"_source": map[string]interface{}{
								// Missing address
								"balance":       float64(1000),
								"frozenBalance": float64(100),
								"timestamp":     float64(timestamp),
							},
						},
					},
				},
			}, nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	// Should handle gracefully when address is missing
	err = elasticProc.RevertAccountBalances(timestamp)
	require.NoError(t, err)
}

func TestRemoveAccountsHistory_IndexNotEnabled(t *testing.T) {
	t.Parallel()

	args := createMockElasticProcessorArgs()
	// Disable accounts history index
	args.EnabledIndexes = map[string]struct{}{
		blockIndex:    {},
		txIndex:       {},
		accountsIndex: {},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(12345)
	require.NoError(t, err)
}

func TestRemoveAccountsHistory_Success(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	bulkRemoveCalled := false
	var capturedIndex string
	var capturedTimestamp time.Duration

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoBulkRemoveByTimestampCalled: func(index string, ts time.Duration) error {
			bulkRemoveCalled = true
			capturedIndex = index
			capturedTimestamp = ts
			return nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(timestamp)
	require.NoError(t, err)
	require.True(t, bulkRemoveCalled)
	require.Equal(t, accountsHistoryIndex, capturedIndex)
	require.Equal(t, time.Duration(timestamp*1000), capturedTimestamp)
}

func TestRemoveAccountsHistory_BulkRemoveError(t *testing.T) {
	t.Parallel()

	timestamp := int64(12345)
	expectedErr := errors.New("bulk remove error")
	bulkRemoveCalled := false

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoBulkRemoveByTimestampCalled: func(index string, ts time.Duration) error {
			bulkRemoveCalled = true
			require.Equal(t, accountsHistoryIndex, index)
			require.Equal(t, time.Duration(timestamp*1000), ts)
			return expectedErr
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(timestamp)
	require.True(t, bulkRemoveCalled)
	require.Equal(t, expectedErr, err)
}

func TestRemoveAccountsHistory_DifferentTimestamps(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		timestamp int64
	}{
		{
			name:      "Timestamp zero",
			timestamp: 0,
		},
		{
			name:      "Positive timestamp",
			timestamp: 123456789,
		},
		{
			name:      "Large timestamp",
			timestamp: 9999999999,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedTimestamp time.Duration

			args := createMockElasticProcessorArgs()
			args.DBClient = &imock.DatabaseWriterStub{
				DoBulkRemoveByTimestampCalled: func(index string, ts time.Duration) error {
					capturedTimestamp = ts
					return nil
				},
			}

			elasticProc, err := NewElasticProcessor(args)
			require.NoError(t, err)

			err = elasticProc.RemoveAccountsHistory(tc.timestamp)
			require.NoError(t, err)
			require.Equal(t, time.Duration(tc.timestamp*1000), capturedTimestamp)
		})
	}
}

func TestRemoveAccountsHistory_VerifyIndexParameter(t *testing.T) {
	t.Parallel()

	timestamp := int64(54321)
	callCount := 0
	var capturedIndexes []string

	args := createMockElasticProcessorArgs()
	args.DBClient = &imock.DatabaseWriterStub{
		DoBulkRemoveByTimestampCalled: func(index string, ts time.Duration) error {
			callCount++
			capturedIndexes = append(capturedIndexes, index)
			return nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	// Call multiple times
	err = elasticProc.RemoveAccountsHistory(timestamp)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(timestamp + 1)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(timestamp + 2)
	require.NoError(t, err)

	require.Equal(t, 3, callCount)
	// Verify all calls use the correct index
	for _, idx := range capturedIndexes {
		require.Equal(t, accountsHistoryIndex, idx)
	}
}

func TestRemoveAccountsHistory_RemovesKDAHistoryWhenEnabled(t *testing.T) {
	t.Parallel()

	timestamp := int64(54321)
	var capturedIndexes []string

	args := createMockElasticProcessorArgs()
	args.EnabledIndexes[accountsKDAHistoryIndex] = struct{}{}
	args.DBClient = &imock.DatabaseWriterStub{
		DoBulkRemoveByTimestampCalled: func(index string, ts time.Duration) error {
			require.Equal(t, toMilliseconds(timestamp), ts)
			capturedIndexes = append(capturedIndexes, index)
			return nil
		},
	}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)

	err = elasticProc.RemoveAccountsHistory(timestamp)
	require.NoError(t, err)
	require.Equal(t, []string{accountsHistoryIndex, accountsKDAHistoryIndex}, capturedIndexes)
}

// kdaHistoryRows returns the accountskdahistory rows of a block bulk. The bulk also
// carries accountskda deletes, which have no source line, so it is not read pairwise.
func kdaHistoryRows(t *testing.T, buffSlice *data.BufferSlice) map[string]data.AccountBalanceHistory {
	t.Helper()

	rows := make(map[string]data.AccountBalanceHistory)
	for _, buff := range buffSlice.Buffers() {
		lines := splitNDJSON(buff.Bytes())
		for i := 0; i < len(lines)-1; i++ {
			if !bytes.Contains(lines[i], []byte(`"_index":"accountskdahistory"`)) {
				continue
			}
			row := data.AccountBalanceHistory{}
			require.NoError(t, json.Unmarshal(lines[i+1], &row))
			rows[row.TokenIdentifier] = row
		}
	}

	return rows
}

func TestElasticProcessor_IndexAlteredAccountsWritesKDAHistory(t *testing.T) {
	t.Parallel()

	timestamp := int64(1234567890)
	block := &data.PreparedBlockData{
		Accounts: map[string]*data.AccountInfo{"klv1a": {Address: "klv1a"}},
		AccountsKDA: []*data.AccountKDA{
			{AccountAddress: "klv1a", AssetID: "KFI", Balance: 100, FrozenBalance: 20, UnfrozenBalance: 5},
			{AccountAddress: "klv1a", AssetID: "COLL-1A2B/7", Balance: 0},
		},
	}

	t.Run("enabled index writes one row per address and asset", func(t *testing.T) {
		t.Parallel()

		arguments := createMockElasticProcessorArgs()
		arguments.EnabledIndexes = map[string]struct{}{accountsIndex: {}, accountsKDAHistoryIndex: {}}
		elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)
		require.NoError(t, elasticDatabase.indexAlteredAccounts(timestamp, block, buffSlice))

		body := buffSlice.Buffers()[0].String()
		require.Contains(t, body, `"_index":"accountskdahistory", "_id" : "klv1a-KFI_1234567890"`)
		require.Contains(t, body, `"_index":"accountskdahistory", "_id" : "klv1a-COLL-1A2B/7_1234567890"`)

		rows := kdaHistoryRows(t, buffSlice)
		require.Len(t, rows, 2)
		for _, row := range rows {
			require.Equal(t, "klv1a", row.Address)
			require.Equal(t, toMilliseconds(timestamp), row.Timestamp)
		}
		require.Equal(t, int64(100), rows["KFI"].Balance)
		require.Equal(t, int64(20), rows["KFI"].FrozenBalance)
		require.Equal(t, int64(5), rows["KFI"].UnfrozenBalance)
		require.Contains(t, rows, "COLL-1A2B/7", "a balance that dropped to zero is still a history point")
	})

	t.Run("disabled index writes nothing", func(t *testing.T) {
		t.Parallel()

		arguments := createMockElasticProcessorArgs()
		elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)
		require.NoError(t, elasticDatabase.indexAlteredAccounts(timestamp, block, buffSlice))
		require.Empty(t, kdaHistoryRows(t, buffSlice))
	})
}

func TestCalculateUnfrozenBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		buckets  map[string]*kapps.UserBucket
		expected int64
	}{
		{
			name:     "nil buckets returns zero",
			buckets:  nil,
			expected: 0,
		},
		{
			name:     "empty buckets returns zero",
			buckets:  map[string]*kapps.UserBucket{},
			expected: 0,
		},
		{
			name: "all staked buckets returns zero",
			buckets: map[string]*kapps.UserBucket{
				"bucket1": {Value: 1000, UnstakedEpoch: core.DefaultUnstakedEpoch},
				"bucket2": {Value: 2000, UnstakedEpoch: core.DefaultUnstakedEpoch},
			},
			expected: 0,
		},
		{
			name: "all unstaked buckets returns sum",
			buckets: map[string]*kapps.UserBucket{
				"bucket1": {Value: 1000, UnstakedEpoch: 10},
				"bucket2": {Value: 2000, UnstakedEpoch: 20},
			},
			expected: 3000,
		},
		{
			name: "mixed buckets returns only unstaked sum",
			buckets: map[string]*kapps.UserBucket{
				"staked1":   {Value: 5000, UnstakedEpoch: core.DefaultUnstakedEpoch},
				"unstaked1": {Value: 1000, UnstakedEpoch: 10},
				"staked2":   {Value: 3000, UnstakedEpoch: core.DefaultUnstakedEpoch},
				"unstaked2": {Value: 2000, UnstakedEpoch: 20},
			},
			expected: 3000,
		},
		{
			name: "bucket with zero unstaked epoch is considered unstaked",
			buckets: map[string]*kapps.UserBucket{
				"bucket1": {Value: 500, UnstakedEpoch: 0},
			},
			expected: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := calculateUnfrozenBalance(tt.buckets)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllowanceWithPendingRewards(t *testing.T) {
	t.Parallel()

	t.Run("nil kappsController returns base allowance", func(t *testing.T) {
		t.Parallel()

		args := createMockElasticProcessorArgs()
		args.KAppController = nil
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = nil

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 1000
			},
		}

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
		require.Equal(t, int64(1000), result)
	})

	t.Run("with pending rewards adds to allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 500, nil
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = kappController

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 2000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
		require.Equal(t, int64(2500), result)
	})

	t.Run("zero pending rewards returns base allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 0, nil
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = kappController

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 3000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
		require.Equal(t, int64(3000), result)
	})

	t.Run("error getting pending rewards returns base allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 0, errors.New("some error")
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = kappController

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 4000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
		require.Equal(t, int64(4000), result)
	})
}

func TestConvertPermissions(t *testing.T) {
	t.Parallel()

	args := createMockElasticProcessorArgs()
	ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)

	t.Run("nil permissions returns empty slice", func(t *testing.T) {
		result := convertPermissions(ep.addressPubkeyConverter, nil)
		require.Empty(t, result)
		require.NotNil(t, result)
	})

	t.Run("empty permissions returns empty slice", func(t *testing.T) {
		result := convertPermissions(ep.addressPubkeyConverter, []*state.Permission{})
		require.Empty(t, result)
	})

	t.Run("converts permissions with signers correctly", func(t *testing.T) {
		perms := []*state.Permission{
			{
				ID:             1,
				Type:           state.Permission_Owner,
				PermissionName: "TestPermission",
				Threshold:      2,
				Operations:     []byte{0x01, 0x02},
				Signers: []*state.Key{
					{Address: []byte{0xAA, 0xBB}, Weight: 1},
					{Address: []byte{0xCC, 0xDD}, Weight: 2},
				},
			},
		}

		result := convertPermissions(ep.addressPubkeyConverter, perms)
		require.Len(t, result, 1)
		require.Equal(t, int32(1), result[0].ID)
		require.Equal(t, int32(state.Permission_Owner), result[0].Type)
		require.Equal(t, "TestPermission", result[0].PermissionName)
		require.Equal(t, int64(2), result[0].Threshold)
		require.Equal(t, "0102", result[0].Operations)
		require.Len(t, result[0].Signers, 2)
		require.Equal(t, int64(1), result[0].Signers[0].Weight)
		require.Equal(t, int64(2), result[0].Signers[1].Weight)
	})
}

func TestDispatchAccountEvents(t *testing.T) {
	// Not parallel - modifies global EventQueue and UseEventQueue

	t.Run("does not dispatch when UseEventQueue is false", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		originalEventQueue := EventQueue
		testQueue := make(chan Event, 1)
		EventQueue = testQueue
		UseEventQueue = false
		defer func() {
			UseEventQueue = originalUseEventQueue
			EventQueue = originalEventQueue
		}()

		accountsMap := map[string]*data.AccountInfo{
			"addr1": {Address: "addr1"},
		}

		dispatchAccountEvents(accountsMap)

		// Verify nothing was dispatched
		select {
		case <-testQueue:
			t.Fatal("expected no event to be dispatched when UseEventQueue is false")
		default:
			// Success - no event dispatched
		}
	})

	t.Run("does not dispatch when accountsMap is empty", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		originalEventQueue := EventQueue
		testQueue := make(chan Event, 1)
		EventQueue = testQueue
		UseEventQueue = true
		defer func() {
			UseEventQueue = originalUseEventQueue
			EventQueue = originalEventQueue
		}()

		dispatchAccountEvents(map[string]*data.AccountInfo{})

		// Verify nothing was dispatched
		select {
		case <-testQueue:
			t.Fatal("expected no event to be dispatched when accountsMap is empty")
		default:
			// Success - no event dispatched
		}
	})

	t.Run("dispatches event with correct type and data when enabled", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		originalEventQueue := EventQueue
		testQueue := make(chan Event, 1)
		EventQueue = testQueue
		UseEventQueue = true
		defer func() {
			UseEventQueue = originalUseEventQueue
			EventQueue = originalEventQueue
		}()

		accountsMap := map[string]*data.AccountInfo{
			"addr1": {Address: "addr1", Balance: 1000},
			"addr2": {Address: "addr2", Balance: 2000},
		}

		dispatchAccountEvents(accountsMap)

		// Verify event was dispatched with correct data
		select {
		case event := <-testQueue:
			require.Equal(t, ACCOUNTS, event.EvType)
			receivedMap, ok := event.Message.(map[string]*data.AccountInfo)
			require.True(t, ok, "event message should be map[string]*data.AccountInfo")
			require.Len(t, receivedMap, 2)
			require.Equal(t, "addr1", receivedMap["addr1"].Address)
			require.Equal(t, int64(1000), receivedMap["addr1"].Balance)
			require.Equal(t, "addr2", receivedMap["addr2"].Address)
			require.Equal(t, int64(2000), receivedMap["addr2"].Balance)
		default:
			t.Fatal("expected event to be dispatched")
		}
	})
}

func TestElasticProcessor_SaveHeader_DoesNotDispatchEvents(t *testing.T) {
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = true
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1},
	}
	signer := []byte("signer")
	arguments := createMockElasticProcessorArgs()

	dbWriter := &imock.DatabaseWriterStub{
		DoRequestCalled: func(req *esapi.IndexRequest) error {
			return nil
		},
	}

	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	err := elasticDatabase.SaveHeader(header, signer, 1, []string{})
	require.Nil(t, err)

	require.Len(t, testQueue, 0, "elasticProcessor must not enqueue BLOCKS events; dispatch is owned by eventsProcessor")
}

func TestElasticProcessor_SaveTransactions_DoesNotDispatchEvents(t *testing.T) {
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = true
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	arguments := createMockElasticProcessorArgs()
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, index string) error {
			return nil
		},
	}

	contract := transaction.TransferContract{
		ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		Amount:    45,
	}

	txHash1 := []byte("txHash1")
	tx1, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))

	header := &dataBlock.Block{
		Header:   &dataBlock.BlockHeader{},
		TxHashes: [][]byte{txHash1},
	}

	txPool := map[string]nodeData.TransactionHandler{
		string(txHash1): tx1,
	}

	logsPool := []*nodeData.LogData{}

	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	err := elasticDatabase.SaveTransactions(header, &indexer.Pool{Txs: txPool, Logs: logsPool}, nil)
	require.Nil(t, err)

	require.Len(t, testQueue, 0, "elasticProcessor must not enqueue events; events are dispatched by eventsProcessor")
}

func TestBuildAccountInfo(t *testing.T) {
	t.Parallel()

	t.Run("builds account info successfully", func(t *testing.T) {
		t.Parallel()

		userAccount := &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte {
				return []byte("testaddress12345678901234567890")
			},
			GetNonceCalled: func() uint64 {
				return 42
			},
			GetNameCalled: func() []byte {
				return []byte("TestAccount")
			},
			GetRootHashCalled: func() []byte {
				return []byte{0x01, 0x02, 0x03}
			},
			GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
				return 10000
			},
			GetAllowanceCalled: func() int64 {
				return 500
			},
			GetPermissionsCalled: func() []*state.Permission {
				return nil
			},
			GetCodeHashCalled: func() []byte {
				return []byte{0x0A, 0x0B}
			},
			GetCodeMetadataCalled: func() []byte {
				return []byte{0x0C, 0x0D}
			},
			GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{
					FrozenBalance: 2000,
					Buckets:       map[string]*kapps.UserBucket{},
				}, nil
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = nil // nil so getAllowanceWithPendingRewards returns base allowance

		account := &data.Account{
			UserAccount: userAccount,
			IsSender:    true,
		}

		result, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, account.UserAccount, 1234567890)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, uint64(42), result.Nonce)
		require.Equal(t, "TestAccount", result.Name)
		require.Equal(t, int64(10000), result.Balance)
		require.Equal(t, int64(2000), result.FrozenBalance)
		require.Equal(t, int64(0), result.UnfrozenBalance)
		require.Equal(t, int64(500), result.Allowance)
		require.Equal(t, "0a0b", result.CodeHash)
		require.Equal(t, "0c0d", result.CodeMetadata)
	})

	t.Run("returns error when GetUserKDA fails", func(t *testing.T) {
		t.Parallel()

		userAccount := &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte {
				return []byte("testaddress12345678901234567890")
			},
			GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
				return nil, errors.New("KDA not found")
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)

		account := &data.Account{
			UserAccount: userAccount,
			IsSender:    true,
		}

		result, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, account.UserAccount, 1234567890)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "KDA not found")
	})

	t.Run("includes pending rewards in allowance", func(t *testing.T) {
		t.Parallel()

		userAccount := &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte {
				return []byte("testaddress12345678901234567890")
			},
			GetNonceCalled: func() uint64 {
				return 1
			},
			GetNameCalled: func() []byte {
				return nil
			},
			GetRootHashCalled: func() []byte {
				return nil
			},
			GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
				return 1000
			},
			GetAllowanceCalled: func() int64 {
				return 500
			},
			GetPermissionsCalled: func() []*state.Permission {
				return nil
			},
			GetCodeHashCalled: func() []byte {
				return nil
			},
			GetCodeMetadataCalled: func() []byte {
				return nil
			},
			GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{
					FrozenBalance: 0,
					Buckets:       nil,
				}, nil
			},
		}

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 250, nil
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		args := createMockElasticProcessorArgs()
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		ep.kappsController = kappController

		account := &data.Account{
			UserAccount: userAccount,
			IsSender:    true,
		}

		result, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, account.UserAccount, 1234567890)

		require.NoError(t, err)
		require.NotNil(t, result)
		// Base allowance (500) + pending rewards (250) = 750
		require.Equal(t, int64(750), result.Allowance)
	})
}

func TestElasticProcessor_indexRefresh(t *testing.T) {
	t.Parallel()

	t.Run("live mode forces refresh", func(t *testing.T) {
		t.Parallel()
		args := createMockElasticProcessorArgs()
		args.IsInImportDBMode = false
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		require.Equal(t, "true", ep.indexRefresh())
	})

	t.Run("import-db mode disables force refresh", func(t *testing.T) {
		t.Parallel()
		args := createMockElasticProcessorArgs()
		args.IsInImportDBMode = true
		ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, args)
		require.Equal(t, "false", ep.indexRefresh())
	})
}

func TestElasticProcessor_SaveHeader_RefreshPolicy(t *testing.T) {
	t.Parallel()

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1},
	}
	signer := []byte("signer")

	t.Run("live mode sets refresh true", func(t *testing.T) {
		t.Parallel()
		var captured *esapi.IndexRequest
		args := createMockElasticProcessorArgs()
		args.IsInImportDBMode = false
		args.EnabledIndexes = map[string]struct{}{blockIndex: {}}
		dbWriter := &imock.DatabaseWriterStub{
			DoRequestCalled: func(req *esapi.IndexRequest) error {
				captured = req
				return nil
			},
		}
		ep := newTestElasticSearchDatabase(dbWriter, args)
		require.NoError(t, ep.SaveHeader(header, signer, 1, []string{}))
		require.NotNil(t, captured)
		require.Equal(t, "true", captured.Refresh)
	})

	t.Run("import-db mode sets refresh false", func(t *testing.T) {
		t.Parallel()
		var captured *esapi.IndexRequest
		args := createMockElasticProcessorArgs()
		args.IsInImportDBMode = true
		args.EnabledIndexes = map[string]struct{}{blockIndex: {}}
		dbWriter := &imock.DatabaseWriterStub{
			DoRequestCalled: func(req *esapi.IndexRequest) error {
				captured = req
				return nil
			},
		}
		ep := newTestElasticSearchDatabase(dbWriter, args)
		require.NoError(t, ep.SaveHeader(header, signer, 1, []string{}))
		require.NotNil(t, captured)
		require.Equal(t, "false", captured.Refresh)
	})
}

func decodeAccountsHistoryBulk(t *testing.T, buff *bytes.Buffer) []data.AccountBalanceHistory {
	t.Helper()

	// the bulk body is strict meta/source pairs, so the records are the odd lines
	lines := splitNDJSON(buff.Bytes())
	records := make([]data.AccountBalanceHistory, 0, len(lines)/2)
	for i := 1; i < len(lines); i += 2 {
		record := data.AccountBalanceHistory{}
		require.NoError(t, json.Unmarshal(lines[i], &record))
		records = append(records, record)
	}

	return records
}

// captureAccountsHistory returns a DatabaseWriterStub keeping a copy of every
// accountshistory bulk it receives.
func captureAccountsHistory() (*imock.DatabaseWriterStub, *[]*bytes.Buffer) {
	historyBuffers := make([]*bytes.Buffer, 0)
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, index string) error {
			if index == accountsHistoryIndex {
				historyBuffers = append(historyBuffers, bytes.NewBuffer(buff.Bytes()))
			}
			return nil
		},
	}

	return dbWriter, &historyBuffers
}

func TestElasticProcessor_SaveAccountsHistoryRecordsUnfrozenBalance(t *testing.T) {
	t.Parallel()

	address := "klv1d05ju9jaj6u99zph0ant9jh7gksf"
	userAccount := createTestAccountStub()
	userAccount.AddressBytesCalled = func() []byte { return []byte(address) }
	userAccount.GetBalanceCalled = func(assetID []byte, cdd bool) int64 { return 1000 }
	userAccount.GetUserKDACalled = func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
		return &kapps.UserKDA{
			FrozenBalance: 2000,
			Buckets: map[string]*kapps.UserBucket{
				"stakedBucket":   {Value: 2000, UnstakedEpoch: core.DefaultUnstakedEpoch},
				"unstakedBucket": {Value: 3000, UnstakedEpoch: 12},
			},
		}, nil
	}

	dbWriter, historyBuffers := captureAccountsHistory()

	arguments := createMockElasticProcessorArgs()
	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	elasticDatabase.kappsController = nil

	err := elasticDatabase.SaveAccounts(1234567890, []*data.Account{{UserAccount: userAccount, IsSender: true}})
	require.NoError(t, err)
	require.Len(t, *historyBuffers, 1)

	records := decodeAccountsHistoryBulk(t, (*historyBuffers)[0])
	require.Len(t, records, 1)
	require.Equal(t, address, records[0].Address)
	require.Equal(t, int64(1000), records[0].Balance)
	require.Equal(t, int64(2000), records[0].FrozenBalance)
	require.Equal(t, int64(3000), records[0].UnfrozenBalance)
}

func TestElasticProcessor_SaveAccountsHistoryKeepsTotalValueAcrossUnfreezeCooldown(t *testing.T) {
	t.Parallel()

	// A freeze/unfreeze/withdraw cycle moves the same 4000 between the three
	// balances without ever losing any of it, which is the reason the history
	// record needs the unfrozen field at all.
	stages := []struct {
		name            string
		balance         int64
		frozenBalance   int64
		buckets         map[string]*kapps.UserBucket
		expectedFrozen  int64
		expectedUnfroze int64
	}{
		{
			name:            "after freeze",
			balance:         1000,
			frozenBalance:   3000,
			buckets:         map[string]*kapps.UserBucket{"b1": {Value: 3000, UnstakedEpoch: core.DefaultUnstakedEpoch}},
			expectedFrozen:  3000,
			expectedUnfroze: 0,
		},
		{
			name:            "after unfreeze",
			balance:         1000,
			frozenBalance:   0,
			buckets:         map[string]*kapps.UserBucket{"b1": {Value: 3000, UnstakedEpoch: 12}},
			expectedFrozen:  0,
			expectedUnfroze: 3000,
		},
		{
			name:            "after withdraw",
			balance:         4000,
			frozenBalance:   0,
			buckets:         map[string]*kapps.UserBucket{},
			expectedFrozen:  0,
			expectedUnfroze: 0,
		},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			t.Parallel()

			userAccount := createTestAccountStub()
			userAccount.GetBalanceCalled = func(assetID []byte, cdd bool) int64 { return stage.balance }
			userAccount.GetUserKDACalled = func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{
					FrozenBalance: stage.frozenBalance,
					Buckets:       stage.buckets,
				}, nil
			}

			ep := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, createMockElasticProcessorArgs())
			ep.kappsController = nil // nil so getAllowanceWithPendingRewards returns base allowance

			result, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, userAccount, 1234567890)

			require.NoError(t, err)
			require.Equal(t, stage.balance, result.Balance)
			require.Equal(t, stage.expectedFrozen, result.FrozenBalance)
			require.Equal(t, stage.expectedUnfroze, result.UnfrozenBalance)
		})
	}
}

func TestElasticProcessor_SaveEpochInfo(t *testing.T) {
	t.Parallel()

	klvKey := string(kdautils.ToKDAKey(kdautils.KLVIdentifier, nil))
	kfiKey := string(kdautils.ToKDAKey([]byte(kdautils.KFIIdentifier), nil))

	// mustMarshal encodes with MarshalizerMock, matching createMockElasticProcessorArgs's marshalizer
	// (the one SaveEpochInfo decodes with).
	mustMarshal := func(t *testing.T, obj interface{}) []byte {
		raw, err := (&mock.MarshalizerMock{}).Marshal(obj)
		require.NoError(t, err)
		return raw
	}

	newEpochProcessor := func(t *testing.T, stakingRecords map[string][]byte, capture **esapi.IndexRequest, importDB bool) *elasticProcessor {
		args := createMockElasticProcessorArgs()
		args.IsInImportDBMode = importDB
		args.EnabledIndexes = map[string]struct{}{epochIndex: {}}

		kdaRecords := map[string][]byte{
			klvKey: mustMarshal(t, &kapps.KDAData{CirculatingSupply: 9_000}),
			kfiKey: mustMarshal(t, &kapps.KDAData{CirculatingSupply: 21_000}),
		}
		newApp := func(records map[string][]byte) state.AccountHandler {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(key []byte) ([]byte, error) {
						return records[string(key)], nil
					}}
				},
			}
		}
		apps := map[string]state.AccountHandler{
			string(kapps.KDAKAppAddress):     newApp(kdaRecords),
			string(kapps.StakingKAppAddress): newApp(stakingRecords),
		}
		args.KappsDB = &mock.AccountsStub{LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			app, ok := apps[string(address)]
			require.True(t, ok, "unexpected KApp address")
			return app, nil
		}}

		dbWriter := &imock.DatabaseWriterStub{DoRequestCalled: func(req *esapi.IndexRequest) error {
			*capture = req
			return nil
		}}
		return newTestElasticSearchDatabase(dbWriter, args)
	}

	t.Run("indexes per-asset supply and staking figures", func(t *testing.T) {
		t.Parallel()
		var captured *esapi.IndexRequest
		ep := newEpochProcessor(t, map[string][]byte{
			klvKey: mustMarshal(t, &kapps.StakingData{TotalStaked: 5_000}),
			kfiKey: mustMarshal(t, &kapps.StakingData{TotalStaked: 800}),
		}, &captured, false)

		require.NoError(t, ep.SaveEpochInfo(7, nil))
		require.NotNil(t, captured)
		require.Equal(t, epochIndex, captured.Index)
		require.Equal(t, "7", captured.DocumentID)
		require.Equal(t, "true", captured.Refresh)

		body, err := io.ReadAll(captured.Body)
		require.NoError(t, err)
		doc := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(body, &doc))
		require.EqualValues(t, 9_000, doc["klvCirculationSupply"])
		require.EqualValues(t, 21_000, doc["kfiCirculationSupply"])
		require.EqualValues(t, 5_000, doc["klvTotalStaked"])
		require.EqualValues(t, 800, doc["kfiTotalStaked"])
	})

	t.Run("missing KFI staking record fails loudly", func(t *testing.T) {
		t.Parallel()
		var captured *esapi.IndexRequest
		ep := newEpochProcessor(t, map[string][]byte{
			klvKey: mustMarshal(t, &kapps.StakingData{TotalStaked: 5_000}),
		}, &captured, false)

		require.ErrorIs(t, ep.SaveEpochInfo(7, nil), common.ErrEmptyString)
		require.Nil(t, captured)
	})

	t.Run("import-db mode sets refresh false on epoch index", func(t *testing.T) {
		t.Parallel()
		var captured *esapi.IndexRequest
		ep := newEpochProcessor(t, map[string][]byte{
			klvKey: mustMarshal(t, &kapps.StakingData{TotalStaked: 5_000}),
			kfiKey: mustMarshal(t, &kapps.StakingData{TotalStaked: 800}),
		}, &captured, true)

		require.NoError(t, ep.SaveEpochInfo(7, nil))
		require.NotNil(t, captured)
		require.Equal(t, epochIndex, captured.Index)
		require.Equal(t, "false", captured.Refresh)
	})
}

// accountAtBlock drives an AccountsStub that reports one balance for the first
// LoadAccount call and another for every later call, standing in for block N+1
// committing while block N's indexing job waits in the queue.
func accountAtBlock(address string, firstBalance, laterBalance int64) *mock.AccountsStub {
	calls := int64(0)

	return &mock.AccountsStub{
		LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			balance := laterBalance
			if atomic.AddInt64(&calls, 1) == 1 {
				balance = firstBalance
			}

			account := createTestAccountStub()
			account.AddressBytesCalled = func() []byte { return []byte(address) }
			account.GetBalanceCalled = func(_ []byte, _ bool) int64 { return balance }

			return account, nil
		},
	}
}

// blockDataForAccounts builds a block with one transaction sent by each address.
func blockDataForAccounts(t *testing.T, timestamp int64, addresses ...string) (*dataBlock.Block, *indexer.Pool) {
	t.Helper()

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: timestamp}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{}}
	for i, address := range addresses {
		txHash := fmt.Sprintf("txhash%d", i)
		header.TxHashes = append(header.TxHashes, []byte(txHash))
		pool.Txs[txHash] = &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Nonce:  1,
				Sender: []byte(address),
				Data:   [][]byte{},
			},
		}
	}

	return header, pool
}

// TestElasticProcessor_PrepareBlockDataSnapshotsAccountsAtCommit covers CertiK KLR-68.
// The indexing job is queued by CommitBlock and drained later by a single worker. The
// worker used to resolve balances from the live state adapter, so a block that committed
// in the meantime was recorded under the earlier block's timestamp. Resolving on the
// commit goroutine pins the right state into the payload. A work item is retried forever
// and re-sent unchanged, so every attempt must emit that same snapshot rather than
// re-reading state that has drifted further.
func TestElasticProcessor_PrepareBlockDataSnapshotsAccountsAtCommit(t *testing.T) {
	address := "klv1d05ju9jaj6u99zph0ant9jh7gksf"
	const timestamp = int64(1234567890)

	dbWriter, historyBuffers := captureAccountsHistory()

	arguments := createMockElasticProcessorArgs()
	arguments.AccountsDB = accountAtBlock(address, 100, 70)
	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	elasticDatabase.kappsController = nil

	header, pool := blockDataForAccounts(t, timestamp, address)

	// on the commit goroutine: live state is still block N's committed state
	prepared, err := elasticDatabase.PrepareBlockData(header, pool, nil)
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.Len(t, prepared.Accounts, 1)

	// later, on the worker: live state has moved on to block N+1
	for attempt := 0; attempt < 3; attempt++ {
		require.NoError(t, elasticDatabase.SaveTransactions(header, pool, prepared))
	}

	require.Len(t, *historyBuffers, 3)
	for attempt, buff := range *historyBuffers {
		records := decodeAccountsHistoryBulk(t, buff)
		require.Len(t, records, 1)
		require.Equal(t, address, records[0].Address)
		require.Equal(t, int64(100), records[0].Balance, "attempt %d", attempt)
		require.Equal(t, toMilliseconds(timestamp), records[0].Timestamp)
	}
}

// With the accounts index disabled indexAlteredAccounts discards every document - the
// accountskda ones included - so block commit must pay nothing.
func TestElasticProcessor_PrepareBlockDataSkipsWhenAccountsIndexDisabled(t *testing.T) {
	address := "klv1d05ju9jaj6u99zph0ant9jh7gksf"

	loadCalls := int64(0)
	arguments := createMockElasticProcessorArgs()
	arguments.EnabledIndexes = map[string]struct{}{blockIndex: {}, txIndex: {}, accountsKDAIndex: {}}
	arguments.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			atomic.AddInt64(&loadCalls, 1)
			return nil, errors.New("must not be called")
		},
	}
	elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)

	header, pool := blockDataForAccounts(t, 1234567890, address)

	prepared, err := elasticDatabase.PrepareBlockData(header, pool, nil)
	require.NoError(t, err)
	require.Nil(t, prepared)
	require.Equal(t, int64(0), atomic.LoadInt64(&loadCalls))
}

// eventsProcessor.prepare returns a typed-nil payload for an empty pool, which reaches
// PrepareBlockData as a non-nil any. It must be treated as absent and resolved.
func TestElasticProcessor_PrepareBlockDataResolvesTypedNilPayload(t *testing.T) {
	address := "klv1d05ju9jaj6u99zph0ant9jh7gksf"

	arguments := createMockElasticProcessorArgs()
	arguments.AccountsDB = accountAtBlock(address, 100, 70)
	elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
	elasticDatabase.kappsController = nil

	header, pool := blockDataForAccounts(t, 1234567890, address)

	prepared, err := elasticDatabase.PrepareBlockData(header, pool, (*data.PreparedBlockData)(nil))
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.Len(t, prepared.Accounts, 1)
	require.Equal(t, int64(100), prepared.Accounts[address].Balance)
}

// An account whose state cannot be read at commit is left out and its error returned,
// without dropping the other accounts of the block. The worker must not retry the read:
// by then live state may hold a later block, which is KLR-68 again.
func TestElasticProcessor_PrepareBlockDataSkipsOnlyTheFailingAccount(t *testing.T) {
	good := "klv1d05ju9jaj6u99zph0ant9jh7gksf"
	bad := "klv1unreadableaccount"
	errTrie := errors.New("transient trie error")

	dbWriter, historyBuffers := captureAccountsHistory()

	loadCalls := int64(0)
	arguments := createMockElasticProcessorArgs()
	arguments.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
			atomic.AddInt64(&loadCalls, 1)
			account := createTestAccountStub()
			account.AddressBytesCalled = func() []byte { return addr }
			if string(addr) == bad {
				account.GetUserKDACalled = func(_ []byte, _ []byte, _ bool) (*kapps.UserKDA, error) {
					return nil, errTrie
				}
			}
			return account, nil
		},
	}
	elasticDatabase := newTestElasticSearchDatabase(dbWriter, arguments)
	elasticDatabase.kappsController = nil

	header, pool := blockDataForAccounts(t, 1234567890, good, bad)

	prepared, err := elasticDatabase.PrepareBlockData(header, pool, nil)
	require.ErrorIs(t, err, errTrie)
	require.NotNil(t, prepared)
	require.Len(t, prepared.Accounts, 1)
	require.Contains(t, prepared.Accounts, good)

	require.NoError(t, elasticDatabase.SaveTransactions(header, pool, prepared))
	require.Len(t, *historyBuffers, 1)
	records := decodeAccountsHistoryBulk(t, (*historyBuffers)[0])
	require.Len(t, records, 1)
	require.Equal(t, good, records[0].Address)
	require.Equal(t, int64(2), atomic.LoadInt64(&loadCalls), "the worker must not re-read live state")
}

// With websocket subscribers on, eventsProcessor has already built the documents of the
// accounts it dispatched: they are reused rather than built a second time on the commit
// goroutine, and the websocket's map is left untouched.
func TestElasticProcessor_resolveAlteredAccountsReusesBuiltDocuments(t *testing.T) {
	dispatched := "klv1d05ju9jaj6u99zph0ant9jh7gksf"
	notDispatched := "klv1someaddress"

	builds := int64(0)
	arguments := createMockElasticProcessorArgs()
	arguments.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
			account := createTestAccountStub()
			account.AddressBytesCalled = func() []byte { return addr }
			account.GetUserKDACalled = func(_ []byte, _ []byte, _ bool) (*kapps.UserKDA, error) {
				atomic.AddInt64(&builds, 1)
				return &kapps.UserKDA{}, nil
			}
			return account, nil
		},
	}
	elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
	elasticDatabase.kappsController = nil

	built := map[string]*data.AccountInfo{dispatched: {Address: dispatched, Balance: 999}}
	accountsInfo, _, err := elasticDatabase.resolveAlteredAccounts(
		1234567890,
		map[string][]*data.AlteredAccount{dispatched: {{IsSender: true}}, notDispatched: {{IsSender: true}}},
		built,
	)
	require.NoError(t, err)
	require.Len(t, accountsInfo, 2)
	require.Same(t, built[dispatched], accountsInfo[dispatched])
	require.Equal(t, int64(1), atomic.LoadInt64(&builds), "only the account not dispatched is built")
	require.Len(t, built, 1)
}

// getKDAInfo reads the KDA and staking KApp accounts for every altered entry, and those
// are the same two addresses for a whole block. Now that resolution happens on the
// commit goroutine, loading them once per block rather than once per entry is what keeps
// the cost off the critical path.
func TestElasticProcessor_resolveAlteredAccountsLoadsKAppAccountsOncePerBlock(t *testing.T) {
	address := "klv1d05ju9jaj6u99zph0ant9jh7gksf"

	mustMarshal := func(obj interface{}) []byte {
		raw, err := (&mock.MarshalizerMock{}).Marshal(obj)
		require.NoError(t, err)
		return raw
	}

	assetIDs := []string{"AAA-1111", "BBB-2222", "CCC-3333"}
	kdaRecords := map[string][]byte{}
	for _, assetID := range assetIDs {
		key := string(kdautils.ToKDAKey([]byte(assetID), nil))
		kdaRecords[key] = mustMarshal(&kapps.KDAData{Name: []byte(assetID), Precision: 6})
	}

	newApp := func(records map[string][]byte) state.AccountHandler {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(key []byte) ([]byte, error) {
					return records[string(key)], nil
				}}
			},
		}
	}
	apps := map[string]state.AccountHandler{
		string(kapps.KDAKAppAddress):     newApp(kdaRecords),
		string(kapps.StakingKAppAddress): newApp(map[string][]byte{}),
	}

	kappLoads := int64(0)
	arguments := createMockElasticProcessorArgs()
	arguments.EnabledIndexes[accountsKDAIndex] = struct{}{}
	arguments.KappsDB = &mock.AccountsStub{LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
		atomic.AddInt64(&kappLoads, 1)
		app, ok := apps[string(addr)]
		require.True(t, ok, "unexpected KApp address")
		return app, nil
	}}
	arguments.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			account := createTestAccountStub()
			account.AddressBytesCalled = func() []byte { return []byte(address) }
			account.GetUserKDACalled = func(_ []byte, _ []byte, _ bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{Balance: 42}, nil
			}
			return account, nil
		},
	}

	elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
	elasticDatabase.kappsController = nil
	elasticDatabase.marshalizer = &mock.MarshalizerMock{}

	altered := make([]*data.AlteredAccount, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		altered = append(altered, &data.AlteredAccount{
			IsKDAOperation:  true,
			TokenIdentifier: assetID,
		})
	}

	accountsInfo, accountsKDA, err := elasticDatabase.resolveAlteredAccounts(
		1234567890,
		map[string][]*data.AlteredAccount{address: altered},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, accountsInfo, 1)
	require.Len(t, accountsKDA, len(assetIDs))

	// the KDA and staking accounts, once each - not once per altered entry
	require.Equal(t, int64(2), atomic.LoadInt64(&kappLoads))
}

// TestElasticProcessor_resolveAlteredAccountsSkipPaths pins the routing of the paths that
// are logged and skipped: a failed load must not drop the whole block, and an account
// whose KApp state cannot be read is still indexed - only its KDA entries are skipped.
func TestElasticProcessor_resolveAlteredAccountsSkipPaths(t *testing.T) {
	errLoad := errors.New("cannot load account")

	tests := []struct {
		name          string
		address       string
		accountsDBErr error
		numAccounts   int
	}{
		{
			name:        "address that cannot be decoded is skipped",
			address:     "not-an-address",
			numAccounts: 0,
		},
		{
			name:          "user account that cannot be loaded is skipped",
			address:       "klv1someaddress",
			accountsDBErr: errLoad,
			numAccounts:   0,
		},
		{
			name:        "kapp account that cannot be loaded is still indexed, without its KDA",
			address:     hex.EncodeToString(kapps.KDAKAppAddress),
			numAccounts: 1,
		},
		{
			name:        "KDA entry is skipped when the kapp lookups cannot be loaded",
			address:     "klv1someaddress",
			numAccounts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := createMockElasticProcessorArgs()
			arguments.EnabledIndexes[accountsKDAIndex] = struct{}{}
			arguments.AccountsDB = &mock.AccountsStub{
				LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
					if tt.accountsDBErr != nil {
						return nil, tt.accountsDBErr
					}

					account := createTestAccountStub()
					account.AddressBytesCalled = func() []byte { return addr }
					return account, nil
				},
			}
			// both the KApp account of a KApp address and the KDA lookups come from here
			arguments.KappsDB = &mock.AccountsStub{
				LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
					return nil, errLoad
				},
			}

			elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
			elasticDatabase.kappsController = nil
			elasticDatabase.marshalizer = &mock.MarshalizerMock{}

			accountsInfo, accountsKDA, err := elasticDatabase.resolveAlteredAccounts(
				1234567890,
				map[string][]*data.AlteredAccount{
					tt.address: {{IsKDAOperation: true, TokenIdentifier: "AAA-1111"}},
				},
				nil,
			)
			require.NoError(t, err)
			require.Len(t, accountsInfo, tt.numAccounts)
			require.Empty(t, accountsKDA)
		})
	}
}

// A failed KApp load is memoised for the block too, so later KDA entries neither retry
// it nor log it again.
func TestElasticProcessor_resolveAlteredAccountsDoesNotRetryFailedKAppLoad(t *testing.T) {
	kappLoads := int64(0)
	arguments := createMockElasticProcessorArgs()
	arguments.EnabledIndexes[accountsKDAIndex] = struct{}{}
	arguments.AccountsDB = &mock.AccountsStub{
		LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
			account := createTestAccountStub()
			account.AddressBytesCalled = func() []byte { return addr }
			return account, nil
		},
	}
	arguments.KappsDB = &mock.AccountsStub{
		LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			atomic.AddInt64(&kappLoads, 1)
			return nil, errors.New("cannot load account")
		},
	}

	elasticDatabase := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
	elasticDatabase.kappsController = nil

	kdaEntries := []*data.AlteredAccount{
		{IsKDAOperation: true, TokenIdentifier: "AAA-1111"},
		{IsKDAOperation: true, TokenIdentifier: "BBB-2222"},
	}
	accountsInfo, accountsKDA, err := elasticDatabase.resolveAlteredAccounts(
		1234567890,
		map[string][]*data.AlteredAccount{"klv1someaddress1": kdaEntries, "klv1someaddress2": kdaEntries},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, accountsInfo, 2)
	require.Empty(t, accountsKDA)
	require.Equal(t, int64(1), atomic.LoadInt64(&kappLoads))
}

type proposalKappWithTrie struct {
	state.KAppAccountHandler
	trie nodeData.Trie
}

func (p *proposalKappWithTrie) DataTrie() nodeData.Trie {
	return p.trie
}

type proposalVotersFixture struct {
	marshalizer marshal.Marshalizer
	adb         *state.AccountsDB
	storage     map[string][]byte
}

func newProposalVotersFixture(t *testing.T) *proposalVotersFixture {
	t.Helper()

	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10, Shards: 1})
	require.NoError(t, err)
	persist, err := memorydb.NewlruDB(100000)
	require.NoError(t, err)
	unit, err := storageUnit.NewStorageUnit(cache, persist)
	require.NoError(t, err)

	marshalizer := marshal.NewProtoMarshalizer()
	hasher := &sha256.Sha256{}
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(unit)
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hasher, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hasher, marshalizer, factory.NewKAppAccountCreator(), core.Normal)
	require.NoError(t, err)

	return &proposalVotersFixture{marshalizer: marshalizer, adb: adb, storage: make(map[string][]byte)}
}

func (f *proposalVotersFixture) put(t *testing.T, key []byte, msg any) {
	t.Helper()

	value, ok := msg.([]byte)
	if !ok {
		var err error
		value, err = f.marshalizer.Marshal(msg)
		require.NoError(t, err)
	}
	f.storage[string(key)] = value
}

func (f *proposalVotersFixture) putProposal(t *testing.T, proposalID uint64, proposal *kapps.ProposalData) {
	t.Helper()

	if proposal.Proposer == nil {
		proposal.Proposer = voterAddress(900)
	}
	f.put(t, kdautils.ToProposalKey(proposalID), proposal)
}

func (f *proposalVotersFixture) putVoter(t *testing.T, proposalID uint64, address []byte, detail *kapps.ProposalData_VoteDetail) {
	t.Helper()

	f.put(t, kdautils.ToProposalVoterKey(proposalID, hex.EncodeToString(address)), detail)
}

func (f *proposalVotersFixture) save(t *testing.T) state.KAppAccountHandler {
	t.Helper()

	f.put(t, kdautils.ProposalControllerKey, &kapps.ProposalController{})

	acnt, err := f.adb.LoadAccount(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	app, ok := acnt.(state.KAppAccountHandler)
	require.True(t, ok)
	for key, value := range f.storage {
		require.NoError(t, app.SetStorage([]byte(key), value))
	}
	require.NoError(t, f.adb.SaveAccount(app))

	return app
}

func (f *proposalVotersFixture) commit(t *testing.T) state.KAppAccountHandler {
	t.Helper()

	f.save(t)
	_, err := f.adb.Commit()
	require.NoError(t, err)

	reloaded, err := f.adb.LoadAccount(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	return reloaded.(state.KAppAccountHandler)
}

func (f *proposalVotersFixture) processor(app state.KAppAccountHandler) *elasticProcessor {
	arguments := createMockElasticProcessorArgs()
	arguments.KappsDB = &mock.AccountsStub{LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
		return app, nil
	}}
	processor := newTestElasticSearchDatabase(&imock.DatabaseWriterStub{}, arguments)
	processor.marshalizer = f.marshalizer

	return processor
}

func voterAddress(index int) []byte {
	address := make([]byte, 32)
	address[0] = byte(index >> 16)
	address[1] = byte(index >> 8)
	address[2] = byte(index)

	return address
}

func TestElasticProcessor_getProposalFillsVotersFromVoterKeys(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{Proposer: voterAddress(900), Votes: map[int32]int64{0: 30, 1: 20}})
	fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Type: kapps.ProposalData_VoteDetail_Yes, Amount: 30, Timestamp: 111})
	fixture.putVoter(t, 1, voterAddress(2), &kapps.ProposalData_VoteDetail{Type: kapps.ProposalData_VoteDetail_No, Amount: 20, Timestamp: 222})
	app := fixture.commit(t)

	proposal, err := fixture.processor(app).getProposal(app, 1)
	require.NoError(t, err)
	require.Len(t, proposal.Voters, 2)
	require.Equal(t, map[int32]int64{0: 30, 1: 20}, proposal.Votes)

	byAddress := make(map[string]*data.VotersInfo)
	for _, voter := range proposal.Voters {
		byAddress[voter.Address] = voter
	}
	yes := byAddress[string(voterAddress(1))]
	require.NotNil(t, yes)
	require.Equal(t, int32(kapps.ProposalData_VoteDetail_Yes), yes.Type)
	require.Equal(t, int64(30), yes.Amount)
	require.Equal(t, time.Duration(111*1000), yes.Timestamp)
	no := byAddress[string(voterAddress(2))]
	require.NotNil(t, no)
	require.Equal(t, int32(kapps.ProposalData_VoteDetail_No), no.Type)
	require.Equal(t, int64(20), no.Amount)
	require.Equal(t, time.Duration(222*1000), no.Timestamp)
}

func TestElasticProcessor_getProposalKeepsInlineVotersWithoutWalkingTheTrie(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{Voters: map[string]*kapps.ProposalData_VoteDetail{
		hex.EncodeToString(voterAddress(7)): {Type: kapps.ProposalData_VoteDetail_Yes, Amount: 5, Timestamp: 9},
	}})
	fixture.putVoter(t, 1, voterAddress(8), &kapps.ProposalData_VoteDetail{Amount: 99})
	app := fixture.commit(t)

	walked := false
	wrapped := &proposalKappWithTrie{KAppAccountHandler: app, trie: &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(_ []byte) (*nodeData.TrieIteratorChannels, error) {
			walked = true
			return nodeData.NewCompletedTrieIteratorChannels(), nil
		},
	}}

	proposal, err := fixture.processor(app).getProposal(wrapped, 1)
	require.NoError(t, err)
	require.False(t, walked)
	require.Len(t, proposal.Voters, 1)
	require.Equal(t, string(voterAddress(7)), proposal.Voters[0].Address)
	require.Equal(t, int64(5), proposal.Voters[0].Amount)
}

func TestElasticProcessor_getProposalVotersExcludeOtherProposals(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	fixture.putProposal(t, 12, &kapps.ProposalData{})
	fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Amount: 1})
	fixture.putVoter(t, 12, voterAddress(2), &kapps.ProposalData_VoteDetail{Amount: 2})
	fixture.putVoter(t, 12, voterAddress(3), &kapps.ProposalData_VoteDetail{Amount: 3})
	app := fixture.commit(t)
	processor := fixture.processor(app)

	first, err := processor.getProposal(app, 1)
	require.NoError(t, err)
	require.Len(t, first.Voters, 1)
	require.Equal(t, string(voterAddress(1)), first.Voters[0].Address)

	twelfth, err := processor.getProposal(app, 12)
	require.NoError(t, err)
	require.Len(t, twelfth.Voters, 2)
}

func TestElasticProcessor_getProposalReturnsErrorWhenVoterWalkFails(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	app := fixture.commit(t)
	expectedErr := errors.New("walk failed")
	processor := fixture.processor(app)

	t.Run("walk cannot start", func(t *testing.T) {
		wrapped := &proposalKappWithTrie{KAppAccountHandler: app, trie: &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(_ []byte) (*nodeData.TrieIteratorChannels, error) {
				return nil, expectedErr
			},
		}}

		_, err := processor.getProposal(wrapped, 1)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("walk is interrupted", func(t *testing.T) {
		wrapped := &proposalKappWithTrie{KAppAccountHandler: app, trie: &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(_ []byte) (*nodeData.TrieIteratorChannels, error) {
				return nodeData.NewFailedTrieIteratorChannels(expectedErr), nil
			},
		}}

		_, err := processor.getProposal(wrapped, 1)
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestElasticProcessor_getProposalReturnsErrorOnMalformedVoterRecord(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Amount: 1})
	fixture.put(t, kdautils.ToProposalVoterKey(1, hex.EncodeToString(voterAddress(2))), []byte{0xff, 0xff, 0xff})
	app := fixture.commit(t)

	_, err := fixture.processor(app).getProposal(app, 1)
	require.Error(t, err)
}

type proposalVoterRead struct {
	value []byte
	err   error
}

type proposalTrackerWithReads struct {
	state.DataTrieTracker
	reads map[string]proposalVoterRead
}

func (p *proposalTrackerWithReads) RetrieveValue(key []byte) ([]byte, error) {
	if read, ok := p.reads[string(key)]; ok {
		return read.value, read.err
	}

	return p.DataTrieTracker.RetrieveValue(key)
}

type proposalKappWithTracker struct {
	state.KAppAccountHandler
	tracker state.DataTrieTracker
}

func (p *proposalKappWithTracker) DataTrieTracker() state.DataTrieTracker {
	return p.tracker
}

func TestElasticProcessor_getProposalReturnsErrorWhenAVoterReadFails(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Amount: 1})
	fixture.putVoter(t, 1, voterAddress(2), &kapps.ProposalData_VoteDetail{Amount: 2})
	app := fixture.commit(t)

	expectedErr := errors.New("voter read failed")
	wrapped := &proposalKappWithTracker{KAppAccountHandler: app, tracker: &proposalTrackerWithReads{
		DataTrieTracker: app.DataTrieTracker(),
		reads: map[string]proposalVoterRead{
			string(kdautils.ToProposalVoterKey(1, hex.EncodeToString(voterAddress(2)))): {err: expectedErr},
		},
	}}

	_, err := fixture.processor(app).getProposal(wrapped, 1)
	require.ErrorIs(t, err, expectedErr)
}

func TestElasticProcessor_getProposalSkipsAVoterRemovedAfterTheWalk(t *testing.T) {
	t.Parallel()

	removedKey := kdautils.ToProposalVoterKey(1, hex.EncodeToString(voterAddress(2)))

	newApp := func(t *testing.T) (*proposalVotersFixture, state.KAppAccountHandler) {
		fixture := newProposalVotersFixture(t)
		fixture.putProposal(t, 1, &kapps.ProposalData{})
		fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Amount: 1})
		fixture.putVoter(t, 1, voterAddress(2), &kapps.ProposalData_VoteDetail{Amount: 2})

		return fixture, fixture.commit(t)
	}

	requireOnlyTheFirstVoter := func(t *testing.T, proposal *data.Proposal, err error) {
		require.NoError(t, err)
		require.Len(t, proposal.Voters, 1)
		require.Equal(t, string(voterAddress(1)), proposal.Voters[0].Address)
	}

	reads := map[string]proposalVoterRead{
		"absent from the data trie":           {},
		"an empty value":                      {value: []byte{}},
		"no data trie":                        {err: common.ErrNilTrie},
		"a value shorter than its identifier": {err: common.ErrNegativeValue},
	}

	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			fixture, app := newApp(t)
			wrapped := &proposalKappWithTracker{KAppAccountHandler: app, tracker: &proposalTrackerWithReads{
				DataTrieTracker: app.DataTrieTracker(),
				reads:           map[string]proposalVoterRead{string(removedKey): read},
			}}

			proposal, err := fixture.processor(app).getProposal(wrapped, 1)
			requireOnlyTheFirstVoter(t, proposal, err)
		})
	}

	t.Run("a delete still in the dirty cache", func(t *testing.T) {
		fixture, app := newApp(t)
		require.NoError(t, app.SetStorage(removedKey, nil))

		_, readErr := app.DataTrieTracker().RetrieveValue(removedKey)
		require.ErrorIs(t, readErr, common.ErrNegativeValue)

		proposal, err := fixture.processor(app).getProposal(app, 1)
		requireOnlyTheFirstVoter(t, proposal, err)
	})
}

func TestElasticProcessor_getProposalReadsVotersOnlyFromACommittedAccount(t *testing.T) {
	t.Parallel()

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	fixture.putVoter(t, 1, voterAddress(1), &kapps.ProposalData_VoteDetail{Amount: 1})
	uncommitted := fixture.save(t)

	_, err := fixture.processor(uncommitted).getProposal(uncommitted, 1)
	require.Error(t, err)

	committed := fixture.commit(t)
	proposal, err := fixture.processor(committed).getProposal(committed, 1)
	require.NoError(t, err)
	require.Len(t, proposal.Voters, 1)
}

func TestElasticProcessor_getProposalKeepsHeaviestVotersAboveTheIndexedLimit(t *testing.T) {
	t.Parallel()

	extra := 5
	total := 10000 + extra

	fixture := newProposalVotersFixture(t)
	fixture.putProposal(t, 1, &kapps.ProposalData{})
	for i := 0; i < total; i++ {
		amount := int64(1000 + i)
		if i < extra {
			amount = 1
		}
		fixture.putVoter(t, 1, voterAddress(i), &kapps.ProposalData_VoteDetail{Amount: amount})
	}
	app := fixture.commit(t)

	proposal, err := fixture.processor(app).getProposal(app, 1)
	require.NoError(t, err)
	require.Len(t, proposal.Voters, 10000)
	for _, voter := range proposal.Voters {
		require.Greater(t, voter.Amount, int64(1))
	}
}

func TestCapProposalVotersIsStableWhenAmountsTie(t *testing.T) {
	t.Parallel()

	total := maxIndexedProposalVoters + 3
	build := func() map[string]*kapps.ProposalData_VoteDetail {
		voters := make(map[string]*kapps.ProposalData_VoteDetail, total)
		for i := 0; i < total; i++ {
			voters[hex.EncodeToString(voterAddress(i))] = &kapps.ProposalData_VoteDetail{Amount: 7}
		}
		return voters
	}

	first := capProposalVoters(build())
	second := capProposalVoters(build())
	require.Len(t, first, maxIndexedProposalVoters)
	require.Equal(t, first, second)

	for i := 0; i < maxIndexedProposalVoters; i++ {
		require.Contains(t, first, hex.EncodeToString(voterAddress(i)))
	}
	for i := maxIndexedProposalVoters; i < total; i++ {
		require.NotContains(t, first, hex.EncodeToString(voterAddress(i)))
	}
}
