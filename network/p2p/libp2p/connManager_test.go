package libp2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/klever-io/klever-go/tools/check"
	golibp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	p2pConnMgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnManagerSettings_ZeroMeansDefault(t *testing.T) {
	low, high, grace := libp2p.ConnManagerSettings(config.ConnectionManagerConfig{})
	assert.Equal(t, 100, low)
	assert.Equal(t, 300, high)
	assert.Equal(t, time.Minute, grace)

	low, high, grace = libp2p.ConnManagerSettings(config.ConnectionManagerConfig{HighWater: 400})
	assert.Equal(t, 100, low, "an unset lowWater keeps its default")
	assert.Equal(t, 400, high)
	assert.Equal(t, time.Minute, grace)

	_, _, grace = libp2p.ConnManagerSettings(config.ConnectionManagerConfig{GracePeriodInSec: 5})
	assert.Equal(t, 5*time.Second, grace)
}

func TestValidateConnectionManagerConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.ConnectionManagerConfig
		wantErr string
	}{
		{name: "zero is the default", cfg: config.ConnectionManagerConfig{}},
		{name: "explicit defaults", cfg: config.ConnectionManagerConfig{LowWater: 100, HighWater: 300, GracePeriodInSec: 60}},
		{name: "only highWater above the default low", cfg: config.ConnectionManagerConfig{HighWater: 101}},
		{name: "upper bounds", cfg: config.ConnectionManagerConfig{LowWater: 99999, HighWater: 100000, GracePeriodInSec: 3600}},
		{name: "lowWater above the bound", cfg: config.ConnectionManagerConfig{LowWater: 100001, HighWater: 100000}, wantErr: "must not exceed 100000"},
		{name: "highWater above the bound", cfg: config.ConnectionManagerConfig{HighWater: 100001}, wantErr: "must not exceed 100000"},
		{name: "grace above one hour", cfg: config.ConnectionManagerConfig{GracePeriodInSec: 3601}, wantErr: "gracePeriodInSec must not exceed 3600"},
		{name: "inbound cap at the bound", cfg: config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 100000}},
		{name: "negative inbound cap", cfg: config.ConnectionManagerConfig{MaxUnprotectedInboundConns: -1}, wantErr: "maxUnprotectedInboundConns must be non-negative"},
		{name: "inbound cap above the bound", cfg: config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 100001}, wantErr: "maxUnprotectedInboundConns must not exceed 100000"},
		{name: "negative lowWater", cfg: config.ConnectionManagerConfig{LowWater: -1}, wantErr: "lowWater must be non-negative"},
		{name: "negative highWater", cfg: config.ConnectionManagerConfig{HighWater: -1}, wantErr: "highWater must be non-negative"},
		{name: "negative grace", cfg: config.ConnectionManagerConfig{GracePeriodInSec: -1}, wantErr: "gracePeriodInSec must be non-negative"},
		{name: "low equal to high", cfg: config.ConnectionManagerConfig{LowWater: 100, HighWater: 100}, wantErr: "must be lower than highWater"},
		{name: "low above high", cfg: config.ConnectionManagerConfig{LowWater: 200, HighWater: 100}, wantErr: "must be lower than highWater"},
		{name: "only lowWater, above the default high", cfg: config.ConnectionManagerConfig{LowWater: 400}, wantErr: "must be lower than highWater"},
		{name: "only highWater, below the default low", cfg: config.ConnectionManagerConfig{HighWater: 50}, wantErr: "must be lower than highWater"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := libp2p.ValidateConnectionManagerConfig(tc.cfg)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Contains(t, err.Error(), "p2p.connectionManager")
		})
	}
}

func TestNewNetworkMessenger_InvalidConnectionManagerConfigFailsStartup(t *testing.T) {
	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{LowWater: 300, HighWater: 300}

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.Error(t, err)
	assert.True(t, check.IfNil(mes))
	assert.Contains(t, err.Error(), "p2p.connectionManager")
}

func TestEffectiveResourceManagerConfig_RegularNodeWithoutSectionGetsScaled(t *testing.T) {
	got := libp2p.EffectiveResourceManagerConfig(config.ResourceManagerConfig{}, false)
	assert.Equal(t, config.ResourceManagerStrategyScaled, got.Strategy)
	assert.Equal(t, 4096, got.ScaledMemoryMiB)
	assert.Zero(t, got.MaxConnsPerIPv4)
	assert.Zero(t, got.MaxConnsPerIPv6Subnet)
	assert.Zero(t, got.ConnRatePerSec)
	assert.Zero(t, got.ConnRateBurst)
}

