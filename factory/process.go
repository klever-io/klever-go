package factory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/block/coordinator"
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/factory/chain"
	"github.com/klever-io/klever-go/core/process/factory/interceptorscontainer"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/core/process/peer"
	"github.com/klever-io/klever-go/core/process/smartContract"
	"github.com/klever-io/klever-go/core/process/smartContract/builtInFunctions"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	processSync "github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/core/process/throttle"
	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/factory/resolverscontainer"
	"github.com/klever-io/klever-go/data/retriever/factory/storageResolversContainers"
	"github.com/klever-io/klever-go/data/retriever/requestHandlers"
	"github.com/klever-io/klever-go/data/state"
	dataTx "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/eventNotifier/epochStart"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/genesis/process/disabled"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/pathmanager"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

// timeSpanForBadHeaders is the expiry time for an added block header hash
var timeSpanForBadHeaders = time.Minute * 2

type ProcessComponentsFactoryArgs struct {
	coreComponents            *CoreComponentsFactoryArgs
	accountsParser            genesis.AccountsParser
	economicsData             process.EconomicsDataHandler
	nodesConfig               *sharding.NodesSetup
	nodesCoordinator          sharding.NodesCoordinator
	data                      *DataComponents
	coreData                  *CoreComponents
	crypto                    *CryptoComponents
	state                     *StateComponents
	network                   *NetworkComponents
	tries                     *TriesComponents
	requestedItemsHandler     retriever.RequestedItemsHandler
	whiteListHandler          process.WhiteListHandler
	whiteListerVerifiedTxs    process.WhiteListHandler
	epochStartNotifier        notifier.EpochStartNotifier
	mainConfig                config.Config
	rater                     sharding.PeerAccountListAndRatingHandler
	ratingsData               process.RatingsInfoHandler
	startEpochNum             uint32
	stateCheckpointModulus    uint
	numConcurrentResolverJobs int32
	minSizeInBytes            uint32
	maxSizeInBytes            uint32
	maxRating                 uint32
	validatorPubkeyConverter  core.PubkeyConverter
	version                   string
	workingDir                string
	indexer                   process.Indexer
	txLogsProcessor           process.TransactionLogProcessor
	uint64Converter           typeConverters.Uint64ByteSliceConverter
	tpsBenchmark              statistics.TPSBenchmark
	epochNotifier             process.EpochNotifier
	slotManager               consensus.SlotManager
	storageReolverImportPath  string
	chanGracefullyClose       chan endProcess.ArgEndProcess
	fallbackHeaderValidator   process.FallbackHeaderValidator
	indexRating               bool
	accountsCacher            state.AccountsCacher
	forkController            core.ForkController
}

// NewProcessComponentsFactoryArgs initializes the arguments necessary for creating the process components
func NewProcessComponentsFactoryArgs(
	coreComponents *CoreComponentsFactoryArgs,
	accountsParser genesis.AccountsParser,
	economicsData process.EconomicsDataHandler,
	nodesConfig *sharding.NodesSetup,
	nodesCoordinator sharding.NodesCoordinator,
	data *DataComponents,
	coreData *CoreComponents,
	crypto *CryptoComponents,
	state *StateComponents,
	network *NetworkComponents,
	tries *TriesComponents,
	requestedItemsHandler retriever.RequestedItemsHandler,
	whiteListHandler process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	epochStartNotifier notifier.EpochStartNotifier,
	mainConfig config.Config,
	rater sharding.PeerAccountListAndRatingHandler,
	ratingsData process.RatingsInfoHandler,
	startEpochNum uint32,
	validatorPubkeyConverter core.PubkeyConverter,
	version string,
	uint64Converter typeConverters.Uint64ByteSliceConverter,
	workingDir string,
	indexer process.Indexer,
	tpsBenchmark statistics.TPSBenchmark,
	epochNotifier process.EpochNotifier,
	storageReolverImportPath string,
	slotManager consensus.SlotManager,
	chanGracefullyClose chan endProcess.ArgEndProcess,
	fallbackHeaderValidator process.FallbackHeaderValidator,
	indexRating bool,
	cacher state.AccountsCacher,
	forkController core.ForkController,
	txLogProcessor process.TransactionLogProcessor,
) *ProcessComponentsFactoryArgs {

	minSizeInBytes := mainConfig.BlockSizeThrottle.MinSizeInBytes
	maxSizeInBytes := mainConfig.BlockSizeThrottle.MaxSizeInBytes
	maxRating := mainConfig.Ratings.General.MaxRating

	// if not after fork, use the old hardcoded values
	if !forkController.EnableSmartContracts() {
		minSizeInBytes = 0
		maxSizeInBytes = 100000000
		maxRating = 10000000

	}

	return &ProcessComponentsFactoryArgs{
		coreComponents:            coreComponents,
		accountsParser:            accountsParser,
		economicsData:             economicsData,
		nodesConfig:               nodesConfig,
		nodesCoordinator:          nodesCoordinator,
		data:                      data,
		coreData:                  coreData,
		crypto:                    crypto,
		state:                     state,
		network:                   network,
		tries:                     tries,
		requestedItemsHandler:     requestedItemsHandler,
		whiteListHandler:          whiteListHandler,
		whiteListerVerifiedTxs:    whiteListerVerifiedTxs,
		epochStartNotifier:        epochStartNotifier,
		mainConfig:                mainConfig,
		rater:                     rater,
		ratingsData:               ratingsData,
		startEpochNum:             startEpochNum,
		stateCheckpointModulus:    mainConfig.StateTriesConfig.CheckpointSlotsModulus,
		numConcurrentResolverJobs: mainConfig.Antiflood.NumConcurrentResolverJobs,
		minSizeInBytes:            minSizeInBytes,
		maxSizeInBytes:            maxSizeInBytes,
		maxRating:                 maxRating,
		validatorPubkeyConverter:  validatorPubkeyConverter,
		version:                   version,
		uint64Converter:           uint64Converter,
		workingDir:                workingDir,
		indexer:                   indexer,
		tpsBenchmark:              tpsBenchmark,
		epochNotifier:             epochNotifier,
		storageReolverImportPath:  storageReolverImportPath,
		slotManager:               slotManager,
		chanGracefullyClose:       chanGracefullyClose,
		fallbackHeaderValidator:   fallbackHeaderValidator,
		indexRating:               indexRating,
		accountsCacher:            cacher,
		forkController:            forkController,
		txLogsProcessor:           txLogProcessor,
	}

}

