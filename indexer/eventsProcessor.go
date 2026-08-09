package indexer

import (
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	indexerData "github.com/klever-io/klever-go/data/indexer"
	dataState "github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/klever-io/klever-go/tools/check"
)

type eventsProcessor struct {
	*txDatabaseProcessor
	indexer           Indexer
	parser            *dataParser
	kappsController   kapp.KAppController
	accountsDB        dataState.AccountsAdapter
	logsAndEventsProc LogsAndEventsHandler
}

func NewEventsProcessor(arguments ArgEventsProcessor) (*eventsProcessor, error) {
	err := checkArgEventsProcessor(arguments)
	if err != nil {
		return nil, err
	}

	// Reuses the same converter (bech32 addresses, hex topics/data) already used to
	// prepare logs for the Elasticsearch index, so a LOGS websocket event and its
	// Elasticsearch counterpart for the same block never drift in shape.
	logsAndEventsProc, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: arguments.AddressPubkeyConverter,
		Marshalizer:     arguments.Marshalizer,
		Hasher:          arguments.Hasher,
	})
	if err != nil {
		return nil, err
	}

	ep := &eventsProcessor{
		txDatabaseProcessor: newTxDatabaseProcessor(
			arguments.Hasher,
			arguments.Marshalizer,
			arguments.AddressPubkeyConverter,
			arguments.ValidatorPubkeyConverter,
			false,
		),
		indexer: arguments.Indexer,
		parser: &dataParser{
			hasher:      arguments.Hasher,
			marshalizer: arguments.Marshalizer,
		},
		kappsController:   arguments.KAppController,
		accountsDB:        arguments.AccountsDB,
		logsAndEventsProc: logsAndEventsProc,
	}

	return ep, nil
}

