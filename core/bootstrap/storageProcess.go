package bootstrap

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/eventNotifier/notifier"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/data/endProcess"
	dataRetriever "github.com/klever-io/klever-go/data/retriever"
	factoryDataPool "github.com/klever-io/klever-go/data/retriever/factory"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/factory/storageResolversContainers"
	"github.com/klever-io/klever-go/data/retriever/requestHandlers"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/storage/pathmanager"
	"github.com/klever-io/klever-go/storage/timecache"
)

// ArgsStorageEpochStartBootstrap holds the arguments needed for creating an epoch start data provider component
// from storage
type ArgsStorageEpochStartBootstrap struct {
	ArgsEpochStartBootstrap
	ImportDbConfig             config.ImportDbConfig
	ChanGracefullyClose        chan endProcess.ArgEndProcess
	TimeToWaitForRequestedData time.Duration
}

type storageEpochStartBootstrap struct {
	*epochStartBootstrap
	resolvers                  dataRetriever.ResolversContainer
	store                      dataRetriever.StorageService
	importDbConfig             config.ImportDbConfig
	chanGracefullyClose        chan endProcess.ArgEndProcess
	chainID                    string
	timeToWaitForRequestedData time.Duration
}

// NewStorageEpochStartBootstrap will return a new instance of storageEpochStartBootstrap that can bootstrap
// the node with the help of storage resolvers through the import-db process
func NewStorageEpochStartBootstrap(args ArgsStorageEpochStartBootstrap) (*storageEpochStartBootstrap, error) {
	esb, err := NewEpochStartBootstrap(args.ArgsEpochStartBootstrap)
	if err != nil {
		return nil, err
	}

	if args.ChanGracefullyClose == nil {
		return nil, common.ErrNilGracefullyCloseChannel
	}

	sesb := &storageEpochStartBootstrap{
		epochStartBootstrap:        esb,
		importDbConfig:             args.ImportDbConfig,
		chanGracefullyClose:        args.ChanGracefullyClose,
		chainID:                    args.GenesisNodesConfig.GetChainID(), // todo: check
		timeToWaitForRequestedData: args.TimeToWaitForRequestedData,
	}

	return sesb, nil
}

// Bootstrap runs the fast bootstrap method from local storage or from import-db directory
func (sesb *storageEpochStartBootstrap) Bootstrap() (Parameters, error) {
	// TODO: Check if it will be needed
	// defer sesb.closeTrieComponents()

	if !sesb.generalConfig.StartInEpoch {
		return sesb.fromLocalStorage()
	}

	defer func() {
		log.Debug("unregistering all message processor and un-joining all topics")
		errMessenger := sesb.messenger.UnregisterAllMessageProcessors()
		log.LogIfError(errMessenger)

		errMessenger = sesb.messenger.UnjoinAllTopics()
		log.LogIfError(errMessenger)
	}()

	var err error
	sesb.dataPool, err = factoryDataPool.NewDataPoolFromConfig(
		factoryDataPool.ArgsDataPool{
			Config: &sesb.generalConfig,
		},
	)
	if err != nil {
		return Parameters{}, err
	}

	params, shouldContinue, err := sesb.startFromSavedEpoch()
	if !shouldContinue {
		return params, err
	}

	err = sesb.prepareComponentsToSync()
	if err != nil {
		return Parameters{}, err
	}

	sesb.epochStartMeta, err = sesb.epochStartMetaBlockSyncer.SyncEpochStartMeta(sesb.timeToWaitForRequestedData)
	if err != nil {
		return Parameters{}, err
	}

	sesb.prevEpochStartMeta, err = sesb.epochStartMetaBlockSyncer.SyncPrevEpochStartMeta(timeToWait, sesb.epochStartMeta.GetEpoch()-1)
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("start in epoch bootstrap: got epoch start meta header from storage", "epoch", sesb.epochStartMeta.GetEpoch(), "nonce", sesb.epochStartMeta.GetNonce())
	log.Debug("start in epoch bootstrap: got prev epoch start meta header", "epoch", sesb.prevEpochStartMeta.GetEpoch(), "nonce", sesb.prevEpochStartMeta.GetNonce())
	sesb.setEpochStartMetrics()

	err = sesb.createSyncers()
	if err != nil {
		return Parameters{}, err
	}

	params, err = sesb.requestAndProcessFromStorage()
	if err != nil {
		return Parameters{}, err
	}

	return params, nil
}

