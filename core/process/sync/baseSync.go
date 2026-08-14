package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/closing"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

var log = logger.GetOrCreate("process/sync")

var _ closing.Closer = (*baseBootstrap)(nil)

// sleepTime defines the time in milliseconds between each iteration made in syncBlocks method
const sleepTime = 5 * time.Millisecond

// HdrInfo hold the data related to a header
type HdrInfo struct {
	Nonce uint64
	Hash  []byte
}

type notarizedInfo struct {
	lastNotarized           map[uint32]*HdrInfo
	finalNotarized          map[uint32]*HdrInfo
	blockWithLastNotarized  map[uint32]uint64
	blockWithFinalNotarized map[uint32]uint64
	startNonce              uint64
}

type baseBootstrap struct {
	headers retriever.HeadersPool
	txPool  retriever.ShardedDataCacherNotifier

	chainHandler   data.ChainHandler
	blockProcessor process.BlockProcessor
	store          retriever.StorageService

	slotManager       consensus.SlotManager
	hasher            hashing.Hasher
	marshalizer       marshal.Marshalizer
	forkDetector      process.ForkDetector
	requestHandler    process.RequestHandler
	accounts          state.AccountsAdapter
	blockBootstrapper blockBootstrapper
	blackListHandler  process.TimeCacher

	mutHeader     sync.RWMutex
	headerNonce   *uint64
	headerhash    []byte
	chRcvHdrNonce chan bool
	chRcvHdrHash  chan bool

	requestedHashes process.RequiredDataPool

	statusHandler core.AppStatusHandler

	waitTime time.Duration

	mutNodeState          sync.RWMutex
	isNodeSynchronized    bool
	isNodeStateCalculated bool
	hasLastBlock          bool
	slotIndex             int64

	forkInfo *process.ForkInfo

	mutRcvHdrNonce           sync.RWMutex
	mutRcvHdrHash            sync.RWMutex
	syncStateListeners       []func(bool)
	mutSyncStateListeners    sync.RWMutex
	uint64Converter          typeConverters.Uint64ByteSliceConverter
	mapNonceSyncedWithErrors map[uint64]uint32
	mutNonceSyncedWithErrors sync.RWMutex

	requestBlockTXs func(headerHandler data.HeaderHandler)

	networkWatcher    process.NetworkConnectionWatcher
	getHeaderFromPool func([]byte) (data.HeaderHandler, error)

	headerStore          storage.Storer
	headerNonceHashStore storage.Storer
	syncStarter          syncStarter
	bootStorer           process.BootStorer
	storageBootstrapper  process.BootstrapperFromStorage

	lastRollbackHash               []byte
	avoidingSameRollbackRetryCount int

	indexer           process.Indexer
	isInImportMode    bool
	mutRequestHeaders sync.Mutex
	// lastStuckRequestSlot caps the stuck-recovery burst and its warning to once
	// per slot, regardless of how often the trigger path runs.
	lastStuckRequestSlot atomic.Int64
	cancelFunc           func()
	hasStarted           bool

	// For Backtest Purpose
	chanStopNodeProcess chan endProcess.ArgEndProcess
	syncUntil           uint32
	epochToEndFinished  bool
}

// setRequestedHeaderNonce method sets the header nonce requested by the sync mechanism
func (boot *baseBootstrap) setRequestedHeaderNonce(nonce *uint64) {
	boot.mutHeader.Lock()
	boot.headerNonce = nonce
	boot.mutHeader.Unlock()
}

// setRequestedHeaderHash method sets the header hash requested by the sync mechanism
func (boot *baseBootstrap) setRequestedHeaderHash(hash []byte) {
	boot.mutHeader.Lock()
	boot.headerhash = hash
	boot.mutHeader.Unlock()
}

// requestedHeaderNonce method gets the header nonce requested by the sync mechanism
func (boot *baseBootstrap) requestedHeaderNonce() *uint64 {
	boot.mutHeader.RLock()
	defer boot.mutHeader.RUnlock()
	return boot.headerNonce
}

// requestedHeaderHash method gets the header hash requested by the sync mechanism
func (boot *baseBootstrap) requestedHeaderHash() []byte {
	boot.mutHeader.RLock()
	defer boot.mutHeader.RUnlock()
	return boot.headerhash
}

func (boot *baseBootstrap) processReceivedHeader(headerHandler data.HeaderHandler, headerHash []byte) {
	log.Trace("sync received header from network",
		"slot", headerHandler.GetSlot(),
		"nonce", headerHandler.GetNonce(),
		"hash", headerHash,
	)

	if headerHandler.GetSlot() > tools.SafeI64ToU64(boot.slotManager.Index()) {
		log.Trace("Skipping sync received header due to slot mismatch",
			"slot", headerHandler.GetSlot(),
			"slotManager.Index", boot.slotManager.Index(),
		)
		return
	}

	err := boot.forkDetector.AddHeader(headerHandler, headerHash, process.BHReceived, nil, nil)
	if err != nil {
		log.Debug("forkDetector.AddHeader", "error", err.Error())
	}

	// request block TXS on sync
	go boot.requestBlockTXs(headerHandler)

	boot.confirmHeaderReceivedByNonce(headerHandler, headerHash)
	boot.confirmHeaderReceivedByHash(headerHandler, headerHash)
}

