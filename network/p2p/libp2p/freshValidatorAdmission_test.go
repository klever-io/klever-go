//go:build p2pexperiment

package libp2p_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// This is a measurement, not a regression test, so it only builds with the p2pexperiment tag:
//
//	go test -tags p2pexperiment ./network/p2p/libp2p/ -run TestExperiment_FreshValidatorAdmission -v -count=1 -timeout 90m
//
// It starts 1 seed + N regular nodes (kad-dht discovery, full inbound cap for unknown peers),
// then a validator with a fresh peer ID bootstrapped to the seed only, and times how long it takes
// to get connected. The validator's heartbeat identification is simulated by adding it to a shared
// "identified" set that every existing node's PeerShardResolver consults.
//
// Limits: localhost only, and with N=5 every DHT lookup returns every node (bucket size 20), so all
// existing nodes dial the validator on the same refresh. On a network of a few hundred nodes only
// the validator's ~20 DHT neighbours dial it before it is identified; the other links wait for its
// heartbeat and its own dials.

const (
	expNumExisting = 5
	expRuns        = 3
	expTimeout     = 3 * time.Minute
	expDirTimeout  = 2 * time.Second
)

type expIntervals struct {
	name           string
	refreshSec     uint32
	routingTableSc uint32
}

type expResult struct {
	t1, t2, control time.Duration
	firstDir        string
	timedOut        string
}

type identifiedSet struct {
	mut sync.RWMutex
	set map[peer.ID]struct{}
}

func (s *identifiedSet) add(id peer.ID) {
	s.mut.Lock()
	s.set[id] = struct{}{}
	s.mut.Unlock()
}

func (s *identifiedSet) has(id peer.ID) bool {
	s.mut.RLock()
	defer s.mut.RUnlock()
	_, ok := s.set[id]
	return ok
}

type existingNode struct {
	mes interface {
		ID() core.PeerID
		Bootstrap() error
		Close() error
		Host() libp2p.ConnectableHost
		SetPeerShardResolver(psp p2p.PeerShardResolver) error
	}
}

// peerLabels names the peers of a scenario, for the logs of who holds an existing node's inbound
// connections.
type peerLabels struct {
	mut    sync.Mutex
	labels map[peer.ID]string
}

func (l *peerLabels) set(id peer.ID, label string) {
	l.mut.Lock()
	l.labels[id] = label
	l.mut.Unlock()
}

func (l *peerLabels) get(id peer.ID) string {
	l.mut.Lock()
	defer l.mut.Unlock()
	if label, ok := l.labels[id]; ok {
		return label
	}
	return "other:" + id.String()[:8]
}

// connEvents coalesces connection notifications of several networks into one channel, so the waits
// below recheck their condition on every connect or disconnect instead of polling.
type connEvents struct {
	ch     chan struct{}
	bundle *network.NotifyBundle
}

func newConnEvents() *connEvents {
	ce := &connEvents{ch: make(chan struct{}, 1)}
	signal := func(network.Network, network.Conn) {
		select {
		case ce.ch <- struct{}{}:
		default:
		}
	}
	ce.bundle = &network.NotifyBundle{ConnectedF: signal, DisconnectedF: signal}
	return ce
}

func (ce *connEvents) watch(n network.Network) {
	n.Notify(ce.bundle)
}

// waitFor returns how long after start cond became true, or false once timeout has passed since
// start. cond is checked once up front and then after every connection event.
func (ce *connEvents) waitFor(start time.Time, timeout time.Duration, cond func() bool) (time.Duration, bool) {
	timer := time.NewTimer(time.Until(start.Add(timeout)))
	defer timer.Stop()
	for {
		if cond() {
			return time.Since(start), true
		}
		select {
		case <-ce.ch:
		case <-timer.C:
			return timeout, false
		}
	}
}

// protectorSettleTimeout covers one protector pass (10s) with margin.
const protectorSettleTimeout = 30 * time.Second