func TestEffectiveResourceManagerConfig_RegularNodeKeepsConfiguredScaledMemory(t *testing.T) {
	// scaledMemoryMiB without a strategy used to be replaced by 4096 silently.
	got := libp2p.EffectiveResourceManagerConfig(config.ResourceManagerConfig{ScaledMemoryMiB: 16384}, false)
	assert.Equal(t, config.ResourceManagerStrategyScaled, got.Strategy)
	assert.Equal(t, 16384, got.ScaledMemoryMiB)

	seed := libp2p.EffectiveResourceManagerConfig(config.ResourceManagerConfig{ScaledMemoryMiB: 16384}, true)
	assert.Equal(t, config.ResourceManagerStrategyDefault, seed.Strategy, "seeds keep libp2p auto-scale for \"\"")
}

func TestEffectiveResourceManagerConfig_LeavesOtherCasesAlone(t *testing.T) {
	cases := []struct {
		name   string
		cfg    config.ResourceManagerConfig
		isSeed bool
	}{
		{name: "seed without section", cfg: config.ResourceManagerConfig{}, isSeed: true},
		{name: "explicit libp2p default", cfg: config.ResourceManagerConfig{Strategy: config.ResourceManagerStrategyLibp2pDefault}},
		{name: "null", cfg: config.ResourceManagerConfig{Strategy: config.ResourceManagerStrategyNull}},
		{name: "scaled", cfg: config.ResourceManagerConfig{Strategy: config.ResourceManagerStrategyScaled, ScaledMemoryMiB: 1024}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.cfg, libp2p.EffectiveResourceManagerConfig(tc.cfg, tc.isSeed))
		})
	}
}

// expectedScaled4096SystemLimits is what the in-code default ("scaled", 4096 MiB) yields on this host.
func expectedScaled4096SystemLimits() rcmgr.ResourceLimits {
	limits := libp2p.ScaledLimits(libp2p.ClampToPhysicalMemory(4096<<20), libp2p.GetFDLimit()/2, false)
	return limits.ToPartialLimitConfig().System
}

// libp2pAutoScaledSystemLimits is what libp2p's DefaultResourceManager yields on this host.
func libp2pAutoScaledSystemLimits() rcmgr.ResourceLimits {
	limits := rcmgr.DefaultLimits
	golibp2p.SetDefaultServiceLimits(&limits)
	return limits.AutoScale().ToPartialLimitConfig().System
}

