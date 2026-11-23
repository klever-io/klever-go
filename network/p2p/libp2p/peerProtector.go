package libp2p

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// validatorProtectionTag is the Protect tag held on every connected peer the PeerShardResolver
	// reports as a validator. BasicConnMgr never trims a protected peer.
	validatorProtectionTag = "klever-validator"
	// observerTag gives known observers a small positive value in the connection manager. It does
	// not decide BasicConnMgr's trim order on its own: gossipsub's decaying pubsub-deliveries tags
	// (one per topic, bumped for every message a peer delivers first) usually dominate a peer's
	// value. The protector's own inbound cap orders by class instead (see peerProtector).
	observerTag      = "klever-observer"
	observerTagValue = 10
)

// peerProtectorInterval is how often the protector re-classifies the connected peers. A newly
// connected validator stays unprotected until the next pass, which the connection manager's
// grace period (60s by default) covers. It is a var so tests can shorten it.
var peerProtectorInterval = 10 * time.Second

// inboundCapWarmUp is how long the unprotected inbound cap waits after it is armed, which happens
// when SetPeerShardResolver installs the node's real resolver (before that every peer resolves as
// unknown). A validator is UnknownPeer until the node learns its public key, from a heartbeat
// (broadcast every 20-25s) or a consensus message; 60s covers two heartbeat rounds plus a protector
// pass, so reconnecting validators are identified and protected before the cap can close them.
// Until armed and during the warm-up, passes still protect, tag and count, but close nothing. It
// is a var so tests can change it.
var inboundCapWarmUp = 60 * time.Second

// unknownPeerIdentificationWindow is how long an unknown peer is given to identify itself, with
// the same reasoning as inboundCapWarmUp: a validator that has just connected is unknown until its
// next heartbeat. Unknown peers younger than this are the last ones the cap closes. A var so tests
// can change it.
var unknownPeerIdentificationWindow = 60 * time.Second

// peerProtectorPassResult counts what one protector pass did.
type peerProtectorPassResult struct {
	Validators  int
	Observers   int
	Unknown     int
	Unprotected int
	// UnprotectedInboundUnknown and UnprotectedInboundObservers count the peers subject to the
	// inbound cap and UnprotectedInboundConns their inbound connections, before any was closed.
	// Closed is how many peers the pass left with no inbound connection and ClosedConns how many
	// inbound connections its closes removed.
	UnprotectedInboundUnknown   int
	UnprotectedInboundObservers int
	UnprotectedInboundConns     int
	Closed                      int
	ClosedConns                 int
	// NotArmed is true until SetPeerShardResolver arms the cap; InWarmUp while it is armed but
	// still waiting out inboundCapWarmUp. In both cases nothing is closed.
	NotArmed bool
	InWarmUp bool
}

// protectorNetwork is the part of network.Network the protector uses.
type protectorNetwork interface {
	Peers() []peer.ID
	ConnsToPeer(p peer.ID) []network.Conn
}

// Closing priority classes of the inbound cap, lowest first.
const (
	priorityIdentifiedTooLateUnknown = iota
	priorityObserver
	priorityYoungUnknown
)

// inboundCandidate is a connected peer subject to the inbound cap.
type inboundCandidate struct {
	pid      peer.ID
	priority int
	// firstInbound is when the peer's oldest open inbound connection was opened.
	firstInbound time.Time
	inboundConns int
}