func (boot *baseBootstrap) confirmHeaderReceivedByNonce(headerHandler data.HeaderHandler, hdrHash []byte) {
	boot.mutRcvHdrNonce.Lock()
	n := boot.requestedHeaderNonce()
	if n != nil && *n == headerHandler.GetNonce() {
		log.Debug("received requested header from network",
			"slot", headerHandler.GetSlot(),
			"nonce", headerHandler.GetNonce(),
			"hash", hdrHash,
		)
		boot.setRequestedHeaderNonce(nil)
		boot.mutRcvHdrNonce.Unlock()
		boot.chRcvHdrNonce <- true

		return
	}

	boot.mutRcvHdrNonce.Unlock()
}

func (boot *baseBootstrap) confirmHeaderReceivedByHash(headerHandler data.HeaderHandler, hdrHash []byte) {
	boot.mutRcvHdrHash.Lock()
	hash := boot.requestedHeaderHash()
	if hash != nil && bytes.Equal(hash, hdrHash) {
		log.Debug("received requested header from network",
			"slot", headerHandler.GetSlot(),
			"nonce", headerHandler.GetNonce(),
			"hash", hash,
		)
		boot.setRequestedHeaderHash(nil)
		boot.mutRcvHdrHash.Unlock()
		boot.chRcvHdrHash <- true

		return
	}
	boot.mutRcvHdrHash.Unlock()
}

// AddSyncStateListener adds a syncStateListener that get notified each time the sync status of the node changes
func (boot *baseBootstrap) AddSyncStateListener(syncStateListener func(isSyncing bool)) {
	boot.mutSyncStateListeners.Lock()
	boot.syncStateListeners = append(boot.syncStateListeners, syncStateListener)
	boot.mutSyncStateListeners.Unlock()
}

// SetStatusHandler will set the instance of the AppStatusHandler
func (boot *baseBootstrap) SetStatusHandler(handler core.AppStatusHandler) error {
	if handler == nil || handler.IsInterfaceNil() {
		return common.ErrNilAppStatusHandler
	}
	boot.statusHandler = handler

	return nil
}

func (boot *baseBootstrap) notifySyncStateListeners(isNodeSynchronized bool) {
	boot.mutSyncStateListeners.RLock()
	for i := 0; i < len(boot.syncStateListeners); i++ {
		go boot.syncStateListeners[i](isNodeSynchronized)
	}
	boot.mutSyncStateListeners.RUnlock()
}

// getNonceForNextBlock will get the nonce for the next block
func (boot *baseBootstrap) getNonceForNextBlock() uint64 {
	nonce := boot.chainHandler.GetGenesisHeader().GetNonce() + 1 // first block nonce after genesis block
	currentBlockHeader := boot.chainHandler.GetCurrentBlockHeader()
	if !check.IfNil(currentBlockHeader) {
		nonce = currentBlockHeader.GetNonce() + 1
	}
	return nonce
}

// getNonceForCurrentBlock will get the nonce for the current block
func (boot *baseBootstrap) getNonceForCurrentBlock() uint64 {
	nonce := boot.chainHandler.GetGenesisHeader().GetNonce() // genesis block nonce
	currentBlockHeader := boot.chainHandler.GetCurrentBlockHeader()
	if !check.IfNil(currentBlockHeader) {
		nonce = currentBlockHeader.GetNonce()
	}
	return nonce
}

// waitForHeaderNonce method wait for header with the requested nonce to be received
func (boot *baseBootstrap) waitForHeaderNonce() error {
	select {
	case <-boot.chRcvHdrNonce:
		return nil
	case <-time.After(boot.waitTime):
		return process.ErrTimeIsOut
	}
}

// waitForHeaderHash method wait for header with the requested hash to be received
func (boot *baseBootstrap) waitForHeaderHash() error {
	select {
	case <-boot.chRcvHdrHash:
		return nil
	case <-time.After(boot.waitTime):
		return process.ErrTimeIsOut
	}
}

