package mock

import (
	"github.com/klever-io/klever-go/core"
)

// NetworkShardingCollectorStub -
type NetworkShardingCollectorStub struct {
	UpdatePeerIDPublicKeyCalled func(pid core.PeerID, pk []byte)
	UpdatePublicKeyCalled       func(pk []byte)
	UpdatePeerIDCalled          func(pid core.PeerID)
}

// UpdatePeerIDPublicKey -
func (nscs *NetworkShardingCollectorStub) UpdatePeerIDPublicKey(pid core.PeerID, pk []byte) {
	nscs.UpdatePeerIDPublicKeyCalled(pid, pk)
}

// UpdatePublicKey -
func (nscs *NetworkShardingCollectorStub) UpdatePublicKey(pk []byte) {
	nscs.UpdatePublicKeyCalled(pk)
}

// UpdatePeerID -
func (nscs *NetworkShardingCollectorStub) UpdatePeerID(pid core.PeerID) {
	nscs.UpdatePeerIDCalled(pid)
}

// IsInterfaceNil -
func (nscs *NetworkShardingCollectorStub) IsInterfaceNil() bool {
	return nscs == nil
}
