package indexer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	imock "github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/indexer/templates/noKibana"
	"github.com/stretchr/testify/require"
)

// contractAddressForTest builds a 32-byte address core.IsSmartContractAddress accepts.
func contractAddressForTest(tail byte) []byte {
	address := make([]byte, 32)
	copy(address[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	address[31] = tail

	return address
}

// hexEncodingArgs are the processor arguments with an address converter whose output is
// safe to look for in a JSON body. The default test converter returns the raw bytes as a
// string, which a contract address (leading zero bytes) turns into escaped control
// characters.
func hexEncodingArgs() ArgElasticProcessor {
	args := createMockElasticProcessorArgs()
	args.AddressPubkeyConverter = &mock.PubkeyConverterStub{
		EncodeCalled: func(pkBytes []byte) string { return hex.EncodeToString(pkBytes) },
		DecodeCalled: hex.DecodeString,
		LenCalled:    func() int { return 32 },
	}

	return args
}

// TestSaveTransactions_IndexesTheContractsATransactionTookPartIn is the end-to-end check for
// the field issue 434 asks for: the contract addresses come out of the logs and land in the
// bulk request, on the transaction document, both in the upsert body and in the update
// script's parameters. The bulk body is what Elasticsearch receives, so that is what is read.
func TestSaveTransactions_IndexesTheContractsATransactionTookPartIn(t *testing.T) {
	var sent bytes.Buffer
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, _ string) error {
			sent.Write(buff.Bytes())
			return nil
		},
	}

	invoked, inner := contractAddressForTest(1), contractAddressForTest(2)
	tx, err := createTransactionHandlerMock(
		&transaction.TransferContract{ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"), Amount: 45},
		transaction.TXContract_TransferContractType,
		[]byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"),
	)
	require.NoError(t, err)

	header := &dataBlock.Block{
		Header:   &dataBlock.BlockHeader{Nonce: 7, Timestamp: 100},
		TxHashes: [][]byte{[]byte("h1")},
	}
	pool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{"h1": tx},
		Logs: []*nodeData.LogData{{
			TxHash: "h1",
			LogHandler: &transaction.Log{Address: invoked, Events: []*transaction.Event{
				{Address: inner, Identifier: []byte("swapTokensFixedInput")},
				{Address: invoked, Identifier: []byte(core.CompletedTxEventIdentifier)},
			}},
		}},
	}

	ep := newTestElasticSearchDatabase(dbWriter, hexEncodingArgs())
	require.NoError(t, ep.SaveTransactions(header, pool, nil))

	want := []interface{}{hex.EncodeToString(invoked), hex.EncodeToString(inner)}
	item := transactionBulkItem(t, sent.Bytes(), hex.EncodeToString([]byte("h1")))

	// Two paths, both checked on their own: the upsert body is what a new document is
	// created from, the script parameters are all an existing document receives. A count of
	// occurrences would not tell them apart, since the per-contract update item carries a
	// second copy of the upsert body.
	require.Equal(t, want, item["upsert"].(map[string]interface{})["scAddresses"],
		"a new document must get the field from the upsert body")
	require.Equal(t, want, item["script"].(map[string]interface{})["params"].(map[string]interface{})["scAddresses"],
		"an existing document must get the field from the script parameters")
}

// transactionBulkItem returns the decoded payload of the first bulk item that updates the
// transaction document txHash: bulk bodies are NDJSON, an action line followed by its
// payload line, and the transaction's own item is the first one addressed to its id.
func transactionBulkItem(t *testing.T, bulk []byte, txHash string) map[string]interface{} {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(bulk), []byte("\n"))
	for i := 0; i+1 < len(lines); i += 2 {
		var action map[string]map[string]interface{}
		require.NoError(t, json.Unmarshal(lines[i], &action), "action line %d must be JSON", i)
		if action["update"]["_id"] != txHash {
			continue
		}

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(lines[i+1], &payload), "payload line %d must be JSON", i+1)

		return payload
	}

	require.FailNow(t, "no bulk item updates the transaction", "id %s in:\n%s", txHash, bulk)
	return nil
}

// TestSaveTransactions_LeavesTheFieldOffTransactionsWithoutContracts is the other half: a
// transaction with no contract in its logs must not carry an empty array, and the script
// parameters must carry null so the guarded script leaves an existing document alone.
func TestSaveTransactions_LeavesTheFieldOffTransactionsWithoutContracts(t *testing.T) {
	var sent bytes.Buffer
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, _ string) error {
			sent.Write(buff.Bytes())
			return nil
		},
	}

	tx, err := createTransactionHandlerMock(
		&transaction.TransferContract{ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"), Amount: 45},
		transaction.TXContract_TransferContractType,
		[]byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"),
	)
	require.NoError(t, err)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 7, Timestamp: 100}, TxHashes: [][]byte{[]byte("h1")}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"h1": tx}, Logs: []*nodeData.LogData{}}

	ep := newTestElasticSearchDatabase(dbWriter, hexEncodingArgs())
	require.NoError(t, ep.SaveTransactions(header, pool, nil))

	require.NotContains(t, sent.String(), `"scAddresses":[`)
	require.Contains(t, sent.String(), `"scAddresses":null`, "the script parameter must be null, not absent and not empty")
}

