package libp2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/netip"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/mem"

	logging "github.com/ipfs/go-log/v2"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/data"
	connMonitorFactory "github.com/klever-io/klever-go/network/p2p/libp2p/connectionMonitor/factory"
	"github.com/klever-io/klever-go/network/p2p/libp2p/disabled"
	discoveryFactory "github.com/klever-io/klever-go/network/p2p/libp2p/discovery/factory"
	"github.com/klever-io/klever-go/network/p2p/libp2p/metrics"
	"github.com/klever-io/klever-go/network/p2p/libp2p/networksharding/factory"
	"github.com/klever-io/klever-go/network/p2p/loadBalancer"
	"github.com/klever-io/klever-go/tools/check"
	p2pDebug "github.com/klever-io/klever-go/tools/debug/p2p"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/connmgr"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	p2pConnMgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	rate "github.com/libp2p/go-libp2p/x/rate"
	ma "github.com/multiformats/go-multiaddr"
)

// ListenAddrWithIp4AndTcp defines the listening address with ip v.4 and TCP
const ListenAddrWithIp4AndTcp = "/ip4/0.0.0.0/tcp/"

// ListenLocalhostAddrWithIp4AndTcp defines the local host listening ip v.4 address and TCP
const ListenLocalhostAddrWithIp4AndTcp = "/ip4/127.0.0.1/tcp/"

// DirectSendID represents the protocol ID for sending and receiving direct P2P messages
const DirectSendID = protocol.ID("/klv/directsend/1.0.0")

const errFmtCreatingMessenger = "%w when creating a new network messenger"

const durationBetweenSends = time.Microsecond * 10
const durationCheckConnections = time.Second
const refreshPeersOnTopic = time.Second * 3
const ttlPeersOnTopic = time.Second * 10
const pubsubTimeCacheDuration = 10 * time.Minute
const acceptMessagesInAdvanceDuration = 5 * time.Second //we are accepting the messages with timestamp in the future only for this delta
const broadcastGoRoutines = 1000
const timeBetweenPeerPrints = time.Second * 20
const timeBetweenExternalLoggersCheck = time.Second * 20
const minRangePortValue = 1025
const noSignPolicy = pubsub.MessageSignaturePolicy(0) //should be used only in tests

var log = logger.GetOrCreate("p2p/libp2p")

var _ p2p.Messenger = (*networkMessenger)(nil)

// externalPackages are libp2p subsystem logger names bridged into klever-go's
// "external/<name>" namespace; setting external/<name>:TRACE flips the libp2p
// subsystem to DEBUG. rcmgr/swarm2/connmgr are needed to diagnose
// connection-acceptance issues (e.g. "failed to negotiate security protocol:
// EOF" when ResourceManager scopes fill).
var externalPackages = []string{
	"dht",
	"nat",
	"basichost",
	"pubsub",
	"swarm2",
	"rcmgr",
	"connmgr",
	"net/identify",
	"autonat",
}

func init() {
	for _, external := range externalPackages {
		_ = logger.GetOrCreate(fmt.Sprintf("external/%s", external))
	}
}

type networkMessenger struct {
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	p2pHost             ConnectableHost
	pb                  *pubsub.PubSub
	ds                  p2p.DirectSender
	connMonitor         ConnectionMonitor
	connMonitorWrapper  p2p.ConnectionMonitorWrapper
	peerDiscoverer      p2p.PeerDiscoverer
	sharder             p2p.CommonSharder
	mutPeerResolver     sync.RWMutex
	peerShardResolver   p2p.PeerShardResolver
	peerProtector       *peerProtector
	mutTopics           sync.RWMutex
	processors          map[string]p2p.MessageProcessor
	topics              map[string]*pubsub.Topic
	subscriptions       map[string]*pubsub.Subscription
	outgoingPLB         p2p.ChannelLoadBalancer
	poc                 *peersOnChannel
	goRoutinesThrottler *throttler.NumGoRoutinesThrottler
	connectionsMetric   *metrics.Connections
	debugger            p2p.Debugger
	marshalizer         p2p.Marshalizer
	syncTimer           p2p.SyncTimer
	rmStrategy          string
	inboundGater        *inboundCapGater
}

// ArgsNetworkMessenger defines the options used to create a p2p wrapper
type ArgsNetworkMessenger struct {
	ListenAddress string
	Marshalizer   p2p.Marshalizer
	P2pConfig     config.P2PConfig
	SyncTimer     p2p.SyncTimer
	// P2pPrivateKey is the node's libp2p identity key (see the p2pkey package)
	P2pPrivateKey libp2pCrypto.PrivKey
	// IsSeedNode selects the no-op NullConnMgr, so a seed never trims the peers it serves.
	// Regular nodes, whatever the sharder type, run a BasicConnMgr configured from
	// P2pConfig.ConnectionManager that protects known validators (see newConnManager).
	// A regular node without a resourceManager section also gets the "scaled" ResourceManager
	// (see effectiveResourceManagerConfig).
	IsSeedNode bool
}

// NewNetworkMessenger creates a libP2P messenger by opening a port on the current machine
func NewNetworkMessenger(args ArgsNetworkMessenger) (*networkMessenger, error) {
	err := checkNetworkMessengerArgs(args)
	if err != nil {
		return nil, err
	}

	port, err := getPort(args.P2pConfig.Node.Port, checkFreePort)
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf(args.ListenAddress+"%d", port)

	sourceMultiAddr, _ := ma.NewMultiaddr(address)
	addressFactory, err := buildAddressFactory(args.P2pConfig.Node.BroadcastIP, port)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(args.P2pPrivateKey),
		libp2p.AddrsFactory(addressFactory),
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.DefaultTransports,
		//we need the disable relay option in order to save the node's bandwidth as much as possible
		libp2p.DisableRelay(),
		libp2p.NATPortMap(),
	}

	rmSetup, err := buildResourceManager(args)
	if err != nil {
		return nil, err
	}
	if rmSetup.option != nil {
		opts = append(opts, rmSetup.option)
	}

	cm, err := newConnManager(args)
	if err != nil {
		if rmSetup.closer != nil {
			_ = rmSetup.closer.Close()
		}
		return nil, err
	}
	opts = append(opts, libp2p.ConnectionManager(cm))

	// Seeds run no gater: they must accept everyone, and validators rely on reaching them.
	var gater *inboundCapGater
	if !args.IsSeedNode {
		gater = newInboundCapGater(time.Now)
		opts = append(opts, libp2p.ConnectionGater(gater))
	}

	setupExternalP2PLoggers()

	ctx, cancelFunc := context.WithCancel(context.Background())
	h, err := libp2p.New(opts...)
	if err != nil {
		cancelFunc()
		// libp2p v0.48 closes the ResourceManager and ConnManager it was given only when its
		// config validation fails, not when an option or the fx app start fails, so close both
		// here. Both Close methods are idempotent, so a double close is harmless.
		if rmSetup.closer != nil {
			_ = rmSetup.closer.Close()
		}
		_ = cm.Close()
		return nil, err
	}

	logNetworkLimits(args, rmSetup.strategy, h.Network().ResourceManager())

	p2pNode, err := createMessenger(args, h, ctx, cancelFunc, true)
	if err != nil {
		// createMessenger has already started goroutines bound to ctx (pubsub, connection monitor).
		cancelFunc()
		log.LogIfError(h.Close())
		return nil, err
	}
	p2pNode.rmStrategy = rmSetup.strategy
	p2pNode.bindInboundGater(gater, h.Network())

	return p2pNode, nil
}

// checkNetworkMessengerArgs rejects nil dependencies and invalid p2p config before anything is
// opened or allocated.
func checkNetworkMessengerArgs(args ArgsNetworkMessenger) error {
	if check.IfNil(args.Marshalizer) {
		return fmt.Errorf(errFmtCreatingMessenger, p2p.ErrNilMarshalizer)
	}
	if check.IfNil(args.SyncTimer) {
		return fmt.Errorf(errFmtCreatingMessenger, p2p.ErrNilSyncTimer)
	}
	if check.IfNilReflect(args.P2pPrivateKey) {
		return fmt.Errorf(errFmtCreatingMessenger, p2p.ErrNilP2pPrivateKey)
	}

	err := validateDirectSendConfig(args.P2pConfig.DirectSend)
	if err != nil {
		return err
	}

	return validateConnectionManagerConfig(args.P2pConfig.ConnectionManager)
}

// bindInboundGater attaches the inbound cap gater (nil on seeds) to the messenger and, once the
// peer protector exists, binds the two so the gater starts enforcing the cap.
func (netMes *networkMessenger) bindInboundGater(gater *inboundCapGater, net network.Network) {
	if gater == nil {
		return
	}
	netMes.inboundGater = gater
	if netMes.peerProtector != nil {
		gater.bind(netMes.peerProtector, net)
	}
}

