package networksharding

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/storage"
)

const MaxNumPidsPerPk = maxNumPidsPerPk

func (psm *PeerShardMapper) GetPkFromPidPk(pid core.PeerID) []byte {
	pk, ok := psm.peerIDPk.Get([]byte(pid))
	if !ok {
		return nil
	}

	return pk.([]byte)
}

func (psm *PeerShardMapper) GetFromPkPeerID(pk []byte) []core.PeerID {
	objsPidsQueue, found := psm.pkPeerID.Get(pk)
	if !found {
		return nil
	}

	return objsPidsQueue.(*pidQueue).data
}

func (psm *PeerShardMapper) PeerIDPk() storage.Cacher {
	return psm.peerIDPk
}

func (psm *PeerShardMapper) PkPeerID() storage.Cacher {
	return psm.pkPeerID
}

func (psm *PeerShardMapper) FallbackPidShard() storage.Cacher {
	return psm.fallbackPidShard
}

func (psm *PeerShardMapper) Epoch() uint32 {
	psm.mutEpoch.RLock()
	defer psm.mutEpoch.RUnlock()

	return psm.epoch
}
