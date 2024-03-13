package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/accumulator"
	"github.com/klever-io/klever-go/core/alarm"
	"github.com/klever-io/klever-go/core/bootstrap"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/rating/peerHonesty"
	"github.com/klever-io/klever-go/core/process/throttle/antiflood/blackList"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/core/watchdog"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/node/nodeDebugFactory"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

func createNode(
	config *config.Config,
	enableEpochsConfig *config.EnableEpochsConfig,
	nodesConfig *sharding.NodesSetup,
	economicsData process.EconomicsDataHandler,
	syncer ntp.SyncTimer,
	keyGen crypto.KeyGenerator,
	privKey crypto.PrivateKey,
	pubKey crypto.PublicKey,
	nodesCoordinator sharding.NodesCoordinator,
	coreData *factory.CoreComponents,
	stateComponents *factory.StateComponents,
	data *factory.DataComponents,
	crypto *factory.CryptoComponents,
	process *factory.Process,
	network *factory.NetworkComponents,
	bootstrapSlotIndex uint64,
	sm consensus.SlotManager,
	version string,
	esIndexer process.Indexer,
	requestedItemsHandler retriever.RequestedItemsHandler,
	epochStartRegistrationHandler eventNotifier.RegistrationHandler,
	whiteListRequest process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	epochStart uint32,
	hardForkTrigger node.HardforkTrigger,
	chanStopNodeProcess chan endProcess.ArgEndProcess,
	fallbackHeaderValidator consensus.FallbackHeaderValidator,
	nodeRedundancyHandler consensus.NodeRedundancyHandler,
	isInImportMode bool,
	syncUntil uint32,
	startWithInSync bool,
) (*node.Node, error) {
	var err error

	var txAccumulator node.Accumulator
	txAccumulatorConfig := config.Antiflood.TxAccumulator
	txAccumulator, err = accumulator.NewTimeAccumulator(
		time.Duration(txAccumulatorConfig.MaxAllowedTimeInMilliseconds)*time.Millisecond,
		time.Duration(txAccumulatorConfig.MaxDeviationTimeInMilliseconds)*time.Millisecond,
	)
	if err != nil {
		return nil, err
	}

	networkShardingCollector, err := factory.PrepareNetworkShardingCollector(
		network,
		config,
		nodesCoordinator,
		epochStartRegistrationHandler,
		epochStart,
	)
	if err != nil {
		return nil, err
	}

	factory.PrepareOpenTopics(network.InputAntifloodHandler)

	alarmScheduler := alarm.NewAlarmScheduler()
	watchdogTimer, err := watchdog.NewWatchdog(alarmScheduler, chanStopNodeProcess)
	if err != nil {
		return nil, err
	}

	peerDenialEvaluator, err := blackList.NewPeerDenialEvaluator(
		network.PeerBlackListHandler,
		network.PkTimeCache,
		networkShardingCollector,
	)
	if err != nil {
		return nil, err
	}

	err = network.NetMessenger.SetPeerDenialEvaluator(peerDenialEvaluator)
	if err != nil {
		return nil, err
	}

	peerHonestyHandler, err := createPeerHonestyHandler(config, network.PkTimeCache)
	if err != nil {
		return nil, err
	}

	var nd *node.Node
	nd, err = node.NewNode(
		node.WithMessenger(network.NetMessenger),
		node.WithHasher(coreData.Hasher),
		node.WithInternalMarshalizer(coreData.InternalMarshalizer),
		node.WithTxSignMarshalizer(coreData.TxSignMarshalizer),
		node.WithInitialNodesPubKeys(crypto.InitialPubKeys),
		node.WithAddressPubkeyConverter(stateComponents.AddressPubkeyConverter),
		node.WithValidatorPubkeyConverter(stateComponents.ValidatorPubkeyConverter),
		node.WithAccountsAdapter(stateComponents.AccountsAdapter),
		node.WithKAppsAdapter(stateComponents.KAppsAdapter),
		node.WithKAppController(stateComponents.KAppController),
		node.WithSimulatorKAppController(stateComponents.KAppControllerSimulator),
		node.WithBlockChain(data.Blkc),
		node.WithDataStore(data.Store),
		node.WithConsensusGroupSize(int(nodesConfig.ConsensusGroupSize)),
		node.WithSyncer(syncer),
		node.WithTxFeeHandler(economicsData),
		node.WithBlockProcessor(process.BlockProcessor),
		node.WithTransactionProcessor(process.TransactionProcessor),
		node.WithGenesisTime(time.Unix(nodesConfig.StartTime, 0)),
		node.WithNodesCoordinator(nodesCoordinator),
		node.WithUint64ByteSliceConverter(coreData.Uint64ByteSliceConverter),
		node.WithSingleSigner(crypto.SingleSigner),
		node.WithMultiSigner(crypto.MultiSigner),
		node.WithKeyGen(keyGen),
		node.WithKeyGenForAccounts(crypto.TxSignKeyGen),
		node.WithPubKey(pubKey),
		node.WithPrivKey(privKey),
		node.WithInterceptorsContainer(process.InterceptorsContainer),
		node.WithResolversFinder(process.ResolversFinder),
		node.WithTxSingleSigner(crypto.TxSingleSigner),
		node.WithAppStatusHandler(coreData.StatusHandler),
		node.WithIndexer(esIndexer),
		node.WithEpochStartTrigger(process.EpochStartTrigger),
		node.WithEpochStartEventNotifier(epochStartRegistrationHandler),
		node.WithBlockBlackListHandler(process.BlackListHandler),
		node.WithPeerDenialEvaluator(peerDenialEvaluator),
		node.WithNetworkShardingCollector(networkShardingCollector),
		node.WithConsensusType("bls"),
		node.WithBootstrapSlotIndex(bootstrapSlotIndex),
		node.WithBootStorer(process.BootStorer),
		node.WithRequestedItemsHandler(requestedItemsHandler),
		node.WithValidatorsProvider(process.ValidatorsProvider),
		node.WithChainID(coreData.ChainID),
		node.WithForkDetector(process.ForkDetector),
		node.WithMinTransactionVersion(nodesConfig.MinTransactionVersion),
		node.WithRequestHandler(process.RequestHandler),
		node.WithInputAntifloodHandler(network.InputAntifloodHandler),
		node.WithHardforkTrigger(hardForkTrigger),
		node.WithTxAccumulator(txAccumulator),
		node.WithWhiteListHandler(whiteListRequest),
		node.WithWhiteListHandlerVerified(whiteListerVerifiedTxs),
		node.WithPublicKeySize(96),
		node.WithValidatorSignatureSize(48),
		node.WithNodeStopChannel(chanStopNodeProcess),
		node.WithPeerHonestyHandler(peerHonestyHandler),
		node.WithWatchdogTimer(watchdogTimer),
		node.WithPeerSignatureHandler(crypto.PeerSignatureHandler),
		node.WithTxSignHasher(coreData.TxSignHasher),
		node.WithSlotManager(sm),
		node.WithHeaderSigVerifier(process.HeaderSigVerifier),
		node.WithHeaderIntegrityVerifier(process.HeaderIntegrityVerifier),
		node.WithFallbackHeaderValidator(fallbackHeaderValidator),
		node.WithNodeRedundancyHandler(nodeRedundancyHandler),
		node.WithValidatorStatistics(process.ValidatorsStatistics),
		node.WithImportMode(isInImportMode),
		node.WithEnableEpochsConfig(enableEpochsConfig),
		node.WithForkController(process.ForkController),
		node.WithConsensusMonitorList(config.ConsensusMonitorList),
		node.WithSyncUntil(syncUntil),
		node.WithStartInSync(startWithInSync),
	)
	if err != nil {
		return nil, errors.New("error creating node: " + err.Error())
	}

	err = nd.StartHeartbeat(config.Heartbeat, version, config.Preferences)
	if err != nil {
		return nil, err
	}

	err = nd.ApplyOptions(node.WithDataPool(data.Datapool))
	if err != nil {
		return nil, errors.New("error creating node: " + err.Error())
	}

	err = nd.CreateStores()
	if err != nil {
		return nil, err
	}

	err = nodeDebugFactory.CreateInterceptedDebugHandler(
		nd,
		process.InterceptorsContainer,
		process.ResolversFinder,
		config.Debug.InterceptorResolver,
	)
	if err != nil {
		return nil, err
	}

	return nd, nil
}