// waitTicking checks cond every 100ms until it holds or timeout passes. It is for state that no
// connection event announces, such as the protector's tags.
func waitTicking(timeout time.Duration, cond func() bool) (time.Duration, bool) {
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		if cond() {
			return time.Since(start), true
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			return timeout, false
		}
	}
}

func expArgs(isSeed bool, initial []string, iv expIntervals) libp2p.ArgsNetworkMessenger {
	arg := createMockNetworkArgs()
	arg.IsSeedNode = isSeed
	arg.P2pConfig.KadDhtPeerDiscovery = config.KadDhtPeerDiscoveryConfig{
		Enabled:                          true,
		RefreshIntervalInSec:             iv.refreshSec,
		ProtocolID:                       "/klv/kad/0.0.1",
		InitialPeerList:                  initial,
		BucketSize:                       100,
		RoutingTableRefreshIntervalInSec: iv.routingTableSc,
	}
	arg.P2pConfig.Sharding = config.ShardingConfig{
		TargetPeerCount:         75,
		MaxIntraShardValidators: 25,
		MaxIntraShardObservers:  15,
		Type:                    p2p.NilListSharder,
	}
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 1}
	return arg
}

func dirString(d network.Direction) string {
	switch d {
	case network.DirInbound:
		return "inbound"
	case network.DirOutbound:
		return "outbound"
	}
	return "unknown"
}

func connectedCount(n network.Network, ids []peer.ID) int {
	count := 0
	for _, id := range ids {
		if n.Connectedness(id) == network.Connected {
			count++
		}
	}
	return count
}

// inboundHolders lists the peers that hold inbound connections on n and are not identified, the
// ones the cap counts.
func inboundHolders(n network.Network, identified *identifiedSet, labels *peerLabels) string {
	var holders []string
	for _, pid := range n.Peers() {
		if identified.has(pid) {
			continue
		}
		for _, conn := range n.ConnsToPeer(pid) {
			if conn.Stat().Direction == network.DirInbound {
				holders = append(holders, fmt.Sprintf("%s(opened %s ago)",
					labels.get(pid), time.Since(conn.Stat().Opened).Round(time.Millisecond)))
			}
		}
	}
	sort.Strings(holders)
	return "[" + strings.Join(holders, ", ") + "]"
}

