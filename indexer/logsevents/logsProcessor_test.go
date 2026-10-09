package logsevents_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractDataFromLogs_LightweightModeSkipsExpensiveProcessors guards the fix for a
// finding a security audit raised: making ExtractDataFromLogs run unconditionally (for
// websocket payload parity, regardless of whether Elasticsearch is enabled) newly exposed
// ws-only nodes to the scDeploy/scInvocation processors' address-decoding of
// contract-controlled event topics — expensive, and previously reachable only on
// ES-enabled nodes. full=false must skip those processors (and their pubKeyConverter.Encode
// calls) entirely, while still setting the informative tx.Status flag every caller needs.
func TestExtractDataFromLogs_LightweightModeSkipsExpensiveProcessors(t *testing.T) {
	var encodeCalls int
	pubKeyConverter := &mock.PubkeyConverterStub{
		LenCalled: func() int { return 32 },
		EncodeCalled: func(pkBytes []byte) string {
			encodeCalls++
			return "klv1encoded"
		},
	}

	lep, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: pubKeyConverter,
		Marshalizer:     &mock.MarshalizerMock{},
		Hasher:          &mock.HasherMock{},
	})
	require.NoError(t, err)

	// Two events on the same log: an SCDeploy-identified one (well-formed topics) that only
	// scDeploysProcessor reacts to — the full processor set would populate ScDeploys via
	// two Encode calls (creator + deployed address) — and a writeLog-identified one that
	// only informativeLogsProcessor reacts to, overwriting tx.Status. txsMap is keyed by
	// tx.Hash matched against hex.EncodeToString(TxHash) (see converters.ConvertTxsSliceIntoMap
	// and ExtractDataFromLogs's lookup), so the tx's Hash must be the hex-encoded raw hash.
	rawTxHash := "txhash1"
	deployEvent := &transaction.Event{
		Identifier: []byte(core.SCDeployIdentifier),
		Topics:     [][]byte{[]byte("deployedaddr12345678901234567890"), []byte("creatoraddr123456789012345678901")},
		// The node's own deploy log; see TestExtractDataFromLogs_ForgedSCDeployEventIgnored.
		IsSystemLog: true,
	}
	writeLogEvent := &transaction.Event{Identifier: []byte("writeLog")}
	logHandler := &transaction.Log{
		Address: []byte("contractaddr"),
		Events:  []*transaction.Event{deployEvent, writeLogEvent},
	}
	pool := &indexer.Pool{Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: rawTxHash}}}
	newTxs := func() []*data.Transaction {
		return []*data.Transaction{{Hash: hex.EncodeToString([]byte(rawTxHash))}}
	}

	lightTxs := newTxs()
	lightResult := lep.ExtractDataFromLogs(pool, lightTxs, 100, false)
	assert.Empty(t, lightResult.ScDeploys, "lightweight mode must not populate ScDeploys")
	assert.Equal(t, 0, encodeCalls, "lightweight mode must never call pubKeyConverter.Encode")
	assert.True(t, lightTxs[0].HasLogs)
	assert.True(t, lightTxs[0].HasOperations, "the cheap informativeLogsProcessor path must still mark HasOperations")
	assert.Equal(t, transaction.Transaction_SUCCESS.String(), lightTxs[0].Status,
		"informativeLogsProcessor's Status override must survive in lightweight mode — it's what the websocket payload reports")

	fullTxs := newTxs()
	fullResult := lep.ExtractDataFromLogs(pool, fullTxs, 100, true)
	assert.NotEmpty(t, fullResult.ScDeploys, "full mode must populate ScDeploys as before")
	assert.Greater(t, encodeCalls, 0, "full mode must still call pubKeyConverter.Encode")
	assert.True(t, fullTxs[0].HasLogs)
	assert.True(t, fullTxs[0].HasOperations)
	assert.Equal(t, transaction.Transaction_SUCCESS.String(), fullTxs[0].Status,
		"full mode must produce the same tx-field parity as lightweight mode")
}

// TestExtractDataFromLogs_ForgedSCDeployEventIgnored pins that a contract cannot forge an
// ES scdeploys record: an event identifier is just the name of the endpoint that wrote it,
// so a contract exporting an endpoint called SCDeploy emits an identical-looking event. Only
// the node's own (system) log with address-sized topics may produce one, and a rejected
// event must not reach pubKeyConverter.Encode at all (it logs a stack-trace WARN on a
// wrong-sized input).
func TestExtractDataFromLogs_ForgedSCDeployEventIgnored(t *testing.T) {
	addr := func(c byte) []byte { return []byte(strings.Repeat(string(c), 32)) }
	tests := []struct {
		name  string
		event *transaction.Event
	}{
		{"contract-written (not a system log)", &transaction.Event{
			Identifier: []byte(core.SCDeployIdentifier), Topics: [][]byte{addr('a'), addr('b')}}},
		{"system log with short topic", &transaction.Event{
			Identifier: []byte(core.SCDeployIdentifier), IsSystemLog: true, Topics: [][]byte{[]byte("short"), addr('b')}}},
		{"system log with oversized creator topic", &transaction.Event{
			Identifier: []byte(core.SCUpgradeIdentifier), IsSystemLog: true, Topics: [][]byte{addr('a'), append(addr('b'), 'x')}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var encodeCalls int
			lep, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
				PubKeyConverter: &mock.PubkeyConverterStub{
					LenCalled:    func() int { return 32 },
					EncodeCalled: func([]byte) string { encodeCalls++; return "klv1encoded" },
				},
				Marshalizer: &mock.MarshalizerMock{},
				Hasher:      &mock.HasherMock{},
			})
			require.NoError(t, err)

			pool := &indexer.Pool{Logs: []*nodeData.LogData{{
				LogHandler: &transaction.Log{Address: []byte("contractaddr"), Events: []*transaction.Event{tc.event}},
				TxHash:     "txhash1",
			}}}
			txs := []*data.Transaction{{Hash: hex.EncodeToString([]byte("txhash1"))}}

			result := lep.ExtractDataFromLogs(pool, txs, 100, true)
			assert.Empty(t, result.ScDeploys)
			assert.Equal(t, 0, encodeCalls)
		})
	}
}