func (sesb *storageEpochStartBootstrap) prepareComponentsToSync() error {
	sesb.closeTrieComponents()
	sesb.storageService = disabled.NewChainStorer()
	err := sesb.createTriesComponents()
	if err != nil {
		return err
	}

	err = sesb.createStorageRequestHandler()
	if err != nil {
		return err
	}

	blockProcessor, err := NewStorageEpochStartBlockProcessor(
		sesb.messenger,
		sesb.requestHandler,
		sesb.marshalizer,
		sesb.hasher,
	)
	if err != nil {
		return err
	}

	argsEpochStartSyncer := ArgsNewEpochStartMetaSyncer{
		RequestHandler:          sesb.requestHandler,
		Messenger:               sesb.messenger,
		Marshalizer:             sesb.marshalizer,
		TxSignMarshalizer:       sesb.txSignMarshalizer,
		Hasher:                  sesb.hasher,
		ChainID:                 []byte(sesb.genesisNodesConfig.GetChainID()),
		KeyGen:                  sesb.keyGen,
		BlockKeyGen:             sesb.blockKeyGen,
		Signer:                  sesb.singleSigner,
		BlockSigner:             sesb.blockSingleSigner,
		WhitelistHandler:        sesb.whiteListHandler,
		AddressPubkeyConv:       sesb.addressPubkeyConverter,
		NonceConverter:          sesb.uint64Converter,
		HeaderIntegrityVerifier: sesb.headerIntegrityVerifier,
		BlockProcessor:          blockProcessor,
		ForkController:          sesb.forkController,
	}

	sesb.epochStartMetaBlockSyncer, err = NewEpochStartMetaSyncer(argsEpochStartSyncer)
	if err != nil {
		return err
	}

	return nil
}

func (sesb *storageEpochStartBootstrap) createStorageRequestHandler() error {
	err := sesb.createStorageResolvers()
	if err != nil {
		return err
	}

	finder, err := containers.NewResolversFinder(sesb.resolvers)
	if err != nil {
		return err
	}

	requestedItemsHandler := timecache.NewTimeCache(timeBetweenRequests)
	sesb.requestHandler, err = requestHandlers.NewResolverRequestHandler(
		finder,
		requestedItemsHandler,
		sesb.whiteListHandler,
		maxToRequest,
		timeBetweenRequests,
	)
	return err
}

func (sesb *storageEpochStartBootstrap) createStorageResolvers() error {
	dataPacker, err := partitioning.NewSimpleDataPacker(sesb.marshalizer)
	if err != nil {
		return err
	}

	mesn := notifier.NewManualEpochStartNotifier()
	mesn.NewEpoch(sesb.importDbConfig.ImportDBStartInEpoch + 1)
	sesb.store, err = sesb.createStoreForStorageResolvers(mesn)
	if err != nil {
		return err
	}

	resolversContainerFactoryArgs := storageResolversContainers.FactoryArgs{
		GeneralConfig:            sesb.generalConfig,
		ChainID:                  sesb.chainID,
		WorkingDirectory:         sesb.importDbConfig.ImportDBWorkingDir,
		Hasher:                   sesb.hasher,
		Messenger:                sesb.messenger,
		Store:                    sesb.store,
		Marshalizer:              sesb.marshalizer,
		Uint64ByteSliceConverter: sesb.uint64Converter,
		DataPacker:               dataPacker,
		ManualEpochStartNotifier: mesn,
		ChanGracefullyClose:      sesb.chanGracefullyClose,
		// DisableOldTrieStorageEpoch: sesb.DisableOldTrieStorageEpoch, //todo: check
		EpochNotifier: sesb.epochNotifier,
	}

	resolversContainerFactory, err := storageResolversContainers.NewMetaResolversContainerFactory(resolversContainerFactoryArgs)
	if err != nil {
		return err
	}

	sesb.resolvers, err = resolversContainerFactory.Create()

	return err
}

func (sesb *storageEpochStartBootstrap) createStoreForStorageResolvers(mesn ManualEpochStartNotifier) (dataRetriever.StorageService, error) {

	dbPathWithChainID := filepath.Join(
		sesb.importDbConfig.ImportDBWorkingDir,
		factory.DefaultDBPath,
		sesb.chainID,
	)

	pathTemplateForPruningStorer := filepath.Join(
		dbPathWithChainID,
		fmt.Sprintf("%s_%s", factory.DefaultEpochString, core.PathEpochPlaceholder),
		core.PathIdentifierPlaceholder,
	)

	pathTemplateForStaticStorer := filepath.Join(
		dbPathWithChainID,
		factory.DefaultStaticDbString,
		core.PathIdentifierPlaceholder)

	pathManager, err := pathmanager.NewPathManager(pathTemplateForPruningStorer, pathTemplateForStaticStorer, dbPathWithChainID)
	if err != nil {
		return nil, err
	}

	return sesb.createStorageService(
		pathManager,
		mesn,
		sesb.importDbConfig.ImportDBStartInEpoch,
	)
}

