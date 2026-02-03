package indexer

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	indexerData "github.com/klever-io/klever-go/data/indexer"
	dataState "github.com/klever-io/klever-go/data/state"
	dataTx "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/tools/check"
)

type eventsProcessor struct {
	*txDatabaseProcessor
	indexer Indexer
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

func (ep *eventsProcessor) SaveBlock(args *indexerData.ArgsSaveBlockData) {
	ep.dispatchWebsocketEvents(args.Header, args.TransactionsPool)
	ep.dispatchToIndexer(args)
}

func (ep *eventsProcessor) dispatchWebsocketEvents(header nodeData.HeaderHandler, pool *indexerData.Pool) {
	if !UseEventQueue {
		return
	}

	serializedBlock, err := ep.marshalizer.Marshal(header.GetBlockHeader())
	if err != nil {
		log.Warn("eventsProcessor.dispatchWebsocketEvents", "could not marshal block", err.Error())
		return
	}

	EventQueue <- Event{
		EvType:  BLOCKS,
		Message: serializedBlock,
	}

	ep.processTransactionEvents(header, pool)
}

func (ep *eventsProcessor) dispatchToIndexer(args *indexerData.ArgsSaveBlockData) {
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SaveBlock(args)
}

func (ep *eventsProcessor) processTransactionEvents(header nodeData.HeaderHandler, pool *indexerData.Pool) {
	if pool == nil || len(pool.Txs) == 0 {
		return
	}

	txs := ep.prepareTransactionsForEvents(header, pool)
	if len(txs) == 0 {
		return
	}

	EventQueue <- Event{
		EvType:  USER_TRANSACTION,
		Message: txs,
	}
	EventQueue <- Event{
		EvType:  TRANSACTION,
		Message: txs,
	}
}

func (ep *eventsProcessor) prepareTransactionsForEvents(header nodeData.HeaderHandler, pool *indexerData.Pool) []*data.Transaction {
	txs := make([]*data.Transaction, 0, len(pool.Txs))

	for txHash, txHandler := range pool.Txs {
		tx, ok := txHandler.(*dataTx.Transaction)
		if !ok {
			continue
		}

		hash := hex.EncodeToString([]byte(txHash))
		dbTx := ep.BuildTransaction(tx, hash, header)
		if dbTx == nil {
			continue
		}

		txs = append(txs, dbTx)
	}

	return txs
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
	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SaveAccounts(blockTimestamp, acc)
}

func (ep *eventsProcessor) ProcessAccountEvents(accountsInfo map[string]*data.AccountInfo) {
	if !UseEventQueue || len(accountsInfo) == 0 {
		return
	}

	EventQueue <- Event{
		EvType:  ACCOUNTS,
		Message: accountsInfo,
	}
}

func (ep *eventsProcessor) IsInterfaceNil() bool {
	return ep == nil
}