func TestBuildResourceManager_InCodeDefault(t *testing.T) {
	t.Run("regular node with strategy \"\" runs scaled 4096", func(t *testing.T) {
		arg := createMockNetworkArgs()
		strategy, inbound, total, hasLimits, err := libp2p.BuildResourceManagerStrategy(arg)
		require.NoError(t, err)
		assert.Equal(t, config.ResourceManagerStrategyScaled, strategy)
		require.True(t, hasLimits)
		expected := expectedScaled4096SystemLimits()
		assert.Equal(t, int(expected.ConnsInbound), inbound)
		assert.Equal(t, int(expected.Conns), total)
	})

	t.Run("seed with strategy \"\" keeps libp2p auto-scale", func(t *testing.T) {
		arg := createMockNetworkArgs()
		arg.IsSeedNode = true
		strategy, _, _, hasLimits, err := libp2p.BuildResourceManagerStrategy(arg)
		require.NoError(t, err)
		assert.Equal(t, config.ResourceManagerStrategyDefault, strategy)
		assert.False(t, hasLimits)
	})

	t.Run("explicit default keeps libp2p auto-scale on a regular node", func(t *testing.T) {
		arg := createMockNetworkArgs()
		arg.P2pConfig.ResourceManager.Strategy = config.ResourceManagerStrategyLibp2pDefault
		strategy, _, _, hasLimits, err := libp2p.BuildResourceManagerStrategy(arg)
		require.NoError(t, err)
		assert.Equal(t, config.ResourceManagerStrategyLibp2pDefault, strategy)
		assert.False(t, hasLimits)
	})

	t.Run("per-subnet knobs still require an explicit scaled strategy", func(t *testing.T) {
		// The in-code default must not make "" + knobs valid: validation runs on the config as written.
		arg := createMockNetworkArgs()
		arg.P2pConfig.ResourceManager.MaxConnsPerIPv4 = 16
		_, _, _, _, err := libp2p.BuildResourceManagerStrategy(arg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scaled")
	})
}

func TestNewNetworkMessenger_RegularNodeWithoutRMSectionGetsScaledLimits(t *testing.T) {
	arg := createMockNetworkArgs()

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	// libp2p's auto-scale budgets TotalRAM/8, so on a 32 GiB host it lands on the same limits as
	// "scaled" 4096; the strategy is what tells the two apart there.
	assert.Equal(t, config.ResourceManagerStrategyScaled,
		libp2p.EffectiveResourceManagerConfig(arg.P2pConfig.ResourceManager, arg.IsSeedNode).Strategy)

	var inbound, total int
	err = mes.Host().Network().ResourceManager().ViewSystem(func(scope network.ResourceScope) error {
		limit := scope.(rcmgr.ResourceScopeLimiter).Limit()
		inbound, total = limit.GetConnLimit(network.DirInbound), limit.GetConnTotalLimit()
		return nil
	})
	require.NoError(t, err)

	expected := expectedScaled4096SystemLimits()
	assert.Equal(t, int(expected.ConnsInbound), inbound)
	assert.Equal(t, int(expected.Conns), total)

	auto := libp2pAutoScaledSystemLimits()
	if auto.ConnsInbound == expected.ConnsInbound && auto.Conns == expected.Conns {
		t.Logf("libp2p auto-scale gives the same %d/%d on this host; only the strategy assertion distinguishes them",
			auto.ConnsInbound, auto.Conns)
	}
}

func TestNewNetworkMessenger_SeedWithoutRMSectionKeepsLibp2pAutoScale(t *testing.T) {
	arg := createMockNetworkArgs()
	arg.IsSeedNode = true

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	assert.Equal(t, config.ResourceManagerStrategyDefault,
		libp2p.EffectiveResourceManagerConfig(arg.P2pConfig.ResourceManager, arg.IsSeedNode).Strategy)

	inbound, total, hasLimits := libp2p.SystemConnLimits(mes.Host().Network().ResourceManager())
	require.True(t, hasLimits)
	auto := libp2pAutoScaledSystemLimits()
	assert.Equal(t, int(auto.ConnsInbound), inbound)
	assert.Equal(t, int(auto.Conns), total)
}

func TestConnManagerLimitWarnings(t *testing.T) {
	assert.Empty(t, libp2p.ConnManagerLimitWarnings(100, 300, 100, 320, 640), "the shipped defaults fit")
	assert.Empty(t, libp2p.ConnManagerLimitWarnings(100, 640, 100, 320, 640), "highWater equal to the total limit is still reachable")

	warnings := libp2p.ConnManagerLimitWarnings(100, 641, 100, 320, 640)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "never trims")

	warnings = libp2p.ConnManagerLimitWarnings(320, 400, 100, 320, 640)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "lowWater (320)")

	warnings = libp2p.ConnManagerLimitWarnings(100, 300, 320, 320, 640)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "maxUnprotectedInboundConns (320)")

	assert.Len(t, libp2p.ConnManagerLimitWarnings(200, 300, 150, 128, 256), 3,
		"an 8 GB host on libp2p auto-scale fits none of the three")

	// Explicit "default" on an 8 GB host: 128 inbound and the default cap of 100 leave 28 slots for
	// about 200 validators.
	warnings = libp2p.ConnManagerLimitWarnings(50, 250, 100, 128, 256)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "only 28 inbound slots are left for validators")
	assert.Empty(t, libp2p.ConnManagerLimitWarnings(100, 300, 120, 320, 640), "exactly 200 slots left is enough")
}