// Process struct holds the process components
type Process struct {
	InterceptorsContainer process.InterceptorsContainer
	SlotManager           consensus.SlotManager
	ResolversFinder       retriever.ResolversFinder
	EpochStartTrigger     process.TriggerHandler
	RequestHandler        process.RequestHandler
	BlackListHandler      process.TimeCacher
	ValidatorsStatistics  process.ValidatorStatisticsProcessor
	ForkDetector          process.ForkDetector
	BootStorer            process.BootStorer

	ValidatorsProvider      process.ValidatorsProvider
	BlockProcessor          process.BlockProcessor
	TransactionProcessor    process.TransactionProcessor
	TXSimulatorProcessor    txsimulator.TransactionSimulatorProcessor
	HeaderSigVerifier       HeaderSigVerifierHandler
	HeaderIntegrityVerifier HeaderIntegrityVerifierHandler
	ForkController          core.ForkController
}

func createHeaderSigVerifier(args *ProcessComponentsFactoryArgs) (*headerCheck.HeaderSigVerifier, error) {
	argsHeaderSig := &headerCheck.ArgsHeaderSigVerifier{
		Marshalizer:             args.coreData.InternalMarshalizer,
		Hasher:                  args.coreData.Hasher,
		NodesCoordinator:        args.nodesCoordinator,
		MultiSigVerifier:        args.crypto.MultiSigner,
		SingleSigVerifier:       args.crypto.SingleSigner,
		KeyGen:                  args.crypto.BlockSignKeyGen,
		FallbackHeaderValidator: args.fallbackHeaderValidator,
	}

	return headerCheck.NewHeaderSigVerifier(argsHeaderSig)
}

func createHeaderIntegrityVerifier(args *ProcessComponentsFactoryArgs) (HeaderIntegrityVerifierHandler, error) {
	versionsCache, err := createCache(args.mainConfig.Versions.Cache)
	if err != nil {
		return nil, err
	}

	return headerCheck.NewHeaderIntegrityVerifier(
		[]byte(args.nodesConfig.ChainID),
		args.mainConfig.Versions.VersionsByEpochs,
		args.mainConfig.Versions.DefaultVersion,
		versionsCache,
	)
}

func createResolversFinder(args *ProcessComponentsFactoryArgs) (retriever.ResolversFinder, error) {
	resolversContainerFactory, err := newResolverContainerFactory(
		args.data,
		args.coreData,
		args.network,
		args.tries,
		args.numConcurrentResolverJobs,
		args.storageReolverImportPath,
		&args.mainConfig,
		args.startEpochNum,
		args.chanGracefullyClose,
	)
	if err != nil {
		return nil, err
	}

	resolversContainer, err := resolversContainerFactory.Create()
	if err != nil {
		return nil, err
	}

	return containers.NewResolversFinder(resolversContainer)
}

func setupGenesis(args *ProcessComponentsFactoryArgs) (data.HeaderHandler, error) {
	genesisBlock, err := generateGenesisHeadersAndApplyInitialBalances(args, args.workingDir)
	if err != nil {
		return nil, err
	}

	err = indexGenesisAccounts(args.nodesConfig.GetStartTime(), args.state.AccountsAdapter, args.indexer, args.coreData.InternalMarshalizer)
	if err != nil {
		log.Warn("cannot index genesis accounts", "error", err)
	}

	if args.startEpochNum == 0 {
		err = indexGenesisBlock(args, genesisBlock)
		if err != nil {
			return nil, err
		}

		//Index active parameters
		args.indexer.UpdateProposalsAndParameters([]string{})
	}

	return genesisBlock, args.data.Blkc.SetGenesisHeader(genesisBlock)
}

func createValidatorsProvider(args *ProcessComponentsFactoryArgs, validatorStatisticsProcessor process.ValidatorStatisticsProcessor) (process.ValidatorsProvider, error) {
	cacheRefreshIntervalInSec := args.mainConfig.ValidatorStatistics.CacheRefreshIntervalInSec
	// default value for cache refresh interval to 60 seconds
	if cacheRefreshIntervalInSec == 0 {
		cacheRefreshIntervalInSec = 60
	}

	cacheRefreshDuration := time.Duration(cacheRefreshIntervalInSec) * time.Second
	argVSP := peer.ArgValidatorsProvider{
		NodesCoordinator:                  args.nodesCoordinator,
		StartEpoch:                        args.startEpochNum,
		EpochStartEventNotifier:           args.epochStartNotifier,
		CacheRefreshIntervalDurationInSec: cacheRefreshDuration,
		ValidatorStatistics:               validatorStatisticsProcessor,
		MaxRating:                         args.maxRating,
		PubKeyConverter:                   args.validatorPubkeyConverter,
	}

	return peer.NewValidatorsProvider(argVSP)
}

