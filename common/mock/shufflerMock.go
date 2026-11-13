package mock

import "github.com/klever-io/klever-go/sharding"

// NodeShufflerMock -
type NodeShufflerMock struct {
	ElectionLimitsCalled func(epoch uint32) (uint32, uint32)
}

// UpdateParams -
func (nsm *NodeShufflerMock) UpdateParams(
	_ uint32,
) {

}

// UpdateNodeLists -
func (nsm *NodeShufflerMock) UpdateNodeLists(args sharding.ArgsUpdateNodes) (*sharding.ResUpdateNodes, error) {
	return &sharding.ResUpdateNodes{
		Elected:  args.Eligible,
		Eligible: args.Eligible,
	}, nil
}

func (nsm *NodeShufflerMock) ElectionLimits(epoch uint32) (uint32, uint32) {
	if nsm.ElectionLimitsCalled != nil {
		return nsm.ElectionLimitsCalled(epoch)
	}

	return 0, 0
}

// IsInterfaceNil -
func (nsm *NodeShufflerMock) IsInterfaceNil() bool {
	return nsm == nil
}
