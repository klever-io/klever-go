package logsevents

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/indexer/data"
)

var log = logger.GetOrCreate("indexer/logsevents/scInvocations")

type scInvocationsProcessor struct {
	scInvocationIdentifiers map[string]struct{}
	pubKeyConverter         core.PubkeyConverter
}

func newSCInvocationsProcessor(pubKeyConverter core.PubkeyConverter) *scInvocationsProcessor {
	return &scInvocationsProcessor{
		pubKeyConverter: pubKeyConverter,
		scInvocationIdentifiers: map[string]struct{}{
			core.CompletedTxEventIdentifier: {},
		},
	}
}

func (sip *scInvocationsProcessor) processEvent(args *argsProcessEvent) argOutputProcessEvent {
	_, ok := sip.scInvocationIdentifiers[string(args.event.GetIdentifier())]
	if !ok {
		return argOutputProcessEvent{}
	}

	// For completedTxEvent, the address where this log is emitted is the smart contract address
	scAddress := sip.pubKeyConverter.Encode(args.logAddress)

	// Check if this is a smart contract address (starts with klv1qqqq...)
	if !core.IsSmartContractAddress(args.logAddress) {
		// Even if not a smart contract address, mark as processed since we handled this event
		return argOutputProcessEvent{
			processed: true,
		}
	}

	// Add debug logging to trace what's happening
	log.Debug("scInvocationsProcessor: processing completedTxEvent",
		"scAddress", scAddress,
		"txHash", args.txHashHexEncoded)

	if args.alteredSC != nil {
		args.alteredSC.Add(scAddress, &data.AlteredSmartContract{
			IsNew: false,
		})

		log.Debug("scInvocationsProcessor: added to alteredSC",
			"scAddress", scAddress,
			"txHash", args.txHashHexEncoded)
	}

	return argOutputProcessEvent{
		processed: true,
	}
}
