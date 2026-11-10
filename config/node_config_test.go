package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

// TestNodeConfig_ResourceManagerIsScaled guards KLC-2417: with NilListSharder no connection
// manager closes idle peers, so validators must ship a scaled ResourceManager whose inbound
// ceiling (64 + scaledMemoryMiB/16) fits the validator set, instead of libp2p's auto-scaled
// default (128 inbound on an 8 GB host). Loads the shipped file the way cmd/node does.
func TestNodeConfig_ResourceManagerIsScaled(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("node", "config.yaml"))
	require.NoError(t, err)

	rm := cfg.P2P.ResourceManager
	require.Equal(t, config.ResourceManagerStrategyScaled, rm.Strategy,
		"node must ship the scaled ResourceManager, not libp2p's auto-scaled default or null")
	require.Equal(t, 4096, rm.ScaledMemoryMiB, "4096 MiB gives 320 inbound / 640 total connections")
	require.Zero(t, rm.MaxConnsPerIPv4, "per-IP limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.MaxConnsPerIPv6Subnet, "per-IP limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.ConnRatePerSec, "connection rate limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.ConnRateBurst, "connection rate limits stay at libp2p defaults on regular nodes")
}
