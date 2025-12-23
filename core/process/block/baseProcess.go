package block

import (
	"bytes"
	"context"
	"fmt"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/display"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

var log = logger.GetOrCreate("process/block")

const (
	// Log parameter names
	LogHeaderTxRootHash   = "header tx root hash"
	LogComputedTxRootHash = "computed tx root hash"
)

type baseProcessor struct {
	nodesCoordinator             sharding.NodesCoordinator
	accountsDB                   map[state.AccountsDbIdentifier]state.AccountsAdapter
	forkDetector                 process.ForkDetector
	hasher                       hashing.Hasher
	marshalizer                  marshal.Marshalizer
	store                        retriever.StorageService
	uint64Converter              typeConverters.Uint64ByteSliceConverter
	blockSizeThrottler           process.BlockSizeThrottler
	epochStartTrigger            process.EpochStartTriggerHandler
	slotManager                  consensus.SlotManager
	bootStorer                   process.BootStorer
	requestHandler               process.RequestHandler
	dataPool                     retriever.PoolsHolder
	blockChain                   data.ChainHandler
	validatorStatisticsProcessor process.ValidatorStatisticsProcessor
	genesisNonce                 uint64
	txCoordinator                process.TransactionCoordinator
	feeHandler                   process.TransactionFeeHandler
	indexer                      process.Indexer
	blockChainHook               process.BlockChainHookHandler
	headerIntegrityVerifier      process.HeaderIntegrityVerifier
	appStatusHandler             core.AppStatusHandler
	stateCheckpointModulus       uint
	processingMode               core.NodeProcessingMode
	txCounter                    *transactionCounter

	tpsBenchmark statistics.TPSBenchmark

	kAppController kapp.KAppController

	epochNotifier      process.EpochNotifier
	forkController     core.ForkController
	proposalController kapps.ActiveProposalController
}

type bootStorerDataArgs struct {
	headerInfo                 *bootstrapStorage.BootstrapHeaderInfo
	slot                       uint64
	highestFinalBlockNonce     uint64
	nodesCoordinatorConfigKey  []byte
	epochStartTriggerConfigKey []byte
}

// SetAppStatusHandler method is used to set appStatusHandler
func (bp *baseProcessor) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return common.ErrNilAppStatusHandler
	}

	bp.appStatusHandler = ash
	return nil
}

// LoadProposalController will load the proposal controller into blockProcess instance
func (bp *baseProcessor) SetProposalController(controller kapps.ActiveProposalController) error {
	if check.IfNil(controller) {
		return common.ErrNilProposalController
	}

	bp.proposalController = controller

	return nil
}

// checkBlockValidity method checks if the given block is valid
func (bp *baseProcessor) checkBlockValidity(
	headerHandler data.HeaderHandler,
) error {
	if check.IfNil(headerHandler) {
		return process.ErrNilBlockHeader
	}

	currentBlockHeader := bp.blockChain.GetCurrentBlockHeader()

	if check.IfNil(currentBlockHeader) {
		return bp.validateGenesisBlock(headerHandler, currentBlockHeader)
	}

	if err := bp.validateBlockAgainstCurrentHeader(headerHandler, currentBlockHeader); err != nil {
		return err
	}

	return bp.validateBlockAndSlot(headerHandler)
}

// Helper function to validate the genesis block
func (bp *baseProcessor) validateGenesisBlock(headerHandler data.HeaderHandler, currentBlockHeader data.HeaderHandler) error {
	if check.IfNil(currentBlockHeader) {
		if headerHandler.GetNonce() == bp.genesisNonce+1 {
			if bytes.Equal(headerHandler.GetParentHash(), bp.blockChain.GetGenesisHeaderHash()) {
				// TODO: add genesis block verification
				return nil
			}
			log.Debug("hash does not match",
				"local genesis block hash", bp.blockChain.GetGenesisHeaderHash(),
				"received previous hash", headerHandler.GetParentHash())
			return process.ErrBlockHashDoesNotMatch
		}

		log.Debug("nonce does not match",
			"local block nonce", 0,
			"received nonce", headerHandler.GetNonce())
		return process.ErrWrongNonceInBlock
	}

	return nil
}

