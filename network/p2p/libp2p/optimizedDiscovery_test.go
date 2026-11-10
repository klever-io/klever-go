package libp2p_test

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createKadDhtNetworkArgs(discovererType string, initialPeers []string) libp2p.ArgsNetworkMessenger {
	args := createMockNetworkArgs()
	args.P2pConfig.KadDhtPeerDiscovery = config.KadDhtPeerDiscoveryConfig{
		Enabled:                          true,
		Type:                             discovererType,
		RefreshIntervalInSec:             1,
		ProtocolID:                       "/klv/kad/test",
		InitialPeerList:                  initialPeers,
		BucketSize:                       100,
		RoutingTableRefreshIntervalInSec: 300,
	}

	return args
}

func startTestSeeders(t *testing.T, numSeeders int) ([]p2p.Messenger, []string) {
	seeders := make([]p2p.Messenger, 0, numSeeders)
	seederAddresses := make([]string, 0, numSeeders)
	for i := 0; i < numSeeders; i++ {
		seeder, err := libp2p.NewNetworkMessenger(createKadDhtNetworkArgs(config.KadDhtTypeContinuous, nil))
		require.NoError(t, err)
		t.Cleanup(func() { _ = seeder.Close() })
		require.NoError(t, seeder.Bootstrap())

		seeders = append(seeders, seeder)
		seederAddresses = append(seederAddresses, getConnectableAddress(seeder))
	}

	return seeders, seederAddresses
}

func TestOptimizedDiscoverer_ConnectsToEverySeeder(t *testing.T) {
	if testing.Short() {
		t.Skip("this test opens real TCP connections")
	}

	seeders, seederAddresses := startTestSeeders(t, 3)

	node, err := libp2p.NewNetworkMessenger(createKadDhtNetworkArgs(config.KadDhtTypeOptimized, seederAddresses))
	require.NoError(t, err)
	defer func() { _ = node.Close() }()
	require.NoError(t, node.Bootstrap())

	for _, seeder := range seeders {
		s := seeder
		assert.Eventually(t, func() bool { return node.IsConnected(s.ID()) }, 5*time.Second, 50*time.Millisecond,
			"node should be connected to seeder %s", s.ID().Pretty())
	}
}

func TestOptimizedDiscoverer_MaxSeedersBoundsSeederDials(t *testing.T) {
	if testing.Short() {
		t.Skip("this test opens real TCP connections")
	}

	seeders, seederAddresses := startTestSeeders(t, 3)

	// the seeders start with empty initial peer lists and do not know each other, so the DHT can
	// not lead the node to a seeder it did not dial: every seeder connection comes from the cap
	args := createKadDhtNetworkArgs(config.KadDhtTypeOptimized, seederAddresses)
	args.P2pConfig.KadDhtPeerDiscovery.MaxSeeders = 2
	node, err := libp2p.NewNetworkMessenger(args)
	require.NoError(t, err)
	defer func() { _ = node.Close() }()
	require.NoError(t, node.Bootstrap())

	numConnectedSeeders := func() int {
		n := 0
		for _, s := range seeders {
			if node.IsConnected(s.ID()) {
				n++
			}
		}
		return n
	}
	assert.Eventually(t, func() bool { return numConnectedSeeders() == 2 }, 5*time.Second, 50*time.Millisecond)
	// several fast refresh cycles later, still only two
	time.Sleep(3 * time.Second)
	assert.Equal(t, 2, numConnectedSeeders())

	// the connected seeders are protected from the node's connection manager trimming
	for _, s := range seeders {
		pid := peer.ID(s.ID())
		assert.Equal(t, node.IsConnected(s.ID()), node.Host().ConnManager().IsProtected(pid, "seeder"), pid.String())
	}
}

func TestNewNetworkMessenger_OwnPeerIDInInitialPeersShouldErr(t *testing.T) {
	sk := createP2pPrivateKey()
	self, err := peer.IDFromPrivateKey(sk)
	require.NoError(t, err)

	ownAddress := []string{"/ip4/127.0.0.1/tcp/37373/p2p/" + self.String()}

	args := createKadDhtNetworkArgs(config.KadDhtTypeOptimized, ownAddress)
	args.P2pPrivateKey = sk
	mes, err := libp2p.NewNetworkMessenger(args)
	assert.True(t, check.IfNil(mes))
	assert.True(t, errors.Is(err, p2p.ErrInvalidValue), "%v", err)

	// the continuous discoverer drops the entry and starts
	args = createKadDhtNetworkArgs(config.KadDhtTypeContinuous, ownAddress)
	args.P2pPrivateKey = sk
	mes, err = libp2p.NewNetworkMessenger(args)
	require.NoError(t, err)
	assert.NoError(t, mes.Close())
}
