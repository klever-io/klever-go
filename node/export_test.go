package node

// GetNetworkDegradedThreshold -
func (n *Node) GetNetworkDegradedThreshold() uint32 {
	return n.networkDegradedThreshold
}

// GetNetworkDegradedCooldownSlots -
func (n *Node) GetNetworkDegradedCooldownSlots() uint32 {
	return n.networkDegradedCooldownSlots
}

// SetHeartbeatHandler sets the heartbeat handler, for tests that exercise
// StartConsensus without building the full heartbeat stack
func (n *Node) SetHeartbeatHandler(handler HeartbeatHandler) {
	n.heartbeatHandler = handler
}