// Helper function to validate the block against the current block header
func (bp *baseProcessor) validateBlockAgainstCurrentHeader(headerHandler data.HeaderHandler, currentBlockHeader data.HeaderHandler) error {
	if headerHandler.GetSlot() <= currentBlockHeader.GetSlot() {
		log.Debug("slot does not match",
			"local block slot", currentBlockHeader.GetSlot(),
			"received block slot", headerHandler.GetSlot())
		return process.ErrLowerSlotInBlock
	}

	if headerHandler.GetNonce() != currentBlockHeader.GetNonce()+1 {
		log.Debug("nonce does not match",
			"local block nonce", currentBlockHeader.GetNonce(),
			"received nonce", headerHandler.GetNonce())
		return process.ErrWrongNonceInBlock
	}

	if !bytes.Equal(headerHandler.GetParentHash(), bp.blockChain.GetCurrentBlockHeaderHash()) {
		log.Debug("hash does not match",
			"local block hash", bp.blockChain.GetCurrentBlockHeaderHash(),
			"received previous hash", headerHandler.GetParentHash())
		return process.ErrBlockHashDoesNotMatch
	}

	if !bytes.Equal(headerHandler.GetPrevRandSeed(), currentBlockHeader.GetRandSeed()) {
		log.Debug("random seed does not match",
			"local random seed", currentBlockHeader.GetRandSeed(),
			"received previous random seed", headerHandler.GetPrevRandSeed())
		return process.ErrRandSeedDoesNotMatch
	}

	// verification of epoch
	if headerHandler.GetEpoch() < currentBlockHeader.GetEpoch() {
		return process.ErrEpochDoesNotMatch
	}

	return nil
}

func (bp *baseProcessor) validateBlockAndSlot(headerHandler data.HeaderHandler) error {
	if !bp.slotManager.ValidateSlotTimestamp(tools.SafeU64ToI64(headerHandler.GetSlot()), headerHandler.GetTimestamp()) {
		return process.ErrInvalidBlockTimestamp
	}

	// #nosec G115
	if headerHandler.GetTxCount() != uint32(len(headerHandler.GetTxHashes())) {
		log.Error("checkBlockValidity tx count does not match",
			"count", headerHandler.GetTxCount(),
			"len", len(headerHandler.GetTxHashes()))

		return process.ErrInvalidTXCount
	}

	burnnedTxFees := headerHandler.GetTxFees() - bp.validatorStatisticsProcessor.LeaderRewards(headerHandler.GetTxFees())
	if headerHandler.GetTxBurnedFees() != burnnedTxFees {
		return process.ErrInvalidTXFees
	}

	return bp.validateTxRootHash(headerHandler)
}

// Helper function to validate zero transaction root hash prior/after fork
func (bp *baseProcessor) validateZeroTXRootHash(headerHandler data.HeaderHandler) error {
	if headerHandler.GetTxRootHash() == nil {
		return nil
	}

	// must allow if zero tx root hash for empty block prior fork
	if !bp.forkController.EnableSmartContracts() {
		if !bytes.Equal(headerHandler.GetTxRootHash(), bp.hasher.EmptyHash()) {
			log.Debug("tx root hash does not match for empty block",
				LogHeaderTxRootHash, headerHandler.GetTxRootHash(),
				LogComputedTxRootHash, bp.hasher.EmptyHash())
			return process.ErrTxRootHashDoesNotMatch
		}

		return nil
	}

	log.Debug("invalid block hash with no transactions",
		LogHeaderTxRootHash, headerHandler.GetTxRootHash())
	return process.ErrTxRootHashInvalidForEmptyBlock

}

