package mock

import "github.com/klever-io/klever-go/sharding"

// NodesSetupStub -
type NodesSetupStub struct {
	InitialNodesInfoCalled            func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error)
	GetStartTimeCalled                func() int64
	GetSlotIntervalCalled             func() uint64
	GetMinTransactionVersionCalled    func() uint32
	GetSlotsPerEpochCalled            func() uint64
	GetConsensusGroupSizeCalled       func() uint32
	MinNumberOfNodesCalled            func() uint32
	InitialNodesPubKeysCalled         func() []string
	GetChainIDCalled                  func() string
	InitialEligibleNodesPubKeysCalled func() ([]string, error)
	InitialElectedNodesPubKeysCalled  func() ([]string, error)
}

// MinNumberOfNodes -
func (n *NodesSetupStub) MinNumberOfNodes() uint32 {
	if n.MinNumberOfNodesCalled != nil {
		return n.MinNumberOfNodesCalled()
	}
	return 1
}

// InitialNodesPubKeys -
func (n *NodesSetupStub) InitialNodesPubKeys() []string {
	if n.InitialNodesPubKeysCalled != nil {
		return n.InitialNodesPubKeysCalled()
	}

	return []string{"val1", "val2"}
}

// GetChainID -
func (n *NodesSetupStub) GetChainID() string {
	if n.GetChainIDCalled != nil {
		return n.GetChainIDCalled()
	}

	return "klv"
}

// InitialEligibleNodesPubKeys -
func (n *NodesSetupStub) InitialElectedNodesPubKeys() ([]string, error) {
	if n.InitialElectedNodesPubKeysCalled != nil {
		return n.InitialElectedNodesPubKeysCalled()
	}

	return []string{"val1", "val2"}, nil
}

// InitialEligibleNodesPubKeys -
func (n *NodesSetupStub) InitialEligibleNodesPubKeys() ([]string, error) {
	if n.InitialEligibleNodesPubKeysCalled != nil {
		return n.InitialEligibleNodesPubKeysCalled()
	}

	return []string{"val1", "val2"}, nil
}

// GetStartTime -
func (n *NodesSetupStub) GetStartTime() int64 {
	if n.GetStartTimeCalled != nil {
		return n.GetStartTimeCalled()
	}
	return 0
}

// GetMinTransactionVersion -
func (n *NodesSetupStub) GetMinTransactionVersion() uint32 {
	if n.GetMinTransactionVersionCalled != nil {
		return n.GetMinTransactionVersionCalled()
	}
	return 1
}

// GetSlotsPerEpoch -
func (n *NodesSetupStub) GetSlotsPerEpoch() uint64 {
	if n.GetSlotsPerEpochCalled != nil {
		return n.GetSlotsPerEpochCalled()
	}
	return 0
}

// GetSlotInterval -
func (n *NodesSetupStub) GetSlotInterval() uint64 {
	if n.GetSlotIntervalCalled != nil {
		return n.GetSlotIntervalCalled()
	}
	return 0
}

// GetConsensusGroupSize -
func (n *NodesSetupStub) GetConsensusGroupSize() uint32 {
	if n.GetConsensusGroupSizeCalled != nil {
		return n.GetConsensusGroupSizeCalled()
	}
	return 0
}

// InitialNodesInfo -
func (n *NodesSetupStub) InitialNodesInfo() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
	if n.InitialNodesInfoCalled != nil {
		return n.InitialNodesInfoCalled()
	}
	return nil, nil, nil
}

// IsInterfaceNil -
func (n *NodesSetupStub) IsInterfaceNil() bool {
	return n == nil
}
