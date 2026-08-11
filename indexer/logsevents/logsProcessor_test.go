package logsevents_test

import (
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

	// An SCDeploy-identified event with well-formed topics: the full processor set would
	// populate ScDeploys via two Encode calls (creator + deployed address); informativeLogsProcessor
	// doesn't react to this identifier at all, so lightweight mode should do nothing with it.
	event := &transaction.Event{
		Identifier: []byte(core.SCDeployIdentifier),
		Topics:     [][]byte{[]byte("deployedaddr12345678901234567890"), []byte("creatoraddr123456789012345678901")},
	}
	logHandler := &transaction.Log{
		Address: []byte("contractaddr"),
		Events:  []*transaction.Event{event},
	}
	pool := &indexer.Pool{Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txhash1"}}}
	txs := []*data.Transaction{{Hash: "txhash1"}}

	lightResult := lep.ExtractDataFromLogs(pool, txs, 100, false)
	assert.Empty(t, lightResult.ScDeploys, "lightweight mode must not populate ScDeploys")
	assert.Equal(t, 0, encodeCalls, "lightweight mode must never call pubKeyConverter.Encode")

	fullResult := lep.ExtractDataFromLogs(pool, txs, 100, true)
	assert.NotEmpty(t, fullResult.ScDeploys, "full mode must populate ScDeploys as before")
	assert.Greater(t, encodeCalls, 0, "full mode must still call pubKeyConverter.Encode")
}
