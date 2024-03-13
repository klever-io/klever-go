package mock

import (
	"time"

	"github.com/klever-io/klever-go/sharding"
)

// GenesisNodesSetupHandlerStub -
type GenesisNodesSetupHandlerStub struct {
	InitialNodesInfoCalled         func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error)
	GetStartTimeCalled             func() int64
	GetSlotIntervalCalled          func() uint64
	GetChainIDCalled               func() string
	GetMinTransactionVersionCalled func() uint32
	GetConsensusGroupSizeCalled    func() uint32
	NumberOfShardsCalled           func() uint32
	MinNumberOfNodesCalled         func() uint32
}

// InitialNodesInfo -
func (g *GenesisNodesSetupHandlerStub) InitialNodesInfo() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
	if g.InitialNodesInfoCalled != nil {
		return g.InitialNodesInfoCalled()
	}

	return nil, nil, nil
}

// GetStartTime -
func (g *GenesisNodesSetupHandlerStub) GetStartTime() int64 {
	if g.GetStartTimeCalled != nil {
		return g.GetStartTimeCalled()
	}

	return time.Now().Unix()
}

// GetSlotInterval -
func (g *GenesisNodesSetupHandlerStub) GetSlotInterval() uint64 {
	if g.GetSlotIntervalCalled != nil {
		return g.GetSlotIntervalCalled()
	}

	return 4500
}

// GetChainID -
func (g *GenesisNodesSetupHandlerStub) GetChainID() string {
	if g.GetChainIDCalled != nil {
		return g.GetChainIDCalled()
	}

	return "chainID"
}

// GetMinTransactionVersion -
func (g *GenesisNodesSetupHandlerStub) GetMinTransactionVersion() uint32 {
	if g.GetMinTransactionVersionCalled != nil {
		return g.GetMinTransactionVersionCalled()
	}

	return 1
}

// GetShardConsensusGroupSize -
func (g *GenesisNodesSetupHandlerStub) GetConsensusGroupSize() uint32 {
	if g.GetConsensusGroupSizeCalled != nil {
		return g.GetConsensusGroupSizeCalled()
	}

	return 1
}

// MinNumberOfShardNodes -
func (g *GenesisNodesSetupHandlerStub) MinNumberOfNodes() uint32 {
	if g.MinNumberOfNodesCalled != nil {
		return g.MinNumberOfNodesCalled()
	}

	return 1
}

// IsInterfaceNil -
func (g *GenesisNodesSetupHandlerStub) IsInterfaceNil() bool {
	return g == nil
}