// buildAddressFactory returns an AddrsFactory that appends the operator-supplied
// external multiaddr (built from broadcastIP and port) to the host's advertised
// addresses. Returns an identity factory when broadcastIP is empty.
func buildAddressFactory(broadcastIP string, port int) (func([]ma.Multiaddr) []ma.Multiaddr, error) {
	if len(broadcastIP) == 0 {
		return func(addrs []ma.Multiaddr) []ma.Multiaddr { return addrs }, nil
	}
	extMultiAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", broadcastIP, port))
	if err != nil {
		log.Error("broadcastIP", "err", err.Error())
		return nil, err
	}
	return func(addrs []ma.Multiaddr) []ma.Multiaddr {
		return append(addrs, extMultiAddr)
	}, nil
}

func setupExternalP2PLoggers() {
	for _, external := range externalPackages {
		logLevel := logger.GetLoggerLogLevel("external/" + external)
		if logLevel > logger.LogTrace {
			continue
		}

		_ = logging.SetLogLevel(external, "DEBUG")
	}
}

func createMessenger(
	args ArgsNetworkMessenger,
	p2pHost host.Host,
	ctx context.Context,
	cancelFunc context.CancelFunc,
	withMessageSigning bool,
) (*networkMessenger, error) {
	var err error
	netMes := networkMessenger{
		ctx:               ctx,
		cancelFunc:        cancelFunc,
		p2pHost:           NewConnectableHost(p2pHost),
		processors:        make(map[string]p2p.MessageProcessor),
		topics:            make(map[string]*pubsub.Topic),
		subscriptions:     make(map[string]*pubsub.Subscription),
		outgoingPLB:       loadBalancer.NewOutgoingChannelLoadBalancer(),
		peerShardResolver: &unknownPeerShardResolver{},
		marshalizer:       args.Marshalizer,
		syncTimer:         args.SyncTimer,
	}
	netMes.debugger = p2pDebug.NewP2PDebugger(core.PeerID(p2pHost.ID()))

	err = netMes.createPubSub(withMessageSigning)
	if err != nil {
		return nil, err
	}

	err = netMes.createSharder(args.P2pConfig)
	if err != nil {
		return nil, err
	}

	err = netMes.createDiscoverer(args.P2pConfig)
	if err != nil {
		return nil, err
	}

	err = netMes.createConnectionMonitor(args.P2pConfig)
	if err != nil {
		return nil, err
	}

	netMes.createConnectionsMetric()

	netMes.ds, err = NewDirectSender(
		ctx,
		p2pHost,
		netMes.directMessageHandler,
		withDirectSendConfig(args.P2pConfig.DirectSend),
		withTopicProcessorChecker(netMes.hasTopicProcessor),
	)
	if err != nil {
		return nil, err
	}

	netMes.goRoutinesThrottler, err = throttler.NewNumGoRoutinesThrottler(broadcastGoRoutines)
	if err != nil {
		return nil, err
	}

	// Started last: createMessenger's callers do not cancel ctx when it fails.
	netMes.createPeerProtector(args, p2pHost)

	netMes.printLogs()

	return &netMes, nil
}

func (netMes *networkMessenger) createPubSub(withMessageSigning bool) error {
	optsPS := []pubsub.Option{
		// Configure the seen-messages TTL per pubsub instance instead of mutating
		// the package-level pubsub.TimeCacheDuration global. Writing the global on
		// every construction races when messengers are built concurrently (e.g.
		// parallel tests). WithSeenMessagesTTL is the libp2p-recommended way to set
		// this per instance.
		pubsub.WithSeenMessagesTTL(pubsubTimeCacheDuration),
	}
	if !withMessageSigning {
		log.Warn("signature verification is turned off in network messenger instance")
		optsPS = append(optsPS, pubsub.WithMessageSignaturePolicy(noSignPolicy))
	}

	var err error
	netMes.pb, err = pubsub.NewGossipSub(netMes.ctx, netMes.p2pHost, optsPS...)
	if err != nil {
		return err
	}

	netMes.poc, err = newPeersOnChannel(
		netMes.pb.ListPeers,
		refreshPeersOnTopic,
		ttlPeersOnTopic)
	if err != nil {
		return err
	}

	go func(plb p2p.ChannelLoadBalancer) {
		for {
			select {
			case <-time.After(durationBetweenSends):
			case <-netMes.ctx.Done():
				return
			}

			sendableData := plb.CollectOneElementFromChannels()
			if sendableData == nil {
				continue
			}

			netMes.mutTopics.RLock()
			topic := netMes.topics[sendableData.Topic]
			netMes.mutTopics.RUnlock()

			if topic == nil {
				log.Warn("writing on a topic that the node did not register on - message dropped",
					"topic", sendableData.Topic,
				)

				continue
			}

			buffToSend := netMes.createMessageBytes(sendableData.Buff)
			if len(buffToSend) == 0 {
				continue
			}

			errPublish := topic.Publish(netMes.ctx, buffToSend)
			if errPublish != nil {
				log.Trace("error sending data", "error", errPublish)
			}
		}
	}(netMes.outgoingPLB)

	return nil
}

func (netMes *networkMessenger) createMessageBytes(buff []byte) []byte {
	message := &data.TopicMessage{
		Version:   currentTopicMessageVersion,
		Payload:   buff,
		Timestamp: netMes.syncTimer.CurrentTime().Unix(),
	}

	buffToSend, errMarshal := netMes.marshalizer.Marshal(message)
	if errMarshal != nil {
		log.Warn("error sending data", "error", errMarshal)
		return nil
	}

	return buffToSend
}

func (netMes *networkMessenger) createSharder(p2pConfig config.P2PConfig) error {
	args := factory.ArgsSharderFactory{
		PeerShardResolver:       &unknownPeerShardResolver{},
		Pid:                     netMes.p2pHost.ID(),
		MaxConnectionCount:      p2pConfig.Sharding.TargetPeerCount,
		MaxIntraShardValidators: int(p2pConfig.Sharding.MaxIntraShardValidators),
		MaxCrossShardValidators: int(p2pConfig.Sharding.MaxCrossShardValidators),
		MaxIntraShardObservers:  int(p2pConfig.Sharding.MaxIntraShardObservers),
		MaxCrossShardObservers:  int(p2pConfig.Sharding.MaxCrossShardObservers),
		Type:                    p2pConfig.Sharding.Type,
	}

	var err error
	netMes.sharder, err = factory.NewSharder(args)

	return err
}

func (netMes *networkMessenger) createDiscoverer(p2pConfig config.P2PConfig) error {
	var err error
	netMes.peerDiscoverer, err = discoveryFactory.NewPeerDiscoverer(
		netMes.ctx,
		netMes.p2pHost,
		netMes.sharder,
		p2pConfig,
	)

	return err
}