// peerProtector keeps the connection manager's protection and tags in line with what the
// PeerShardResolver knows: validators are protected, observers are tagged, unknown peers carry
// neither. BasicConnMgr keeps its protection map apart from the per-connection state and never
// clears it on disconnect, so the protector remembers which peers it protected and unprotects
// each one once it is no longer a connected validator; otherwise the map would grow with every
// validator identity ever seen. Tags live in the per-peer record that BasicConnMgr drops on the
// last disconnect, so only still-connected peers need their observer tag removed.
//
// BasicConnMgr counts connections, not peers. Once the count reaches highWater it closes the
// connections of the lowest-value unprotected peers that are past their grace period, until
// lowWater unprotected, out-of-grace connections remain. Which peers go first follows their
// connection-manager value, which gossipsub's decaying delivery tags dominate; the observer tag
// adds little. Protected peers are not counted against lowWater and are never trimmed, so lowWater
// is the budget for unprotected peers, and lowWater plus the validators' inbound connections must
// fit in the inbound ResourceManager limit.
//
// Each pass also enforces maxUnprotectedInboundConns. BasicConnMgr never trims a connection
// younger than its grace period, so a peer that keeps rotating its identity and reconnecting stays
// inside the grace period forever, is never a trim candidate, and can hold inbound slots
// indefinitely. The protector therefore sums the inbound connections of the connected peers that
// are not validators, and closes their inbound connections, a peer at a time and regardless of
// grace, until that sum is within the cap. Outbound connections are never closed: we chose them.
// Only validators are exempt (resolver-identified, or carrying validatorProtectionTag). Protection
// set by other subsystems does not exempt a peer: kad-dht protects most routing-table peers (its
// two lowest buckets), and gossipsub never unprotects a mesh peer that disconnects (its
// RemovePeer does not call Prune, and BasicConnMgr keeps protections across disconnects), so any
// identity that joined a mesh once would otherwise bypass the cap on every reconnect. Those peers
// still keep libp2p's own trim protection. The
// budget is in connections because the ResourceManager limits connections: one identity can hold
// several inbound connections. Closing order:
//  1. unknown peers that have been connected longer than unknownPeerIdentificationWindow, newest
//     first: they had time to identify themselves and did not, and rotating identities are newer
//     than established peers;
//  2. observers, newest first: a peer becomes an "observer" just by sending a heartbeat signed
//     with a key it generated itself, so that label is no proof of trust;
//  3. unknown peers still inside the identification window, oldest first: a validator that has
//     just connected looks like one of these until its next heartbeat, so it is the last thing
//     closed.
//
// The last rule is a trade-off. An attacker that reopens its connections under a fresh peer ID
// every <60s also stays in class 3, so when the cap is exceeded it outlives honest observers and
// long-lived unknown peers and keeps its share of the cap. That share is all it gets: the cap bounds
// every non-validator together, so the inbound slots above it stay free for validators. Closing
// young peers first would instead close the validator that has just connected, which is what the
// cap exists to protect. libp2p's per-IP limits (8 connections per IPv4 /32) bound how much of the
// cap one address can take.
//
// The pass only acts above the cap; at the cap, inboundCapGater evicts at admission (see its doc and
// evictionCandidate).
//
// A validator protected by an earlier pass stays protected for a bounded number of passes while it
// is connected and the resolver only reports it as unknown, so a resolver cache miss does not expose
// it to the cap (see passPeerType).
type peerProtector struct {
	mut                        sync.Mutex
	cm                         connmgr.ConnManager
	net                        protectorNetwork
	resolver                   func() p2p.PeerShardResolver
	maxUnprotectedInboundConns int
	protected                  map[peer.ID]struct{}
	tagged                     map[peer.ID]struct{}
	now                        func() time.Time
	warmUp                     time.Duration
	// droppedMut guards dropped. It is separate from mut so the swarm's Disconnected notifications
	// never wait for a pass to finish.
	droppedMut sync.Mutex
	// dropped holds the peers that have had zero connections since the current pass started.
	dropped map[peer.ID]struct{}
	// unknownPasses counts, per protected peer, the consecutive passes it was kept protected on an
	// UnknownPeer reading. Guarded by mut.
	unknownPasses map[peer.ID]int
	// identificationWindow is unknownPeerIdentificationWindow, copied at construction so passes on
	// the protector's goroutine never read the package variable.
	identificationWindow time.Duration
	// capEnforcedFrom is when the cap starts being enforced, or nil while it is not armed. Atomic,
	// so the connection gater reads it without waiting for a pass to finish. A time.Time rather
	// than UnixNano keeps the monotonic reading, so a wall-clock step cannot stretch the warm-up.
	capEnforcedFrom atomic.Pointer[time.Time]
}

func newPeerProtector(
	cm connmgr.ConnManager,
	net protectorNetwork,
	resolver func() p2p.PeerShardResolver,
	maxUnprotectedInboundConns int,
	warmUp time.Duration,
	now func() time.Time,
) *peerProtector {
	return &peerProtector{
		cm:                         cm,
		net:                        net,
		resolver:                   resolver,
		maxUnprotectedInboundConns: maxUnprotectedInboundConns,
		protected:                  make(map[peer.ID]struct{}),
		tagged:                     make(map[peer.ID]struct{}),
		dropped:                    make(map[peer.ID]struct{}),
		unknownPasses:              make(map[peer.ID]int),
		now:                        now,
		warmUp:                     warmUp,
		identificationWindow:       unknownPeerIdentificationWindow,
	}
}

