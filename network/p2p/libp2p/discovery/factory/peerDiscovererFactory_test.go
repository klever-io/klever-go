package factory_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery"
	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery/factory"
	"github.com/klever-io/klever-go/network/p2p/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPeerDiscoverer_NoDiscoveryEnabledShouldRetNullDiscoverer(t *testing.T) {
	t.Parallel()

	p2pConfig := config.P2PConfig{
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled: false,
		},
	}

	pDiscoverer, err := factory.NewPeerDiscoverer(
		context.Background(),
		&mock.ConnectableHostStub{},
		&mock.SharderStub{},
		p2pConfig,
	)
	_, ok := pDiscoverer.(*discovery.NilDiscoverer)

	assert.True(t, ok)
	assert.Nil(t, err)
}

func TestNewPeerDiscoverer_ListsSharderShouldWork(t *testing.T) {
	t.Parallel()

	p2pConfig := config.P2PConfig{
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled:                          true,
			RefreshIntervalInSec:             1,
			RoutingTableRefreshIntervalInSec: 300,
		},
		Sharding: config.ShardingConfig{
			Type: p2p.ListsSharder,
		},
	}

	pDiscoverer, err := factory.NewPeerDiscoverer(
		context.Background(),
		&mock.ConnectableHostStub{},
		&mock.SharderStub{},
		p2pConfig,
	)
	_, ok := pDiscoverer.(*discovery.ContinuousKadDhtDiscoverer)

	assert.NotNil(t, pDiscoverer)
	assert.True(t, ok)
	assert.Nil(t, err)
}

func TestNewPeerDiscoverer_UnknownShouldErr(t *testing.T) {
	t.Parallel()

	p2pConfig := config.P2PConfig{
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled:              true,
			RefreshIntervalInSec: 1,
		},
		Sharding: config.ShardingConfig{
			Type: "unknown",
		},
	}

	pDiscoverer, err := factory.NewPeerDiscoverer(
		context.Background(),
		&mock.ConnectableHostStub{},
		&mock.SharderStub{},
		p2pConfig,
	)

	assert.True(t, check.IfNil(pDiscoverer))
	assert.True(t, errors.Is(err, p2p.ErrInvalidValue))
}

func createKadDhtTestConfig(discovererType string, initialPeers []string) config.P2PConfig {
	return config.P2PConfig{
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled:                          true,
			Type:                             discovererType,
			RefreshIntervalInSec:             1,
			RoutingTableRefreshIntervalInSec: 300,
			InitialPeerList:                  initialPeers,
		},
		Sharding: config.ShardingConfig{
			Type: p2p.NilListSharder,
		},
	}
}

func newTestPeerID(t *testing.T) peer.ID {
	t.Helper()

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(sk)
	require.NoError(t, err)

	return pid
}

func TestNewPeerDiscoverer_DiscovererTypes(t *testing.T) {
	t.Parallel()

	seederAddress := "/ip4/10.0.0.1/tcp/37373/p2p/" + newTestPeerID(t).String()

	tests := []struct {
		discovererType string
		expectedName   string
	}{
		{"", "kad-dht discovery"},
		{config.KadDhtTypeContinuous, "kad-dht discovery"},
		{config.KadDhtTypeOptimized, "optimized kad-dht discovery"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("type "+tt.discovererType, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			pDiscoverer, err := factory.NewPeerDiscoverer(
				ctx,
				&mock.ConnectableHostStub{},
				&mock.SharderStub{},
				createKadDhtTestConfig(tt.discovererType, []string{seederAddress}),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedName, pDiscoverer.Name())
		})
	}
}

func TestNewPeerDiscoverer_UnknownDiscovererTypeShouldErr(t *testing.T) {
	t.Parallel()

	pDiscoverer, err := factory.NewPeerDiscoverer(
		context.Background(),
		&mock.ConnectableHostStub{},
		&mock.SharderStub{},
		createKadDhtTestConfig("legacy", nil),
	)

	assert.True(t, check.IfNil(pDiscoverer))
	assert.True(t, errors.Is(err, p2p.ErrInvalidValue))
}

