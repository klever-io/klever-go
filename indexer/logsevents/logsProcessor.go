package logsevents

import (
	"encoding/hex"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/converters"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type logsData struct {
	timestamp int64
	txsMap    map[string]*data.Transaction
	scDeploys map[string]*data.ScDeployInfo
	alteredSC data.AlteredSmartContractsHandler
}

// ArgsLogsAndEventsProcessor  holds all dependencies required to create new instances of logsAndEventsProcessor
type ArgsLogsAndEventsProcessor struct {
	PubKeyConverter core.PubkeyConverter
	Marshalizer     marshal.Marshalizer
	Hasher          hashing.Hasher
}

type logsAndEventsProcessor struct {
	hasher          hashing.Hasher
	pubKeyConverter core.PubkeyConverter
	// eventsProcessors is the full set (scDeploy/scInvocation extraction, plus the cheap
	// informative-status tracking); lightweightProcessors holds only the latter. Kept as
	// two slices sharing the same informativeLogsProcessor instance, rather than filtering
	// eventsProcessors per call, so ExtractDataFromLogs(full=false) never even reaches the
	// scDeploy/scInvocation processors' address-decoding of contract-controlled event
	// topics — those results (ScDeploys/AlteredSCs) are Elasticsearch-only and unused by
	// the websocket payload, so paying their cost when no indexer will consume them is
	// pure waste, and on malformed/adversarial topics a real (if bounded) one.
	eventsProcessors      []eventsProcessor
	lightweightProcessors []eventsProcessor

	logsData *logsData
}

// ExtractDataFromLogs extracts data from the provided logs and events, always setting the
// cheap per-tx tracking (HasLogs/HasOperations/Status via informativeLogsProcessor) needed
// for websocket payload parity. full additionally runs the scDeploy/scInvocation
// processors and populates the returned ScDeploys/AlteredSCs — pass true only when the
// caller will actually index them (Elasticsearch enabled); the websocket dispatcher never
// reads those fields.
func (lep *logsAndEventsProcessor) ExtractDataFromLogs(
	pool *indexer.Pool,
	txs []*data.Transaction,
	timestamp int64,
	full bool,
) *data.PreparedLogsResults {
	lep.logsData = newLogsData(timestamp, txs)
	procs := lep.lightweightProcessors
	if full {
		procs = lep.eventsProcessors
	}

	for _, txLog := range pool.Logs {
		if txLog == nil || check.IfNil(txLog.LogHandler) {
			continue
		}

		txHashHexEncoded := hex.EncodeToString([]byte(txLog.TxHash))

		events := txLog.GetLogEvents()
		lep.processEvents(procs, txHashHexEncoded, txLog.GetAddress(), events)

		tx, ok := lep.logsData.txsMap[txHashHexEncoded]
		if ok {
			tx.HasLogs = true
			continue
		}
	}

	return &data.PreparedLogsResults{
		ScDeploys:  lep.logsData.scDeploys,
		AlteredSCs: lep.logsData.alteredSC,
	}
}

func newLogsData(
	timestamp int64,
	txs []*data.Transaction,
) *logsData {
	ld := &logsData{}

	ld.txsMap = converters.ConvertTxsSliceIntoMap(txs)
	ld.timestamp = timestamp
	ld.scDeploys = make(map[string]*data.ScDeployInfo)
	ld.alteredSC = data.NewAlteredSmartContracts()

	return ld
}

func (lep *logsAndEventsProcessor) processEvents(procs []eventsProcessor, logHash string, logAddress []byte, events []transaction.EventHandler) {
	for _, event := range events {
		if check.IfNil(event) {
			continue
		}

		lep.processEvent(procs, logHash, logAddress, event)
	}

}

func (lep *logsAndEventsProcessor) processEvent(procs []eventsProcessor, logHashHexEncoded string, logAddress []byte, event transaction.EventHandler) {
	for _, proc := range procs {
		res := proc.processEvent(&argsProcessEvent{
			event:            event,
			txHashHexEncoded: logHashHexEncoded,
			logAddress:       logAddress,
			timestamp:        lep.logsData.timestamp,
			scDeploys:        lep.logsData.scDeploys,
			txs:              lep.logsData.txsMap,
			alteredSC:        lep.logsData.alteredSC,
		})

		tx, ok := lep.logsData.txsMap[logHashHexEncoded]
		if ok {
			tx.HasOperations = true
			continue
		}

		if res.processed {
			return
		}
	}
}

