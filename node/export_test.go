package node

// GetNetworkDegradedThreshold -
func (n *Node) GetNetworkDegradedThreshold() uint32 {
	return n.networkDegradedThreshold
}

// GetNetworkDegradedCooldownSlots -
func (n *Node) GetNetworkDegradedCooldownSlots() uint32 {
	return n.networkDegradedCooldownSlots
}
