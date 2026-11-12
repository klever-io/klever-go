package libp2p_test

import (
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2pConnMgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn is a network.Conn that only answers Stat, RemotePeer and RemoteMultiaddr; the protector
// and the gater read nothing else.
type fakeConn struct {
	network.Conn
	stat   network.ConnStats
	remote peer.ID
}

func (fc *fakeConn) Stat() network.ConnStats {
	return fc.stat
}

func (fc *fakeConn) RemotePeer() peer.ID {
	return fc.remote
}

func (fc *fakeConn) RemoteMultiaddr() ma.Multiaddr {
	return nil
}

func connFrom(pid peer.ID, dir network.Direction) network.Conn {
	return &fakeConn{stat: network.ConnStats{Stats: network.Stats{Direction: dir, Opened: time.Now()}}, remote: pid}
}

func conn(dir network.Direction, opened time.Time) network.Conn {
	return &fakeConn{stat: network.ConnStats{Stats: network.Stats{Direction: dir, Opened: opened}}}
}

// mutablePeers is a fake network (connected peers, their connections) and a peer-type map the
// tests change between passes. It implements libp2p.ProtectorNetwork.
type mutablePeers struct {
	mut    sync.Mutex
	peers  []peer.ID
	types  map[peer.ID]core.P2PPeerType
	conns  map[peer.ID][]network.Conn
	closed []peer.ID
}

func (mp *mutablePeers) Peers() []peer.ID {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	return append([]peer.ID(nil), mp.peers...)
}

func (mp *mutablePeers) ConnsToPeer(pid peer.ID) []network.Conn {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	return mp.conns[pid]
}

func (mp *mutablePeers) ClosePeer(pid peer.ID) error {
	mp.mut.Lock()
	mp.closed = append(mp.closed, pid)
	mp.mut.Unlock()
	mp.disconnect(pid)
	return nil
}

func (mp *mutablePeers) connect(pid peer.ID, pt core.P2PPeerType, conns ...network.Conn) {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	if mp.types == nil {
		mp.types = make(map[peer.ID]core.P2PPeerType)
	}
	if mp.conns == nil {
		mp.conns = make(map[peer.ID][]network.Conn)
	}
	mp.peers = append(mp.peers, pid)
	mp.types[pid] = pt
	mp.conns[pid] = conns
}

func (mp *mutablePeers) closedPeers() []peer.ID {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	return append([]peer.ID(nil), mp.closed...)
}

// noCap disables the unprotected inbound cap in tests that only look at protection and tags.
const noCap = 1 << 20

func (mp *mutablePeers) set(pid peer.ID, pt core.P2PPeerType) {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	mp.types[pid] = pt
}

func (mp *mutablePeers) disconnect(pid peer.ID) {
	mp.mut.Lock()
	defer mp.mut.Unlock()
	for i, p := range mp.peers {
		if p == pid {
			mp.peers = append(mp.peers[:i], mp.peers[i+1:]...)
			return
		}
	}
}

func (mp *mutablePeers) resolver() *mock.PeerShardResolverStub {
	return &mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(pid core.PeerID) core.P2PPeerInfo {
			mp.mut.Lock()
			defer mp.mut.Unlock()
			return core.P2PPeerInfo{PeerType: mp.types[peer.ID(pid)]} // missing → UnknownPeer (0)
		},
	}
}

func observerTagOf(cm *p2pConnMgr.BasicConnMgr, pid peer.ID) (int, bool) {
	info := cm.GetTagInfo(pid)
	if info == nil {
		return 0, false
	}
	v, ok := info.Tags[libp2p.ObserverTag]
	return v, ok
}

