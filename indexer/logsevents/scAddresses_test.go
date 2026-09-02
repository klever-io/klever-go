package logsevents_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/stretchr/testify/require"
)

// scAddress builds a 32-byte address that core.IsSmartContractAddress accepts: the leading
// bytes zero, then the VM type, then a tail that tells contracts apart.
func scAddress(tail byte) []byte {
	address := make([]byte, 32)
	copy(address[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	address[31] = tail

	return address
}

func walletAddress(tail byte) []byte {
	address := make([]byte, 32)
	for i := range address {
		address[i] = 0xAB
	}
	address[31] = tail

	return address
}

// logsExtractor is the one method these tests drive; the constructor returns an unexported
// type, so the tests name the behaviour rather than the type.
type logsExtractor interface {
	ExtractDataFromLogs(pool *indexer.Pool, txs []*data.Transaction, timestamp int64) *data.PreparedLogsResults
}

func newLogsProcessorForTest(t *testing.T) logsExtractor {
	t.Helper()

	proc, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string { return hex.EncodeToString(pkBytes) },
			LenCalled:    func() int { return 32 },
		},
		Marshalizer: &mock.MarshalizerMock{},
		Hasher:      &mock.HasherMock{},
	})
	require.NoError(t, err)

	return proc
}

func logFor(txHash string, logAddress []byte, eventAddresses ...[]byte) *nodeData.LogData {
	events := make([]*transaction.Event, 0, len(eventAddresses))
	for _, address := range eventAddresses {
		events = append(events, &transaction.Event{Address: address, Identifier: []byte("anyEvent")})
	}

	return &nodeData.LogData{
		LogHandler: &transaction.Log{Address: logAddress, Events: events},
		TxHash:     txHash,
	}
}

// TestExtractDataFromLogs_ListsEveryContractThatTookPart is the field issue 434 asks for.
//
// A transaction that invokes contract A, which calls contract B, carries B only in its
// events: contract.parameter.address names A, and B leaves no receipt unless it moved
// value. The log's own address is A; the event addresses are whichever contract emitted
// each event. Both land in scAddresses, distinct and sorted, and nothing else does.
func TestExtractDataFromLogs_ListsEveryContractThatTookPart(t *testing.T) {
	t.Parallel()

	contractA, contractB := scAddress(1), scAddress(2)
	tx := &data.Transaction{Hash: hex.EncodeToString([]byte("h1"))}

	pool := &indexer.Pool{Logs: []*nodeData.LogData{
		logFor("h1", contractA, contractB, contractA, walletAddress(9), make([]byte, 32)),
	}}

	newLogsProcessorForTest(t).ExtractDataFromLogs(pool, []*data.Transaction{tx}, 100)

	require.True(t, tx.HasLogs)
	require.Equal(t, []string{hex.EncodeToString(contractA), hex.EncodeToString(contractB)}, tx.SCAddresses,
		"the invoked contract and the contract that emitted an event, once each, sorted; "+
			"the wallet and the empty system address must not be listed")
}

// TestExtractDataFromLogs_ExcludesTheEmptySystemAddress pins the one address that passes
// core.IsSmartContractAddress without being a contract: the all-zero address is accepted
// through its IsEmptyAddress branch, and burn transfers put it in a log. A count keyed on
// it would credit "the burn address was used" to every such transaction.
func TestExtractDataFromLogs_ExcludesTheEmptySystemAddress(t *testing.T) {
	t.Parallel()

	empty := make([]byte, 32)
	require.True(t, core.IsSmartContractAddress(empty), "the premise: the empty address passes the SC check")

	tx := &data.Transaction{Hash: hex.EncodeToString([]byte("h1"))}
	pool := &indexer.Pool{Logs: []*nodeData.LogData{logFor("h1", empty, empty)}}

	newLogsProcessorForTest(t).ExtractDataFromLogs(pool, []*data.Transaction{tx}, 100)

	require.True(t, tx.HasLogs)
	require.Nil(t, tx.SCAddresses, "nothing qualifies, so the field stays off the document")
}

// TestExtractDataFromLogs_MergesSeveralLogsOfOneTransaction covers a transaction that
// appears more than once in the pool's logs: the second log must add to the first, not
// replace it.
func TestExtractDataFromLogs_MergesSeveralLogsOfOneTransaction(t *testing.T) {
	t.Parallel()

	contractA, contractB := scAddress(1), scAddress(2)
	tx := &data.Transaction{Hash: hex.EncodeToString([]byte("h1"))}
	pool := &indexer.Pool{Logs: []*nodeData.LogData{
		logFor("h1", contractB),
		logFor("h1", contractA),
	}}

	newLogsProcessorForTest(t).ExtractDataFromLogs(pool, []*data.Transaction{tx}, 100)

	require.Equal(t, []string{hex.EncodeToString(contractA), hex.EncodeToString(contractB)}, tx.SCAddresses)
}

// TestExtractDataFromLogs_LeavesTransactionsWithoutContractsAlone is the omitempty half: a
// plain transfer has a log with a wallet address and no contract events, and must not gain
// an empty array, which would be indexed and would make "has the field" a wrong test for
// "touched a contract".
func TestExtractDataFromLogs_LeavesTransactionsWithoutContractsAlone(t *testing.T) {
	t.Parallel()

	withWalletLog := &data.Transaction{Hash: hex.EncodeToString([]byte("h1"))}
	withoutLog := &data.Transaction{Hash: hex.EncodeToString([]byte("h2"))}
	pool := &indexer.Pool{Logs: []*nodeData.LogData{logFor("h1", walletAddress(1), walletAddress(2))}}

	newLogsProcessorForTest(t).ExtractDataFromLogs(pool, []*data.Transaction{withWalletLog, withoutLog}, 100)

	require.True(t, withWalletLog.HasLogs)
	require.Nil(t, withWalletLog.SCAddresses)
	require.False(t, withoutLog.HasLogs)
	require.Nil(t, withoutLog.SCAddresses)
}