func createBaseProcessor(
	args *ProcessComponentsFactoryArgs,
	requestHandler process.RequestHandler,
	resolversFinder retriever.ResolversFinder,
	headerSigVerifier HeaderSigVerifierHandler,
	headerIntegrityVerifier HeaderIntegrityVerifierHandler,
	epochStartTrigger eventNotifier.TriggerHandler,
) (*Process, error) {
	validatorStatisticsProcessor, err := newValidatorStatisticsProcessor(args)
	if err != nil {
		return nil, err
	}

	validatorsProvider, err := createValidatorsProvider(args, validatorStatisticsProcessor)
	if err != nil {
		return nil, err
	}

	validatorStatsRootHash, err := validatorStatisticsProcessor.RootHash()
	if err != nil {
		return nil, err
	}

	log.Debug("Validator Stats created", "validatorStatsRootHash", validatorStatsRootHash)

	interceptorContainerFactory, blackListHandler, err := newInterceptorContainerFactory(
		args.nodesCoordinator,
		args.data,
		args.coreData,
		args.crypto,
		args.state,
		args.network,
		args.economicsData,
		headerSigVerifier,
		headerIntegrityVerifier,
		epochStartTrigger,
		args.whiteListHandler,
		args.whiteListerVerifiedTxs,
		args.epochNotifier,
	)
	if err != nil {
		return nil, err
	}

	interceptorsContainer, err := interceptorContainerFactory.Create()
	if err != nil {
		return nil, err
	}

	forkDetector, err := processSync.NewMetaForkDetector(args.slotManager, blackListHandler, args.slotManager.GenesisTimestamp().Unix())
	if err != nil {
		return nil, err
	}

	bootStr := args.data.Store.GetStorer(retriever.BootstrapUnit)
	bootStorer, err := bootstrapStorage.NewBootstrapStorer(args.coreData.InternalMarshalizer, bootStr)
	if err != nil {
		return nil, err
	}

	vmOutputCacherConfig := storageFactory.GetCacherFromConfig(args.mainConfig.VMOutputCacher)
	vmOutputCacher, err := storageUnit.NewCache(vmOutputCacherConfig)
	if err != nil {
		return nil, err
	}

	txSimulatorProcessorArgs := &txsimulator.ArgsTxSimulator{
		AddressPubKeyConverter: args.state.AddressPubkeyConverter,
		VMOutputCacher:         vmOutputCacher,
		Hasher:                 args.coreData.Hasher,
		Marshalizer:            args.coreData.InternalMarshalizer,
	}

	blockProcessor, transactionProcessor, err := newBlockProcessor(
		args,
		requestHandler,
		forkDetector,
		epochStartTrigger,
		bootStorer,
		validatorStatisticsProcessor,
		txSimulatorProcessorArgs,
		headerIntegrityVerifier,
		args.indexRating,
	)
	if err != nil {
		return nil, err
	}

	txSimulator, err := txsimulator.NewTransactionSimulator(*txSimulatorProcessorArgs)
	if err != nil {
		return nil, err
	}

	err = args.economicsData.SetTXSimulatorProcessor(txSimulator)
	if err != nil {
		return nil, err
	}

	return &Process{
		InterceptorsContainer:   interceptorsContainer,
		ResolversFinder:         resolversFinder,
		EpochStartTrigger:       epochStartTrigger,
		RequestHandler:          requestHandler,
		BlackListHandler:        blackListHandler,
		ValidatorsStatistics:    validatorStatisticsProcessor,
		ForkDetector:            forkDetector,
		BootStorer:              bootStorer,
		ValidatorsProvider:      validatorsProvider,
		BlockProcessor:          blockProcessor,
		TransactionProcessor:    transactionProcessor,
		TXSimulatorProcessor:    txSimulator,
		HeaderSigVerifier:       headerSigVerifier,
		HeaderIntegrityVerifier: headerIntegrityVerifier,
		SlotManager:             args.slotManager,
		ForkController:          args.forkController,
	}, nil
}

// ProcessComponentsFactory creates the process components
func ProcessComponentsFactory(args *ProcessComponentsFactoryArgs) (*Process, error) {
	headerSigVerifier, err := createHeaderSigVerifier(args)
	if err != nil {
		return nil, err
	}

	headerIntegrityVerifier, err := createHeaderIntegrityVerifier(args)
	if err != nil {
		return nil, err
	}

	resolversFinder, err := createResolversFinder(args)
	if err != nil {
		return nil, err
	}

	requestHandler, err := requestHandlers.NewResolverRequestHandler(
		resolversFinder,
		args.requestedItemsHandler,
		args.whiteListHandler,
		maxTxsToRequest,
		time.Second,
	)
	if err != nil {
		return nil, err
	}

	genesisBlock, err := setupGenesis(args)
	if err != nil {
		return nil, err
	}

	epochStartTrigger, err := newEpochStartTrigger(args)
	if err != nil {
		return nil, err
	}
	requestHandler.SetEpoch(epochStartTrigger.Epoch())

	err = retriever.SetEpochHandlerToHdrResolver(resolversFinder, epochStartTrigger)
	if err != nil {
		return nil, err
	}

	err = prepareGenesisBlock(args, genesisBlock)
	if err != nil {
		return nil, err
	}

	return createBaseProcessor(args, requestHandler, resolversFinder, headerSigVerifier, headerIntegrityVerifier, epochStartTrigger)
}

func newResolverContainerFactory(
	data *DataComponents,
	coreData *CoreComponents,
	network *NetworkComponents,
	tries *TriesComponents,
	numConcurrentResolverJobs int32,
	storageResolverImportPath string,
	config *config.Config,
	currentEpoch uint32,
	chanGracefullyClose chan endProcess.ArgEndProcess,
) (retriever.ResolversContainerFactory, error) {

	if config.ImportDbConfig.IsImportDBMode {
		log.Debug("starting with storage resolvers", "path", storageResolverImportPath)
		return newStorageResolver(
			coreData,
			network,
			config,
			currentEpoch,
			chanGracefullyClose,
		)
	}

	return newMetaResolverContainerFactory(
		data,
		coreData,
		network,
		tries,
		numConcurrentResolverJobs,
	)

}

