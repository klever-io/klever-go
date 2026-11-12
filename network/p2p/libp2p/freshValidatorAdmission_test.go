package libp2p_test

import (
	"context"
	"fmt"
	"os"
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

// This is a measurement, not a regression test. Run it with:
//
//	KLV_P2P_EXPERIMENT=1 go test ./network/p2p/libp2p/ -run TestExperiment_FreshValidatorAdmission -v -count=1 -timeout 90m
//
// It starts 1 seed + N regular nodes (kad-dht discovery, full inbound cap for unknown peers),
// then a validator with a fresh peer ID bootstrapped to the seed only, and times how long it takes
// to get connected. The validator's heartbeat identification is simulated by adding it to a shared
// "identified" set that every existing node's PeerShardResolver consults.

const (
	expNumExisting = 5
	expRuns        = 3
	expTimeout     = 3 * time.Minute
	expPoll        = 100 * time.Millisecond
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

func connectedCount(v interface{ ID() core.PeerID }, vHost network.Network, existing []peer.ID) int {
	n := 0
	for _, id := range existing {
		if vHost.Connectedness(id) == network.Connected {
			n++
		}
	}
	return n
}

func runFreshValidatorScenario(t *testing.T, iv expIntervals, control bool) expResult {
	identified := &identifiedSet{set: map[peer.ID]struct{}{}}

	seed, err := libp2p.NewNetworkMessenger(expArgs(true, nil, iv))
	require.NoError(t, err)
	defer func() { _ = seed.Close() }()
	require.NoError(t, seed.Bootstrap())
	seedAddr := seed.Addresses()[0]

	existing := make([]interface {
		Close() error
	}, 0)
	var existingMes []*existingNode
	for i := 0; i < expNumExisting; i++ {
		m, err := libp2p.NewNetworkMessenger(expArgs(false, []string{seedAddr}, iv))
		require.NoError(t, err)
		existing = append(existing, m)
		defer func() { _ = m.Close() }()
		identified.add(peer.ID(m.ID()))
		existingMes = append(existingMes, &existingNode{mes: m})
	}
	for _, e := range existingMes {
		require.NoError(t, e.mes.Bootstrap())
	}
	ids := make([]peer.ID, 0, expNumExisting)
	for _, e := range existingMes {
		ids = append(ids, peer.ID(e.mes.ID()))
	}

	// phase 1 (cap not armed yet): wait for the existing nodes to form a full mesh via seed/kad
	meshStart := time.Now()
	require.Eventually(t, func() bool {
		for _, e := range existingMes {
			if connectedCount(nil, e.mes.Host().Network(), ids) < expNumExisting-1 {
				return false
			}
		}
		return true
	}, 3*time.Minute, expPoll, "existing nodes did not form a full mesh")
	t.Logf("  existing mesh formed in %v", time.Since(meshStart))

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
	for _, e := range existingMes {
		// best effort: the node's cap may already be full from inbound links of non-identified peers
		// (e.g. the seed), in which case the attacker is refused; the sanity check below proves fullness.
		attacker := newLocalPeerHost(t)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = attacker.Connect(ctx, peer.AddrInfo{ID: e.mes.Host().ID(), Addrs: e.mes.Host().Addrs()})
		cancel()
	}

	// sanity: the cap really refuses an unknown dialer on every existing node
	probe := newLocalPeerHost(t)
	for _, e := range existingMes {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = probe.Connect(ctx, peer.AddrInfo{ID: e.mes.Host().ID(), Addrs: e.mes.Host().Addrs()})
		cancel()
		require.NotEqual(t, network.Connected, e.mes.Host().Network().Connectedness(probe.ID()),
			"the cap must be full on every existing node")
	}

	// phase 3: fresh validator, bootstrapped to the seed only. Its own cap is never armed.
	vArgs := expArgs(false, []string{seedAddr}, iv)
	vMes, err := libp2p.NewNetworkMessenger(vArgs)
	require.NoError(t, err)
	defer func() { _ = vMes.Close() }()
	vID := peer.ID(vMes.ID())
	if control {
		identified.add(vID)
	}

	res := expResult{}
	start := time.Now()
	require.NoError(t, vMes.Bootstrap())
	vNet := vMes.Host().Network()

	if control {
		for time.Since(start) < expTimeout {
			if connectedCount(nil, vNet, ids) == expNumExisting {
				res.control = time.Since(start)
				return res
			}
			time.Sleep(expPoll)
		}
		res.control = expTimeout
		res.timedOut = fmt.Sprintf("control: only %d/%d connected", connectedCount(nil, vNet, ids), expNumExisting)
		return res
	}

	// t1: first link to any existing node
	for time.Since(start) < expTimeout {
		if connectedCount(nil, vNet, ids) > 0 {
			res.t1 = time.Since(start)
			break
		}
		time.Sleep(expPoll)
	}
	if res.t1 == 0 {
		res.t1 = expTimeout
		res.timedOut = "t1: never connected to any existing node"
		return res
	}
	var dirs []string
	// give the other side up to 2s to register the same link before reading its direction
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		dirs = dirs[:0]
		for _, e := range existingMes {
			conns := e.mes.Host().Network().ConnsToPeer(vID)
			vConns := vNet.ConnsToPeer(e.mes.Host().ID())
			if len(conns) > 0 && len(vConns) > 0 {
				dirs = append(dirs, fmt.Sprintf("existing side=%s / V side=%s",
					dirString(conns[0].Stat().Direction), dirString(vConns[0].Stat().Direction)))
			}
		}
		if len(dirs) > 0 {
			break
		}
	}
	res.firstDir = strings.Join(dirs, "; ")

	// t2: V gets identified network-wide (heartbeat seen), time until it reaches all N
	identified.add(vID)
	t2Start := time.Now()
	for time.Since(t2Start) < expTimeout {
		if connectedCount(nil, vNet, ids) == expNumExisting {
			res.t2 = time.Since(t2Start)
			return res
		}
		time.Sleep(expPoll)
	}
	res.t2 = expTimeout
	res.timedOut = fmt.Sprintf("t2: only %d/%d connected after timeout", connectedCount(nil, vNet, ids), expNumExisting)
	return res
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

func summarize(ds []time.Duration) string {
	if len(ds) == 0 {
		return "n/a"
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return fmt.Sprintf("min=%v median=%v max=%v", s[0].Round(time.Millisecond), s[len(s)/2].Round(time.Millisecond), s[len(s)-1].Round(time.Millisecond))
}

func TestExperiment_FreshValidatorAdmission(t *testing.T) {
	if os.Getenv("KLV_P2P_EXPERIMENT") != "1" {
		t.Skip("set KLV_P2P_EXPERIMENT=1 to run this measurement")
	}
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
