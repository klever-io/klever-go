package sync

import (
	"context"
	"math"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// MetaBootstrap implements the bootstrap mechanism
type MetaBootstrap struct {
	*baseBootstrap
	epochBootstrapper process.EpochBootstrapper
}

// NewMetaBootstrap creates a new Bootstrap object
func NewMetaBootstrap(arguments ArgMetaBootstrapper) (*MetaBootstrap, error) {
	if check.IfNil(arguments.PoolsHolder) {
		return nil, common.ErrNilPoolsHolder
	}
	if check.IfNil(arguments.PoolsHolder.Headers()) {
		return nil, common.ErrNilBlocksPool
	}
	if check.IfNil(arguments.SlotManager) {
		return nil, process.ErrNilSlotManager
	}
	if check.IfNil(arguments.EpochBootstrapper) {
		return nil, common.ErrNilEpochStartTrigger
	}

	err := checkBootstrapNilParameters(arguments.ArgBaseBootstrapper)
	if err != nil {
		return nil, err
	}

	base := &baseBootstrap{
		chainHandler:        arguments.ChainHandler,
		blockProcessor:      arguments.BlockProcessor,
		store:               arguments.Store,
		headers:             arguments.PoolsHolder.Headers(),
		txPool:              arguments.PoolsHolder.Transactions(),
		slotManager:         arguments.SlotManager,
		waitTime:            arguments.WaitTime,
		hasher:              arguments.Hasher,
		marshalizer:         arguments.Marshalizer,
		forkDetector:        arguments.ForkDetector,
		requestHandler:      arguments.RequestHandler,
		accounts:            arguments.Accounts,
		blackListHandler:    arguments.BlackListHandler,
		networkWatcher:      arguments.NetworkWatcher,
		bootStorer:          arguments.BootStorer,
		storageBootstrapper: arguments.StorageBootstrapper,
		//epochHandler:    arguments.EpochHandler,
		uint64Converter: arguments.Uint64Converter,
		indexer:         arguments.Indexer,
		isInImportMode:  arguments.IsInImportMode,
		hasStarted:      arguments.SlotManager.BeforeGenesis() || arguments.IsInImportMode || arguments.StartWithInSync,
	}
	// Sentinel below any real slot index, mirroring the fork detector's warning
	// throttle: the zero value would equal slot index 0 and swallow the first
	// burst there.
	base.lastStuckRequestSlot.Store(math.MinInt64)

	if base.hasStarted {
		log.Warn("node starting inSync state")
	}

	if base.isInImportMode {
		log.Warn("using always-not-synced status because the node is running in import-db")
	}

	boot := MetaBootstrap{
		baseBootstrap:     base,
		epochBootstrapper: arguments.EpochBootstrapper,
	}

	base.blockBootstrapper = &boot
	base.syncStarter = &boot
	base.getHeaderFromPool = boot.getHeaderFromPool
	base.requestBlockTXs = boot.requestBlockTXsFromHeaderWithNonceIfMissing

	//placed in struct fields for performance reasons
	base.headerStore = boot.store.GetStorer(retriever.BlockUnit)
	base.headerNonceHashStore = boot.store.GetStorer(retriever.HdrNonceHashDataUnit)

	base.init()

	return &boot, nil
}

// LoadStorage method will load the storage
func (boot *MetaBootstrap) LoadStorage() {
	// when a node starts it first tries to bootstrap from storage, if there already exist a database saved
	errNotCritical := boot.storageBootstrapper.LoadFromStorage()
	if errNotCritical != nil {
		log.Debug("syncFromStorer", "error", errNotCritical.Error())
	} else {
		numTxs, _ := updateMetricsFromStorage(boot.store, boot.uint64Converter, boot.marshalizer, boot.statusHandler, boot.storageBootstrapper.GetHighestBlockNonce())
		boot.blockProcessor.SetNumProcessedObj(numTxs)
		boot.setLastEpochStartSlot()
	}
}

func (boot *MetaBootstrap) LoadSyncUntil(syncUntil uint32) {
	boot.syncUntil = syncUntil
}

func (boot *MetaBootstrap) LoadStopNodeChannel(chanStopNodeProcess chan endProcess.ArgEndProcess) {
	boot.chanStopNodeProcess = chanStopNodeProcess
}

// StartSyncingBlocks method will start syncing blocks as a go routine
func (boot *MetaBootstrap) StartSyncingBlocks() {
	var ctx context.Context
	ctx, boot.cancelFunc = context.WithCancel(context.Background())
	go boot.syncBlocks(ctx)
}

func (boot *MetaBootstrap) setLastEpochStartSlot() {
	hdr := boot.chainHandler.GetCurrentBlockHeader()
	if check.IfNil(hdr) || hdr.GetEpoch() < 1 {
		return
	}

	epochIdentifier := core.EpochStartIdentifier(hdr.GetEpoch())
	epochStartHdr, err := boot.headerStore.Get([]byte(epochIdentifier))
	if err != nil {
		return
	}

	epochStartMetaBlock := &block.Block{}
	err = boot.marshalizer.Unmarshal(epochStartMetaBlock, epochStartHdr)
	if err != nil {
		return
	}

	boot.epochBootstrapper.SetCurrentEpochStartSlot(epochStartMetaBlock.GetSlot())
}

// SyncBlock method actually does the synchronization. It requests the next block header from the pool
// and if it is not found there it will be requested from the network. After the header is received,
// it requests the block body in the same way(pool and than, if it is not found in the pool, from network).
// If either header and body are received the ProcessBlock and CommitBlock method will be called successively.
// These methods will execute the block and its transactions. Finally if everything works, the block will be committed
// in the blockchain, and all this mechanism will be reiterated for the next block.
func (boot *MetaBootstrap) SyncBlock() error {
	return boot.syncBlock()
}

// requestHeaderWithNonce method requests a block header from network when it is not found in the pool
func (boot *MetaBootstrap) requestHeaderWithNonce(nonce uint64) {
	boot.setRequestedHeaderNonce(&nonce)
	log.Debug("requesting meta header from network",
		"nonce", nonce,
		"probable highest nonce", boot.forkDetector.ProbableHighestNonce(),
	)
	boot.requestHandler.RequestHeaderByNonce(nonce)
}

// requestHeaderWithHash method requests a block header from network when it is not found in the pool
func (boot *MetaBootstrap) requestHeaderWithHash(hash []byte) {
	boot.setRequestedHeaderHash(hash)
	log.Debug("requesting meta header from network",
		"hash", hash,
		"probable highest nonce", boot.forkDetector.ProbableHighestNonce(),
	)
	boot.requestHandler.RequestHeader(hash)
}

// getHeaderWithNonceRequestingIfMissing method gets the header with a given nonce from pool. If it is not found there, it will
// be requested from network
func (boot *MetaBootstrap) getHeaderWithNonceRequestingIfMissing(nonce uint64) (data.HeaderHandler, error) {
	hdr, _, err := process.GetHeaderFromPoolWithNonce(
		nonce,
		boot.headers)
	if err != nil {
		_ = tools.EmptyChannel(boot.chRcvHdrNonce)
		boot.requestHeaderWithNonce(nonce)
		err = boot.waitForHeaderNonce()
		if err != nil {
			return nil, err
		}

		hdr, _, err = process.GetHeaderFromPoolWithNonce(
			nonce,
			boot.headers)
		if err != nil {
			return nil, err
		}
	}

	return hdr, nil
}

// getHeaderWithHashRequestingIfMissing method gets the header with a given hash from pool. If it is not found there,
// it will be requested from network
func (boot *MetaBootstrap) getHeaderWithHashRequestingIfMissing(hash []byte) (data.HeaderHandler, error) {
	hdr, err := process.GetHeader(hash, boot.headers, boot.marshalizer, boot.store)
	if err != nil {
		_ = tools.EmptyChannel(boot.chRcvHdrHash)
		boot.requestHeaderWithHash(hash)
		err = boot.waitForHeaderHash()
		if err != nil {
			return nil, err
		}

		hdr, err = process.GetHeaderFromPool(hash, boot.headers)
		if err != nil {
			return nil, err
		}
	}

	return hdr, nil
}

func (boot *MetaBootstrap) getPrevHeader(
	header data.HeaderHandler,
	headerStore storage.Storer,
) (data.HeaderHandler, error) {

	prevHash := header.GetParentHash()
	buffHeader, err := headerStore.Get(prevHash)
	if err != nil {
		return nil, err
	}

	prevHeader := &block.Block{}
	err = boot.marshalizer.Unmarshal(prevHeader, buffHeader)
	if err != nil {
		return nil, err
	}

	return prevHeader, nil
}

func (boot *MetaBootstrap) getCurrHeader() (data.HeaderHandler, error) {
	blockHeader := boot.chainHandler.GetCurrentBlockHeader()
	if check.IfNil(blockHeader) {
		return nil, process.ErrNilBlockHeader
	}

	header, ok := blockHeader.(*block.Block)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	return header, nil
}

func (boot *MetaBootstrap) haveHeaderInPoolWithNonce(nonce uint64) bool {
	_, _, err := process.GetHeaderFromPoolWithNonce(
		nonce,
		boot.headers)

	return err == nil
}

func (boot *MetaBootstrap) getHeaderFromPool(headerHash []byte) (data.HeaderHandler, error) {
	return process.GetHeaderFromPool(headerHash, boot.headers)
}

func (boot *MetaBootstrap) requestBlockTXsFromHeaderWithNonceIfMissing(headerHandler data.HeaderHandler) {
	nextBlockNonce := boot.getNonceForNextBlock()
	maxNonce := tools.MinUint64(nextBlockNonce+process.MaxHeadersToRequestInAdvance-1, boot.forkDetector.ProbableHighestNonce())
	if headerHandler.GetNonce() < nextBlockNonce || headerHandler.GetNonce() > maxNonce {
		return
	}

	missingTXs := make([][]byte, 0)
	for _, txHash := range headerHandler.GetTxHashes() {
		err := process.CheckIfInTxPool(txHash, boot.txPool)
		if err != nil {
			missingTXs = append(missingTXs, txHash)
		}
	}

	boot.requestHandler.RequestTransaction(missingTXs)
}

func (boot *MetaBootstrap) isForkTriggeredByMeta() bool {
	return false
}

func (boot *MetaBootstrap) requestHeaderByNonce(nonce uint64) {
	boot.requestHandler.RequestHeaderByNonce(nonce)
}
