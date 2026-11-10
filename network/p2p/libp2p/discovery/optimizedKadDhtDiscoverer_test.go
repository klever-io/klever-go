package discovery_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPeerID(t *testing.T) peer.ID {
	t.Helper()

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(sk)
	require.NoError(t, err)

	return pid
}

// kadDhtStub counts Bootstrap and Close calls
type kadDhtStub struct {
	numBootstrap atomic.Int32
	numClose     atomic.Int32
}

func (kds *kadDhtStub) Bootstrap(_ context.Context) error {
	kds.numBootstrap.Add(1)
	return nil
}

func (kds *kadDhtStub) Close() error {
	kds.numClose.Add(1)
	return nil
}

// fakeSeederNetwork is a host whose connections are kept in memory: a successful Connect adds a
// connection whose remote address is the dialled one
type fakeSeederNetwork struct {
	mut         sync.Mutex
	conns       map[peer.ID][]multiaddr.Multiaddr
	unreachable map[peer.ID]bool
	blackholed  map[peer.ID]bool
	dials       map[peer.ID]int
	ttls        map[peer.ID]time.Duration
	protected   map[peer.ID]bool
	// dialLandsAt makes a successful Connect to the peer leave a connection at this remote address
	// instead of the dialled one (libp2p dials every peerstore address of the peer ID)
	dialLandsAt map[peer.ID]multiaddr.Multiaddr
	self        peer.ID
}

func newFakeSeederNetwork(self peer.ID) *fakeSeederNetwork {
	return &fakeSeederNetwork{
		conns:       make(map[peer.ID][]multiaddr.Multiaddr),
		unreachable: make(map[peer.ID]bool),
		blackholed:  make(map[peer.ID]bool),
		dials:       make(map[peer.ID]int),
		ttls:        make(map[peer.ID]time.Duration),
		protected:   make(map[peer.ID]bool),
		dialLandsAt: make(map[peer.ID]multiaddr.Multiaddr),
		self:        self,
	}
}

// protectRecorder records which peers hold the seeder protection
type protectRecorder struct {
	connmgr.NullConnMgr
	fsn *fakeSeederNetwork
}

func (pr *protectRecorder) Protect(id peer.ID, tag string) {
	if tag != "seeder" {
		return
	}

	pr.fsn.mut.Lock()
	pr.fsn.protected[id] = true
	pr.fsn.mut.Unlock()
}

func (pr *protectRecorder) Unprotect(id peer.ID, tag string) bool {
	if tag != "seeder" {
		return false
	}

	pr.fsn.mut.Lock()
	delete(pr.fsn.protected, id)
	pr.fsn.mut.Unlock()

	return false
}

func (pr *protectRecorder) IsProtected(id peer.ID, tag string) bool {
	if tag != "seeder" {
		return false
	}

	pr.fsn.mut.Lock()
	defer pr.fsn.mut.Unlock()

	return pr.fsn.protected[id]
}

// connectedAndProtected returns the seeders that are connected, and those that are protected
func (fsn *fakeSeederNetwork) connectedAndProtected(ids []peer.ID) ([]peer.ID, []peer.ID) {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	connected := make([]peer.ID, 0)
	protected := make([]peer.ID, 0)
	for _, pid := range ids {
		if len(fsn.conns[pid]) > 0 {
			connected = append(connected, pid)
		}
		if fsn.protected[pid] {
			protected = append(protected, pid)
		}
	}

	return connected, protected
}