// arm starts the warm-up after which the inbound cap is enforced. Only the first call counts.
func (pp *peerProtector) arm() {
	from := pp.now().Add(pp.warmUp)
	pp.capEnforcedFrom.CompareAndSwap(nil, &from)
}

// runPass classifies every connected peer once and updates protection and tags accordingly.
func (pp *peerProtector) runPass() peerProtectorPassResult {
	pp.mut.Lock()
	defer pp.mut.Unlock()

	var res peerProtectorPassResult
	dropped := pp.takeDropped()
	peers := pp.net.Peers()
	resolver := pp.resolver()

	connected := make(map[peer.ID]struct{}, len(peers))
	protected := make(map[peer.ID]struct{}, len(pp.protected))
	tagged := make(map[peer.ID]struct{}, len(pp.tagged))
	peerTypes := make(map[peer.ID]core.P2PPeerType, len(peers))

	for _, pid := range peers {
		connected[pid] = struct{}{}
		peerType := pp.passPeerType(resolver, pid, dropped)
		peerTypes[pid] = peerType

		switch peerType {
		case core.ValidatorPeer:
			pp.cm.Protect(pid, validatorProtectionTag)
			protected[pid] = struct{}{}
			res.Validators++
		case core.ObserverPeer:
			pp.cm.TagPeer(pid, observerTag, observerTagValue)
			tagged[pid] = struct{}{}
			res.Observers++
		default:
			res.Unknown++
		}
	}

	// Peers protected last pass that are no longer connected validators: demoted to observer, or
	// disconnected (whose entry would otherwise leak).
	for pid := range pp.protected {
		if _, ok := protected[pid]; ok {
			continue
		}
		pp.cm.Unprotect(pid, validatorProtectionTag)
		delete(pp.unknownPasses, pid)
		res.Unprotected++
	}

	// Peers tagged last pass that are still connected but are no longer observers.
	for pid := range pp.tagged {
		if _, ok := tagged[pid]; ok {
			continue
		}
		if _, ok := connected[pid]; !ok {
			continue
		}
		pp.cm.UntagPeer(pid, observerTag)
	}

	pp.protected = protected
	pp.tagged = tagged

	pp.enforceInboundCap(peers, peerTypes, &res)

	return res
}

// maxUnknownPassesProtected is how many consecutive passes a validator protected last pass stays
// protected while the resolver reports it as unknown: about 60s, two to three heartbeat windows. A
// validator still in the set heartbeats within that time, which refreshes the resolver, so only a
// peer that stopped identifying itself (one that left the validator set, for instance) loses
// protection, and flushing the resolver's cache has to be sustained that long to expose a validator.
const maxUnknownPassesProtected = 6

// passPeerType is pid's type for this pass. A validator protected last pass stays a validator for up
// to maxUnknownPassesProtected consecutive passes while it is connected and the resolver merely stops
// knowing it (its entry evicted from the resolver's cache, for instance); an observer reading, a
// disconnect, or running out of those passes unprotects it. A peer that had zero connections since
// the previous pass (in dropped) gets the resolver's reading, so a reconnect does not inherit the old
// protection, while one that replaced a connection without ever dropping to zero keeps it. The swarm
// sends Disconnected from a goroutine after removing the connection, so a reconnect whose handshake
// completes before that notification is taken for a replacement. Called with mut held.
func (pp *peerProtector) passPeerType(resolver p2p.PeerShardResolver, pid peer.ID, dropped map[peer.ID]struct{}) core.P2PPeerType {
	peerType := resolver.GetPeerInfo(core.PeerID(pid)).PeerType
	if peerType != core.UnknownPeer {
		delete(pp.unknownPasses, pid)
		return peerType
	}
	_, wasProtected := pp.protected[pid]
	_, wasDropped := dropped[pid]
	if !wasProtected || wasDropped || pp.unknownPasses[pid] >= maxUnknownPassesProtected {
		delete(pp.unknownPasses, pid)
		return peerType
	}
	pp.unknownPasses[pid]++

	return core.ValidatorPeer
}

// takeDropped returns the peers that dropped to zero connections since the previous pass and starts
// a new set for the next one.
func (pp *peerProtector) takeDropped() map[peer.ID]struct{} {
	pp.droppedMut.Lock()
	defer pp.droppedMut.Unlock()
	dropped := pp.dropped
	pp.dropped = make(map[peer.ID]struct{})
	return dropped
}

