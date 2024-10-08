package mock

import (
	"github.com/klever-io/klever-go/core"
)

// PeerListCreatorStub -
type PeerListCreatorStub struct {
	PeerListCalled          func() []core.PeerID
	ConsensusPeerListCalled func() []core.PeerID
}

// PeerList -
func (p *PeerListCreatorStub) PeerList() []core.PeerID {
	return p.PeerListCalled()
}

// IntraShardPeerList -
func (p *PeerListCreatorStub) ConsensusPeerList() []core.PeerID {
	return p.ConsensusPeerListCalled()
}

// IsInterfaceNil returns true if there is no value under the interface
func (p *PeerListCreatorStub) IsInterfaceNil() bool {
	return p == nil
}