func (fsn *fakeSeederNetwork) host() *mock.ConnectableHostStub {
	netw := &mock.NetworkStub{
		ConnsToPeerCalled: func(p peer.ID) []network.Conn {
			fsn.mut.Lock()
			defer fsn.mut.Unlock()

			conns := make([]network.Conn, 0, len(fsn.conns[p]))
			for _, remote := range fsn.conns[p] {
				remoteAddr := remote
				conns = append(conns, &mock.ConnStub{
					RemoteMultiaddrCalled: func() multiaddr.Multiaddr { return remoteAddr },
				})
			}
			return conns
		},
	}
	ps := &mock.PeerstoreStub{
		AddAddrsCalled: func(p peer.ID, _ []multiaddr.Multiaddr, ttl time.Duration) {
			fsn.mut.Lock()
			fsn.ttls[p] = ttl
			fsn.mut.Unlock()
		},
	}

	cm := &protectRecorder{fsn: fsn}

	return &mock.ConnectableHostStub{
		IDCalled:          func() peer.ID { return fsn.self },
		NetworkCalled:     func() network.Network { return netw },
		PeerstoreCalled:   func() peerstore.Peerstore { return ps },
		ConnManagerCalled: func() connmgr.ConnManager { return cm },
		ConnectCalled: func(ctx context.Context, pi peer.AddrInfo) error {
			fsn.mut.Lock()
			fsn.dials[pi.ID]++
			isBlackholed := fsn.blackholed[pi.ID]
			fsn.mut.Unlock()
			if isBlackholed {
				// packets dropped: the dial only ends when its context does
				<-ctx.Done()
				return ctx.Err()
			}

			fsn.mut.Lock()
			defer fsn.mut.Unlock()
			if fsn.unreachable[pi.ID] {
				return errors.New("unreachable")
			}
			remote := pi.Addrs[0]
			if landsAt, ok := fsn.dialLandsAt[pi.ID]; ok {
				remote = landsAt
			}
			fsn.conns[pi.ID] = append(fsn.conns[pi.ID], remote)
			return nil
		},
	}
}

func (fsn *fakeSeederNetwork) setConn(pid peer.ID, remote string) {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	fsn.conns[pid] = []multiaddr.Multiaddr{multiaddr.StringCast(remote)}
}

func (fsn *fakeSeederNetwork) drop(pid peer.ID) {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	delete(fsn.conns, pid)
}

func (fsn *fakeSeederNetwork) setUnreachable(pid peer.ID, unreachable bool) {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	fsn.unreachable[pid] = unreachable
}

func (fsn *fakeSeederNetwork) numDials(pid peer.ID) int {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	return fsn.dials[pid]
}

func (fsn *fakeSeederNetwork) isConnected(pid peer.ID) bool {
	fsn.mut.Lock()
	defer fsn.mut.Unlock()

	return len(fsn.conns[pid]) > 0
}

type testSeeders struct {
	ids       []peer.ID
	addresses []string
}

func newTestSeeders(t *testing.T, n int) testSeeders {
	ts := testSeeders{}
	for i := 0; i < n; i++ {
		pid := newTestPeerID(t)
		ts.ids = append(ts.ids, pid)
		ts.addresses = append(ts.addresses, fmt.Sprintf("/ip4/10.0.0.%d/tcp/37373/p2p/%s", i+1, pid))
	}

	return ts
}

func createOptimizedTestArgument(ctx context.Context, host discovery.ConnectableHost, initialPeers []string) discovery.ArgKadDht {
	return discovery.ArgKadDht{
		Context:                     ctx,
		Host:                        host,
		KddSharder:                  &mock.SharderStub{},
		PeersRefreshInterval:        time.Second,
		SeedersReconnectionInterval: time.Hour,
		ProtocolID:                  "/klv/test/0.0.0",
		InitialPeersList:            initialPeers,
		BucketSize:                  100,
		RoutingTableRefresh:         5 * time.Second,
	}
}

//------- NewOptimizedKadDhtDiscoverer

func TestNewOptimizedKadDhtDiscoverer_InvalidArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	self := newTestPeerID(t)
	seeders := newTestSeeders(t, 1)

	tests := []struct {
		name        string
		modify      func(arg *discovery.ArgKadDht)
		expectedErr error
	}{
		{"nil context", func(arg *discovery.ArgKadDht) { arg.Context = nil }, p2p.ErrNilContext},
		{"nil host", func(arg *discovery.ArgKadDht) { arg.Host = nil }, p2p.ErrNilHost},
		{"nil sharder", func(arg *discovery.ArgKadDht) { arg.KddSharder = nil }, p2p.ErrNilSharder},
		{"wrong sharder", func(arg *discovery.ArgKadDht) { arg.KddSharder = &mock.CommonSharder{} }, p2p.ErrWrongTypeAssertion},
		{"peers refresh interval", func(arg *discovery.ArgKadDht) { arg.PeersRefreshInterval = time.Millisecond }, p2p.ErrInvalidValue},
		{"routing table refresh", func(arg *discovery.ArgKadDht) { arg.RoutingTableRefresh = time.Millisecond }, p2p.ErrInvalidValue},
		{"seeders reconnection interval", func(arg *discovery.ArgKadDht) { arg.SeedersReconnectionInterval = time.Millisecond }, p2p.ErrInvalidValue},
		{"malformed initial peer", func(arg *discovery.ArgKadDht) { arg.InitialPeersList = []string{"peer1"} }, p2p.ErrInvalidValue},
		{"initial peer without transport address", func(arg *discovery.ArgKadDht) {
			arg.InitialPeersList = []string{"/p2p/" + seeders.ids[0].String()}
		}, p2p.ErrInvalidValue},
		{"own peer ID in initial peers", func(arg *discovery.ArgKadDht) {
			arg.InitialPeersList = []string{seeders.addresses[0], "/ip4/10.0.0.9/tcp/37373/p2p/" + self.String()}
		}, p2p.ErrInvalidValue},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			arg := createOptimizedTestArgument(ctx, newFakeSeederNetwork(self).host(), seeders.addresses)
			tt.modify(&arg)

			okdd, err := discovery.NewOptimizedKadDhtDiscoverer(arg)
			assert.True(t, check.IfNil(okdd))
			assert.True(t, errors.Is(err, tt.expectedErr), "got %v", err)
		})
	}
}