// notifiee returns the connection notification the protector needs; register it on the network.
func (pp *peerProtector) notifiee() network.Notifiee {
	return &network.NotifyBundle{
		DisconnectedF: func(_ network.Network, c network.Conn) { pp.connClosed(c.RemotePeer()) },
	}
}

// connClosed records pid in dropped when it has no connection left. The swarm removes a closed
// connection before notifying, so the connections left are the peer's other open ones, including any
// that replaced the closed one.
func (pp *peerProtector) connClosed(pid peer.ID) {
	if len(pp.net.ConnsToPeer(pid)) > 0 {
		return
	}

	pp.droppedMut.Lock()
	defer pp.droppedMut.Unlock()
	pp.dropped[pid] = struct{}{}
}

// enforceInboundCap closes unprotected inbound peers, in the order described on peerProtector,
// until their inbound connections fit in maxUnprotectedInboundConns.
func (pp *peerProtector) enforceInboundCap(peers []peer.ID, peerTypes map[peer.ID]core.P2PPeerType, res *peerProtectorPassResult) {
	now := pp.now()
	candidates, total := pp.collectInboundCandidates(peers, peerTypes, now, res)
	res.UnprotectedInboundConns = total

	armed, enforced := pp.capState(now)
	if !armed {
		res.NotArmed = true
		return
	}
	if !enforced {
		res.InWarmUp = true
		return
	}
	if total <= pp.maxUnprotectedInboundConns {
		return
	}

	sortInboundCandidates(candidates)
	pp.closeExcessInbound(candidates, total, res)
}

// collectInboundCandidates returns the connected peers that count against the cap (not a
// validator, not carrying validatorProtectionTag, at least one inbound connection), each with its closing
// priority, and the total of their inbound connections.
func (pp *peerProtector) collectInboundCandidates(
	peers []peer.ID,
	peerTypes map[peer.ID]core.P2PPeerType,
	now time.Time,
	res *peerProtectorPassResult,
) ([]inboundCandidate, int) {
	identifiedBy := now.Add(-pp.identificationWindow)

	candidates := make([]inboundCandidate, 0)
	total := 0
	for _, pid := range peers {
		peerType := peerTypes[pid]
		if peerType == core.ValidatorPeer {
			continue
		}
		// Only our validator tag exempts a peer; see peerProtector for why other tags do not.
		if pp.cm.IsProtected(pid, validatorProtectionTag) {
			continue
		}

		firstInbound, inboundConns := inboundConnStats(pp.net.ConnsToPeer(pid))
		if inboundConns == 0 {
			continue
		}

		priority := inboundPriority(peerType, firstInbound, identifiedBy)
		if priority == priorityObserver {
			res.UnprotectedInboundObservers++
		} else {
			res.UnprotectedInboundUnknown++
		}
		total += inboundConns
		candidates = append(candidates, inboundCandidate{
			pid:          pid,
			priority:     priority,
			firstInbound: firstInbound,
			inboundConns: inboundConns,
		})
	}

	return candidates, total
}

// inboundPriority classifies an unprotected inbound peer: observers, unknown peers that connected
// inside the identification window, and unknown peers that had time to identify themselves.
func inboundPriority(peerType core.P2PPeerType, firstInbound time.Time, identifiedBy time.Time) int {
	switch {
	case peerType == core.ObserverPeer:
		return priorityObserver
	case firstInbound.After(identifiedBy):
		return priorityYoungUnknown
	default:
		return priorityIdentifiedTooLateUnknown
	}
}

// sortInboundCandidates orders candidates for closing: by priority, then newest first, except
// young unknown peers, which go oldest first so a just-connected validator is closed last.
func sortInboundCandidates(candidates []inboundCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		ci, cj := candidates[i], candidates[j]
		if ci.priority != cj.priority {
			return ci.priority < cj.priority
		}
		if ci.priority == priorityYoungUnknown {
			return ci.firstInbound.Before(cj.firstInbound)
		}
		return ci.firstInbound.After(cj.firstInbound)
	})
}

// closeExcessInbound closes the inbound connections of candidates, in order, until the inbound
// connections fit in the cap. What a close freed is recounted from the network, so a close that
// errors after unregistering its connection still counts, and one that left it open does not.
func (pp *peerProtector) closeExcessInbound(candidates []inboundCandidate, total int, res *peerProtectorPassResult) {
	for _, c := range candidates {
		if total <= pp.maxUnprotectedInboundConns {
			return
		}
		closeInboundConns(pp.net.ConnsToPeer(c.pid), c.pid)
		_, remaining := inboundConnStats(pp.net.ConnsToPeer(c.pid))
		removed := c.inboundConns - remaining
		if removed <= 0 {
			continue
		}
		total -= removed
		res.ClosedConns += removed
		if remaining == 0 {
			res.Closed++
		}
	}
}

