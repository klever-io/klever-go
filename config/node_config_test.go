package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

// TestNodeConfig_ResourceManagerIsScaled guards KLC-2417: regular nodes cap unknown inbound peers
// at maxUnprotectedInboundConns and keep the remaining inbound slots for validators, so they must
// ship a scaled ResourceManager whose inbound ceiling for established connections (64 +
// scaledMemoryMiB/16; the transient slots for handshaking connections come on top) leaves room for
// the validator set above that cap, instead of libp2p's auto-scaled default (128 inbound on an 8 GB
// host). Loads the shipped file the way cmd/node does.
func TestNodeConfig_ResourceManagerIsScaled(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("node", "config.yaml"))
	require.NoError(t, err)

	rm := cfg.P2P.ResourceManager
	require.Equal(t, config.ResourceManagerStrategyScaled, rm.Strategy,
		"node must ship the scaled ResourceManager, not libp2p's auto-scaled default or null")
	require.Equal(t, 4096, rm.ScaledMemoryMiB, "4096 MiB gives 320 inbound slots for established connections")
	require.Zero(t, rm.MaxConnsPerIPv4, "per-IP limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.MaxConnsPerIPv6Subnet, "per-IP limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.ConnRatePerSec, "connection rate limits stay at libp2p defaults on regular nodes")
	require.Zero(t, rm.ConnRateBurst, "connection rate limits stay at libp2p defaults on regular nodes")
}

// validatorInboundHeadroom is the number of inbound slots kept for protected peers (validators,
// gossipsub mesh peers) on top of the unprotected budget, sized for a ~200-validator network.
const validatorInboundHeadroom = 200

// TestNodeConfig_ConnectionManagerDefaults guards the shipped connectionManager section. A trim
// closes only unprotected, out-of-grace peers and stops at lowWater of them; protected peers come
// on top. So lowWater is the inbound budget of unprotected peers, and lowWater plus the protected
// validators must fit in the scaled ResourceManager's 64 + scaledMemoryMiB/16 inbound slots for
// established connections (scaledLimits adds the handshaking connections' transient slots on top),
// or an attacker's unprotected connections can keep validators out without ever being trimmed.
func TestNodeConfig_ConnectionManagerDefaults(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("node", "config.yaml"))
	require.NoError(t, err)

	cm := cfg.P2P.ConnectionManager
	require.Less(t, cm.LowWater, cm.HighWater)
	inboundLimit := 64 + cfg.P2P.ResourceManager.ScaledMemoryMiB/16
	require.Less(t, cm.LowWater+validatorInboundHeadroom, inboundLimit,
		"lowWater (%d) plus %d protected validators must fit in the %d inbound slots",
		cm.LowWater, validatorInboundHeadroom, inboundLimit)
	require.Equal(t, 60, cm.GracePeriodInSec)
	require.Equal(t, 100, cm.MaxUnprotectedInboundConns)
	require.Less(t, cm.MaxUnprotectedInboundConns+validatorInboundHeadroom, inboundLimit,
		"maxUnprotectedInboundConns (%d) plus %d protected validators must fit in the %d inbound slots",
		cm.MaxUnprotectedInboundConns, validatorInboundHeadroom, inboundLimit)
}

func TestNodeConfig_WasmVMVersionsMatchHost(t *testing.T) {
	paths := []string{
		filepath.Join("node", "config.yaml"),
		filepath.Join("..", "integrationTest", "config", "config.yaml"),
	}

	for _, path := range paths {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		execution := cfg.VirtualMachine.Execution.WasmVMVersions
		querying := cfg.VirtualMachine.Querying.WasmVMVersions
		require.NotEmpty(t, execution, "%s: execution has no Wasm VM version", path)

		versions := make([]config.WasmVMVersionByEpoch, 0, len(execution)+len(querying))
		versions = append(versions, execution...)
		versions = append(versions, querying...)

		for _, ver := range versions {
			require.Equal(t, vmhost.VMVersion, ver.Version,
				"%s: Wasm VM version from epoch %d must be the VM host version", path, ver.StartEpoch)
		}
	}
}