func createPeerHonestyHandler(
	cfg *config.Config,
	pkTimeCache process.TimeCacher,
) (consensus.PeerHonestyHandler, error) {

	cache, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(cfg.PeerHonesty))
	if err != nil {
		return nil, err
	}

	return peerHonesty.NewP2pPeerHonesty(cfg.Ratings.PeerHonesty, pkTimeCache, cache)
}

func initStatsFileMonitor(
	config *config.Config,
	pathManager storage.PathManagerHandler,
) error {
	err := startStatisticsMonitor(config, pathManager)
	if err != nil {
		return err
	}

	return nil
}

func startStatisticsMonitor(
	generalConfig *config.Config,
	pathManager storage.PathManagerHandler,
) error {
	if !generalConfig.ResourceStats.Enabled {
		return nil
	}

	if generalConfig.ResourceStats.RefreshIntervalInSec < 1 {
		return errors.New("invalid RefreshIntervalInSec in section [ResourceStats]. Should be an integer higher than 1")
	}

	resMon := statistics.NewResourceMonitor()

	go func() {
		for {
			resMon.SaveStatistics(generalConfig, pathManager, core.MetachainName)
			time.Sleep(time.Second * time.Duration(generalConfig.ResourceStats.RefreshIntervalInSec))
		}
	}()

	return nil
}

