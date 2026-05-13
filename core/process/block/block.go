package block

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.BlockProcessor = (*metaProcessor)(nil)

// metaProcessor implements metaProcessor interface and actually it tries to execute block
type metaProcessor struct {
	*baseProcessor
	economicsData process.EconomicsDataHandler
	chRcvAllHdrs  chan bool
	indexRating   bool
}

// NewMetaProcessor creates a new metaProcessor object
func NewMetaProcessor(arguments ArgMetaProcessor) (*metaProcessor, error) {
	err := checkProcessorNilParameters(arguments.ArgBaseProcessor)
	if err != nil {
		return nil, err
	}
	if check.IfNil(arguments.DataPool) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(arguments.DataPool.Headers()) {
		return nil, common.ErrNilHeadersDataPool
	}
	if check.IfNil(arguments.EconomicsData) {
		return nil, common.ErrNilEconomicsData
	}
	if check.IfNil(arguments.ValidatorStatisticsProcessor) {
		return nil, common.ErrNilValidatorStatistics
	}
	if check.IfNil(arguments.KAppController) {
		return nil, common.ErrKAppController
	}

	genesisHdr := arguments.BlockChain.GetGenesisHeader()
	base := &baseProcessor{
		accountsDB:                   arguments.AccountsDB,
		blockSizeThrottler:           arguments.BlockSizeThrottler,
		forkDetector:                 arguments.ForkDetector,
		hasher:                       arguments.Hasher,
		marshalizer:                  arguments.Marshalizer,
		store:                        arguments.Store,
		nodesCoordinator:             arguments.NodesCoordinator,
		uint64Converter:              arguments.Uint64Converter,
		requestHandler:               arguments.RequestHandler,
		appStatusHandler:             statusHandler.NewNilStatusHandler(),
		epochStartTrigger:            arguments.EpochStartTrigger,
		slotManager:                  arguments.SlotManager,
		bootStorer:                   arguments.BootStorer,
		dataPool:                     arguments.DataPool,
		blockChain:                   arguments.BlockChain,
		stateCheckpointModulus:       arguments.StateCheckpointModulus,
		processingMode:               arguments.ProcessingMode,
		tpsBenchmark:                 arguments.TpsBenchmark,
		genesisNonce:                 genesisHdr.GetNonce(),
		epochNotifier:                arguments.EpochNotifier,
		validatorStatisticsProcessor: arguments.ValidatorStatisticsProcessor,
		txCoordinator:                arguments.TxCoordinator,
		feeHandler:                   arguments.FeeHandler,
		eventsProcessor:              arguments.EventsProcessor,
		headerIntegrityVerifier:      arguments.HeaderIntegrityVerifier,
		kAppController:               arguments.KAppController,
		blockChainHook:               arguments.BlockChainHook,
		forkController:               arguments.ForkController,
	}

	mp := metaProcessor{
		baseProcessor: base,
		economicsData: arguments.EconomicsData,
		indexRating:   arguments.IndexRating,
	}

	mp.txCounter = NewTransactionCounter()

	headersPool := mp.dataPool.Headers()
	headersPool.RegisterHandler(mp.receivedHeader)

	mp.chRcvAllHdrs = make(chan bool)

	return &mp, nil
}

// ProcessBlock processes a block. It returns nil if all ok or the specific error
func (mp *metaProcessor) ProcessBlock(
	headerHandler data.HeaderHandler,
	haveTime func() time.Duration,
) error {
	startProcess := time.Now()

	defer func() {
		mp.appStatusHandler.SetUInt64Value(
			core.MetricBlockProcessDuration,
			uint64(time.Since(startProcess).Milliseconds()), // #nosec G115
		)
	}()

	if haveTime == nil {
		return process.ErrNilHaveTimeHandler
	}

	err := mp.validateBlockAndRequestMissing(headerHandler)
	if err != nil {
		return err
	}

	mp.epochNotifier.CheckEpoch(headerHandler.GetEpoch())
	mp.requestHandler.SetEpoch(headerHandler.GetEpoch())

	mp.kAppController.GetValidatorsKApp().GetAccountsCacher().ResetAll(
		mp.forkController.ProcessorFlowITOPrice(),
	)

	log.Debug("started processing block",
		"epoch", headerHandler.GetEpoch(),
		"slot", headerHandler.GetSlot(),
		"nonce", headerHandler.GetNonce())

	header, ok := headerHandler.(*block.Block)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	mp.dispatchAsyncHeaderMetrics(header)

	defer func() {
		if err != nil {
			log.Error("ProcessBlock revert: ", "error", err.Error())
			mp.RevertStateToSnapshot(header)
		}
	}()

	mp.createBlockStarted()
	mp.blockChainHook.SetCurrentHeader(header)
	mp.epochStartTrigger.Update(header.GetSlot(), header.GetNonce())

	err = mp.checkEpochCorrectness(header)
	if err != nil {
		return err
	}

	// request missing TXs
	mp.txCoordinator.RequestBlockTransactions(header)

	if haveTime() < 0 {
		return process.ErrTimeIsOut
	}

	err = mp.txCoordinator.IsDataPreparedForProcessing(haveTime)
	if err != nil {
		log.Debug("ProcessBlock missing tx", "slot", header.GetSlot(), "nonce", header.GetNonce(), "error", err.Error())
		return err
	}

	if mp.accountsDB[state.UserAccountsState].JournalLen() != 0 {
		return process.ErrAccountStateDirty
	}

	err = mp.processIfFirstBlockAfterEpochStart()
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process first block after epoch start: %w", err))
		return err
	}

	if header.GetIsEpochStart() {
		return mp.handleEpochStartBlock(header)
	}

	err = mp.verifyFees(header)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process verify fees: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	// Process Transactions
	startTx := time.Now()
	processResults, err := mp.txCoordinator.ProcessBlockTransactions(header, haveTime)
	mp.appStatusHandler.SetUInt64Value(
		core.MetricTxProcessingDuration,
		uint64(time.Since(startTx).Milliseconds()), // #nosec G115
	)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process block transactions: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	go getMetricsFromTXProcessed(
		processResults,
		mp.appStatusHandler,
	)

	err = mp.verifyBlockTrieRoots(header)
	if err != nil {
		return err
	}

	return nil
}

