package libp2p

import (
	"context"
	"sort"
	"sync"
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
	// Closed is how many peers the pass closed and ClosedConns how many inbound connections they held.
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
	ClosePeer(p peer.ID) error
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
// are not validators, and closes whole peers regardless of grace until that sum is within the cap.
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
	// identificationWindow is unknownPeerIdentificationWindow, copied at construction so passes on
	// the protector's goroutine never read the package variable.
	identificationWindow time.Duration
	// mutArm guards armed and capEnforcedFrom apart from mut, so the connection gater can read
	// them without waiting for a pass to finish.
	mutArm          sync.RWMutex
	armed           bool
	capEnforcedFrom time.Time
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
		now:                        now,
		warmUp:                     warmUp,
		identificationWindow:       unknownPeerIdentificationWindow,
	}
}

// arm starts the warm-up after which the inbound cap is enforced. Only the first call counts.
func (pp *peerProtector) arm() {
	pp.mutArm.Lock()
	defer pp.mutArm.Unlock()

	if pp.armed {
		return
	}
	pp.armed = true
	pp.capEnforcedFrom = pp.now().Add(pp.warmUp)
}

// runPass classifies every connected peer once and updates protection and tags accordingly.
func (pp *peerProtector) runPass() peerProtectorPassResult {
	pp.mut.Lock()
	defer pp.mut.Unlock()

	var res peerProtectorPassResult
	peers := pp.net.Peers()
	resolver := pp.resolver()

	connected := make(map[peer.ID]struct{}, len(peers))
	protected := make(map[peer.ID]struct{}, len(pp.protected))
	tagged := make(map[peer.ID]struct{}, len(pp.tagged))
	peerTypes := make(map[peer.ID]core.P2PPeerType, len(peers))

	for _, pid := range peers {
		connected[pid] = struct{}{}
		peerType := resolver.GetPeerInfo(core.PeerID(pid)).PeerType
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

	// Peers protected last pass that are no longer connected validators: demoted to observer or
	// unknown, or disconnected (whose entry would otherwise leak).
	for pid := range pp.protected {
		if _, ok := protected[pid]; ok {
			continue
		}
		pp.cm.Unprotect(pid, validatorProtectionTag)
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

// closeExcessInbound closes candidates in order until the inbound connections fit in the cap.
func (pp *peerProtector) closeExcessInbound(candidates []inboundCandidate, total int, res *peerProtectorPassResult) {
	for _, c := range candidates {
		if total <= pp.maxUnprotectedInboundConns {
			return
		}
		err := pp.net.ClosePeer(c.pid)
		if err != nil {
			log.Debug("peer protector: closing unprotected inbound peer", "pid", c.pid.String(), "error", err)
			continue
		}
		total -= c.inboundConns
		res.Closed++
		res.ClosedConns += c.inboundConns
	}
}

// capState reports whether the cap is armed and whether, at now, it is past its warm-up.
func (pp *peerProtector) capState(now time.Time) (armed bool, enforced bool) {
	pp.mutArm.RLock()
	defer pp.mutArm.RUnlock()

	return pp.armed, pp.armed && !now.Before(pp.capEnforcedFrom)
}

// admitInbound decides, for the connection gater, whether a new inbound connection from pid may be
// admitted. It refuses only when the cap is enforced, pid is neither a validator per the resolver
// nor carrying validatorProtectionTag, and the unprotected inbound connections already reach the cap. It
// does not take mut, so it never waits for a pass.
func (pp *peerProtector) admitInbound(pid peer.ID) bool {
	if !pp.capEnforcedNow() || pp.exemptFromCap(pid) {
		return true
	}

	count, _ := pp.unprotectedInboundConns()
	return count < pp.maxUnprotectedInboundConns
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

// unprotectedInboundConns counts, and returns, the inbound connections of connected peers that do
// not carry validatorProtectionTag. Unlike the pass it does not consult the resolver for every peer:
// connected validators are protected by the previous pass, and one that connected since is counted
// until the next pass, which can only make admission stricter for a few seconds.
func (pp *peerProtector) unprotectedInboundConns() (int, map[network.Conn]struct{}) {
	conns := make(map[network.Conn]struct{})
	for _, pid := range pp.net.Peers() {
		if pp.cm.IsProtected(pid, validatorProtectionTag) {
			continue
		}
		for _, conn := range pp.net.ConnsToPeer(pid) {
			if conn.Stat().Direction == network.DirInbound {
				conns[conn] = struct{}{}
			}
		}
	}

	return len(conns), conns
}

// inboundConnStats returns when the oldest inbound connection among conns was opened and how many
// inbound connections there are.
func inboundConnStats(conns []network.Conn) (time.Time, int) {
	var first time.Time
	count := 0
	for _, conn := range conns {
		stat := conn.Stat()
		if stat.Direction != network.DirInbound {
			continue
		}
		if count == 0 || stat.Opened.Before(first) {
			first = stat.Opened
		}
		count++
	}

	return first, count
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
