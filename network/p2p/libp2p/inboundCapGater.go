package libp2p

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var _ connmgr.ConnectionGater = (*inboundCapGater)(nil)

// inboundCapRejectionSummaryInterval is how often refused connections are summarized at Info.
const inboundCapRejectionSummaryInterval = time.Minute

// inboundCapPendingTTL bounds how long a connection admitted by InterceptUpgraded counts as pending
// before the swarm registers it. Registration normally follows within microseconds; a connection
// that fails in between (the swarm closing, for instance) never produces a Connected notification,
// so its entry expires instead of holding cap budget forever. A var so tests can change it.
var inboundCapPendingTTL = 5 * time.Second

// inboundCapGater enforces the peer protector's unprotected inbound cap at admission (see
// peerProtector for the cap itself). The 10s protector pass alone reserves nothing: the
// ResourceManager admits any inbound connection while it is below its limit, so an attacker refills
// the slots a pass frees before a restarted validator's dials get through. While the cap is
// reached, the gater admits an inbound connection from a peer that is not exempt
// (peerProtector.exemptFromCap) only by evicting an unknown peer that outstayed the identification
// window (peerProtector.evictionCandidate), closing that peer's inbound connections; with no such
// peer it refuses. Outbound connections, and every connection before the cap is armed and past its
// warm-up, are admitted. libp2p needs the gater before the host exists, so the protector is bound
// late; until then everything is admitted.
//
// A validator that starts with a fresh peer ID is unknown to every node. Where the cap is full it
// gets in by evicting such a stale unknown peer; where only observers and young unknown peers hold
// the cap its own dials are refused, and it gets in when another node dials it (discovered through
// the DHT; that connection is outbound on the other side, so never gated), or when it reaches a node
// below the cap. Once one such link exists it joins the heartbeat mesh and its heartbeat identifies
// it everywhere. Seeds do not help here: they join no topic, so they never relay heartbeats.
//
// Two checks, because the swarm only lists a connection once it is fully set up:
//   - InterceptSecured runs right after the security handshake and rejects early, from the
//     connections already registered, only when the cap is reached and no eviction could make
//     room. It evicts nothing. It is not authoritative: on TCP a client can stall muxer
//     negotiation for the 15s accept timeout, so many connections can pass it while the count is
//     still low.
//   - InterceptUpgraded is authoritative. libp2p calls it from swarm.addConn after muxer
//     negotiation, just before the connection is registered. Under mut, it counts the registered
//     unprotected inbound connections plus the ones it admitted that are not registered yet
//     (pending), and records the new one as pending when it admits it. The Connected
//     notification carries the same network.Conn, which clears the entry; entries also expire
//     after inboundCapPendingTTL. A pending connection already visible in the swarm is counted
//     once. At the cap it picks the eviction victim under mut and records the victim's
//     connections as evicting until they are closed: they no longer count, and no concurrent
//     admission can pick the same victim.
//
// Locking (libp2p v0.48): neither hook is called with a swarm lock held. Counting reads
// Network().Peers() and ConnsToPeer, which take only the swarm's connection read lock, and the
// writer of that lock never waits on the gater. Connected is called with the connection's notify
// lock and the swarm's notifiee read lock held. Neither is taken by the code that holds mut, so
// Connected waiting on mut cannot deadlock. A victim is closed in a goroutine, without mut:
// Conn.Close takes the swarm's connection write lock to unregister the connection and then closes
// the transport, which can block for seconds (a TLS close_notify write has a 5s deadline), so it
// must neither run under mut nor delay the admitted connection past inboundCapPendingTTL.
type inboundCapGater struct {
	protector            atomic.Pointer[peerProtector]
	rejectedSinceSummary atomic.Uint64
	lastSummaryNanos     atomic.Int64

	mut     sync.Mutex
	pending map[network.Conn]time.Time
	// evicting holds the inbound connections of eviction victims until their close returns.
	evicting map[network.Conn]struct{}
	now      func() time.Time
	// pendingTTL is inboundCapPendingTTL, copied at construction so connection goroutines never
	// read the package variable.
	pendingTTL time.Duration
}

func newInboundCapGater(now func() time.Time) *inboundCapGater {
	return &inboundCapGater{
		pending:    make(map[network.Conn]time.Time),
		evicting:   make(map[network.Conn]struct{}),
		now:        now,
		pendingTTL: inboundCapPendingTTL,
	}
}

// bind sets the protector whose cap the gater enforces and subscribes to the network's connection
// notifications, which clear pending entries.
func (g *inboundCapGater) bind(pp *peerProtector, net network.Network) {
	g.protector.Store(pp)
	if net != nil {
		net.Notify(&network.NotifyBundle{ConnectedF: g.Connected})
	}
}

// InterceptPeerDial admits every outbound dial.
func (g *inboundCapGater) InterceptPeerDial(_ peer.ID) bool {
	return true
}

