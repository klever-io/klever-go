package libp2p

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rate "github.com/libp2p/go-libp2p/x/rate"
	ma "github.com/multiformats/go-multiaddr"
)

var SeenMessagesPerPeer = seenMessagesPerPeer
var BroadcastGoRoutines = broadcastGoRoutines
var PubsubTimeCacheDuration = pubsubTimeCacheDuration
var AcceptMessagesInAdvanceDuration = acceptMessagesInAdvanceDuration
var ScaledLimits = scaledLimits

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
	permissive := func(topic string) bool { return true }

	return NewDirectSender(ctx, h, messageHandler, withDirectSendConfig(dsCfg), withTopicProcessorChecker(permissive))
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

func (ds *directSender) ProcessInboundFrame(msg *pubsub_pb.Message, s network.Stream) error {
	return ds.processInboundFrame(msg, s)
}

func (ds *directSender) MarkSeen(from []byte, seqno []byte) {
	ds.seenMessages.hasOrAdd(string(from), binary.BigEndian.Uint64(seqno))
}

func (ds *directSender) HasSeen(from []byte, seqno []byte) bool {
	return ds.seenMessages.has(string(from), binary.BigEndian.Uint64(seqno))
}

func (ds *directSender) SeenMessagesLen() int {
	return ds.seenMessages.len()
}

// WithTopicProcessorChecker exposes the (required) constructor option to the external test
// package, which cannot name the unexported directSenderOption type but can pass one along.
func WithTopicProcessorChecker(hasTopicProcessor func(topic string) bool) directSenderOption {
	return withTopicProcessorChecker(hasTopicProcessor)
}

func (ds *directSender) Counter() uint64 {
	return ds.counter
}

