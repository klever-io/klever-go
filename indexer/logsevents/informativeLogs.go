package logsevents

import (
	"github.com/klever-io/klever-go/data/transaction"
)

const (
	writeLogOperation    = "writeLog"
	signalErrorOperation = "signalError"
)

type informativeLogsProcessor struct {
	operations map[string]struct{}
}

func newInformativeLogsProcessor() *informativeLogsProcessor {
	return &informativeLogsProcessor{
		operations: map[string]struct{}{
			writeLogOperation:    {},
			signalErrorOperation: {},
		},
	}
}

func (ilp *informativeLogsProcessor) processEvent(args *argsProcessEvent) argOutputProcessEvent {
	identifier := string(args.event.GetIdentifier())
	_, ok := ilp.operations[identifier]
	if !ok {
		return argOutputProcessEvent{}
	}

	tx, ok := args.txs[args.txHashHexEncoded]
	if !ok {
		return argOutputProcessEvent{
			processed: true,
		}
	}

	switch identifier {
	case writeLogOperation:
		{
			tx.Status = transaction.Transaction_SUCCESS.String()
		}
	case signalErrorOperation:
		{
			tx.Status = transaction.Transaction_Fail.String()
		}
	}

	return argOutputProcessEvent{
		processed: true,
	}
}
