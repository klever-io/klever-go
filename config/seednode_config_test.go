package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

// TestSeednodeConfig_ResourceManagerIsBoundedScaled guards KLC-2433: seeds must not ship with
// NullResourceManager, which disables all libp2p per-IP connection and stream limits.
// Loads the shipped file through config.LoadFromPath — the same path cmd/seednode uses.
func TestSeednodeConfig_ResourceManagerIsBoundedScaled(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("seednode", "config.yaml"))
	require.NoError(t, err)

	rm := cfg.P2P.ResourceManager
	require.Equal(t, config.ResourceManagerStrategyScaled, rm.Strategy,
		"seed must run a bounded (scaled) ResourceManager, not null")
	require.Greater(t, rm.ScaledMemoryMiB, 0, "scaled strategy requires a memory budget")
	require.Greater(t, rm.MaxConnsPerIPv4, 0, "seed must bound connections per source IPv4")
	require.Greater(t, rm.MaxConnsPerIPv6Subnet, 0, "seed must bound connections per source IPv6 subnet")
	require.Greater(t, rm.ConnRatePerSec, float64(0), "seed must rate-limit new connections per subnet")
	require.Greater(t, rm.ConnRateBurst, 0, "seed connection rate burst must be set")
}
