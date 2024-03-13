package networksharding

import (
	"encoding/hex"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/tools/check"
)

const maxNumPidsPerPk = 3
const uint32Size = 4

var log = logger.GetOrCreate("sharding/networksharding")

var _ p2p.NetworkShardingCollector = (*PeerShardMapper)(nil)
var _ p2p.PeerShardResolver = (*PeerShardMapper)(nil)

// PeerShardMapper stores the mappings between peer IDs and shard IDs
// Both public key and peer id are verified before they are appended in this cache. In time, the current node
// will learn a large majority (or even the whole network) of the nodes that make up the network. The public key provided
// by this map is then fed to the nodes coordinator that will output the shard id in which that public key resides.
// This component also have a reversed lookup map that will ensure that there won't be unlimited peer ids with the
// same public key. This will prevent eclipse attacks.
type PeerShardMapper struct {
	peerIDPk         storage.Cacher
	pkPeerID         storage.Cacher
	fallbackPidShard storage.Cacher

	mutUpdatePeerIDPublicKey sync.Mutex

	mutEpoch         sync.RWMutex
	epoch            uint32
	nodesCoordinator sharding.NodesCoordinator
}

// NewPeerShardMapper creates a new peerShardMapper instance
func NewPeerShardMapper(
	peerIDPk storage.Cacher,
	fallbackPidShard storage.Cacher,
	nodesCoordinator sharding.NodesCoordinator,
	epochStart uint32,
) (*PeerShardMapper, error) {

	if check.IfNil(nodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(peerIDPk) {
		return nil, common.ErrNilCacher
	}
	if check.IfNil(fallbackPidShard) {
		return nil, common.ErrNilCacher
	}

	pkPeerID, err := lrucache.NewCache(peerIDPk.MaxSize())
	if err != nil {
		return nil, err
	}

	log.Debug("peerShardMapper epoch", "epoch", epochStart)

	return &PeerShardMapper{
		peerIDPk:         peerIDPk,
		pkPeerID:         pkPeerID,
		fallbackPidShard: fallbackPidShard,
		nodesCoordinator: nodesCoordinator,
		epoch:            epochStart,
	}, nil
}

// GetPeerInfo returns the corresponding shard ID of a given peer ID.
// It also returns the type of provided peer
func (psm *PeerShardMapper) GetPeerInfo(pid core.PeerID) core.P2PPeerInfo {
	var pInfo *core.P2PPeerInfo
	var ok bool

	defer func() {
		if pInfo != nil {
			log.Trace("PeerShardMapper.GetPeerInfo",
				"peer type", pInfo.PeerType.String(),
				"pid", p2p.PeerIDToShortString(pid),
				"pk", hex.EncodeToString(pInfo.PkBytes),
			)
		}
	}()

	pInfo, ok = psm.getPeerInfoWithNodesCoordinator(pid)
	if ok {
		return *pInfo
	}

	pInfoCache := psm.getPeerInfoSearchingPidInFallbackCache(pid)
	pInfo.PeerType = pInfoCache.PeerType
	pInfo.ShardID = pInfoCache.ShardID
	if pInfo.PeerType == core.UnknownPeer {
		pInfo.PkBytes = nil

	}

	return *pInfo
}

func (psm *PeerShardMapper) getPeerInfoWithNodesCoordinator(pid core.PeerID) (*core.P2PPeerInfo, bool) {
	pkObj, ok := psm.peerIDPk.Get([]byte(pid))
	if !ok {
		return &core.P2PPeerInfo{
			PeerType: core.UnknownPeer,
		}, false
	}

	pkBuff, ok := pkObj.([]byte)
	if !ok {
		log.Warn("PeerShardMapper.getShardIDWithNodesCoordinator: the contained element should have been of type []byte")

		return &core.P2PPeerInfo{
			PeerType: core.UnknownPeer,
		}, false
	}

	_, err := psm.nodesCoordinator.GetValidatorWithPublicKey(pkBuff)
	if err != nil {
		return &core.P2PPeerInfo{
			PeerType: core.UnknownPeer,
			PkBytes:  pkBuff,
		}, false
	}

	return &core.P2PPeerInfo{
		PeerType: core.ValidatorPeer,
		PkBytes:  pkBuff,
	}, true
}

func (psm *PeerShardMapper) getPeerInfoSearchingPidInFallbackCache(pid core.PeerID) *core.P2PPeerInfo {
	shardObj, ok := psm.fallbackPidShard.Get([]byte(pid))
	if !ok {
		return &core.P2PPeerInfo{
			PeerType: core.UnknownPeer,
			ShardID:  0,
		}
	}

	shard, ok := shardObj.(uint32)
	if !ok {
		log.Warn("PeerShardMapper.getShardIDSearchingPidInFallbackCache: the contained element should have been of type uint32")

		return &core.P2PPeerInfo{
			PeerType: core.UnknownPeer,
			ShardID:  0,
		}
	}
	return &core.P2PPeerInfo{
		PeerType: core.ObserverPeer,
		ShardID:  shard,
	}
}

// UpdatePeerIDPublicKey updates the peer ID - public key pair in the corresponding map
// It also uses the intermediate pkPeerID cache that will prevent having thousands of peer ID's with
// the same Klever PK that will make the node prone to an eclipse attack
func (psm *PeerShardMapper) UpdatePeerIDPublicKey(pid core.PeerID, pk []byte) {
	//mutUpdatePeerIDPublicKey is used as to consider this function a critical section
	psm.mutUpdatePeerIDPublicKey.Lock()
	defer psm.mutUpdatePeerIDPublicKey.Unlock()

	psm.removePidAssociation(pid)

	objPidsQueue, found := psm.pkPeerID.Get(pk)
	if !found {
		psm.peerIDPk.Put([]byte(pid), pk, len(pk))
		pq := newPidQueue()
		pq.push(pid)
		psm.pkPeerID.Put(pk, pq, len(pk))
		return
	}

	pq, ok := objPidsQueue.(*pidQueue)
	if !ok {
		log.Warn("PeerShardMapper.UpdatePeerIDPublicKey: the contained element should have been of type pidQueue")

		return
	}

	idxPid := pq.indexOf(pid)
	if idxPid != indexNotFound {
		pq.promote(idxPid)
		psm.peerIDPk.Put([]byte(pid), pk, len(pk))
		return
	}

	pq.push(pid)
	for len(pq.data) > maxNumPidsPerPk {
		evictedPid := pq.pop()

		psm.peerIDPk.Remove([]byte(evictedPid))
		psm.fallbackPidShard.Remove([]byte(evictedPid))
	}
	psm.pkPeerID.Put(pk, pq, pq.size())
	psm.peerIDPk.Put([]byte(pid), pk, len(pk))
}

func (psm *PeerShardMapper) removePidAssociation(pid core.PeerID) {
	oldPk, found := psm.peerIDPk.Get([]byte(pid))
	if !found {
		return
	}

	oldPkBuff, ok := oldPk.([]byte)
	if !ok {
		psm.peerIDPk.Remove([]byte(pid))
		return
	}

	objPidsQueue, found := psm.pkPeerID.Get(oldPkBuff)
	if !found {
		return
	}

	pq, ok := objPidsQueue.(*pidQueue)
	if !ok {
		psm.pkPeerID.Remove(oldPkBuff)
		return
	}

	pq.remove(pid)
	if len(pq.data) == 0 {
		psm.pkPeerID.Remove(oldPkBuff)
		return
	}

	psm.pkPeerID.Put(oldPkBuff, pq, pq.size())
}

// UpdatePeerID updates the fallback search map containing peer IDs and shard IDs
func (psm *PeerShardMapper) UpdatePeerID(pid core.PeerID) {
	psm.fallbackPidShard.HasOrAdd([]byte(pid), uint32(0), uint32Size)
}

// EpochStartAction is the method called whenever an action needs to be undertaken in respect to the epoch change
func (psm *PeerShardMapper) EpochStartAction(hdr data.HeaderHandler) {
	if check.IfNil(hdr) {
		log.Warn("nil header on PeerShardMapper.EpochStartAction")
		return
	}
	log.Trace("PeerShardMapper.EpochStartAction event", "epoch", hdr.GetEpoch())

	psm.mutEpoch.Lock()
	psm.epoch = hdr.GetEpoch()
	log.Debug("peerShardMapper epoch", "epoch", psm.epoch)
	psm.mutEpoch.Unlock()
}

// EpochStartPrepare is the method called whenever an action needs to be undertaken in respect to the epoch preparation change
func (psm *PeerShardMapper) EpochStartPrepare(blk data.HeaderHandler) {
	if check.IfNil(blk) {
		log.Warn("nil header on PeerShardMapper.EpochStartPrepare")
		return
	}

	log.Trace("PeerShardMapper.EpochStartPrepare event", "epoch", blk.GetEpoch())
}

// NotifyOrder returns the notification order of this component
func (psm *PeerShardMapper) NotifyOrder() uint32 {
	return core.NetworkShardingOrder
}

// IsInterfaceNil returns true if there is no value under the interface
func (psm *PeerShardMapper) IsInterfaceNil() bool {
	return psm == nil
}
