package mock

import "github.com/klever-io/klever-go/sharding"

// InitialNodesHandlerStub -
type InitialNodesHandlerStub struct {
	InitialNodesInfoCalled func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error)
	MinNumberOfNodesCalled func() uint32
}

// InitialNodesInfo -
func (inhs *InitialNodesHandlerStub) InitialNodesInfo() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
	if inhs.InitialNodesInfoCalled != nil {
		return inhs.InitialNodesInfoCalled()
	}

	return make([]sharding.GenesisNodeInfoHandler, 0), make([]sharding.GenesisNodeInfoHandler, 0), nil
}

// MinNumberOfNodes -
func (inhs *InitialNodesHandlerStub) MinNumberOfNodes() uint32 {
	if inhs.MinNumberOfNodesCalled != nil {
		return inhs.MinNumberOfNodesCalled()
	}

	return 0
}

// IsInterfaceNil -
func (inhs *InitialNodesHandlerStub) IsInterfaceNil() bool {
	return inhs == nil
}
