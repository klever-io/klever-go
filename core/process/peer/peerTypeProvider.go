package peer

import (
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
)

type peerListAndShard struct {
	pType  core.PeerType
	pShard uint32
}

// PeerTypeProvider handles the computation of a peer type
type PeerTypeProvider struct {
	nodesCoordinator sharding.NodesCoordinator
	cache            map[string]*peerListAndShard
	mutCache         sync.RWMutex
}

// ArgPeerTypeProvider contains all parameters needed for creating a PeerTypeProvider
type ArgPeerTypeProvider struct {
	NodesCoordinator        sharding.NodesCoordinator
	StartEpoch              uint32
	EpochStartEventNotifier eventNotifier.EpochStartEventNotifier
}

// NewPeerTypeProvider will return a new instance of PeerTypeProvider
func NewPeerTypeProvider(arg ArgPeerTypeProvider) (*PeerTypeProvider, error) {
	if check.IfNil(arg.NodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(arg.EpochStartEventNotifier) {
		return nil, common.ErrNilEpochStartNotifier
	}

	ptp := &PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            make(map[string]*peerListAndShard),
		mutCache:         sync.RWMutex{},
	}

	ptp.UpdateCache(arg.StartEpoch)

	arg.EpochStartEventNotifier.RegisterHandler(ptp.epochStartEventHandler())

	return ptp, nil
}

// ComputeForPubKey returns the peer type for a given public key and shard id
func (ptp *PeerTypeProvider) ComputeForPubKey(pubKey []byte) (core.PeerType, uint32, error) {
	ptp.mutCache.RLock()
	peerData, ok := ptp.cache[string(pubKey)]
	ptp.mutCache.RUnlock()

	if ok {
		return peerData.pType, peerData.pShard, nil
	}

	return core.ObserverList, 0, nil
}

// GetAllPeerTypeInfos returns all known peer type infos
func (ptp *PeerTypeProvider) GetAllPeerTypeInfos() []*state.PeerTypeInfo {
	ptp.mutCache.RLock()
	defer ptp.mutCache.RUnlock()

	peerTypeInfos := make([]*state.PeerTypeInfo, 0, len(ptp.cache))
	for pkString, peerListAndShardVal := range ptp.cache {
		peerTypeInfos = append(peerTypeInfos, &state.PeerTypeInfo{
			PublicKey: pkString,
			PeerType:  string(peerListAndShardVal.pType),
			ShardId:   0,
		})
	}

	return peerTypeInfos
}

func (ptp *PeerTypeProvider) epochStartEventHandler() sharding.EpochStartActionHandler {
	subscribeHandler := notifier.NewHandlerForEpochStart(
		func(hdr data.HeaderHandler) {
			log.Trace("epochStartEventHandler - refreshCache forced",
				"nonce", hdr.GetNonce(),
				"slot", hdr.GetSlot(),
				"epoch", hdr.GetEpoch())
			ptp.UpdateCache(hdr.GetEpoch())
		},
		func(_ data.HeaderHandler) {
			// nothing to prepare before an epoch start; the cache is rebuilt in the action handler above
		},
		core.IndexerOrder,
	)

	return subscribeHandler
}

// UpdateCache rebuilds the validator-type cache for the given epoch.
// When the coordinator cannot provide validator lists (e.g. unknown epoch),
// the previous cache is kept to avoid replacing valid data with an empty map.
func (ptp *PeerTypeProvider) UpdateCache(epoch uint32) {
	newCache, ok := ptp.createNewCache(epoch)
	if !ok {
		log.Warn("peerTypeProvider - keeping previous cache, validator list unavailable", "epoch", epoch)
		return
	}

	ptp.mutCache.Lock()
	ptp.cache = newCache
	ptp.mutCache.Unlock()
}

func (ptp *PeerTypeProvider) createNewCache(
	epoch uint32,
) (map[string]*peerListAndShard, bool) {
	newCache := make(map[string]*peerListAndShard)

	// the leaving list is seeded as jailed: the coordinator fills it exclusively
	// from validators whose list is jailed (computeNodesConfigFromList), and
	// operators should see that actionable state. Seeding order is defensive:
	// later lists win, so working types take precedence if a key ever appears
	// in more than one list (in production the lists are a partition, the
	// numToStay promotion removes promoted keys from the leaving list).
	listSources := []struct {
		name     string
		peerType core.PeerType
		getKeys  func(epoch uint32, ownerKey bool) ([][]byte, error)
	}{
		{"GetAllLeavingValidatorsKeys", core.JailedList, ptp.nodesCoordinator.GetAllLeavingValidatorsKeys},
		{"GetAllWaitingValidatorsKeys", core.WaitingList, ptp.nodesCoordinator.GetAllWaitingValidatorsKeys},
		{"GetAllElectedValidatorsKeys", core.ElectedList, ptp.nodesCoordinator.GetAllElectedValidatorsKeys},
		{"GetAllEligibleValidatorsKeys", core.EligibleList, ptp.nodesCoordinator.GetAllEligibleValidatorsKeys},
	}

	for _, src := range listSources {
		keys, err := src.getKeys(epoch, false)
		if err != nil {
			log.Warn("peerTypeProvider - "+src.name+" failed", "epoch", epoch, "error", err)
			return nil, false
		}
		computePeerType(newCache, keys, src.peerType)
	}

	return newCache, true
}

func computePeerType(
	newCache map[string]*peerListAndShard,
	validatorsList [][]byte,
	currentPeerType core.PeerType,
) {
	for _, val := range validatorsList {
		newCache[string(val)] = &peerListAndShard{
			pType:  currentPeerType,
			pShard: 0,
		}
	}

}

// IsInterfaceNil returns true if there is no value under the interface
func (ptp *PeerTypeProvider) IsInterfaceNil() bool {
	return ptp == nil
}
