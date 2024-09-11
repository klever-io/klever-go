package indexer

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	imock "github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func NewDataIndexerArguments() ArgDataIndexer {
	return ArgDataIndexer{
		Marshalizer:        &mock.MarshalizerMock{},
		NodesCoordinator:   &mock.NodesCoordinatorMock{},
		EpochStartNotifier: &mock.EpochStartNotifierStub{},
		DataDispatcher:     &imock.DispatcherMock{},
		ElasticProcessor:   &imock.ElasticProcessorStub{},
	}
}

func TestDataIndexer_NewIndexerWithNilNodesCoordinatorShouldErr(t *testing.T) {
	arguments := NewDataIndexerArguments()
	arguments.NodesCoordinator = nil
	ei, err := NewDataIndexer(arguments)

	require.Nil(t, ei)
	require.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestDataIndexer_NewIndexerWithNilDataDispatcherShouldErr(t *testing.T) {
	arguments := NewDataIndexerArguments()
	arguments.DataDispatcher = nil
	ei, err := NewDataIndexer(arguments)

	require.Nil(t, ei)
	require.Equal(t, ErrNilDataDispatcher, err)
}

func TestDataIndexer_NewIndexerWithNilElasticProcessorShouldErr(t *testing.T) {
	arguments := NewDataIndexerArguments()
	arguments.ElasticProcessor = nil
	ei, err := NewDataIndexer(arguments)

	require.Nil(t, ei)
	require.Equal(t, ErrNilElasticProcessor, err)
}

func TestDataIndexer_NewIndexerWithNilMarshalizerShouldErr(t *testing.T) {
	arguments := NewDataIndexerArguments()
	arguments.Marshalizer = nil
	ei, err := NewDataIndexer(arguments)

	require.Nil(t, ei)
	require.Equal(t, common.ErrNilMarshalizer, err)
}

func TestDataIndexer_NewIndexerWithNilEpochStartNotifierShouldErr(t *testing.T) {
	arguments := NewDataIndexerArguments()
	arguments.EpochStartNotifier = nil
	ei, err := NewDataIndexer(arguments)

	require.Nil(t, ei)
	require.Equal(t, common.ErrNilEpochStartNotifier, err)
}

func TestDataIndexer_NewIndexerWithCorrectParamsShouldWork(t *testing.T) {
	arguments := NewDataIndexerArguments()

	ei, err := NewDataIndexer(arguments)

	require.Nil(t, err)
	require.False(t, check.IfNil(ei))
	require.False(t, ei.IsNilIndexer())
}

func TestDataIndexer_SaveBlock(t *testing.T) {
	called := false

	arguments := NewDataIndexerArguments()
	arguments.DataDispatcher = &imock.DispatcherMock{
		AddCalled: func(item workItems.WorkItemHandler) {
			called = true
		},
	}
	ei, _ := NewDataIndexer(arguments)

	args := &indexer.ArgsSaveBlockData{
		Header:     &dataBlock.Block{},
		HeaderHash: []byte("hash"),
	}
	ei.SaveBlock(args)
	require.True(t, called)
}

func TestDataIndexer(t *testing.T) {
	t.Skip("this is not a short test")

	testCreateIndexer(t)
}

// nolint
func testCreateIndexer(t *testing.T) {
	indexTemplates, indexPolicies := getIndexTemplateAndPolicies()

	dispatcher, _ := NewDataDispatcher(100)
	dbClient, _ := NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "",
		Password:  "",
	})

	elasticIndexer, _ := NewElasticProcessor(ArgElasticProcessor{
		IndexTemplates:           indexTemplates,
		IndexPolicies:            indexPolicies,
		Marshalizer:              &marshal.JSONMarshalizer{},
		Hasher:                   sha256.Sha256{},
		AddressPubkeyConverter:   &mock.PubkeyConverterMock{},
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 dbClient,
		CacheExpirationTime:      time.Hour,
		CacheCleanUpInterval:     2 * time.Hour,
	})

	di, err := NewDataIndexer(ArgDataIndexer{
		Marshalizer:        &marshal.JSONMarshalizer{},
		EpochStartNotifier: &mock.EpochStartNotifierStub{},
		DataDispatcher:     dispatcher,
		ElasticProcessor:   elasticIndexer,
	})
	if err != nil {
		fmt.Println(err)
		t.Fail()
	}

	// Generate transaction and hashes
	numTransactions := 10
	dataSize := 1000
	signer := []byte("signature")

	for i := 0; i < 100; i++ {
		txs, hashes := generateTransactions(numTransactions, dataSize)

		header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: uint64(i)}}

		txsPool := make(map[string]data.TransactionHandler)
		for j := 0; j < numTransactions; j++ {
			txsPool[hashes[j]] = &txs[j]
		}

		args := &indexer.ArgsSaveBlockData{
			HeaderHash:       []byte("hash"),
			Header:           header,
			Signer:           signer,
			TransactionsPool: &indexer.Pool{Txs: txsPool},
		}
		di.SaveBlock(args)
	}

	time.Sleep(100 * time.Second)
}

// nolint
func generateTransactions(numTxs int, datFieldSize int) ([]transaction.Transaction, []string) {
	txs := make([]transaction.Transaction, 0)
	hashes := make([]string, 0)

	randomByteArray := make([]byte, datFieldSize)
	_, _ = rand.Read(randomByteArray)

	for i := 0; i < numTxs; i++ {

		contract := transaction.TransferContract{
			ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
			Amount:    int64(i),
		}
		tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
		tx.RawData.Data = [][]byte{randomByteArray}

		// hash
		hashes = append(hashes, fmt.Sprintf("%032x", i))

		txs = append(txs, *tx.Clone())
	}

	return txs, hashes
}

func createTransactionMock(contract protoreflect.ProtoMessage, txType transaction.TXContract_ContractType, sender []byte) (*transaction.Transaction, error) {
	var tx transaction.Transaction

	var signatures [][]byte
	tx.Signature = append(signatures, []byte("txSigner"))

	tx.RawData = &transaction.Transaction_Raw{
		Sender: sender,
	}

	err := tx.PushContract(txType, contract)

	return &tx, err
}

// nolint
func getIndexTemplateAndPolicies() (map[string]*bytes.Buffer, map[string]*bytes.Buffer) {
	indexTemplates := make(map[string]*bytes.Buffer)
	indexPolicies := make(map[string]*bytes.Buffer)

	template := &bytes.Buffer{}
	_ = tools.LoadJSONFile(template, "./testdata/opendistro.json")
	indexTemplates["opendistro"] = template

	_ = tools.LoadJSONFile(template, "./testdata/blocks.json")
	indexTemplates["blocks"] = template
	policy := &bytes.Buffer{}
	_ = tools.LoadJSONFile(template, "./testdata/blocks_policy.json")
	indexPolicies["blocks_policy"] = policy

	return indexTemplates, indexPolicies
}
