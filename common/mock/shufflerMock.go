package mock

import "github.com/klever-io/klever-go/sharding"

// NodeShufflerMock -
type NodeShufflerMock struct {
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

// IsInterfaceNil -
func (nsm *NodeShufflerMock) IsInterfaceNil() bool {
	return nsm == nil
}