// TestNewElasticProcessor_PutsTheAddedPropertiesOnTheTransactionsIndex covers start-up: the
// template only reaches indices created after it, so the processor must put the added
// properties onto the live transactions index itself, every time it starts, rewrite the
// stored template so later indices carry them too, and refuse to start when either fails.
// A processor that started anyway would write the field into an index that types it
// dynamically, and the filter written for the keyword type would miss it.
func TestNewElasticProcessor_PutsTheAddedPropertiesOnTheTransactionsIndex(t *testing.T) {
	t.Run("checks and updates the mapping on the transactions alias", func(t *testing.T) {
		var gotIndex string
		var gotProperties templates.Object
		args := createMockElasticProcessorArgs()
		args.DBClient = &imock.DatabaseWriterStub{
			CheckAndUpdateMappingCalled: func(index string, properties templates.Object) error {
				gotIndex, gotProperties = index, properties
				return nil
			},
		}

		_, err := NewElasticProcessor(args)
		require.NoError(t, err)

		require.Equal(t, txIndex, gotIndex)
		properties, ok := gotProperties["properties"].(templates.Object)
		require.True(t, ok, "the body must be a mapping update, {\"properties\": ...}")
		require.Equal(t, templates.Object{"type": "keyword"}, properties["scAddresses"])
	})

	t.Run("rewrites the stored transactions template", func(t *testing.T) {
		var gotTemplates []string
		args := createMockElasticProcessorArgs()
		args.IndexTemplates = map[string]*bytes.Buffer{txIndex: noKibana.Transactions.ToBuffer(), blockIndex: noKibana.Blocks.ToBuffer()}
		args.DBClient = &imock.DatabaseWriterStub{
			PutTemplateCalled: func(name string, template *bytes.Buffer) error {
				gotTemplates = append(gotTemplates, name)
				require.True(t, bytes.Contains(template.Bytes(), []byte(`"scAddresses":{"type":"keyword"}`)), "template: %s", template.String())
				return nil
			},
		}

		_, err := NewElasticProcessor(args)
		require.NoError(t, err)
		require.Equal(t, []string{txIndex}, gotTemplates, "only the transactions template is rewritten unconditionally")
	})

	t.Run("a failed mapping update stops start-up", func(t *testing.T) {
		mappingErr := errors.New("mapping rejected")
		args := createMockElasticProcessorArgs()
		args.DBClient = &imock.DatabaseWriterStub{
			CheckAndUpdateMappingCalled: func(string, templates.Object) error { return mappingErr },
		}

		_, err := NewElasticProcessor(args)
		require.ErrorIs(t, err, mappingErr)
	})

	t.Run("a failed template write stops start-up", func(t *testing.T) {
		templateErr := errors.New("template rejected")
		args := createMockElasticProcessorArgs()
		args.IndexTemplates = map[string]*bytes.Buffer{txIndex: noKibana.Transactions.ToBuffer()}
		args.DBClient = &imock.DatabaseWriterStub{
			PutTemplateCalled: func(string, *bytes.Buffer) error { return templateErr },
		}

		_, err := NewElasticProcessor(args)
		require.ErrorIs(t, err, templateErr)
	})
}

// TestSaveTransactions_WritesTheFieldOnThePreparedPath covers the production path of every
// websocket-enabled node: the transactions arrive already prepared, and the field must be
// derived on those structs before they are serialized, exactly as on the fallback path.
func TestSaveTransactions_WritesTheFieldOnThePreparedPath(t *testing.T) {
	var sent bytes.Buffer
	dbWriter := &imock.DatabaseWriterStub{
		DoBulkRequestCalled: func(buff *bytes.Buffer, _ string) error {
			sent.Write(buff.Bytes())
			return nil
		},
	}

	invoked := contractAddressForTest(1)
	tx, err := createTransactionHandlerMock(
		&transaction.TransferContract{ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"), Amount: 45},
		transaction.TXContract_TransferContractType,
		[]byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"),
	)
	require.NoError(t, err)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 7, Timestamp: 100}, TxHashes: [][]byte{[]byte("h1")}}
	pool := &indexer.Pool{
		Txs:  map[string]nodeData.TransactionHandler{"h1": tx},
		Logs: []*nodeData.LogData{{TxHash: "h1", LogHandler: &transaction.Log{Address: invoked}}},
	}

	ep := newTestElasticSearchDatabase(dbWriter, hexEncodingArgs())
	txs, txsMap, altered, err := ep.prepareTransactionsForDatabase(header, pool)
	require.NoError(t, err)

	require.NoError(t, ep.SaveTransactions(header, pool, &data.PreparedBlockData{Txs: txs, TxsMap: txsMap, Altered: altered}))

	item := transactionBulkItem(t, sent.Bytes(), hex.EncodeToString([]byte("h1")))
	require.Equal(t, []interface{}{hex.EncodeToString(invoked)}, item["upsert"].(map[string]interface{})["scAddresses"])
}
