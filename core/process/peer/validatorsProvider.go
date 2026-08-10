package peer

import (
	"context"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.ValidatorsProvider = (*validatorsProvider)(nil)

// validatorsProvider is the main interface for validators' provider
type validatorsProvider struct {
	nodesCoordinator             process.NodesCoordinator
	validatorStatistics          process.ValidatorStatisticsProcessor
	cache                        map[string]*state.ValidatorApiResponse
	cacheRefreshIntervalDuration time.Duration
	refreshCache                 chan uint32
	lastCacheUpdate              time.Time
	lock                         sync.RWMutex
	cancelFunc                   func()
	pubkeyConverter              core.PubkeyConverter
	maxRating                    uint32
	currentEpoch                 uint32
}

// ArgValidatorsProvider contains all parameters needed for creating a validatorsProvider
type ArgValidatorsProvider struct {
	NodesCoordinator        process.NodesCoordinator
	EpochStartEventNotifier eventNotifier.EpochStartEventNotifier
	CacheRefreshInterval    time.Duration
	ValidatorStatistics     process.ValidatorStatisticsProcessor
	PubKeyConverter         core.PubkeyConverter
	StartEpoch              uint32
	MaxRating               uint32
}

// NewValidatorsProvider instantiates a new validatorsProvider structure responsible of keeping account of
//
//	the latest information about the validators
func NewValidatorsProvider(
	args ArgValidatorsProvider,
) (*validatorsProvider, error) {
	if check.IfNil(args.ValidatorStatistics) {
		return nil, common.ErrNilValidatorStatistics
	}
	if check.IfNil(args.PubKeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.NodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(args.EpochStartEventNotifier) {
		return nil, common.ErrNilEpochStartNotifier
	}
	if args.MaxRating == 0 {
		return nil, common.ErrMaxRatingZero
	}
	if args.CacheRefreshInterval <= 0 {
		return nil, common.ErrInvalidCacheRefreshIntervalInSec
	}

	currentContext, cancelfunc := context.WithCancel(context.Background())

	valProvider := &validatorsProvider{
		nodesCoordinator:             args.NodesCoordinator,
		validatorStatistics:          args.ValidatorStatistics,
		cache:                        make(map[string]*state.ValidatorApiResponse),
		cacheRefreshIntervalDuration: args.CacheRefreshInterval,
		refreshCache:                 make(chan uint32),
		lock:                         sync.RWMutex{},
		cancelFunc:                   cancelfunc,
		maxRating:                    args.MaxRating,
		pubkeyConverter:              args.PubKeyConverter,
		currentEpoch:                 args.StartEpoch,
	}

	go valProvider.startRefreshProcess(currentContext)
	args.EpochStartEventNotifier.RegisterHandler(valProvider.epochStartEventHandler())

	return valProvider, nil
}

// GetLatestValidators gets the latest configuration of validators from the peerAccountsTrie
func (vp *validatorsProvider) GetLatestValidators() (map[string]*state.ValidatorApiResponse, error) {
	vp.lock.RLock()
	shouldUpdate := time.Since(vp.lastCacheUpdate) > vp.cacheRefreshIntervalDuration
	vp.lock.RUnlock()

	if shouldUpdate {
		err := vp.updateCache()
		if err != nil {
			return nil, err
		}
	}

	vp.lock.RLock()
	clonedMap := cloneMap(vp.cache)
	vp.lock.RUnlock()

	return clonedMap, nil
}

// GetLatestPeers gets the latest configuration of validators from the peerAccountsTrie
func (vp *validatorsProvider) GetLatestPeers() ([]state.PeerAccountHandler, error) {
	lastFinalizedRootHash := vp.validatorStatistics.LastFinalizedRootHash()
	if len(lastFinalizedRootHash) == 0 {
		return nil, nil
	}

	// A truncated peer trie walk must not be answered as an empty peer set.
	allPeers, err := vp.validatorStatistics.ListPeerAccounts(lastFinalizedRootHash)
	if err != nil {
		log.Warn("validatorsProvider - ListPeerAccounts failed", "error", err)
		return nil, err
	}

	return allPeers, nil
}

func cloneMap(cache map[string]*state.ValidatorApiResponse) map[string]*state.ValidatorApiResponse {
	newMap := make(map[string]*state.ValidatorApiResponse)

	for k, v := range cache {
		newMap[k] = cloneValidatorAPIResponse(v)
	}

	return newMap
}

func cloneValidatorAPIResponse(v *state.ValidatorApiResponse) *state.ValidatorApiResponse {
	if v == nil {
		return nil
	}
	return &state.ValidatorApiResponse{
		TempRating:                         v.TempRating,
		NumLeaderSuccess:                   v.NumLeaderSuccess,
		NumLeaderFailure:                   v.NumLeaderFailure,
		NumValidatorSuccess:                v.NumValidatorSuccess,
		NumValidatorFailure:                v.NumValidatorFailure,
		NumValidatorIgnoredSignatures:      v.NumValidatorIgnoredSignatures,
		Rating:                             v.Rating,
		RatingModifier:                     v.RatingModifier,
		TotalNumLeaderSuccess:              v.TotalNumLeaderSuccess,
		TotalNumLeaderFailure:              v.TotalNumLeaderFailure,
		TotalNumValidatorSuccess:           v.TotalNumValidatorSuccess,
		TotalNumValidatorFailure:           v.TotalNumValidatorFailure,
		TotalNumValidatorIgnoredSignatures: v.TotalNumValidatorIgnoredSignatures,
		ValidatorStatus:                    v.ValidatorStatus,
		Revoked:                            v.Revoked,
	}
}

func (vp *validatorsProvider) epochStartEventHandler() sharding.EpochStartActionHandler {
	subscribeHandler := notifier.NewHandlerForEpochStart(
		func(hdr data.HeaderHandler) {
			log.Trace("epochStartEventHandler - refreshCache forced",
				"nonce", hdr.GetNonce(),
				"slot", hdr.GetSlot(),
				"epoch", hdr.GetEpoch())
			go func() {
				vp.refreshCache <- hdr.GetEpoch()
			}()
		},
		func(_ data.HeaderHandler) {
			// nothing to prepare before an epoch start; the cache is refreshed in the action handler above
		},
		core.IndexerOrder,
	)

	return subscribeHandler
}

func (vp *validatorsProvider) startRefreshProcess(ctx context.Context) {
	for {
		err := vp.updateCache()
		if err != nil {
			log.Warn("startRefreshProcess - updateCache failed", "error", err)
		}
		select {
		case epoch := <-vp.refreshCache:
			vp.lock.Lock()
			vp.currentEpoch = epoch
			vp.lock.Unlock()
			log.Trace("startRefreshProcess - forced refresh", "epoch", vp.currentEpoch)
		case <-ctx.Done():
			log.Debug("validatorsProvider's go routine is stopping...")
			return
		}
	}
}

func (vp *validatorsProvider) updateCache() error {
	lastFinalizedRootHash := vp.validatorStatistics.LastFinalizedRootHash()
	if len(lastFinalizedRootHash) == 0 {
		return nil
	}
	allNodes, err := vp.validatorStatistics.GetValidatorInfoForRootHash(lastFinalizedRootHash)
	if err != nil {
		// Rebuilding the cache from a truncated walk would blank every peer-derived field, so the
		// previously cached values are kept until a refresh succeeds.
		log.Warn("validatorsProvider - GetValidatorInfoForRootHash failed, keeping previous cache", "error", err)

		vp.lock.Lock()
		vp.lastCacheUpdate = time.Now()
		vp.lock.Unlock()

		return err
	}

	vp.lock.RLock()
	epoch := vp.currentEpoch
	vp.lock.RUnlock()

	newCache := vp.createNewCache(epoch, allNodes)

	vp.lock.Lock()
	vp.lastCacheUpdate = time.Now()
	vp.cache = newCache
	vp.lock.Unlock()

	return nil
}

func (vp *validatorsProvider) createNewCache(
	epoch uint32,
	allNodes []*state.ValidatorInfo,
) map[string]*state.ValidatorApiResponse {
	newCache := vp.createValidatorAPIResponseMapFromValidatorInfoMap(allNodes)

	// unlike the peerTypeProvider sibling, a failing getter is not fatal here:
	// the trie-based cache is still served, only the list overlay is skipped
	listSources := []struct {
		name     string
		peerType core.PeerType
		getKeys  func(epoch uint32, ownerKey bool) ([][]byte, error)
	}{
		{"GetAllElectedValidatorsKeys", core.ElectedList, vp.nodesCoordinator.GetAllElectedValidatorsKeys},
		{"GetAllEligibleValidatorsKeys", core.EligibleList, vp.nodesCoordinator.GetAllEligibleValidatorsKeys},
	}

	for _, src := range listSources {
		keys, err := src.getKeys(epoch, false)
		if err != nil {
			log.Debug("validatorsProvider - "+src.name+" failed", "epoch", epoch, "error", err)
		}
		vp.aggregateLists(newCache, keys, src.peerType)
	}

	return newCache
}

func (vp *validatorsProvider) createValidatorAPIResponseMapFromValidatorInfoMap(allNodes []*state.ValidatorInfo) map[string]*state.ValidatorApiResponse {
	newCache := make(map[string]*state.ValidatorApiResponse)

	for _, validatorInfo := range allNodes {
		strKey := vp.pubkeyConverter.Encode(validatorInfo.PublicKey)
		if validatorInfo.IsPubKeyRevoked {
			newCache[strKey] = &state.ValidatorApiResponse{
				ValidatorStatus: string(core.RevokedList),
				Revoked:         validatorInfo.IsPubKeyRevoked,
			}
		} else {
			newCache[strKey] = &state.ValidatorApiResponse{
				NumLeaderSuccess:                   validatorInfo.LeaderSuccess,
				NumLeaderFailure:                   validatorInfo.LeaderFailure,
				NumValidatorSuccess:                validatorInfo.ValidatorSuccess,
				NumValidatorFailure:                validatorInfo.ValidatorFailure,
				NumValidatorIgnoredSignatures:      validatorInfo.ValidatorIgnoredSignatures,
				TotalNumLeaderSuccess:              validatorInfo.TotalLeaderSuccess,
				TotalNumLeaderFailure:              validatorInfo.TotalLeaderFailure,
				TotalNumValidatorSuccess:           validatorInfo.TotalValidatorSuccess,
				TotalNumValidatorFailure:           validatorInfo.TotalValidatorFailure,
				TotalNumValidatorIgnoredSignatures: validatorInfo.TotalValidatorIgnoredSignatures,
				RatingModifier:                     validatorInfo.RatingModifier,
				Rating:                             float32(validatorInfo.Rating) * 100 / float32(vp.maxRating),
				TempRating:                         float32(validatorInfo.TempRating) * 100 / float32(vp.maxRating),
				ValidatorStatus:                    validatorInfo.List,
				Revoked:                            validatorInfo.IsPubKeyRevoked,
			}
		}

	}

	return newCache
}

func (vp *validatorsProvider) aggregateLists(
	newCache map[string]*state.ValidatorApiResponse,
	validatorsList [][]byte,
	currentList core.PeerType,
) {

	for _, val := range validatorsList {
		encodedKey := vp.pubkeyConverter.Encode(val)
		foundInTrieValidator, ok := newCache[encodedKey]
		peerType := string(currentList)

		if !ok || foundInTrieValidator == nil {
			newCache[encodedKey] = &state.ValidatorApiResponse{}
			newCache[encodedKey].ValidatorStatus = peerType
			log.Debug("validator from list not found in trie", "pk", encodedKey, "map", peerType)
			continue
		}

		newCache[encodedKey].ValidatorStatus = peerType
	}

}

// IsInterfaceNil returns true if there is no value under the interface
func (vp *validatorsProvider) IsInterfaceNil() bool {
	return vp == nil
}

// Close - frees up everything, cancels long running methods
func (vp *validatorsProvider) Close() error {
	vp.cancelFunc()

	return nil
}
