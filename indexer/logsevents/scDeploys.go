package logsevents

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/indexer/data"
)

type scDeploysProcessor struct {
	scDeploysIdentifiers map[string]struct{}
	pubKeyConverter      core.PubkeyConverter
}

func newSCDeploysProcessor(pubKeyConverter core.PubkeyConverter) *scDeploysProcessor {
	return &scDeploysProcessor{
		pubKeyConverter: pubKeyConverter,
		scDeploysIdentifiers: map[string]struct{}{
			core.SCDeployIdentifier:  {},
			core.SCUpgradeIdentifier: {},
		},
	}
}

func (sdp *scDeploysProcessor) processEvent(args *argsProcessEvent) argOutputProcessEvent {
	_, ok := sdp.scDeploysIdentifiers[string(args.event.GetIdentifier())]
	if !ok {
		return argOutputProcessEvent{}
	}

	// Only the node's own deploy/upgrade log counts: an event identifier is just the name of
	// the endpoint that wrote it, and a contract can export an endpoint called SCDeploy. The
	// topics are also contract-controlled in that case, so require an exact address length
	// before Encode, which logs a stack-trace WARN on a wrong-sized input.
	topics := args.event.GetTopics()
	addressLen := sdp.pubKeyConverter.Len()
	if !args.event.GetIsSystemLog() || len(topics) < 2 ||
		len(topics[0]) != addressLen || len(topics[1]) != addressLen {
		return argOutputProcessEvent{
			processed: true,
		}
	}

	scAddress := sdp.pubKeyConverter.Encode(topics[0])
	args.scDeploys[scAddress] = &data.ScDeployInfo{
		TxHash:    args.txHashHexEncoded,
		Creator:   sdp.pubKeyConverter.Encode(topics[1]),
		Timestamp: uint64(args.timestamp), // #nosec G115
	}

	return argOutputProcessEvent{
		processed: true,
	}
}
