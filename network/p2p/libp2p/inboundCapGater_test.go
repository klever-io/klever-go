package libp2p_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2pConnMgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gaterAtCap returns a gater bound to an armed protector (no warm-up) with cap 2 and two
// unprotected unknown peers holding one inbound connection each, so the cap is full.
func gaterAtCap(t *testing.T) (*mutablePeers, interface {
	InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) bool
}, func(pid peer.ID, tag string)) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(2)))
	pp := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2)
	g := libp2p.NewInboundCapGater()
	g.SetProtector(pp)
	return mp, g, cm.Protect
}

func TestInboundCapGater_RefusesUnknownInboundPeerAtCap(t *testing.T) {
	_, g, _ := gaterAtCap(t)
	assert.False(t, g.InterceptSecured(network.DirInbound, "newcomer", nil))
}

func TestInboundCapGater_AdmitsValidatorAtCap(t *testing.T) {
	mp, g, _ := gaterAtCap(t)
	mp.set("validator", core.ValidatorPeer)
	assert.True(t, g.InterceptSecured(network.DirInbound, "validator", nil))
}

func TestInboundCapGater_RefusesPeerProtectedByOtherSubsystemsAtCap(t *testing.T) {
	_, g, protect := gaterAtCap(t)
	// gossipsub never unprotects a mesh peer that disconnected, and kad-dht protects most
	// routing-table peers, so their tags must not exempt a peer from the cap.
	protect("mesh-peer", "pubsub:topic")
	protect("dht-peer", "kbucket")
	assert.False(t, g.InterceptSecured(network.DirInbound, "mesh-peer", nil))
	assert.False(t, g.InterceptSecured(network.DirInbound, "dht-peer", nil))
}

func TestInboundCapGater_AdmitsPeerWithValidatorTagAtCap(t *testing.T) {
	_, g, protect := gaterAtCap(t)
	protect("validator", libp2p.ValidatorProtectionTag)
	assert.True(t, g.InterceptSecured(network.DirInbound, "validator", nil))
}

func TestInboundCapGater_AdmitsOutboundAtCap(t *testing.T) {
	_, g, _ := gaterAtCap(t)
	assert.True(t, g.InterceptSecured(network.DirOutbound, "newcomer", nil))
}

func TestInboundCapGater_ValidatorTaggedPeersDoNotCountAgainstTheCap(t *testing.T) {
	_, g, protect := gaterAtCap(t)
	protect("u1", libp2p.ValidatorProtectionTag)
	assert.True(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "only u2's connection counts: 1 < 2")
}

func TestInboundCapGater_PeersProtectedByOtherSubsystemsCountAgainstTheCap(t *testing.T) {
	_, g, protect := gaterAtCap(t)
	protect("u1", "kbucket")
	protect("u2", "pubsub:topic")
	assert.False(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "u1 and u2 still count: 2 >= 2")
}

func TestInboundCapGater_AdmitsBelowCap(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("out", core.UnknownPeer, conn(network.DirOutbound, at(2)))
	g := libp2p.NewInboundCapGater()
	g.SetProtector(libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2))
	assert.True(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "1 inbound connection < 2; outbound ones do not count")
}

func TestInboundCapGater_AdmitsWhenNotArmedOrInWarmUp(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(2)))

	now := baseTime
	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 1, time.Minute, func() time.Time { return now })
	g := libp2p.NewInboundCapGater()
	g.SetProtector(pp)

	assert.True(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "not armed")
	pp.Arm()
	assert.True(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "armed, in warm-up")
	now = baseTime.Add(time.Minute)
	assert.False(t, g.InterceptSecured(network.DirInbound, "newcomer", nil), "past the warm-up the cap applies")
}

func TestInboundCapGater_AdmitsEverythingBeforeBinding(t *testing.T) {
	g := libp2p.NewInboundCapGater()
	assert.True(t, g.InterceptSecured(network.DirInbound, "newcomer", nil))
	assert.True(t, g.InterceptPeerDial("p"))
	assert.True(t, g.InterceptAddrDial("p", nil))
	assert.True(t, g.InterceptAccept(nil))
	allow, _ := g.InterceptUpgraded(connFrom("p", network.DirInbound))
	assert.True(t, allow)
}

func TestNewNetworkMessenger_GaterOnlyOnRegularNodes(t *testing.T) {
	regular, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
	require.NoError(t, err)
	defer func() { _ = regular.Close() }()
	gater, bound := regular.InboundGater()
	assert.NotNil(t, gater)
	assert.True(t, bound, "the gater must be bound to the messenger's protector")

	seedArgs := createMockNetworkArgs()
	seedArgs.IsSeedNode = true
	seed, err := libp2p.NewNetworkMessenger(seedArgs)
	require.NoError(t, err)
	defer func() { _ = seed.Close() }()
	gater, _ = seed.InboundGater()
	assert.Nil(t, gater, "seeds must accept everyone")
}