// Helper function to validate transaction root hash
func (bp *baseProcessor) validateTxRootHash(headerHandler data.HeaderHandler) error {
	if headerHandler.GetTxCount() == 0 {
		return bp.validateZeroTXRootHash(headerHandler)
	}

	txRootHash, err := headerHandler.ComputeRootHash(bp.hasher)
	if err != nil {
		return err
	}
	if !bytes.Equal(headerHandler.GetTxRootHash(), txRootHash) {
		log.Debug("tx root hash does not match",
			LogHeaderTxRootHash, headerHandler.GetTxRootHash(),
			LogComputedTxRootHash, txRootHash)
		return process.ErrTxRootHashDoesNotMatch
	}

	return nil
}

// verifyStateRootAccount verifies the state root hash given as parameter against the
// Merkle trie root hash stored for accounts and returns if equal or not
func (bp *baseProcessor) verifyStateRootAccount(rootHash []byte) bool {
	trieRootHash, err := bp.accountsDB[state.UserAccountsState].RootHash()
	if err != nil {
		log.Debug("verify account.RootHash", "error", err.Error())
	}

	return bytes.Equal(trieRootHash, rootHash)
}

// getRootHashAccount returns the accounts merkle tree root hash
func (bp *baseProcessor) getRootHashAccount() []byte {
	rootHash, err := bp.accountsDB[state.UserAccountsState].RootHash()
	if err != nil {
		log.Trace("get account.RootHash", "error", err.Error())
	}

	return rootHash
}

// verifyStateRootKApp verifies the state root hash given as parameter against the
// Merkle trie root hash stored for kapps and returns if equal or not
func (bp *baseProcessor) verifyStateRootKApp(rootHash []byte) bool {
	trieRootHash, err := bp.accountsDB[state.KAppAccountsState].RootHash()
	if err != nil {
		log.Debug("verify kapps.RootHash", "error", err.Error())
	}

	return bytes.Equal(trieRootHash, rootHash)
}

// getRootHashKApp returns the accounts merkle tree root hash
func (bp *baseProcessor) getRootHashKApp() []byte {
	rootHash, err := bp.accountsDB[state.KAppAccountsState].RootHash()
	if err != nil {
		log.Trace("get account.RootHash", "error", err.Error())
	}

	return rootHash
}

func displayHeader(headerHandler data.HeaderHandler, headerHash []byte) []*display.LineData {
	return []*display.LineData{
		display.NewLineData(false, []string{
			"",
			"ChainID",
			string(headerHandler.GetChainID())}),
		display.NewLineData(false, []string{
			"",
			"Epoch",
			fmt.Sprintf("%d", headerHandler.GetEpoch())}),
		display.NewLineData(false, []string{
			"",
			"IsEpochStart",
			fmt.Sprintf("%t", headerHandler.GetIsEpochStart())}),
		display.NewLineData(false, []string{
			"",
			"Slot",
			fmt.Sprintf("%d", headerHandler.GetSlot())}),
		display.NewLineData(false, []string{
			"",
			"Timestamp",
			fmt.Sprintf("%d", headerHandler.GetTimestamp())}),
		display.NewLineData(false, []string{
			"",
			"Nonce",
			fmt.Sprintf("%d", headerHandler.GetNonce())}),
		display.NewLineData(false, []string{
			"",
			"Prev hash",
			logger.DisplayByteSlice(headerHandler.GetParentHash())}),
		display.NewLineData(false, []string{
			"",
			"Block hash",
			logger.DisplayByteSlice(headerHash)}),
		display.NewLineData(false, []string{
			"",
			"Prev rand seed",
			logger.DisplayByteSlice(headerHandler.GetPrevRandSeed())}),
		display.NewLineData(false, []string{
			"",
			"Rand seed",
			logger.DisplayByteSlice(headerHandler.GetRandSeed())}),
		display.NewLineData(false, []string{
			"",
			"Signature",
			logger.DisplayByteSlice(headerHandler.GetSignature())}),
		display.NewLineData(false, []string{
			"",
			"TxRootHash",
			logger.DisplayByteSlice(headerHandler.GetTxRootHash())}),
		display.NewLineData(false, []string{
			"",
			"Root hash",
			logger.DisplayByteSlice(headerHandler.GetTrieRoot())}),
		display.NewLineData(false, []string{
			"",
			"Validator stats root hash",
			logger.DisplayByteSlice(headerHandler.GetValidatorsTrieRoot())}),
		display.NewLineData(false, []string{
			"",
			"KApp root hash",
			logger.DisplayByteSlice(headerHandler.GetKAppsTrieRoot())}),
	}
}