func (sesb *storageEpochStartBootstrap) requestAndProcessFromStorage() (Parameters, error) {
	var err error
	sesb.baseData.lastEpoch = sesb.epochStartMeta.GetEpoch()

	epochStartValidators, err := sesb.GetEpochValidatorsFromTrie(sesb.epochStartMeta.GetValidatorsTrieRoot())
	if err != nil {
		return Parameters{}, err
	}

	prevEpochStartValidators, err := sesb.GetEpochValidatorsFromTrie(sesb.prevEpochStartMeta.GetValidatorsTrieRoot())
	if err != nil {
		return Parameters{}, err
	}

	pubKeyBytes, err := sesb.publicKey.ToByteArray()
	if err != nil {
		return Parameters{}, err
	}

	err = sesb.processNodesConfig(pubKeyBytes, epochStartValidators, prevEpochStartValidators)
	if err != nil {
		return Parameters{}, err
	}
	log.Debug("start in epoch bootstrap: processNodesConfig")

	err = sesb.messenger.CreateTopic(common.ConsensusTopic, true)
	if err != nil {
		return Parameters{}, err
	}

	storageHandlerComponent, err := NewMetaStorageHandler(
		sesb.generalConfig,
		sesb.pathManager,
		sesb.marshalizer,
		sesb.hasher,
		sesb.epochStartMeta.Header.Epoch,
		sesb.uint64Converter,
	)
	if err != nil {
		return Parameters{}, err
	}

	defer storageHandlerComponent.CloseStorageService()
	sesb.closeTrieComponents()
	err = sesb.createTriesComponents()
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("start in epoch bootstrap: syncValidatorAccountsState")
	err = sesb.syncValidatorAccountsState(sesb.epochStartMeta.Header.ValidatorsTrieRoot)
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("start in epoch bootstrap: syncAccountsState")
	err = sesb.syncUserAccountsState(sesb.epochStartMeta.Header.TrieRoot)
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("start in epoch bootstrap: syncKappAccountsState")
	err = sesb.syncKappAccountsState(sesb.epochStartMeta.Header.KAppsTrieRoot)
	if err != nil {
		return Parameters{}, err
	}

	components := &ComponentsNeededForBootstrap{
		EpochStartBlock:    sesb.epochStartMeta,
		PreviousEpochStart: sesb.prevEpochStartMeta,
		NodesConfig:        sesb.nodesConfig,
		Headers:            sesb.syncedHeaders,
		UserAccountTries:   sesb.userAccountTries,
		PeerAccountTries:   sesb.peerAccountTries,
		KAppAccountTries:   sesb.kappAccountTries,
	}

	errSavingToStorage := storageHandlerComponent.SaveDataToStorage(components)
	if errSavingToStorage != nil {
		return Parameters{}, err
	}

	log.Debug("removing cached received trie nodes")
	sesb.dataPool.TrieNodes().Clear()

	parameters := Parameters{
		Epoch:                   sesb.baseData.lastEpoch,
		NodesConfig:             sesb.nodesConfig,
		CurrEpochValidatorsInfo: epochStartValidators,
		PrevEpochValidatorsInfo: prevEpochStartValidators,
	}

	return parameters, nil
}

func (sesb *storageEpochStartBootstrap) fromLocalStorage() (Parameters, error) {

	sesb.initializeFromLocalStorage()
	if !sesb.baseData.storageExists {
		err := sesb.createTriesComponents()
		if err != nil {
			return Parameters{}, err
		}

		return Parameters{
			Epoch: sesb.startEpoch,
		}, nil
	}

	err := sesb.createTriesComponents()
	if err != nil {
		return Parameters{}, err
	}
	epochToStart := sesb.baseData.lastEpoch

	return Parameters{
		Epoch:       epochToStart,
		NodesConfig: sesb.nodesConfig,
	}, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (sesb *storageEpochStartBootstrap) IsInterfaceNil() bool {
	return sesb == nil
}
