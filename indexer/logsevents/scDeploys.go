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
	systemLog, ok := args.event.(interface{ GetIsSystemLog() bool })
	if !ok || !systemLog.GetIsSystemLog() {
		return argOutputProcessEvent{
			processed: true,
		}
	}

	topics := args.event.GetTopics()
	if len(topics) < 2 {
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