func newStorageResolver(
	coreData *CoreComponents,
	network *NetworkComponents,
	config *config.Config,
	currentEpoch uint32,
	chanGracefullyClose chan endProcess.ArgEndProcess,
) (retriever.ResolversContainerFactory, error) {
	pathManager, err := createPathManager(config.ImportDbConfig.ImportDBWorkingDir, string(coreData.ChainID))
	if err != nil {
		return nil, err
	}

	manualEpochStartNotifier := notifier.NewManualEpochStartNotifier()
	defer func() {
		// we need to call this after we wired all the notified components
		if config.ImportDbConfig.IsImportDBMode {
			manualEpochStartNotifier.NewEpoch(currentEpoch + 1)
		}
	}()
	storageServiceCreator, err := storageFactory.NewStorageServiceFactory(
		config,
		pathManager,
		manualEpochStartNotifier,
		currentEpoch,
	)
	if err != nil {
		return nil, err
	}

	store, errStore := storageServiceCreator.CreateForMeta()
	if errStore != nil {
		return nil, errStore
	}

	manualEpochStartNotifier.NewEpoch(currentEpoch + 1)

	return createStorageResolversForMeta(
		coreData,
		network,
		store,
		manualEpochStartNotifier,
		chanGracefullyClose,
	)
}

func newMetaResolverContainerFactory(
	data *DataComponents,
	core *CoreComponents,
	network *NetworkComponents,
	tries *TriesComponents,
	numConcurrentResolverJobs int32,
) (retriever.ResolversContainerFactory, error) {
	dataPacker, err := partitioning.NewSimpleDataPacker(core.InternalMarshalizer)
	if err != nil {
		return nil, err
	}

	resolversContainerFactoryArgs := resolverscontainer.FactoryArgs{
		Messenger:                  network.NetMessenger,
		Store:                      data.Store,
		Marshalizer:                core.InternalMarshalizer,
		DataPools:                  data.Datapool,
		Uint64ByteSliceConverter:   core.Uint64ByteSliceConverter,
		DataPacker:                 dataPacker,
		TriesContainer:             tries.TriesContainer,
		InputAntifloodHandler:      network.InputAntifloodHandler,
		OutputAntifloodHandler:     network.OutputAntifloodHandler,
		NumConcurrentResolvingJobs: numConcurrentResolverJobs,
	}

	resolversContainerFactory, err := resolverscontainer.NewMetaResolversContainerFactory(resolversContainerFactoryArgs)
	if err != nil {
		return nil, err
	}

	return resolversContainerFactory, nil
}

func newInterceptorContainerFactory(
	nodesCoordinator sharding.NodesCoordinator,
	data *DataComponents,
	coreData *CoreComponents,
	crypto *CryptoComponents,
	state *StateComponents,
	network *NetworkComponents,
	economics process.EconomicsDataHandler,
	headerSigVerifier HeaderSigVerifierHandler,
	headerIntegrityVerifier HeaderIntegrityVerifierHandler,
	epochStartTrigger process.EpochStartTriggerHandler,
	whiteListHandler process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	epochNotifier process.EpochNotifier,
) (process.InterceptorsContainerFactory, process.TimeCacher, error) {

	return newMetaInterceptorContainerFactory(
		nodesCoordinator,
		data,
		coreData,
		crypto,
		network,
		state,
		economics,
		headerSigVerifier,
		headerIntegrityVerifier,
		epochStartTrigger,
		whiteListHandler,
		whiteListerVerifiedTxs,
		epochNotifier,
	)
}

func newMetaInterceptorContainerFactory(
	nodesCoordinator sharding.NodesCoordinator,
	data *DataComponents,
	dataCore *CoreComponents,
	crypto *CryptoComponents,
	network *NetworkComponents,
	state *StateComponents,
	economics process.EconomicsDataHandler,
	headerSigVerifier HeaderSigVerifierHandler,
	headerIntegrityVerifier HeaderIntegrityVerifierHandler,
	epochStartTrigger process.EpochStartTriggerHandler,
	whiteListHandler process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	epochNotifier process.EpochNotifier,
) (process.InterceptorsContainerFactory, process.TimeCacher, error) {
	headerBlackList := timecache.NewTimeCache(timeSpanForBadHeaders)
	metaInterceptorsContainerFactoryArgs := interceptorscontainer.MetaInterceptorsContainerFactoryArgs{
		NodesCoordinator:        nodesCoordinator,
		Messenger:               network.NetMessenger,
		Store:                   data.Store,
		ProtoMarshalizer:        dataCore.InternalMarshalizer,
		TxSignMarshalizer:       dataCore.TxSignMarshalizer,
		Hasher:                  dataCore.Hasher,
		MultiSigner:             crypto.MultiSigner,
		DataPool:                data.Datapool,
		Accounts:                state.AccountsAdapter,
		AddressPubkeyConverter:  state.AddressPubkeyConverter,
		SingleSigner:            crypto.TxSingleSigner,
		BlockSingleSigner:       crypto.SingleSigner,
		KeyGen:                  crypto.TxSignKeyGen,
		BlockKeyGen:             crypto.BlockSignKeyGen,
		TxFeeHandler:            economics,
		BlackList:               headerBlackList,
		HeaderSigVerifier:       headerSigVerifier,
		HeaderIntegrityVerifier: headerIntegrityVerifier,
		WhiteListHandler:        whiteListHandler,
		WhiteListerVerifiedTxs:  whiteListerVerifiedTxs,
		AntifloodHandler:        network.InputAntifloodHandler,
		EpochStartTrigger:       epochStartTrigger,
		ChainID:                 dataCore.ChainID,
		MinTransactionVersion:   dataCore.MinTransactionVersion,
		TxSignHasher:            dataCore.TxSignHasher,
		EpochNotifier:           epochNotifier,
		KAppController:          state.KAppController,
	}
	interceptorContainerFactory, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(metaInterceptorsContainerFactoryArgs)
	if err != nil {
		return nil, nil, err
	}

	return interceptorContainerFactory, headerBlackList, nil
}

