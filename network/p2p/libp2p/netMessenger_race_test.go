package libp2p_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/network/p2p/libp2p"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

// TestNetworkMessenger_ParallelConstructionIsRaceFree guards KLC-2430.
// Constructing messengers concurrently must not race on libp2p's package-level
// pubsub.TimeCacheDuration global. Before the fix, createPubSub wrote that global
// on every construction, so two parallel constructions tripped the -race detector.
// The seen-messages TTL is now set per instance via pubsub.WithSeenMessagesTTL, so
// this test must pass cleanly under `go test -race`.
//
// Two paths are covered because they exercise different construction code:
//   - mock:  NewMockMessenger -> createMessenger -> createPubSub (no host/loggers)
//   - real:  NewNetworkMessenger -> setupExternalP2PLoggers + libp2p.New +
//     createMessenger -> createPubSub (the full production path)
//
// The racy global lived in the shared createPubSub, but covering the real path too
// asserts the *entire* construction sequence is race-free, matching the ticket's
// concern that "any future test or tool that constructs messengers in parallel"
// must be safe.
func TestNetworkMessenger_ParallelConstructionIsRaceFree(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}

	t.Run("mock messengers in parallel", func(t *testing.T) {
		const numMessengers = 8

		for i := range numMessengers {
			t.Run(fmt.Sprintf("messenger-%d", i), func(t *testing.T) {
				t.Parallel()

				netw := mocknet.New()
				mes, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
				require.Nil(t, err)
				defer func() { _ = mes.Close() }()

				require.NotNil(t, mes)
			})
		}
	})

	t.Run("real messengers in parallel", func(t *testing.T) {
		// Fewer instances: each opens a real libp2p host on an ephemeral port.
		// This path also runs setupExternalP2PLoggers concurrently, so it guards
		// the whole NewNetworkMessenger sequence, not just createPubSub.
		const numMessengers = 4

		for i := range numMessengers {
			t.Run(fmt.Sprintf("messenger-%d", i), func(t *testing.T) {
				t.Parallel()

				mes, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
				require.Nil(t, err)
				defer func() { _ = mes.Close() }()

				require.NotNil(t, mes)
			})
		}
	})
}