func (mh *MutexHolder) Mutexes() storage.Cacher {
	return mh.mutexes
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

// ValidatorProtectionTag, ObserverTag and ObserverTagValue expose the peer protector's tags.
const ValidatorProtectionTag = validatorProtectionTag
const ObserverTag = observerTag
const ObserverTagValue = observerTagValue

// DefaultConnMgrLowWater and DefaultConnMgrHighWater expose the connection manager defaults.
const DefaultConnMgrLowWater = defaultConnMgrLowWater
const DefaultConnMgrHighWater = defaultConnMgrHighWater

// PeerProtectorPassResult exposes peerProtectorPassResult for testing.
type PeerProtectorPassResult = peerProtectorPassResult

// ProtectorNetwork exposes the network subset the peer protector uses.
type ProtectorNetwork = protectorNetwork

// MaxUnprotectedInboundConns exposes maxUnprotectedInboundConns for testing.
var MaxUnprotectedInboundConns = maxUnprotectedInboundConns

// RegularNodePeerConnsInbound exposes the regular-node per-peer inbound connection limit.
const RegularNodePeerConnsInbound = regularNodePeerConnsInbound

// NewPeerProtector builds a peer protector over an arbitrary connection manager and network.
// The inbound cap is armed and enforced from the first pass (no warm-up).
func NewPeerProtector(cm connmgr.ConnManager, net ProtectorNetwork, resolver p2p.PeerShardResolver, maxUnprotectedInboundConns int) *peerProtector {
	pp := NewPeerProtectorWithClock(cm, net, resolver, maxUnprotectedInboundConns, 0, time.Now)
	pp.arm()
	return pp
}

// Arm arms the inbound cap, as SetPeerShardResolver does.
func (pp *peerProtector) Arm() {
	pp.arm()
}

// SetUnknownPeerIdentificationWindow changes the identification window. The returned func
// restores the original.
func SetUnknownPeerIdentificationWindow(d time.Duration) func() {
	old := unknownPeerIdentificationWindow
	unknownPeerIdentificationWindow = d
	return func() { unknownPeerIdentificationWindow = old }
}

// NewPeerProtectorWithClock builds an unarmed peer protector with an explicit warm-up and clock.
func NewPeerProtectorWithClock(
	cm connmgr.ConnManager,
	net ProtectorNetwork,
	resolver p2p.PeerShardResolver,
	maxUnprotectedInboundConns int,
	warmUp time.Duration,
	now func() time.Time,
) *peerProtector {
	return newPeerProtector(cm, net, func() p2p.PeerShardResolver { return resolver }, maxUnprotectedInboundConns, warmUp, now)
}

// SetInboundCapWarmUp changes the inbound cap warm-up for messengers created afterwards. The
// returned func restores the original.
func SetInboundCapWarmUp(d time.Duration) func() {
	old := inboundCapWarmUp
	inboundCapWarmUp = d
	return func() { inboundCapWarmUp = old }
}

// RunPass runs one protector pass.
func (pp *peerProtector) RunPass() PeerProtectorPassResult {
	return pp.runPass()
}

// RunPeerProtectorPass runs one pass of the messenger's own peer protector. ok is false when the
// messenger has none (seeds, mocknet hosts).
func (netMes *networkMessenger) RunPeerProtectorPass() (res PeerProtectorPassResult, ok bool) {
	if netMes.peerProtector == nil {
		return PeerProtectorPassResult{}, false
	}
	return netMes.peerProtector.runPass(), true
}

// SetPeerProtectorInterval changes the protector loop interval for messengers created afterwards.
// The returned func restores the original.
func SetPeerProtectorInterval(d time.Duration) func() {
	old := peerProtectorInterval
	peerProtectorInterval = d
	return func() { peerProtectorInterval = old }
}

// SetDefaultConnMgrGracePeriod changes the grace period used when gracePeriodInSec is 0, for
// messengers created afterwards. The returned func restores the original.
func SetDefaultConnMgrGracePeriod(d time.Duration) func() {
	old := defaultConnMgrGracePeriod
	defaultConnMgrGracePeriod = d
	return func() { defaultConnMgrGracePeriod = old }
}

// ValidateConnectionManagerConfig exposes validateConnectionManagerConfig for testing.
func ValidateConnectionManagerConfig(cmCfg config.ConnectionManagerConfig) error {
	return validateConnectionManagerConfig(cmCfg)
}

// ConnManagerSettings exposes connManagerSettings for testing.
func ConnManagerSettings(cmCfg config.ConnectionManagerConfig) (int, int, time.Duration) {
	return connManagerSettings(cmCfg)
}

// ClampToPhysicalMemory exposes clampToPhysicalMemory for testing.
var ClampToPhysicalMemory = clampToPhysicalMemory

// GetFDLimit exposes getFDLimit for testing.
var GetFDLimit = getFDLimit

// ConnManagerLimitWarnings exposes connManagerLimitWarnings for testing.
var ConnManagerLimitWarnings = connManagerLimitWarnings

// SystemConnLimits exposes systemConnLimits for testing.
var SystemConnLimits = systemConnLimits

// ResourceManagerStrategy returns the effective ResourceManager strategy NewNetworkMessenger chose.
func (netMes *networkMessenger) ResourceManagerStrategy() string {
	return netMes.rmStrategy
}

// EffectiveResourceManagerConfig exposes effectiveResourceManagerConfig for testing.
func EffectiveResourceManagerConfig(rmCfg config.ResourceManagerConfig, isSeedNode bool) config.ResourceManagerConfig {
	return effectiveResourceManagerConfig(rmCfg, isSeedNode)
}

// BuildResourceManagerStrategy runs the messenger's ResourceManager setup for args and returns the
// effective strategy and, for "scaled", the system limits; it closes what it built.
func BuildResourceManagerStrategy(args ArgsNetworkMessenger) (string, *rcmgr.PartialLimitConfig, error) {
	setup, err := buildResourceManager(args)
	if err != nil {
		return "", nil, err
	}
	if setup.closer != nil {
		_ = setup.closer.Close()
	}
	if setup.limits == nil {
		return setup.strategy, nil, nil
	}
	partial := setup.limits.ToPartialLimitConfig()
	return setup.strategy, &partial, nil
}

// NewInboundCapGater exposes newInboundCapGater for testing, on the wall clock.
func NewInboundCapGater() *inboundCapGater {
	return newInboundCapGater(time.Now)
}

// NewInboundCapGaterWithClock exposes newInboundCapGater for testing.
func NewInboundCapGaterWithClock(now func() time.Time) *inboundCapGater {
	return newInboundCapGater(now)
}

// SetProtector binds the protector whose cap the gater enforces, without a network to notify it.
func (g *inboundCapGater) SetProtector(pp *peerProtector) {
	g.bind(pp, nil)
}

// PendingLen returns how many admitted connections are waiting to be registered.
func (g *inboundCapGater) PendingLen() int {
	g.mut.Lock()
	defer g.mut.Unlock()
	return len(g.pending)
}

// InboundCapPendingTTL exposes inboundCapPendingTTL.
var InboundCapPendingTTL = inboundCapPendingTTL

// InboundGater returns the messenger's connection gater (nil on seeds) and whether it is bound to
// the messenger's peer protector.
func (netMes *networkMessenger) InboundGater() (gater connmgr.ConnectionGater, bound bool) {
	if netMes.inboundGater == nil {
		return nil, false
	}
	return netMes.inboundGater, netMes.inboundGater.protector.Load() == netMes.peerProtector
}