func invalidInitialPeersCases(self peer.ID, seederAddress string) map[string][]string {
	return map[string][]string{
		"own peer ID":       {seederAddress, "/ip4/10.0.0.2/tcp/37373/p2p/" + self.String()},
		"malformed address": {seederAddress, "10.0.0.2:37373"},
		"missing peer ID":   {"/ip4/10.0.0.2/tcp/37373"},
	}
}

func TestNewPeerDiscoverer_OptimizedRefusesInvalidInitialPeers(t *testing.T) {
	t.Parallel()

	self := newTestPeerID(t)
	seederAddress := "/ip4/10.0.0.1/tcp/37373/p2p/" + newTestPeerID(t).String()
	host := &mock.ConnectableHostStub{
		IDCalled: func() peer.ID { return self },
	}

	for name, initialPeers := range invalidInitialPeersCases(self, seederAddress) {
		pDiscoverer, err := factory.NewPeerDiscoverer(
			context.Background(),
			host,
			&mock.SharderStub{},
			createKadDhtTestConfig(config.KadDhtTypeOptimized, initialPeers),
		)

		assert.True(t, check.IfNil(pDiscoverer), name)
		assert.True(t, errors.Is(err, p2p.ErrInvalidValue), "%s: %v", name, err)
	}
}

func TestNewPeerDiscoverer_ContinuousToleratesInvalidInitialPeers(t *testing.T) {
	t.Parallel()

	self := newTestPeerID(t)
	seederAddress := "/ip4/10.0.0.1/tcp/37373/p2p/" + newTestPeerID(t).String()
	host := &mock.ConnectableHostStub{
		IDCalled: func() peer.ID { return self },
	}

	for name, initialPeers := range invalidInitialPeersCases(self, seederAddress) {
		pDiscoverer, err := factory.NewPeerDiscoverer(
			context.Background(),
			host,
			&mock.SharderStub{},
			createKadDhtTestConfig(config.KadDhtTypeContinuous, initialPeers),
		)

		assert.NoError(t, err, name)
		assert.False(t, check.IfNil(pDiscoverer), name)
	}
}

func TestNewPeerDiscoverer_OptimizedAcceptsARepeatedPeerID(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pid := newTestPeerID(t).String()
	initialPeers := []string{"/ip4/10.0.0.1/tcp/37373/p2p/" + pid, "/dns4/seed.example.org/tcp/37373/p2p/" + pid}

	pDiscoverer, err := factory.NewPeerDiscoverer(ctx, &mock.ConnectableHostStub{}, &mock.SharderStub{},
		createKadDhtTestConfig(config.KadDhtTypeOptimized, initialPeers))

	assert.NoError(t, err)
	assert.False(t, check.IfNil(pDiscoverer))
}

func TestNewPeerDiscoverer_OptimizedRequiresNilListSharder(t *testing.T) {
	t.Parallel()

	for _, sharderType := range []string{p2p.ListsSharder, p2p.OneListSharder} {
		p2pConfig := createKadDhtTestConfig(config.KadDhtTypeOptimized, nil)
		p2pConfig.Sharding.Type = sharderType

		pDiscoverer, err := factory.NewPeerDiscoverer(context.Background(), &mock.ConnectableHostStub{},
			&mock.SharderStub{}, p2pConfig)

		assert.True(t, check.IfNil(pDiscoverer), sharderType)
		assert.True(t, errors.Is(err, p2p.ErrInvalidValue), "%s: %v", sharderType, err)
	}

	// the continuous discoverer keeps working with every sharder
	p2pConfig := createKadDhtTestConfig(config.KadDhtTypeContinuous, nil)
	p2pConfig.Sharding.Type = p2p.ListsSharder
	pDiscoverer, err := factory.NewPeerDiscoverer(context.Background(), &mock.ConnectableHostStub{},
		&mock.SharderStub{}, p2pConfig)
	assert.NoError(t, err)
	assert.False(t, check.IfNil(pDiscoverer))
}