func TestNewOptimizedKadDhtDiscoverer_ShouldWork(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 2)
	arg := createOptimizedTestArgument(ctx, newFakeSeederNetwork(newTestPeerID(t)).host(), seeders.addresses)

	okdd, err := discovery.NewOptimizedKadDhtDiscoverer(arg)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(okdd))
	assert.Equal(t, discovery.OptimizedKadDhtName, okdd.Name())
}

//------- Bootstrap

func TestOptimizedKadDhtDiscoverer_BootstrapDialsAllSeeders(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	kadDht := &kadDhtStub{}
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), kadDht)
	require.NoError(t, err)

	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool { return kadDht.numBootstrap.Load() >= 1 }, timeoutWaitResponses, 10*time.Millisecond)
	for _, pid := range seeders.ids {
		assert.Equal(t, 1, fsn.numDials(pid))
		assert.True(t, fsn.isConnected(pid))

		fsn.mut.Lock()
		assert.Equal(t, time.Duration(peerstore.PermanentAddrTTL), fsn.ttls[pid])
		fsn.mut.Unlock()
	}
}

func TestOptimizedKadDhtDiscoverer_BootstrapTwiceShouldErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsn := newFakeSeederNetwork(newTestPeerID(t))
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), nil), &kadDhtStub{})
	require.NoError(t, err)

	assert.Nil(t, okdd.Bootstrap())
	assert.Equal(t, p2p.ErrPeerDiscoveryProcessAlreadyStarted, okdd.Bootstrap())
}

// waitSeederCycles waits until the discoverer has finished at least n seeder top-ups; the timeout
// allows one fast (1s) interval per cycle on top of the usual wait
func waitSeederCycles(t *testing.T, okdd interface{ NumSeederCycles() uint64 }, n uint64) {
	t.Helper()

	timeout := timeoutWaitResponses + time.Duration(n)*time.Second
	require.Eventually(t, func() bool { return okdd.NumSeederCycles() >= n }, timeout, 5*time.Millisecond)
}

// countedWithConnectedSeeder starts a discoverer whose only initial peer, seederAddress, is already
// connected under its peer ID from remote, and returns how many seeders the first top-up counted
func countedWithConnectedSeeder(t *testing.T, seederPID peer.ID, seederAddress string, remote string) (int, *fakeSeederNetwork) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsn := newFakeSeederNetwork(newTestPeerID(t))
	fsn.setConn(seederPID, remote)

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), []string{seederAddress}), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())
	waitSeederCycles(t, okdd, 1)

	return okdd.LastNumCountedSeeders(), fsn
}

