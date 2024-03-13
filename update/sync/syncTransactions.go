package sync

import (
	"context"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/update"
)

var _ update.PendingTransactionsSyncHandler = (*pendingTransactions)(nil)

type pendingTransactions struct {
	mutPendingTx            sync.Mutex
	mapTransactions         map[string]data.TransactionHandler
	mapHashes               map[string]*block.Block
	txPools                 retriever.ShardedDataCacherNotifier
	storage                 update.HistoryStorer
	chReceivedAll           chan bool
	requestHandler          process.RequestHandler
	marshalizer             marshal.Marshalizer
	epochToSync             uint32
	stopSync                bool
	syncedAll               bool
	waitTimeBetweenRequests time.Duration
}

// ArgsNewPendingTransactionsSyncer defines the arguments needed for a new transactions syncer
type ArgsNewPendingTransactionsSyncer struct {
	DataPools      retriever.PoolsHolder
	Storages       retriever.StorageService
	Marshalizer    marshal.Marshalizer
	RequestHandler process.RequestHandler
}

// NewPendingTransactionsSyncer creates a new transactions syncer
func NewPendingTransactionsSyncer(args ArgsNewPendingTransactionsSyncer) (*pendingTransactions, error) {
	if check.IfNil(args.Storages) {
		return nil, common.ErrNilHeadersStorage
	}
	if check.IfNil(args.DataPools) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.RequestHandler) {
		return nil, common.ErrNilRequestHandler
	}

	p := &pendingTransactions{
		mutPendingTx:            sync.Mutex{},
		mapTransactions:         make(map[string]data.TransactionHandler),
		mapHashes:               make(map[string]*block.Block),
		chReceivedAll:           make(chan bool),
		requestHandler:          args.RequestHandler,
		marshalizer:             args.Marshalizer,
		stopSync:                true,
		syncedAll:               true,
		waitTimeBetweenRequests: args.RequestHandler.RequestInterval(),
	}

	p.txPools = args.DataPools.Transactions()
	p.storage = args.Storages.GetStorer(retriever.TransactionUnit)

	p.txPools.RegisterOnAdded(p.receivedTransaction)

	return p, nil
}

// SyncPendingTransactions syncs pending transactions for a list of miniblocks
func (p *pendingTransactions) SyncPendingTransactions(b *block.Block, epoch uint32, ctx context.Context) error {
	_ = tools.EmptyChannel(p.chReceivedAll)

	for {
		p.mutPendingTx.Lock()
		p.epochToSync = epoch
		p.syncedAll = false
		p.stopSync = false

		requestedTxs := 0

		for _, txHash := range b.TxHashes {
			p.mapHashes[string(txHash)] = b
		}
		requestedTxs += p.requestTransactionsFor(b)

		p.mutPendingTx.Unlock()

		if requestedTxs == 0 {
			p.mutPendingTx.Lock()
			p.stopSync = true
			p.syncedAll = true
			p.mutPendingTx.Unlock()
			return nil
		}

		select {
		case <-p.chReceivedAll:
			p.mutPendingTx.Lock()
			p.stopSync = true
			p.syncedAll = true
			p.mutPendingTx.Unlock()
			return nil
		case <-time.After(p.waitTimeBetweenRequests):
			continue
		case <-ctx.Done():
			p.mutPendingTx.Lock()
			p.stopSync = true
			p.mutPendingTx.Unlock()
			return common.ErrTimeIsOut
		}
	}
}

func (p *pendingTransactions) requestTransactionsFor(miniBlock *block.Block) int {
	missingTxs := make([][]byte, 0)
	for _, txHash := range miniBlock.TxHashes {
		if _, ok := p.mapTransactions[string(txHash)]; ok {
			continue
		}

		tx, ok := p.getTransactionFromPoolOrStorage(txHash)
		if ok {
			p.mapTransactions[string(txHash)] = tx
			continue
		}

		missingTxs = append(missingTxs, txHash)
	}

	for _, txHash := range missingTxs {
		p.mapHashes[string(txHash)] = miniBlock
	}

	p.requestHandler.RequestTransaction(missingTxs)

	return len(missingTxs)
}

// receivedMiniBlock is a callback function when a new transactions was received
func (p *pendingTransactions) receivedTransaction(txHash []byte, val interface{}) {
	p.mutPendingTx.Lock()
	if p.stopSync {
		p.mutPendingTx.Unlock()
		return
	}

	if _, ok := p.mapHashes[string(txHash)]; !ok {
		p.mutPendingTx.Unlock()
		return
	}
	if _, ok := p.mapTransactions[string(txHash)]; ok {
		p.mutPendingTx.Unlock()
		return
	}

	tx, ok := val.(data.TransactionHandler)
	if !ok {
		p.mutPendingTx.Unlock()
		return
	}

	p.mapTransactions[string(txHash)] = tx
	receivedAllMissing := len(p.mapHashes) == len(p.mapTransactions)
	p.mutPendingTx.Unlock()

	if receivedAllMissing {
		p.chReceivedAll <- true
	}
}

func (p *pendingTransactions) getTransactionFromPool(txHash []byte) (data.TransactionHandler, bool) {
	storeId := "0"
	shardTxStore := p.txPools.ShardDataStore(storeId)
	if check.IfNil(shardTxStore) {
		return nil, false
	}

	val, ok := shardTxStore.Peek(txHash)
	if !ok {
		return nil, false
	}

	tx, ok := val.(data.TransactionHandler)
	if !ok {
		return nil, false
	}

	return tx, true
}

func (p *pendingTransactions) getTransactionFromPoolOrStorage(hash []byte) (data.TransactionHandler, bool) {
	txFromPool, ok := p.getTransactionFromPool(hash)
	if ok {
		return txFromPool, true
	}

	txData, err := GetDataFromStorage(hash, p.storage)
	if err != nil {
		return nil, false
	}

	tx := &transaction.Transaction{}
	err = p.marshalizer.Unmarshal(tx, txData)
	if err != nil {
		return nil, false
	}

	return tx, true
}

// GetTransactions returns the synced transactions
func (p *pendingTransactions) GetTransactions() (map[string]data.TransactionHandler, error) {
	p.mutPendingTx.Lock()
	defer p.mutPendingTx.Unlock()
	if !p.syncedAll {
		return nil, update.ErrNotSynced
	}

	return p.mapTransactions, nil
}

// IsInterfaceNil returns true if underlying object is nil
func (p *pendingTransactions) IsInterfaceNil() bool {
	return p == nil
}
