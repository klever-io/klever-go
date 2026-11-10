package discovery

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	manet "github.com/multiformats/go-multiaddr/net"
)

var _ p2p.PeerDiscoverer = (*optimizedKadDhtDiscoverer)(nil)
var _ p2p.Reconnecter = (*optimizedKadDhtDiscoverer)(nil)

const optimizedKadDhtName = "optimized kad-dht discovery"
const minIntervalForSeedersReconnection = time.Second

// seederProtectionTag marks the connection-manager protection held on the counted seeders
const seederProtectionTag = "seeder"

// KadDhtHandler is the part of the kad-dht implementation the optimized discoverer drives
type KadDhtHandler interface {
	Bootstrap(ctx context.Context) error
	Close() error
}

// optimizedKadDhtDiscoverer connects to up to maxSeeders seeders (peer IDs) of the initial peers list on start
// and keeps topping them up: every SeedersReconnectionInterval while at least one seeder is
// connected, every PeersRefreshInterval while none is. DHT refreshes, seeder reconnects and
// reconnect requests all run on one process loop.
type optimizedKadDhtDiscoverer struct {
	ctx                         context.Context
	host                        ConnectableHost
	hostConnManagement          *hostWithConnectionManagement
	seeders                     []seeder
	maxSeeders                  int
	peersRefreshInterval        time.Duration
	seedersReconnectionInterval time.Duration
	protocolID                  string
	routingTableRefresh         time.Duration

	// owned by the process loop
	kadDHT          KadDhtHandler
	initialized     bool
	seederElsewhere []bool

	// written by the process loop, read by tests through export_test.go
	numSeederCycles       atomic.Uint64
	lastNumCountedSeeders atomic.Int64

	chanInit             chan struct{}
	errChanInit          chan error
	chanConnectToSeeders chan struct{}
	chanLoopDone         chan struct{}
	createKadDhtHandler  func(ctx context.Context) (KadDhtHandler, error)
}

// NewOptimizedKadDhtDiscoverer creates the optimized kad-dht discovery implementation and starts
// its process loop, which stops when arg.Context is done.
// initialPeersList can be nil or empty, no initial connection will be attempted, a warning message will appear
func NewOptimizedKadDhtDiscoverer(arg ArgKadDht) (*optimizedKadDhtDiscoverer, error) {
	okdd, err := newOptimizedKadDhtDiscoverer(arg)
	if err != nil {
		return nil, err
	}

	go okdd.processLoop(arg.Context)

	return okdd, nil
}

func newOptimizedKadDhtDiscoverer(arg ArgKadDht) (*optimizedKadDhtDiscoverer, error) {
	if check.IfNilReflect(arg.Context) {
		return nil, p2p.ErrNilContext
	}
	if check.IfNilReflect(arg.Host) {
		return nil, p2p.ErrNilHost
	}
	if check.IfNil(arg.KddSharder) {
		return nil, p2p.ErrNilSharder
	}
	sharder, ok := arg.KddSharder.(Sharder)
	if !ok {
		return nil, fmt.Errorf("%w for sharder: expected discovery.Sharder type of interface", p2p.ErrWrongTypeAssertion)
	}
	if arg.PeersRefreshInterval < time.Second {
		return nil, fmt.Errorf("%w, PeersRefreshInterval should have been at least 1 second", p2p.ErrInvalidValue)
	}
	if arg.RoutingTableRefresh < time.Second {
		return nil, fmt.Errorf("%w, RoutingTableRefresh should have been at least 1 second", p2p.ErrInvalidValue)
	}
	if arg.SeedersReconnectionInterval < minIntervalForSeedersReconnection {
		return nil, fmt.Errorf("%w, SeedersReconnectionInterval should have been at least 1 second", p2p.ErrInvalidValue)
	}
	seeders, err := parseSeeders(arg.InitialPeersList, arg.Host.ID())
	if err != nil {
		return nil, err
	}
	if len(seeders) == 0 {
		log.Warn("nil or empty initial peers list provided to kad dht implementation. " +
			"No initial connection will be done")
	}

	hostConnManagement, err := NewHostWithConnectionManagement(arg.Host, sharder)
	if err != nil {
		return nil, err
	}

	maxSeeders := len(seeders)
	if arg.MaxSeeders > 0 && int(arg.MaxSeeders) < maxSeeders {
		maxSeeders = int(arg.MaxSeeders)
	}

	okdd := &optimizedKadDhtDiscoverer{
		ctx:                         arg.Context,
		host:                        arg.Host,
		hostConnManagement:          hostConnManagement,
		seeders:                     seeders,
		maxSeeders:                  maxSeeders,
		peersRefreshInterval:        arg.PeersRefreshInterval,
		seedersReconnectionInterval: arg.SeedersReconnectionInterval,
		protocolID:                  arg.ProtocolID,
		routingTableRefresh:         arg.RoutingTableRefresh,
		chanInit:                    make(chan struct{}),
		errChanInit:                 make(chan error),
		chanConnectToSeeders:        make(chan struct{}, 1),
		chanLoopDone:                make(chan struct{}),
		seederElsewhere:             make([]bool, len(seeders)),
	}
	// -1: no top-up has run yet, so the first one that counts no seeder is logged
	okdd.lastNumCountedSeeders.Store(-1)
	okdd.createKadDhtHandler = okdd.createKadDht

	return okdd, nil
}

