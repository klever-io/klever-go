package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/google/gops/agent"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/node/metrics"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/fallback"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/core/process/interceptors"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/core/redundancy"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/docs"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/genesis/parsing"
	"github.com/klever-io/klever-go/health"
	"github.com/klever-io/klever-go/indexer"
	indexerFactory "github.com/klever-io/klever-go/indexer/factory"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/pathmanager"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/logging"
	"github.com/klever-io/klever-go/tools/marshal"
	factoryMarshalizer "github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/klever-io/klever-go/tools/tracing"
	"github.com/klever-io/klever-go/tools/typeConverters/uint64ByteSlice"
	"github.com/urfave/cli"
)

func startNode(ctx *cli.Context, log logger.Logger, version string) error {
	log.Trace("startNode called")
	chanStopNodeProcess := make(chan endProcess.ArgEndProcess, 1)
	workingDir := getWorkingDir(ctx, log)

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:          ctx.GlobalString(bugsnagKey.Name),
		ReleaseStage:    ctx.GlobalString(bugsnagStage.Name),
		ProjectPackages: []string{"main", "github.com/klever-io/klver-go/**"},
		AppVersion:      version,
		Logger:          logger.NewDummy(),
	})

	var fileLogging tools.FileLoggingHandler
	var err error
	withLogFile := ctx.GlobalBool(logSaveFile.Name)
	if withLogFile {
		fileLogging, err = logging.NewFileLogging(workingDir, defaultLogsPath, logFilePrefix)
		if err != nil {
			return fmt.Errorf("%w creating a log file", err)
		}
	}

	err = logger.SetDisplayByteSlice(logger.ToHex)
	log.LogIfError(err)
	logger.ToggleCorrelation(ctx.GlobalBool(logWithCorrelation.Name))
	logger.ToggleLoggerName(ctx.GlobalBool(logWithLoggerName.Name))
	logLevelFlagValue := ctx.GlobalString(logLevel.Name)
	err = logger.SetLogLevel(logLevelFlagValue)
	if err != nil {
		return err
	}
	noAnsiColor := ctx.GlobalBool(disableAnsiColor.Name)
	if noAnsiColor {
		err = logger.RemoveLogObserver(os.Stdout)
		if err != nil {
			//we need to print this manually as we do not have console log observer
			fmt.Println("error removing log observer: " + err.Error())
			return err
		}

		err = logger.AddLogObserver(os.Stdout, &logger.PlainFormatter{})
		if err != nil {
			//we need to print this manually as we do not have console log observer
			fmt.Println("error setting log observer: " + err.Error())
			return err
		}
	}
	log.Trace("logger updated", "level", logLevelFlagValue, "disable ANSI color", noAnsiColor)

	enableGopsIfNeeded(ctx, log)

	docs.SwaggerInfonode.Host = ctx.GlobalString(swaggerUrl.Name)
	docs.SwaggerInfonode.Schemes = []string{"https", "http"}

	log.Info("starting node", "version", version, "pid", os.Getpid())
	log.Trace("reading configs")

	configurationFileName := ctx.GlobalString(configurationFile.Name)
	cfg, err := config.LoadFromPath(configurationFileName)
	if err != nil {
		return err
	}
	log.Debug("config", "file", cfg)

	externalConfigurationFileName := ctx.GlobalString(externalConfigFile.Name)
	externalConfig, err := config.LoadExternalConfig(externalConfigurationFileName)
	if err != nil {
		return err
	}
	log.Debug("config", "file", externalConfig)

	importDbDirectoryValue := ctx.GlobalString(importDbDirectory.Name)
	var importDBConfigs config.ImportDbConfig
	if importDbDirectoryValue != "" {
		importDBConfigs = config.ImportDbConfig{
			IsImportDBMode:         len(importDbDirectoryValue) > 0,
			ImportDBWorkingDir:     importDbDirectoryValue,
			ImportDbNoSigCheckFlag: ctx.GlobalBool(importDbNoSigCheck.Name),
			ImportDBStartInEpoch:   tools.SafeU64ToU32(ctx.GlobalUint64(importDbStartInEpoch.Name)),
		}
		cfg.ImportDbConfig = importDBConfigs
	}

	// Handle --disable-state-pruning flag
	if ctx.GlobalBool(disableStatePruning.Name) {
		cfg.StateTriesConfig.AccountsStatePruningEnabled = false
		cfg.StateTriesConfig.PeerStatePruningEnabled = false
		cfg.StateTriesConfig.KAppStatePruningEnabled = false
		log.Info("state pruning disabled via --disable-state-pruning flag")
	}

	err = applyCompatibleConfigs(log, &cfg)
	if err != nil {
		return err
	}

	configurationAPIFileName := ctx.GlobalString(configurationAPIFile.Name)
	apiRoutesConfig, err := config.LoadAPIConfig(configurationAPIFileName)
	if err != nil {
		return err
	}
	log.Debug("config", "file", configurationAPIFileName)

	configurationEnableEpochsFileName := ctx.GlobalString(configurationEnableEpochsFile.Name)
	enableEpochsConfig, err := config.LoadEnableEpochsConfig(configurationEnableEpochsFileName)
	if err != nil {
		return err
	}
	log.Debug("config", "file", configurationAPIFileName)
	// rewrite enable epoch in main config
	cfg.EnableEpochs = enableEpochsConfig.EnableEpochs
	cfg.MaxNodesChangeEnableEpoch = enableEpochsConfig.MaxNodesChangeEnableEpoch
	cfg.GasScheduleConfig = enableEpochsConfig.GasSchedule

	if ctx.IsSet(port.Name) {
		cfg.P2P.Node.Port = ctx.GlobalString(port.Name)
	}

	if ctx.IsSet(p2pSeed.Name) {
		cfg.P2P.Node.Seed = ctx.GlobalString(p2pSeed.Name)
	}

	if !check.IfNil(fileLogging) {
		logFileLifeSpan := time.Second * time.Duration(cfg.Logs.LogFileLifeSpanInSec)
		logFileSizeSpan := time.Second * time.Duration(cfg.Logs.LogFileSizeSpanInSec)

		err = fileLogging.SetFileRotation(logFileLifeSpan, logFileSizeSpan, cfg.Logs.MaxBackups, cfg.Logs.MaxFileSize)
		if err != nil {
			return err
		}

		err := fileLogging.ChangeFileLifeSpan(logFileLifeSpan)
		if err != nil {
			return err
		}
	}

	addressPubkeyConverter, err := factory.NewPubkeyConverter(factory.Bech32Format, 32)
	if err != nil {
		return fmt.Errorf("%w for AddressPubKeyConverter", err)
	}
	validatorPubkeyConverter, err := factory.NewPubkeyConverter(factory.HexFormat, 96)
	if err != nil {
		return fmt.Errorf("%w for ValidatorPubkeyConverter", err)
	}

	nodesSetupPath := ctx.GlobalString(nodesFile.Name)
	log.Debug("config", "file", nodesSetupPath)

	genesisNodesConfig, err := sharding.NewNodesSetup(
		nodesSetupPath,
		addressPubkeyConverter,
		validatorPubkeyConverter,
	)
	if err != nil {
		return err
	}

	syncer := ntp.NewSyncTime(cfg.NTP, nil)
	syncer.StartSyncingTime()

	log.Debug("NTP average clock offset", "value", syncer.ClockOffset())

	// Update genesis timestamp for development purposes only
	if genesisNodesConfig.StartTime == 0 {
		time.Sleep(1000 * time.Millisecond)
		ntpTime := syncer.CurrentTime()
		// dev only purpose, we add two seconds
		// in the future to avoid issues with the time
		genesisNodesConfig.StartTime = ntpTime.Unix() + 2
	}

	startTime := time.Unix(genesisNodesConfig.StartTime, 0)

	log.Info("start time",
		"formatted", startTime.Format("Mon Jan 2 15:04:05 MST 2006"),
		"seconds", startTime.Unix())

	log.Trace("getting suite")
	suite, err := getSuite(&cfg)
	if err != nil {
		return err
	}

	validatorKeyPemFileName := ctx.GlobalString(validatorKeyPemFile.Name)
	cryptoParamsLoader, err := factory.NewCryptoSigningParamsLoader(
		validatorPubkeyConverter,
		ctx.GlobalInt(validatorKeyIndex.Name),
		validatorKeyPemFileName,
		suite,
		importDBConfigs.IsImportDBMode,
	)
	if err != nil {
		return err
	}

	cryptoParams, err := cryptoParamsLoader.Get()
	if err != nil {
		return fmt.Errorf("%w: consider regenerating your keys", err)
	}

	log.Debug("block sign pubkey", "value", cryptoParams.PublicKeyString)

	applyPreferencesConfigs(ctx, &cfg)

	err = cleanupStorageIfNecessary(workingDir, ctx, log)
	if err != nil {
		return err
	}

	dbPathWithChainID := filepath.Join(
		workingDir,
		factory.DefaultDBPath,
		genesisNodesConfig.ChainID,
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

	var pathManager *pathmanager.PathManager
	pathManager, err = pathmanager.NewPathManager(pathTemplateForPruningStorer, pathTemplateForStaticStorer, dbPathWithChainID)
	if err != nil {
		return err
	}

	internalMarshalizer, err := factoryMarshalizer.NewMarshalizer(cfg.Marshalizer.Type)
	if err != nil {
		return fmt.Errorf("error creating marshalizer (internal): %s", err.Error())
	}
	uint64ByteSliceConverter := uint64ByteSlice.NewBigEndianConverter()

	log.Trace("creating crypto components")
	cryptoArgs := factory.CryptoComponentsFactoryArgs{
		Config:      cfg,
		NodesConfig: genesisNodesConfig,
		KeyGen:      cryptoParams.KeyGenerator,
		PrivKey:     cryptoParams.PrivateKey,
	}
	cryptoComponentsFactory, err := factory.NewCryptoComponentsFactory(cryptoArgs, importDBConfigs.ImportDbNoSigCheckFlag)
	if err != nil {
		return err
	}
	cryptoComponents, err := cryptoComponentsFactory.Create()
	if err != nil {
		return err
	}

	accountsParser, err := parsing.NewAccountsParser(
		ctx.GlobalString(genesisFile.Name),
		addressPubkeyConverter,
		cryptoComponents.TxSignKeyGen,
	)
	if err != nil {
		return err
	}

	log.Trace("creating core components")

	coreArgs := factory.CoreComponentsFactoryArgs{
		Config:                cfg,
		ChainID:               []byte(genesisNodesConfig.ChainID),
		MinTransactionVersion: genesisNodesConfig.MinTransactionVersion,
	}
	coreComponentsFactory := factory.NewCoreComponentsFactory(coreArgs)
	coreComponents, err := coreComponentsFactory.Create()
	if err != nil {
		return err
	}

	healthService := health.NewHealthService(cfg.Health, workingDir)
	if ctx.IsSet(useHealthService.Name) {
		healthService.Start()
	}

	chanCreateViews := make(chan struct{}, 1)
	chanLogRewrite := make(chan struct{}, 1)

	//  Init Status handler
	handlersArgs, err := factory.NewStatusHandlersFactoryArgs(
		useLogView.Name,
		ctx,
		internalMarshalizer,
		uint64ByteSliceConverter,
		chanCreateViews,
		chanLogRewrite,
	)
	if err != nil {
		return err
	}

	statusHandlersInfo, err := factory.CreateStatusHandlers(handlersArgs)
	if err != nil {
		return err
	}

	coreComponents.StatusHandler = statusHandlersInfo.StatusHandler

	startSlot := int64(0)

	log.Trace("creating slotManager")
	sm, err := slot.NewSlotManager(
		time.Unix(genesisNodesConfig.StartTime, 0),
		syncer.CurrentTime(),
		time.Duration(genesisNodesConfig.SlotInterval)*time.Millisecond, // #nosec G115
		syncer,
		startSlot,
	)
	if err != nil {
		return fmt.Errorf("%w: not able to init slot manager", err)
	}

	log.Trace("creating network components")
	networkComponentFactory, err := factory.NewNetworkComponentsFactory(
		cfg,
		statusHandlersInfo.StatusHandler,
		internalMarshalizer,
		syncer,
	)
	if err != nil {
		return err
	}
	networkComponents, err := networkComponentFactory.Create()
	if err != nil {
		return err
	}

	// skip messengers bootstrap for import db mode
	if !importDBConfigs.IsImportDBMode {
		err := networkComponents.NetMessenger.Bootstrap()
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("waiting %d seconds for network discovery...", secondsToWaitForP2PBootstrap))
		time.Sleep(secondsToWaitForP2PBootstrap * time.Second)
	} else {
		log.Info("import db mode: skipping network discovery")
	}

	log.Trace("creating ratings data components")
	ratingDataArgs := rating.RatingsDataArg{
		Config:                   cfg.Ratings,
		ConsensusSize:            genesisNodesConfig.ConsensusGroupSize,
		MinNodes:                 genesisNodesConfig.MinNodes,
		SlotDurationMilliseconds: genesisNodesConfig.SlotInterval,
	}
	ratingsData, err := rating.NewRatingsData(ratingDataArgs)
	if err != nil {
		return err
	}

	rater, err := rating.NewBlockSigningRater(ratingsData)
	if err != nil {
		return err
	}

	argsNodesShuffler := &sharding.NodesShufflerArgs{
		Nodes:                genesisNodesConfig.MinNumberOfNodes(),
		MaxNodesEnableConfig: cfg.MaxNodesChangeEnableEpoch,
	}

	nodesShuffler, err := sharding.NewHashValidatorsShuffler(argsNodesShuffler)
	if err != nil {
		return err
	}

	bootstrapDataProvider, err := storageFactory.NewBootstrapDataProvider(coreComponents.InternalMarshalizer)
	if err != nil {
		return err
	}

	cfg.StartInEpoch = ctx.GlobalBool(startInEpoch.Name)

	latestStorageDataProvider, err := factory.CreateLatestStorageDataProvider(
		bootstrapDataProvider,
		coreComponents.InternalMarshalizer,
		coreComponents.Hasher,
		cfg,
		genesisNodesConfig.ChainID,
		workingDir,
		factory.DefaultDBPath,
		factory.DefaultEpochString,
	)
	if err != nil {
		return err
	}

	unitOpener, err := factory.CreateUnitOpener(
		bootstrapDataProvider,
		latestStorageDataProvider,
		coreComponents.InternalMarshalizer,
		cfg,
		genesisNodesConfig.ChainID,
		workingDir,
		factory.DefaultDBPath,
		factory.DefaultEpochString,
	)
	if err != nil {
		return err
	}

	versionsCache, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(cfg.Versions.Cache))
	if err != nil {
		return err
	}

	headerIntegrityVerifier, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte(genesisNodesConfig.ChainID),
		cfg.Versions.VersionsByEpochs,
		cfg.Versions.DefaultVersion,
		versionsCache,
	)
	if err != nil {
		return err
	}

	epochNotifier := notifier.NewGenericEpochNotifier()

	epochStartNotifier := notifier.NewEpochStartSubscriptionHandler()

	forkController, err := fork.NewForkController(cfg.EnableEpochs, epochNotifier)
	if err != nil {
		return err
	}

	// epoch bootstrapper
	epochStartBootstrapArgs := bootstrap.ArgsEpochStartBootstrap{
		PublicKey:                 cryptoParams.PublicKey,
		Marshalizer:               coreComponents.InternalMarshalizer,
		TxSignMarshalizer:         coreComponents.TxSignMarshalizer,
		Hasher:                    coreComponents.Hasher,
		Messenger:                 networkComponents.NetMessenger,
		GeneralConfig:             cfg,
		SingleSigner:              cryptoComponents.TxSingleSigner,
		BlockSingleSigner:         cryptoComponents.SingleSigner,
		KeyGen:                    cryptoComponents.TxSignKeyGen,
		BlockKeyGen:               cryptoComponents.BlockSignKeyGen,
		GenesisNodesConfig:        genesisNodesConfig,
		PathManager:               pathManager,
		StorageUnitOpener:         unitOpener,
		WorkingDir:                workingDir,
		DefaultDBPath:             factory.DefaultDBPath,
		DefaultEpochString:        factory.DefaultEpochString,
		Rater:                     rater,
		Uint64Converter:           coreComponents.Uint64ByteSliceConverter,
		NodeShuffler:              nodesShuffler,
		SlotManager:               sm,
		AddressPubkeyConverter:    addressPubkeyConverter,
		LatestStorageDataProvider: latestStorageDataProvider,
		StatusHandler:             coreComponents.StatusHandler,
		HeaderIntegrityVerifier:   headerIntegrityVerifier,
		TxSignHasher:              coreComponents.TxSignHasher,
		EpochNotifier:             epochNotifier,
		ForkController:            forkController,
	}

	var bootstrapper bootstrap.EpochStartBootstrapper
	if importDBConfigs.IsImportDBMode {
		storageArg := bootstrap.ArgsStorageEpochStartBootstrap{
			ArgsEpochStartBootstrap:    epochStartBootstrapArgs,
			ImportDbConfig:             importDBConfigs,
			ChanGracefullyClose:        chanStopNodeProcess,
			TimeToWaitForRequestedData: time.Minute,
		}

		bootstrapper, err = bootstrap.NewStorageEpochStartBootstrap(storageArg)
		if err != nil {
			return err
		}
	} else {
		bootstrapper, err = bootstrap.NewEpochStartBootstrap(epochStartBootstrapArgs)
		if err != nil {
			return err
		}
	}

	bootstrapParameters, err := bootstrapper.Bootstrap()
	if err != nil {
		log.Error("bootstrap return error", "error", err)
		return err
	}

	trieContainer, trieStorageManager := bootstrapper.GetTriesComponents()
	triesComponents := &factory.TriesComponents{
		TriesContainer:      trieContainer,
		TrieStorageManagers: trieStorageManager,
	}

	processingMode := core.Normal
	if importDBConfigs.IsImportDBMode {
		processingMode = core.ImportDb
	}

	log.Trace("creating state components")
	stateArgs := factory.StateComponentsFactoryArgs{
		Config:         cfg,
		Core:           coreComponents,
		PathManager:    pathManager,
		Tries:          triesComponents,
		RatingsData:    ratingsData,
		ProcessingMode: processingMode,
	}
	stateComponentsFactory, err := factory.NewStateComponentsFactory(stateArgs)
	if err != nil {
		return err
	}
	stateComponents, err := stateComponentsFactory.Create(forkController)
	if err != nil {
		return err
	}

	log.Info("bootstrap parameters", "epoch", bootstrapParameters.Epoch)

	// get epoch from storer
	storerEpoch := bootstrapParameters.Epoch

	log.Trace("creating data data components")
	dataArgs := factory.DataComponentsFactoryArgs{
		Config:             cfg,
		Core:               coreComponents,
		PathManager:        pathManager,
		EpochStartNotifier: epochStartNotifier,
		CurrentEpoch:       storerEpoch,
	}
	dataComponentsFactory, err := factory.NewDataComponentsFactory(dataArgs)
	if err != nil {
		return err
	}
	dataComponents, err := dataComponentsFactory.Create()
	if err != nil {
		return err
	}

	log.Trace("initializing stats file")
	err = initStatsFileMonitor(&cfg, pathManager)
	if err != nil {
		return err
	}

	log.Trace("creating data components")

	slotsPerEpoch := genesisNodesConfig.SlotsPerEpoch
	// total supply is updated on genesis block
	totalSupply := int64(0)

	log.Trace("initializing metrics")
	err = metrics.InitMetrics(
		statusHandlersInfo.StatusHandler,
		cryptoParams.PublicKeyString,
		core.NodeTypeValidator,
		genesisNodesConfig,
		version,
		strconv.FormatInt(totalSupply, 10),
		slotsPerEpoch,
	)
	if err != nil {
		return err
	}

	chanLogRewrite <- struct{}{}
	chanCreateViews <- struct{}{}

	err = statusHandlersInfo.UpdateStorerAndMetricsForPersistentHandler(dataComponents.Store.GetStorer(retriever.StatusMetricsUnit))
	if err != nil {
		return err
	}

	log.Trace("creating nodes coordinator")
	if ctx.IsSet(keepOldEpochsData.Name) {
		cfg.StoragePruning.CleanOldEpochsData = !ctx.GlobalBool(keepOldEpochsData.Name)
	}

	nodesCoordinator, err := createNodesCoordinator(
		genesisNodesConfig,
		coreComponents,
		cryptoParams,
		epochStartNotifier,
		dataComponents.Store.GetStorer(retriever.BootstrapUnit),
		nodesShuffler,
		bootstrapParameters,
		storerEpoch,
	)
	if err != nil {
		return err
	}

	metrics.SaveStringMetric(coreComponents.StatusHandler, core.MetricNodeDisplayName, cfg.Preferences.NodeDisplayName)
	metrics.SaveStringMetric(coreComponents.StatusHandler, core.MetricChainID, genesisNodesConfig.ChainID)

	sessionInfoFileOutput := fmt.Sprintf("%s:%s\n%s:%s\n%s:%v\n",
		"PkBlockSign", cryptoParams.PublicKeyString,
		"AppVersion", version,
		"GenesisTimestamp", startTime.Unix(),
	)

	sessionInfoFileOutput += "\nStarted with parameters:\n"
	for _, flag := range ctx.App.Flags {
		flagValue := fmt.Sprintf("%v", ctx.GlobalGeneric(flag.GetName()))
		if flagValue != "" {
			sessionInfoFileOutput += fmt.Sprintf("%s = %v\n", flag.GetName(), flagValue)
		}
	}

	statsFolder := filepath.Join(workingDir, defaultStatsPath)
	copyConfigToStatsFolder(
		log,
		statsFolder,
		[]string{
			configurationFileName,
			configurationAPIFileName,
			ctx.GlobalString(genesisFile.Name),
			ctx.GlobalString(nodesFile.Name),
		})

	statsFile := filepath.Join(statsFolder, "session.info")
	err = os.WriteFile(statsFile, []byte(sessionInfoFileOutput), common.DefaultDirPermission)
	log.LogIfError(err)

	log.Trace("creating tps benchmark components")
	initialTpsBenchmark := statusHandlersInfo.LoadTpsBenchmarkFromStorage(
		dataComponents.Store.GetStorer(retriever.StatusMetricsUnit),
		coreComponents.InternalMarshalizer,
	)
	tpsBenchmark, err := statistics.NewTPSBenchmarkWithInitialData(
		statusHandlersInfo.StatusHandler,
		initialTpsBenchmark,
		genesisNodesConfig.SlotInterval/1000,
	)
	if err != nil {
		return err
	}

	esIndexer, err := createElasticIndexer(
		externalConfig.ElasticSearchConnector,
		coreComponents.InternalMarshalizer,
		coreComponents.Hasher,
		nodesCoordinator,
		epochStartNotifier,
		addressPubkeyConverter,
		validatorPubkeyConverter,
		stateComponents.AccountsAdapter,
		stateComponents.KAppsAdapter,
		stateComponents.KAppController,
		int(genesisNodesConfig.KlvDenomination),
		importDBConfigs.IsImportDBMode,
	)
	if err != nil {
		return err
	}

	eventsProcessor, err := indexer.NewEventsProcessor(indexer.ArgEventsProcessor{
		Marshalizer:              coreComponents.InternalMarshalizer,
		Hasher:                   coreComponents.Hasher,
		AddressPubkeyConverter:   addressPubkeyConverter,
		ValidatorPubkeyConverter: validatorPubkeyConverter,
		Indexer:                  esIndexer,
		KAppController:           stateComponents.KAppController,
		AccountsDB:               stateComponents.AccountsAdapter,
	})
	if err != nil {
		return err
	}

	if externalConfig.Websocket.Enabled {
		indexer.UseEventQueue = true
	}

	log.Trace("creating time cache for requested items components")
	// #nosec G115
	requestedItemsHandler := timecache.NewTimeCache(time.Duration(uint64(time.Millisecond) * genesisNodesConfig.SlotInterval))

	whiteListCache, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(cfg.WhiteListPool))
	if err != nil {
		return err
	}
	whiteListRequest, err := interceptors.NewWhiteListDataVerifier(whiteListCache)
	if err != nil {
		return err
	}

	whiteListerVerifiedTxs, err := createWhiteListerVerifiedTxs(&cfg)
	if err != nil {
		return err
	}

	fallbackHeaderValidator, err := fallback.NewFallbackHeaderValidator(
		dataComponents.Datapool.Headers(),
		coreComponents.InternalMarshalizer,
		dataComponents.Store,
	)
	if err != nil {
		return err
	}

	accCacher, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: stateComponents.AccountsAdapter,
			Kapps:    stateComponents.KAppsAdapter,
			Peers:    stateComponents.PeersAdapter,
		},
	)
	if err != nil {
		return err
	}

	// reset accounts KApp to use cache based on ProcessorFlowITOPrice fork
	accCacher.ResetAll(forkController.ProcessorFlowITOPrice())

	err = stateComponents.KAppController.InitKApps(accCacher)
	if err != nil {
		return err
	}

	argsNewEconomicsData := economics.ArgsNewEconomicsData{
		EpochNotifier: epochNotifier,
	}
	economicsData, err := economics.NewEconomicsData(argsNewEconomicsData)
	if err != nil {
		return err
	}

	txLogsStorage := dataComponents.Store.GetStorer(retriever.TxLogsUnit)

	if !cfg.LogsAndEvents.SaveInStorageEnabled && cfg.DbLookupExtensions.Enabled {
		log.Warn("processComponentsFactory.Create() node will save logs in storage because DbLookupExtensions is enabled")
	}

	saveLogsInStorage := cfg.LogsAndEvents.SaveInStorageEnabled || cfg.DbLookupExtensions.Enabled
	txLogsProcessor, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:               txLogsStorage,
		Marshalizer:          coreComponents.InternalMarshalizer,
		SaveInStorageEnabled: saveLogsInStorage,
	})
	if err != nil {
		return err
	}

	log.Trace("creating process components")
	processArgs := factory.NewProcessComponentsFactoryArgs(
		&coreArgs,
		accountsParser,
		economicsData,
		genesisNodesConfig,
		nodesCoordinator,
		dataComponents,
		coreComponents,
		cryptoComponents,
		stateComponents,
		networkComponents,
		triesComponents,
		requestedItemsHandler,
		whiteListRequest,
		whiteListerVerifiedTxs,
		epochStartNotifier,
		cfg,
		rater,
		ratingsData,
		storerEpoch,
		validatorPubkeyConverter,
		version,
		coreComponents.Uint64ByteSliceConverter,
		workingDir,
		esIndexer,
		eventsProcessor,
		tpsBenchmark,
		epochNotifier,
		ctx.GlobalString(importDbDirectory.Name),
		sm,
		chanStopNodeProcess,
		fallbackHeaderValidator,
		!importDBConfigs.IsImportDBMode,
		accCacher,
		forkController,
		txLogsProcessor,
		processingMode,
	)
	processComponents, err := factory.ProcessComponentsFactory(processArgs)
	if err != nil {
		return err
	}

	observerBLSPrivateKey, observerBLSPublicKey := cryptoComponents.BlockSignKeyGen.GeneratePair()
	observerBLSPublicKeyBuff, err := observerBLSPublicKey.ToByteArray()
	if err != nil {
		log.Error("error generating observerBLSPublicKeyBuff", "error", err)
	} else {
		log.Debug("generated BLS private key for redundancy handler. This key will be used on heartbeat messages "+
			"if the node is in backup mode and the main node is active", "hex public key", observerBLSPublicKeyBuff)
	}
	arg := redundancy.ArgNodeRedundancy{
		RedundancyLevel:    cfg.Preferences.RedundancyLevel,
		Messenger:          networkComponents.NetMessenger,
		ObserverPrivateKey: observerBLSPrivateKey,
	}

	nodeRedundancy, err := redundancy.NewNodeRedundancy(arg)
	if err != nil {
		return err
	}

	log.Trace("creating node structure")
	currentNode, err := createNode(
		&cfg,
		&enableEpochsConfig,
		genesisNodesConfig,
		economicsData,
		syncer,
		cryptoParams.KeyGenerator,
		cryptoParams.PrivateKey,
		cryptoParams.PublicKey,
		nodesCoordinator,
		coreComponents,
		stateComponents,
		dataComponents,
		cryptoComponents,
		processComponents,
		networkComponents,
		ctx.GlobalUint64(bootstrapSlotIndex.Name),
		sm,
		version,
		esIndexer,
		requestedItemsHandler,
		epochStartNotifier,
		whiteListRequest,
		whiteListerVerifiedTxs,
		storerEpoch,
		chanStopNodeProcess,
		fallbackHeaderValidator,
		nodeRedundancy,
		importDBConfigs.IsImportDBMode,
		tools.SafeU64ToU32(ctx.GlobalUint64(syncUntil.Name)),
		ctx.GlobalBool(startWithInSync.Name),
	)
	if err != nil {
		return err
	}

	log.Trace("creating software checker structure")
	softwareVersionChecker, err := factory.CreateSoftwareVersionChecker(coreComponents.StatusHandler, cfg.SoftwareVersionConfig)
	if err != nil {
		log.Debug("nil software version checker", "error", err.Error())
	} else {
		softwareVersionChecker.StartCheckSoftwareVersion()
	}

	log.Trace("activating nodesCoordinator's validators indexing")

	wasmVMChangeLocker := &sync.RWMutex{}
	argsGasScheduleNotifier := notifier.ArgsNewGasScheduleNotifier{
		GasScheduleConfig:  cfg.GasScheduleConfig,
		EpochNotifier:      epochNotifier,
		WasmVMChangeLocker: wasmVMChangeLocker,
	}

	gasScheduleNotifier, err := notifier.NewGasScheduleNotifier(argsGasScheduleNotifier)
	if err != nil {
		return err
	}

	// this channel will trigger the moment when the sc query service should be able to process VM Query requests
	allowExternalVMQueriesChan := make(chan struct{})

	log.Trace("creating api resolver structure")
	apiWorkingDir := filepath.Join(workingDir, factory.TemporaryPath)
	apiResolver, err := createAPIResolver(
		&cfg,
		stateComponents.AccountsAdapter,
		coreComponents.InternalMarshalizer,
		statusHandlersInfo.StatusMetrics,
		economicsData,
		epochNotifier,
		apiWorkingDir,
		coreComponents,
		stateComponents,
		dataComponents,
		processComponents,
		gasScheduleNotifier,
		forkController,
		rater,
		allowExternalVMQueriesChan,
		currentNode.GetNodeState,
	)
	if err != nil {
		return err
	}

	// TODO: remove this and treat better the VM versions switching
	go func(statusHandler core.AppStatusHandler) {
		time.Sleep(2 * time.Second)
		close(allowExternalVMQueriesChan)
		statusHandler.SetStringValue(core.MetricAreVMQueriesReady, strconv.FormatBool(true))
	}(currentNode.GetAppStatusHandler())

	log.Trace("starting status pooling components")
	statusPollingInterval := time.Duration(cfg.Preferences.StatusPollingIntervalSec) * time.Second
	statusPollingCloser, err := metrics.StartStatusPolling(
		currentNode.GetAppStatusHandler(),
		statusPollingInterval,
		networkComponents,
		processComponents,
	)
	if err != nil {
		return err
	}

	updateMachineStatisticsDuration := time.Second
	machineStatsCloser, err := metrics.StartMachineStatisticsPolling(coreComponents.StatusHandler, epochStartNotifier, updateMachineStatisticsDuration, workingDir)
	if err != nil {
		return err
	}

	nodeMetricsCloser, err := metrics.StartNodeMetricsPolling(coreComponents.StatusHandler, statusPollingInterval, nodeRedundancy)
	if err != nil {
		return err
	}

	// Pollers are independent; order doesn't matter.
	pollingClosers := []io.Closer{statusPollingCloser, machineStatsCloser, nodeMetricsCloser}

	log.Trace("creating klever node facade")
	restAPIServerDebugMode := ctx.GlobalBool(restAPIDebug.Name)

	argNodeFacade := facade.ArgNodeFacade{
		Node:                   currentNode,
		APIResolver:            apiResolver,
		RestAPIServerDebugMode: restAPIServerDebugMode,
		WsAntifloodConfig:      cfg.Antiflood.WebServer,
		FacadeConfig: config.FacadeConfig{
			RestAPIInterface:   ctx.GlobalString(restAPIInterface.Name),
			PprofEnabled:       ctx.GlobalBool(profileMode.Name),
			WSConnectionAPIKey: ctx.GlobalString(WSConnectionApiKey.Name),
			WSConnectionURL:    ctx.GlobalString(WSConnectionUrl.Name),
		},
		APIRoutesConfig: apiRoutesConfig,
		AccountsState:   stateComponents.AccountsAdapter,
		KAppsState:      stateComponents.KAppsAdapter,
		PeerState:       stateComponents.PeersAdapter,
	}

	ef, err := facade.NewNodeFacade(argNodeFacade)
	if err != nil {
		return fmt.Errorf("%w while creating NodeFacade", err)
	}

	ef.SetSyncer(syncer)
	ef.SetTpsBenchmark(tpsBenchmark)

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Trace("starting background services")
	ef.StartBackgroundServices(appCtx)
	log.Debug("starting node...")
	err = ef.StartNode()
	if err != nil {
		log.Error("starting node failed", "epoch", storerEpoch, "error", err.Error())
		return err
	}

	log.Info("application is now running")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	var sig endProcess.ArgEndProcess
	select {
	case s := <-sigs:
		log.Info("terminating at user's signal...", "signal", s)
	case sig = <-chanStopNodeProcess:
		log.Info("terminating at internal stop signal", "reason", sig.Reason, "description", sig.Description)
	}

	chanCloseComponents := make(chan struct{})
	go func() {
		closeAllComponents(log, healthService, dataComponents, triesComponents, networkComponents, pollingClosers, chanCloseComponents)
	}()

	_ = tracing.Shutdown() // errors logged internally

	select {
	case <-chanCloseComponents:
	case <-time.After(maxTimeToClose):
		log.Warn("force closing the node", "error", "closeAllComponents did not finished on time")
	}

	log.Debug("closing node")
	if !check.IfNil(fileLogging) {
		err = fileLogging.Close()
		log.LogIfError(err)
	}

	return nil
}

