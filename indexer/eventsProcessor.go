package indexer

import (
	"errors"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	indexerData "github.com/klever-io/klever-go/data/indexer"
	dataState "github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/klever-io/klever-go/tools/check"
)

type eventsProcessor struct {
	*txDatabaseProcessor
	indexer         Indexer
	parser          *dataParser
	kappsController kapp.KAppController
	accountsDB      dataState.AccountsAdapter
}

func NewEventsProcessor(arguments ArgEventsProcessor) (*eventsProcessor, error) {
	err := checkArgEventsProcessor(arguments)
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
		kappsController: arguments.KAppController,
		accountsDB:      arguments.AccountsDB,
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
	return UseEventQueue || (ep.indexer != nil && !ep.indexer.IsNilIndexer())
}

// SaveBlock is the single per-block orchestrator. It runs on the commit
// goroutine and is the only place that calls prepareTransactionsForDatabase
// for a given block. Websocket events are enqueued here (non-blocking
// trySendEvent) so subscribers see consistent timing across configurations,
// and the prepared payload is then forwarded to the indexer via
// ArgsSaveBlockData.Prepared so the elastic worker never re-preps.
func (ep *eventsProcessor) SaveBlock(args *indexerData.ArgsSaveBlockData) {
	wsEnabled := UseEventQueue
	indexerEnabled := !check.IfNil(ep.indexer) && !ep.indexer.IsNilIndexer()
	if !wsEnabled && !indexerEnabled {
		return
	}

	var prepared *data.PreparedBlockData
	if args.TransactionsPool != nil && len(args.TransactionsPool.Txs) > 0 {
		txs, txsMap, ad, err := ep.prepareTransactionsForDatabase(args.Header, args.TransactionsPool)
		if err != nil {
			// Per-block prep failure: ws subscribers get no TXN/ACCOUNT events for this
			// block, and the indexer worker's fallback re-prep will hit the same error.
			// Loud signal so it shows up in node operator alerting.
			log.Error("eventsProcessor.SaveBlock: prepare failed", "nonce", args.Header.GetNonce(), "error", err)
		} else {
			prepared = &data.PreparedBlockData{Txs: txs, TxsMap: txsMap, Altered: ad}
		}
	}

	if wsEnabled {
		ep.dispatchBlockEvent(args)
		if prepared != nil {
			ep.dispatchTransactionEvents(prepared.Txs)
			ep.dispatchAccountEventsFromAlteredAccounts(args.Header.GetTimestamp(), prepared.Altered.Accounts)
		}
	}

	if indexerEnabled {
		args.Prepared = prepared
		ep.indexer.SaveBlock(args)
	}
}

func (ep *eventsProcessor) dispatchBlockEvent(args *indexerData.ArgsSaveBlockData) {
	// Use byte-size (not tx count) so the BLOCKS event payload's SizeTxs/VirtualBlockSize
	// match what the elastic indexer writes via workItems.ComputeSizeOfTxs in SaveHeader.
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

func (ep *eventsProcessor) dispatchAccountEventsFromAlteredAccounts(blockTimestamp int64, alteredAccounts data.AlteredAccountsHandler) {
	if check.IfNil(ep.accountsDB) || alteredAccounts.Len() == 0 {
		return
	}

	accountsMap := make(map[string]*data.AccountInfo, alteredAccounts.Len())
	for address := range alteredAccounts.GetAll() {
		addrBytes, err := ep.addressPubkeyConverter.Decode(address)
		if err != nil {
			log.Warn("eventsProcessor.dispatchAccountEventsFromAlteredAccounts: cannot decode address", "address", address, "error", err)
			continue
		}

		// GetExistingAccount (vs LoadAccount used by the indexer): we do not
		// want to broadcast empty ghost-account payloads for addresses that
		// have no persisted state (e.g., ZeroAddress); a missing account is
		// a no-op for subscribers.
		accountHandler, err := ep.accountsDB.GetExistingAccount(addrBytes)
		if err != nil {
			if !errors.Is(err, common.ErrAccNotFound) {
				log.Warn("eventsProcessor.dispatchAccountEventsFromAlteredAccounts: cannot load account", "address", address, "error", err)
			}
			continue
		}

		userAccount, ok := accountHandler.(dataState.UserAccountHandler)
		if !ok {
			log.Warn("eventsProcessor.dispatchAccountEventsFromAlteredAccounts: cannot cast AccountHandler to UserAccountHandler", "address", address)
			continue
		}

		info, err := buildAccountInfo(ep.addressPubkeyConverter, ep.kappsController, userAccount, blockTimestamp)
		if err != nil {
			log.Warn("eventsProcessor.dispatchAccountEventsFromAlteredAccounts: cannot build account info", "address", address, "error", err)
			continue
		}
		accountsMap[info.Address] = info
	}

	if len(accountsMap) == 0 {
		return
	}

	dispatchAccountEvents(accountsMap)
}

func (ep *eventsProcessor) RevertIndexedBlock(header nodeData.HeaderHandler) {
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.RevertIndexedBlock(header)
}

func (ep *eventsProcessor) SaveValidatorsRating(validators []kapp.ValidatorAccountInfoHandler) {
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SavePeersAccounts(validators)
}

func (ep *eventsProcessor) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SaveEpochInfo(epoch, validators)
}

func (ep *eventsProcessor) UpdateProposalsAndParameters(proposalIDs []string) {
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.UpdateProposalsAndParameters(proposalIDs)
}

func (ep *eventsProcessor) SaveAccounts(blockTimestamp int64, acc []dataState.UserAccountHandler) {
	// Websocket dispatch is independent of indexer enablement — fire whenever
	// the event queue is on. The duplicate dispatch in elasticProcessor.saveAccounts
	// has been removed so this is the single source of ACCOUNTS events.
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

	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SaveAccounts(blockTimestamp, acc)
}

func (ep *eventsProcessor) IsInterfaceNil() bool {
	return ep == nil
}
