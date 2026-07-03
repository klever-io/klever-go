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
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	imock "github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
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

	newEpochProcessor := func(t *testing.T, stakingRecords map[string][]byte, capture **esapi.IndexRequest) *elasticProcessor {
		args := createMockElasticProcessorArgs()
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
		}, &captured)

		require.NoError(t, ep.SaveEpochInfo(7, nil))
		require.NotNil(t, captured)
		require.Equal(t, epochIndex, captured.Index)
		require.Equal(t, "7", captured.DocumentID)

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
		}, &captured)

		require.ErrorIs(t, ep.SaveEpochInfo(7, nil), common.ErrEmptyString)
		require.Nil(t, captured)
	})
}
