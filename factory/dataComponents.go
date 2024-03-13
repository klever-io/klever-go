package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/data/retriever"
	dataRetrieverFactory "github.com/klever-io/klever-go/data/retriever/factory"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/tools/check"
)

// DataComponentsFactoryArgs holds the arguments needed for creating a data components factory
type DataComponentsFactoryArgs struct {
	Config             config.Config
	Core               *CoreComponents
	PathManager        storage.PathManagerHandler
	EpochStartNotifier notifier.EpochStartNotifier
	CurrentEpoch       uint32
}

type dataComponentsFactory struct {
	config             config.Config
	core               *CoreComponents
	pathManager        storage.PathManagerHandler
	epochStartNotifier notifier.EpochStartNotifier
	currentEpoch       uint32
}

// NewDataComponentsFactory will return a new instance of dataComponentsFactory
func NewDataComponentsFactory(args DataComponentsFactoryArgs) (*dataComponentsFactory, error) {
	if args.Core == nil {
		return nil, ErrNilCoreComponents
	}
	if check.IfNil(args.PathManager) {
		return nil, ErrNilPathManager
	}
	if check.IfNil(args.EpochStartNotifier) {
		return nil, ErrNilEpochStartNotifier
	}

	return &dataComponentsFactory{
		config:             args.Config,
		core:               args.Core,
		pathManager:        args.PathManager,
		epochStartNotifier: args.EpochStartNotifier,
		currentEpoch:       args.CurrentEpoch,
	}, nil
}

// Create will create and return the data components
func (dcf *dataComponentsFactory) Create() (*DataComponents, error) {
	var datapool retriever.PoolsHolder
	blkc, err := dcf.createBlockChainFromConfig()
	if err != nil {
		return nil, err
	}

	store, err := dcf.createDataStoreFromConfig()
	if err != nil {
		return nil, err
	}

	dataPoolArgs := dataRetrieverFactory.ArgsDataPool{
		Config: &dcf.config,
	}
	datapool, err = dataRetrieverFactory.NewDataPoolFromConfig(dataPoolArgs)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataPoolCreation, err.Error())
	}

	return &DataComponents{
		Blkc:     blkc,
		Store:    store,
		Datapool: datapool,
	}, nil
}

func (dcf *dataComponentsFactory) createBlockChainFromConfig() (data.ChainHandler, error) {

	blockChain := blockchain.NewBlockChain()

	err := blockChain.SetAppStatusHandler(dcf.core.StatusHandler)
	if err != nil {
		return nil, err
	}

	return blockChain, nil
}

func (dcf *dataComponentsFactory) createDataStoreFromConfig() (retriever.StorageService, error) {
	storageServiceFactory, err := factory.NewStorageServiceFactory(
		&dcf.config,
		dcf.pathManager,
		dcf.epochStartNotifier,
		dcf.currentEpoch,
	)
	if err != nil {
		return nil, err
	}

	return storageServiceFactory.CreateForMeta()
}