// InterceptAddrDial admits every outbound dial.
func (g *inboundCapGater) InterceptAddrDial(_ peer.ID, _ ma.Multiaddr) bool {
	return true
}

// InterceptAccept admits every inbound connection: the peer ID is not known yet.
func (g *inboundCapGater) InterceptAccept(_ network.ConnMultiaddrs) bool {
	return true
}

// InterceptSecured is the early, non-authoritative check (see inboundCapGater).
func (g *inboundCapGater) InterceptSecured(dir network.Direction, pid peer.ID, addrs network.ConnMultiaddrs) bool {
	if dir != network.DirInbound {
		return true
	}
	pp := g.protector.Load()
	if pp == nil {
		return true
	}
	if pp.admitInbound(pid) {
		return true
	}

	g.recordRejection(pid, addrs, pp.maxUnprotectedInboundConns)
	return false
}

// InterceptUpgraded is the authoritative check (see inboundCapGater).
func (g *inboundCapGater) InterceptUpgraded(conn network.Conn) (bool, control.DisconnectReason) {
	if conn == nil || conn.Stat().Direction != network.DirInbound {
		return true, 0
	}
	pp := g.protector.Load()
	if pp == nil {
		return true, 0
	}
	pid := conn.RemotePeer()
	if !pp.capEnforcedNow() || pp.exemptFromCap(pid) {
		return true, 0
	}

	g.mut.Lock()
	registeredConns := pp.unprotectedInboundConns()
	registered := 0
	for c := range registeredConns {
		if _, isEvicting := g.evicting[c]; !isEvicting {
			registered++
		}
	}
	now := g.now()
	pending := 0
	for c, admittedAt := range g.pending {
		if now.Sub(admittedAt) >= g.pendingTTL {
			delete(g.pending, c)
			continue
		}
		if _, isRegistered := registeredConns[c]; !isRegistered {
			pending++
		}
	}
	used := registered + pending
	admit := used < pp.maxUnprotectedInboundConns
	var victim []network.Conn
	if !admit {
		victim = pp.evictionCandidate(pid, g.isEvicting)
		admit = len(victim) > 0 && used-len(victim) < pp.maxUnprotectedInboundConns
		if !admit {
			victim = nil
		}
		for _, c := range victim {
			g.evicting[c] = struct{}{}
		}
	}
	if admit {
		g.pending[conn] = now
	}
	g.mut.Unlock()

	if !admit {
		g.recordRejection(pid, conn, pp.maxUnprotectedInboundConns)
		return false, 0
	}
	if len(victim) > 0 {
		go g.evict(victim, pid)
	}
	return true, 0
}

// isEvicting reports whether conn belongs to an eviction victim. Called with mut held.
func (g *inboundCapGater) isEvicting(conn network.Conn) bool {
	_, ok := g.evicting[conn]
	return ok
}

// evict closes a victim's inbound connections, then stops tracking them: by then the swarm has
// unregistered them.
func (g *inboundCapGater) evict(victim []network.Conn, admitted peer.ID) {
	for _, c := range victim {
		if err := c.Close(); err != nil {
			log.Debug("inbound cap: closing evicted connection", "pid", c.RemotePeer().String(), "error", err)
		}
	}
	log.Debug("inbound cap: evicted an unknown peer past the identification window",
		"evicted", victim[0].RemotePeer().String(), "conns", len(victim), "admitted", admitted.String())

	g.mut.Lock()
	for _, c := range victim {
		delete(g.evicting, c)
	}
	g.mut.Unlock()
}

// Connected clears the pending entry of a connection the swarm has just registered.
func (g *inboundCapGater) Connected(_ network.Network, conn network.Conn) {
	if conn.Stat().Direction != network.DirInbound {
		return
	}
	g.mut.Lock()
	delete(g.pending, conn)
	g.mut.Unlock()
}

func (g *inboundCapGater) recordRejection(pid peer.ID, addrs network.ConnMultiaddrs, maxConns int) {
	remote := ""
	if addrs != nil && addrs.RemoteMultiaddr() != nil {
		remote = addrs.RemoteMultiaddr().String()
	}
	log.Debug("inbound connection refused: unprotected inbound connections at the cap",
		"pid", pid.String(), "remote", remote, "max unprotected inbound conns", maxConns)

	g.rejectedSinceSummary.Add(1)
	now := time.Now().UnixNano()
	last := g.lastSummaryNanos.Load()
	if now-last < int64(inboundCapRejectionSummaryInterval) || !g.lastSummaryNanos.CompareAndSwap(last, now) {
		return
	}
	log.Info("inbound connections refused because unprotected inbound connections reached the cap",
		"refused since last summary", g.rejectedSinceSummary.Swap(0),
		"max unprotected inbound conns", maxConns)
}