// Bootstrap will start the bootstrapping new peers process
func (okdd *optimizedKadDhtDiscoverer) Bootstrap() error {
	select {
	case okdd.chanInit <- struct{}{}:
	case <-okdd.chanLoopDone:
		return okdd.ctx.Err()
	}

	return <-okdd.errChanInit
}

func (okdd *optimizedKadDhtDiscoverer) processLoop(ctx context.Context) {
	defer close(okdd.chanLoopDone)

	chTimeSeedersReconnect := time.After(okdd.seedersReconnectionInterval)
	chTimeFindPeers := time.After(okdd.peersRefreshInterval)

	for {
		select {
		case <-okdd.chanInit:
			ch := okdd.processInit(ctx)
			if ch != nil {
				chTimeSeedersReconnect = ch
			}

		case <-chTimeSeedersReconnect:
			chTimeSeedersReconnect = okdd.processSeedersReconnect(ctx)

		case <-okdd.chanConnectToSeeders:
			chTimeSeedersReconnect = okdd.processSeedersReconnect(ctx)

		case <-chTimeFindPeers:
			okdd.findPeers(ctx)
			chTimeFindPeers = time.After(okdd.peersRefreshInterval)

		case <-ctx.Done():
			log.Debug("closing the p2p bootstrapping process")
			okdd.closeKadDht()
			return
		}
	}
}

// processInit returns the next seeders reconnect timer, or nil to keep the running one when the
// init failed (e.g. a second Bootstrap call)
func (okdd *optimizedKadDhtDiscoverer) processInit(ctx context.Context) <-chan time.Time {
	err := okdd.init(ctx)
	okdd.errChanInit <- err
	if err != nil {
		return nil
	}

	ch := okdd.processSeedersReconnect(ctx)
	okdd.findPeers(ctx)

	return ch
}

func (okdd *optimizedKadDhtDiscoverer) processSeedersReconnect(ctx context.Context) <-chan time.Time {
	isConnectedToSeeders := okdd.connectToSeeders(ctx)
	return okdd.createChTimeSeedersReconnect(isConnectedToSeeders)
}

func (okdd *optimizedKadDhtDiscoverer) createChTimeSeedersReconnect(isConnectedToSeeders bool) <-chan time.Time {
	if isConnectedToSeeders {
		return time.After(okdd.seedersReconnectionInterval)
	}

	// no connection to any seeder, try again sooner
	return time.After(okdd.peersRefreshInterval)
}