func runFreshValidatorScenario(t *testing.T, iv expIntervals, control bool) expResult {
	identified := &identifiedSet{set: map[peer.ID]struct{}{}}
	labels := &peerLabels{labels: map[peer.ID]string{}}

	seed, err := libp2p.NewNetworkMessenger(expArgs(true, nil, iv))
	require.NoError(t, err)
	defer func() { _ = seed.Close() }()
	require.NoError(t, seed.Bootstrap())
	seedAddr := seed.Addresses()[0]
	labels.set(peer.ID(seed.ID()), "seed")

	var existingMes []*existingNode
	for i := 0; i < expNumExisting; i++ {
		m, err := libp2p.NewNetworkMessenger(expArgs(false, []string{seedAddr}, iv))
		require.NoError(t, err)
		defer func() { _ = m.Close() }()
		identified.add(peer.ID(m.ID()))
		labels.set(peer.ID(m.ID()), fmt.Sprintf("existing%d", i))
		existingMes = append(existingMes, &existingNode{mes: m})
	}
	for _, e := range existingMes {
		require.NoError(t, e.mes.Bootstrap())
	}
	ids := make([]peer.ID, 0, expNumExisting)
	for _, e := range existingMes {
		ids = append(ids, peer.ID(e.mes.ID()))
	}

	events := newConnEvents()
	for _, e := range existingMes {
		events.watch(e.mes.Host().Network())
	}

	// phase 1 (cap not armed yet): wait for the existing nodes to form a full mesh via seed/kad
	meshStart := time.Now()
	meshTime, ok := events.waitFor(meshStart, 3*time.Minute, func() bool {
		for _, e := range existingMes {
			if connectedCount(e.mes.Host().Network(), ids) < expNumExisting-1 {
				return false
			}
		}
		return true
	})
	require.True(t, ok, "existing nodes did not form a full mesh")
	t.Logf("  existing mesh formed in %v", meshTime)

	// phase 2: arm the cap and fill it with one unknown inbound attacker per node
	for _, e := range existingMes {
		require.NoError(t, e.mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
			GetPeerInfoCalled: func(pid core.PeerID) core.P2PPeerInfo {
				if identified.has(peer.ID(pid)) {
					return core.P2PPeerInfo{PeerType: core.ValidatorPeer}
				}
				return core.P2PPeerInfo{PeerType: core.UnknownPeer}
			},
		}))
	}
	// Until the protector's first pass tags them, the other existing nodes' inbound links count
	// against the cap like unknown peers, so a cap of 1 looks full while no unknown peer holds it,
	// and it frees up at that pass. Wait for every existing node to protect the others first, so the
	// attackers below are what fills the cap.
	_, ok = waitTicking(protectorSettleTimeout, func() bool {
		for _, e := range existingMes {
			cm := e.mes.Host().ConnManager()
			for _, id := range ids {
				if id != e.mes.Host().ID() && !cm.IsProtected(id, libp2p.ValidatorProtectionTag) {
					return false
				}
			}
		}
		return true
	})
	require.True(t, ok, "the existing nodes did not protect each other")

	for i, e := range existingMes {
		attacker := newLocalPeerHost(t)
		labels.set(attacker.ID(), fmt.Sprintf("attacker%d", i))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := attacker.Connect(ctx, peer.AddrInfo{ID: e.mes.Host().ID(), Addrs: e.mes.Host().Addrs()})
		cancel()
		require.NoError(t, err, "attacker%d must take existing%d's only unknown inbound slot", i, i)
	}

	// sanity: the cap really refuses an unknown dialer on every existing node, and an attacker holds it
	probe := newLocalPeerHost(t)
	labels.set(probe.ID(), "probe")
	for i, e := range existingMes {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = probe.Connect(ctx, peer.AddrInfo{ID: e.mes.Host().ID(), Addrs: e.mes.Host().Addrs()})
		cancel()
		require.NotEqual(t, network.Connected, e.mes.Host().Network().Connectedness(probe.ID()),
			"the cap must be full on every existing node")
		holders := inboundHolders(e.mes.Host().Network(), identified, labels)
		require.Contains(t, holders, fmt.Sprintf("attacker%d", i), "existing%d's cap must be held by its attacker", i)
		t.Logf("  existing%d cap held by %s", i, holders)
	}

	// phase 3: fresh validator, bootstrapped to the seed only. Its own cap is never armed.
	vArgs := expArgs(false, []string{seedAddr}, iv)
	vMes, err := libp2p.NewNetworkMessenger(vArgs)
	require.NoError(t, err)
	defer func() { _ = vMes.Close() }()
	vID := peer.ID(vMes.ID())
	labels.set(vID, "V")
	if control {
		identified.add(vID)
	}
	vNet := vMes.Host().Network()
	events.watch(vNet)

	res := expResult{}
	start := time.Now()
	require.NoError(t, vMes.Bootstrap())

	if control {
		res.control, ok = events.waitFor(start, expTimeout, func() bool {
			return connectedCount(vNet, ids) == expNumExisting
		})
		if !ok {
			res.timedOut = fmt.Sprintf("control: only %d/%d connected", connectedCount(vNet, ids), expNumExisting)
		}
		return res
	}

	// t1: first link to an existing node that both sides hold. V's own dial can show up on V's side
	// for a moment before the existing node's gater refuses it, so V's side alone is not enough.
	res.t1, ok = events.waitFor(start, expTimeout, func() bool {
		for _, e := range existingMes {
			eNet := e.mes.Host().Network()
			if len(eNet.ConnsToPeer(vID)) > 0 && len(vNet.ConnsToPeer(eNet.LocalPeer())) > 0 {
				return true
			}
		}
		return false
	})
	if !ok {
		res.timedOut = "t1: never connected to any existing node"
		return res
	}

	// give the other side up to expDirTimeout to register the same link before reading its direction
	var dirs []string
	_, _ = events.waitFor(time.Now(), expDirTimeout, func() bool {
		dirs = dirs[:0]
		for i, e := range existingMes {
			eNet := e.mes.Host().Network()
			conns := eNet.ConnsToPeer(vID)
			vConns := vNet.ConnsToPeer(eNet.LocalPeer())
			if len(conns) == 0 || len(vConns) == 0 {
				continue
			}
			dir := fmt.Sprintf("existing%d side=%s / V side=%s", i,
				dirString(conns[0].Stat().Direction), dirString(vConns[0].Stat().Direction))
			if conns[0].Stat().Direction == network.DirInbound {
				// the cap admitted an unidentified peer: record who else holds its inbound slots
				dir += " inbound holders=" + inboundHolders(eNet, identified, labels)
			}
			dirs = append(dirs, dir)
		}
		return len(dirs) > 0
	})
	res.firstDir = strings.Join(dirs, "; ")

	// t2: V gets identified network-wide (heartbeat seen), time until it reaches all N
	identified.add(vID)
	t2Start := time.Now()
	res.t2, ok = events.waitFor(t2Start, expTimeout, func() bool {
		return connectedCount(vNet, ids) == expNumExisting
	})
	if !ok {
		res.timedOut = fmt.Sprintf("t2: only %d/%d connected after timeout", connectedCount(vNet, ids), expNumExisting)
	}
	return res
}

