package indexer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/klever-io/klever-go/indexer/mock"
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
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Hasher:                   &mock.HasherMock{},
		Marshalizer:              &mock.MarshalizerMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		IsInImportDBMode:         false,
		EnabledIndexes: map[string]struct{}{
			blockIndex: {}, txIndex: {}, accountsIndex: {}, proposalsIndex: {}, accountsHistoryIndex: {}, peersAccountsIndex: {}, kdaPoolsIndex: {},
		},
		KappsDB:        &mock.KappsDBMock{},
		KAppController: &mock.KappsControllerMock{},
		AccountsDB:     &mock.AccountsStub{},
	}
}

func TestNewElasticProcessorWithKibana(t *testing.T) {
	args := createMockElasticProcessorArgs()
	args.UseKibana = true
	args.DBClient = &mock.DatabaseWriterStub{}

	elasticProc, err := NewElasticProcessor(args)
	require.NoError(t, err)
	require.NotNil(t, elasticProc)
}

func TestElasticProcessor_RemoveHeader(t *testing.T) {
	called := false

	args := createMockElasticProcessorArgs()
	args.DBClient = &mock.DatabaseWriterStub{
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
	dbWriter := &mock.DatabaseWriterStub{
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

	dbWriter := &mock.DatabaseWriterStub{
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

func TestElasticseachSaveTransactions_ShouldReturnErr(t *testing.T) {
	localErr := errors.New("localErr")
	arguments := createMockElasticProcessorArgs()
	dbWriter := &mock.DatabaseWriterStub{
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
	err := elasticDatabase.SaveTransactions(header, &indexer.Pool{Txs: txPool, Logs: logsPool})
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
	err = esDatabase.SaveTransactions(body, &indexer.Pool{Txs: txPool})
	require.Nil(t, err)

	fmt.Println(hex.EncodeToString(txHash1))

	// header.TimeStamp = 1234
	txPool = map[string]nodeData.TransactionHandler{
		string(txHash1): tx1,
		string(txHash2): tx2,
	}

	body.TxHashes = append(body.TxHashes, txHash3)

	// update
	err = esDatabase.SaveTransactions(body, &indexer.Pool{Txs: txPool})
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

		err := esDatabase.SaveTransactions(header, &indexer.Pool{Txs: txsPool})
		require.Nil(t, err)
	}
}
