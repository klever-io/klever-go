package storageBootstrap

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
)

var _ process.BootstrapperFromStorage = (*metaStorageBootstrapper)(nil)

type metaStorageBootstrapper struct {
	*storageBootstrapper
}

// NewMetaStorageBootstrapper is method used to create a new storage bootstrapper
func NewMetaStorageBootstrapper(arguments ArgsMetaStorageBootstrapper) (*metaStorageBootstrapper, error) {
	err := checkMetaStorageBootstrapperArgs(arguments)
	if err != nil {
		return nil, err
	}

	base := &storageBootstrapper{
		bootStorer:        arguments.BootStorer,
		forkDetector:      arguments.ForkDetector,
		blkExecutor:       arguments.BlockProcessor,
		blkc:              arguments.ChainHandler,
		marshalizer:       arguments.Marshalizer,
		store:             arguments.Store,
		nodesCoordinator:  arguments.NodesCoordinator,
		epochStartTrigger: arguments.EpochStartTrigger,
		//blockTracker:      arguments.BlockTracker,

		uint64Converter:    arguments.Uint64Converter,
		bootstrapSlotIndex: arguments.BootstrapSlotIndex,
		chainID:            arguments.ChainID,
	}

	boot := metaStorageBootstrapper{
		storageBootstrapper: base,
	}

	base.bootstrapper = &boot
	base.headerNonceHashStore = boot.store.GetStorer(retriever.HdrNonceHashDataUnit)

	return &boot, nil
}

// LoadFromStorage will load all blocks from storage
func (msb *metaStorageBootstrapper) LoadFromStorage() error {
	return msb.loadBlocks()
}

// IsInterfaceNil returns true if there is no value under the interface
func (msb *metaStorageBootstrapper) IsInterfaceNil() bool {
	return msb == nil
}

func (msb *metaStorageBootstrapper) getHeader(hash []byte) (data.HeaderHandler, error) {
	return process.GetHeaderFromStorage(hash, msb.marshalizer, msb.store)
}

func (msb *metaStorageBootstrapper) getHeaderWithNonce(nonce uint64) (data.HeaderHandler, []byte, error) {
	return process.GetHeaderFromStorageWithNonce(nonce, msb.store, msb.uint64Converter, msb.marshalizer)
}

func (msb *metaStorageBootstrapper) cleanupNotarizedStorage(metaBlockHash []byte) {
	log.Debug("cleanup notarized storage")

	metaBlock, err := process.GetHeaderFromStorage(metaBlockHash, msb.marshalizer, msb.store)
	if err != nil {
		log.Debug("meta block is not found in MetaBlockUnit storage",
			"hash", metaBlockHash)
		return
	}

	log.Debug("removing header from HdrNonceHashDataUnit storage",
		"nonce", metaBlock.Header.GetNonce(),
		"hash", metaBlockHash)

	hdrNonceHashDataUnit := retriever.HdrNonceHashDataUnit
	storer := msb.store.GetStorer(hdrNonceHashDataUnit)
	nonceToByteSlice := msb.uint64Converter.ToByteSlice(metaBlock.Header.GetNonce())
	err = storer.Remove(nonceToByteSlice)
	if err != nil {
		log.Debug("header was not removed from HdrNonceHashDataUnit storage",
			"nonce", metaBlock.Header.GetNonce(),
			"hash", metaBlockHash,
			"error", err.Error())
	}

}

func checkMetaStorageBootstrapperArgs(args ArgsMetaStorageBootstrapper) error {
	err := checkBaseStorageBootstrapperArguments(args.ArgsBaseStorageBootstrapper)
	if err != nil {
		return err
	}

	return nil
}