func (boot *baseBootstrap) computeNodeState() {
	boot.mutNodeState.Lock()
	defer boot.mutNodeState.Unlock()

	isNodeStateCalculatedInCurrentSlot := boot.slotIndex == boot.slotManager.Index() && boot.isNodeStateCalculated
	if isNodeStateCalculatedInCurrentSlot {
		return
	}

	boot.forkInfo = boot.forkDetector.CheckFork()

	genesisNonce := boot.chainHandler.GetGenesisHeader().GetNonce()
	currentHeader := boot.chainHandler.GetCurrentBlockHeader()
	lastNonce := genesisNonce
	lastSlot := boot.chainHandler.GetGenesisHeader().GetSlot()
	if check.IfNil(currentHeader) {
		boot.hasLastBlock = boot.forkDetector.ProbableHighestNonce() == genesisNonce
		log.Debug("computeNodeState",
			"probableHighestNonce", boot.forkDetector.ProbableHighestNonce(),
			"currentBlockNonce", nil,
			"boot.hasLastBlock", boot.hasLastBlock)
	} else {
		lastNonce = currentHeader.GetNonce()
		lastSlot = currentHeader.GetSlot()
		boot.hasLastBlock = boot.forkDetector.ProbableHighestNonce() <= boot.chainHandler.GetCurrentBlockHeader().GetNonce()
		log.Debug("computeNodeState",
			"probableHighestNonce", boot.forkDetector.ProbableHighestNonce(),
			"currentBlockNonce", boot.chainHandler.GetCurrentBlockHeader().GetNonce(),
			"boot.hasLastBlock", boot.hasLastBlock)
	}

	if !boot.hasStarted && lastSlot >= tools.SafeI64ToU64(boot.slotManager.Index()) {
		boot.hasStarted = true

		log.Info("node has started")
	}

	isNodeConnectedToTheNetwork := boot.networkWatcher.IsConnectedToTheNetwork()
	isNodeSynchronized := !boot.forkInfo.IsDetected && boot.hasLastBlock && isNodeConnectedToTheNetwork && boot.hasStarted
	if isNodeSynchronized != boot.isNodeSynchronized {
		log.Debug("node has changed its synchronized state",
			"state", isNodeSynchronized, "hasLastBlock", boot.hasLastBlock, "forkInfo.IsDetected", boot.forkInfo.IsDetected,
			"isNodeConnectedToTheNetwork", isNodeConnectedToTheNetwork, "ProbableHighestNonce",
			boot.forkDetector.ProbableHighestNonce(), "GetNonce", lastNonce,
		)
	}

	boot.isNodeSynchronized = isNodeSynchronized
	boot.isNodeStateCalculated = true
	boot.slotIndex = boot.slotManager.Index()
	boot.notifySyncStateListeners(isNodeSynchronized)

	result := uint64(1)
	if isNodeSynchronized {
		result = uint64(0)
	}

	boot.statusHandler.SetUInt64Value(core.MetricIsSyncing, result)
	log.Debug("computeNodeState",
		"isNodeStateCalculated", boot.isNodeStateCalculated,
		"isNodeSynchronized", boot.isNodeSynchronized)

	if boot.shouldTryToRequestHeaders() {
		go boot.requestHeadersIfSyncIsStuck(boot.isNodeSynchronized)
	}
}

func (boot *baseBootstrap) shouldTryToRequestHeaders() bool {
	if boot.slotManager.BeforeGenesis() {
		return false
	}
	if boot.isForcedRollBackOneBlock() {
		return false
	}
	if boot.isForcedRollBackToNonce() {
		return false
	}
	if !boot.isNodeSynchronized {
		return true
	}

	// The node believes it is synchronized, but that belief comes from the fork
	// detector, whose counters only move for headers that were accepted. While
	// header intake is blocked (for instance right after an epoch boundary, when
	// the new epoch's consensus config does not exist yet) the node sits at a
	// stale nonce and still reports itself synced. The slot lag is derived from
	// the slot manager and the last committed block instead, so it stays truthful
	// in exactly that state.
	//
	// This replaces a slot-index modulus trigger (every 20th slot) that could
	// never fire here: baseForkDetector.isConsensusStuck uses the same lag
	// threshold and fires on process.SlotModulusTrigger (5), so every slot that
	// satisfied the old modulus of 20 also produced a forced-rollback ForkInfo,
	// which the isForcedRollBackOneBlock guard above short-circuits on.
	// Requesting on the slots in between is what keeps the fork detector out of
	// that forced-rollback loop: once a requested header lands, isSyncing() turns
	// true and the next stuck check stands down.
	//
	// Import mode replays historical blocks, so the slot lag is unbounded by
	// construction and there are no peers to request from.
	if boot.isInImportMode {
		return false
	}

	// The resulting burst and its warning are capped to once per slot by
	// lastStuckRequestSlot inside requestHeadersIfSyncIsStuck, independently of
	// how often this predicate is evaluated.
	return boot.slotsSinceLastCommittedBlock() > process.MaxSlotsWithoutNewBlockReceived
}

