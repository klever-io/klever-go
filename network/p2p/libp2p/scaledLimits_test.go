package libp2p_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/p2p/libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScaledLimits_KeepsLibp2pServiceLimits guards that the "scaled" strategy keeps libp2p's
// bundled per-service limits, as libp2p's own default ResourceManager does. Without them a
// single peer could open as many ping streams as the generic per-peer limit allows.
func TestScaledLimits_KeepsLibp2pServiceLimits(t *testing.T) {
	limits := libp2p.ScaledLimits(4096<<20, 4096, true).ToPartialLimitConfig()

	pingPeer, ok := limits.ServicePeer[ping.ServiceName]
	require.True(t, ok, "ping must have its own per-peer service limit")
	assert.Equal(t, rcmgr.LimitVal(2), pingPeer.StreamsInbound, "libp2p allows 2 inbound ping streams per peer")

	_, ok = limits.Service[identify.ServiceName]
	assert.True(t, ok, "identify must have its own service limit")

	// The connection ceiling still comes from the scaled budget: 64 + 4096/16 inbound.
	assert.Equal(t, rcmgr.LimitVal(320), limits.System.ConnsInbound)
}