func TestOptimizedKadDhtDiscoverer_SeederConnectedFromConfiguredIPCounts(t *testing.T) {
	t.Parallel()

	seeders := newTestSeeders(t, 1)
	// the seeder dialled us from its configured IP on an outgoing port
	numCounted, fsn := countedWithConnectedSeeder(t, seeders.ids[0], seeders.addresses[0], "/ip4/10.0.0.1/tcp/51234")

	assert.Equal(t, 1, numCounted)
	assert.Equal(t, 0, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_SeederConnectedFromIPv4MappedIPv6Counts(t *testing.T) {
	t.Parallel()

	seeders := newTestSeeders(t, 1)
	numCounted, fsn := countedWithConnectedSeeder(t, seeders.ids[0], seeders.addresses[0], "/ip6/::ffff:10.0.0.1/tcp/51234")

	assert.Equal(t, 1, numCounted)
	assert.Equal(t, 0, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_SeederPeerIDAtAnotherIPDoesNotCount(t *testing.T) {
	t.Parallel()

	seeders := newTestSeeders(t, 1)
	numCounted, fsn := countedWithConnectedSeeder(t, seeders.ids[0], seeders.addresses[0], "/ip4/192.168.7.7/tcp/37373")

	// libp2p would reuse the foreign connection, so no dial is made
	assert.Equal(t, 0, numCounted)
	assert.Equal(t, 0, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_DNSSeederMatchesOnPeerID(t *testing.T) {
	t.Parallel()

	pid := newTestPeerID(t)
	numCounted, fsn := countedWithConnectedSeeder(t, pid, "/dns4/seed.example.org/tcp/37373/p2p/"+pid.String(), "/ip4/192.168.7.7/tcp/40000")

	assert.Equal(t, 1, numCounted)
	assert.Equal(t, 0, fsn.numDials(pid))
}

func TestOptimizedKadDhtDiscoverer_DialLandingAtAnotherIPDoesNotCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 1)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	// the dial succeeds, but on an address libp2p learned elsewhere
	fsn.dialLandsAt[seeders.ids[0]] = multiaddr.StringCast("/ip4/192.168.7.7/tcp/37373")

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	// nothing is counted, so the fast timer keeps the top-ups coming; the later ones find the
	// connection from the other IP and do not dial again
	waitSeederCycles(t, okdd, 3)

	_, protected := fsn.connectedAndProtected(seeders.ids)
	assert.Empty(t, protected)
	assert.Equal(t, 0, okdd.LastNumCountedSeeders())
	assert.Equal(t, 1, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_SeederListedUnderTwoAddressesCountsOnce(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 2)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	// seeder 0 is listed under its IP and a DNS name, and is connected from the IP
	fsn.setConn(seeders.ids[0], "/ip4/10.0.0.1/tcp/51234")
	initialPeers := []string{
		seeders.addresses[0],
		"/dns4/seed-a.example.org/tcp/37373/p2p/" + seeders.ids[0].String(),
		seeders.addresses[1],
	}

	arg := createOptimizedTestArgument(ctx, fsn.host(), initialPeers)
	arg.MaxSeeders = 2
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())
	waitSeederCycles(t, okdd, 1)

	// seeder 0 takes one slot, so seeder 1 is dialled for the second; both end up protected
	_, protected := fsn.connectedAndProtected(seeders.ids)
	assert.Equal(t, 2, okdd.LastNumCountedSeeders())
	assert.Equal(t, 1, fsn.numDials(seeders.ids[1]))
	assert.ElementsMatch(t, seeders.ids, protected)
}

func TestOptimizedKadDhtDiscoverer_SeederListedUnderTwoIPsMatchesEither(t *testing.T) {
	t.Parallel()

	seeders := newTestSeeders(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsn := newFakeSeederNetwork(newTestPeerID(t))
	// connected from the second configured IP
	fsn.setConn(seeders.ids[0], "/ip4/10.0.0.9/tcp/51234")
	initialPeers := []string{seeders.addresses[0], "/ip4/10.0.0.9/tcp/37373/p2p/" + seeders.ids[0].String()}

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), initialPeers), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())
	waitSeederCycles(t, okdd, 1)

	_, protected := fsn.connectedAndProtected(seeders.ids)
	assert.Equal(t, 1, okdd.LastNumCountedSeeders())
	assert.Equal(t, seeders.ids, protected)
	assert.Equal(t, 0, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_BlackholedSeedersAreDialledInOneWave(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 4)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	for _, pid := range seeders.ids {
		fsn.blackholed[pid] = true
	}

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)

	start := time.Now()
	require.NoError(t, okdd.Bootstrap())
	waitSeederCycles(t, okdd, 1)

	// every dial waits for its 1s timeout; dialled one after another they would take 4s
	assert.Less(t, time.Since(start), 2*time.Second)
	for _, pid := range seeders.ids {
		assert.Equal(t, 1, fsn.numDials(pid))
	}
}

func TestOptimizedKadDhtDiscoverer_SeederDialBypassesSharder(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 1)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	// a sharder that evicts every peer
	arg.KddSharder = &mock.SharderStub{
		ComputeEvictListCalled: func(pidList []peer.ID) []peer.ID { return pidList },
		HasCalled: func(pid peer.ID, list []peer.ID) bool {
			for _, p := range list {
				if p == pid {
					return true
				}
			}
			return false
		},
	}

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool { return fsn.isConnected(seeders.ids[0]) }, timeoutWaitResponses, 10*time.Millisecond)
}

