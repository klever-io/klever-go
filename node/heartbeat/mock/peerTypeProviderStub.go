package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
)

// PeerTypeProviderStub -
type PeerTypeProviderStub struct {
	ComputeForPubKeyCalled func(pubKey []byte) (core.PeerType, uint32, error)
}

// ComputeForPubKey -
func (p *PeerTypeProviderStub) ComputeForPubKey(pubKey []byte) (core.PeerType, uint32, error) {
	if p.ComputeForPubKeyCalled != nil {
		return p.ComputeForPubKeyCalled(pubKey)
	}

	return "", 0, nil
}

// GetAllPeerTypeInfos -
func (p *PeerTypeProviderStub) GetAllPeerTypeInfos() []*state.PeerTypeInfo {
	return nil
}

// IsInterfaceNil -
func (p *PeerTypeProviderStub) IsInterfaceNil() bool {
	return p == nil
}

// Close  -
func (p *PeerTypeProviderStub) Close() error {
	return nil
}