// slotsSinceLastCommittedBlock returns how many slots have passed since the slot
// of the last committed block. It deliberately avoids the fork detector, so that
// it remains meaningful when header intake is blocked and the fork detector's
// counters are frozen.
func (boot *baseBootstrap) slotsSinceLastCommittedBlock() uint64 {
	lastSyncedSlot := boot.chainHandler.GetGenesisHeader().GetSlot()
	currHeader := boot.chainHandler.GetCurrentBlockHeader()
	if !check.IfNil(currHeader) {
		lastSyncedSlot = currHeader.GetSlot()
	}

	lag, err := tools.SafeSubUint64(tools.SafeI64ToU64(boot.slotManager.Index()), lastSyncedSlot)
	if err != nil {
		// The last committed block is ahead of our own slot index, so no slots
		// have elapsed since it. Subtracting would wrap around on uint64 and
		// report an enormous lag, which gates a per-slot request path. Debug
		// rather than warn on purpose: this can run under mutNodeState, and the
		// operator-facing warning for this state lives in
		// baseForkDetector.isConsensusStuck, which observes the same condition.
		log.Debug("last committed block is ahead of the local slot index",
			"local slot index", boot.slotManager.Index(),
			"last committed block slot", lastSyncedSlot)
		return 0
	}

	return lag
}

// requestHeadersIfSyncIsStuck fires the recovery burst once the node has not
// committed a block for more than MaxSlotsWithoutNewBlockReceived slots.
// stalledWhileSynced tells it whether the node believed it was synchronized at
// the moment the burst was decided; that state deserves a warning, because the
// node advertises NsSynchronized and MetricIsSyncing reads 0 throughout, while
// an honestly syncing node passing through here is just catching up.
func (boot *baseBootstrap) requestHeadersIfSyncIsStuck(stalledWhileSynced bool) {
	slotDiff := boot.slotsSinceLastCommittedBlock()
	if slotDiff <= process.MaxSlotsWithoutNewBlockReceived {
		return
	}

	// Self-enforcing once-per-slot cap. Before this field the bound depended on
	// computeNodeState's per-slot memoization, which in turn depended on the
	// position of a defer in syncBlock; a refactor there would have silently put
	// this burst on the 5 ms sync loop. The swap also keeps concurrent callers
	// in the same slot down to one burst.
	currentSlot := boot.slotManager.Index()
	if boot.lastStuckRequestSlot.Swap(currentSlot) == currentSlot {
		return
	}

	if stalledWhileSynced {
		// Warn level, and emitted here rather than in shouldTryToRequestHeaders:
		// this goroutine runs outside mutNodeState, which consensus contends on
		// through GetNodeState, so log formatting stays out of that lock. It
		// cannot be driven by a peer, since both operands are local (own slot
		// index, own last committed block).
		log.Warn("node believes it is synchronized but has not committed a block for a while",
			"slots since last committed block", slotDiff,
			"last committed nonce", boot.getNonceForCurrentBlock(),
			"fork detector highest nonce", boot.forkDetector.ProbableHighestNonce())
	}

	fromNonce := boot.getNonceForNextBlock()
	numHeadersToRequest := tools.MinUint64(process.MaxHeadersToRequestInAdvance, slotDiff-1)
	toNonce := fromNonce + numHeadersToRequest - 1

	if fromNonce > toNonce {
		return
	}

	log.Debug("requestHeadersIfSyncIsStuck",
		"from nonce", fromNonce,
		"to nonce", toNonce,
		"probable highest nonce", boot.forkDetector.ProbableHighestNonce())

	boot.requestHeaders(fromNonce, toNonce)
}

func (boot *baseBootstrap) removeHeaderFromPools(blck data.HeaderHandler) []byte {
	blckData, ok := blck.(*block.Block)
	if !ok {
		log.Debug("CalculateHash", "error", common.ErrWrongTypeAssertion.Error())
		return nil
	}

	hash, err := tools.CalculateHash(boot.marshalizer, boot.hasher, blckData.Header)
	if err != nil {
		log.Debug("CalculateHash", "error", err.Error())
		return nil
	}

	log.Debug("removeHeaderFromPools",
		"epoch", blck.GetEpoch(),
		"slot", blck.GetSlot(),
		"nonce", blck.GetNonce(),
		"hash", hash)

	boot.headers.RemoveHeaderByHash(hash)

	return hash
}

func (boot *baseBootstrap) removeHeadersHigherThanNonceFromPool(nonce uint64) {
	log.Debug("removeHeadersHigherThanNonceFromPool",
		"nonce", nonce)

	nonces := boot.headers.Nonces()
	for _, currentNonce := range nonces {
		if currentNonce <= nonce {
			continue
		}

		boot.headers.RemoveHeaderByNonce(currentNonce)
	}

	boot.requestHandler.ResetRequests()
}

func (boot *baseBootstrap) cleanCachesAndStorageOnRollback(header data.HeaderHandler) {
	hash := boot.removeHeaderFromPools(header)
	boot.forkDetector.RemoveHeader(header.GetNonce(), hash)
	nonceToByteSlice := boot.uint64Converter.ToByteSlice(header.GetNonce())
	_ = boot.headerNonceHashStore.Remove(nonceToByteSlice)
}

