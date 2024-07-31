package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/update"
)

var _ update.StateSyncer = (*syncState)(nil)
var log = logger.GetOrCreate("update/genesis")
var defaultIntervalToPrintStatus = time.Second * 20

type syncState struct {
	syncingEpoch uint32

	headers      update.HeaderSyncHandler
	tries        update.EpochStartTriesSyncHandler
	transactions update.PendingTransactionsSyncHandler
}

// ArgsNewSyncState defines the arguments for the new sync state
type ArgsNewSyncState struct {
	Headers      update.HeaderSyncHandler
	Tries        update.EpochStartTriesSyncHandler
	Transactions update.PendingTransactionsSyncHandler
}

// NewSyncState creates a complete syncer which saves the state of the blockchain with pending values as well
func NewSyncState(args ArgsNewSyncState) (*syncState, error) {
	if check.IfNil(args.Headers) {
		return nil, update.ErrNilHeaderSyncHandler
	}
	if check.IfNil(args.Tries) {
		return nil, update.ErrNilTrieSyncers
	}
	if check.IfNil(args.Transactions) {
		return nil, update.ErrNilTransactionsSyncHandler
	}

	ss := &syncState{
		tries:        args.Tries,
		transactions: args.Transactions,
		headers:      args.Headers,
		syncingEpoch: 0,
	}

	return ss, nil
}

// SyncAllState gets an epoch number and will sync the complete data for that epoch start metablock
func (ss *syncState) SyncAllState(epoch uint32) error {
	ctxDisplay, cancelDisplay := context.WithCancel(context.Background())
	go displayStatusMessage("getting epoch start metablock", ctxDisplay)
	meta, err := ss.headers.GetEpochStartMetaBlock(epoch)
	cancelDisplay()
	if err != nil {
		return fmt.Errorf("%w in syncState.SyncAllState - GetEpochStartMetaBlock for epoch %d", err, epoch)
	}

	ss.printMetablockInfo(meta)

	ss.syncingEpoch = meta.GetEpoch()

	wg := sync.WaitGroup{}
	wg.Add(2)

	var errFound error
	mutErr := sync.Mutex{}

	go func() {
		errSync := ss.tries.SyncTriesFrom(meta)
		if errSync != nil {
			mutErr.Lock()
			errFound = fmt.Errorf("%w in syncState.SyncAllState - SyncTriesFrom", errSync)
			mutErr.Unlock()
		}
		wg.Done()
	}()

	go func() {
		defer wg.Done()

		ctxDisplay, cancelDisplay = context.WithCancel(context.Background())

		go displayStatusMessage(fmt.Sprintf("syncing pending transactions for epoch %d", epoch), ctxDisplay)
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		errSync := ss.transactions.SyncPendingTransactions(meta, ss.syncingEpoch, ctx)
		cancelDisplay()
		cancel()
		if errSync != nil {
			mutErr.Lock()
			errFound = fmt.Errorf("%w in syncState.SyncAllState - SyncPendingTransactions", errSync)
			mutErr.Unlock()
			return
		}
	}()

	wg.Wait()

	if errFound != nil {
		log.Error("sync data process finished with error", "error", errFound)
	} else {
		log.Info("sync data process finished successfully")
	}

	return errFound
}

func displayStatusMessage(message string, ctx context.Context) {
	log.Info(message, "status", "syncing...please wait")
	for {
		select {
		case <-time.After(defaultIntervalToPrintStatus):
			log.Info(message, "status", "syncing...please wait")

		case <-ctx.Done():
			log.Info(message, "status", "done")
			return
		}
	}
}

func (ss *syncState) printMetablockInfo(metaBlock *block.Block) {
	log.Info("epoch start meta block",
		"nonce", metaBlock.Header.Nonce,
		"slot", metaBlock.Header.Slot,
		"root hash", metaBlock.Header.TrieRoot,
		"epoch", metaBlock.Header.Epoch,
	)
}

// GetEpochStartMetaBlock returns the synced metablock
func (ss *syncState) GetEpochStartMetaBlock(epoch uint32) (*block.Block, error) {
	return ss.headers.GetEpochStartMetaBlock(epoch)
}

// GetAllTries returns the synced tries
func (ss *syncState) GetAllTries() (map[string]data.Trie, error) {
	return ss.tries.GetTries()
}

// GetAllTransactions returns the synced transactions
func (ss *syncState) GetAllTransactions() (map[string]data.TransactionHandler, error) {
	return ss.transactions.GetTransactions()
}

// IsInterfaceNil returns if underlying objects in nil
func (ss *syncState) IsInterfaceNil() bool {
	return ss == nil
}
