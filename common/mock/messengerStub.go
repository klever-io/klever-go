package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
)

// MessengerStub -
type MessengerStub struct {
	ConnectedPeersOnTopicCalled          func(topic string) []core.PeerID
	CloseCalled                          func() error
	IDCalled                             func() core.PeerID
	PeersCalled                          func() []core.PeerID
	PeerAddressesCalled                  func(pid core.PeerID) []string
	AddressesCalled                      func() []string
	ConnectToPeerCalled                  func(address string) error
	ConnectedAddressesCalled             func() []string
	GetConnectedPeersInfoCalled          func() *p2p.ConnectedPeersInfo
	TrimConnectionsCalled                func()
	IsConnectedCalled                    func(peerID core.PeerID) bool
	ConnectedPeersCalled                 func() []core.PeerID
	CreateTopicCalled                    func(name string, createChannelForTopic bool) error
	HasTopicCalled                       func(name string) bool
	HasTopicValidatorCalled              func(name string) bool
	BroadcastOnChannelCalled             func(channel string, topic string, buff []byte)
	BroadcastOnChannelBlockingCalled     func(channel string, topic string, buff []byte) error
	BroadcastCalled                      func(topic string, buff []byte)
	RegisterMessageProcessorCalled       func(topic string, handler p2p.MessageProcessor) error
	UnregisterMessageProcessorCalled     func(topic string) error
	SendToConnectedPeerCalled            func(topic string, buff []byte, peerID core.PeerID) error
	OutgoingChannelLoadBalancerCalled    func() p2p.ChannelLoadBalancer
	BootstrapCalled                      func() error
	UnregisterAllMessageProcessorsCalled func() error
	UnjoinAllTopicsCalled                func() error
	IsConnectedToTheNetworkCalled        func() bool
	ThresholdMinConnectedPeersCalled     func() int
	SetThresholdMinConnectedPeersCalled  func(minConnectedPeers int) error
	SetPeerDenialEvaluatorCalled         func(handler p2p.PeerDenialEvaluator) error
	SetPeerShardResolverCalled           func(peerShardResolver p2p.PeerShardResolver) error
}

// ConnectedPeersOnTopic -
func (ms *MessengerStub) ConnectedPeersOnTopic(topic string) []core.PeerID {
	if ms.ConnectedPeersOnTopicCalled != nil {
		return ms.ConnectedPeersOnTopicCalled(topic)
	}

	return make([]core.PeerID, 0)
}

// RegisterMessageProcessor -
func (ms *MessengerStub) RegisterMessageProcessor(topic string, handler p2p.MessageProcessor) error {
	if ms.RegisterMessageProcessorCalled != nil {
		return ms.RegisterMessageProcessorCalled(topic, handler)
	}
	return nil
}

// UnregisterMessageProcessor -
func (ms *MessengerStub) UnregisterMessageProcessor(topic string) error {
	if ms.UnregisterMessageProcessorCalled != nil {
		return ms.UnregisterMessageProcessorCalled(topic)
	}
	return nil
}

// Broadcast -
func (ms *MessengerStub) Broadcast(topic string, buff []byte) {
	ms.BroadcastCalled(topic, buff)
}

// OutgoingChannelLoadBalancer -
func (ms *MessengerStub) OutgoingChannelLoadBalancer() p2p.ChannelLoadBalancer {
	return ms.OutgoingChannelLoadBalancerCalled()
}

// Close -
func (ms *MessengerStub) Close() error {
	return ms.CloseCalled()
}

// ID -
func (ms *MessengerStub) ID() core.PeerID {
	if ms.IDCalled != nil {
		return ms.IDCalled()
	}

	return "peer ID"
}

// Peers -
func (ms *MessengerStub) Peers() []core.PeerID {
	return ms.PeersCalled()
}

// PeerAddresses -
func (ms *MessengerStub) PeerAddresses(pid core.PeerID) []string {
	return ms.PeerAddressesCalled(pid)
}

// Addresses -
func (ms *MessengerStub) Addresses() []string {
	return ms.AddressesCalled()
}

// ConnectToPeer -
func (ms *MessengerStub) ConnectToPeer(address string) error {
	return ms.ConnectToPeerCalled(address)
}

// ConnectedAddresses -
func (ms *MessengerStub) ConnectedAddresses() []string {
	return ms.ConnectedAddressesCalled()
}