func summarize(ds []time.Duration) string {
	if len(ds) == 0 {
		return "n/a"
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return fmt.Sprintf("min=%v median=%v max=%v", s[0].Round(time.Millisecond), s[len(s)/2].Round(time.Millisecond), s[len(s)-1].Round(time.Millisecond))
}

func TestExperiment_FreshValidatorAdmission(t *testing.T) {
	defer libp2p.SetInboundCapWarmUp(0)()

	intervals := []expIntervals{
		{name: "fast (refresh 1s, routingTableRefresh 5s)", refreshSec: 1, routingTableSc: 5},
		{name: "shipped (refresh 10s, routingTableRefresh 300s)", refreshSec: 10, routingTableSc: 300},
	}
	for _, iv := range intervals {
		for _, control := range []bool{false, true} {
			var t1s, t2s, ctrls []time.Duration
			var notes []string
			label := "fresh validator"
			if control {
				label = "control (V identified from start)"
			}
			for r := 0; r < expRuns; r++ {
				res := runFreshValidatorScenario(t, iv, control)
				t.Logf("[%s | %s] run %d: t1=%v t2=%v control=%v firstLink={%s} timedOut=%q",
					iv.name, label, r+1, res.t1, res.t2, res.control, res.firstDir, res.timedOut)
				if control {
					ctrls = append(ctrls, res.control)
				} else {
					t1s = append(t1s, res.t1)
					if res.t2 > 0 {
						t2s = append(t2s, res.t2)
					}
				}
				if res.timedOut != "" {
					notes = append(notes, res.timedOut)
				}
			}
			if control {
				t.Logf("SUMMARY [%s | %s] N=%d runs=%d connect-to-all: %s timeouts=%v", iv.name, label, expNumExisting, expRuns, summarize(ctrls), notes)
			} else {
				t.Logf("SUMMARY [%s | %s] N=%d runs=%d t1: %s | t2: %s timeouts=%v", iv.name, label, expNumExisting, expRuns, summarize(t1s), summarize(t2s), notes)
			}
		}
	}
}