func TestOptimizedKadDhtDiscoverer_ConnectedSeedersAreSkipped(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	// seeder 0 dialled us from its configured IP on an outgoing port: counts as connected
	fsn.setConn(seeders.ids[0], "/ip4/10.0.0.1/tcp/51234")
	// seeder 1 is connected at its configured address
	fsn.setConn(seeders.ids[1], "/ip4/10.0.0.2/tcp/37373")

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool { return fsn.numDials(seeders.ids[2]) == 1 }, timeoutWaitResponses, 10*time.Millisecond)
	assert.Equal(t, 0, fsn.numDials(seeders.ids[0]))
	assert.Equal(t, 0, fsn.numDials(seeders.ids[1]))
}

//------- max seeders and protection

func TestOptimizedKadDhtDiscoverer_DialsOnlyUpToMaxSeeders(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	arg.MaxSeeders = 2
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())
	// the slow timer is an hour, so no top-up follows this one during the test
	waitSeederCycles(t, okdd, 1)

	totalDials := 0
	for _, pid := range seeders.ids {
		totalDials += fsn.numDials(pid)
	}
	connected, protected := fsn.connectedAndProtected(seeders.ids)
	assert.Equal(t, 2, totalDials)
	assert.Len(t, connected, 2)
	assert.ElementsMatch(t, connected, protected)
}

func TestOptimizedKadDhtDiscoverer_MaxSeedersRefillsADroppedSeeder(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	arg.MaxSeeders = 2
	arg.SeedersReconnectionInterval = 2 * time.Second
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool {
		connected, _ := fsn.connectedAndProtected(seeders.ids)
		return len(connected) == 2
	}, timeoutWaitResponses, 10*time.Millisecond)

	connected, _ := fsn.connectedAndProtected(seeders.ids)
	dropped := connected[0]
	fsn.drop(dropped)
	fsn.setUnreachable(dropped, true)

	// within one seeders reconnection interval the node is back to 2 seeders, which are the ones
	// protected; the dropped one lost its protection
	// (the protection is updated after the dials of a cycle, so wait for the sets to agree)
	sameSet := func(a, b []peer.ID) bool {
		if len(a) != len(b) {
			return false
		}
		for _, pid := range a {
			if !slices.Contains(b, pid) {
				return false
			}
		}
		return true
	}
	assert.Eventually(t, func() bool {
		connected, protected := fsn.connectedAndProtected(seeders.ids)
		return len(connected) == 2 && sameSet(connected, protected) && !slices.Contains(protected, dropped)
	}, 3*time.Second, 50*time.Millisecond)
}

func TestOptimizedKadDhtDiscoverer_SeedersOverTheCapAreNotProtected(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	// all three are connected, e.g. one of them reached through a DHT lookup
	for i, pid := range seeders.ids {
		fsn.setConn(pid, fmt.Sprintf("/ip4/10.0.0.%d/tcp/37373", i+1))
	}

	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	arg.MaxSeeders = 2
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool {
		_, protected := fsn.connectedAndProtected(seeders.ids)
		return len(protected) == 2
	}, timeoutWaitResponses, 10*time.Millisecond)
	for _, pid := range seeders.ids {
		assert.Equal(t, 0, fsn.numDials(pid))
	}
}

func TestOptimizedKadDhtDiscoverer_ProtectedSeedersKeepTheirSlots(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	fsn.setUnreachable(seeders.ids[0], true)

	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	arg.MaxSeeders = 2
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool {
		_, protected := fsn.connectedAndProtected(seeders.ids)
		return len(protected) == 2
	}, timeoutWaitResponses, 10*time.Millisecond)

	// the first seeder of the list shows up through some other path (e.g. a DHT lookup); the next
	// cycle must keep the two seeders it already protects rather than count the list's first two
	fsn.setConn(seeders.ids[0], "/ip4/10.0.0.1/tcp/37373")
	cyclesBefore := okdd.NumSeederCycles()
	<-okdd.ReconnectToNetwork()
	waitSeederCycles(t, okdd, cyclesBefore+1)

	_, protected := fsn.connectedAndProtected(seeders.ids)
	assert.ElementsMatch(t, seeders.ids[1:], protected)
}