func createPathManager(
	storageResolverImportPath string,
	chainID string,
) (storage.PathManagerHandler, error) {
	dbPathWithChainID := filepath.Join(
		storageResolverImportPath,
		DefaultDBPath,
		chainID,
	)
	pathTemplateForPruningStorer := filepath.Join(
		dbPathWithChainID,
		fmt.Sprintf("%s_%s", DefaultEpochString, core.PathEpochPlaceholder),
		core.PathIdentifierPlaceholder)

	pathTemplateForStaticStorer := filepath.Join(
		dbPathWithChainID,
		DefaultStaticDbString,
		core.PathIdentifierPlaceholder)

	return pathmanager.NewPathManager(pathTemplateForPruningStorer, pathTemplateForStaticStorer, dbPathWithChainID)
}

func createStorageResolversForMeta(
	coreData *CoreComponents,
	network *NetworkComponents,
	store retriever.StorageService,
	manualEpochStartNotifier retriever.ManualEpochStartNotifier,
	chanGracefullyClose chan endProcess.ArgEndProcess,
) (retriever.ResolversContainerFactory, error) {
	dataPacker, err := partitioning.NewSimpleDataPacker(coreData.InternalMarshalizer)
	if err != nil {
		return nil, err
	}

	resolversContainerFactoryArgs := storageResolversContainers.FactoryArgs{
		Messenger:                network.NetMessenger,
		Store:                    store,
		Marshalizer:              coreData.InternalMarshalizer,
		Uint64ByteSliceConverter: coreData.Uint64ByteSliceConverter,
		DataPacker:               dataPacker,
		ManualEpochStartNotifier: manualEpochStartNotifier,
		ChanGracefullyClose:      chanGracefullyClose,
	}
	resolversContainerFactory, err := storageResolversContainers.NewMetaResolversContainerFactory(resolversContainerFactoryArgs)
	if err != nil {
		return nil, err
	}

	return resolversContainerFactory, nil
}

// PrepareOpenTopics will set to the anti flood handler the topics for which
// the node can receive messages from others than validators
func PrepareOpenTopics(
	antiflood P2PAntifloodHandler,
) {
	antiflood.SetTopicsForAll(common.HeartbeatTopic, common.TransactionTopic)
}

func newValidatorStatisticsProcessor(
	processComponents *ProcessComponentsFactoryArgs,
) (process.ValidatorStatisticsProcessor, error) {

	storageService := processComponents.data.Store

	var peerDataPool peer.DataPool = processComponents.data.Datapool
	//hardForkConfig := processComponents.mainConfig.Hardfork

	arguments := peer.ArgValidatorStatisticsProcessor{
		PeerAdapter: processComponents.state.PeersAdapter,
		KAppAdapter: processComponents.state.KAppsAdapter,
		PubkeyConv:  processComponents.state.ValidatorPubkeyConverter,
		//Use Nodes Coordinator from Bootstrap
		NodesCoordinator:   processComponents.nodesCoordinator,
		DataPool:           peerDataPool,
		StorageService:     storageService,
		Marshalizer:        processComponents.coreData.InternalMarshalizer,
		MaxComputableSlots: processComponents.mainConfig.Preferences.MaxComputableSlots,
		Rater:              processComponents.rater,
		RewardsHandler:     processComponents.economicsData,
		NodesSetup:         processComponents.nodesConfig,
		GenesisNonce:       processComponents.data.Blkc.GetGenesisHeader().GetNonce(),
		EpochNotifier:      processComponents.epochNotifier,
		VKApp:              processComponents.state.KAppController.GetValidatorsKApp(),
		ForkController:     processComponents.forkController,
	}

	validatorStatisticsProcessor, err := peer.NewValidatorStatisticsProcessor(arguments)
	if err != nil {
		return nil, err
	}

	return validatorStatisticsProcessor, nil
}

