package mock

import (
	"github.com/klever-io/klever-go/core"
)

// NetworkShardingCollectorStub -
type NetworkShardingCollectorStub struct {
	UpdatePeerIDPublicKeyCalled func(pid core.PeerID, pk []byte)
}

// UpdatePeerIDPublicKey -
func (nscs *NetworkShardingCollectorStub) UpdatePeerIDPublicKey(pid core.PeerID, pk []byte) {
	nscs.UpdatePeerIDPublicKeyCalled(pid, pk)
}

// IsInterfaceNil -
func (nscs *NetworkShardingCollectorStub) IsInterfaceNil() bool {
	return nscs == nil
}