// TestNetworkMessenger_GaterRefusesUnknownDialAtCapButAdmitsValidator fills the cap with unknown
// inbound peers, then checks that the next unknown dial is refused at admission while a validator
// (per the resolver) still gets in, with no protector pass in between.
func TestNetworkMessenger_GaterRefusesUnknownDialAtCapButAdmitsValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	defer libp2p.SetInboundCapWarmUp(0)()

	const maxConns = 2
	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{MaxUnprotectedInboundConns: maxConns}
	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	validatorHost := newLocalPeerHost(t)
	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(pid core.PeerID) core.P2PPeerInfo {
			if peer.ID(pid) == validatorHost.ID() {
				return core.P2PPeerInfo{PeerType: core.ValidatorPeer}
			}
			return core.P2PPeerInfo{PeerType: core.UnknownPeer}
		},
	}))

	for i := 0; i < maxConns; i++ {
		connectToMessenger(t, newLocalPeerHost(t), mes)
	}
	gater, _ := mes.InboundGater()
	pendingGater, ok := gater.(interface{ PendingLen() int })
	require.True(t, ok)
	assert.Eventually(t, func() bool { return pendingGater.PendingLen() == 0 }, 5*time.Second, 10*time.Millisecond,
		"the swarm's Connected notifications must clear the pending entries")

	target := peer.AddrInfo{ID: mes.Host().ID(), Addrs: mes.Host().Addrs()}
	dial := func(ctxTimeout time.Duration, p interface {
		Connect(context.Context, peer.AddrInfo) error
	}) error {
		ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
		defer cancel()
		return p.Connect(ctx, target)
	}

	refused := newLocalPeerHost(t)
	// The dialer may or may not see the refusal as an error (the gater can refuse after the dialer's
	// side has completed its upgrade), so the outcome is asserted on the node's side below.
	_ = dial(5*time.Second, refused)
	assert.Never(t, func() bool {
		return mes.Host().Network().Connectedness(refused.ID()) == network.Connected
	}, 500*time.Millisecond, 20*time.Millisecond, "an unknown peer over the cap must be refused at admission")

	require.NoError(t, dial(5*time.Second, validatorHost))
	assert.Eventually(t, func() bool {
		return mes.Host().Network().Connectedness(validatorHost.ID()) == network.Connected
	}, 5*time.Second, 10*time.Millisecond, "a validator must be admitted even at the cap")
}

// upgradedGater is a gater bound to an armed protector (no warm-up) over mp with the given cap.
func upgradedGater(t *testing.T, mp *mutablePeers, maxConns int, now func() time.Time) (*p2pConnMgrBasic, gaterUnderTest) {
	cm := newTestConnMgr(t)
	g := libp2p.NewInboundCapGaterWithClock(now)
	g.SetProtector(libp2p.NewPeerProtector(cm, mp, mp.resolver(), maxConns))
	return cm, g
}

func TestInboundCapGater_UpgradedAdmitsOnlyTheRemainingBudgetUnderConcurrency(t *testing.T) {
	// Two connections are registered, the cap is 5, and 10 upgrades race. Nothing registers the
	// admitted connections while they race (as when clients stall before the swarm adds them), so
	// only the pending count can hold the line: exactly 3 may get in.
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, connFrom("u1", network.DirInbound))
	mp.connect("u2", core.UnknownPeer, connFrom("u2", network.DirInbound))
	_, g := upgradedGater(t, mp, 5, time.Now)

	const numUpgrades = 10
	var admitted atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numUpgrades; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if allow, _ := g.InterceptUpgraded(connFrom(peer.ID(fmt.Sprintf("racer-%d", i)), network.DirInbound)); allow {
				admitted.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(3), admitted.Load())
	assert.Equal(t, 3, g.PendingLen())
}

func TestInboundCapGater_ConnectedClearsPending(t *testing.T) {
	mp := &mutablePeers{}
	_, g := upgradedGater(t, mp, 1, time.Now)

	c1 := connFrom("p1", network.DirInbound)
	allow, _ := g.InterceptUpgraded(c1)
	require.True(t, allow)
	allow, _ = g.InterceptUpgraded(connFrom("p2", network.DirInbound))
	require.False(t, allow, "c1 is pending, so the cap of 1 is used")

	g.Connected(nil, c1)
	g.Connected(nil, c1) // a repeated notification must not free budget twice
	assert.Zero(t, g.PendingLen())
	allow, _ = g.InterceptUpgraded(connFrom("p3", network.DirInbound))
	assert.True(t, allow, "c1 is no longer pending and the fake network never registered it")
}

