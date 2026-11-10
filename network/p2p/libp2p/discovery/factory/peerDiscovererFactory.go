package factory

import (
	"context"
	"fmt"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery"
	"github.com/klever-io/klever-go/tools/check"
)

// defaultSeedersReconnectionInterval is how often the optimized discoverer re-dials seeders while
// it is connected to at least one of them
const defaultSeedersReconnectionInterval = time.Minute * 5

var log = logger.GetOrCreate("p2p/libp2p/discovery/factory")

// NewPeerDiscoverer generates an implementation of PeerDiscoverer by parsing the p2pConfig struct
// Errors if config is badly formatted
func NewPeerDiscoverer(
	context context.Context,
	host discovery.ConnectableHost,
	sharder p2p.CommonSharder,
	p2pConfig config.P2PConfig,
) (p2p.PeerDiscoverer, error) {
	if p2pConfig.KadDhtPeerDiscovery.Enabled {
		return createKadDhtPeerDiscoverer(context, host, sharder, p2pConfig)
	}

	return discovery.NewNilDiscoverer(), nil
}

func createKadDhtPeerDiscoverer(
	context context.Context,
	host discovery.ConnectableHost,
	sharder p2p.CommonSharder,
	p2pConfig config.P2PConfig,
) (p2p.PeerDiscoverer, error) {
	if check.IfNilReflect(host) {
		return nil, p2p.ErrNilHost
	}

	arg := discovery.ArgKadDht{
		Context:                     context,
		Host:                        host,
		KddSharder:                  sharder,
		PeersRefreshInterval:        time.Second * time.Duration(p2pConfig.KadDhtPeerDiscovery.RefreshIntervalInSec),
		SeedersReconnectionInterval: defaultSeedersReconnectionInterval,
		MaxSeeders:                  p2pConfig.KadDhtPeerDiscovery.MaxSeeders,
		ProtocolID:                  p2pConfig.KadDhtPeerDiscovery.ProtocolID,
		InitialPeersList:            p2pConfig.KadDhtPeerDiscovery.InitialPeerList,
		BucketSize:                  p2pConfig.KadDhtPeerDiscovery.BucketSize,
		RoutingTableRefresh:         time.Second * time.Duration(p2pConfig.KadDhtPeerDiscovery.RoutingTableRefreshIntervalInSec),
	}

	switch p2pConfig.Sharding.Type {
	case p2p.ListsSharder, p2p.OneListSharder, p2p.NilListSharder:
		return createKadDhtDiscoverer(p2pConfig.KadDhtPeerDiscovery.Type, p2pConfig.Sharding.Type, arg)
	default:
		return nil, fmt.Errorf("%w unable to select peer discoverer based on "+
			"selected sharder: unknown sharder '%s'", p2p.ErrInvalidValue, p2pConfig.Sharding.Type)
	}
}

func createKadDhtDiscoverer(discovererType string, sharderType string, arg discovery.ArgKadDht) (p2p.PeerDiscoverer, error) {
	switch discovererType {
	case config.KadDhtTypeContinuous, "":
		log.Debug("using continuous kad dht discoverer")
		// the continuous discoverer moves on to the next initial peer when a dial fails, so a bad
		// entry costs it nothing; only an entry naming this node is dropped, with a warning
		arg.InitialPeersList = discovery.WithoutOwnPeerID(arg.InitialPeersList, arg.Host.ID())
		return discovery.NewContinuousKadDhtDiscoverer(arg)
	case config.KadDhtTypeOptimized:
		// the lists sharders evict peers on connection without regard to the connection-manager
		// protection the optimized discoverer puts on its seeders, so a counted seeder could be
		// closed and re-dialled every cycle
		if sharderType != p2p.NilListSharder {
			return nil, fmt.Errorf("%w: kad dht discoverer type '%s' requires sharder type '%s', got '%s'",
				p2p.ErrInvalidValue, config.KadDhtTypeOptimized, p2p.NilListSharder, sharderType)
		}
		err := discovery.CheckInitialPeersList(arg.InitialPeersList, arg.Host.ID())
		if err != nil {
			return nil, err
		}

		log.Debug("using optimized kad dht discoverer")
		return discovery.NewOptimizedKadDhtDiscoverer(arg)
	default:
		return nil, fmt.Errorf("%w unable to select peer discoverer based on type '%s'",
			p2p.ErrInvalidValue, discovererType)
	}
}