func TestPeerProtector_ProtectsValidatorsTagsObserversAndCleansUp(t *testing.T) {
	cm, err := p2pConnMgr.NewConnManager(1, 2)
	require.NoError(t, err)
	defer func() { _ = cm.Close() }()

	validator, observer, unknown, leaving := peer.ID("validator"), peer.ID("observer"), peer.ID("unknown"), peer.ID("leaving")
	mp := &mutablePeers{
		peers: []peer.ID{validator, observer, unknown, leaving},
		types: map[peer.ID]core.P2PPeerType{
			validator: core.ValidatorPeer,
			observer:  core.ObserverPeer,
			leaving:   core.ValidatorPeer,
		},
	}
	pp := libp2p.NewPeerProtector(cm, mp, mp.resolver(), noCap)

	res := pp.RunPass()
	assert.Equal(t, libp2p.PeerProtectorPassResult{Validators: 2, Observers: 1, Unknown: 1}, res)
	assert.True(t, cm.IsProtected(validator, libp2p.ValidatorProtectionTag))
	assert.True(t, cm.IsProtected(leaving, libp2p.ValidatorProtectionTag))
	assert.False(t, cm.IsProtected(observer, libp2p.ValidatorProtectionTag))
	assert.False(t, cm.IsProtected(unknown, libp2p.ValidatorProtectionTag))
	val, tagged := observerTagOf(cm, observer)
	assert.True(t, tagged, "observer must be tagged")
	assert.Equal(t, libp2p.ObserverTagValue, val)
	_, tagged = observerTagOf(cm, validator)
	assert.False(t, tagged, "validator must not carry the observer tag")
	_, tagged = observerTagOf(cm, unknown)
	assert.False(t, tagged, "unknown peer must not be tagged")

	t.Run("a validator that becomes unknown is unprotected", func(t *testing.T) {
		mp.set(validator, core.UnknownPeer)
		res := pp.RunPass()
		assert.Equal(t, 1, res.Unprotected)
		assert.False(t, cm.IsProtected(validator, libp2p.ValidatorProtectionTag))
	})

	t.Run("a disconnected validator is unprotected so the protection map does not leak", func(t *testing.T) {
		mp.disconnect(leaving)
		res := pp.RunPass()
		assert.Equal(t, 1, res.Unprotected)
		assert.False(t, cm.IsProtected(leaving, "")) // "" checks for any protection tag
	})

	t.Run("an observer that becomes a validator is protected and loses the observer tag", func(t *testing.T) {
		mp.set(observer, core.ValidatorPeer)
		pp.RunPass()
		assert.True(t, cm.IsProtected(observer, libp2p.ValidatorProtectionTag))
		_, tagged := observerTagOf(cm, observer)
		assert.False(t, tagged)
	})

	t.Run("a validator that becomes an observer is unprotected and tagged", func(t *testing.T) {
		mp.set(observer, core.ObserverPeer)
		pp.RunPass()
		assert.False(t, cm.IsProtected(observer, libp2p.ValidatorProtectionTag))
		val, tagged := observerTagOf(cm, observer)
		assert.True(t, tagged)
		assert.Equal(t, libp2p.ObserverTagValue, val)
	})

	t.Run("an observer that becomes unknown loses the observer tag", func(t *testing.T) {
		mp.set(observer, core.UnknownPeer)
		pp.RunPass()
		_, tagged := observerTagOf(cm, observer)
		assert.False(t, tagged)
		assert.False(t, cm.IsProtected(observer, libp2p.ValidatorProtectionTag))
	})

	t.Run("an unknown peer is never protected", func(t *testing.T) {
		pp.RunPass()
		assert.False(t, cm.IsProtected(unknown, ""))
	})
}

func TestPeerProtector_LeavesOtherProtectionTagsAlone(t *testing.T) {
	// gossipsub and kad-dht protect peers through the same connection manager under their own tags.
	cm, err := p2pConnMgr.NewConnManager(1, 2)
	require.NoError(t, err)
	defer func() { _ = cm.Close() }()

	pid := peer.ID("mesh-peer")
	cm.Protect(pid, "pubsub:topic")
	mp := &mutablePeers{peers: []peer.ID{pid}, types: map[peer.ID]core.P2PPeerType{pid: core.ValidatorPeer}}
	pp := libp2p.NewPeerProtector(cm, mp, mp.resolver(), noCap)

	pp.RunPass()
	mp.set(pid, core.UnknownPeer)
	pp.RunPass()

	assert.False(t, cm.IsProtected(pid, libp2p.ValidatorProtectionTag))
	assert.True(t, cm.IsProtected(pid, "pubsub:topic"), "the protector must only remove its own tag")
}

func newTestConnMgr(t *testing.T) *p2pConnMgr.BasicConnMgr {
	cm, err := p2pConnMgr.NewConnManager(1, 2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cm.Close() })
	return cm
}

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(sec int) time.Time {
	return baseTime.Add(time.Duration(sec) * time.Second)
}

// The fixed times below are long past, so these unknown peers are outside the identification window.
func TestPeerProtector_InboundCapClosesNewestUnknownPeersFirst(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(2)))
	mp.connect("u3", core.UnknownPeer, conn(network.DirInbound, at(3)))
	mp.connect("u4", core.UnknownPeer, conn(network.DirInbound, at(4)))

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2).RunPass()

	assert.Equal(t, 4, res.UnprotectedInboundUnknown)
	assert.Equal(t, 2, res.Closed)
	assert.Equal(t, []peer.ID{"u4", "u3"}, mp.closedPeers(), "the newest peers go first")
	assert.ElementsMatch(t, []peer.ID{"u1", "u2"}, mp.Peers())
}