// checkProcessorNilParameters will check the input parameters for nil values
func checkProcessorNilParameters(arguments ArgBaseProcessor) error {

	for key := range arguments.AccountsDB {
		if check.IfNil(arguments.AccountsDB[key]) {
			return common.ErrNilAccountsAdapter
		}
	}
	if check.IfNil(arguments.ForkDetector) {
		return common.ErrNilForkDetector
	}
	if check.IfNil(arguments.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arguments.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arguments.Store) {
		return common.ErrNilStorage
	}
	if check.IfNil(arguments.NodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(arguments.Uint64Converter) {
		return process.ErrNilUint64Converter
	}
	if check.IfNil(arguments.RequestHandler) {
		return common.ErrNilRequestHandler
	}
	if check.IfNil(arguments.EpochStartTrigger) {
		return common.ErrNilEpochStartTrigger
	}
	if check.IfNil(arguments.SlotManager) {
		return process.ErrNilSlotManager
	}
	if check.IfNil(arguments.BootStorer) {
		return common.ErrNilStorage
	}
	if check.IfNil(arguments.TxCoordinator) {
		return process.ErrNilTransactionCoordinator
	}
	if check.IfNil(arguments.FeeHandler) {
		return process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(arguments.BlockChain) {
		return process.ErrNilBlockChain
	}
	if check.IfNil(arguments.HeaderIntegrityVerifier) {
		return common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(arguments.BlockSizeThrottler) {
		return process.ErrNilBlockSizeThrottler
	}
	if check.IfNil(arguments.TpsBenchmark) {
		return process.ErrNilTpsBenchmark
	}
	if check.IfNil(arguments.EpochNotifier) {
		return common.ErrNilEpochNotifier
	}
	if check.IfNil(arguments.BlockChainHook) {
		return common.ErrNilBlockchainHooks
	}

	return nil
}

func (bp *baseProcessor) createBlockStarted() {
	bp.txCoordinator.CreateBlockStarted()
	bp.feeHandler.CreateBlockStarted()
}

func (bp *baseProcessor) verifyFees(header data.HeaderHandler) error {
	if !bp.forkController.FixStakingBuckets() {
		return nil
	}

	// check if block/staking rewards are valid
	if header.GetBlockRewards() !=
		bp.proposalController.GetParameterInt(kapps.EnumParameter_BlockRewards) ||
		header.GetStakingRewards() !=
			bp.proposalController.GetParameterInt(kapps.EnumParameter_StakingRewards) {

		log.Error("checkBlockValidity verifyFees rewards does not match",
			"block rewards", header.GetBlockRewards(),
			"staking rewards", header.GetStakingRewards())

		return process.ErrInvalidMiningRewards
	}

	return nil
}

func (bp *baseProcessor) cleanupPools(headerHandler data.HeaderHandler) {
	bp.removeHeadersBehindNonceFromPools(true, bp.forkDetector.GetHighestFinalBlockNonce())
}

func (bp *baseProcessor) removeHeadersBehindNonceFromPools(
	shouldRemoveBlockHeader bool,
	nonce uint64,
) {
	if nonce <= 1 {
		return
	}

	headersPool := bp.dataPool.Headers()
	nonces := headersPool.Nonces()
	for _, nonceFromCache := range nonces {
		if nonceFromCache >= nonce {
			continue
		}

		if shouldRemoveBlockHeader {
			bp.removeBlocksHeader(nonceFromCache)
		}

		headersPool.RemoveHeaderByNonce(nonceFromCache)
	}
}

func (bp *baseProcessor) removeBlocksHeader(nonce uint64) {
	headersPool := bp.dataPool.Headers()
	headers, _, err := headersPool.GetHeadersByNonce(nonce)
	if err != nil {
		return
	}

	for _, header := range headers {
		errNotCritical := bp.removeBlockDataFromPools(header)
		if errNotCritical != nil {
			log.Debug("RemoveBlockDataFromPool", "error", errNotCritical.Error())
		}
	}
}

func (bp *baseProcessor) removeBlockDataFromPools(headerHandler data.HeaderHandler) error {
	// remote TX from pool
	body, ok := headerHandler.(*block.Block)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	err := bp.txCoordinator.RemoveBlockDataFromPool(body)
	if err != nil {
		return err
	}

	return nil
}

func (bp *baseProcessor) removeTxsFromPools(blck *block.Block) error {
	return bp.txCoordinator.RemoveTxsFromPool(blck)
}

func (bp *baseProcessor) prepareDataForBootStorer(args *bootStorerDataArgs) {

	bootData := &bootstrapStorage.BootstrapData{
		LastHeader:                 args.headerInfo,
		HighestFinalBlockNonce:     args.highestFinalBlockNonce,
		NodesCoordinatorConfigKey:  args.nodesCoordinatorConfigKey,
		EpochStartTriggerConfigKey: args.epochStartTriggerConfigKey,
	}

	startTime := time.Now()

	err := bp.bootStorer.Put(tools.SafeU64ToI64(args.slot), bootData)
	if err != nil {
		log.Warn("cannot save boot data in storage",
			"error", err.Error())
	}

	elapsedTime := time.Since(startTime)
	if elapsedTime >= core.PutInStorerMaxTime {
		log.Warn("saveDataForBootStorer", "elapsed time", elapsedTime)
	}
}

// DecodeBlockHeader method decodes block header from a given byte array
func (bp *baseProcessor) DecodeBlockHeader(dta []byte) data.HeaderHandler {
	if dta == nil {
		return nil
	}

	header := bp.blockChain.CreateNewHeader()

	err := bp.marshalizer.Unmarshal(header, dta)
	if err != nil {
		log.Debug("DecodeBlockHeader.Unmarshal", "error", err.Error())
		return nil
	}

	return header
}

func (bp *baseProcessor) saveBody(blck *block.Block) {
	startTime := time.Now()

	errNotCritical := bp.txCoordinator.SaveTxsToStorage(blck)
	if errNotCritical != nil {
		log.Warn("saveBody.SaveTxsToStorage", "error", errNotCritical.Error())
	}

	elapsedTime := time.Since(startTime)

	if elapsedTime >= core.PutInStorerMaxTime {
		log.Warn("saveBody", "elapsed time", elapsedTime)
	} else {
		log.Trace("saveBody.SaveTxsToStorage", "time", elapsedTime)
	}
}

func (bp *baseProcessor) saveHeader(header data.HeaderHandler, headerHash []byte, marshalizedHeader []byte) {
	startTime := time.Now()

	nonceToByteSlice := bp.uint64Converter.ToByteSlice(header.GetNonce())

	errNotCritical := bp.store.Put(retriever.HdrNonceHashDataUnit, nonceToByteSlice, headerHash)
	if errNotCritical != nil {
		log.Warn("saveHeader.Put -> HdrNonceHashDataUnit", "error", errNotCritical.Error())
	}

	errNotCritical = bp.store.Put(retriever.BlockUnit, headerHash, marshalizedHeader)
	if errNotCritical != nil {
		log.Warn("saveHeader.Put -> BlockUnit", "error", errNotCritical.Error())
	}

	elapsedTime := time.Since(startTime)
	if elapsedTime >= core.PutInStorerMaxTime {
		log.Warn("saveHeader", "elapsed time", elapsedTime)
	}
}

func (bp *baseProcessor) updateStateStorage(
	finalHeader data.HeaderHandler,
	rootHash []byte,
	prevRootHash []byte,
	accounts state.AccountsAdapter,
) {
	if !accounts.IsPruningEnabled() {
		return
	}

	// Skip checkpoints during import-db mode to prevent pruning buffer overflow (see KLC-2057)
	if bp.processingMode == core.ImportDb {
		log.Trace("skipping checkpoint in import-db mode", "rootHash", rootHash)
	} else if bp.stateCheckpointModulus != 0 {
		if finalHeader.GetNonce()%uint64(bp.stateCheckpointModulus) == 0 {
			log.Debug("trie checkpoint", "rootHash", rootHash)
			ctx := context.Background()
			accounts.SetStateCheckpoint(rootHash, ctx)
		}
	}

	if bytes.Equal(prevRootHash, rootHash) {
		return
	}

	accounts.CancelPrune(prevRootHash, data.NewRoot)
	accounts.PruneTrie(prevRootHash, data.OldRoot)
}

// RevertStateToSnapshot reverts the account state for cleanup failed process
func (bp *baseProcessor) RevertStateToSnapshot(_ data.HeaderHandler) {
	for key := range bp.accountsDB {
		err := bp.accountsDB[key].RevertToSnapshot(0)
		if err != nil {
			log.Debug("RevertToSnapshotAccount", "error", err.Error())
		}
	}
}

func (bp *baseProcessor) commitAll() error {
	for key := range bp.accountsDB {
		_, err := bp.accountsDB[key].Commit()
		if err != nil {
			return err
		}
	}

	return nil
}

// PruneStateOnRollback recreates the state tries to the root hashes indicated by the provided header
func (bp *baseProcessor) PruneStateOnRollback(currHeader data.HeaderHandler, prevHeader data.HeaderHandler) {
	for key := range bp.accountsDB {
		if !bp.accountsDB[key].IsPruningEnabled() {
			continue
		}

		rootHash, prevRootHash := bp.getAccountRootHashes(currHeader, prevHeader, key)

		if bytes.Equal(rootHash, prevRootHash) {
			continue
		}

		bp.accountsDB[key].CancelPrune(prevRootHash, data.OldRoot)
		bp.accountsDB[key].PruneTrie(rootHash, data.NewRoot)
	}
}

func (bp *baseProcessor) getAccountRootHashes(
	currHeader data.HeaderHandler,
	prevHeader data.HeaderHandler,
	identifier state.AccountsDbIdentifier,
) ([]byte, []byte) {
	switch identifier {
	case state.UserAccountsState:
		return currHeader.GetTrieRoot(), prevHeader.GetTrieRoot()
	case state.PeerAccountsState:
		return currHeader.GetValidatorsTrieRoot(), prevHeader.GetValidatorsTrieRoot()
	case state.KAppAccountsState:
		return currHeader.GetKAppsTrieRoot(), prevHeader.GetKAppsTrieRoot()
	default:
		return []byte{}, []byte{}
	}
}

func (mp *metaProcessor) getNativeStakingKApps() (state.KAppAccountHandler, *kapps.StakingData, *kapps.StakingData, error) {
	acnt, err := mp.accountsDB[state.KAppAccountsState].LoadAccount(kapps.StakingKAppAddress)
	if err != nil {
		return nil, nil, nil, err
	}

	stakingKapp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return nil, nil, nil, common.ErrWrongTypeAssertion
	}

	klvStakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kdautils.KLVKey)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(klvStakedBytes) == 0 {
		return nil, nil, nil, common.ErrEmptyString
	}

	kfiStakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kdautils.KFIKey)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(kfiStakedBytes) == 0 {
		return nil, nil, nil, common.ErrEmptyString
	}

	klvStaking := &kapps.StakingData{}
	err = mp.marshalizer.Unmarshal(klvStaking, klvStakedBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	kfiStaking := &kapps.StakingData{}
	err = mp.marshalizer.Unmarshal(kfiStaking, kfiStakedBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	return stakingKapp, klvStaking, kfiStaking, nil
}

func (mp *metaProcessor) setNativeStakingKApps(stakingKapp state.KAppAccountHandler, klvStaking, kfiStaking *kapps.StakingData) error {
	klvData, err := mp.marshalizer.Marshal(klvStaking)
	if err != nil {
		return err
	}

	kfiData, err := mp.marshalizer.Marshal(kfiStaking)
	if err != nil {
		return err
	}

	err = stakingKapp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, klvData)
	if err != nil {
		return err
	}

	err = stakingKapp.DataTrieTracker().SaveKeyValue(kdautils.KFIKey, kfiData)
	if err != nil {
		return err
	}

	return mp.accountsDB[state.KAppAccountsState].SaveAccount(stakingKapp)
}

func (mp *metaProcessor) UpdateKLVCirculationSupply(amountMint int64, amountBurn int64) error {
	acnt, err := mp.accountsDB[state.KAppAccountsState].LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return err
	}

	kdaKapp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	KDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(kdautils.KLVKey)
	if err != nil {
		return err
	}
	if len(KDABytes) == 0 {
		return common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = mp.marshalizer.Unmarshal(kda, KDABytes)
	if err != nil {
		return err
	}

	kda.CirculatingSupply += (amountMint - amountBurn)
	kda.MintedValue += amountMint
	kda.BurnedValue += amountBurn

	kdaData, err := mp.marshalizer.Marshal(kda)
	if err != nil {
		return err
	}

	err = kdaKapp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, kdaData)
	if err != nil {
		return err
	}

	return mp.accountsDB[state.KAppAccountsState].SaveAccount(kdaKapp)
}