// validateBlockAndRequestMissing wraps checkBlockValidity and, on an
// ErrBlockHashDoesNotMatch result, fires an async request for the missing
// parent header before returning the error to the caller.
func (mp *metaProcessor) validateBlockAndRequestMissing(headerHandler data.HeaderHandler) error {
	err := mp.checkBlockValidity(headerHandler)
	if err == nil {
		return nil
	}

	if err == process.ErrBlockHashDoesNotMatch {
		log.Debug("requested missing header",
			"hash", headerHandler.GetParentHash(),
		)
		go mp.requestHandler.RequestHeader(headerHandler.GetParentHash())
	}

	return err
}

// dispatchAsyncHeaderMetrics clones the header and emits per-block metrics in
// a goroutine. ProcessBlock keeps mutating the original header pointer (e.g.,
// epochStartNativeStakingKapps -> SetBurnedUnclaimed), so handing the same
// reference to a background reader races on every field access.
func (mp *metaProcessor) dispatchAsyncHeaderMetrics(header *block.Block) {
	txCounts := mp.txCounter.getPoolCounts(mp.dataPool)
	log.Debug("total txs in pool", "counts", txCounts.String())

	metricsHeader, ok := header.Clone().(*block.Block)
	if !ok {
		log.Error("ProcessBlock: cloned header has unexpected type, skipping async metrics")
		return
	}

	go getMetricsFromHeader(
		metricsHeader,
		tools.SafeI64ToU64(txCounts.GetTotal()),
		mp.marshalizer,
		mp.appStatusHandler,
	)
}

// handleEpochStartBlock runs the epoch-start branch of ProcessBlock: process
// the epoch-start block, then verify fees. The fees error is reported and
// returned, but does not skip the bugsnag notification.
func (mp *metaProcessor) handleEpochStartBlock(header *block.Block) error {
	err := mp.processEpochStartBlock(header)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process epoch start block: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	err = mp.verifyFees(header)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process verify fees: %w", err), bugsnag.MetaData{"data": {"header": header}})
	}

	return nil
}

