package mock

// NodeInfoMock -
type NodeInfoMock struct {
	address       []byte
	pubKey        []byte
	initialRating uint32
}

// NewNodeInfo -
func NewNodeInfo(address []byte, pubKey []byte, initialRating uint32) *NodeInfoMock {
	return &NodeInfoMock{
		address:       address,
		pubKey:        pubKey,
		initialRating: initialRating,
	}
}

// GetInitialRating -
func (n *NodeInfoMock) GetInitialRating() uint32 {
	return n.initialRating
}

// AddressBytes -
func (n *NodeInfoMock) AddressBytes() []byte {
	return n.address
}

// PubKeyBytes -
func (n *NodeInfoMock) PubKeyBytes() []byte {
	return n.pubKey
}

// IsInterfaceNil -
func (n *NodeInfoMock) IsInterfaceNil() bool {
	return n == nil
}