func newBlockProcessor(
	processArgs *ProcessComponentsFactoryArgs,
	requestHandler process.RequestHandler,
	forkDetector process.ForkDetector,
	epochStartTrigger eventNotifier.TriggerHandler,
	bootStorer process.BootStorer,
	validatorStatisticsProcessor process.ValidatorStatisticsProcessor,
	txSimulatorProcessorArgs *txsimulator.ArgsTxSimulator,
	headerIntegrityVerifier HeaderIntegrityVerifierHandler,
	indexRating bool,
) (process.BlockProcessor, process.TransactionProcessor, error) {
	txFeeHandler, err := postprocess.NewFeeAccumulator()
	if err != nil {
		return nil, nil, err
	}

	wasmVMChangeLocker := &sync.RWMutex{}
	argsGasScheduleNotifier := notifier.ArgsNewGasScheduleNotifier{
		GasScheduleConfig:  processArgs.mainConfig.GasScheduleConfig,
		EpochNotifier:      processArgs.epochNotifier,
		WasmVMChangeLocker: wasmVMChangeLocker,
	}
	gasScheduleNotifier, err := notifier.NewGasScheduleNotifier(argsGasScheduleNotifier)
	if err != nil {
		return nil, nil, err
	}

	argsBuiltIn := builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		AccountsCacher:  processArgs.accountsCacher,
		KAppController:  processArgs.state.KAppController,
		GasSchedule:     gasScheduleNotifier,
		MapDNSAddresses: make(map[string]struct{}),
		Marshalizer:     processArgs.coreData.InternalMarshalizer,
		EpochNotifier:   processArgs.epochNotifier,
		ForkController:  processArgs.forkController,
	}

	builtInFuncFactory, err := builtInFunctions.CreateBuiltInFunctionsFactory(argsBuiltIn)
	if err != nil {
		return nil, nil, err
	}

	kdaTransferParser, err := parsers.NewKDATransferParser(processArgs.coreData.InternalMarshalizer)
	if err != nil {
		return nil, nil, err
	}

	counter, err := counters.NewUsageCounter(kdaTransferParser)
	if err != nil {
		return nil, nil, err
	}

	argsHook := hooks.ArgBlockChainHook{
		AccountsCacher:     processArgs.accountsCacher,
		KAppController:     processArgs.state.KAppController,
		PubkeyConv:         processArgs.state.AddressPubkeyConverter,
		StorageService:     processArgs.data.Store,
		BlockChain:         processArgs.data.Blkc,
		Marshalizer:        processArgs.coreData.InternalMarshalizer,
		Uint64Converter:    processArgs.coreData.Uint64ByteSliceConverter,
		BuiltInFunctions:   builtInFuncFactory.BuiltInFunctionContainer(),
		DataPool:           processArgs.data.Datapool,
		CompiledSCPool:     processArgs.data.Datapool.SmartContracts(),
		EpochNotifier:      processArgs.epochNotifier,
		ForkController:     processArgs.forkController,
		GasSchedule:        gasScheduleNotifier,
		ConfigSCStorage:    processArgs.mainConfig.Storages.SmartContractsStorage,
		NilCompiledSCStore: false,
		Counter:            counter,
		WorkingDir:         processArgs.workingDir,
	}

	blockChainHookImpl, err := hooks.NewBlockChainHookImpl(argsHook)
	if err != nil {
		return nil, nil, err
	}

	argsNewVMFactory := chain.ArgVMContainerFactory{
		BlockChainHook:     blockChainHookImpl,
		BuiltInFunctions:   argsHook.BuiltInFunctions,
		Config:             processArgs.mainConfig.VirtualMachine.Execution,
		EpochNotifier:      processArgs.epochNotifier,
		ForkController:     processArgs.forkController,
		WasmVMChangeLocker: processArgs.coreData.WasmVMChangeLocker,
		KDATransferParser:  kdaTransferParser,
		Hasher:             processArgs.coreData.Hasher,
		GasSchedule:        gasScheduleNotifier,
	}
	virtualMachineFactory, err := chain.NewVMContainerFactory(argsNewVMFactory)
	if err != nil {
		return nil, nil, err
	}

	err = builtInFuncFactory.SetPayableHandler(virtualMachineFactory.BlockChainHookImpl())
	if err != nil {
		return nil, nil, err
	}

	vmContainer, err := virtualMachineFactory.Create()
	if err != nil {
		return nil, nil, err
	}

	argsParser := smartContract.NewArgumentParser()
	argsNewSCProcessor := smartContract.ArgsNewSmartContractProcessor{
		VmContainer:         vmContainer,
		ArgsParser:          argsParser,
		Hasher:              processArgs.coreData.Hasher,
		Marshalizer:         processArgs.coreData.InternalMarshalizer,
		BlockChainHook:      virtualMachineFactory.BlockChainHookImpl(),
		BuiltInFunctions:    builtInFuncFactory.BuiltInFunctionContainer(),
		PubkeyConv:          processArgs.state.AddressPubkeyConverter,
		TxFeeHandler:        txFeeHandler,
		EconomicsFee:        processArgs.economicsData,
		GasSchedule:         gasScheduleNotifier,
		TxLogsProcessor:     processArgs.txLogsProcessor,
		ForkController:      processArgs.forkController,
		VMOutputCacher:      txcache.NewDisabledCache(),
		AccountsCacher:      processArgs.accountsCacher,
		WasmVMChangeLocker:  &sync.RWMutex{},
		IsGenesisProcessing: true,
	}
	scProcessor, err := smartContract.NewSmartContractProcessor(argsNewSCProcessor)
	if err != nil {
		return nil, nil, err
	}

	err = createMetaTxSimulatorProcessor(txSimulatorProcessorArgs, processArgs, argsNewSCProcessor, kdaTransferParser, gasScheduleNotifier)
	if err != nil {
		return nil, nil, err
	}

	args := transaction.ArgsNewTxProcessor{
		Cfg:            processArgs.mainConfig,
		KAppController: processArgs.state.KAppController,
		Hasher:         processArgs.coreData.Hasher,
		PubkeyConv:     processArgs.state.AddressPubkeyConverter,
		KeyGen:         processArgs.crypto.TxSignKeyGen,
		SingleSigner:   processArgs.crypto.TxSingleSigner,
		Marshalizer:    processArgs.coreData.InternalMarshalizer,
		EconomicsFee:   processArgs.economicsData,
		TxFeeHandler:   txFeeHandler,
		EpochNotifier:  processArgs.epochNotifier,
		RatingsData:    processArgs.ratingsData,
		AccountsCacher: processArgs.accountsCacher,
		ForkController: processArgs.forkController,
		ScProcessor:    scProcessor,
	}

	txProcessor, err := transaction.NewTxProcessor(args)
	if err != nil {
		return nil, nil, err
	}

	txPreProcessor, err := preprocess.NewTransactionPreprocessor(
		processArgs.data.Datapool.Transactions(),
		processArgs.data.Store,
		processArgs.coreData.Hasher,
		processArgs.coreData.InternalMarshalizer,
		txProcessor,
		processArgs.state.AccountsAdapter,
		processArgs.state.KAppsAdapter,
		processArgs.state.PeersAdapter,
		requestHandler.RequestTransaction,
		processArgs.economicsData,
		processArgs.validatorPubkeyConverter,
		processArgs.forkController,
	)
	if err != nil {
		return nil, nil, err
	}

	txCoordinator, err := coordinator.NewTransactionCoordinator(
		processArgs.coreData.Hasher,
		processArgs.coreData.InternalMarshalizer,
		processArgs.state.AccountsAdapter,
		processArgs.state.KAppsAdapter,
		requestHandler,
		txPreProcessor,
		txFeeHandler,
		processArgs.economicsData,
		processArgs.txLogsProcessor,
		processArgs.forkController,
	)
	if err != nil {
		return nil, nil, err
	}

	blockSizeThrottler, err := throttle.NewBlockSizeThrottle(processArgs.minSizeInBytes, processArgs.maxSizeInBytes)
	if err != nil {
		return nil, nil, err
	}

	genesisHdr := processArgs.data.Blkc.GetGenesisHeader()
	argsEpochStartData := epochStart.ArgsNewEpochStartData{
		Marshalizer:       processArgs.coreData.InternalMarshalizer,
		Hasher:            processArgs.coreData.Hasher,
		Store:             processArgs.data.Store,
		DataPool:          processArgs.data.Datapool,
		EpochStartTrigger: epochStartTrigger,
		RequestHandler:    requestHandler,
		GenesisEpoch:      genesisHdr.GetEpoch(),
	}
	epochStartDataCreator, err := epochStart.NewEpochStartData(argsEpochStartData)
	if err != nil {
		return nil, nil, err
	}

	accountsDb := make(map[state.AccountsDbIdentifier]state.AccountsAdapter)
	accountsDb[state.UserAccountsState] = processArgs.state.AccountsAdapter
	accountsDb[state.PeerAccountsState] = processArgs.state.PeersAdapter
	accountsDb[state.KAppAccountsState] = processArgs.state.KAppsAdapter

	argumentsBaseProcessor := block.ArgBaseProcessor{
		AccountsDB:              accountsDb,
		ForkDetector:            forkDetector,
		Hasher:                  processArgs.coreData.Hasher,
		Marshalizer:             processArgs.coreData.InternalMarshalizer,
		Store:                   processArgs.data.Store,
		NodesCoordinator:        processArgs.nodesCoordinator,
		Uint64Converter:         processArgs.coreData.Uint64ByteSliceConverter,
		RequestHandler:          requestHandler,
		SlotManager:             processArgs.slotManager,
		BootStorer:              bootStorer,
		DataPool:                processArgs.data.Datapool,
		BlockChain:              processArgs.data.Blkc,
		StateCheckpointModulus:  processArgs.stateCheckpointModulus,
		BlockSizeThrottler:      blockSizeThrottler,
		Indexer:                 processArgs.indexer,
		TpsBenchmark:            processArgs.tpsBenchmark,
		EpochNotifier:           processArgs.epochNotifier,
		EpochStartTrigger:       epochStartTrigger,
		TxCoordinator:           txCoordinator,
		FeeHandler:              txFeeHandler,
		HeaderIntegrityVerifier: headerIntegrityVerifier,
		KAppController:          processArgs.state.KAppController,
		BlockChainHook:          virtualMachineFactory.BlockChainHookImpl(),
	}

	arguments := block.ArgMetaProcessor{
		ArgBaseProcessor:             argumentsBaseProcessor,
		EpochStartDataCreator:        epochStartDataCreator,
		EconomicsData:                processArgs.economicsData,
		ValidatorStatisticsProcessor: validatorStatisticsProcessor,
		IndexRating:                  indexRating,
		ForkController:               processArgs.forkController,
	}

	processor, err := block.NewMetaProcessor(arguments)
	if err != nil {
		return nil, nil, errors.New("could not create block processor: " + err.Error())
	}

	err = processor.SetAppStatusHandler(processArgs.coreData.StatusHandler)
	if err != nil {
		return nil, nil, err
	}

	return processor, txProcessor, nil
}