func TestInboundCapGater_PendingConnAlreadyRegisteredCountsOnce(t *testing.T) {
	mp := &mutablePeers{}
	_, g := upgradedGater(t, mp, 2, time.Now)

	c1 := connFrom("p1", network.DirInbound)
	allow, _ := g.InterceptUpgraded(c1)
	require.True(t, allow)
	mp.connect("p1", core.UnknownPeer, c1) // registered, Connected not delivered yet

	allow, _ = g.InterceptUpgraded(connFrom("p2", network.DirInbound))
	assert.True(t, allow, "1 registered + 0 pending < 2")
	allow, _ = g.InterceptUpgraded(connFrom("p3", network.DirInbound))
	assert.False(t, allow, "1 registered + 1 pending = 2")
}

func TestInboundCapGater_PendingExpiresAfterTTL(t *testing.T) {
	var mutNow sync.Mutex
	now := baseTime
	clock := func() time.Time {
		mutNow.Lock()
		defer mutNow.Unlock()
		return now
	}
	setNow := func(d time.Duration) {
		mutNow.Lock()
		now = baseTime.Add(d)
		mutNow.Unlock()
	}
	mp := &mutablePeers{}
	_, g := upgradedGater(t, mp, 1, clock)

	allow, _ := g.InterceptUpgraded(connFrom("lost", network.DirInbound)) // never registered
	require.True(t, allow)

	setNow(libp2p.InboundCapPendingTTL - time.Nanosecond)
	allow, _ = g.InterceptUpgraded(connFrom("p2", network.DirInbound))
	assert.False(t, allow, "still pending")

	setNow(libp2p.InboundCapPendingTTL)
	allow, _ = g.InterceptUpgraded(connFrom("p3", network.DirInbound))
	assert.True(t, allow, "the lost connection's entry expired")
	assert.Equal(t, 1, g.PendingLen(), "only p3 is pending")
}

func TestInboundCapGater_UpgradedExemptions(t *testing.T) {
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, connFrom("u1", network.DirInbound))
	cm, g := upgradedGater(t, mp, 1, time.Now) // full: 1 registered unprotected inbound conn

	allow, _ := g.InterceptUpgraded(connFrom("newcomer", network.DirInbound))
	require.False(t, allow, "the cap is full")

	mp.set("validator", core.ValidatorPeer)
	allow, _ = g.InterceptUpgraded(connFrom("validator", network.DirInbound))
	assert.True(t, allow, "validators are exempt")

	cm.Protect("tagged-validator", libp2p.ValidatorProtectionTag)
	allow, _ = g.InterceptUpgraded(connFrom("tagged-validator", network.DirInbound))
	assert.True(t, allow, "peers with the validator tag are exempt")

	cm.Protect("mesh-peer", "pubsub:topic")
	allow, _ = g.InterceptUpgraded(connFrom("mesh-peer", network.DirInbound))
	assert.False(t, allow, "protection from other subsystems does not exempt a peer")

	allow, _ = g.InterceptUpgraded(connFrom("newcomer", network.DirOutbound))
	assert.True(t, allow, "outbound is exempt")

	assert.Zero(t, g.PendingLen(), "exempt connections do not use cap budget")
}

func TestInboundCapGater_UpgradedAdmitsWhenNotArmedOrInWarmUp(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, connFrom("u1", network.DirInbound))
	now := baseTime
	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 1, time.Minute, func() time.Time { return now })
	g := libp2p.NewInboundCapGater()
	g.SetProtector(pp)

	allow, _ := g.InterceptUpgraded(connFrom("newcomer", network.DirInbound))
	assert.True(t, allow, "not armed")
	pp.Arm()
	allow, _ = g.InterceptUpgraded(connFrom("newcomer", network.DirInbound))
	assert.True(t, allow, "in warm-up")
	now = baseTime.Add(time.Minute)
	allow, _ = g.InterceptUpgraded(connFrom("newcomer", network.DirInbound))
	assert.False(t, allow, "enforced")
}

type p2pConnMgrBasic = p2pConnMgr.BasicConnMgr

type gaterUnderTest interface {
	InterceptUpgraded(network.Conn) (bool, control.DisconnectReason)
	PendingLen() int
	Connected(network.Network, network.Conn)
}
