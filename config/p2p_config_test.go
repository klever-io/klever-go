package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

// TestShippedConfigs_SelectPeerDiscovererExplicitly guards KLC-2716: every shipped config names its
// kad-dht peer discoverer instead of relying on the empty-string default.
func TestShippedConfigs_SelectPeerDiscovererExplicitly(t *testing.T) {
	paths := []string{
		filepath.Join("node", "config.yaml"),
		filepath.Join("seednode", "config.yaml"),
		filepath.Join("..", "integrationTest", "config", "config.yaml"),
	}

	for _, path := range paths {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		discovererType := cfg.P2P.KadDhtPeerDiscovery.Type
		require.Contains(t, []string{config.KadDhtTypeContinuous, config.KadDhtTypeOptimized}, discovererType, path)
	}
}

// TestShippedNodeConfigs_CapSeeders guards KLC-2716: a node keeps a bounded number of seednode
// connections, so seednode load does not grow with nodes × seednodes.
func TestShippedNodeConfigs_CapSeeders(t *testing.T) {
	paths := []string{
		filepath.Join("node", "config.yaml"),
		filepath.Join("..", "integrationTest", "config", "config.yaml"),
	}

	for _, path := range paths {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)
		require.Equal(t, uint32(2), cfg.P2P.KadDhtPeerDiscovery.MaxSeeders, path)
	}
}