func indexGenesisAccounts(startTime int64, accountsAdapter state.AccountsAdapter, indexer process.Indexer, marshalizer marshal.Marshalizer) error {
	if indexer.IsNilIndexer() {
		return nil
	}

	rootHash, err := accountsAdapter.RootHash()
	if err != nil {
		return err
	}

	ctx := context.Background()
	leavesChannel, err := accountsAdapter.GetAllLeaves(rootHash, ctx)
	if err != nil {
		return err
	}

	genesisAccounts := make([]state.UserAccountHandler, 0)
	for leaf := range leavesChannel {
		userAccount, errUnmarshal := unmarshalUserAccount(leaf.Key(), leaf.Value(), marshalizer)
		if errUnmarshal != nil {
			log.Debug("cannot unmarshal genesis user account. it may be a code leaf", "error", errUnmarshal)
			continue
		}

		genesisAccounts = append(genesisAccounts, userAccount)
	}

	indexer.SaveAccounts(startTime, genesisAccounts)
	return nil
}

func unmarshalUserAccount(address []byte, userAccountsBytes []byte, marshalizer marshal.Marshalizer) (state.UserAccountHandler, error) {
	userAccount, err := state.NewUserAccount(address)
	if err != nil {
		return nil, err
	}
	err = marshalizer.Unmarshal(userAccount, userAccountsBytes)
	if err != nil {
		return nil, err
	}

	return userAccount, nil
}

func indexGenesisBlock(args *ProcessComponentsFactoryArgs, genesisBlockHeader data.HeaderHandler) error {
	// In Elastic Indexer, only index the chain block
	genesisBlockHash, err := tools.CalculateHash(args.coreData.InternalMarshalizer, args.coreData.Hasher, genesisBlockHeader.GetBlockHeader())
	if err != nil {
		return err
	}

	pool := &indexer.Pool{
		Txs: map[string]data.TransactionHandler{},
	}

	storer := args.data.Store.GetStorer(retriever.TransactionUnit)
	blockTxHashes := genesisBlockHeader.GetTxHashes()
	for _, txHash := range blockTxHashes {
		var tx dataTx.Transaction
		txBytes, err := storer.Get(txHash)
		if err != nil {
			return err
		}

		err = args.coreData.InternalMarshalizer.Unmarshal(&tx, txBytes)
		if err != nil {
			return err
		}

		pool.Txs[string(txHash)] = &tx
	}

	if !args.indexer.IsNilIndexer() {
		log.Info("indexGenesisBlocks(): indexer.SaveBlock", "hash", genesisBlockHash)

		arg := &indexer.ArgsSaveBlockData{
			HeaderHash:       genesisBlockHash,
			Header:           genesisBlockHeader,
			TransactionsPool: pool,
		}
		args.indexer.SaveBlock(arg)
	}

	return nil
}