func (mp *metaProcessor) getProposalController() (state.KAppAccountHandler, *kapps.ProposalController, error) {
	acnt, err := mp.accountsDB[state.KAppAccountsState].LoadAccount(kapps.ProposalKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	proposalKApp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return nil, nil, common.ErrWrongTypeAssertion
	}

	controllerBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(kdautils.ProposalControllerKey)
	if err != nil {
		return nil, nil, err
	}

	currentProposal := &kapps.ProposalController{}
	err = mp.marshalizer.Unmarshal(currentProposal, controllerBytes)
	if err != nil {
		return nil, nil, err
	}

	initialProposal, err := kapps.NewProposalController(mp.forkController)
	if err != nil {
		return nil, nil, err
	}

	if len(initialProposal.ActiveParameters) > len(currentProposal.ActiveParameters) {
		for key, v := range currentProposal.ActiveParameters {
			initialProposal.ActiveParameters[key] = v
		}

		currentProposal.ActiveParameters = initialProposal.ActiveParameters

		proposalData, err := mp.marshalizer.Marshal(currentProposal)
		if err != nil {
			return nil, nil, err
		}

		err = proposalKApp.DataTrieTracker().SaveKeyValue(kdautils.ProposalControllerKey, proposalData)
		if err != nil {
			return nil, nil, err
		}

		mp.proposalController.UpdateParameters(currentProposal.ActiveParameters)
	}

	return proposalKApp, currentProposal, nil
}

func (mp *metaProcessor) getProposalKApp(proposalKApp state.KAppAccountHandler, proposalID uint64) (*kapps.ProposalData, error) {
	proposal := &kapps.ProposalData{}
	if proposalID != 0 {
		key := kdautils.ToProposalKey(proposalID)

		proposalBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(key)
		if err != nil {
			return nil, err
		}
		if len(proposalBytes) == 0 {
			return nil, common.ErrEmptyString
		}

		err = mp.marshalizer.Unmarshal(proposal, proposalBytes)
		if err != nil {
			return nil, err
		}
	}

	return proposal, nil
}

func (mp *metaProcessor) setProposalController(kdaKapp state.KAppAccountHandler, controller *kapps.ProposalController) error {
	data, err := mp.marshalizer.Marshal(controller)
	if err != nil {
		return err
	}

	err = kdaKapp.DataTrieTracker().SaveKeyValue(kdautils.ProposalControllerKey, data)
	if err != nil {
		return err
	}

	return nil
}

func (mp *metaProcessor) setProposalKApp(kdaKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData) error {
	data, err := mp.marshalizer.Marshal(proposal)
	if err != nil {
		return err
	}

	key := kdautils.ToProposalKey(proposalID)
	err = kdaKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}