func TestPeerProtector_InboundCapBudgetsConnectionsNotPeers(t *testing.T) {
	// Two identities holding five inbound connections each: two peers fit a cap of 4, ten
	// connections do not.
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	fiveConns := func(sec int) []network.Conn {
		conns := make([]network.Conn, 0, 5)
		for i := 0; i < 5; i++ {
			conns = append(conns, conn(network.DirInbound, at(sec)))
		}
		return conns
	}
	mp.connect("a", core.UnknownPeer, fiveConns(1)...)
	mp.connect("b", core.UnknownPeer, fiveConns(2)...)

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 4).RunPass()

	assert.Equal(t, 10, res.UnprotectedInboundConns)
	assert.Equal(t, 2, res.Closed, "closing one peer leaves 5 > 4, so both go")
	assert.Equal(t, 10, res.ClosedConns)
	assert.Equal(t, []peer.ID{"b", "a"}, mp.closedPeers())
}

func TestPeerProtector_InboundCapStopsOnceConnectionsFit(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("old", core.UnknownPeer, conn(network.DirInbound, at(1)), conn(network.DirInbound, at(1)))
	mp.connect("new", core.UnknownPeer, conn(network.DirInbound, at(2)), conn(network.DirInbound, at(2)), conn(network.DirInbound, at(2)))

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2).RunPass()

	assert.Equal(t, []peer.ID{"new"}, mp.closedPeers(), "5 conns, closing the newest peer leaves 2 <= 2")
	assert.Equal(t, 3, res.ClosedConns)
}

func TestPeerProtector_InboundCapOrderKeepsJustArrivedUnknownPeersLast(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	now := at(1000) // identification window (60s) starts at at(940)
	mp.connect("old-unknown-1", core.UnknownPeer, conn(network.DirInbound, at(100)))
	mp.connect("old-unknown-2", core.UnknownPeer, conn(network.DirInbound, at(200)))
	mp.connect("observer-1", core.ObserverPeer, conn(network.DirInbound, at(300)))
	mp.connect("observer-2", core.ObserverPeer, conn(network.DirInbound, at(990)))
	mp.connect("young-unknown-1", core.UnknownPeer, conn(network.DirInbound, at(950)))
	mp.connect("young-unknown-2", core.UnknownPeer, conn(network.DirInbound, at(980)))

	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 0, 0, func() time.Time { return now })
	pp.Arm()
	pp.RunPass()

	assert.Equal(t, []peer.ID{
		"old-unknown-2", "old-unknown-1", // unknown past the window, newest first
		"observer-2", "observer-1", // observers, newest first
		"young-unknown-1", "young-unknown-2", // unknown inside the window, oldest first
	}, mp.closedPeers())
}

func TestPeerProtector_InboundCapNotEnforcedUntilArmed(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(2)))

	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 1, 0, time.Now)
	res := pp.RunPass()
	assert.True(t, res.NotArmed)
	assert.Zero(t, res.Closed, "not armed: nothing is closed, even with no warm-up")

	pp.Arm()
	res = pp.RunPass()
	assert.False(t, res.NotArmed)
	assert.Equal(t, 1, res.Closed)
}

func TestPeerProtector_InboundCapIgnoresOutboundOnlyPeers(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("in", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("out1", core.UnknownPeer, conn(network.DirOutbound, at(2)))
	mp.connect("out2", core.UnknownPeer, conn(network.DirOutbound, at(3)))
	mp.connect("out3", core.UnknownPeer, conn(network.DirOutbound, at(4)))

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 1).RunPass()

	assert.Equal(t, 1, res.UnprotectedInboundUnknown, "outbound-only peers hold no inbound slot")
	assert.Zero(t, res.Closed)
	assert.Empty(t, mp.closedPeers())
}

func TestPeerProtector_InboundCapNeverClosesValidatorsOrProtectedPeers(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("validator", core.ValidatorPeer, conn(network.DirInbound, at(10)))
	mp.connect("mesh", core.UnknownPeer, conn(network.DirInbound, at(11)))
	mp.connect("dht", core.ObserverPeer, conn(network.DirInbound, at(12)))
	mp.connect("unknown", core.UnknownPeer, conn(network.DirInbound, at(1)))
	cm.Protect("mesh", "pubsub:topic")
	cm.Protect("dht", "kbucket")

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 0).RunPass()

	assert.Equal(t, []peer.ID{"unknown"}, mp.closedPeers(),
		"only the peer without any protection may be closed, even with a cap of 0")
	assert.Equal(t, 1, res.Closed)
}

