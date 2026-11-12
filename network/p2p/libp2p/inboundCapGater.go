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
var _ network.Notifiee = (*inboundCapGater)(nil)

// inboundCapRejectionSummaryInterval is how often refused connections are summarized at Info.
const inboundCapRejectionSummaryInterval = time.Minute

// inboundCapPendingTTL bounds how long a connection admitted by InterceptUpgraded counts as pending
// before the swarm registers it. Registration normally follows within microseconds; a connection
// that fails in between (the swarm closing, for instance) never produces a Connected notification,
// so its entry expires instead of holding cap budget forever. A var so tests can change it.
var inboundCapPendingTTL = 5 * time.Second

// inboundCapGater enforces the peer protector's unprotected inbound cap at admission. The 10s
// protector pass alone reserves nothing: the ResourceManager admits any inbound connection while
// it is below its limit, so an attacker refills the slots a pass frees before a restarted
// validator's dials get through, and those dials back off. The gater refuses an inbound connection
// from a peer that is neither a validator per the resolver nor protected by any tag while the
// unprotected inbound connections already reach the cap. Validators with a fresh peer ID are
// admitted as soon as their heartbeat reaches this node: their connections to seeds are never
// gated (seeds run no gater, and KLC-2716 keeps every node connected to at least one seed), so the
// heartbeat gossips network-wide within ~25s. Outbound connections, and every connection before the
// cap is armed and past its warm-up, are admitted. libp2p needs the gater before the host exists, so
// the protector is bound late; until then everything is admitted. The pass still trims what is
// already over the cap.
//
// Two checks, because the swarm only lists a connection once it is fully set up:
//   - InterceptSecured runs right after the security handshake and rejects early, from the
//     connections already registered. It is not authoritative: on TCP a client can stall muxer
//     negotiation for the 15s accept timeout, so many connections can pass it while the count is
//     still low.
//   - InterceptUpgraded is authoritative. libp2p calls it from swarm.addConn after muxer
//     negotiation, just before the connection is registered. Under mut, it counts the registered
//     unprotected inbound connections plus the ones it admitted that are not registered yet
//     (pending), and records the new one as pending when it admits it. The Connected
//     notification carries the same network.Conn, which clears the entry; entries also expire
//     after inboundCapPendingTTL. A pending connection already visible in the swarm is counted
//     once.
//
// Locking (libp2p v0.48): neither hook is called with a swarm lock held. Counting reads
// Network().Peers() and ConnsToPeer, which take only the swarm's connection read lock, and the
// writer of that lock never waits on the gater. Connected is called with the connection's notify
// lock and the swarm's notifiee read lock held. Neither is taken by the code that holds mut, so
// Connected waiting on mut cannot deadlock.
type inboundCapGater struct {
	protector            atomic.Pointer[peerProtector]
	rejectedSinceSummary atomic.Uint64
	lastSummaryNanos     atomic.Int64

	mut     sync.Mutex
	pending map[network.Conn]time.Time
	now     func() time.Time
	// pendingTTL is inboundCapPendingTTL, copied at construction so connection goroutines never
	// read the package variable.
	pendingTTL time.Duration
}

func newInboundCapGater(now func() time.Time) *inboundCapGater {
	return &inboundCapGater{
		pending:    make(map[network.Conn]time.Time),
		now:        now,
		pendingTTL: inboundCapPendingTTL,
	}
}

// bind sets the protector whose cap the gater enforces and subscribes to the network's connection
// notifications, which clear pending entries.
func (g *inboundCapGater) bind(pp *peerProtector, net network.Network) {
	g.protector.Store(pp)
	if net != nil {
		net.Notify(g)
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
	registered, registeredConns := pp.unprotectedInboundConns()
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
	admit := registered+pending < pp.maxUnprotectedInboundConns
	if admit {
		g.pending[conn] = now
	}
	g.mut.Unlock()

	if !admit {
		g.recordRejection(pid, conn, pp.maxUnprotectedInboundConns)
		return false, 0
	}
	return true, 0
}

// Connected clears the pending entry of a connection the swarm has just registered.
func (g *inboundCapGater) Connected(_ network.Network, conn network.Conn) {
	g.mut.Lock()
	delete(g.pending, conn)
	g.mut.Unlock()
}

// Disconnected is a no-op: a pending entry is cleared by Connected or expires.
func (g *inboundCapGater) Disconnected(_ network.Network, _ network.Conn) {
	// Nothing to do: a pending entry is cleared by Connected, or by its TTL if the connection
	// never got registered.
}

// Listen is a no-op.
func (g *inboundCapGater) Listen(_ network.Network, _ ma.Multiaddr) {
	// Nothing to do: the gater only tracks connections, not listen addresses.
}

// ListenClose is a no-op.
func (g *inboundCapGater) ListenClose(_ network.Network, _ ma.Multiaddr) {
	// Nothing to do: the gater only tracks connections, not listen addresses.
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