func (okdd *optimizedKadDhtDiscoverer) init(ctx context.Context) error {
	if okdd.initialized {
		return p2p.ErrPeerDiscoveryProcessAlreadyStarted
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	kadDhtHandler, err := okdd.createKadDhtHandler(ctx)
	if err != nil {
		return err
	}

	okdd.kadDHT = kadDhtHandler
	okdd.initialized = true

	return nil
}

func (okdd *optimizedKadDhtDiscoverer) createKadDht(ctx context.Context) (KadDhtHandler, error) {
	return dht.New(
		ctx,
		okdd.hostConnManagement,
		dht.ProtocolPrefix(protocol.ID(okdd.protocolID)),
		dht.RoutingTableRefreshPeriod(okdd.routingTableRefresh),
		dht.Mode(dht.ModeServer),
	)
}

func (okdd *optimizedKadDhtDiscoverer) closeKadDht() {
	if okdd.kadDHT == nil {
		return
	}

	err := okdd.kadDHT.Close()
	if err != nil {
		log.Debug("closing kad dht", "error", err)
	}
}

// seederStatus is the outcome of one attempt to reach a seeder
type seederStatus int

const (
	// seederConnected: a connection under the seeder's peer ID comes from a configured IP
	seederConnected seederStatus = iota
	// seederElsewhere: connected under the seeder's peer ID, but only from other IPs
	seederElsewhere
	// seederUnreachable: not connected, and the dial failed
	seederUnreachable
)

// connectToSeeders tops the connected seeders up to maxSeeders: it counts the seeders already
// connected at a configured address, then dials the others in random order, in waves of as many
// seeders as there are free slots, until the count reaches maxSeeders. The counted seeders are
// protected from connection-manager trimming and every other seeder is unprotected, so a seeder
// this node reached incidentally (e.g. through a DHT lookup) neither counts towards nor survives
// past the cap. It returns true if at least one seeder is counted (or the list is empty).
func (okdd *optimizedKadDhtDiscoverer) connectToSeeders(ctx context.Context) bool {
	if !okdd.initialized {
		return false
	}
	if len(okdd.seeders) == 0 {
		return true
	}

	counted := make([]bool, len(okdd.seeders))
	numCounted := okdd.countConnectedSeeders(counted)
	numCounted += okdd.dialSeeders(ctx, counted, numCounted)
	okdd.protectCountedSeeders(counted)
	okdd.logSeedersCount(numCounted)
	okdd.numSeederCycles.Add(1)

	return numCounted > 0
}

// countConnectedSeeders marks, up to maxSeeders, the seeders connected at a configured address.
// It counts the ones that already hold the protection first, so a seeder reached incidentally does
// not take the slot of one this node is keeping.
func (okdd *optimizedKadDhtDiscoverer) countConnectedSeeders(counted []bool) int {
	connManager := okdd.host.ConnManager()
	numCounted := 0
	for _, wantProtected := range []bool{true, false} {
		for i, s := range okdd.seeders {
			if numCounted >= okdd.maxSeeders {
				return numCounted
			}
			isCandidate := !counted[i] && connManager.IsProtected(s.addrInfo.ID, seederProtectionTag) == wantProtected
			if isCandidate && okdd.isConnectedToSeeder(s) {
				counted[i] = true
				numCounted++
				okdd.seederElsewhere[i] = false
			}
		}
	}

	return numCounted
}

// dialSeeders dials the uncounted seeders in random order until numCounted reaches maxSeeders.
// Each wave dials, concurrently, as many seeders as there are free slots, so it can not overshoot
// the cap and an unreachable seeder holds the loop for one dial timeout per wave, not per seeder.
// It returns how many seeders it added to the count.
func (okdd *optimizedKadDhtDiscoverer) dialSeeders(ctx context.Context, counted []bool, numCounted int) int {
	candidates := make([]int, 0, len(okdd.seeders))
	for i := range okdd.seeders {
		if !counted[i] {
			candidates = append(candidates, i)
		}
	}
	// the dial order only spreads nodes across seeders; it has no security role
	rand.Shuffle(len(candidates), func(i, j int) { //nolint:gosec
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	added := 0
	for len(candidates) > 0 && numCounted+added < okdd.maxSeeders && ctx.Err() == nil {
		wave := candidates[:min(okdd.maxSeeders-numCounted-added, len(candidates))]
		candidates = candidates[len(wave):]

		for j, status := range okdd.dialWave(ctx, wave) {
			idx := wave[j]
			okdd.noteSeederElsewhere(idx, status == seederElsewhere)
			if status == seederConnected {
				counted[idx] = true
				added++
			}
		}
	}

	return added
}

func (okdd *optimizedKadDhtDiscoverer) dialWave(ctx context.Context, wave []int) []seederStatus {
	statuses := make([]seederStatus, len(wave))
	wg := sync.WaitGroup{}
	wg.Add(len(wave))
	for j, idx := range wave {
		go func(j int, s seeder) {
			defer wg.Done()
			statuses[j] = okdd.connectToSeeder(ctx, s)
		}(j, okdd.seeders[idx])
	}
	wg.Wait()

	return statuses
}

func (okdd *optimizedKadDhtDiscoverer) protectCountedSeeders(counted []bool) {
	connManager := okdd.host.ConnManager()
	for i, s := range okdd.seeders {
		if counted[i] {
			connManager.Protect(s.addrInfo.ID, seederProtectionTag)
			continue
		}
		connManager.Unprotect(s.addrInfo.ID, seederProtectionTag)
	}
}

// noteSeederElsewhere logs, once per episode, that a seeder's peer ID is connected only from
// addresses other than the configured ones
func (okdd *optimizedKadDhtDiscoverer) noteSeederElsewhere(idx int, isElsewhere bool) {
	if isElsewhere && !okdd.seederElsewhere[idx] {
		s := okdd.seeders[idx]
		log.Warn("seeder peer ID is connected from an address other than the configured ones; "+
			"the connection is kept but not counted",
			"seeder", s.addresses,
			"connected from", okdd.remoteAddresses(s))
	}
	okdd.seederElsewhere[idx] = isElsewhere
}

// logSeedersCount logs when the node loses its last counted seeder and when it gets one back
func (okdd *optimizedKadDhtDiscoverer) logSeedersCount(numCounted int) {
	previous := okdd.lastNumCountedSeeders.Swap(int64(numCounted))
	switch {
	case numCounted == 0 && previous != 0:
		log.Warn("not connected to any seeder of the initial peers list, retrying",
			"num seeders", len(okdd.seeders),
			"retry interval", okdd.peersRefreshInterval)
	case numCounted > 0 && previous == 0:
		log.Info("connected to seeders again", "connected", numCounted)
	}

	log.Debug("optimizedKadDhtDiscoverer.connectToSeeders",
		"num seeders", len(okdd.seeders),
		"max seeders", okdd.maxSeeders,
		"connected", numCounted)
}

// connectToSeeder runs on a dial goroutine: it only reads the discoverer's immutable fields
func (okdd *optimizedKadDhtDiscoverer) connectToSeeder(ctx context.Context, s seeder) seederStatus {
	if okdd.isConnectedToSeeder(s) {
		return seederConnected
	}
	if len(okdd.host.Network().ConnsToPeer(s.addrInfo.ID)) > 0 {
		// libp2p reuses that connection instead of dialling, so there is nothing to dial
		return seederElsewhere
	}

	// keep the configured addresses in the peerstore for as long as the node runs; Connect would
	// otherwise add them with a temporary TTL only
	okdd.host.Peerstore().AddAddrs(s.addrInfo.ID, s.addrInfo.Addrs, peerstore.PermanentAddrTTL)

	// the seeder is dialled through the host directly, not through the sharder-managed wrapper:
	// the sharder must not be able to refuse a connection to a configured seeder
	// a dial is bounded by the fast retry interval so an unreachable seeder does not hold up the loop
	dialCtx, cancel := context.WithTimeout(ctx, okdd.peersRefreshInterval)
	defer cancel()

	err := okdd.host.Connect(dialCtx, s.addrInfo)
	if err != nil {
		log.Debug("error connecting to seeder",
			"seeder", s.addresses,
			"error", err.Error(),
		)
		return seederUnreachable
	}
	if okdd.isConnectedToSeeder(s) {
		return seederConnected
	}

	return seederElsewhere
}

// isConnectedToSeeder returns true if there is a connection under the seeder's peer ID whose
// remote IP is a configured one. The port is not compared: a connection the seeder opened to
// this node carries the seeder's outgoing port, not its listening one.
func (okdd *optimizedKadDhtDiscoverer) isConnectedToSeeder(s seeder) bool {
	for _, conn := range okdd.host.Network().ConnsToPeer(s.addrInfo.ID) {
		ip, err := manet.ToIP(conn.RemoteMultiaddr())
		if err == nil && s.matchesIP(ip) {
			return true
		}
		if err != nil && s.matchesAnyIP {
			return true
		}
	}

	return false
}

func (okdd *optimizedKadDhtDiscoverer) remoteAddresses(s seeder) []string {
	conns := okdd.host.Network().ConnsToPeer(s.addrInfo.ID)
	remotes := make([]string, 0, len(conns))
	for _, conn := range conns {
		remotes = append(remotes, conn.RemoteMultiaddr().String())
	}

	return remotes
}

func (okdd *optimizedKadDhtDiscoverer) findPeers(ctx context.Context) {
	if !okdd.initialized {
		return
	}

	err := okdd.kadDHT.Bootstrap(ctx)
	if err != nil {
		log.Debug("kad dht bootstrap", "error", err)
	}
}

// Name returns the name of the kad dht peer discovery implementation
func (okdd *optimizedKadDhtDiscoverer) Name() string {
	return optimizedKadDhtName
}

// ReconnectToNetwork asks the process loop to dial the seeders it is not connected to and returns
// at once: the returned channel is already closed. A request made while one is pending is dropped.
func (okdd *optimizedKadDhtDiscoverer) ReconnectToNetwork() <-chan struct{} {
	select {
	case okdd.chanConnectToSeeders <- struct{}{}:
	default:
	}

	chDone := make(chan struct{})
	close(chDone)
	return chDone
}

// IsInterfaceNil returns true if there is no value under the interface
func (okdd *optimizedKadDhtDiscoverer) IsInterfaceNil() bool {
	return okdd == nil
}
