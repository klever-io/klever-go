package peer

import (
	"context"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("process/peer")

var _ process.ValidatorStatisticsProcessor = (*validatorStatistics)(nil)

// ArgValidatorStatisticsProcessor holds all dependencies for the validatorStatistics
type ArgValidatorStatisticsProcessor struct {
	Marshalizer        marshal.Marshalizer
	NodesCoordinator   sharding.NodesCoordinator
	DataPool           DataPool
	StorageService     retriever.StorageService
	PubkeyConv         core.PubkeyConverter
	PeerAdapter        state.AccountsAdapter
	KAppAdapter        state.AccountsAdapter
	Rater              sharding.PeerAccountListAndRatingHandler
	RewardsHandler     process.EconomicsDataHandler
	MaxComputableSlots uint64
	NodesSetup         sharding.GenesisNodesSetupHandler
	GenesisNonce       uint64
	EpochNotifier      process.EpochNotifier
	VKApp              kapp.ValidatorsKapp
	ForkController     core.ForkController
}

type validatorStatistics struct {
	marshalizer            marshal.Marshalizer
	dataPool               DataPool
	storageService         retriever.StorageService
	nodesCoordinator       sharding.NodesCoordinator
	pubkeyConv             core.PubkeyConverter
	peerAdapter            state.AccountsAdapter
	vKApp                  kapp.ValidatorsKapp
	rater                  sharding.PeerAccountListAndRatingHandler
	rewardsHandler         process.EconomicsDataHandler
	maxComputableSlots     uint64
	missedBlocksCounters   validatorSlotCounters
	mutValidatorStatistics sync.RWMutex
	genesisNonce           uint64
	forkController         core.ForkController
	lastFinalizedRootHash  []byte
}

// NewValidatorStatisticsProcessor instantiates a new validatorStatistics structure responsible of keeping account of
// each validator actions in the consensus process
func NewValidatorStatisticsProcessor(arguments ArgValidatorStatisticsProcessor) (*validatorStatistics, error) {
	if check.IfNil(arguments.PeerAdapter) {
		return nil, common.ErrNilPeerAccountsAdapter
	}
	if check.IfNil(arguments.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(arguments.DataPool) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(arguments.StorageService) {
		return nil, common.ErrNilStorage
	}
	if check.IfNil(arguments.NodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(arguments.Marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(arguments.Rater) {
		return nil, common.ErrNilRater
	}
	if check.IfNil(arguments.RewardsHandler) {
		return nil, common.ErrNilRewardsHandler
	}
	if check.IfNil(arguments.NodesSetup) {
		return nil, common.ErrNilNodesSetup
	}
	if check.IfNil(arguments.EpochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}
	if check.IfNil(arguments.VKApp) {
		return nil, common.ErrNilKAppValidator
	}

	// default value for maxComputableSlots to 100
	if arguments.MaxComputableSlots == 0 {
		arguments.MaxComputableSlots = process.DefaultMaxComputableSlots
	}

	arguments.VKApp.SetRater(arguments.Rater)

	vs := &validatorStatistics{
		peerAdapter:          arguments.PeerAdapter,
		pubkeyConv:           arguments.PubkeyConv,
		nodesCoordinator:     arguments.NodesCoordinator,
		dataPool:             arguments.DataPool,
		storageService:       arguments.StorageService,
		marshalizer:          arguments.Marshalizer,
		missedBlocksCounters: make(validatorSlotCounters),
		rater:                arguments.Rater,
		rewardsHandler:       arguments.RewardsHandler,
		maxComputableSlots:   arguments.MaxComputableSlots,
		genesisNonce:         arguments.GenesisNonce,
		forkController:       arguments.ForkController,
		vKApp:                arguments.VKApp,
	}

	arguments.EpochNotifier.RegisterNotifyHandler(vs)

	return vs, nil
}

// saveNodesCoordinatorUpdates is called at the first block after start in epoch to update the state trie according to
// the shuffling and changes done by the nodesCoordinator at the end of the epoch
func (vs *validatorStatistics) saveNodesCoordinatorUpdates(epoch uint32) (bool, error) {
	log.Debug("save nodes coordinator updates ", "epoch", epoch)

	nodesMap, err := vs.nodesCoordinator.GetAllElectedValidatorsKeys(epoch, true)
	if err != nil {
		return false, err
	}

	var tmpNodeForcedToRemain, nodeForcedToRemain bool
	tmpNodeForcedToRemain, err = vs.vKApp.SaveUpdatesForNodesMap(nodesMap, state.List_elected.String())
	if err != nil {
		return false, err
	}
	nodeForcedToRemain = nodeForcedToRemain || tmpNodeForcedToRemain

	nodesMap, err = vs.nodesCoordinator.GetAllEligibleValidatorsKeys(epoch, true)
	if err != nil {
		return false, err
	}

	tmpNodeForcedToRemain, err = vs.vKApp.SaveUpdatesForNodesMap(nodesMap, state.List_eligible.String())
	if err != nil {
		return false, err
	}
	nodeForcedToRemain = nodeForcedToRemain || tmpNodeForcedToRemain

	nodesMap, err = vs.nodesCoordinator.GetAllWaitingValidatorsKeys(epoch, true)
	if err != nil {
		return false, err
	}

	tmpNodeForcedToRemain, err = vs.vKApp.SaveUpdatesForNodesMap(nodesMap, state.List_waiting.String())
	if err != nil {
		return false, err
	}
	nodeForcedToRemain = nodeForcedToRemain || tmpNodeForcedToRemain

	return nodeForcedToRemain, nil
}

// SaveNodesCoordinatorUpdates saves the results from the nodes coordinator changes after end of epoch
func (vs *validatorStatistics) SaveNodesCoordinatorUpdates(epoch uint32) (bool, error) {
	nodeForcedToRemain, err := vs.saveNodesCoordinatorUpdates(epoch)
	if err != nil {
		log.Error("load validator failed", "error", err.Error())
		return nodeForcedToRemain, err
	}

	return nodeForcedToRemain, nil
}

// UpdatePeerState takes a header, updates the peer state for all of the
// consensus members and returns the new root hash
func (vs *validatorStatistics) UpdatePeerState(header data.HeaderHandler, previousHeader data.HeaderHandler) ([]byte, error) {
	if header.GetNonce() == vs.genesisNonce {
		return vs.peerAdapter.RootHash()
	}

	vs.mutValidatorStatistics.Lock()
	vs.missedBlocksCounters.reset()
	vs.mutValidatorStatistics.Unlock()

	epoch := ComputeEpoch(header)
	err := vs.checkForMissedBlocks(
		header.GetSlot(),
		previousHeader.GetSlot(),
		previousHeader.GetRandSeed(),
		epoch,
	)
	if err != nil {
		return nil, err
	}

	err = vs.updateMissedBlocksCounters()
	if err != nil {
		return nil, err
	}

	if header.GetNonce() == vs.genesisNonce+1 {
		return vs.peerAdapter.RootHash()
	}
	log.Trace("Increasing", "slot", previousHeader.GetSlot(), "prevRandSeed", previousHeader.GetPrevRandSeed())

	consensusGroupEpoch := ComputeEpoch(previousHeader)
	consensusGroup, err := vs.nodesCoordinator.ComputeConsensusGroup(
		previousHeader.GetPrevRandSeed(),
		previousHeader.GetSlot(),
		consensusGroupEpoch)
	if err != nil {
		return nil, err
	}
	leaderPK := tools.GetTrimmedPk(vs.pubkeyConv.Encode(consensusGroup[0].PubKey()))
	log.Trace("Increasing for leader", "leader", leaderPK, "slot", previousHeader.GetSlot())

	accumulatedFees := (previousHeader.GetTxFees() - previousHeader.GetTxBurnedFees()) + previousHeader.GetBlockRewards()

	err = vs.updateValidatorInfoOnSuccessfulBlock(
		consensusGroup,
		previousHeader.GetPubKeysBitmap(),
		accumulatedFees,
	)
	if err != nil {
		return nil, err
	}

	rootHash, err := vs.peerAdapter.RootHash()
	if err != nil {
		return nil, err
	}

	log.Trace("after updating validator stats", "rootHash", rootHash, "slot", header.GetSlot())

	return rootHash, nil
}

func ComputeEpoch(currentHeader data.HeaderHandler) uint32 {
	// If it is epoch start the Block is proposed by the consensus group of the previous epoch
	// so we need to decrease the epoch
	// If the epoch is 0 we don't need to decrease it (Genesis block)
	epoch := currentHeader.GetEpoch()
	if currentHeader.GetIsEpochStart() && epoch > 0 {
		// we always decrease the epoch by 1
		// blockchain ensures that every epoch have at least one block
		// eventNotifier/EpochStart const minSlotsToTrigger = uint64(5)
		// so we can safely decrease the epoch by 1 ()
		epoch = epoch - 1
	}

	return epoch
}

// DisplayRatings will print the ratings
func (vs *validatorStatistics) DisplayRatings(epoch uint32) {
	validatorOwners, err := vs.nodesCoordinator.GetAllElectedValidatorsKeys(epoch, true)
	if err != nil {
		log.Warn("could not get ValidatorKeys", "epoch", epoch)
		return
	}
	log.Trace("started printing tempRatings")

	vs.vKApp.DisplayRating(validatorOwners)

	log.Trace("finished printing tempRatings")
}

// Commit commits the validator statistics trie and returns the root hash
func (vs *validatorStatistics) Commit() ([]byte, error) {
	return vs.peerAdapter.Commit()
}

// RootHash returns the root hash of the validator statistics trie
func (vs *validatorStatistics) RootHash() ([]byte, error) {
	return vs.peerAdapter.RootHash()
}

func (vs *validatorStatistics) PeerAccountToValidatorInfo(peer state.PeerAccountHandler) *state.ValidatorInfo {
	return &state.ValidatorInfo{
		OwnerAddress:                    peer.GetOwnerAddress(),
		PublicKey:                       peer.GetBLSPublicKey(),
		List:                            peer.GetListString(),
		TempRating:                      peer.GetTempRating(),
		Rating:                          peer.GetRating(),
		RatingModifier:                  0,
		LeaderSuccess:                   peer.GetLeaderSuccessRateSuccess(),
		LeaderFailure:                   peer.GetLeaderSuccessRateFailure(),
		ValidatorSuccess:                peer.GetValidatorSuccessRateSuccess(),
		ValidatorFailure:                peer.GetValidatorSuccessRateFailure(),
		ValidatorIgnoredSignatures:      peer.GetValidatorIgnoredSignaturesRate(),
		TotalLeaderSuccess:              peer.GetTotalLeaderSuccessRateSuccess(),
		TotalLeaderFailure:              peer.GetTotalLeaderSuccessRateFailure(),
		TotalValidatorSuccess:           peer.GetTotalValidatorSuccessRateSuccess(),
		TotalValidatorFailure:           peer.GetTotalValidatorSuccessRateFailure(),
		TotalValidatorIgnoredSignatures: peer.GetTotalValidatorIgnoredSignaturesRate(),
		NumSelectedInSuccessBlocks:      peer.GetNumSelectedInSuccessBlocks(),
		IsPubKeyRevoked:                 peer.GetRevoked(),
	}
}

func (vs *validatorStatistics) getValidatorDataFromLeaves(
	leavesChannels *data.TrieIteratorChannels,
) ([]*state.ValidatorInfo, error) {

	validators := make([]*state.ValidatorInfo, 0)

	err := leavesChannels.ForEach(func(pa data.KeyValueHolder) error {
		peerAccount, errUnmarshal := vs.unmarshalPeer(pa.Value())
		if errUnmarshal != nil {
			return errUnmarshal
		}

		if vs.forkController.FixStakingBuckets() &&
			peerAccount.GetRevoked() {
			return nil
		}

		validatorInfoData := vs.PeerAccountToValidatorInfo(peerAccount)
		validators = append(validators, validatorInfoData)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return validators, nil
}

// ListPeerAccounts list the peers starting on a given roothash for the state trie
func (vs *validatorStatistics) ListPeerAccounts(rootHash []byte) ([]state.PeerAccountHandler, error) {
	sw := tools.NewStopWatch()
	sw.Start("ListPeerAccounts")
	defer func() {
		sw.Stop("ListPeerAccounts")
		log.Debug("ListPeerAccounts", sw.GetMeasurements()...)
	}()

	ctx := context.Background()
	leavesChannels, err := vs.peerAdapter.GetAllLeaves(rootHash, ctx)
	if err != nil {
		return nil, err
	}

	vInfos, err := vs.makePeerAccountFromBuffer(leavesChannels)
	if err != nil {
		return nil, err
	}

	return vInfos, err
}

func (vs *validatorStatistics) makePeerAccountFromBuffer(leavesChannels *data.TrieIteratorChannels) ([]state.PeerAccountHandler, error) {
	peers := make([]state.PeerAccountHandler, 0)

	err := leavesChannels.ForEach(func(pa data.KeyValueHolder) error {
		peerAccount, errUnmarshal := vs.unmarshalPeer(pa.Value())
		if errUnmarshal != nil {
			return errUnmarshal
		}

		peers = append(peers, peerAccount)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (vs *validatorStatistics) unmarshalPeer(pa []byte) (state.PeerAccountHandler, error) {
	peerAccount := state.NewEmptyPeerAccount()
	err := vs.marshalizer.Unmarshal(peerAccount, pa)
	if err != nil {
		return nil, err
	}
	return peerAccount, nil
}

// GetValidatorInfoForRootHash returns all the peer accounts from the trie with the given rootHash
func (vs *validatorStatistics) GetValidatorInfoForRootHash(rootHash []byte) ([]*state.ValidatorInfo, error) {
	sw := tools.NewStopWatch()
	sw.Start("GetValidatorInfoForRootHash")
	defer func() {
		sw.Stop("GetValidatorInfoForRootHash")
		log.Debug("GetValidatorInfoForRootHash", sw.GetMeasurements()...)
	}()

	ctx := context.Background()
	leavesChannels, err := vs.peerAdapter.GetAllLeaves(rootHash, ctx)
	if err != nil {
		return nil, err
	}

	vInfos, err := vs.getValidatorDataFromLeaves(leavesChannels)
	if err != nil {
		return nil, err
	}

	return vInfos, err
}

func (vs *validatorStatistics) GetValidatorAccountRootHash(rootHash []byte) ([]kapp.ValidatorAccountInfoHandler, error) {
	sw := tools.NewStopWatch()
	sw.Start("GetValidatorAccountRootHash")
	defer func() {
		sw.Stop("GetValidatorAccountRootHash")
		log.Debug("GetValidatorAccountRootHash", sw.GetMeasurements()...)
	}()

	ctx := context.Background()
	leavesChannels, err := vs.peerAdapter.GetAllLeaves(rootHash, ctx)
	if err != nil {
		return nil, err
	}

	valOwners, err := vs.collectOwnerAddresses(leavesChannels)
	if err != nil {
		return nil, err
	}

	// get info from kapp
	return vs.vKApp.GetValidatorsInfo(valOwners)
}

func (vs *validatorStatistics) GetPeersAccountsPubkeysFromRootHash(rootHash []byte) ([][]byte, error) {
	sw := tools.NewStopWatch()
	sw.Start("GetPeersAccountsPubkeysFromRootHash")
	defer func() {
		sw.Stop("GetPeersAccountsPubkeysFromRootHash")
		log.Debug("GetPeersAccountsPubkeysFromRootHash", sw.GetMeasurements()...)
	}()

	ctx := context.Background()
	leavesChannels, err := vs.peerAdapter.GetAllLeaves(rootHash, ctx)
	if err != nil {
		return nil, err
	}

	return vs.collectOwnerAddresses(leavesChannels)
}

func (vs *validatorStatistics) collectOwnerAddresses(leavesChannels *data.TrieIteratorChannels) ([][]byte, error) {
	valOwners := make([][]byte, 0)

	err := leavesChannels.ForEach(func(pa data.KeyValueHolder) error {
		peerAccount, errUnmarshal := vs.unmarshalPeer(pa.Value())
		if errUnmarshal != nil {
			return errUnmarshal
		}
		if peerAccount.GetRevoked() {
			return nil
		}
		valOwners = append(valOwners, peerAccount.GetOwnerAddress())

		return nil
	})
	if err != nil {
		return nil, err
	}

	return valOwners, nil
}

// ProcessRatingsEndOfEpoch makes end of epoch process on the rating
func (vs *validatorStatistics) ProcessRatingsEndOfEpoch(
	validatorInfos []*state.ValidatorInfo,
	epoch uint32,
) error {
	if len(validatorInfos) == 0 {
		return process.ErrNilValidatorInfos
	}

	if epoch <= 0 {
		return nil
	}

	return vs.vKApp.ProcessRatingsEndOfEpoch(validatorInfos)
}

// ResetValidatorStatisticsAtNewEpoch resets the validator info at the start of a new epoch
func (vs *validatorStatistics) ResetValidatorStatisticsAtNewEpoch(vInfos []*state.ValidatorInfo) ([]*state.ValidatorInfo, error) {
	sw := tools.NewStopWatch()
	sw.Start("ResetValidatorStatisticsAtNewEpoch")
	defer func() {
		sw.Stop("ResetValidatorStatisticsAtNewEpoch")
		log.Debug("ResetValidatorStatisticsAtNewEpoch", sw.GetMeasurements()...)
	}()

	return vs.vKApp.ResetValidatorStatisticsAtNewEpoch(vInfos)
}

func (vs *validatorStatistics) checkForMissedBlocks(
	currentHeaderSlot,
	previousHeaderSlot uint64,
	prevRandSeed []byte,
	epoch uint32,
) error {
	missedSlots := currentHeaderSlot - previousHeaderSlot
	if missedSlots <= 1 {
		return nil
	}

	tooManyComputations := missedSlots > vs.maxComputableSlots
	if !tooManyComputations {
		return vs.computeDecrease(previousHeaderSlot, currentHeaderSlot, prevRandSeed, epoch)
	}

	return vs.decreaseAll(missedSlots-1, epoch)
}

func (vs *validatorStatistics) computeDecrease(
	previousHeaderSlot uint64,
	currentHeaderSlot uint64,
	prevRandSeed []byte,
	epoch uint32,
) error {
	sw := tools.NewStopWatch()
	sw.Start("checkForMissedBlocks")
	defer func() {
		sw.Stop("checkForMissedBlocks")
		log.Trace("measurements checkForMissedBlocks", sw.GetMeasurements()...)
	}()

	for i := previousHeaderSlot + 1; i < currentHeaderSlot; i++ {
		swInner := tools.NewStopWatch()

		swInner.Start("ComputeValidatorsGroup")
		log.Trace("Decreasing", "slot", i, "prevRandSeed", prevRandSeed)
		consensusGroup, err := vs.nodesCoordinator.ComputeConsensusGroup(prevRandSeed, i, epoch)
		swInner.Stop("ComputeValidatorsGroup")
		if err != nil {
			return err
		}

		if len(consensusGroup) == 0 {
			return process.ErrEmptyConsensusGroup
		}

		leaderPK := tools.GetTrimmedPk(vs.pubkeyConv.Encode(consensusGroup[0].PubKey()))
		log.Trace("Decreasing for leader", "leader", leaderPK, "slot", i)
		if err != nil {
			return err
		}

		vs.mutValidatorStatistics.Lock()
		vs.missedBlocksCounters.decreaseLeader(consensusGroup[0].OwnerAddress())
		vs.mutValidatorStatistics.Unlock()

		swInner.Start("ComputeDecreaseProposer")
		err = vs.vKApp.DecreaseTempRating(consensusGroup[0].OwnerAddress(), true)
		swInner.Stop("ComputeDecreaseProposer")
		if err != nil {
			return err
		}

		swInner.Start("ComputeDecreaseAllValidators")
		err = vs.decreaseForConsensusValidators(consensusGroup, epoch)
		swInner.Stop("ComputeDecreaseAllValidators")
		if err != nil {
			return err
		}
		sw.Add(swInner)
	}

	return nil
}

func (vs *validatorStatistics) decreaseForConsensusValidators(
	consensusGroup []sharding.Validator,
	epoch uint32,
) error {
	vs.mutValidatorStatistics.Lock()
	defer vs.mutValidatorStatistics.Unlock()

	for j := 1; j < len(consensusGroup); j++ {
		vs.missedBlocksCounters.decreaseValidator(consensusGroup[j].OwnerAddress())
		err := vs.vKApp.DecreaseTempRating(consensusGroup[j].OwnerAddress(), false)
		if err != nil {
			return err
		}
	}

	return nil
}

// RevertPeerState takes the current and previous headers and undos the peer state
// for all of the consensus members
func (vs *validatorStatistics) RevertPeerState(header data.HeaderHandler) error {
	return vs.peerAdapter.RecreateTrie(header.GetValidatorsTrieRoot())
}

func (vs *validatorStatistics) updateValidatorInfoOnSuccessfulBlock(
	validatorList []sharding.Validator,
	signingBitmap []byte,
	accumulatedFees int64,
) error {
	if len(signingBitmap) == 0 {
		return process.ErrNilPubKeysBitmap
	}

	return vs.vKApp.UpdateValidatorInfoOnSuccessfulBlock(validatorList, signingBitmap, accumulatedFees)
}

func (vs *validatorStatistics) loadPeerAccount(address []byte) (state.PeerAccountHandler, error) {
	account, err := vs.peerAdapter.LoadAccount(address)
	if err != nil {
		return nil, err
	}

	peerAccount, ok := account.(state.PeerAccountHandler)
	if !ok {
		return nil, process.ErrInvalidPeerAccount
	}

	return peerAccount, nil
}

func (vs *validatorStatistics) updateMissedBlocksCounters() error {
	vs.mutValidatorStatistics.Lock()
	defer func() {
		vs.missedBlocksCounters.reset()
		vs.mutValidatorStatistics.Unlock()
	}()

	mb := make(map[string]kapp.RateChange)
	for pubKey, slotCounters := range vs.missedBlocksCounters {
		mb[pubKey] = kapp.RateChange{Leader: slotCounters.leaderDecreaseCount, Validator: slotCounters.validatorDecreaseCount}
	}

	return vs.vKApp.UpdateMissedBlocksCounters(mb)
}

// IsInterfaceNil returns true if there is no value under the interface
func (vs *validatorStatistics) IsInterfaceNil() bool {
	return vs == nil
}

func (vs *validatorStatistics) decreaseAll(
	missedSlots uint64,
	epoch uint32,
) error {
	log.Debug("ValidatorStatistics decreasing all", "missedSlots", missedSlots)
	consensusGroupSize := vs.nodesCoordinator.ConsensusGroupSize()
	validators, err := vs.nodesCoordinator.GetAllElectedValidatorsKeys(epoch, false)
	if err != nil {
		return err
	}

	return vs.vKApp.DecreaseAll(validators, missedSlots, consensusGroupSize)
}

// SetLastFinalizedRootHash - sets the last finalized root hash needed for correct validatorStatistics computations
func (vs *validatorStatistics) SetLastFinalizedRootHash(lastFinalizedRootHash []byte) {
	if len(lastFinalizedRootHash) == 0 {
		return
	}

	vs.mutValidatorStatistics.Lock()
	vs.lastFinalizedRootHash = lastFinalizedRootHash
	vs.mutValidatorStatistics.Unlock()
}

// LastFinalizedRootHash returns the root hash of the validator statistics trie that was last finalized
func (vs *validatorStatistics) LastFinalizedRootHash() []byte {
	vs.mutValidatorStatistics.RLock()
	defer vs.mutValidatorStatistics.RUnlock()
	return vs.lastFinalizedRootHash
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (vs *validatorStatistics) EpochConfirmed(epoch uint32) {
}

// LeaderRewards -
func (vs *validatorStatistics) LeaderRewards(accumulatedFees int64) int64 {
	return int64(float64(accumulatedFees) * vs.rewardsHandler.LeaderPercentage())
}
