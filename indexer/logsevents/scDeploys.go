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

	// Only the node emits these events, as system logs. A contract can export a
	// SCDeploy/SCUpgrade function and forge an identical event with IsSystemLog
	// false, so reject non-system logs to prevent forged scdeploys entries.
	if !args.event.GetIsSystemLog() {
		return argOutputProcessEvent{
			processed: true,
		}
	}

	// Skip malformed topics: Encode warns with a stack trace on a wrong-size
	// address and returns "", which would be stored as a scDeploys[""] entry.
	topics := args.event.GetTopics()
	addressLen := sdp.pubKeyConverter.Len()
	if len(topics) < 2 || len(topics[0]) != addressLen || len(topics[1]) != addressLen {
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