func TestScaledLimits_RegularNodesLimitInboundConnsPerPeer(t *testing.T) {
	regular := libp2p.ScaledLimits(4096<<20, 4096, false).ToPartialLimitConfig()
	seed := libp2p.ScaledLimits(4096<<20, 4096, true).ToPartialLimitConfig()

	assert.Equal(t, rcmgr.LimitVal(libp2p.RegularNodePeerConnsInbound), regular.PeerDefault.ConnsInbound)
	assert.Equal(t, rcmgr.LimitVal(4), regular.PeerDefault.ConnsInbound)
	assert.Equal(t, rcmgr.LimitVal(8), seed.PeerDefault.ConnsInbound, "seeds keep libp2p's 8")
	assert.Equal(t, seed.PeerDefault.Conns, regular.PeerDefault.Conns, "the per-peer total keeps libp2p's value")
	assert.Equal(t, seed.System, regular.System, "system limits are unchanged")
}

func peerInboundLimit(t *testing.T, mes interface {
	Host() libp2p.ConnectableHost
}) int {
	var limit int
	err := mes.Host().Network().ResourceManager().ViewPeer(peer.ID("some-peer"), func(scope network.PeerScope) error {
		limit = scope.(rcmgr.ResourceScopeLimiter).Limit().GetConnLimit(network.DirInbound)
		return nil
	})
	require.NoError(t, err)
	return limit
}

func TestNewNetworkMessenger_PerPeerInboundLimit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		isSeed bool
		rmCfg  config.ResourceManagerConfig
		want   int
	}{
		{name: "regular node, in-code default", want: 4},
		{name: "regular node, explicit scaled", rmCfg: config.ResourceManagerConfig{Strategy: config.ResourceManagerStrategyScaled, ScaledMemoryMiB: 4096}, want: 4},
		{name: "seed, explicit scaled", isSeed: true, rmCfg: config.ResourceManagerConfig{Strategy: config.ResourceManagerStrategyScaled, ScaledMemoryMiB: 4096}, want: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arg := createMockNetworkArgs()
			arg.IsSeedNode = tc.isSeed
			arg.P2pConfig.ResourceManager = tc.rmCfg

			mes, err := libp2p.NewNetworkMessenger(arg)
			require.NoError(t, err)
			defer func() { _ = mes.Close() }()

			assert.Equal(t, tc.want, peerInboundLimit(t, mes))
		})
	}
}

func TestMaxUnprotectedInboundConns_ZeroMeansDefault(t *testing.T) {
	assert.Equal(t, 100, libp2p.MaxUnprotectedInboundConns(config.ConnectionManagerConfig{}))
	assert.Equal(t, 7, libp2p.MaxUnprotectedInboundConns(config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 7}))
}

func TestSystemConnLimits(t *testing.T) {
	_, _, ok := libp2p.SystemConnLimits(&network.NullResourceManager{})
	assert.False(t, ok, "the null ResourceManager has no limits")

	limits := libp2p.ScaledLimits(4096<<20, 4096, false)
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits))
	require.NoError(t, err)
	defer func() { _ = rm.Close() }()

	inbound, total, ok := libp2p.SystemConnLimits(rm)
	require.True(t, ok)
	assert.Equal(t, 320, inbound)
	assert.Equal(t, 640, total)
}

