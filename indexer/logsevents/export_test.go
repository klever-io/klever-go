package logsevents

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
)

// Exported for testing purposes

// NewSCInvocationsProcessorForTest exports newSCInvocationsProcessor for testing
func NewSCInvocationsProcessorForTest(pubKeyConverter core.PubkeyConverter) *scInvocationsProcessor {
	return newSCInvocationsProcessor(pubKeyConverter)
}

// NewArgsProcessEventForTest creates argsProcessEvent for testing
func NewArgsProcessEventForTest(
	event transaction.EventHandler,
	txHashHexEncoded string,
	logAddress []byte,
	alteredSC data.AlteredSmartContractsHandler,
) *argsProcessEvent {
	return &argsProcessEvent{
		event:            event,
		txHashHexEncoded: txHashHexEncoded,
		logAddress:       logAddress,
		alteredSC:        alteredSC,
	}
}

// ProcessEventForTest exports processEvent for testing
func (sip *scInvocationsProcessor) ProcessEventForTest(args *argsProcessEvent) argOutputProcessEvent {
	return sip.processEvent(args)
}

// IsProcessedForTest checks if the result was processed
func (a argOutputProcessEvent) IsProcessedForTest() bool {
	return a.processed
}