func newEpochStartTrigger(
	args *ProcessComponentsFactoryArgs,
) (eventNotifier.TriggerHandler, error) {

	argEpochStart := &epochStart.ArgsNewEpochStartTrigger{
		GenesisTime:        time.Unix(args.nodesConfig.StartTime, 0),
		Epoch:              args.startEpochNum,
		SlotsPerEpoch:      args.nodesConfig.SlotsPerEpoch,
		EpochStartSlot:     args.data.Blkc.GetGenesisHeader().GetSlot(),
		EpochStartNotifier: args.epochStartNotifier,
		Storage:            args.data.Store,
		Marshalizer:        args.coreData.InternalMarshalizer,
		ForkController:     args.forkController,
		Hasher:             args.coreData.Hasher,
	}
	epochStartTrigger, err := epochStart.NewEpochStartTrigger(argEpochStart)
	if err != nil {
		return nil, errors.New("error creating new start of epoch trigger" + err.Error())
	}
	err = epochStartTrigger.SetAppStatusHandler(args.coreData.StatusHandler)
	if err != nil {
		return nil, err
	}

	return epochStartTrigger, nil

}

func createMetaTxSimulatorProcessor(
	txSimulatorProcessorArgs *txsimulator.ArgsTxSimulator,
	processArgs *ProcessComponentsFactoryArgs,
	scProcArgs smartContract.ArgsNewSmartContractProcessor,
	kdaTransferParser vmcommon.KDATransferParser,
	gasScheduleNotifier core.GasScheduleNotifier,
) error {

	scProcArgs.VMOutputCacher = txSimulatorProcessorArgs.VMOutputCacher

	scProcArgs.TxFeeHandler = &disabled.TXFeeHandler{}

	readOnlyAccountsCacher, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: processArgs.state.AccountsAdapter,
			Kapps:    processArgs.state.KAppsAdapter,
			Peers:    processArgs.state.PeersAdapter,
		},
	)
	if err != nil {
		return err
	}

	readOnlyAccountsCacher.ResetAll(true)

	// Init KAppController with the readOnlyAccountsCacher for Simulator
	err = processArgs.state.KAppControllerSimulator.InitKApps(readOnlyAccountsCacher)
	if err != nil {
		return err
	}

	argsBuiltIn := builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		AccountsCacher:  readOnlyAccountsCacher,
		KAppController:  processArgs.state.KAppControllerSimulator,
		GasSchedule:     gasScheduleNotifier,
		MapDNSAddresses: make(map[string]struct{}),
		Marshalizer:     processArgs.coreData.InternalMarshalizer,
		EpochNotifier:   processArgs.epochNotifier,
		ForkController:  processArgs.forkController,
	}

	builtInFuncFactory, err := builtInFunctions.CreateBuiltInFunctionsFactory(argsBuiltIn)
	if err != nil {
		return err
	}

	argsHook := hooks.ArgBlockChainHook{
		AccountsCacher:     readOnlyAccountsCacher,
		KAppController:     processArgs.state.KAppControllerSimulator,
		PubkeyConv:         processArgs.state.AddressPubkeyConverter,
		StorageService:     processArgs.data.Store,
		BlockChain:         processArgs.data.Blkc,
		Marshalizer:        processArgs.coreData.InternalMarshalizer,
		Uint64Converter:    processArgs.coreData.Uint64ByteSliceConverter,
		BuiltInFunctions:   builtInFuncFactory.BuiltInFunctionContainer(),
		DataPool:           processArgs.data.Datapool,
		CompiledSCPool:     processArgs.data.Datapool.SmartContracts(),
		EpochNotifier:      processArgs.epochNotifier,
		ForkController:     processArgs.forkController,
		ConfigSCStorage:    processArgs.mainConfig.Storages.SmartContractsStorageSimulate,
		GasSchedule:        gasScheduleNotifier,
		NilCompiledSCStore: false,
		Counter:            counters.NewDisabledCounter(),
		WorkingDir:         processArgs.workingDir,
	}

	blockChainHookImpl, err := hooks.NewBlockChainHookImpl(argsHook)
	if err != nil {
		return err
	}

	argsNewVMFactory := chain.ArgVMContainerFactory{
		BlockChainHook:     blockChainHookImpl,
		BuiltInFunctions:   argsHook.BuiltInFunctions,
		Config:             processArgs.mainConfig.VirtualMachine.Execution,
		EpochNotifier:      processArgs.epochNotifier,
		ForkController:     processArgs.forkController,
		WasmVMChangeLocker: processArgs.coreData.WasmVMChangeLocker,
		KDATransferParser:  kdaTransferParser,
		Hasher:             processArgs.coreData.Hasher,
		GasSchedule:        gasScheduleNotifier,
	}
	virtualMachineFactory, err := chain.NewVMContainerFactory(argsNewVMFactory)
	if err != nil {
		return err
	}

	err = builtInFuncFactory.SetPayableHandler(virtualMachineFactory.BlockChainHookImpl())
	if err != nil {
		return err
	}

	vmContainer, err := virtualMachineFactory.Create()
	if err != nil {
		return err
	}

	scProcArgs.VmContainer = vmContainer
	scProcArgs.BlockChainHook = virtualMachineFactory.BlockChainHookImpl()

	scProcessor, err := smartContract.NewSmartContractProcessor(scProcArgs)
	if err != nil {
		return err
	}

	argsNewMetaTx := transaction.ArgsNewSimulateTxProcessor{
		Hasher:          processArgs.coreData.Hasher,
		Marshalizer:     processArgs.coreData.InternalMarshalizer,
		AccountsCacher:  readOnlyAccountsCacher,
		PubkeyConv:      processArgs.state.AddressPubkeyConverter,
		ScProcessor:     scProcessor,
		EconomicsFee:    processArgs.economicsData,
		ForkController:  processArgs.forkController,
		KAppsController: processArgs.state.KAppControllerSimulator,
		VMOutputCacher:  scProcArgs.VMOutputCacher,
	}
	//Add the meta tx processor to the tx simulator processor args for the simulator instantiation
	txSimulatorProcessorArgs.TransactionProcessor, err = transaction.NewSimulateTxProcessor(argsNewMetaTx)
	if err != nil {
		return err
	}

	return nil
}