// checkBootstrapNilParameters will check the imput parameters for nil values
func checkBootstrapNilParameters(arguments ArgBaseBootstrapper) error {
	if check.IfNil(arguments.ChainHandler) {
		return common.ErrNilBlockChain
	}
	if check.IfNil(arguments.SlotManager) {
		return process.ErrNilSlotManager
	}
	if check.IfNil(arguments.BlockProcessor) {
		return common.ErrNilBlockProcessor
	}
	if check.IfNil(arguments.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arguments.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arguments.ForkDetector) {
		return common.ErrNilForkDetector
	}
	if check.IfNil(arguments.RequestHandler) {
		return common.ErrNilRequestHandler
	}
	if check.IfNil(arguments.Accounts) {
		return common.ErrNilAccountsAdapter
	}
	if check.IfNil(arguments.Store) {
		return common.ErrNilStore
	}
	if check.IfNil(arguments.BlackListHandler) {
		return process.ErrNilBlackListCacher
	}
	if check.IfNil(arguments.NetworkWatcher) {
		return process.ErrNilNetworkWatcher
	}

	return nil
}

func (boot *baseBootstrap) requestHeadersFromNonceIfMissing(fromNonce uint64) {
	toNonce := tools.MinUint64(fromNonce+process.MaxHeadersToRequestInAdvance-1, boot.forkDetector.ProbableHighestNonce())

	if fromNonce > toNonce {
		return
	}

	log.Debug("requestHeadersFromNonceIfMissing",
		"from nonce", fromNonce,
		"to nonce", toNonce,
		"probable highest nonce", boot.forkDetector.ProbableHighestNonce())

	boot.requestHeaders(fromNonce, toNonce)
}

// syncBlocks method calls repeatedly synchronization method SyncBlock
func (boot *baseBootstrap) syncBlocks(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("bootstrap's go routine is stopping...")
			if boot.epochToEndFinished {
				boot.chanStopNodeProcess <- endProcess.ArgEndProcess{
					Reason:      "EndSyncUntil",
					Description: fmt.Sprintf("Node reached the passed epoch %d", boot.syncUntil),
				}
			}
			return
		case <-time.After(sleepTime):
		}

		if boot.epochToEndFinished {
			log.Warn("finish sync", "epoch", boot.syncUntil)
			if err := boot.Close(); err != nil {
				log.Debug("Close", "error", err.Error())
			}
		}

		// check connection
		if !boot.networkWatcher.IsConnectedToTheNetwork() {
			continue
		}

		if boot.slotManager.BeforeGenesis() {
			// don't need to sync now
			continue
		}
		err := boot.syncStarter.SyncBlock()
		if err != nil {
			log.Debug("SyncBlock", "error", err.Error())
		}
	}
}

func (boot *baseBootstrap) doJobOnSyncBlockFail(headerHandler data.HeaderHandler, err error) {
	processBlockStarted := !check.IfNil(headerHandler)
	isProcessWithError := processBlockStarted && !errors.Is(err, process.ErrTimeIsOut)

	numSyncedWithErrors := boot.incrementSyncedWithErrorsForNonce(boot.getNonceForNextBlock())
	allowedSyncWithErrorsLimitReached := numSyncedWithErrors >= process.MaxSyncWithErrorsAllowed
	isInProperSlot := process.IsInProperSlot(boot.slotManager.Index())
	isSyncWithErrorsLimitReachedInProperSlot := allowedSyncWithErrorsLimitReached && isInProperSlot

	shouldRollBack := isProcessWithError || isSyncWithErrorsLimitReachedInProperSlot
	if shouldRollBack {
		if !check.IfNil(headerHandler) {
			hash := boot.removeHeaderFromPools(headerHandler)
			boot.forkDetector.RemoveHeader(headerHandler.GetNonce(), hash)
		}

		errNotCritical := boot.rollBack(false)
		if errNotCritical != nil {
			log.Debug("rollBack", "error", errNotCritical.Error())
		}

		if isSyncWithErrorsLimitReachedInProperSlot {
			boot.forkDetector.ResetProbableHighestNonce()
			boot.removeHeadersHigherThanNonceFromPool(boot.getNonceForCurrentBlock())
		}
	}
}

func (boot *baseBootstrap) incrementSyncedWithErrorsForNonce(nonce uint64) uint32 {
	boot.mutNonceSyncedWithErrors.Lock()
	boot.mapNonceSyncedWithErrors[nonce]++
	numSyncedWithErrors := boot.mapNonceSyncedWithErrors[nonce]
	boot.mutNonceSyncedWithErrors.Unlock()

	return numSyncedWithErrors
}

