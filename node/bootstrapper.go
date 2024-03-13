package node

import (
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/core/process/sync/storageBootstrap"
)

func (n *Node) createMetaChainBootstrapper(slotManager consensus.SlotManager) (process.Bootstrapper, error) {
	argsBaseStorageBootstrapper := storageBootstrap.ArgsBaseStorageBootstrapper{
		BootStorer:         n.bootStorer,
		ForkDetector:       n.forkDetector,
		BlockProcessor:     n.blockProcessor,
		ChainHandler:       n.blkc,
		Marshalizer:        n.internalMarshalizer,
		Store:              n.store,
		Uint64Converter:    n.uint64ByteSliceConverter,
		BootstrapSlotIndex: n.bootstrapSlotIndex,
		NodesCoordinator:   n.nodesCoordinator,
		EpochStartTrigger:  n.epochStartTrigger,
		//BlockTracker:        n.blockTracker,
		ChainID: string(n.chainID),
	}

	argsMetaStorageBootstrapper := storageBootstrap.ArgsMetaStorageBootstrapper{
		ArgsBaseStorageBootstrapper: argsBaseStorageBootstrapper,
	}

	metaStorageBootstrapper, err := storageBootstrap.NewMetaStorageBootstrapper(argsMetaStorageBootstrapper)
	if err != nil {
		return nil, err
	}

	argsBaseBootstrapper := sync.ArgBaseBootstrapper{
		PoolsHolder:         n.dataPool,
		Store:               n.store,
		ChainHandler:        n.blkc,
		SlotManager:         slotManager,
		BlockProcessor:      n.blockProcessor,
		WaitTime:            n.slotManager.TimeDuration(),
		Hasher:              n.hasher,
		Marshalizer:         n.internalMarshalizer,
		ForkDetector:        n.forkDetector,
		RequestHandler:      n.requestHandler,
		Accounts:            n.accounts,
		BlackListHandler:    n.blocksBlackListHandler,
		NetworkWatcher:      n.messenger,
		BootStorer:          n.bootStorer,
		StorageBootstrapper: metaStorageBootstrapper,
		EpochHandler:        n.epochStartTrigger,
		Uint64Converter:     n.uint64ByteSliceConverter,
		Indexer:             n.indexer,
		IsInImportMode:      n.isInImportMode,
		StartWithInSync:     n.startWithInSync,
	}

	argsMetaBootstrapper := sync.ArgMetaBootstrapper{
		ArgBaseBootstrapper: argsBaseBootstrapper,
		EpochBootstrapper:   n.epochStartTrigger,
	}

	bootstrap, err := sync.NewMetaBootstrap(argsMetaBootstrapper)
	if err != nil {
		return nil, err
	}

	return bootstrap, nil
}