func TestOptimizedKadDhtDiscoverer_NoCapProtectsEveryConnectedSeeder(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 3)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	fsn.setUnreachable(seeders.ids[2], true)
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool {
		_, protected := fsn.connectedAndProtected(seeders.ids)
		return len(protected) == 2
	}, timeoutWaitResponses, 10*time.Millisecond)
	_, protected := fsn.connectedAndProtected(seeders.ids)
	assert.ElementsMatch(t, seeders.ids[:2], protected)
}

//------- reconnect intervals

func TestOptimizedKadDhtDiscoverer_NoSeederReachableRetriesAtRefreshInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 2)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	for _, pid := range seeders.ids {
		fsn.setUnreachable(pid, true)
	}

	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	// one dial on Bootstrap, then one per second
	assert.Eventually(t, func() bool { return fsn.numDials(seeders.ids[0]) >= 3 }, 4*time.Second, 50*time.Millisecond)

	// a seeder that comes up is connected at the next fast retry
	fsn.setUnreachable(seeders.ids[1], false)
	assert.Eventually(t, func() bool { return fsn.isConnected(seeders.ids[1]) }, timeoutWaitResponses, 50*time.Millisecond)
}

func TestOptimizedKadDhtDiscoverer_ConnectedRetriesAtSeedersReconnectionInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 2)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	fsn.setUnreachable(seeders.ids[1], true)

	arg := createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses)
	arg.SeedersReconnectionInterval = 3 * time.Second
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(arg, &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	assert.Eventually(t, func() bool { return fsn.isConnected(seeders.ids[0]) }, timeoutWaitResponses, 10*time.Millisecond)

	// connected to one seeder: the unreachable one is not retried at the 1s refresh interval
	assert.Never(t, func() bool { return fsn.numDials(seeders.ids[1]) > 1 }, 2*time.Second, 50*time.Millisecond)

	// a seeder that drops is re-dialled within one seeders reconnection interval
	fsn.drop(seeders.ids[0])
	assert.Eventually(t, func() bool {
		return fsn.isConnected(seeders.ids[0]) && fsn.numDials(seeders.ids[1]) == 2
	}, 4*time.Second, 50*time.Millisecond)
	assert.Equal(t, 2, fsn.numDials(seeders.ids[0]))
}

func TestOptimizedKadDhtDiscoverer_ReconnectToNetworkDialsAtOnce(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seeders := newTestSeeders(t, 1)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), &kadDhtStub{})
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())
	assert.Eventually(t, func() bool { return fsn.numDials(seeders.ids[0]) == 1 }, timeoutWaitResponses, 10*time.Millisecond)

	fsn.drop(seeders.ids[0])
	select {
	case <-okdd.ReconnectToNetwork():
	case <-time.After(timeoutWaitResponses):
		assert.Fail(t, "ReconnectToNetwork should not block")
	}

	assert.Eventually(t, func() bool { return fsn.numDials(seeders.ids[0]) == 2 }, timeoutWaitResponses, 10*time.Millisecond)
}

//------- context cancellation

func TestOptimizedKadDhtDiscoverer_ContextCancelStopsTheLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	seeders := newTestSeeders(t, 1)
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	fsn.setUnreachable(seeders.ids[0], true)
	kadDht := &kadDhtStub{}
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), seeders.addresses), kadDht)
	require.NoError(t, err)
	require.NoError(t, okdd.Bootstrap())

	cancel()
	assert.Eventually(t, func() bool { return kadDht.numClose.Load() == 1 }, timeoutWaitResponses, 10*time.Millisecond)

	dialsAfterCancel := fsn.numDials(seeders.ids[0])
	assert.Never(t, func() bool { return fsn.numDials(seeders.ids[0]) != dialsAfterCancel }, 1500*time.Millisecond, 50*time.Millisecond)
	assert.True(t, errors.Is(okdd.Bootstrap(), context.Canceled))
}

func TestOptimizedKadDhtDiscoverer_BootstrapAfterCancelShouldNotBlock(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	fsn := newFakeSeederNetwork(newTestPeerID(t))
	okdd, err := discovery.NewOptimizedKadDhtDiscovererWithKadDht(
		createOptimizedTestArgument(ctx, fsn.host(), nil), &kadDhtStub{})
	require.NoError(t, err)

	cancel()

	chErr := make(chan error, 1)
	go func() { chErr <- okdd.Bootstrap() }()
	select {
	case err = <-chErr:
		assert.True(t, errors.Is(err, context.Canceled))
	case <-time.After(timeoutWaitResponses):
		assert.Fail(t, "Bootstrap should not block once the context is done")
	}
}