func (netMes *networkMessenger) createConnectionMonitor(p2pConfig config.P2PConfig) error {
	reconnecter, ok := netMes.peerDiscoverer.(p2p.Reconnecter)
	if !ok {
		return fmt.Errorf("%w when converting peerDiscoverer to reconnecter interface", p2p.ErrWrongTypeAssertion)
	}

	args := connMonitorFactory.ArgsConnectionMonitorFactory{
		Reconnecter:                reconnecter,
		Sharder:                    netMes.sharder,
		ThresholdMinConnectedPeers: p2pConfig.Node.ThresholdMinConnectedPeers,
		TargetCount:                p2pConfig.Sharding.TargetPeerCount,
	}
	var err error
	netMes.connMonitor, err = connMonitorFactory.NewConnectionMonitor(args)
	if err != nil {
		return err
	}

	cmw := newConnectionMonitorWrapper(
		netMes.p2pHost.Network(),
		netMes.connMonitor,
		&disabled.NilPeerDenialEvaluator{},
	)
	netMes.p2pHost.Network().Notify(cmw)
	netMes.connMonitorWrapper = cmw

	go func() {
		// Bound this loop to the messenger's lifetime. Previously this
		// goroutine had no termination signal and leaked on Close(), which
		// accumulated under test rigs that spin clusters up and down in the
		// same process (-count=N integration tests).
		ticker := time.NewTicker(durationCheckConnections)
		defer ticker.Stop()
		for {
			cmw.CheckConnectionsBlocking()
			select {
			case <-netMes.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

// createPeerProtector starts the loop that protects known validators in the host's connection
// manager. Seeds run NullConnMgr, as do hosts built without a connection manager (mocknet), so
// there is nothing to protect there and no loop is started.
func (netMes *networkMessenger) createPeerProtector(args ArgsNetworkMessenger, p2pHost host.Host) {
	if args.IsSeedNode {
		return
	}

	cm := p2pHost.ConnManager()
	switch cm.(type) {
	case connmgr.NullConnMgr, *connmgr.NullConnMgr:
		return
	}

	netMes.peerProtector = newPeerProtector(
		cm,
		p2pHost.Network(),
		netMes.getPeerShardResolver,
		maxUnprotectedInboundConns(args.P2pConfig.ConnectionManager),
		inboundCapWarmUp,
		time.Now,
	)
	netMes.peerProtector.start(netMes.ctx, peerProtectorInterval)
}

func (netMes *networkMessenger) createConnectionsMetric() {
	netMes.connectionsMetric = metrics.NewConnections()
	netMes.p2pHost.Network().Notify(netMes.connectionsMetric)
}

func (netMes *networkMessenger) printLogs() {
	addresses := make([]interface{}, 0)
	for i, address := range netMes.p2pHost.Addrs() {
		addresses = append(addresses, fmt.Sprintf("addr%d", i))
		addresses = append(addresses, address.String()+"/p2p/"+netMes.ID().Pretty())
	}
	log.Info("listening on addresses", addresses...)

	go netMes.printLogsStats()
	go netMes.checkExternalLoggers()
}

func (netMes *networkMessenger) printLogsStats() {
	for {
		select {
		case <-netMes.ctx.Done():
			return
		case <-time.After(timeBetweenPeerPrints):
		}

		conns := netMes.connectionsMetric.ResetNumConnections()
		disconns := netMes.connectionsMetric.ResetNumDisconnections()

		peersInfo := netMes.GetConnectedPeersInfo()
		log.Debug("network connection status",
			"known peers", len(netMes.Peers()),
			"connected peers", len(netMes.ConnectedPeers()),
			"intra shard validators", peersInfo.NumIntraShardValidators,
			"intra shard observers", peersInfo.NumIntraShardObservers,
			"cross shard validators", peersInfo.NumCrossShardValidators,
			"cross shard observers", peersInfo.NumCrossShardObservers,
			"unknown", len(peersInfo.UnknownPeers),
			"validators histogram", netMes.mapHistogram(peersInfo.NumValidatorsOnShard),
			"observers histogram", netMes.mapHistogram(peersInfo.NumObserversOnShard),
		)

		connsPerSec := conns / uint32(timeBetweenPeerPrints/time.Second)
		disconnsPerSec := disconns / uint32(timeBetweenPeerPrints/time.Second)

		log.Debug("network connection metrics",
			"connections/s", connsPerSec,
			"disconnections/s", disconnsPerSec,
		)
	}
}

func (netMes *networkMessenger) mapHistogram(input map[uint32]int) string {
	keys := make([]uint32, 0, len(input))
	for shard := range input {
		keys = append(keys, shard)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	vals := make([]string, 0, len(keys))
	for _, key := range keys {
		shard := "meta"

		vals = append(vals, fmt.Sprintf("%s: %d", shard, input[key]))
	}

	return strings.Join(vals, ", ")
}

func (netMes *networkMessenger) checkExternalLoggers() {
	for {
		select {
		case <-netMes.ctx.Done():
			return
		case <-time.After(timeBetweenExternalLoggersCheck):
		}

		setupExternalP2PLoggers()
	}
}

// Close closes the host, connections and streams
func (netMes *networkMessenger) Close() error {
	log.Debug("closing network messenger's host...")

	var err error
	errHost := netMes.p2pHost.Close()
	if errHost != nil {
		err = errHost
		log.Warn("networkMessenger.Close",
			"component", "host",
			"error", err)
	}

	log.Debug("closing network messenger's outgoing load balancer...")

	errOplb := netMes.outgoingPLB.Close()
	if errOplb != nil {
		err = errOplb
		log.Warn("networkMessenger.Close",
			"component", "outgoingPLB",
			"error", err)
	}

	log.Debug("closing network messenger's components through the context...")
	netMes.cancelFunc()

	log.Debug("closing network messenger's debugger...")
	errDebugger := netMes.debugger.Close()
	if errDebugger != nil {
		err = errDebugger
		log.Warn("networkMessenger.Close",
			"component", "debugger",
			"error", err)
	}

	if err == nil {
		log.Info("network messenger closed successfully")
	}

	return err
}

// ID returns the messenger's ID
func (netMes *networkMessenger) ID() core.PeerID {
	h := netMes.p2pHost

	return core.PeerID(h.ID())
}

// Peers returns the list of all known peers ID (including self)
func (netMes *networkMessenger) Peers() []core.PeerID {
	peers := make([]core.PeerID, 0)

	for _, p := range netMes.p2pHost.Peerstore().Peers() {
		peers = append(peers, core.PeerID(p))
	}
	return peers
}

// Addresses returns all addresses found in peerstore
func (netMes *networkMessenger) Addresses() []string {
	addrs := make([]string, 0)

	for _, address := range netMes.p2pHost.Addrs() {
		addrs = append(addrs, address.String()+"/p2p/"+netMes.ID().Pretty())
	}

	return addrs
}

// ConnectToPeer tries to open a new connection to a peer
func (netMes *networkMessenger) ConnectToPeer(address string) error {
	return netMes.p2pHost.ConnectToPeer(netMes.ctx, address)
}

// Bootstrap will start the peer discovery mechanism
func (netMes *networkMessenger) Bootstrap() error {
	return netMes.peerDiscoverer.Bootstrap()
}

// IsConnected returns true if current node is connected to provided peer
func (netMes *networkMessenger) IsConnected(peerID core.PeerID) bool {
	h := netMes.p2pHost

	connectedness := h.Network().Connectedness(peer.ID(peerID))

	return connectedness == network.Connected
}

// ConnectedPeers returns the current connected peers list
func (netMes *networkMessenger) ConnectedPeers() []core.PeerID {
	h := netMes.p2pHost

	connectedPeers := make(map[core.PeerID]struct{})

	for _, conn := range h.Network().Conns() {
		p := core.PeerID(conn.RemotePeer())

		if netMes.IsConnected(p) {
			connectedPeers[p] = struct{}{}
		}
	}

	peerList := make([]core.PeerID, len(connectedPeers))

	index := 0
	for k := range connectedPeers {
		peerList[index] = k
		index++
	}

	return peerList
}

// ConnectedAddresses returns all connected peer's addresses
func (netMes *networkMessenger) ConnectedAddresses() []string {
	h := netMes.p2pHost
	conns := make([]string, 0)

	for _, c := range h.Network().Conns() {
		conns = append(conns, c.RemoteMultiaddr().String()+"/p2p/"+c.RemotePeer().String())
	}
	return conns
}

// PeerAddresses returns the peer's addresses or empty slice if the peer is unknown
func (netMes *networkMessenger) PeerAddresses(pid core.PeerID) []string {
	h := netMes.p2pHost

	resultMap := make(map[string]interface{})

	//check if the peer is connected to return it's connected address
	for _, c := range h.Network().Conns() {
		if string(c.RemotePeer()) == string(pid.Bytes()) {
			resultMap[c.RemoteMultiaddr().String()] = nil
			break
		}
	}

	//check in peerstore (maybe it is known but not connected)
	addresses := h.Peerstore().Addrs(peer.ID(pid.Bytes()))
	for _, addr := range addresses {
		resultMap[addr.String()] = nil
	}

	result := make([]string, 0)
	_ = types.NewDeterministicMap(resultMap).Each(func(key string, value interface{}) error {
		result = append(result, key)
		return nil
	})

	return result
}

// ConnectedPeersOnTopic returns the connected peers on a provided topic
func (netMes *networkMessenger) ConnectedPeersOnTopic(topic string) []core.PeerID {
	return netMes.poc.ConnectedPeersOnChannel(topic)
}

// CreateTopic opens a new topic using pubsub infrastructure
func (netMes *networkMessenger) CreateTopic(name string, createChannelForTopic bool) error {
	netMes.mutTopics.Lock()
	defer netMes.mutTopics.Unlock()
	_, found := netMes.topics[name]
	if found {
		return nil
	}

	topic, err := netMes.pb.Join(name)
	if err != nil {
		return fmt.Errorf("%w for topic %s", err, name)
	}

	netMes.topics[name] = topic
	subscrRequest, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("%w for topic %s", err, name)
	}

	netMes.subscriptions[name] = subscrRequest
	if createChannelForTopic {
		err = netMes.outgoingPLB.AddChannel(name)
	}

	//just a dummy func to consume messages received by the newly created topic
	go func() {
		var errSubscrNext error
		for {
			_, errSubscrNext = subscrRequest.Next(netMes.ctx)
			if errSubscrNext != nil {
				log.Debug("closed subscription",
					"topic", subscrRequest.Topic(),
					"err", errSubscrNext,
				)
				return
			}
		}
	}()

	return err
}

// HasTopic returns true if the topic has been created
func (netMes *networkMessenger) HasTopic(name string) bool {
	netMes.mutTopics.RLock()
	_, found := netMes.topics[name]
	netMes.mutTopics.RUnlock()

	return found
}

// HasTopicValidator returns true if the topic has a validator set
func (netMes *networkMessenger) HasTopicValidator(name string) bool {
	netMes.mutTopics.RLock()
	validator := netMes.processors[name]
	netMes.mutTopics.RUnlock()

	return validator != nil
}

// BroadcastOnChannelBlocking tries to send a byte buffer onto a topic using provided channel
// It is a blocking method. It needs to be launched on a go routine
func (netMes *networkMessenger) BroadcastOnChannelBlocking(channel string, topic string, buff []byte) error {
	err := netMes.checkSendableData(buff)
	if err != nil {
		return err
	}

	if !netMes.goRoutinesThrottler.CanProcess() {
		return p2p.ErrTooManyGoroutines
	}

	netMes.goRoutinesThrottler.StartProcessing()
	// Deferred: a panic on the send (a closed channel, say) would otherwise leak the slot, and a
	// caller that recovers it would, one leak at a time, starve every broadcast on this throttler.
	defer netMes.goRoutinesThrottler.EndProcessing()

	sendable := &p2p.SendableData{
		Buff:  buff,
		Topic: topic,
	}
	netMes.outgoingPLB.GetChannelOrDefault(channel) <- sendable
	return nil
}

func (netMes *networkMessenger) checkSendableData(buff []byte) error {
	if len(buff) > p2p.MaxSendBuffSize {
		return fmt.Errorf("%w, to be sent: %d, maximum: %d", p2p.ErrMessageTooLarge, len(buff), p2p.MaxSendBuffSize)
	}
	if len(buff) == 0 {
		return p2p.ErrEmptyBufferToSend
	}

	return nil
}

// BroadcastOnChannel tries to send a byte buffer onto a topic using provided channel
func (netMes *networkMessenger) BroadcastOnChannel(channel string, topic string, buff []byte) {
	go func() {
		err := netMes.BroadcastOnChannelBlocking(channel, topic, buff)
		if err != nil {
			log.Warn("p2p broadcast", "error", err.Error())
		}
	}()
}

// Broadcast tries to send a byte buffer onto a topic using the topic name as channel
func (netMes *networkMessenger) Broadcast(topic string, buff []byte) {
	netMes.BroadcastOnChannel(topic, topic, buff)
}

// RegisterMessageProcessor registers a message process on a topic
func (netMes *networkMessenger) RegisterMessageProcessor(topic string, handler p2p.MessageProcessor) error {
	if check.IfNil(handler) {
		return fmt.Errorf("%w when calling networkMessenger.RegisterMessageProcessor for topic %s",
			p2p.ErrNilValidator, topic)
	}

	netMes.mutTopics.Lock()
	defer netMes.mutTopics.Unlock()
	validator := netMes.processors[topic]
	if !check.IfNil(validator) {
		return fmt.Errorf("%w, operation RegisterMessageProcessor, topic %s",
			p2p.ErrTopicValidatorOperationNotSupported,
			topic,
		)
	}

	err := netMes.pb.RegisterTopicValidator(topic, netMes.pubsubCallback(handler, topic))
	if err != nil {
		return err
	}

	netMes.processors[topic] = handler
	return nil
}

func (netMes *networkMessenger) pubsubCallback(handler p2p.MessageProcessor, topic string) func(ctx context.Context, pid peer.ID, message *pubsub.Message) bool {
	return func(ctx context.Context, pid peer.ID, message *pubsub.Message) (isValid bool) {
		fromConnectedPeer := core.PeerID(pid)

		// Circuit-breaker: recover so a panic on an untrusted message cannot crash the
		// node (GHSA-rm5c-5x2p-48wr).
		defer func() {
			if r := recover(); r != nil {
				netMes.recoverFromMessagePanic(r, topic, fromConnectedPeer, message)
				dataLen := 0
				if message != nil {
					dataLen = len(message.GetData())
				}
				netMes.processDebugMessage(topic, fromConnectedPeer, uint64(dataLen), true)
				isValid = false
			}
		}()

		msg, err := netMes.transformAndCheckMessage(message, fromConnectedPeer, topic)
		if err != nil {
			log.Trace("p2p validator - new message", "error", err.Error(), "topic", topic)
			return false
		}

		err = handler.ProcessReceivedMessage(msg, fromConnectedPeer)
		if err != nil {
			log.Trace("p2p validator pubsubCallback",
				"error", err.Error(),
				"topic", topic,
				"originator", p2p.MessageOriginatorPid(msg),
				"from connected peer", p2p.PeerIDToShortString(fromConnectedPeer),
				"seq no", p2p.MessageOriginatorSeq(msg),
			)
			netMes.processDebugMessage(topic, fromConnectedPeer, uint64(len(message.Data)), true)
			return false
		}

		netMes.processDebugMessage(topic, fromConnectedPeer, uint64(len(message.Data)), false)
		return true
	}
}

// recoverFromMessagePanic logs a recovered panic from processing an untrusted p2p
// message and blacklists both the connected peer and the originator, using nil-safe
// accessors. Shared by the gossip and direct-send receive paths (GHSA-rm5c-5x2p-48wr).
func (netMes *networkMessenger) recoverFromMessagePanic(r interface{}, topic string, fromConnectedPeer core.PeerID, message *pubsub.Message) {
	var originator core.PeerID
	if message != nil {
		originator = core.PeerID(message.GetFrom())
	}

	log.Error("recovered from panic while processing p2p message",
		"topic", topic,
		"originator", p2p.PeerIDToShortString(originator),
		"from connected peer", p2p.PeerIDToShortString(fromConnectedPeer),
		"panic", fmt.Sprintf("%v", r),
		"stack", string(debug.Stack()),
	)
	netMes.blacklistPid(fromConnectedPeer, core.WrongP2PMessageBlacklistDuration)
	netMes.blacklistPid(originator, core.WrongP2PMessageBlacklistDuration)
}

func (netMes *networkMessenger) transformAndCheckMessage(pbMsg *pubsub.Message, pid core.PeerID, topic string) (p2p.MessageP2P, error) {
	msg, errUnmarshal := NewMessage(pbMsg, netMes.marshalizer)
	if errUnmarshal != nil {
		//this error is so severe that will need to blacklist both the originator and the connected peer as there is
		// no way this node can communicate with them
		pidFrom := core.PeerID(pbMsg.From)
		netMes.blacklistPid(pid, core.WrongP2PMessageBlacklistDuration)
		netMes.blacklistPid(pidFrom, core.WrongP2PMessageBlacklistDuration)

		return nil, errUnmarshal
	}

	err := netMes.validMessageByTimestamp(msg)
	if err != nil {
		//not reprocessing nor rebrodcasting the same message over and over again
		log.Trace("received an invalid message",
			"originator pid", p2p.MessageOriginatorPid(msg),
			"from connected pid", p2p.PeerIDToShortString(pid),
			"sequence", hex.EncodeToString(msg.SeqNo()),
			"timestamp", msg.Timestamp(),
			"error", err,
		)
		netMes.processDebugMessage(topic, pid, uint64(len(msg.Data())), true)

		return nil, err
	}

	return msg, nil
}

func (netMes *networkMessenger) blacklistPid(pid core.PeerID, banDuration time.Duration) {
	if netMes.connMonitorWrapper.PeerDenialEvaluator().IsDenied(pid) {
		return
	}
	if len(pid) == 0 {
		return
	}

	log.Debug("blacklisted due to incompatible p2p message",
		"pid", pid.Pretty(),
		"time", banDuration,
	)

	err := netMes.connMonitorWrapper.PeerDenialEvaluator().UpsertPeerID(pid, banDuration)
	if err != nil {
		log.Warn("error blacklisting peer ID in network messenger",
			"pid", pid.Pretty(),
			"error", err.Error(),
		)
	}
}

// invalidMessageByTimestamp will check that the message time stamp should be in the interval
// (now-pubsubTimeCacheDuration+acceptMessagesInAdvanceDuration, now+acceptMessagesInAdvanceDuration)
func (netMes *networkMessenger) validMessageByTimestamp(msg p2p.MessageP2P) error {
	now := netMes.syncTimer.CurrentTime()
	isInFuture := now.Add(acceptMessagesInAdvanceDuration).Unix() < msg.Timestamp()
	if isInFuture {
		return fmt.Errorf("%w, self timestamp %d, message timestamp %d",
			p2p.ErrMessageTooNew, now.Unix(), msg.Timestamp())
	}

	past := now.Unix() - int64(pubsubTimeCacheDuration.Seconds())
	if msg.Timestamp() < past {
		return fmt.Errorf("%w, self timestamp %d, message timestamp %d",
			p2p.ErrMessageTooOld, now.Unix(), msg.Timestamp())
	}

	return nil
}

func (netMes *networkMessenger) processDebugMessage(topic string, fromConnectedPeer core.PeerID, size uint64, isRejected bool) {
	if fromConnectedPeer == netMes.ID() {
		netMes.debugger.AddOutgoingMessage(topic, size, isRejected)
	} else {
		netMes.debugger.AddIncomingMessage(topic, size, isRejected)
	}
}

// UnregisterAllMessageProcessors will unregister all message processors for topics
func (netMes *networkMessenger) UnregisterAllMessageProcessors() error {
	netMes.mutTopics.Lock()
	defer netMes.mutTopics.Unlock()

	for topic, validator := range netMes.processors {
		if check.IfNil(validator) {
			continue
		}

		err := netMes.pb.UnregisterTopicValidator(topic)
		if err != nil {
			return err
		}

		delete(netMes.processors, topic)
	}
	return nil
}

// UnjoinAllTopics call close on all topics
func (netMes *networkMessenger) UnjoinAllTopics() error {
	netMes.mutTopics.Lock()
	defer netMes.mutTopics.Unlock()

	var errFound error
	for topicName, t := range netMes.topics {
		subscr := netMes.subscriptions[topicName]
		if subscr != nil {
			subscr.Cancel()
		}

		err := t.Close()
		if err != nil {
			log.Warn("error closing topic",
				"topic", topicName,
				"error", err,
			)
			errFound = err
		}

		delete(netMes.topics, topicName)
	}

	return errFound
}

// UnregisterMessageProcessor unregisters a message processes on a topic
func (netMes *networkMessenger) UnregisterMessageProcessor(topic string) error {
	netMes.mutTopics.Lock()
	defer netMes.mutTopics.Unlock()

	validator := netMes.processors[topic]
	if check.IfNil(validator) {
		return nil
	}

	err := netMes.pb.UnregisterTopicValidator(topic)
	if err != nil {
		return err
	}

	netMes.processors[topic] = nil
	return nil
}

// SendToConnectedPeer sends a direct message to a connected peer
func (netMes *networkMessenger) SendToConnectedPeer(topic string, buff []byte, peerID core.PeerID) error {
	err := netMes.checkSendableData(buff)
	if err != nil {
		return err
	}

	buffToSend := netMes.createMessageBytes(buff)
	if len(buffToSend) == 0 {
		return nil
	}

	if peerID == netMes.ID() {
		return netMes.sendDirectToSelf(topic, buffToSend)
	}

	err = netMes.ds.Send(topic, buffToSend, peerID)
	netMes.debugger.AddOutgoingMessage(topic, uint64(len(buffToSend)), err != nil)

	return err
}

func (netMes *networkMessenger) sendDirectToSelf(topic string, buff []byte) error {
	msg := &pubsub.Message{
		Message: &pubsub_pb.Message{
			From:      netMes.ID().Bytes(),
			Data:      buff,
			Seqno:     netMes.ds.NextSeqno(),
			Topic:     &topic,
			Signature: netMes.ID().Bytes(),
		},
	}

	return netMes.directMessageHandler(msg, netMes.ID())
}

func (netMes *networkMessenger) hasTopicProcessor(topic string) bool {
	netMes.mutTopics.RLock()
	defer netMes.mutTopics.RUnlock()

	return netMes.processors[topic] != nil
}

func (netMes *networkMessenger) directMessageHandler(message *pubsub.Message, fromConnectedPeer core.PeerID) (err error) {
	var processor p2p.MessageProcessor

	// Guard before dereferencing: callers validate this today, but the deref must not panic
	// outside the recover below, or it would crash the node it is meant to protect.
	if message == nil || message.Topic == nil {
		return p2p.ErrNilMessage
	}
	topic := *message.Topic

	// Recover so a panic on untrusted data cannot crash the node: this now runs synchronously
	// in the stream reader goroutine, which has no recover of its own (GHSA-rm5c-5x2p-48wr).
	defer func() {
		if r := recover(); r != nil {
			netMes.recoverFromMessagePanic(r, topic, fromConnectedPeer, message)
			dataLen := 0
			if message != nil {
				dataLen = len(message.GetData())
			}
			netMes.processDebugMessage(topic, fromConnectedPeer, uint64(dataLen), true)
			err = nil
		}
	}()

	msg, err := netMes.transformAndCheckMessage(message, fromConnectedPeer, topic)
	if err != nil {
		return err
	}

	netMes.mutTopics.RLock()
	processor = netMes.processors[topic]
	netMes.mutTopics.RUnlock()

	if processor == nil {
		return fmt.Errorf("%w on directMessageHandler for topic %s", p2p.ErrNilValidator, topic)
	}

	if check.IfNil(msg) {
		return p2p.ErrNilMessage
	}

	// Process synchronously: the per-stream reader bounds concurrency to one in-flight per stream,
	// instead of one goroutine per message which a single peer could grow unbounded (GHSA-hf2g-6j7h-98wg).
	// Trade-off: a slow processor head-of-line-blocks its own stream, so direct-send processors
	// must stay non-blocking.
	//
	//we won't recheck the message id against the cacher here as there might be collisions since we are using
	// a separate sequence counter for direct sender
	errProcess := processor.ProcessReceivedMessage(msg, fromConnectedPeer)
	if errProcess != nil {
		log.Trace("p2p validator directMessageHandler",
			"error", errProcess.Error(),
			"topic", msg.Topic(),
			"originator", p2p.MessageOriginatorPid(msg),
			"from connected peer", p2p.PeerIDToShortString(fromConnectedPeer),
			"seq no", p2p.MessageOriginatorSeq(msg),
		)
	}
	netMes.debugger.AddIncomingMessage(msg.Topic(), uint64(len(msg.Data())), errProcess != nil)

	return nil
}

// IsConnectedToTheNetwork returns true if the current node is connected to the network
func (netMes *networkMessenger) IsConnectedToTheNetwork() bool {
	netw := netMes.p2pHost.Network()
	return netMes.connMonitor.IsConnectedToTheNetwork(netw)
}

// SetThresholdMinConnectedPeers sets the minimum connected peers before triggering a new reconnection
func (netMes *networkMessenger) SetThresholdMinConnectedPeers(minConnectedPeers int) error {
	if minConnectedPeers < 0 {
		return p2p.ErrInvalidValue
	}

	netw := netMes.p2pHost.Network()
	netMes.connMonitor.SetThresholdMinConnectedPeers(minConnectedPeers, netw)

	return nil
}

// ThresholdMinConnectedPeers returns the minimum connected peers before triggering a new reconnection
func (netMes *networkMessenger) ThresholdMinConnectedPeers() int {
	return netMes.connMonitor.ThresholdMinConnectedPeers()
}

// SetPeerShardResolver sets the peer shard resolver component that is able to resolve the link
// between p2p.PeerID
func (netMes *networkMessenger) SetPeerShardResolver(peerShardResolver p2p.PeerShardResolver) error {
	if check.IfNil(peerShardResolver) {
		return p2p.ErrNilPeerShardResolver
	}

	err := netMes.sharder.SetPeerShardResolver(peerShardResolver)
	if err != nil {
		return err
	}

	netMes.mutPeerResolver.Lock()
	netMes.peerShardResolver = peerShardResolver
	netMes.mutPeerResolver.Unlock()

	// Until now every peer resolved as unknown, so the inbound cap could not tell validators apart.
	if netMes.peerProtector != nil {
		netMes.peerProtector.arm()
	}

	return nil
}

// getPeerShardResolver reads the resolver under mutPeerResolver: SetPeerShardResolver writes it
// while the peer protector loop and GetConnectedPeersInfo read it from other goroutines.
func (netMes *networkMessenger) getPeerShardResolver() p2p.PeerShardResolver {
	netMes.mutPeerResolver.RLock()
	defer netMes.mutPeerResolver.RUnlock()

	return netMes.peerShardResolver
}

// SetPeerDenialEvaluator sets the peer black list handler
func (netMes *networkMessenger) SetPeerDenialEvaluator(handler p2p.PeerDenialEvaluator) error {
	return netMes.connMonitorWrapper.SetPeerDenialEvaluator(handler)
}

// GetConnectedPeersInfo gets the current connected peers information
func (netMes *networkMessenger) GetConnectedPeersInfo() *p2p.ConnectedPeersInfo {
	peers := netMes.p2pHost.Network().Peers()
	connPeerInfo := &p2p.ConnectedPeersInfo{
		UnknownPeers:         make([]string, 0),
		IntraShardValidators: make(map[uint32][]string),
		IntraShardObservers:  make(map[uint32][]string),
		CrossShardValidators: make(map[uint32][]string),
		CrossShardObservers:  make(map[uint32][]string),
		NumObserversOnShard:  make(map[uint32]int),
		NumValidatorsOnShard: make(map[uint32]int),
	}
	peerShardResolver := netMes.getPeerShardResolver()
	selfPeerInfo := peerShardResolver.GetPeerInfo(netMes.ID())
	connPeerInfo.SelfShardID = selfPeerInfo.ShardID

	for _, p := range peers {
		conns := netMes.p2pHost.Network().ConnsToPeer(p)
		connString := "[invalid connection string]"
		if len(conns) > 0 {
			connString = conns[0].RemoteMultiaddr().String() + "/p2p/" + p.String()
		}

		peerInfo := peerShardResolver.GetPeerInfo(core.PeerID(p))
		switch peerInfo.PeerType {
		case core.UnknownPeer:
			connPeerInfo.UnknownPeers = append(connPeerInfo.UnknownPeers, connString)
		case core.ValidatorPeer:
			connPeerInfo.NumValidatorsOnShard[peerInfo.ShardID]++
			if selfPeerInfo.ShardID != peerInfo.ShardID {
				connPeerInfo.CrossShardValidators[peerInfo.ShardID] = append(connPeerInfo.CrossShardValidators[peerInfo.ShardID], connString)
				connPeerInfo.NumCrossShardValidators++
			} else {
				connPeerInfo.IntraShardValidators[peerInfo.ShardID] = append(connPeerInfo.IntraShardValidators[peerInfo.ShardID], connString)
				connPeerInfo.NumIntraShardValidators++
			}
		case core.ObserverPeer:
			connPeerInfo.NumObserversOnShard[peerInfo.ShardID]++
			if selfPeerInfo.ShardID != peerInfo.ShardID {
				connPeerInfo.CrossShardObservers[peerInfo.ShardID] = append(connPeerInfo.CrossShardObservers[peerInfo.ShardID], connString)
				connPeerInfo.NumCrossShardObservers++
			} else {
				connPeerInfo.IntraShardObservers[peerInfo.ShardID] = append(connPeerInfo.IntraShardObservers[peerInfo.ShardID], connString)
				connPeerInfo.NumIntraShardObservers++
			}
		}
	}

	return connPeerInfo
}

// IsInterfaceNil returns true if there is no value under the interface
func (netMes *networkMessenger) IsInterfaceNil() bool {
	return netMes == nil
}

const (
	// maxConnsPerSubnetLimit bounds the per-subnet connection caps. connLimitsPerSubnetV6 derives
	// the /48 aggregate as 8x the /56 value, so inputs are bounded well below MaxInt/8 to keep that
	// derivation from wrapping.
	maxConnsPerSubnetLimit = math.MaxInt32 / 8

	// maxConnRatePerSec and maxConnRateBurst bound the rate limiter inputs to values a real node
	// could act on; anything larger is a configuration mistake rather than an intentional limit.
	maxConnRatePerSec = 1e6
	maxConnRateBurst  = maxConnsPerSubnetLimit
)

// validateResourceManagerConfig checks the per-subnet / rate knobs. They apply only to the
// "scaled" strategy; setting them under any other strategy is a configuration error, not a silent
// no-op. Values must be non-negative and the rate pair must be set together. KLC-2433.
func validateResourceManagerConfig(rmCfg config.ResourceManagerConfig) error {
	hasSubnetKnobs := rmCfg.MaxConnsPerIPv4 != 0 ||
		rmCfg.MaxConnsPerIPv6Subnet != 0 ||
		rmCfg.ConnRatePerSec != 0 ||
		rmCfg.ConnRateBurst != 0

	if hasSubnetKnobs && rmCfg.Strategy != config.ResourceManagerStrategyScaled {
		return fmt.Errorf("p2p.resourceManager per-subnet limits require strategy=%q, got %q",
			config.ResourceManagerStrategyScaled, rmCfg.Strategy)
	}

	// connRatePerSec is a float64 straight out of yaml, so `.nan` and `.inf` are representable.
	// NaN fails every ordering comparison, so it would pass the non-negative check, defeat the
	// paired check (false != false), and then fail `> 0` at build time — attaching no rate limiter
	// at all. That is precisely the silent no-op this validator exists to prevent.
	if math.IsNaN(rmCfg.ConnRatePerSec) || math.IsInf(rmCfg.ConnRatePerSec, 0) {
		return fmt.Errorf("p2p.resourceManager.connRatePerSec must be a finite number, got %v",
			rmCfg.ConnRatePerSec)
	}

	if rmCfg.MaxConnsPerIPv4 < 0 || rmCfg.MaxConnsPerIPv6Subnet < 0 ||
		rmCfg.ConnRatePerSec < 0 || rmCfg.ConnRateBurst < 0 {
		return fmt.Errorf("p2p.resourceManager per-subnet limits must be non-negative")
	}

	// The /48 aggregate is derived as 8x the /56 cap, so an input past MaxInt/8 wraps negative and
	// the aggregate that actually matters against a provider-wide Sybil spread becomes a negative
	// (unbounded) cap. Bound the inputs rather than the derivation, so the error names the knob.
	if rmCfg.MaxConnsPerIPv4 > maxConnsPerSubnetLimit || rmCfg.MaxConnsPerIPv6Subnet > maxConnsPerSubnetLimit {
		return fmt.Errorf("p2p.resourceManager per-subnet limits must not exceed %d", maxConnsPerSubnetLimit)
	}

	if rmCfg.ConnRatePerSec > maxConnRatePerSec {
		return fmt.Errorf("p2p.resourceManager.connRatePerSec must not exceed %v", maxConnRatePerSec)
	}

	if rmCfg.ConnRateBurst > maxConnRateBurst {
		return fmt.Errorf("p2p.resourceManager.connRateBurst must not exceed %d", maxConnRateBurst)
	}

	if (rmCfg.ConnRatePerSec > 0) != (rmCfg.ConnRateBurst > 0) {
		return fmt.Errorf("p2p.resourceManager connRatePerSec and connRateBurst must be set together")
	}

	return nil
}

// scaledLimits builds the "scaled" strategy's limits the same way libp2p builds its default
// ResourceManager: it adds libp2p's bundled per-service and per-protocol limits (identify,
// ping, autonat, ...) before scaling. Scaling rcmgr.DefaultLimits directly would leave those
// protocols on the generic limits, e.g. 80 inbound ping streams per peer at 4 GiB instead of 2.
//
// On regular nodes the per-peer inbound connection limit is also lowered to
// regularNodePeerConnsInbound from libp2p's 8 (libp2p v0.48 does not scale per-peer connections
// with memory): an honest peer holds one connection to us, two or three during a reconnect or a
// simultaneous dial, while each extra inbound connection one identity holds is a slot of the 320
// an attacker gets for free. The per-peer total (inbound + outbound, 8) and outbound limits keep
// libp2p's values, so our own dials are unaffected. The increase is zeroed too, so a future libp2p
// that scales it does not silently raise the limit. Seeds keep libp2p's per-peer limits.
func scaledLimits(memBytes int64, numFD int, isSeedNode bool) rcmgr.ConcreteLimitConfig {
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	if !isSeedNode {
		limits.PeerBaseLimit.ConnsInbound = regularNodePeerConnsInbound
		limits.PeerLimitIncrease.ConnsInbound = 0
	}
	return limits.Scale(memBytes, numFD)
}

// regularNodePeerConnsInbound is the per-peer inbound connection limit of the "scaled" strategy on
// regular nodes (see scaledLimits).
const regularNodePeerConnsInbound = 4

// BasicConnMgr counts connections, not peers. It trims once the connection count reaches
// highWater: it closes the connections of the lowest-value unprotected peers that are past their
// grace period until lowWater unprotected, out-of-grace connections remain. Connections of
// protected peers (validators protected by the peer protector, and peers gossipsub and kad-dht
// protect themselves) are not counted against lowWater and are never closed, so lowWater is the
// connection budget for unprotected peers, and lowWater plus the protected peers' inbound
// connections must stay below the inbound ResourceManager limit: otherwise unprotected peers can
// hold every inbound slot without a trim ever closing them.
const (
	// defaultConnMgrHighWater is the connection count that triggers a trim.
	defaultConnMgrHighWater = 300
	// defaultConnMgrLowWater is the number of unprotected, out-of-grace connections a trim keeps:
	// 100 plus ~200 validator connections fits in the 320 inbound slots of the default "scaled"
	// ResourceManager (4096 MiB).
	defaultConnMgrLowWater = 100
	// maxConnMgrWatermark and maxConnMgrGracePeriodInSec bound the config to values a node could
	// act on; anything larger is a configuration mistake.
	maxConnMgrWatermark        = 100000
	maxConnMgrGracePeriodInSec = 3600
	// defaultMaxUnprotectedInboundConns caps the inbound connections held by peers that are
	// neither validators nor protected by any subsystem; it matches defaultConnMgrLowWater.
	defaultMaxUnprotectedInboundConns = 100
)

// maxUnprotectedInboundConns returns the effective cap the peer protector enforces on the inbound
// connections of unprotected peers, whatever their age. It closes the hole BasicConnMgr's grace
// period leaves: an attacker rotating peer identities keeps every connection younger than the
// grace period, so none is ever a trim candidate. It counts connections, as the ResourceManager
// does, because one identity can hold several. A zero config value selects the default.
func maxUnprotectedInboundConns(cmCfg config.ConnectionManagerConfig) int {
	if cmCfg.MaxUnprotectedInboundConns != 0 {
		return cmCfg.MaxUnprotectedInboundConns
	}

	return defaultMaxUnprotectedInboundConns
}

// defaultConnMgrGracePeriod is how long a new connection is immune to trimming. A var so tests
// can drop it to zero.
var defaultConnMgrGracePeriod = time.Minute

// connManagerSettings returns the effective BasicConnMgr watermarks and grace period: a zero
// config value selects the default.
func connManagerSettings(cmCfg config.ConnectionManagerConfig) (low int, high int, grace time.Duration) {
	low, high, grace = defaultConnMgrLowWater, defaultConnMgrHighWater, defaultConnMgrGracePeriod
	if cmCfg.LowWater != 0 {
		low = cmCfg.LowWater
	}
	if cmCfg.HighWater != 0 {
		high = cmCfg.HighWater
	}
	if cmCfg.GracePeriodInSec != 0 {
		grace = time.Duration(cmCfg.GracePeriodInSec) * time.Second
	}

	return low, high, grace
}

func validateConnectionManagerConfig(cmCfg config.ConnectionManagerConfig) error {
	if cmCfg.LowWater < 0 {
		return fmt.Errorf("p2p.connectionManager.lowWater must be non-negative, got %d", cmCfg.LowWater)
	}

	if cmCfg.HighWater < 0 {
		return fmt.Errorf("p2p.connectionManager.highWater must be non-negative, got %d", cmCfg.HighWater)
	}

	if cmCfg.GracePeriodInSec < 0 {
		return fmt.Errorf("p2p.connectionManager.gracePeriodInSec must be non-negative, got %d",
			cmCfg.GracePeriodInSec)
	}

	if cmCfg.LowWater > maxConnMgrWatermark || cmCfg.HighWater > maxConnMgrWatermark {
		return fmt.Errorf("p2p.connectionManager.lowWater and highWater must not exceed %d, got %d and %d",
			maxConnMgrWatermark, cmCfg.LowWater, cmCfg.HighWater)
	}

	if cmCfg.MaxUnprotectedInboundConns < 0 {
		return fmt.Errorf("p2p.connectionManager.maxUnprotectedInboundConns must be non-negative, got %d",
			cmCfg.MaxUnprotectedInboundConns)
	}

	if cmCfg.MaxUnprotectedInboundConns > maxConnMgrWatermark {
		return fmt.Errorf("p2p.connectionManager.maxUnprotectedInboundConns must not exceed %d, got %d",
			maxConnMgrWatermark, cmCfg.MaxUnprotectedInboundConns)
	}

	if cmCfg.GracePeriodInSec > maxConnMgrGracePeriodInSec {
		return fmt.Errorf("p2p.connectionManager.gracePeriodInSec must not exceed %d, got %d",
			maxConnMgrGracePeriodInSec, cmCfg.GracePeriodInSec)
	}

	// Checked after the defaults are applied, so setting only one watermark cannot cross the
	// other's default (lowWater 400 alone would otherwise run 400/300).
	low, high, _ := connManagerSettings(cmCfg)
	if low >= high {
		return fmt.Errorf("p2p.connectionManager.lowWater (%d) must be lower than highWater (%d); "+
			"a zero value means the default (lowWater %d, highWater %d)",
			low, high, defaultConnMgrLowWater, defaultConnMgrHighWater)
	}

	return nil
}

// newConnManager returns the connection manager handed to libp2p. Seeds get NullConnMgr: they
// exist to be connected to and must not trim the peers they serve; their peer count is bounded by
// the ResourceManager, which refuses new connections rather than closing existing ones. Every
// other node, whatever its sharder type, gets a BasicConnMgr built from p2p.connectionManager.
// Without trimming, a few dozen IPv4 addresses (libp2p allows 8 connections per /32) could hold
// every inbound ResourceManager slot indefinitely; with it, once the high watermark (in
// connections) is reached, the connections of the lowest-value unprotected peers are closed down
// to the low watermark. Which unprotected peers go first follows their connection-manager value,
// which gossipsub's decaying pubsub-deliveries tags dominate (a peer that delivers messages first
// is worth more); the observer tag adds little. Validators are never trimmed: the peer protector
// (see createPeerProtector) protects them, and gossipsub and kad-dht protect their own mesh and
// routing peers through the same manager. Trimming spares connections inside the grace period, so the protector
// also caps the inbound connections of unprotected peers on its own (see
// maxUnprotectedInboundConns), at admission through inboundCapGater and every pass, ordering by
// peer class rather than value. On success the
// host owns the manager and closes it in host.Close.
func newConnManager(args ArgsNetworkMessenger) (connmgr.ConnManager, error) {
	if args.IsSeedNode {
		return connmgr.NullConnMgr{}, nil
	}

	low, high, grace := connManagerSettings(args.P2pConfig.ConnectionManager)
	cm, err := p2pConnMgr.NewConnManager(low, high, p2pConnMgr.WithGracePeriod(grace))
	if err != nil {
		return nil, fmt.Errorf("creating connection manager: %w", err)
	}

	return cm, nil
}

func validateDirectSendConfig(dsCfg config.DirectSendConfig) error {
	if dsCfg.MaxInboundStreamsPerPeer < 0 {
		return fmt.Errorf("p2p.directSend.maxInboundStreamsPerPeer must be non-negative, got %d",
			dsCfg.MaxInboundStreamsPerPeer)
	}

	if dsCfg.MaxInboundStreamsTotal < 0 {
		return fmt.Errorf("p2p.directSend.maxInboundStreamsTotal must be non-negative, got %d",
			dsCfg.MaxInboundStreamsTotal)
	}

	if dsCfg.MaxSeenMessages < 0 {
		return fmt.Errorf("p2p.directSend.maxSeenMessages must be non-negative, got %d",
			dsCfg.MaxSeenMessages)
	}

	// Every tracked peer holds a fixed share of seenMessagesPerPeer entries, so a total below one
	// share still allocates a full share: 1 would be a 64-entry cache. Reject it rather than
	// silently exceed what the operator wrote.
	if dsCfg.MaxSeenMessages > 0 && dsCfg.MaxSeenMessages < seenMessagesPerPeer {
		return fmt.Errorf("p2p.directSend.maxSeenMessages must be 0 (default) or at least %d, got %d",
			seenMessagesPerPeer, dsCfg.MaxSeenMessages)
	}

	return nil
}

// defaultRegularNodeScaledMemoryMiB is the memory budget of the "scaled" ResourceManager a regular
// node gets when its config has no resourceManager section: 64 + 4096/16 = 320 inbound and 640
// total connections, the values config/node/config.yaml ships.
const defaultRegularNodeScaledMemoryMiB = 4096

// effectiveResourceManagerConfig applies the in-code default for regular nodes. Upgraded nodes keep
// their old config.yaml, which may have no resourceManager section (strategy ""); libp2p's
// auto-scaled limits would then be 128 inbound on an 8 GB host, too few for the validator set, so
// a regular node with strategy "" gets "scaled" with 4096 MiB instead. Seeds keep libp2p's
// auto-scale for "", and an explicit "default" always means libp2p's auto-scale.
func effectiveResourceManagerConfig(rmCfg config.ResourceManagerConfig, isSeedNode bool) config.ResourceManagerConfig {
	if isSeedNode || rmCfg.Strategy != config.ResourceManagerStrategyDefault {
		return rmCfg
	}

	rmCfg.Strategy = config.ResourceManagerStrategyScaled
	rmCfg.ScaledMemoryMiB = defaultRegularNodeScaledMemoryMiB

	return rmCfg
}

// resourceManagerSetup is what buildResourceManager decided.
type resourceManagerSetup struct {
	option libp2p.Option
	closer io.Closer
	// strategy is the effective strategy, after effectiveResourceManagerConfig.
	strategy string
	// limits is set only for the "scaled" strategy.
	limits *rcmgr.ConcreteLimitConfig
}

// buildResourceManager validates the configured resourceManager section as written (so the
// per-subnet knobs still require an explicit strategy "scaled") and then builds the option for
// the effective configuration.
func buildResourceManager(args ArgsNetworkMessenger) (resourceManagerSetup, error) {
	if err := validateResourceManagerConfig(args.P2pConfig.ResourceManager); err != nil {
		return resourceManagerSetup{}, err
	}

	rmCfg := effectiveResourceManagerConfig(args.P2pConfig.ResourceManager, args.IsSeedNode)
	opt, closer, limits, err := buildResourceManagerOption(rmCfg, args.IsSeedNode)
	if err != nil {
		return resourceManagerSetup{}, err
	}

	return resourceManagerSetup{
		option:   opt,
		closer:   closer,
		strategy: rmCfg.Strategy,
		limits:   limits,
	}, nil
}

// systemConnLimits reads the inbound and total connection limits of the ResourceManager's system
// scope. ok is false when the manager exposes no limits (strategy "null").
func systemConnLimits(rm network.ResourceManager) (connsInbound int, conns int, ok bool) {
	if rm == nil {
		return 0, 0, false
	}
	_ = rm.ViewSystem(func(scope network.ResourceScope) error {
		limiter, isLimiter := scope.(rcmgr.ResourceScopeLimiter)
		if !isLimiter {
			return nil
		}
		limit := limiter.Limit()
		connsInbound, conns, ok = limit.GetConnLimit(network.DirInbound), limit.GetConnTotalLimit(), true
		return nil
	})

	return connsInbound, conns, ok
}

// connManagerLimitWarnings returns what is wrong with the watermarks and the unprotected inbound
// cap for the given system limits. highWater equal to the total limit is still reachable, so only a
// higher one is reported. A lowWater or cap at or above the inbound limit lets unprotected peers
// alone fill every inbound slot.
func connManagerLimitWarnings(low int, high int, maxUnprotectedInboundConns int, connsInbound int, conns int) []string {
	var warnings []string
	if high > conns {
		warnings = append(warnings, fmt.Sprintf(
			"p2p.connectionManager.highWater (%d) is above the ResourceManager's total connection limit (%d): "+
				"the connection count can never reach it, so the connection manager never trims", high, conns))
	}
	if low >= connsInbound {
		warnings = append(warnings, fmt.Sprintf(
			"p2p.connectionManager.lowWater (%d) is not below the ResourceManager's inbound connection limit (%d): "+
				"unprotected peers can hold every inbound slot and a trim leaves them there", low, connsInbound))
	}
	if maxUnprotectedInboundConns >= connsInbound {
		warnings = append(warnings, fmt.Sprintf(
			"p2p.connectionManager.maxUnprotectedInboundConns (%d) is not below the ResourceManager's inbound "+
				"connection limit (%d): unprotected peers can hold every inbound slot", maxUnprotectedInboundConns, connsInbound))
	}

	return warnings
}

// logNetworkLimits logs, once at startup, the connection limits the host runs with, read back from
// its ResourceManager, and warns when the connection manager's watermarks do not fit them.
func logNetworkLimits(args ArgsNetworkMessenger, rmStrategy string, rm network.ResourceManager) {
	strategy := rmStrategy
	if strategy == config.ResourceManagerStrategyDefault {
		strategy = config.ResourceManagerStrategyLibp2pDefault + " (libp2p auto-scaled)"
	}

	connsInbound, conns, hasLimits := systemConnLimits(rm)
	connsInboundStr, connsStr := "unlimited", "unlimited"
	if hasLimits {
		connsInboundStr, connsStr = fmt.Sprintf("%d", connsInbound), fmt.Sprintf("%d", conns)
	}

	connManager := "none (seed)"
	inboundForProtected := "n/a"
	if !args.IsSeedNode {
		low, high, grace := connManagerSettings(args.P2pConfig.ConnectionManager)
		maxUnprotected := maxUnprotectedInboundConns(args.P2pConfig.ConnectionManager)
		connManager = fmt.Sprintf("low=%d high=%d grace=%s maxUnprotectedInboundConns=%d", low, high, grace, maxUnprotected)
		if hasLimits {
			inboundForProtected = fmt.Sprintf("%d", connsInbound-low)
			for _, warning := range connManagerLimitWarnings(low, high, maxUnprotected, connsInbound, conns) {
				log.Warn(warning)
			}
		}
	}

	log.Info("p2p connection limits",
		"resource manager strategy", strategy,
		"system conns inbound", connsInboundStr,
		"system conns", connsStr,
		"connection manager", connManager,
		"inbound slots left for protected peers", inboundForProtected,
	)
}

// buildResourceManagerOption returns the libp2p ResourceManager option dictated
// by rmCfg. The closer is non-nil only when a fresh rcmgr was constructed
// (currently only the "scaled" strategy); callers must Close() it if
// libp2p.New() fails, since the host can no longer take ownership.
// A nil option means "let libp2p use its DefaultResourceManager". The limits are
// returned only for the "scaled" strategy.
func buildResourceManagerOption(rmCfg config.ResourceManagerConfig, isSeedNode bool) (libp2p.Option, io.Closer, *rcmgr.ConcreteLimitConfig, error) {
	if err := validateResourceManagerConfig(rmCfg); err != nil {
		return nil, nil, nil, err
	}

	switch rmCfg.Strategy {
	case config.ResourceManagerStrategyDefault, config.ResourceManagerStrategyLibp2pDefault:
		return nil, nil, nil, nil

	case config.ResourceManagerStrategyNull:
		return libp2p.ResourceManager(&network.NullResourceManager{}), nil, nil, nil

	case config.ResourceManagerStrategyScaled:
		if rmCfg.ScaledMemoryMiB <= 0 {
			return nil, nil, nil, fmt.Errorf("p2p.resourceManager.strategy=%q requires scaledMemoryMiB > 0",
				rmCfg.Strategy)
		}
		memBytes := clampToPhysicalMemory(int64(rmCfg.ScaledMemoryMiB) << 20)
		limits := scaledLimits(memBytes, getFDLimit()/2, isSeedNode)
		opts := scaledResourceManagerOptions(rmCfg)
		rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits), opts...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("creating scaled resource manager: %w", err)
		}
		return libp2p.ResourceManager(rm), rm, &limits, nil

	default:
		return nil, nil, nil, fmt.Errorf("invalid p2p.resourceManager.strategy %q (valid: %q, %q, %q, %q)",
			rmCfg.Strategy,
			config.ResourceManagerStrategyDefault,
			config.ResourceManagerStrategyLibp2pDefault,
			config.ResourceManagerStrategyNull,
			config.ResourceManagerStrategyScaled,
		)
	}
}

// clampToPhysicalMemory bounds the value handed to rcmgr's Scale by the host's actual RAM. Scale
// treats its argument as the memory available to size limits from, so a shipped default larger than
// the host — a 16 GiB default on a 4 GiB seed, say — produces limits four times too permissive, and
// the resource manager stops protecting exactly when it should. Clamping keeps an oversized config
// safe; the warning makes the mismatch visible rather than silently tolerated. The bound is
// host-level only: detection reads the host's total RAM, so a cgroup memory limit below it (a
// containerised seed) is invisible here and will not clamp.
func clampToPhysicalMemory(configured int64) int64 {
	physical, err := detectPhysicalMemoryBytes()
	if err != nil || physical <= 0 {
		log.Debug("cannot detect physical memory, using configured scaledMemoryMiB as-is",
			"configuredMiB", configured>>20)
		return configured
	}

	if configured <= physical {
		return configured
	}

	log.Warn("p2p.resourceManager.scaledMemoryMiB exceeds detected physical memory, clamping",
		"configuredMiB", configured>>20, "detectedMiB", physical>>20)

	return physical
}

func detectPhysicalMemoryBytes() (int64, error) {
	vms, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	if vms.Total > math.MaxInt64 {
		return math.MaxInt64, nil
	}

	return int64(vms.Total), nil
}

// connLimitsPerSubnetV4 returns the per-/32 connection cap, or nil to keep libp2p's default.
func connLimitsPerSubnetV4(maxConns int) []rcmgr.ConnLimitPerSubnet {
	if maxConns <= 0 {
		return nil
	}
	return []rcmgr.ConnLimitPerSubnet{{PrefixLength: 32, ConnCount: maxConns}}
}

// connLimitsPerSubnetV6 returns per-/56 and per-/48 caps (the /48 aggregate is 8x the /56, mirroring
// libp2p's default ratio), or nil to keep libp2p's default.
func connLimitsPerSubnetV6(maxConns int) []rcmgr.ConnLimitPerSubnet {
	if maxConns <= 0 {
		return nil
	}
	return []rcmgr.ConnLimitPerSubnet{
		{PrefixLength: 56, ConnCount: maxConns},
		{PrefixLength: 48, ConnCount: maxConns * 8},
	}
}

// buildConnRateLimiter builds a per-subnet new-connection rate limiter that preserves libp2p's
// loopback exemption (localhost is never rate limited). KLC-2433.
func buildConnRateLimiter(rps float64, burst int) *rate.Limiter {
	subnetLimit := rate.Limit{RPS: rps, Burst: burst}
	return &rate.Limiter{
		NetworkPrefixLimits: []rate.PrefixLimit{
			{Prefix: netip.MustParsePrefix("127.0.0.0/8"), Limit: rate.Limit{}},
			{Prefix: netip.MustParsePrefix("::1/128"), Limit: rate.Limit{}},
		},
		SubnetRateLimiter: rate.SubnetLimiter{
			IPv4SubnetLimits: []rate.SubnetLimit{{PrefixLength: 32, Limit: subnetLimit}},
			IPv6SubnetLimits: []rate.SubnetLimit{
				{PrefixLength: 56, Limit: subnetLimit},
				{PrefixLength: 48, Limit: rate.Limit{RPS: rps * 8, Burst: burst * 8}},
			},
			GracePeriod: time.Minute,
		},
	}
}

// scaledResourceManagerOptions translates the per-subnet config knobs into rcmgr options. nil
// slice means "use libp2p's built-in defaults". KLC-2433.
func scaledResourceManagerOptions(rmCfg config.ResourceManagerConfig) []rcmgr.Option {
	var opts []rcmgr.Option

	v4 := connLimitsPerSubnetV4(rmCfg.MaxConnsPerIPv4)
	v6 := connLimitsPerSubnetV6(rmCfg.MaxConnsPerIPv6Subnet)
	if v4 != nil || v6 != nil {
		opts = append(opts, rcmgr.WithLimitPerSubnet(v4, v6))
	}

	if rmCfg.ConnRatePerSec > 0 {
		opts = append(opts, rcmgr.WithConnRateLimiters(
			buildConnRateLimiter(rmCfg.ConnRatePerSec, rmCfg.ConnRateBurst)))
	}

	return opts
}

// getFDLimit returns the soft RLIMIT_NOFILE — the effective per-process FD cap
// the operator chose via ulimit. Returns 0 on error; rcmgr then falls back to
// its base FD limits.
func getFDLimit() int {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return 0
	}
	if rLimit.Cur > math.MaxInt {
		return math.MaxInt
	}
	return int(rLimit.Cur)
}