func createNodesCoordinator(
	nodesConfig *sharding.NodesSetup,
	coreComponents *factory.CoreComponents,
	cryptoParams *factory.CryptoParams,
	epochStartNotifier sharding.EpochStartEventNotifier,
	bootStorer storage.Storer,
	nodeShuffler sharding.NodesShuffler,
	bootstrapParameters bootstrap.Parameters,
	startEpoch uint32,
) (sharding.NodesCoordinator, error) {

	electedNodesInfo, eligibleNodesInfo, err := nodesConfig.InitialNodesInfo()
	if err != nil {
		return nil, err
	}

	electedValidators, err := sharding.NodesInfoToValidators(electedNodesInfo)
	if err != nil {
		return nil, err
	}

	eligibleValidators, err := sharding.NodesInfoToValidators(eligibleNodesInfo)
	if err != nil {
		return nil, err
	}

	currentEpoch := startEpoch
	if bootstrapParameters.NodesConfig != nil {
		nodeRegistry := bootstrapParameters.NodesConfig
		currentEpoch = bootstrapParameters.Epoch
		epochsConfig, ok := nodeRegistry.EpochsConfig[fmt.Sprintf("%d", currentEpoch)]
		if ok {
			elected := epochsConfig.ElectedValidators
			electedValidators, err = sharding.SerializableValidatorsToValidators(elected)
			if err != nil {
				return nil, err
			}

			eligibles := epochsConfig.EligibleValidators
			eligibleValidators, err = sharding.SerializableValidatorsToValidators(eligibles)
			if err != nil {
				return nil, err
			}
		}
	}

	consensusGroupCache, err := lrucache.NewCache(25000)
	if err != nil {
		return nil, err
	}

	pubKeyBytes, err := cryptoParams.PublicKey.ToByteArray()
	if err != nil {
		return nil, err
	}

	arguments := sharding.ArgNodesCoordinator{
		ConsensusGroupSize:  int(nodesConfig.ConsensusGroupSize),
		Marshalizer:         coreComponents.InternalMarshalizer,
		Hasher:              coreComponents.Hasher,
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartNotifier,
		BootStorer:          bootStorer,
		ElectedNodes:        electedValidators,
		EligibleNodes:       eligibleValidators,
		SelfPublicKey:       pubKeyBytes,
		ConsensusGroupCache: consensusGroupCache,
		Epoch:               currentEpoch,
		StartEpoch:          startEpoch,
	}

	if len(bootstrapParameters.CurrEpochValidatorsInfo) > 0 {
		arguments.CurrValidatorsInfo = bootstrapParameters.CurrEpochValidatorsInfo
	}

	if len(bootstrapParameters.PrevEpochValidatorsInfo) > 0 {
		arguments.PrevValidatorsInfo = bootstrapParameters.PrevEpochValidatorsInfo
	}

	nodesCoordinator, err := sharding.NewNodesCoordinator(arguments)
	if err != nil {
		return nil, err
	}

	if bootstrapParameters.NodesConfig != nil {
		nodeRegistry := bootstrapParameters.NodesConfig
		prevEpochsConfig, ok := nodeRegistry.EpochsConfig[fmt.Sprintf("%d", currentEpoch-1)]
		if ok {
			elected := prevEpochsConfig.ElectedValidators
			electedValidators, err = sharding.SerializableValidatorsToValidators(elected)
			if err != nil {
				return nil, err
			}

			eligibles := prevEpochsConfig.EligibleValidators
			eligibleValidators, err = sharding.SerializableValidatorsToValidators(eligibles)
			if err != nil {
				return nil, err
			}

			waiting := prevEpochsConfig.WaitingValidators
			waitingValidators, err := sharding.SerializableValidatorsToValidators(waiting)
			if err != nil {
				return nil, err
			}

			err = nodesCoordinator.SetNodes(electedValidators, eligibleValidators, waitingValidators, currentEpoch-1)
			if err != nil {
				return nil, err
			}
		}
	}

	return nodesCoordinator, nil
}