func newLocalPeerHost(t *testing.T) host.Host {
	h, err := golibp2p.New(
		golibp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		golibp2p.DisableRelay(),
		golibp2p.ResourceManager(&network.NullResourceManager{}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func connectToMessenger(t *testing.T, h host.Host, mes interface {
	Host() libp2p.ConnectableHost
}) {
	target := peer.AddrInfo{ID: mes.Host().ID(), Addrs: mes.Host().Addrs()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.Connect(ctx, target))
	require.Eventually(t, func() bool {
		return mes.Host().Network().Connectedness(h.ID()) == network.Connected
	}, 5*time.Second, 10*time.Millisecond)
}

// TestNetworkMessenger_TrimEvictsUnprotectedPeersBelowOldLowWater runs the whole path on real
// localhost hosts: the messenger's BasicConnMgr, its peer protector fed by the PeerShardResolver,
// and a trim. It reproduces, scaled down from the shipped defaults, the case the default lowWater
// must handle: protected validators plus fewer unprotected inbound peers than the old lowWater
// (250) but more than the current one, with the total above highWater. BasicConnMgr only counts
// unprotected, out-of-grace peers against lowWater, so with lowWater 250 that trim closes nothing
// and the unprotected peers keep their inbound slots. Validators are tagged with a negative value
// so that, were they not protected, they would be the first peers the trim closes.
func TestNetworkMessenger_TrimEvictsUnprotectedPeersBelowOldLowWater(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	defer libp2p.SetDefaultConnMgrGracePeriod(0)()

	const scale = 50
	const oldLowWater = 250 / scale
	lowWater := libp2p.DefaultConnMgrLowWater / scale
	highWater := libp2p.DefaultConnMgrHighWater / scale
	numUnprotected := oldLowWater - 1               // fewer than the old lowWater
	numValidators := highWater - numUnprotected + 1 // total above highWater
	t.Logf("lowWater=%d highWater=%d validators=%d unprotected=%d", lowWater, highWater, numValidators, numUnprotected)

	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{LowWater: lowWater, HighWater: highWater}

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	cm, ok := mes.Host().ConnManager().(*p2pConnMgr.BasicConnMgr)
	require.True(t, ok)
	require.Zero(t, cm.GetInfo().GracePeriod)

	validators := make(map[peer.ID]struct{})
	var all []host.Host
	for i := 0; i < numValidators+numUnprotected; i++ {
		h := newLocalPeerHost(t)
		connectToMessenger(t, h, mes) // inbound for the messenger
		if i < numValidators {
			validators[h.ID()] = struct{}{}
			cm.TagPeer(h.ID(), "test-low-value", -100)
		}
		all = append(all, h)
	}
	require.Greater(t, cm.GetInfo().ConnCount, highWater)

	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(pid core.PeerID) core.P2PPeerInfo {
			if _, isValidator := validators[peer.ID(pid)]; isValidator {
				return core.P2PPeerInfo{PeerType: core.ValidatorPeer}
			}
			return core.P2PPeerInfo{PeerType: core.UnknownPeer}
		},
	}))

	res, ok := mes.RunPeerProtectorPass()
	require.True(t, ok, "regular nodes must run a peer protector")
	assert.Equal(t, numValidators, res.Validators)
	assert.Equal(t, numUnprotected, res.Unknown)

	cm.TrimOpenConns(context.Background())

	countUnprotectedConnected := func() int {
		n := 0
		for _, h := range all {
			_, isValidator := validators[h.ID()]
			if !isValidator && mes.Host().Network().Connectedness(h.ID()) == network.Connected {
				n++
			}
		}
		return n
	}
	// Trim closes unprotected peers down to lowWater; protected ones are not counted.
	assert.Eventually(t, func() bool { return countUnprotectedConnected() == lowWater }, 5*time.Second, 10*time.Millisecond,
		"the trim should close unprotected peers down to lowWater=%d, %d still connected", lowWater, countUnprotectedConnected())

	for pid := range validators {
		assert.Equal(t, network.Connected, mes.Host().Network().Connectedness(pid), "validator %s must survive the trim", pid)
	}
}

// TestNetworkMessenger_InboundCapClosesPeersTheGracePeriodShields reproduces identity churn: every
// unprotected inbound peer is inside the grace period, so a trim closes nothing, yet they exceed
// the cap. Once the cap is armed, one protector pass must close the excess and keep the oldest
// peers. The identification window is zeroed so the peers count as unknown peers that had their
// chance to identify, which are closed newest first.
func TestNetworkMessenger_InboundCapClosesPeersTheGracePeriodShields(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}

	defer libp2p.SetInboundCapWarmUp(0)()
	defer libp2p.SetUnknownPeerIdentificationWindow(0)()

	const maxUnprotected, numPeers = 2, 5
	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{
		LowWater:                   1,
		HighWater:                  2,
		GracePeriodInSec:           3600,
		MaxUnprotectedInboundConns: maxUnprotected,
	}

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	var peersInConnectOrder []host.Host
	for i := 0; i < numPeers; i++ {
		h := newLocalPeerHost(t)
		connectToMessenger(t, h, mes) // inbound for the messenger
		peersInConnectOrder = append(peersInConnectOrder, h)
		time.Sleep(5 * time.Millisecond) // distinct Opened timestamps
	}

	isConnected := func(h host.Host) bool {
		return mes.Host().Network().Connectedness(h.ID()) == network.Connected
	}
	countConnected := func() int {
		n := 0
		for _, h := range peersInConnectOrder {
			if isConnected(h) {
				n++
			}
		}
		return n
	}

	mes.Host().ConnManager().TrimOpenConns(context.Background())
	require.Equal(t, numPeers, countConnected(), "inside the grace period a trim closes nothing")

	res, ok := mes.RunPeerProtectorPass()
	require.True(t, ok)
	require.True(t, res.NotArmed, "the cap waits for the node's resolver")
	require.Zero(t, res.Closed)

	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(_ core.PeerID) core.P2PPeerInfo { return core.P2PPeerInfo{PeerType: core.UnknownPeer} },
	}))
	res, ok = mes.RunPeerProtectorPass()
	require.True(t, ok)
	assert.Equal(t, numPeers, res.UnprotectedInboundUnknown)
	assert.Equal(t, numPeers-maxUnprotected, res.Closed)

	require.Eventually(t, func() bool { return countConnected() == maxUnprotected }, 5*time.Second, 10*time.Millisecond)
	for i, h := range peersInConnectOrder {
		assert.Equal(t, i < maxUnprotected, isConnected(h), "peer %d: the oldest peers are kept, the newest closed", i)
	}
}