// syncBlock method actually does the synchronization. It requests the next block header from the pool
// and if it is not found there it will be requested from the network. After the header is received,
// it requests the block body in the same way(pool and than, if it is not found in the pool, from network).
// If either header and body are received the ProcessBlock and CommitBlock method will be called successively.
// These methods will execute the block and its transactions. Finally if everything works, the block will be committed
// in the blockchain, and all this mechanism will be reiterated for the next block.
func (boot *baseBootstrap) syncBlock() error {
	boot.computeNodeState()
	nodeState := boot.GetNodeState()
	if nodeState != core.NsNotSynchronized {
		// Returning here leaves isNodeStateCalculated set, so computeNodeState
		// short circuits for the rest of the slot. The stuck-recovery burst and
		// its warning carry their own once-per-slot cap (lastStuckRequestSlot),
		// so hoisting the defer above this return would waste work but no
		// longer changes how often they fire.
		return nil
	}

	defer func() {
		boot.mutNodeState.Lock()
		boot.isNodeStateCalculated = false
		boot.mutNodeState.Unlock()
	}()

	if boot.forkInfo.IsDetected {
		boot.statusHandler.Increment(core.MetricNumTimesInForkChoice)

		if boot.isForcedRollBackOneBlock() {
			//Avoid multiple rollbacks if the next block is always the same to prevent stuck state
			currHeader := boot.chainHandler.GetCurrentBlockHeader()
			currHeaderHash := boot.chainHandler.GetCurrentBlockHeaderHash()

			currNonce := uint64(0)
			if currHeader != nil && !currHeader.IsInterfaceNil() {
				currNonce = currHeader.GetNonce()
			}
			log.Debug("fork detected", "currNonce", currNonce, "currHeader", currHeaderHash, "lastRollbackHash", boot.lastRollbackHash, "lastRollbackRetry", boot.avoidingSameRollbackRetryCount)

			if bytes.Equal(currHeaderHash, []byte(boot.lastRollbackHash)) {
				boot.avoidingSameRollbackRetryCount++
				if boot.avoidingSameRollbackRetryCount <= core.MaxRetriesAvoidingSameRollback {
					log.Debug("avoiding roll back with same block hash", "retry", boot.avoidingSameRollbackRetryCount, "of", core.MaxRetriesAvoidingSameRollback)
					boot.forkDetector.SoftResetFork(currNonce + 1)
					return nil
				}
			}

			boot.avoidingSameRollbackRetryCount = 0
			boot.lastRollbackHash = currHeaderHash

			log.Debug("roll back one block has been forced")
			boot.rollBackOneBlockForced()
			return nil
		}

		if boot.isForcedRollBackToNonce() {
			log.Debug("roll back to nonce has been forced", "nonce", boot.forkInfo.Nonce)
			boot.rollBackToNonceForced()
			return nil
		}

		log.Debug("fork detected",
			"nonce", boot.forkInfo.Nonce,
			"slot", boot.forkInfo.Slot,
			"hash", boot.forkInfo.Hash,
		)
		err := boot.rollBack(true)
		if err != nil {
			return err
		}
	}

	var header data.HeaderHandler
	var err error

	defer func() {
		if err != nil {
			boot.doJobOnSyncBlockFail(header, err)
		}
	}()

	header, err = boot.getNextHeaderRequestingIfMissing()
	if err != nil {
		return err
	}

	go boot.requestHeadersFromNonceIfMissing(header.GetNonce() + 1)

	if boot.syncUntil != 0 && header.GetEpoch() >= boot.syncUntil {
		// send signal to stop sync and close node
		log.Debug("current epoch is equal to epoch to end, close the node.", "current epoch", header.GetEpoch())
		boot.epochToEndFinished = true
		return nil
	}

	startTime := time.Now()
	waitTime := boot.slotManager.TimeDuration() * process.TimeDurationMultiplierForProcessBlockWhenSync
	haveTime := func() time.Duration {
		return waitTime - time.Since(startTime)
	}

	startProcessBlockTime := time.Now()
	err = boot.blockProcessor.ProcessBlock(header, haveTime)
	elapsedTime := time.Since(startProcessBlockTime)
	log.Debug("elapsed time to process block",
		"time [s]", elapsedTime,
	)
	if err != nil {
		return err
	}

	startCommitBlockTime := time.Now()
	err = boot.blockProcessor.CommitBlock(header)
	elapsedTime = time.Since(startCommitBlockTime)
	if elapsedTime >= core.CommitMaxTime {
		log.Warn("syncBlock.CommitBlock", "elapsed time", elapsedTime)
	} else {
		log.Debug("syncBlock elapsed time to commit block",
			"time [s]", elapsedTime,
		)
	}
	if err != nil {
		return err
	}

	log.Debug("block has been synced successfully",
		"nonce", header.GetNonce(),
	)

	boot.cleanNoncesSyncedWithErrorsBehindFinal()

	return nil
}