// verifyBlockTrieRoots runs the three post-transaction trie-root checks
// (account, validator-statistics, kapp). Each failure is reported via bugsnag
// before being returned to the caller.
func (mp *metaProcessor) verifyBlockTrieRoots(header *block.Block) error {
	if !mp.verifyStateRootAccount(header.GetTrieRoot()) {
		log.Debug("processBlock.verifyStateRootAccount", "blockTrieRoot", logger.DisplayByteSlice(header.GetTrieRoot()))
		err := process.ErrRootStateDoesNotMatch
		_ = bugsnag.Notify(fmt.Errorf("process account state: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	err := mp.verifyValidatorStatisticsRootHash(header)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process epoch valdiator state: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	err = mp.verifyKAppRootHash(header)
	if err != nil {
		_ = bugsnag.Notify(fmt.Errorf("process kapp state: %w", err), bugsnag.MetaData{"data": {"header": header}})
		return err
	}

	return nil
}

func (mp *metaProcessor) processEpochStartBlock(
	header *block.Block,
) error {
	err := mp.baseProcessEpochStartBlock(header)
	if err != nil {
		return err
	}

	// Validate Account Trie after rewards update
	if !mp.verifyStateRootAccount(header.GetTrieRoot()) {
		err = process.ErrRootStateDoesNotMatch
		return err
	}

	// Validate Validator Trie after list update
	err = mp.verifyValidatorStatisticsRootHash(header)
	if err != nil {
		return err
	}

	// Validate KApp Trie after validaotr list and proposals update
	err = mp.verifyKAppRootHash(header)
	if err != nil {
		return err
	}

	return nil
}

func (mp *metaProcessor) baseProcessEpochStartBlock(
	header *block.Block,
) error {
	// Epoch Start does not handle any transaction, so Tx/Kapps Fees must be zero
	if header.GetTxCount() != 0 {
		return process.ErrInvalidTXCount
	}

	if header.GetTxFees() != 0 ||
		header.GetKAppFees() != 0 ||
		header.GetTxBurnedFees() != 0 {
		return process.ErrInvalidTXFees

	}

	currentRootHash, err := mp.validatorStatisticsProcessor.RootHash()
	if err != nil {
		return err
	}

	// Get All Validators Info from PeerAccount List
	allValidatorsInfo, err := mp.validatorStatisticsProcessor.GetValidatorInfoForRootHash(currentRootHash)
	if err != nil {
		return err
	}

	// process new rating and jail if bellow threshold
	err = mp.validatorStatisticsProcessor.ProcessRatingsEndOfEpoch(allValidatorsInfo, header.GetEpoch())
	if err != nil {
		return err
	}

	// process proposals prior economics, as it could change economics model
	err = mp.processProposalsEndOfEpoch(header)
	if err != nil {
		return err
	}

	// distribute rewards, Set node as inactive/waiting/eligible based on stake
	// (will not change list if have been sent to jail)
	err = mp.processEconomicsEndOfEpoch(allValidatorsInfo, header)
	if err != nil {
		return err
	}

	// reset rating and jail if needed... also returns update list to be set in nodes coordinator
	updatedList, err := mp.validatorStatisticsProcessor.ResetValidatorStatisticsAtNewEpoch(allValidatorsInfo)
	if err != nil {
		return err
	}

	// Update nodes coodinator with new list of validators/peers
	err = mp.nodesCoordinator.SetEpochValidatorsInfo(header.GetEpoch(), updatedList)
	if err != nil {
		return err
	}

	return nil
}

func (mp *metaProcessor) verifyKAppRootHash(header *block.Block) error {
	err := mp.UpdateNativeStakingKapps(header)
	if err != nil {
		return err
	}

	// Inflation = Block + Staking Rewards - TXFee Burned
	// Stking Rewards not claimed are burned in EpochStart
	err = mp.UpdateKLVCirculationSupply(header.GetBlockRewards()+header.GetStakingRewards(), header.GetTxBurnedFees())
	if err != nil {
		return err
	}

	if !mp.verifyStateRootKApp(header.GetKAppsTrieRoot()) {
		log.Debug("kApp root hash mismatch", "received", header.GetKAppsTrieRoot())
		return fmt.Errorf("%s, received: %s, meta header nonce: %d",
			process.ErrRootStateDoesNotMatch,
			logger.DisplayByteSlice(header.GetKAppsTrieRoot()),
			header.Header.Nonce,
		)
	}

	return nil
}

func (mp *metaProcessor) verifyValidatorStatisticsRootHash(header *block.Block) error {
	currentBlockHeader := mp.blockChain.GetCurrentBlockHeader()
	if check.IfNil(currentBlockHeader) {
		currentBlockHeader = mp.blockChain.GetGenesisHeader()
	}

	validatorStatsRH, err := mp.validatorStatisticsProcessor.UpdatePeerState(header, currentBlockHeader)
	if err != nil {
		return err
	}

	if !bytes.Equal(validatorStatsRH, header.GetValidatorsTrieRoot()) {
		log.Debug("validator stats root hash mismatch",
			"computed", validatorStatsRH,
			"received", header.GetValidatorsTrieRoot(),
		)
		return fmt.Errorf("%s, metachain, computed: %s, received: %s, meta header nonce: %d",
			process.ErrValidatorStatsRootHashDoesNotMatch,
			logger.DisplayByteSlice(validatorStatsRH),
			logger.DisplayByteSlice(header.GetValidatorsTrieRoot()),
			header.Header.Nonce,
		)
	}

	return nil
}

func (mp *metaProcessor) checkEpochCorrectness(
	headerHandler data.HeaderHandler,
) error {
	currentBlockHeader := mp.blockChain.GetCurrentBlockHeader()
	if check.IfNil(currentBlockHeader) {
		return nil
	}

	isEpochIncorrect := headerHandler.GetEpoch() != currentBlockHeader.GetEpoch() &&
		mp.epochStartTrigger.Epoch() == currentBlockHeader.GetEpoch()
	if isEpochIncorrect {
		log.Warn("epoch does not match", "currentHeaderEpoch", currentBlockHeader.GetEpoch(), "receivedHeaderEpoch", headerHandler.GetEpoch(), "epochStartTrigger", mp.epochStartTrigger.Epoch())
		return process.ErrEpochDoesNotMatch
	}

	isEpochIncorrect = mp.epochStartTrigger.IsEpochStart() &&
		mp.epochStartTrigger.EpochStartSlot() <= headerHandler.GetSlot() &&
		headerHandler.GetEpoch() != currentBlockHeader.GetEpoch()+1
	if isEpochIncorrect {
		log.Warn("is epoch start and epoch does not match",
			"currentHeaderEpoch", currentBlockHeader.GetEpoch(),
			"receivedHeaderEpoch", headerHandler.GetEpoch(),
			"epochStartTrigger", mp.epochStartTrigger.Epoch(),
			"epochStartSlot", mp.epochStartTrigger.EpochStartSlot(),
			"headerSlot", headerHandler.GetSlot(),
		)
		return process.ErrEpochDoesNotMatch
	}

	return nil
}

// CreateBlock creates the final block and header for the current slot
func (mp *metaProcessor) CreateBlock(
	initialHdr data.HeaderHandler,
	haveTime func() bool,
) (data.HeaderHandler, error) {
	if check.IfNil(initialHdr) {
		return nil, process.ErrNilBlockHeader
	}

	blk, ok := initialHdr.(*block.Block)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	mp.epochStartTrigger.Update(initialHdr.GetSlot(), initialHdr.GetNonce())
	blk.Header.Epoch = mp.epochStartTrigger.Epoch()
	blk.Header.SoftwareVersion = []byte(mp.headerIntegrityVerifier.GetVersion(blk.Header.Epoch))
	mp.epochNotifier.CheckEpoch(blk.GetEpoch())
	mp.blockChainHook.SetCurrentHeader(initialHdr)

	mp.kAppController.GetValidatorsKApp().GetAccountsCacher().ResetAll(
		mp.forkController.ProcessorFlowITOPrice(),
	)

	if mp.accountsDB[state.UserAccountsState].JournalLen() != 0 {
		return nil, process.ErrAccountStateDirty
	}
	err := mp.processIfFirstBlockAfterEpochStart()
	if err != nil {
		return nil, err
	}

	if mp.epochStartTrigger.IsEpochStart() {
		err := mp.updateEpochStartHeader(blk)
		if err != nil {
			return nil, err
		}

		err = mp.CreateEpochStartHeader(blk)
		if err != nil {
			return nil, err
		}
	} else {
		err := mp.createBlockHeader(blk, haveTime)
		if err != nil {
			return nil, err
		}
	}

	// update block info
	err = mp.applyHeader(blk)
	if err != nil {
		return nil, err
	}
	mp.requestHandler.SetEpoch(blk.GetEpoch())

	return blk, nil
}

func (mp *metaProcessor) isPreviousBlockEpochStart() (uint32, bool) {
	blockHeader := mp.blockChain.GetCurrentBlockHeader()
	if check.IfNil(blockHeader) {
		blockHeader = mp.blockChain.GetGenesisHeader()
	}

	return blockHeader.GetEpoch(), blockHeader.GetIsEpochStart()
}

func (mp *metaProcessor) processIfFirstBlockAfterEpochStart() error {
	epoch, isPreviousEpochStart := mp.isPreviousBlockEpochStart()
	if !isPreviousEpochStart {
		return nil
	}

	nodesForcedToStay, err := mp.validatorStatisticsProcessor.SaveNodesCoordinatorUpdates(epoch)
	if err != nil {
		return err
	}

	_ = nodesForcedToStay

	return nil
}

func (mp *metaProcessor) updateEpochStartHeader(blk *block.Block) error {
	sw := tools.NewStopWatch()
	sw.Start("createEpochStartForMetablock")
	defer func() {
		sw.Stop("createEpochStartForMetablock")
		log.Debug("epochStartHeaderDataCreation", sw.GetMeasurements()...)
	}()

	blk.Header.IsEpochStart = true
	blk.Header.PrevEpochStartSlot = mp.epochStartTrigger.PrevEpochStartSlot()

	return nil
}

func (mp *metaProcessor) CreateEpochStartHeader(blk *block.Block) error {
	mp.createBlockStarted()

	log.Debug("started creating epoch start block header",
		"epoch", blk.GetEpoch(),
		"slot", blk.GetSlot(),
		"nonce", blk.GetNonce(),
	)

	return mp.baseProcessEpochStartBlock(blk)
}

// CommitBlock commits the block in the blockchain if everything was checked successfully
func (mp *metaProcessor) CommitBlock(
	headerHandler data.HeaderHandler,
) error {
	startCommitBlock := time.Now()

	defer func() {
		mp.appStatusHandler.SetUInt64Value(
			core.MetricBlockCommitDuration,
			uint64(time.Since(startCommitBlock).Milliseconds()), // #nosec G115
		)
	}()
	var err error
	defer func() {
		if err != nil {
			mp.RevertStateToSnapshot(headerHandler)
		}
	}()

	if check.IfNil(headerHandler) {
		return process.ErrNilBlockHeader
	}

	log.Debug("started committing block",
		"epoch", headerHandler.GetEpoch(),
		"slot", headerHandler.GetSlot(),
		"nonce", headerHandler.GetNonce(),
	)

	err = mp.checkBlockValidity(headerHandler)
	if err != nil {
		return err
	}

	mp.store.SetEpochForPutOperation(headerHandler.GetEpoch())

	header, ok := headerHandler.(*block.Block)
	if !ok {
		err = common.ErrWrongTypeAssertion
		return err
	}

	marshalizedHeader, err := mp.marshalizer.Marshal(header.Header)
	if err != nil {
		return err
	}

	marshalizedBlock, err := mp.marshalizer.Marshal(header)
	if err != nil {
		return err
	}

	mp.commitEpochStart(header)
	headerHash := mp.hasher.Compute(string(marshalizedHeader))
	mp.saveHeader(header, headerHash, marshalizedBlock)

	mp.saveBody(header)

	err = mp.commitAll()
	if err != nil {
		return err
	}

	mp.validatorStatisticsProcessor.DisplayRatings(header.GetEpoch())

	log.Info("meta block has been committed successfully",
		"epoch", header.GetEpoch(),
		"slot", header.GetSlot(),
		"nonce", header.GetNonce(),
		"hash", headerHash)

	errNotCritical := mp.forkDetector.AddHeader(header, headerHash, process.BHProcessed, nil, nil)
	if errNotCritical != nil {
		log.Debug("forkDetector.AddHeader", "error", errNotCritical.Error())
	}

	log.Debug("highest final meta block",
		"nonce", mp.forkDetector.GetHighestFinalBlockNonce(),
	)

	lastMetaBlock := mp.blockChain.GetCurrentBlockHeader()
	mp.updateState(lastMetaBlock)

	// set blockchain header info
	err = mp.blockChain.SetCurrentBlockHeader(header)
	if err != nil {
		return err
	}
	mp.blockChain.SetCurrentBlockHeaderHash(headerHash)

	mp.tpsBenchmark.Update(lastMetaBlock)

	mp.indexBlock(header, headerHash, header, lastMetaBlock)
	highestFinalBlockNonce := mp.forkDetector.GetHighestFinalBlockNonce()
	saveMetricsForCommitBlock(mp.appStatusHandler, header, headerHash, mp.nodesCoordinator, highestFinalBlockNonce)

	go mp.txCounter.displayLogInfo(
		header,
		headerHash,
		mp.appStatusHandler,
		uint64(mp.slotManager.TimeDuration().Seconds()),
	)

	headerInfo := &bootstrapStorage.BootstrapHeaderInfo{
		Epoch: header.GetEpoch(),
		Nonce: header.GetNonce(),
		Hash:  headerHash,
	}

	nodesCoordinatorKey := mp.nodesCoordinator.GetSavedStateKey()
	epochStartKey := mp.epochStartTrigger.GetSavedStateKey()

	args := &bootStorerDataArgs{
		headerInfo:                 headerInfo,
		slot:                       header.GetSlot(),
		nodesCoordinatorConfigKey:  nodesCoordinatorKey,
		epochStartTriggerConfigKey: epochStartKey,
		highestFinalBlockNonce:     highestFinalBlockNonce,
	}

	mp.prepareDataForBootStorer(args)

	mp.blockSizeThrottler.Succeed(header.GetSlot())

	mp.displayPoolsInfo()

	errNotCritical = mp.removeTxsFromPools(header)
	if errNotCritical != nil {
		log.Debug("removeTxsFromPools", "error", errNotCritical.Error())
	}

	mp.cleanupPools(headerHandler)

	return nil
}

func (mp *metaProcessor) displayPoolsInfo() {
	headersPool := mp.dataPool.Headers()

	log.Debug("pools info",
		"total headers", headersPool.Len(),
		"headers pool capacity", headersPool.MaxSize(),
	)
}

func (mp *metaProcessor) updateState(lastMetaBlock data.HeaderHandler) {
	if check.IfNil(lastMetaBlock) {
		log.Debug("updateState nil header")
		return
	}

	mp.validatorStatisticsProcessor.SetLastFinalizedRootHash(lastMetaBlock.GetValidatorsTrieRoot())

	prevHeader, errNotCritical := process.GetHeader(
		lastMetaBlock.GetParentHash(),
		mp.dataPool.Headers(),
		mp.marshalizer,
		mp.store,
	)
	if errNotCritical != nil {
		log.Debug("could not get meta header from storage", "error", errNotCritical.Error())
		return
	}

	if lastMetaBlock.GetIsEpochStart() {
		// Skip epoch snapshots during import-db mode to prevent TrieSnapshot directory growth
		// and memory accumulation from multiple LevelDB instances (see KLC-2057)
		if mp.processingMode == core.ImportDb {
			log.Trace("skipping epoch snapshot in import-db mode", "epoch", lastMetaBlock.GetEpoch())
		} else {
			log.Debug("trie snapshot", "rootHash", lastMetaBlock.GetTrieRoot())
			ctx := context.Background()
			mp.accountsDB[state.UserAccountsState].SnapshotState(lastMetaBlock.GetTrieRoot(), ctx)
			mp.accountsDB[state.PeerAccountsState].SnapshotState(lastMetaBlock.GetValidatorsTrieRoot(), ctx)
			mp.accountsDB[state.KAppAccountsState].SnapshotState(lastMetaBlock.GetKAppsTrieRoot(), ctx)
		}
	}

	mp.updateStateStorage(
		lastMetaBlock,
		lastMetaBlock.GetTrieRoot(),
		prevHeader.GetTrieRoot(),
		mp.accountsDB[state.UserAccountsState],
	)

	mp.updateStateStorage(
		lastMetaBlock,
		lastMetaBlock.GetValidatorsTrieRoot(),
		prevHeader.GetValidatorsTrieRoot(),
		mp.accountsDB[state.PeerAccountsState],
	)

	mp.updateStateStorage(
		lastMetaBlock,
		lastMetaBlock.GetKAppsTrieRoot(),
		prevHeader.GetKAppsTrieRoot(),
		mp.accountsDB[state.KAppAccountsState],
	)
}

func (mp *metaProcessor) commitEpochStart(header *block.Block) {
	if header.GetIsEpochStart() {
		mp.epochStartTrigger.SetProcessed(header)
		// TODO: save to storage
		// go mp.validatorInfoCreator.SaveValidatorInfoBlocksToStorage(header, body)
	} else {
		currentHeader := mp.blockChain.GetCurrentBlockHeader()
		if !check.IfNil(currentHeader) && currentHeader.GetIsEpochStart() {
			mp.epochStartTrigger.SetFinalityAttestingSlot(header.GetSlot())
			// TODO: check this process
			// mp.nodesCoordinator.ShuffleOutForEpoch(currentHeader.GetEpoch())
		}
	}
}

// RevertStateToBlock recreates the state tries to the root hashes indicated by the provided header
func (mp *metaProcessor) RevertStateToBlock(header data.HeaderHandler) error {
	err := mp.accountsDB[state.UserAccountsState].RecreateTrie(header.GetTrieRoot())
	if err != nil {
		log.Debug("recreate trie with error for header",
			"nonce", header.GetNonce(),
			"hash", header.GetTrieRoot(),
			"error", err.Error(),
		)

		return err
	}

	err = mp.accountsDB[state.KAppAccountsState].RecreateTrie(header.GetKAppsTrieRoot())
	if err != nil {
		log.Debug("recreate kapps trie with error for header",
			"nonce", header.GetNonce(),
			"hash", header.GetTrieRoot(),
			"error", err.Error(),
		)

		return err
	}

	err = mp.validatorStatisticsProcessor.RevertPeerState(header)
	if err != nil {
		log.Debug("revert peer state with error for header",
			"nonce", header.GetNonce(),
			"validators root hash", header.GetValidatorsTrieRoot(),
			"error", err.Error(),
		)

		return err
	}

	err = mp.epochStartTrigger.RevertStateToBlock(header)
	if err != nil {
		log.Debug("revert epoch start trigger for header",
			"nonce", header.GetNonce(),
			"error", err,
		)
		return err
	}

	return nil
}

// receivedHeader is a call back function which is called when a new header
// is added in the headers pool
func (mp *metaProcessor) receivedHeader(headerHandler data.HeaderHandler, headerHash []byte) {
	blk, ok := headerHandler.(*block.Block)
	if !ok {
		log.Warn("cannot convert data.HeaderHandler in *block.Block")
		return
	}

	log.Trace("block received header from network",
		"slot", blk.GetSlot(),
		"nonce", blk.GetNonce(),
		"hash", headerHash,
	)
}

// applyHeader creates a block header list
func (mp *metaProcessor) applyHeader(blk *block.Block) error {
	sw := tools.NewStopWatch()
	sw.Start("applyHeader")
	defer func() {
		sw.Stop("applyHeader")

		log.Debug("measurements", sw.GetMeasurements()...)
	}()

	if check.IfNil(blk) {
		return process.ErrNilBlockHeader
	}

	var err error

	blk.Header.Epoch = mp.epochStartTrigger.Epoch()
	blk.Header.TrieRoot = mp.getRootHashAccount()
	blk.Header.TxCount = uint32(len(blk.TxHashes)) // #nosec G115
	blk.Header.TxFees = mp.feeHandler.GetAccumulatedTxFees()
	blk.Header.TxBurnedFees = mp.feeHandler.GetAccumulatedTxFees() - mp.validatorStatisticsProcessor.LeaderRewards(blk.GetTxFees())
	blk.Header.KAppFees = mp.feeHandler.GetAccumulatedKAppFees()
	blk.Header.BlockRewards = mp.proposalController.GetParameterInt(kapps.EnumParameter_BlockRewards)
	blk.Header.StakingRewards = mp.proposalController.GetParameterInt(kapps.EnumParameter_StakingRewards)

	sw.Start("UpdatePeerState")
	currentBlockHeader := mp.blockChain.GetCurrentBlockHeader()
	if check.IfNil(currentBlockHeader) {
		currentBlockHeader = mp.blockChain.GetGenesisHeader()
	}

	blk.Header.ValidatorsTrieRoot, err = mp.validatorStatisticsProcessor.UpdatePeerState(blk, currentBlockHeader)
	sw.Stop("UpdatePeerState")
	if err != nil {
		return err
	}

	sw.Start("UpdateNativeStakingKapps")
	err = mp.UpdateNativeStakingKapps(blk)
	if err != nil {
		return err
	}
	sw.Stop("UpdateNativeStakingKapps")

	// Inflation = Block + Staking Rewards - TXFee Burned
	// Staking Rewards not claimed are burned in EpochStart
	err = mp.UpdateKLVCirculationSupply(blk.GetBlockRewards()+blk.GetStakingRewards(), blk.GetTxBurnedFees())
	if err != nil {
		return err
	}
	blk.Header.KAppsTrieRoot = mp.getRootHashKApp()

	marshalizedBody, err := mp.marshalizer.Marshal(blk)
	if err != nil {
		return err
	}
	mp.blockSizeThrottler.Add(blk.GetSlot(), uint32(len(marshalizedBody))) // #nosec G115

	return nil
}

// CreateNewHeader creates a new header
func (mp *metaProcessor) CreateNewHeader(slot uint64, nonce uint64) data.HeaderHandler {
	blk := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
		},
	}

	return blk
}

// MarshalizedDataToBroadcast prepares underlying data into a marshalized object according to destination
func (mp *metaProcessor) MarshalizedDataToBroadcast(
	hdr data.HeaderHandler,
) ([]byte, [][]byte, error) {
	if check.IfNil(hdr) {
		return nil, nil, process.ErrNilBlockHeader
	}

	body, ok := hdr.(*block.Block)
	if !ok {
		return nil, nil, process.ErrWrongTypeAssertion
	}

	mrsTxs, err := mp.txCoordinator.CreateMarshalizedData(body)
	if err != nil {
		log.Warn("metaProcessor.CreateMarshalizedData.Marshal", "error", err.Error())
	}

	mrsData, err := mp.marshalizer.Marshal(body)
	if err != nil {
		log.Error("metaProcessor.MarshalizedDataToBroadcast.Marshal", "error", err.Error())
		return nil, nil, err
	}

	return mrsData, mrsTxs, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (mp *metaProcessor) IsInterfaceNil() bool {
	return mp == nil
}

// RestoreBlockIntoPools restores the block into associated pools
func (mp *metaProcessor) RestoreBlockIntoPools(headerHandler data.HeaderHandler) error {
	if check.IfNil(headerHandler) {
		return process.ErrNilBlockHeader
	}

	blk, ok := headerHandler.(*block.Block)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	hdrHash, err := tools.CalculateHash(mp.marshalizer, mp.hasher, blk.Header)
	if err != nil {
		return err
	}

	headersPool := mp.dataPool.Headers()

	header, errNotCritical := process.GetHeaderFromStorage(hdrHash, mp.marshalizer, mp.store)
	if errNotCritical != nil {
		log.Debug("header not found in BlockUnit",
			"hash", hdrHash,
		)
	}

	headersPool.AddHeader(hdrHash, header)

	hdrNonceHashDataUnit := retriever.HdrNonceHashDataUnit
	storer := mp.store.GetStorer(hdrNonceHashDataUnit)
	nonceToByteSlice := mp.uint64Converter.ToByteSlice(header.GetNonce())
	errNotCritical = storer.Remove(nonceToByteSlice)
	if errNotCritical != nil {
		log.Debug("HdrNonceHashDataUnit.Remove", "error", errNotCritical.Error())
	}

	mp.restoreBlockHeader(blk)

	return nil
}

func (bp *baseProcessor) restoreBlockHeader(blk *block.Block) {
	restoredTxNr, errNotCritical := bp.txCoordinator.RestoreBlockDataFromStorage(blk)
	if errNotCritical != nil {
		log.Debug("restoreBlockHeader RestoreBlockDataFromStorage", "error", errNotCritical.Error())
	}

	go bp.txCounter.subtractRestoredTxs(restoredTxNr)
}

// createBlockHeader -
func (mp *metaProcessor) createBlockHeader(blk *block.Block, haveTime func() bool) error {
	mp.createBlockStarted()

	mp.blockSizeThrottler.ComputeCurrentMaxSize()

	log.Debug("started creating block header",
		"epoch", blk.GetEpoch(),
		"slot", blk.GetSlot(),
		"nonce", blk.GetNonce(),
	)

	if !haveTime() {
		log.Debug("metaProcessor.createBlock", "error", process.ErrTimeIsOut)
		return nil
	}

	startTime := time.Now()
	processResults, err := mp.txCoordinator.CreateAndProcessBlockTransactions(blk, haveTime)
	elapsedTime := time.Since(startTime)
	log.Debug("elapsed time to select tx and create block",
		"time [s]", elapsedTime.Seconds(),
	)
	if err != nil {
		log.Debug("createAndProcessBlock", "error", err.Error())
	}

	if processResults != nil {
		log.Debug("select tx and create block results",
			"Num Txs", processResults.Length(),
			"Txs Size", processResults.Size(),
		)
	}

	return nil
}

func (mp *metaProcessor) indexValidatorsRating() {
	if !mp.indexRating || check.IfNil(mp.eventsProcessor) {
		return
	}

	latestHash, err := mp.validatorStatisticsProcessor.RootHash()
	if err != nil {
		log.Error("indexValidatorsRating Get RootHash", "err", err.Error())
		return
	}

	validators, err := mp.validatorStatisticsProcessor.GetValidatorAccountRootHash(latestHash)
	if err != nil {
		log.Error("indexValidatorsRating GetValidatorAccountRootHash", "err", err.Error())
		return
	}

	mp.eventsProcessor.SaveValidatorsRating(validators)
}

func (mp *metaProcessor) indexEpochInfo(
	epoch uint32,
) {
	if check.IfNil(mp.eventsProcessor) {
		return
	}

	latestHash, err := mp.validatorStatisticsProcessor.RootHash()
	if err != nil {
		log.Error("indexValidatorsRating Get RootHash", "err", err.Error())
		return
	}

	validators, err := mp.validatorStatisticsProcessor.GetValidatorAccountRootHash(latestHash)
	if err != nil {
		log.Error("indexValidatorsRating GetValidatorAccountRootHash", "err", err.Error())
		return
	}

	mp.eventsProcessor.SaveEpochInfo(epoch, validators)
}

func (mp *metaProcessor) indexBlock(
	header data.HeaderHandler,
	headerHash []byte,
	metaBlock *block.Block,
	lastMetaBlock data.HeaderHandler,
) {
	if check.IfNil(mp.eventsProcessor) || !mp.eventsProcessor.Enabled() {
		return
	}

	pool := &indexer.Pool{
		Txs:  mp.txCoordinator.GetAllCurrentUsedTxs(),
		Logs: mp.txCoordinator.GetAllCurrentLogs(),
	}

	log.Debug("preparing to index block", "hash", headerHash, "nonce", metaBlock.GetNonce(), "slot", metaBlock.GetSlot())

	consensusValidators, err := mp.nodesCoordinator.ComputeConsensusGroup(metaBlock.GetPrevRandSeed(), metaBlock.GetSlot(), metaBlock.GetEpoch())
	if err != nil {
		return
	}

	var publicKeys []string
	for i, validator := range consensusValidators {
		validatorSigned := (metaBlock.PubKeysBitmap[i/8] & (1 << (uint16(i) % 8))) != 0 // #nosec G115
		if validatorSigned {
			publicKeys = append(publicKeys, hex.EncodeToString(validator.PubKey()))
		}
	}

	args := &indexer.ArgsSaveBlockData{
		HeaderHash:       headerHash,
		Header:           header,
		Signer:           metaBlock.GetProducerSignature(),
		TransactionsPool: pool,
		Validators:       publicKeys,
	}
	mp.eventsProcessor.SaveBlock(args)
	log.Debug("indexed block", "hash", headerHash, "nonce", metaBlock.GetNonce(), "slot", metaBlock.GetSlot())

	if lastMetaBlock != nil && lastMetaBlock.GetIsEpochStart() {
		mp.indexEpochInfo(metaBlock.GetEpoch())
		return
	}

	mp.indexValidatorsRating()
}

func (mp *metaProcessor) processEconomicsEndOfEpoch(validatorInfos []*state.ValidatorInfo, headerHandler data.HeaderHandler) error {
	err := mp.kAppController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(mp.epochStartTrigger.Epoch(), validatorInfos)
	if err != nil {
		return err
	}

	// Roll FPR
	err = mp.epochStartNativeStakingKapps(headerHandler)
	if err != nil {
		return err
	}

	return nil
}

// SetNumProcessedObj will set the num of processed transactions
func (mp *metaProcessor) SetNumProcessedObj(numObj uint64) {
	mp.txCounter.totalTxs = numObj
}

func (mp *metaProcessor) UpdateNativeStakingKapps(blk data.HeaderHandler) error {
	staking, klv, kfi, err := mp.getNativeStakingKApps()
	if err != nil {
		return err
	}

	// accumulate current epoch in tmp field, update slice on epoch start...
	klv.CurrentFPRAmount += blk.GetStakingRewards()
	kfi.CurrentFPRAmount += blk.GetKAppFees()

	return mp.setNativeStakingKApps(staking, klv, kfi)
}

func (mp *metaProcessor) epochStartNativeStakingKapps(blk data.HeaderHandler) error {
	staking, klv, kfi, err := mp.getNativeStakingKApps()
	if err != nil {
		return err
	}

	maxEpochUnclaimed := mp.proposalController.GetParameterInt(kapps.EnumParameter_MaxEpochsUnclaimed)

	amountKLVToBurn := int64(0)
	if len(klv.FPR) == int(maxEpochUnclaimed) {
		amountKLVToBurn = klv.FPR[0].TotalAmount - klv.FPR[0].TotalClaimed

		klv.FPR = klv.FPR[1:]
	}

	klv.FPR = append(klv.FPR, &kapps.FPRData{
		TotalAmount:  klv.GetCurrentFPRAmount(),
		TotalStaked:  klv.GetTotalStaked(),
		Epoch:        blk.GetEpoch(),
		TotalClaimed: 0,
	})

	amountKLVToBurnKFI := int64(0)
	if len(kfi.FPR) == int(maxEpochUnclaimed) {
		amountKLVToBurnKFI = kfi.FPR[0].TotalAmount - kfi.FPR[0].TotalClaimed

		kfi.FPR = kfi.FPR[1:]
	}

	kfi.FPR = append(kfi.FPR, &kapps.FPRData{
		TotalAmount:  kfi.GetCurrentFPRAmount(),
		TotalStaked:  kfi.GetTotalStaked(),
		Epoch:        blk.GetEpoch(),
		TotalClaimed: 0,
	})

	// Reset current FPR amount
	klv.CurrentFPRAmount = 0
	kfi.CurrentFPRAmount = 0

	burned := amountKLVToBurn + amountKLVToBurnKFI
	// Inflation = Block + Staking Rewards - TXFee Burned
	// Stking Rewards not claimed are burned in EpochStart
	// Burn unclaimed rewards
	err = mp.UpdateKLVCirculationSupply(0, burned)
	if err != nil {
		return err
	}

	blk.SetBurnedUnclaimed(burned)

	return mp.setNativeStakingKApps(staking, klv, kfi)
}

func (mp *metaProcessor) processProposalsEndOfEpoch(headerHandler data.HeaderHandler) error {
	log.Debug("Started Proposals Processing")

	proposalKApp, controller, err := mp.getProposalController()
	if err != nil {
		return err
	}

	// check if there are active proposals for the current epoch
	if controller.ActiveProposals[headerHandler.GetEpoch()] == nil {
		return nil
	}

	// retrieve staking data
	_, _, kfi, err := mp.getNativeStakingKApps()
	if err != nil {
		return err
	}

	proposalsToUpdate, err := mp.processAllProposals(proposalKApp, controller, kfi, headerHandler)
	if err != nil {
		return err
	}

	if !check.IfNil(mp.eventsProcessor) {
		mp.eventsProcessor.UpdateProposalsAndParameters(proposalsToUpdate)
	}
	delete(controller.ActiveProposals, headerHandler.GetEpoch())

	return mp.finalizeProposalUpdates(proposalKApp, controller)
}

func (mp *metaProcessor) processAllProposals(proposalKApp state.KAppAccountHandler, controller *kapps.ProposalController, kfi *kapps.StakingData, headerHandler data.HeaderHandler) ([]string, error) {
	proposalIDs := controller.ActiveProposals[headerHandler.GetEpoch()].ProposalIDs

	var proposalsToUpdate []string
	for _, proposalID := range proposalIDs {
		proposal, err := mp.processSingleProposal(proposalKApp, controller, kfi, proposalID)
		if err != nil {
			return nil, err
		}

		proposalsToUpdate = append(proposalsToUpdate, strconv.FormatUint(proposalID, 10))

		if err := mp.setProposalKApp(proposalKApp, proposalID, proposal); err != nil {
			return nil, err
		}
	}

	return proposalsToUpdate, nil
}

func (mp *metaProcessor) processSingleProposal(proposalKApp state.KAppAccountHandler, controller *kapps.ProposalController, kfi *kapps.StakingData, proposalID uint64) (*kapps.ProposalData, error) {
	proposal, err := mp.getProposalKApp(proposalKApp, proposalID)
	if err != nil {
		return nil, err
	}

	proposal.ProposalStatus = mp.determineProposalStatus(proposal, kfi)
	proposal.TotalStaked = kfi.TotalStaked

	// Update parameters if proposal is approved
	if proposal.ProposalStatus == kapps.ProposalData_ApprovedProposal {
		if err := mp.updateApprovedProposalParameters(proposal, controller); err != nil {
			return nil, err
		}
	}

	return proposal, nil
}

func (mp *metaProcessor) determineProposalStatus(proposal *kapps.ProposalData, kfi *kapps.StakingData) kapps.ProposalData_EnumProposalStatus {
	if proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)] > kfi.TotalStaked/2 {
		return kapps.ProposalData_ApprovedProposal
	}
	return kapps.ProposalData_DeniedProposal
}

func (mp *metaProcessor) updateApprovedProposalParameters(proposal *kapps.ProposalData, controller *kapps.ProposalController) error {
	for parameter, value := range proposal.Parameters {
		if err := mp.validateAndApplyParameter(parameter, value, controller); err != nil {
			return err
		}
	}
	return nil
}

func (mp *metaProcessor) validateAndApplyParameter(parameter int32, value []byte, controller *kapps.ProposalController) error {
	_, err := controller.ParseParamAndValidate(kapps.EnumParameter(parameter), value, mp.forkController)
	if err != nil {
		return err
	}

	controller.ActiveParameters[parameter].Value = make([]byte, len(value))
	copy(controller.ActiveParameters[parameter].Value, value)

	return nil
}

func (mp *metaProcessor) finalizeProposalUpdates(proposalKApp state.KAppAccountHandler, controller *kapps.ProposalController) error {
	if err := mp.setProposalController(proposalKApp, controller); err != nil {
		return err
	}

	if err := mp.accountsDB[state.KAppAccountsState].SaveAccount(proposalKApp); err != nil {
		return err
	}

	// After All processing, updates block activeParams instance
	mp.proposalController.UpdateParameters(controller.ActiveParameters)

	return nil
}