func checkArgEventsProcessor(arguments ArgEventsProcessor) error {
	if check.IfNil(arguments.Marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(arguments.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arguments.AddressPubkeyConverter) {
		return ErrNilPubkeyConverter
	}
	if check.IfNil(arguments.ValidatorPubkeyConverter) {
		return ErrNilPubkeyConverter
	}
	return nil
}

func (ep *eventsProcessor) Enabled() bool {
	return UseEventQueue || ep.indexerEnabled()
}

func (ep *eventsProcessor) indexerEnabled() bool {
	return !check.IfNil(ep.indexer) && !ep.indexer.IsNilIndexer()
}

func (ep *eventsProcessor) SaveBlock(args *indexerData.ArgsSaveBlockData) {
	wsEnabled := UseEventQueue
	indexerEnabled := ep.indexerEnabled()
	if !wsEnabled && !indexerEnabled {
		return
	}

	// Prep runs on the commit goroutine only when websocket subscribers need
	// it. Indexer-only nodes let the elastic worker re-prep on its own
	// goroutine via the fallback in elasticProcessor.SaveTransactions.
	if wsEnabled {
		prepared := ep.prepare(args)
		ep.dispatchBlockEvent(args)
		var txsMap map[string]*data.Transaction
		if prepared != nil {
			txsMap = prepared.TxsMap
			if indexerEnabled {
				// Run here, synchronously, so the elastic worker's async SaveTransactions
				// doesn't race the websocket hub's json.Marshal of the same prepared.Txs
				// pointers by mutating tx.HasLogs/HasOperations on its own goroutine.
				prepared.LogsResults = ep.logsAndEventsProc.ExtractDataFromLogs(args.TransactionsPool, prepared.Txs, args.Header.GetTimestamp())
			}
			ep.dispatchTransactionEvents(prepared.Txs)
			ep.dispatchAccountEventsFromAlteredAccounts(args.Header.GetTimestamp(), prepared.Altered.Accounts)
		}
		// Dispatched outside the prepared-only branch: a tx-prep failure must not silently
		// drop a block's logs, same as BLOCKS already ships regardless of prepare().
		ep.dispatchLogEvents(prepared, args.TransactionsPool, txsMap, args.Header.GetTimestamp())
		if indexerEnabled {
			args.Prepared = prepared
		}
	}

	if indexerEnabled {
		ep.indexer.SaveBlock(args)
	}
}

func (ep *eventsProcessor) prepare(args *indexerData.ArgsSaveBlockData) *data.PreparedBlockData {
	if args.TransactionsPool == nil || len(args.TransactionsPool.Txs) == 0 {
		return nil
	}
	txs, txsMap, ad, err := ep.prepareTransactionsForDatabase(args.Header, args.TransactionsPool)
	if err != nil {
		log.Error("eventsProcessor.SaveBlock: prepare failed", "nonce", args.Header.GetNonce(), "error", err)
		return nil
	}
	return &data.PreparedBlockData{Txs: txs, TxsMap: txsMap, Altered: ad}
}

func (ep *eventsProcessor) dispatchBlockEvent(args *indexerData.ArgsSaveBlockData) {
	// byte-size to match SaveHeader's SizeTxs.
	txsSize := 0
	if args.TransactionsPool != nil {
		txsSize = workItems.ComputeSizeOfTxs(args.TransactionsPool)
	}

	serializedBlock, _, err := ep.parser.getSerializedElasticBlockAndHeaderHash(
		args.Header,
		args.Signer,
		txsSize,
		args.Validators,
	)
	if err != nil {
		log.Warn("eventsProcessor.dispatchBlockEvent", "error", err.Error())
		return
	}

	trySendEvent(Event{
		EvType:  BLOCKS,
		Message: serializedBlock,
	})
}

func (ep *eventsProcessor) dispatchTransactionEvents(txs []*data.Transaction) {
	if len(txs) == 0 {
		return
	}
	trySendEvent(Event{
		EvType:  USER_TRANSACTIONS,
		Message: txs,
	})
	trySendEvent(Event{
		EvType:  TRANSACTIONS,
		Message: txs,
	})
}

// dispatchLogEvents converts the block's raw smart-contract logs into the same shape
// already used for the Elasticsearch index (bech32 addresses, hex topics/data) and
// dispatches them as one LOGS event; the websocket hub fans each entry out by its
// contract address, same as it does per-account for ACCOUNTS. When prepared is non-nil,
// the conversion is also stashed on it (PreparedBlockData.LogsDB) so the elastic worker's
// own logs indexing reuses it instead of paying the same bech32/hex-encoding pass twice.
func (ep *eventsProcessor) dispatchLogEvents(prepared *data.PreparedBlockData, pool *indexerData.Pool, txsMap map[string]*data.Transaction, blockTimestamp int64) {
	if pool == nil || len(pool.Logs) == 0 {
		return
	}
	if LogsSubscriberChecker != nil && !LogsSubscriberChecker() {
		return
	}

	logsDB := ep.logsAndEventsProc.PrepareLogsForDB(pool.Logs, txsMap, blockTimestamp)
	if prepared != nil {
		prepared.LogsDB = logsDB
	}
	if len(logsDB) == 0 {
		return
	}

	trySendEvent(Event{
		EvType:  LOGS,
		Message: logsDB,
	})
}

// dispatchAccountEventsFromAlteredAccounts uses GetExistingAccount so that
// addresses with no persisted state (e.g. ZeroAddress) are silently dropped
// rather than broadcast as empty ghost accounts.
func (ep *eventsProcessor) dispatchAccountEventsFromAlteredAccounts(blockTimestamp int64, alteredAccounts data.AlteredAccountsHandler) {
	if check.IfNil(ep.accountsDB) || alteredAccounts.Len() == 0 {
		return
	}

	accountsMap := make(map[string]*data.AccountInfo, alteredAccounts.Len())
	for address := range alteredAccounts.GetAll() {
		info, err := ep.buildAlteredAccountInfo(address, blockTimestamp)
		if err != nil {
			log.Warn("eventsProcessor.dispatchAccountEventsFromAlteredAccounts", "address", address, "error", err)
			continue
		}
		if info == nil {
			continue
		}
		accountsMap[info.Address] = info
	}

	if len(accountsMap) == 0 {
		return
	}

	dispatchAccountEvents(accountsMap)
}

// buildAlteredAccountInfo resolves an altered address into an AccountInfo.
// Returns (nil, nil) when the account doesn't exist — a normal no-op skip.
func (ep *eventsProcessor) buildAlteredAccountInfo(address string, blockTimestamp int64) (*data.AccountInfo, error) {
	addrBytes, err := ep.addressPubkeyConverter.Decode(address)
	if err != nil {
		return nil, fmt.Errorf("decode address: %w", err)
	}

	accountHandler, err := ep.accountsDB.GetExistingAccount(addrBytes)
	if err != nil {
		if errors.Is(err, common.ErrAccNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load account: %w", err)
	}

	userAccount, ok := accountHandler.(dataState.UserAccountHandler)
	if !ok {
		return nil, fmt.Errorf("account is not a UserAccountHandler")
	}

	return buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, userAccount, blockTimestamp)
}

func (ep *eventsProcessor) RevertIndexedBlock(header nodeData.HeaderHandler) {
	if !ep.indexerEnabled() {
		return
	}
	ep.indexer.RevertIndexedBlock(header)
}

func (ep *eventsProcessor) SaveValidatorsRating(validators []kapp.ValidatorAccountInfoHandler) {
	if !ep.indexerEnabled() {
		return
	}
	ep.indexer.SavePeersAccounts(validators)
}

func (ep *eventsProcessor) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
	if !ep.indexerEnabled() {
		return
	}
	ep.indexer.SaveEpochInfo(epoch, validators)
}

func (ep *eventsProcessor) UpdateProposalsAndParameters(proposalIDs []string) {
	if !ep.indexerEnabled() {
		return
	}
	ep.indexer.UpdateProposalsAndParameters(proposalIDs)
}

func (ep *eventsProcessor) SaveAccounts(blockTimestamp int64, acc []dataState.UserAccountHandler) {
	if UseEventQueue && len(acc) > 0 {
		accountsMap := make(map[string]*data.AccountInfo, len(acc))
		for _, userAccount := range acc {
			info, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, userAccount, blockTimestamp)
			if err != nil {
				log.Warn("eventsProcessor.SaveAccounts", "error", err.Error())
				continue
			}
			accountsMap[info.Address] = info
		}
		dispatchAccountEvents(accountsMap)
	}

	if !ep.indexerEnabled() {
		return
	}
	ep.indexer.SaveAccounts(blockTimestamp, acc)
}

func (ep *eventsProcessor) IsInterfaceNil() bool {
	return ep == nil
}