func (boot *baseBootstrap) cleanNoncesSyncedWithErrorsBehindFinal() {
	boot.mutNonceSyncedWithErrors.Lock()
	defer boot.mutNonceSyncedWithErrors.Unlock()

	finalNonce := boot.forkDetector.GetHighestFinalBlockNonce()
	for nonce := range boot.mapNonceSyncedWithErrors {
		if nonce < finalNonce {
			delete(boot.mapNonceSyncedWithErrors, nonce)
		}
	}
}

// rollBack decides if rollBackOneBlock must be called
func (boot *baseBootstrap) rollBack(revertUsingForkNonce bool) error {
	if boot.headerStore == nil {
		return common.ErrNilHeadersStorage
	}
	if boot.headerNonceHashStore == nil {
		return common.ErrNilHeadersNonceHashStorage
	}

	log.Debug("starting roll back")
	for {
		currHeaderHash := boot.chainHandler.GetCurrentBlockHeaderHash()
		currHeader, err := boot.blockBootstrapper.getCurrHeader()
		if err != nil {
			return err
		}
		if !revertUsingForkNonce && currHeader.GetNonce() <= boot.forkDetector.GetHighestFinalBlockNonce() {
			return ErrRollBackBehindFinalHeader
		}

		shouldEndRollBack := revertUsingForkNonce && currHeader.GetNonce() < boot.forkInfo.Nonce
		if shouldEndRollBack {
			return ErrRollBackBehindForkNonce
		}

		prevHeaderHash := currHeader.GetParentHash()
		prevHeader, err := boot.blockBootstrapper.getPrevHeader(currHeader, boot.headerStore)
		if err != nil {
			return err
		}

		log.Debug("roll back to block",
			"nonce", currHeader.GetNonce()-1,
			"hash", currHeader.GetParentHash(),
		)
		log.Debug("highest final block nonce",
			"nonce", boot.forkDetector.GetHighestFinalBlockNonce(),
		)

		currBody, err := boot.rollBackOneBlock(
			currHeaderHash,
			currHeader,
			prevHeaderHash,
			prevHeader,
		)
		if err != nil {
			return err
		}

		_, _ = updateMetricsFromStorage(boot.store, boot.uint64Converter, boot.marshalizer, boot.statusHandler, prevHeader.GetNonce())

		err = boot.bootStorer.SaveLastSlot(tools.SafeU64ToI64(prevHeader.GetSlot()))
		if err != nil {
			log.Debug("save last slot in storage",
				"error", err.Error(),
				"slot", prevHeader.GetSlot(),
			)
		}

		boot.indexer.RevertIndexedBlock(currBody)

		shouldAddHeaderToBlackList := revertUsingForkNonce && boot.blockBootstrapper.isForkTriggeredByMeta()
		if shouldAddHeaderToBlackList {
			process.AddHeaderToBlackList(boot.blackListHandler, currHeaderHash)
		}

		shouldContinueRollBack := revertUsingForkNonce && currHeader.GetNonce() > boot.forkInfo.Nonce
		if shouldContinueRollBack {
			continue
		}

		break
	}

	log.Debug("ending roll back")
	return nil
}

func (boot *baseBootstrap) rollBackOneBlock(
	currHeaderHash []byte,
	currHeader data.HeaderHandler,
	prevHeaderHash []byte,
	prevHeader data.HeaderHandler,
) (data.HeaderHandler, error) {

	var err error

	defer func() {
		if err != nil {
			boot.restoreState(currHeaderHash, currHeader)
		}
	}()

	if currHeader.GetNonce() > 1 {
		err = boot.setCurrentBlockInfo(prevHeaderHash, prevHeader)
		if err != nil {
			return nil, err
		}
	} else {
		err = boot.setCurrentBlockInfo(nil, nil)
		if err != nil {
			return nil, err
		}
	}

	err = boot.blockProcessor.RevertStateToBlock(prevHeader)
	if err != nil {
		return nil, err
	}
	boot.blockProcessor.PruneStateOnRollback(currHeader, prevHeader)

	err = boot.blockProcessor.RestoreBlockIntoPools(currHeader)
	if err != nil {
		return nil, err
	}

	boot.cleanCachesAndStorageOnRollback(currHeader)

	return currHeader, nil
}

func (boot *baseBootstrap) getNextHeaderRequestingIfMissing() (data.HeaderHandler, error) {
	nonce := boot.getNonceForNextBlock()

	boot.setRequestedHeaderHash(nil)
	boot.setRequestedHeaderNonce(nil)

	hash := boot.forkDetector.GetNotarizedHeaderHash(nonce)
	if boot.forkInfo.IsDetected {
		hash = boot.forkInfo.Hash
	}

	if hash != nil {
		return boot.blockBootstrapper.getHeaderWithHashRequestingIfMissing(hash)
	}

	return boot.blockBootstrapper.getHeaderWithNonceRequestingIfMissing(nonce)
}