// PrepareLogsForDB will prepare logs for database
func (lep *logsAndEventsProcessor) PrepareLogsForDB(
	logsAndEvents []*nodeData.LogData,
	txsMap map[string]*data.Transaction,
	timestamp int64,
) []*data.Logs {
	logs := make([]*data.Logs, 0, len(logsAndEvents))

	for _, txLog := range logsAndEvents {
		if txLog == nil || check.IfNil(txLog.LogHandler) {
			continue
		}

		logHex := hex.EncodeToString([]byte(txLog.TxHash))
		// tx is nil on a txsMap miss; prepareLogsForDB degrades Caller/Status/ResultCode
		// to their zero values in that case, same as the existing Caller behavior.
		tx := txsMap[string(txLog.TxHash)]

		logs = append(logs, lep.prepareLogsForDB(logHex, txLog.LogHandler, timestamp, tx))
	}

	return logs
}

func (lep *logsAndEventsProcessor) prepareLogsForDB(
	logHashHex string,
	logHandler nodeData.LogHandler,
	timestamp int64,
	tx *data.Transaction,
) *data.Logs {
	events := logHandler.GetLogEvents()
	logsDB := &data.Logs{
		ID:         logHashHex,
		Address:    lep.pubKeyConverter.Encode(logHandler.GetAddress()),
		ContractID: logHandler.GetContractID(),
		Timestamp:  time.Duration(timestamp),
		Events:     make([]*data.Event, 0, len(events)),
	}
	if tx != nil {
		// Status/ResultCode let a subscriber tell a log produced by a reverted/failed
		// transaction apart from one whose effects actually committed (a tx can still
		// generate SC logs on its error path, see processIfErrorWithAddedLogs).
		logsDB.Caller = tx.Sender
		logsDB.Status = tx.Status
		logsDB.ResultCode = tx.ResultCode
	}

	for idx, event := range events {
		if check.IfNil(event) {
			continue
		}

		topics := make([]string, 0, len(event.GetTopics()))
		for _, topic := range event.GetTopics() {
			topics = append(topics, hex.EncodeToString(topic))
		}
		logData := make([]string, 0, len(event.GetData()))
		for _, d := range event.GetData() {
			logData = append(logData, hex.EncodeToString(d))
		}

		logsDB.Events = append(logsDB.Events, &data.Event{
			Address:     lep.pubKeyConverter.Encode(event.GetAddress()),
			Identifier:  string(event.GetIdentifier()),
			Topics:      topics,
			Data:        logData,
			Order:       idx,
			IsSystemLog: event.GetIsSystemLog(),
		})

	}

	return logsDB
}

type argsProcessEvent struct {
	txHashHexEncoded string
	scDeploys        map[string]*data.ScDeployInfo
	txs              map[string]*data.Transaction
	event            transaction.EventHandler
	timestamp        int64
	logAddress       []byte
	alteredSC        data.AlteredSmartContractsHandler
}

type argOutputProcessEvent struct {
	processed bool
}

type eventsProcessor interface {
	processEvent(args *argsProcessEvent) argOutputProcessEvent
}

// NewLogsAndEventsProcessor will create a new instance for the logsAndEventsProcessor
func NewLogsAndEventsProcessor(args ArgsLogsAndEventsProcessor) (*logsAndEventsProcessor, error) {
	err := checkArgsLogsAndEventsProcessor(args)
	if err != nil {
		return nil, err
	}

	eventsProcessors, lightweightProcessors := createEventsProcessors(args)

	return &logsAndEventsProcessor{
		pubKeyConverter:       args.PubKeyConverter,
		eventsProcessors:      eventsProcessors,
		lightweightProcessors: lightweightProcessors,
		hasher:                args.Hasher,
	}, nil
}

func checkArgsLogsAndEventsProcessor(args ArgsLogsAndEventsProcessor) error {
	if check.IfNil(args.PubKeyConverter) {
		return common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.Marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return common.ErrNilHasher
	}

	return nil
}

// createEventsProcessors returns the full processor set and, sharing the same
// informativeLogsProcessor instance, a lightweight subset used when the caller won't
// consume ScDeploys/AlteredSCs (see ExtractDataFromLogs's full parameter).
func createEventsProcessors(args ArgsLogsAndEventsProcessor) (full, lightweight []eventsProcessor) {
	scDeploysProc := newSCDeploysProcessor(args.PubKeyConverter)
	scInvocationsProc := newSCInvocationsProcessor(args.PubKeyConverter)
	informativeProc := newInformativeLogsProcessor()

	full = []eventsProcessor{
		scDeploysProc,
		scInvocationsProc,
		informativeProc,
	}
	lightweight = []eventsProcessor{
		informativeProc,
	}

	return full, lightweight
}