func TestPeerProtector_InboundCapClosesObserversOnlyAfterUnknownPeers(t *testing.T) {
	t.Run("unknown peers go first even when observers are newer", func(t *testing.T) {
		cm := newTestConnMgr(t)
		mp := &mutablePeers{}
		mp.connect("u-old", core.UnknownPeer, conn(network.DirInbound, at(1)))
		mp.connect("u-older", core.UnknownPeer, conn(network.DirInbound, at(0)))
		mp.connect("o-new", core.ObserverPeer, conn(network.DirInbound, at(8)))
		mp.connect("o-newer", core.ObserverPeer, conn(network.DirInbound, at(9)))

		res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2).RunPass()

		assert.Equal(t, 2, res.UnprotectedInboundUnknown)
		assert.Equal(t, 2, res.UnprotectedInboundObservers)
		assert.Equal(t, []peer.ID{"u-old", "u-older"}, mp.closedPeers())
	})

	t.Run("observers are closed newest first once no unknown peer is left", func(t *testing.T) {
		cm := newTestConnMgr(t)
		mp := &mutablePeers{}
		mp.connect("u", core.UnknownPeer, conn(network.DirInbound, at(1)))
		mp.connect("o1", core.ObserverPeer, conn(network.DirInbound, at(2)))
		mp.connect("o2", core.ObserverPeer, conn(network.DirInbound, at(3)))
		mp.connect("o3", core.ObserverPeer, conn(network.DirInbound, at(4)))

		libp2p.NewPeerProtector(cm, mp, mp.resolver(), 1).RunPass()

		assert.Equal(t, []peer.ID{"u", "o3", "o2"}, mp.closedPeers())
		assert.Equal(t, []peer.ID{"o1"}, mp.Peers())
	})
}

func TestPeerProtector_InboundCapNotEnforcedDuringWarmUp(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("validator", core.ValidatorPeer, conn(network.DirInbound, at(1)))
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(2)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(3)))
	mp.connect("u3", core.UnknownPeer, conn(network.DirInbound, at(4)))

	var mutNow sync.Mutex
	now := baseTime
	clock := func() time.Time {
		mutNow.Lock()
		defer mutNow.Unlock()
		return now
	}
	advanceTo := func(t time.Time) {
		mutNow.Lock()
		now = t
		mutNow.Unlock()
	}
	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 1, time.Minute, clock)
	advanceTo(baseTime.Add(time.Hour)) // creation time does not start the warm-up, arming does
	pp.Arm()

	advanceTo(baseTime.Add(time.Hour + time.Minute - time.Nanosecond))
	res := pp.RunPass()
	assert.True(t, res.InWarmUp)
	assert.Equal(t, 3, res.UnprotectedInboundUnknown, "warm-up passes still count")
	assert.Zero(t, res.Closed)
	assert.Empty(t, mp.closedPeers(), "nothing is closed during the warm-up")
	assert.True(t, cm.IsProtected("validator", libp2p.ValidatorProtectionTag), "warm-up passes still protect")

	advanceTo(baseTime.Add(time.Hour + time.Minute))
	res = pp.RunPass()
	assert.False(t, res.InWarmUp)
	assert.Equal(t, 2, res.Closed)
}

func TestPeerProtector_ArmingTwiceDoesNotRestartWarmUp(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("u2", core.UnknownPeer, conn(network.DirInbound, at(2)))

	var mutNow sync.Mutex
	now := baseTime
	clock := func() time.Time {
		mutNow.Lock()
		defer mutNow.Unlock()
		return now
	}
	pp := libp2p.NewPeerProtectorWithClock(cm, mp, mp.resolver(), 1, time.Minute, clock)
	pp.Arm()

	mutNow.Lock()
	now = baseTime.Add(time.Minute)
	mutNow.Unlock()
	pp.Arm() // a later SetPeerShardResolver call must not push enforcement back

	res := pp.RunPass()
	assert.False(t, res.InWarmUp)
	assert.Equal(t, 1, res.Closed)
}

func TestPeerProtector_InboundCapUnderCapClosesNothing(t *testing.T) {
	cm := newTestConnMgr(t)
	mp := &mutablePeers{}
	mp.connect("u1", core.UnknownPeer, conn(network.DirInbound, at(1)))
	mp.connect("o1", core.ObserverPeer, conn(network.DirInbound, at(2)))

	res := libp2p.NewPeerProtector(cm, mp, mp.resolver(), 2).RunPass()

	assert.Zero(t, res.Closed)
	assert.Empty(t, mp.closedPeers())
}