// GetConnectedPeersInfo -
func (ms *MessengerStub) GetConnectedPeersInfo() *p2p.ConnectedPeersInfo {
	return ms.GetConnectedPeersInfoCalled()
}

// TrimConnections -
func (ms *MessengerStub) TrimConnections() {
	ms.TrimConnectionsCalled()
}

// IsConnected -
func (ms *MessengerStub) IsConnected(peerID core.PeerID) bool {
	return ms.IsConnectedCalled(peerID)
}

// ConnectedPeers -
func (ms *MessengerStub) ConnectedPeers() []core.PeerID {
	if ms.ConnectedPeersCalled != nil {
		return ms.ConnectedPeersCalled()
	}

	return []core.PeerID{"peer0", "peer1", "peer2", "peer3", "peer4", "peer5"}
}

// CreateTopic -
func (ms *MessengerStub) CreateTopic(name string, createChannelForTopic bool) error {
	if ms.CreateTopicCalled != nil {
		return ms.CreateTopicCalled(name, createChannelForTopic)
	}
	return nil
}

// HasTopic -
func (ms *MessengerStub) HasTopic(name string) bool {
	return ms.HasTopicCalled(name)
}

// HasTopicValidator -
func (ms *MessengerStub) HasTopicValidator(name string) bool {
	return ms.HasTopicValidatorCalled(name)
}

// BroadcastOnChannel -
func (ms *MessengerStub) BroadcastOnChannel(channel string, topic string, buff []byte) {
	ms.BroadcastOnChannelCalled(channel, topic, buff)
}

// BroadcastOnChannelBlocking -
func (ms *MessengerStub) BroadcastOnChannelBlocking(channel string, topic string, buff []byte) error {
	return ms.BroadcastOnChannelBlockingCalled(channel, topic, buff)
}

// SendToConnectedPeer -
func (ms *MessengerStub) SendToConnectedPeer(topic string, buff []byte, peerID core.PeerID) error {
	return ms.SendToConnectedPeerCalled(topic, buff, peerID)
}

// Bootstrap -
func (ms *MessengerStub) Bootstrap() error {
	return ms.BootstrapCalled()
}

// IsInterfaceNil returns true if there is no value under the interface
func (ms *MessengerStub) IsInterfaceNil() bool {
	return ms == nil
}

// UnregisterAllMessageProcessors -
func (ms *MessengerStub) UnregisterAllMessageProcessors() error {
	if ms.UnregisterAllMessageProcessorsCalled != nil {
		return ms.UnregisterAllMessageProcessorsCalled()
	}

	return nil
}

// UnjoinAllTopics -
func (ms *MessengerStub) UnjoinAllTopics() error {
	if ms.UnjoinAllTopicsCalled != nil {
		return ms.UnjoinAllTopicsCalled()
	}

	return nil
}

// IsConnectedToTheNetwork -
func (ms *MessengerStub) IsConnectedToTheNetwork() bool {
	if ms.IsConnectedToTheNetworkCalled != nil {
		return ms.IsConnectedToTheNetworkCalled()
	}

	return true
}

// ThresholdMinConnectedPeers -
func (ms *MessengerStub) ThresholdMinConnectedPeers() int {
	if ms.ThresholdMinConnectedPeersCalled != nil {
		return ms.ThresholdMinConnectedPeersCalled()
	}

	return 0
}

// SetThresholdMinConnectedPeers -
func (ms *MessengerStub) SetThresholdMinConnectedPeers(minConnectedPeers int) error {
	if ms.SetThresholdMinConnectedPeersCalled != nil {
		return ms.SetThresholdMinConnectedPeersCalled(minConnectedPeers)
	}

	return nil
}

// SetPeerDenialEvaluator -
func (ms *MessengerStub) SetPeerDenialEvaluator(handler p2p.PeerDenialEvaluator) error {
	if ms.SetPeerDenialEvaluatorCalled != nil {
		return ms.SetPeerDenialEvaluatorCalled(handler)
	}

	return nil
}

// SetPeerShardResolver -
func (ms *MessengerStub) SetPeerShardResolver(peerShardResolver p2p.PeerShardResolver) error {
	if ms.SetPeerShardResolverCalled != nil {
		return ms.SetPeerShardResolverCalled(peerShardResolver)
	}

	return nil
}