// closeInboundConns closes the inbound connections among conns, all to pid.
func closeInboundConns(conns []network.Conn, pid peer.ID) {
	for _, conn := range inboundOnly(conns) {
		if err := conn.Close(); err != nil {
			log.Debug("peer protector: closing unprotected inbound connection", "pid", pid.String(), "error", err)
		}
	}
}

// capState reports whether the cap is armed and whether, at now, it is past its warm-up.
func (pp *peerProtector) capState(now time.Time) (armed bool, enforced bool) {
	from := pp.capEnforcedFrom.Load()
	if from == nil {
		return false, false
	}
	return true, !now.Before(*from)
}

// admitInbound is the gater's early check of a new inbound connection from pid. It refuses only
// when the cap is enforced, pid is neither a validator per the resolver nor carrying
// validatorProtectionTag, the unprotected inbound connections already reach the cap, and evicting
// evictionCandidate would not bring them below it. It evicts nothing: the gater's authoritative
// check does. It does not take mut, so it never waits for a pass.
func (pp *peerProtector) admitInbound(pid peer.ID) bool {
	if !pp.capEnforcedNow() || pp.exemptFromCap(pid) {
		return true
	}

	count := len(pp.unprotectedInboundConns())
	if count < pp.maxUnprotectedInboundConns {
		return true
	}
	victim := pp.evictionCandidate(pid, nil, 0)
	return len(victim) > 0 && count-len(victim) < pp.maxUnprotectedInboundConns
}

// evictionVictim is the newest eviction candidate of one class.
type evictionVictim struct {
	conns        []network.Conn
	firstInbound time.Time
}

func (v *evictionVictim) offer(conns []network.Conn, firstInbound time.Time) {
	if len(v.conns) == 0 || firstInbound.After(v.firstInbound) {
		v.conns, v.firstInbound = conns, firstInbound
	}
}

// youngUnknownShareDivisor sets the young unknown peers' share of the cap that observer evictions
// stop at: maxUnprotectedInboundConns/youngUnknownShareDivisor (see evictionCandidate).
const youngUnknownShareDivisor = 4

// youngUnknownBudget is how many inbound connections young unknown peers may hold before a newcomer
// can no longer displace an observer. At least 1.
func (pp *peerProtector) youngUnknownBudget() int {
	return max(1, pp.maxUnprotectedInboundConns/youngUnknownShareDivisor)
}

// evictionCandidate returns the inbound connections of the peer whose eviction may admit a new
// inbound connection from pid at a full cap, or nil. Only peers other than pid, not carrying
// validatorProtectionTag, whose first inbound connection is older than identificationWindow
// qualify: the class 1 peer (unknown) that connected last or, when there is none, the observer that
// connected last. Peers with a connection for which skip returns true (being evicted already) are
// passed over.
//
// An observer is offered only while young unknown peers (inside identificationWindow) hold fewer
// than youngUnknownBudget inbound connections, pendingInbound (admitted, not registered yet)
// included. A young identity cannot be evicted, so without that bound an attacker that redials
// under fresh peer IDs every <60s would displace every honest observer, one per dial. With it,
// such an attacker displaces observers only up to its share; past it, it gets only the slots that
// others free. A peer that identifies leaves the young share, so honest newcomers keep displacing
// observer identities that hold the cap. It does not take mut.
func (pp *peerProtector) evictionCandidate(pid peer.ID, skip func(network.Conn) bool, pendingInbound int) []network.Conn {
	identifiedBy := pp.now().Add(-pp.identificationWindow)
	resolver := pp.resolver()

	youngUnknown := pendingInbound
	var unknown, observer evictionVictim
	for _, candidate := range pp.net.Peers() {
		inbound, firstInbound, ok := pp.capInboundConns(candidate, pid)
		if !ok {
			continue
		}
		peerType := resolver.GetPeerInfo(core.PeerID(candidate)).PeerType
		switch {
		case peerType == core.ValidatorPeer:
		case firstInbound.After(identifiedBy):
			if peerType != core.ObserverPeer {
				youngUnknown += len(inbound)
			}
		case anyConn(inbound, skip):
		case peerType == core.ObserverPeer:
			observer.offer(inbound, firstInbound)
		default:
			unknown.offer(inbound, firstInbound)
		}
	}

	if len(unknown.conns) > 0 {
		return unknown.conns
	}
	if youngUnknown >= pp.youngUnknownBudget() {
		return nil
	}
	return observer.conns
}

