package libp2p_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/p2p/libp2p"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

// Spelled out rather than referenced: the constant it mirrors is gone, and re-introducing one
// must not let this guard pass by accident.
const removedAuthProtocolID = "klever-node-auth/0.0.1"

// TestNetworkMessenger_DoesNotServeRemovedAuthProtocol guards the removal of the klever-node-auth
// mechanism (KLC-2676). It was never registered on a running node, so this asserts a property that
// already held - the point is that wiring it back in has to fail here first.
func TestNetworkMessenger_DoesNotServeRemovedAuthProtocol(t *testing.T) {
	t.Parallel()

	mes, err := libp2p.NewMockMessenger(createMockNetworkArgs(), mocknet.New())
	require.Nil(t, err)
	defer func() {
		_ = mes.Close()
	}()

	protocols := mes.Host().Mux().Protocols()

	// positive control, as the p2p-protocol-probe runs: an enumeration that comes back empty
	// would otherwise satisfy the assertion below without proving anything
	require.Contains(t, protocols, libp2p.DirectSendID)

	for _, pid := range protocols {
		require.NotEqual(t, removedAuthProtocolID, string(pid))
	}
}
