package factory

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/sharding/networksharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// PrepareNetworkShardingCollector will create the network sharding collector and apply it to
// the network messenger and antiflood handler
func PrepareNetworkShardingCollector(
	network *NetworkComponents,
	config *config.Config,
	nodesCoordinator sharding.NodesCoordinator,
	epochStartRegistrationHandler eventNotifier.RegistrationHandler,
	epochStart uint32,
) (*networksharding.PeerShardMapper, error) {

	networkShardingCollector, err := createNetworkShardingCollector(config, nodesCoordinator, epochStartRegistrationHandler, epochStart)
	if err != nil {
		return nil, err
	}

	localID := network.NetMessenger.ID()
	networkShardingCollector.UpdatePeerID(localID)

	err = network.NetMessenger.SetPeerShardResolver(networkShardingCollector)
	if err != nil {
		return nil, err
	}

	err = network.InputAntifloodHandler.SetPeerValidatorMapper(networkShardingCollector)
	if err != nil {
		return nil, err
	}

	return networkShardingCollector, nil
}

func createNetworkShardingCollector(
	config *config.Config,
	nodesCoordinator sharding.NodesCoordinator,
	epochStartRegistrationHandler eventNotifier.RegistrationHandler,
	epochStart uint32,
) (*networksharding.PeerShardMapper, error) {

	cacheConfig := config.PublicKeyPeerID
	cachePkPid, err := createCache(cacheConfig)
	if err != nil {
		return nil, err
	}

	cacheConfig = config.PeerIDShardID
	cachePidShardID, err := createCache(cacheConfig)
	if err != nil {
		return nil, err
	}

	psm, err := networksharding.NewPeerShardMapper(
		cachePkPid,
		cachePidShardID,
		nodesCoordinator,
		epochStart,
	)
	if err != nil {
		return nil, err
	}

	epochStartRegistrationHandler.RegisterHandler(psm)

	return psm, nil
}

func createCache(cacheConfig config.CacheConfig) (storage.Cacher, error) {
	return storageUnit.NewCache(storageFactory.GetCacherFromConfig(cacheConfig))
}