// TestNetworkMessenger_InboundCapNotArmedWithoutResolver checks the cap stays disarmed, even past
// the warm-up, until SetPeerShardResolver installs the node's resolver.
func TestNetworkMessenger_InboundCapNotArmedWithoutResolver(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	defer libp2p.SetInboundCapWarmUp(0)()

	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 1}

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	for i := 0; i < 3; i++ {
		connectToMessenger(t, newLocalPeerHost(t), mes)
	}

	res, ok := mes.RunPeerProtectorPass()
	require.True(t, ok)
	assert.True(t, res.NotArmed)
	assert.Zero(t, res.Closed)

	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(_ core.PeerID) core.P2PPeerInfo { return core.P2PPeerInfo{PeerType: core.UnknownPeer} },
	}))
	res, ok = mes.RunPeerProtectorPass()
	require.True(t, ok)
	assert.False(t, res.NotArmed)
	assert.Equal(t, 2, res.Closed, "armed with no warm-up, the cap is enforced")
}

// TestNetworkMessenger_InboundCapWaitsForWarmUp checks the cap is not enforced right after
// SetPeerShardResolver arms it, with the default warm-up.
func TestNetworkMessenger_InboundCapWaitsForWarmUp(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}

	arg := createMockNetworkArgs()
	arg.P2pConfig.ConnectionManager = config.ConnectionManagerConfig{MaxUnprotectedInboundConns: 1}

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	var peers []host.Host
	for i := 0; i < 3; i++ {
		h := newLocalPeerHost(t)
		connectToMessenger(t, h, mes)
		peers = append(peers, h)
	}
	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(_ core.PeerID) core.P2PPeerInfo { return core.P2PPeerInfo{PeerType: core.UnknownPeer} },
	}))

	res, ok := mes.RunPeerProtectorPass()
	require.True(t, ok)
	assert.True(t, res.InWarmUp)
	assert.Equal(t, 3, res.UnprotectedInboundUnknown)
	assert.Zero(t, res.Closed)
	for _, h := range peers {
		assert.Equal(t, network.Connected, mes.Host().Network().Connectedness(h.ID()))
	}
}

// TestNetworkMessenger_PeerProtectorLoopProtectsValidators checks the loop is started for regular
// nodes and picks up the resolver set after start-up.
func TestNetworkMessenger_PeerProtectorLoopProtectsValidators(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}
	defer libp2p.SetPeerProtectorInterval(20 * time.Millisecond)()

	mes, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	h := newLocalPeerHost(t)
	connectToMessenger(t, h, mes)
	require.NoError(t, mes.SetPeerShardResolver(&mock.PeerShardResolverStub{
		GetPeerInfoCalled: func(_ core.PeerID) core.P2PPeerInfo {
			return core.P2PPeerInfo{PeerType: core.ValidatorPeer}
		},
	}))

	cm := mes.Host().ConnManager()
	assert.Eventually(t, func() bool {
		return cm.IsProtected(h.ID(), libp2p.ValidatorProtectionTag)
	}, 5*time.Second, 10*time.Millisecond)
}

func TestNetworkMessenger_SeedRunsNoPeerProtector(t *testing.T) {
	arg := createMockNetworkArgs()
	arg.IsSeedNode = true

	mes, err := libp2p.NewNetworkMessenger(arg)
	require.NoError(t, err)
	defer func() { _ = mes.Close() }()

	_, ok := mes.RunPeerProtectorPass()
	assert.False(t, ok)
}
