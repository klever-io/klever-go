package node

import (
	"io"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/closing"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/node/heartbeat/process"
)

// P2PMessenger defines a subset of the p2p.Messenger interface
type P2PMessenger interface {
	io.Closer
	Bootstrap() error
	Broadcast(topic string, buff []byte)
	BroadcastOnChannel(channel string, topic string, buff []byte)
	BroadcastOnChannelBlocking(channel string, topic string, buff []byte) error
	CreateTopic(name string, createChannelForTopic bool) error
	HasTopic(name string) bool
	HasTopicValidator(name string) bool
	RegisterMessageProcessor(topic string, handler p2p.MessageProcessor) error
	PeerAddresses(pid core.PeerID) []string
	IsConnectedToTheNetwork() bool
	ID() core.PeerID
	Peers() []core.PeerID
	ConnectedPeersOnTopic(topic string) []core.PeerID
	SendToConnectedPeer(topic string, buff []byte, peerID core.PeerID) error

	ConnectToPeer(address string) error
	Addresses() []string
	ConnectedAddresses() []string
	ConnectedPeers() []core.PeerID
	IsConnected(peerID core.PeerID) bool
	SetPeerShardResolver(peerShardResolver p2p.PeerShardResolver) error
	SetPeerDenialEvaluator(handler p2p.PeerDenialEvaluator) error
	GetConnectedPeersInfo() *p2p.ConnectedPeersInfo
	SetThresholdMinConnectedPeers(minConnectedPeers int) error
	ThresholdMinConnectedPeers() int
	UnjoinAllTopics() error
	UnregisterMessageProcessor(topic string) error
	UnregisterAllMessageProcessors() error

	IsInterfaceNil() bool
}

// NetworkShardingCollector defines the updating methods used by the network sharding component
// The interface assures that the collected data will be used by the p2p network sharding components
type NetworkShardingCollector interface {
	UpdatePeerIDPublicKey(pid core.PeerID, pk []byte)
	UpdatePeerID(pid core.PeerID)
	GetPeerInfo(pid core.PeerID) core.P2PPeerInfo
	IsInterfaceNil() bool
}

// P2PAntifloodHandler defines the behavior of a component able to signal that the system is too busy (or flooded) processing
// p2p messages
type P2PAntifloodHandler interface {
	CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopic(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error
	ResetForTopic(topic string)
	SetMaxMessagesForTopic(topic string, maxNum uint32)
	ApplyConsensusSize(size int)
	BlacklistPeer(peer core.PeerID, reason string, duration time.Duration)
	IsInterfaceNil() bool
}

// Accumulator defines the interface able to accumulate data and periodically evict them
type Accumulator interface {
	AddData(data interface{})
	OutputChannel() <-chan []interface{}
	IsInterfaceNil() bool
}

// HeartbeatHandler defines the behavior of a heartbeat handler
type HeartbeatHandler interface {
	Monitor() *process.Monitor
	Sender() *process.Sender
	IsInterfaceNil() bool
}

// HardforkTrigger defines the behavior of a hardfork trigger
type HardforkTrigger interface {
	TriggerReceived(payload []byte, data []byte, pkBytes []byte) (bool, error)
	RecordedTriggerMessage() ([]byte, bool)
	Trigger(epoch uint32, withEarlyEndOfEpoch bool) error
	CreateData() []byte
	AddCloser(closer closing.Closer) error
	NotifyTriggerReceived() <-chan struct{}
	IsSelfTrigger() bool
	IsInterfaceNil() bool
}

type TxCache interface {
	PendingFor([]byte) (uint64, uint64)
}