func getWorkingDir(ctx *cli.Context, log logger.Logger) string {
	var workingDir string
	var err error
	if ctx.IsSet(workingDirectory.Name) {
		workingDir = ctx.GlobalString(workingDirectory.Name)
	} else {
		workingDir, err = os.Getwd()
		if err != nil {
			log.LogIfError(err)
			workingDir = ""
		}
	}
	log.Trace("working directory", "path", workingDir)

	return workingDir
}

func enableGopsIfNeeded(ctx *cli.Context, log logger.Logger) {
	var gopsEnabled bool
	if ctx.IsSet(gopsEn.Name) {
		gopsEnabled = ctx.GlobalBool(gopsEn.Name)
	}

	if gopsEnabled {
		if err := agent.Listen(agent.Options{}); err != nil {
			log.Error("failure to init gops", "error", err.Error())
		}
	}

	log.Trace("gops", "enabled", gopsEnabled)
}

func cleanupStorageIfNecessary(workingDir string, ctx *cli.Context, log logger.Logger) error {
	storageCleanupFlagValue := ctx.GlobalBool(storageCleanup.Name)
	if storageCleanupFlagValue {
		dbPath := filepath.Join(
			workingDir,
			factory.DefaultDBPath)
		log.Trace("cleaning storage", "path", dbPath)
		err := os.RemoveAll(dbPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func applyPreferencesConfigs(ctx *cli.Context, cfg *config.Config) {
	if ctx.IsSet(nodeDisplayName.Name) {
		cfg.Preferences.NodeDisplayName = ctx.GlobalString(nodeDisplayName.Name)
	}

	if len(cfg.Preferences.NodeDisplayName) == 0 {
		petname.NonDeterministicMode()
		nodeName := petname.Generate(2, "-")
		cfg.Preferences.NodeDisplayName = emptyNamePrefix + "-" + nodeName
	}

	if ctx.IsSet(identityFlagName.Name) {
		cfg.Preferences.Identity = ctx.GlobalString(identityFlagName.Name)
	}

	if ctx.IsSet(redundancyLevel.Name) {
		cfg.Preferences.RedundancyLevel = ctx.GlobalInt64(redundancyLevel.Name)
	}
}

func closeAllComponents(
	log logger.Logger,
	healthService io.Closer,
	dataComponents *factory.DataComponents,
	triesComponents *factory.TriesComponents,
	networkComponents *factory.NetworkComponents,
	pollingClosers []io.Closer,
	chanCloseComponents chan struct{},
) {
	// Stop polling first so handlers cannot fire against components being torn down below.
	log.Debug("closing status polling goroutines...")
	for _, c := range pollingClosers {
		if c == nil {
			continue
		}
		log.LogIfError(c.Close())
	}

	log.Debug("closing health service...")
	err := healthService.Close()
	log.LogIfError(err)

	log.Debug("closing all store units....")
	err = dataComponents.Store.CloseAll()
	log.LogIfError(err)

	dataTries := triesComponents.TriesContainer.GetAll()
	for _, trie := range dataTries {
		err = trie.ClosePersister()
		log.LogIfError(err)
	}

	log.Debug("calling close on the network messenger instance...")
	err = networkComponents.NetMessenger.Close()
	log.LogIfError(err)

	chanCloseComponents <- struct{}{}
}

func createWhiteListerVerifiedTxs(generalConfig *config.Config) (process.WhiteListHandler, error) {
	whiteListCacheVerified, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(generalConfig.WhiteListerVerifiedTxs))
	if err != nil {
		return nil, err
	}
	return interceptors.NewWhiteListDataVerifier(whiteListCacheVerified)
}

func getSuite(config *config.Config) (crypto.Suite, error) {
	return mcl.NewSuiteBLS12(), nil
}

func copyConfigToStatsFolder(log logger.Logger, statsFolder string, configs []string) {
	err := os.MkdirAll(statsFolder, common.DefaultDirPermission)
	log.LogIfError(err)

	for _, configFile := range configs {
		copySingleFile(statsFolder, configFile)
	}
}

func copySingleFile(folder string, configFile string) {
	fileName := filepath.Base(configFile)

	source, err := tools.OpenFile(configFile)
	if err != nil {
		return
	}
	defer func() {
		err = source.Close()
		if err != nil {
			fmt.Printf("Could not close %s\n", source.Name())
		}
	}()

	destPath := filepath.Join(folder, fileName)
	destination, err := os.Create(filepath.Clean(destPath))
	if err != nil {
		return
	}
	defer func() {
		err = destination.Close()
		if err != nil {
			fmt.Printf("Could not close %s\n", source.Name())
		}
	}()

	_, err = io.Copy(destination, source)
	if err != nil {
		fmt.Printf("Could not copy %s\n", source.Name())
	}
}

// createElasticIndexer creates a new elasticIndexer where the server listens on the url,
// authentication for the server is using the username and password
func createElasticIndexer(
	elasticSearchConfig config.ElasticSearchConfig,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	nodesCoordinator sharding.NodesCoordinator,
	startNotifier notifier.EpochStartNotifier,
	addressPubkeyConverter core.PubkeyConverter,
	validatorPubkeyConverter core.PubkeyConverter,
	accountsDB state.AccountsAdapter,
	kappsDB state.AccountsAdapter,
	kAppController kapp.KAppController,
	denomination int,
	isInImportDBMode bool,
) (process.Indexer, error) {

	indexerFactoryArgs := &indexerFactory.ArgsIndexerFactory{
		Enabled:                  elasticSearchConfig.Enabled,
		IndexerCacheSize:         elasticSearchConfig.IndexerCacheSize,
		Url:                      elasticSearchConfig.URL,
		UserName:                 elasticSearchConfig.Username,
		Password:                 elasticSearchConfig.Password,
		Marshalizer:              marshalizer,
		Hasher:                   hasher,
		EpochStartNotifier:       startNotifier,
		NodesCoordinator:         nodesCoordinator,
		AddressPubkeyConverter:   addressPubkeyConverter,
		ValidatorPubkeyConverter: validatorPubkeyConverter,
		EnabledIndexes:           elasticSearchConfig.EnabledIndexes,
		AccountsDB:               accountsDB,
		KappsDB:                  kappsDB,
		KAppController:           kAppController,
		Denomination:             denomination,
		UseKibana:                elasticSearchConfig.UseKibana,
		IsInImportDBMode:         isInImportDBMode,
	}

	return indexerFactory.NewIndexer(indexerFactoryArgs)
}

func applyCompatibleConfigs(log logger.Logger, configs *config.Config) error {
	importDbFlags := configs.ImportDbConfig
	importDbFlags.ImportDbNoSigCheckFlag = importDbFlags.ImportDbNoSigCheckFlag && importDbFlags.IsImportDBMode

	if importDbFlags.IsImportDBMode {
		return processConfigImportDBMode(log, configs)
	}

	//TODO: Implement Full Archive Mode
	// if FullArchive is enabled, we override the conflicting StoragePruning settings and StartInEpoch as well
	// if configs.PreferencesConfig.Preferences.FullArchive {
	// 	return processConfigFullArchiveMode(log, configs)
	// }

	return nil
}

func processConfigImportDBMode(log logger.Logger, configs *config.Config) error {
	importDbFlags := &configs.ImportDbConfig
	generalConfigs := configs
	p2pConfigs := &configs.P2P

	if importDbFlags.ImportDBStartInEpoch == 0 {
		generalConfigs.StartInEpoch = false
	}

	generalConfigs.Preferences.StatusPollingIntervalSec = 60
	generalConfigs.Debug.InterceptorResolver.Enabled = false
	generalConfigs.Debug.Antiflood.Enabled = false
	generalConfigs.Antiflood.Enabled = false

	generalConfigs.StoragePruning.NumActivePersisters = generalConfigs.StoragePruning.NumEpochsToKeep
	generalConfigs.StateTriesConfig.CheckpointSlotsModulus = math.MaxUint32
	generalConfigs.TrieStorageManagerConfig.MaxSnapshots = math.MaxUint32

	p2pConfigs.Node.ThresholdMinConnectedPeers = 0
	p2pConfigs.KadDhtPeerDiscovery.Enabled = false
	p2pConfigs.KadDhtPeerDiscovery.InitialPeerList = make([]string, 0)

	defaultSecondsToCheckHealth := 300                   // 5minutes
	defaultDiagnoseMemoryLimit := 6 * 1024 * 1024 * 1024 // 6GB

	generalConfigs.Health.IntervalDiagnoseComponentsDeeplyInSeconds = defaultSecondsToCheckHealth
	generalConfigs.Health.IntervalDiagnoseComponentsInSeconds = defaultSecondsToCheckHealth
	generalConfigs.Health.IntervalVerifyMemoryInSeconds = defaultSecondsToCheckHealth
	generalConfigs.Health.MemoryUsageToCreateProfiles = defaultDiagnoseMemoryLimit

	generalConfigs.Heartbeat.DurationToConsiderUnresponsiveInSec = math.MaxInt32
	generalConfigs.Heartbeat.MinTimeToWaitBetweenBroadcastsInSec = math.MaxInt32 - 2
	generalConfigs.Heartbeat.MaxTimeToWaitBetweenBroadcastsInSec = math.MaxInt32 - 1

	alterStorageConfigsForDBImport(generalConfigs)

	log.Warn("the node is in import mode! Will auto-set some config values, including storage config values",
		"GeneralSettings.StartInEpochEnabled", generalConfigs.StartInEpoch,
		// "StateTriesConfig.CheckpointsEnabled", generalConfigs.StateTriesConfig.CheckpointsEnabled,
		"StateTriesConfig.CheckpointRoundsModulus", generalConfigs.StateTriesConfig.CheckpointSlotsModulus,
		"StoragePruning.NumActivePersisters", generalConfigs.StoragePruning.NumEpochsToKeep,
		// "TrieStorageManagerConfig.KeepSnapshots", generalConfigs.TrieStorageManagerConfig.KeepSnapshots,
		"p2p.ThresholdMinConnectedPeers", p2pConfigs.Node.ThresholdMinConnectedPeers,
		"no sig check", importDbFlags.ImportDbNoSigCheckFlag,
		"import DB start in epoch", importDbFlags.ImportDBStartInEpoch,
		"kad dht discoverer", "off",
		"health interval diagnose components deeply in seconds", generalConfigs.Health.IntervalDiagnoseComponentsDeeplyInSeconds,
		"health interval diagnose components in seconds", generalConfigs.Health.IntervalDiagnoseComponentsInSeconds,
		"health interval verify memory in seconds", generalConfigs.Health.IntervalVerifyMemoryInSeconds,
		"health memory usage threshold", generalConfigs.Health.MemoryUsageToCreateProfiles, // todo: check
	)
	return nil
}

func alterStorageConfigsForDBImport(config *config.Config) {
	changeStorageConfigForDBImport(&config.Storages.BlocksStorage)
	changeStorageConfigForDBImport(&config.Storages.HdrNonceHashStorage)
	changeStorageConfigForDBImport(&config.PeerAccountsTrieStorage)
}

func changeStorageConfigForDBImport(storageConfig *config.StorageConfig) {
	alterCoefficient := uint32(10)

	storageConfig.Cache.Capacity = storageConfig.Cache.Capacity * alterCoefficient
	storageConfig.DB.MaxBatchSize = storageConfig.DB.MaxBatchSize * int(alterCoefficient)
}
