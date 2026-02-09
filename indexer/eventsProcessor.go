package indexer

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
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
	parser  *dataParser
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

func (ep *eventsProcessor) SaveBlock(args *indexerData.ArgsSaveBlockData) {
	ep.dispatchWebsocketEvents(args)
	ep.dispatchToIndexer(args)
}

func (ep *eventsProcessor) dispatchWebsocketEvents(args *indexerData.ArgsSaveBlockData) {
	if !UseEventQueue {
		return
	}

	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		ep.dispatchBlockEvent(args)
		ep.processTransactionEvents(args.Header, args.TransactionsPool)
	}
}

func (ep *eventsProcessor) dispatchBlockEvent(args *indexerData.ArgsSaveBlockData) {
	txsSize := 0
	if args.TransactionsPool != nil {
		txsSize = len(args.TransactionsPool.Txs)
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

	trySendEvent(Event{
		EvType:  USER_TRANSACTION,
		Message: txs,
	})
	trySendEvent(Event{
		EvType:  TRANSACTION,
		Message: txs,
	})
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

		_ = ep.DecodeContract(dbTx, tx, nil, nil, nil, header.GetTimestamp())

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
	if UseEventQueue && len(acc) > 0 && (ep.indexer == nil || ep.indexer.IsNilIndexer()) {
		accountsMap := make(map[string]*data.AccountInfo, len(acc))
		for _, userAccount := range acc {
			address := ep.addressPubkeyConverter.Encode(userAccount.AddressBytes())

			userKDA, err := userAccount.GetUserKDA(kdautils.KLVIdentifier, nil, true)
			if err != nil {
				log.Warn("eventsProcessor.SaveAccounts", "error", err.Error())
				continue
			}

			accountsMap[address] = &data.AccountInfo{
				Address:         address,
				Nonce:           userAccount.GetNonce(),
				Name:            string(userAccount.GetName()),
				RootHash:        hex.EncodeToString(userAccount.GetRootHash()),
				Balance:         userAccount.GetBalance(kdautils.KLVIdentifier, true),
				FrozenBalance:   userKDA.FrozenBalance,
				UnfrozenBalance: calculateUnfrozenBalance(userKDA.Buckets),
				Allowance:       userAccount.GetAllowance(),
				Permissions:     ep.convertPermissions(userAccount.GetPermissions()),
				Timestamp:       toMilliseconds(blockTimestamp),
				UpdatedAt:       toMilliseconds(blockTimestamp),
				CodeHash:        hex.EncodeToString(userAccount.GetCodeHash()),
				CodeMetadata:    hex.EncodeToString(userAccount.GetCodeMetadata()),
			}
		}
		dispatchAccountEvents(accountsMap)
	}

	if ep.indexer == nil || ep.indexer.IsNilIndexer() {
		return
	}
	ep.indexer.SaveAccounts(blockTimestamp, acc)
}

func (ep *eventsProcessor) convertPermissions(perms []*dataState.Permission) []data.Permissions {
	permissions := make([]data.Permissions, 0, len(perms))
	for _, p := range perms {
		keys := make([]data.PermissionKey, 0, len(p.Signers))
		for _, k := range p.Signers {
			keys = append(keys, data.PermissionKey{
				Address: ep.addressPubkeyConverter.Encode(k.Address),
				Weight:  k.Weight,
			})
		}
		permissions = append(permissions, data.Permissions{
			ID:             p.ID,
			Type:           int32(p.Type),
			PermissionName: p.PermissionName,
			Threshold:      p.Threshold,
			Operations:     hex.EncodeToString(p.Operations),
			Signers:        keys,
		})
	}
	return permissions
}

func (ep *eventsProcessor) IsInterfaceNil() bool {
	return ep == nil
}
