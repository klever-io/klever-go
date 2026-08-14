package libp2p

import (
	"context"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rate "github.com/libp2p/go-libp2p/x/rate"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/whyrusleeping/timecache"
)

var MaxSendBuffSize = maxSendBuffSize
var BroadcastGoRoutines = broadcastGoRoutines
var PubsubTimeCacheDuration = pubsubTimeCacheDuration
var AcceptMessagesInAdvanceDuration = acceptMessagesInAdvanceDuration

const CurrentTopicMessageVersion = currentTopicMessageVersion

var MaxInboundDirectStreamsPerPeer = maxInboundDirectStreamsPerPeer

// SetDirectRecvIdleTimeout shortens the inbound direct-stream idle read deadline for tests.
// Call before creating the messenger under test; the returned func restores the original.
func SetDirectRecvIdleTimeout(d time.Duration) func() {
	old := directRecvIdleTimeout
	directRecvIdleTimeout = d
	return func() { directRecvIdleTimeout = old }
}

func SetDirectSendWriteTimeout(d time.Duration) func() {
	old := directSendWriteTimeout
	directSendWriteTimeout = d
	return func() { directSendWriteTimeout = old }
}

// SetMaxInboundDirectStreamsTotal shrinks the cross-peer inbound direct-stream cap for tests.
// Call before creating the messenger under test; the returned func restores the original.
func SetMaxInboundDirectStreamsTotal(n int) func() {
	old := maxInboundDirectStreamsTotal
	maxInboundDirectStreamsTotal = n
	return func() { maxInboundDirectStreamsTotal = old }
}

// SetMaxConsecutiveInvalidFrames shrinks the invalid-frame tolerance for tests.
// Call before creating the messenger under test; the returned func restores the original.
func SetMaxConsecutiveInvalidFrames(n int) func() {
	old := maxConsecutiveInvalidFrames
	maxConsecutiveInvalidFrames = n
	return func() { maxConsecutiveInvalidFrames = old }
}

func (ds *directSender) DirectStreamHandler(s network.Stream) {
	ds.directStreamHandler(s)
}

func NewDirectSenderWithConfig(
	ctx context.Context,
	h host.Host,
	messageHandler func(msg *pubsub.Message, fromConnectedPeer core.PeerID) error,
	dsCfg config.DirectSendConfig,
) (*directSender, error) {
	return NewDirectSender(ctx, h, messageHandler, withDirectSendConfig(dsCfg))
}

func (ds *directSender) InboundStreamCaps() (int, int) {
	return ds.recvStreamsPerPeer, ds.recvStreamsTotalCap
}

func (netMes *networkMessenger) SetHost(newHost ConnectableHost) {
	netMes.p2pHost = newHost
}

func (netMes *networkMessenger) Host() ConnectableHost {
	return netMes.p2pHost
}

func (netMes *networkMessenger) SetLoadBalancer(outgoingPLB p2p.ChannelLoadBalancer) {
	netMes.outgoingPLB = outgoingPLB
}

func (netMes *networkMessenger) SetPeerDiscoverer(discoverer p2p.PeerDiscoverer) {
	netMes.peerDiscoverer = discoverer
}

func (netMes *networkMessenger) PubsubCallback(handler p2p.MessageProcessor, topic string) func(ctx context.Context, pid peer.ID, message *pubsub.Message) bool {
	return netMes.pubsubCallback(handler, topic)
}

func (netMes *networkMessenger) DirectMessageHandler(message *pubsub.Message, fromConnectedPeer core.PeerID) error {
	return netMes.directMessageHandler(message, fromConnectedPeer)
}

func (netMes *networkMessenger) ValidMessageByTimestamp(msg p2p.MessageP2P) error {
	return netMes.validMessageByTimestamp(msg)
}

func (ds *directSender) ValidateDirectMessage(message *pubsub_pb.Message, fromConnectedPeer peer.ID) error {
	return ds.validateDirectMessage(message, fromConnectedPeer)
}

func (ds *directSender) SeenMessages() *timecache.TimeCache {
	return ds.seenMessages
}

func (ds *directSender) Counter() uint64 {
	return ds.counter
}

func (mh *MutexHolder) Mutexes() storage.Cacher {
	return mh.mutexes
}

func (ip *identityProvider) HandleStreams(s network.Stream) {
	ip.handleStreams(s)
}

func (ip *identityProvider) ProcessReceivedData(recvBuff []byte) error {
	return ip.processReceivedData(recvBuff)
}

// CreateP2PPrivKey exports createP2PPrivKey for testing.
func CreateP2PPrivKey(seed string, legacySeed bool) (libp2pCrypto.PrivKey, error) {
	return createP2PPrivKey(seed, legacySeed)
}

// BuildAddressFactory exports buildAddressFactory for testing.
func BuildAddressFactory(broadcastIP string, port int) (func([]ma.Multiaddr) []ma.Multiaddr, error) {
	return buildAddressFactory(broadcastIP, port)
}

// ValidateResourceManagerConfig exposes validateResourceManagerConfig for testing.
func ValidateResourceManagerConfig(rmCfg config.ResourceManagerConfig) error {
	return validateResourceManagerConfig(rmCfg)
}

func ValidateDirectSendConfig(dsCfg config.DirectSendConfig) error {
	return validateDirectSendConfig(dsCfg)
}

// ScaledResourceManagerOptions exposes scaledResourceManagerOptions for testing.
func ScaledResourceManagerOptions(rmCfg config.ResourceManagerConfig) []rcmgr.Option {
	return scaledResourceManagerOptions(rmCfg)
}

// ConnLimitsPerSubnetV4 exposes connLimitsPerSubnetV4 for testing.
func ConnLimitsPerSubnetV4(maxConns int) []rcmgr.ConnLimitPerSubnet {
	return connLimitsPerSubnetV4(maxConns)
}

// ConnLimitsPerSubnetV6 exposes connLimitsPerSubnetV6 for testing.
func ConnLimitsPerSubnetV6(maxConns int) []rcmgr.ConnLimitPerSubnet {
	return connLimitsPerSubnetV6(maxConns)
}

// BuildConnRateLimiter exposes buildConnRateLimiter for testing.
func BuildConnRateLimiter(rps float64, burst int) *rate.Limiter {
	return buildConnRateLimiter(rps, burst)
}