func (boot *baseBootstrap) isForcedRollBackOneBlock() bool {
	return boot.forkInfo.IsDetected &&
		boot.forkInfo.Nonce == math.MaxUint64 &&
		boot.forkInfo.Hash == nil
}

func (boot *baseBootstrap) isForcedRollBackToNonce() bool {
	return boot.forkInfo.IsDetected &&
		boot.forkInfo.Slot == math.MaxUint64 &&
		boot.forkInfo.Hash == nil
}

func (boot *baseBootstrap) rollBackOneBlockForced() {
	err := boot.rollBack(false)
	if err != nil {
		log.Debug("rollBackOneBlockForced", "error", err.Error())
	}

	boot.forkDetector.ResetFork()
	boot.removeHeadersHigherThanNonceFromPool(boot.getNonceForCurrentBlock())
}

func (boot *baseBootstrap) rollBackToNonceForced() {
	err := boot.rollBack(true)
	if err != nil {
		log.Debug("rollBackToNonceForced", "error", err.Error())
	}

	boot.forkDetector.ResetProbableHighestNonce()
	boot.removeHeadersHigherThanNonceFromPool(boot.getNonceForCurrentBlock())
}

func (boot *baseBootstrap) restoreState(
	currHeaderHash []byte,
	currHeader data.HeaderHandler,
) {
	log.Debug("revert state to header",
		"nonce", currHeader.GetNonce(),
		"hash", currHeaderHash)

	err := boot.chainHandler.SetCurrentBlockHeaderAndHash(currHeader, currHeaderHash)
	if err != nil {
		// recovery path after a failed rollback — surface double-failures so ops can react
		log.Warn("restoreState: SetCurrentBlockHeaderAndHash", "error", err.Error())
	}

	err = boot.blockProcessor.RevertStateToBlock(currHeader)
	if err != nil {
		log.Warn("restoreState: RevertStateToBlock", "error", err.Error())
	}
}

func (boot *baseBootstrap) setCurrentBlockInfo(
	headerHash []byte,
	header data.HeaderHandler,
) error {
	return boot.chainHandler.SetCurrentBlockHeaderAndHash(header, headerHash)
}

func (boot *baseBootstrap) init() {
	boot.forkInfo = process.NewForkInfo()

	boot.chRcvHdrNonce = make(chan bool)
	boot.chRcvHdrHash = make(chan bool)

	boot.setRequestedHeaderNonce(nil)
	boot.setRequestedHeaderHash(nil)

	boot.headers.RegisterHandler(boot.processReceivedHeader)

	boot.statusHandler = statusHandler.NewNilStatusHandler()

	boot.syncStateListeners = make([]func(bool), 0)
	boot.requestedHashes = process.RequiredDataPool{}
	boot.mapNonceSyncedWithErrors = make(map[uint64]uint32)
}

func (boot *baseBootstrap) requestHeaders(fromNonce uint64, toNonce uint64) {
	boot.mutRequestHeaders.Lock()
	defer boot.mutRequestHeaders.Unlock()

	for currentNonce := fromNonce; currentNonce <= toNonce; currentNonce++ {
		haveHeader := boot.blockBootstrapper.haveHeaderInPoolWithNonce(currentNonce)
		if haveHeader {
			continue
		}

		boot.blockBootstrapper.requestHeaderByNonce(currentNonce)
	}
}

// GetNodeState method returns the sync state of the node. If it returns 'NsNotSynchronized', this means that the node
// is not synchronized yet and it has to continue the bootstrapping mechanism. If it returns 'NsSynchronized', this means
// that the node is already synced and it can participate to the consensus. This method could also returns 'NsNotCalculated'
// which means that the state of the node in the current slot is not calculated yet. Note that when the node is not
// connected to the network, GetNodeState could return 'NsNotSynchronized' but the SyncBlock is not automatically called.
func (boot *baseBootstrap) GetNodeState() core.NodeState {
	if boot.isInImportMode {
		return core.NsNotSynchronized
	}

	boot.mutNodeState.RLock()
	isNodeStateCalculatedInCurrentSlot := boot.slotIndex == boot.slotManager.Index() && boot.isNodeStateCalculated
	isNodeSynchronized := boot.isNodeSynchronized
	boot.mutNodeState.RUnlock()

	if !isNodeStateCalculatedInCurrentSlot {
		return core.NsNotCalculated
	}

	if isNodeSynchronized {
		return core.NsSynchronized
	}

	return core.NsNotSynchronized
}

// Close will close the endless running go routine
func (boot *baseBootstrap) Close() error {
	if boot.cancelFunc != nil {
		boot.cancelFunc()
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (boot *baseBootstrap) IsInterfaceNil() bool {
	return boot == nil
}