// capInboundConns returns candidate's inbound connections and when the oldest was opened, and
// whether candidate counts for eviction at all: it is not pid, does not carry
// validatorProtectionTag, and has an inbound connection.
func (pp *peerProtector) capInboundConns(candidate peer.ID, pid peer.ID) ([]network.Conn, time.Time, bool) {
	if candidate == pid || pp.cm.IsProtected(candidate, validatorProtectionTag) {
		return nil, time.Time{}, false
	}
	inbound := inboundOnly(pp.net.ConnsToPeer(candidate))
	if len(inbound) == 0 {
		return nil, time.Time{}, false
	}

	return inbound, oldestOpened(inbound), true
}

// anyConn reports whether match returns true for one of conns; a nil match matches nothing.
func anyConn(conns []network.Conn, match func(network.Conn) bool) bool {
	if match == nil {
		return false
	}
	for _, conn := range conns {
		if match(conn) {
			return true
		}
	}
	return false
}

// capEnforcedNow reports whether the cap is armed and past its warm-up.
func (pp *peerProtector) capEnforcedNow() bool {
	_, enforced := pp.capState(pp.now())
	return enforced
}

// exemptFromCap reports whether pid is a validator per the resolver or carries validatorProtectionTag.
// Protection tags from other subsystems do not exempt a peer (see peerProtector).
func (pp *peerProtector) exemptFromCap(pid peer.ID) bool {
	if pp.resolver().GetPeerInfo(core.PeerID(pid)).PeerType == core.ValidatorPeer {
		return true
	}

	return pp.cm.IsProtected(pid, validatorProtectionTag)
}

// unprotectedInboundConns returns the inbound connections of connected peers that do not carry
// validatorProtectionTag. Unlike the pass it does not consult the resolver for every peer:
// connected validators are protected by the previous pass, and one that connected since is counted
// until the next pass, which can only make admission stricter for a few seconds.
func (pp *peerProtector) unprotectedInboundConns() map[network.Conn]struct{} {
	conns := make(map[network.Conn]struct{})
	for _, pid := range pp.net.Peers() {
		if pp.cm.IsProtected(pid, validatorProtectionTag) {
			continue
		}
		for _, conn := range inboundOnly(pp.net.ConnsToPeer(pid)) {
			conns[conn] = struct{}{}
		}
	}

	return conns
}

// inboundConnStats returns when the oldest inbound connection among conns was opened and how many
// inbound connections there are.
func inboundConnStats(conns []network.Conn) (time.Time, int) {
	inbound := inboundOnly(conns)
	return oldestOpened(inbound), len(inbound)
}

// inboundOnly returns the inbound connections among conns.
func inboundOnly(conns []network.Conn) []network.Conn {
	inbound := make([]network.Conn, 0, len(conns))
	for _, conn := range conns {
		if conn.Stat().Direction == network.DirInbound {
			inbound = append(inbound, conn)
		}
	}

	return inbound
}

// oldestOpened returns when the oldest of conns was opened, or the zero time when conns is empty.
func oldestOpened(conns []network.Conn) time.Time {
	var first time.Time
	for i, conn := range conns {
		if opened := conn.Stat().Opened; i == 0 || opened.Before(first) {
			first = opened
		}
	}

	return first
}

// start runs a pass immediately and then every interval until ctx is done.
func (pp *peerProtector) start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			res := pp.runPass()
			logArgs := []interface{}{
				"validators protected", res.Validators,
				"observers tagged", res.Observers,
				"unknown", res.Unknown,
				"unprotected", res.Unprotected,
				"unprotected inbound unknown peers", res.UnprotectedInboundUnknown,
				"unprotected inbound observer peers", res.UnprotectedInboundObservers,
				"unprotected inbound conns", res.UnprotectedInboundConns,
				"max unprotected inbound conns", pp.maxUnprotectedInboundConns,
				"closed peers", res.Closed,
				"closed conns", res.ClosedConns,
				"cap not armed", res.NotArmed,
				"cap in warm-up", res.InWarmUp,
			}
			if res.Closed > 0 {
				log.Info("peer protector closed unprotected inbound peers over the cap", logArgs...)
			} else {
				log.Debug("peer protector pass", logArgs...)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
