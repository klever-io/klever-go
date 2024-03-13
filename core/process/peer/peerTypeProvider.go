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

	ptp.updateCache(arg.StartEpoch)

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
			ptp.updateCache(hdr.GetEpoch())
		},
		func(_ data.HeaderHandler) {},
		core.IndexerOrder,
	)

	return subscribeHandler
}

func (ptp *PeerTypeProvider) updateCache(epoch uint32) {
	newCache := ptp.createNewCache(epoch)

	ptp.mutCache.Lock()
	ptp.cache = newCache
	ptp.mutCache.Unlock()
}

func (ptp *PeerTypeProvider) createNewCache(
	epoch uint32,
) map[string]*peerListAndShard {
	newCache := make(map[string]*peerListAndShard)

	nodesMapElected, err := ptp.nodesCoordinator.GetAllElectedValidatorsKeys(epoch, false)
	if err != nil {
		log.Debug("peerTypeProvider - GetAllElectedValidatorsKeys failed", "epoch", epoch)
	}
	computePeerType(newCache, nodesMapElected, core.ElectedList)

	nodesMapEligible, err := ptp.nodesCoordinator.GetAllEligibleValidatorsKeys(epoch, false)
	if err != nil {
		log.Debug("peerTypeProvider - GetAllEligibleValidatorsKeys failed", "epoch", epoch)
	}
	computePeerType(newCache, nodesMapEligible, core.EligibleList)

	return newCache
}

func computePeerType(
	newCache map[string]*peerListAndShard,
	validatorsList [][]byte,
	currentPeerType core.PeerType,
) {
	for _, val := range validatorsList {
		newCache[string(val)] = &peerListAndShard{
			pType:  currentPeerType,
			pShard: 0, // TODO: default as 0, maybe change this for intrachain communication
		}
	}

}

// IsInterfaceNil returns true if there is no value under the interface
func (ptp *PeerTypeProvider) IsInterfaceNil() bool {
	return ptp == nil
}
