package bootstrap

import (
	"bytes"
	"context"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	factoryInterceptors "github.com/klever-io/klever-go/core/bootstrap/factory"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors"
	disabledInterceptors "github.com/klever-io/klever-go/core/process/interceptors/disabled"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	factoryDataPool "github.com/klever-io/klever-go/data/retriever/factory"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/factory/resolverscontainer"
	"github.com/klever-io/klever-go/data/retriever/requestHandlers"
	"github.com/klever-io/klever-go/data/state"
	factoryState "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/syncer"
	"github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/tools/typeConverters/uint64ByteSlice"
)

var log = logger.GetOrCreate("core/bootstrap")

const timeToWait = time.Minute
const timeoutGettingTrieNode = time.Minute
const timeBetweenRequests = 100 * time.Millisecond
const maxToRequest = 100
const gracePeriodInPercentage = float64(0.25)
const slotGracePeriod = 25

// Parameters defines the DTO for the result produced by the bootstrap component
type Parameters struct {
	Epoch                   uint32
	NodesConfig             *sharding.NodesCoordinatorRegistry
	CurrEpochValidatorsInfo []*state.ValidatorInfo
	PrevEpochValidatorsInfo []*state.ValidatorInfo
}

// ComponentsNeededForBootstrap holds the components which need to be initialized from network
type ComponentsNeededForBootstrap struct {
	EpochStartBlock    *block.Block
	PreviousEpochStart *block.Block
	Header             *block.BlockHeader
	NodesConfig        *sharding.NodesCoordinatorRegistry
	Headers            map[string]data.HeaderHandler
	UserAccountTries   map[string]data.Trie
	PeerAccountTries   map[string]data.Trie
	KAppAccountTries   map[string]data.Trie
}

// epochStartBootstrap will handle requesting the needed data to start when joining late the network
type epochStartBootstrap struct {
	// should come via arguments
	publicKey                 crypto.PublicKey
	marshalizer               marshal.Marshalizer
	txSignMarshalizer         marshal.Marshalizer
	hasher                    hashing.Hasher
	messenger                 Messenger
	generalConfig             config.Config
	singleSigner              crypto.SingleSigner
	blockSingleSigner         crypto.SingleSigner
	keyGen                    crypto.KeyGenerator
	blockKeyGen               crypto.KeyGenerator
	genesisNodesConfig        sharding.GenesisNodesSetupHandler
	pathManager               storage.PathManagerHandler
	workingDir                string
	defaultDBPath             string
	defaultEpochString        string
	rater                     sharding.ChanceComputer
	trieContainer             state.TriesHolder
	trieStorageManagers       map[string]data.StorageManager
	mutTrieStorageManagers    sync.RWMutex
	uint64Converter           typeConverters.Uint64ByteSliceConverter
	nodeShuffler              sharding.NodesShuffler
	slotManager               consensus.SlotManager
	addressPubkeyConverter    core.PubkeyConverter
	statusHandler             core.AppStatusHandler
	headerIntegrityVerifier   process.HeaderIntegrityVerifier
	txSignHasher              hashing.Hasher
	epochNotifier             process.EpochNotifier
	numConcurrentTrieSyncers  int
	maxHardCapForMissingNodes int

	// created components
	requestHandler            process.RequestHandler
	interceptorContainer      process.InterceptorsContainer
	dataPool                  retriever.PoolsHolder
	epochStartMetaBlockSyncer StartOfEpochMetaSyncer
	nodesConfigHandler        StartOfEpochNodesConfigHandler
	whiteListHandler          process.WhiteListHandler
	whiteListerVerifiedTxs    process.WhiteListHandler
	storageOpenerHandler      storage.UnitOpenerHandler
	latestStorageDataProvider storage.LatestStorageDataProviderHandler
	storageService            retriever.StorageService

	// gathered data
	epochStartMeta     *block.Block
	prevEpochStartMeta *block.Block
	syncedHeaders      map[string]data.HeaderHandler
	nodesConfig        *sharding.NodesCoordinatorRegistry
	userAccountTries   map[string]data.Trie
	peerAccountTries   map[string]data.Trie
	kappAccountTries   map[string]data.Trie
	baseData           baseDataInStorage
	startSlot          int64
	nodeType           core.NodeType
	startEpoch         uint32

	chRcvHdrHash   chan bool
	forkController core.ForkController
}

type baseDataInStorage struct {
	lastSlot       int64
	epochStartSlot uint64
	lastEpoch      uint32
	storageExists  bool
}

// ArgsEpochStartBootstrap holds the arguments needed for creating an epoch start data provider component
type ArgsEpochStartBootstrap struct {
	WorkingDir                string
	DefaultDBPath             string
	DefaultEpochString        string
	PublicKey                 crypto.PublicKey
	Marshalizer               marshal.Marshalizer
	TxSignMarshalizer         marshal.Marshalizer
	Hasher                    hashing.Hasher
	Messenger                 Messenger
	GeneralConfig             config.Config
	SingleSigner              crypto.SingleSigner
	BlockSingleSigner         crypto.SingleSigner
	KeyGen                    crypto.KeyGenerator
	BlockKeyGen               crypto.KeyGenerator
	GenesisNodesConfig        sharding.GenesisNodesSetupHandler
	PathManager               storage.PathManagerHandler
	StorageUnitOpener         storage.UnitOpenerHandler
	LatestStorageDataProvider storage.LatestStorageDataProviderHandler
	Rater                     sharding.ChanceComputer
	Uint64Converter           typeConverters.Uint64ByteSliceConverter
	NodeShuffler              sharding.NodesShuffler
	SlotManager               consensus.SlotManager
	AddressPubkeyConverter    core.PubkeyConverter
	StatusHandler             core.AppStatusHandler
	HeaderIntegrityVerifier   process.HeaderIntegrityVerifier
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	ForkController            core.ForkController
}

// NewEpochStartBootstrap will return a new instance of epochStartBootstrap
func NewEpochStartBootstrap(args ArgsEpochStartBootstrap) (*epochStartBootstrap, error) {
	err := checkArguments(args)
	if err != nil {
		return nil, err
	}

	epochStartProvider := &epochStartBootstrap{
		publicKey:                 args.PublicKey,
		marshalizer:               args.Marshalizer,
		txSignMarshalizer:         args.TxSignMarshalizer,
		hasher:                    args.Hasher,
		messenger:                 args.Messenger,
		generalConfig:             args.GeneralConfig,
		genesisNodesConfig:        args.GenesisNodesConfig,
		workingDir:                args.WorkingDir,
		pathManager:               args.PathManager,
		defaultEpochString:        args.DefaultEpochString,
		defaultDBPath:             args.DefaultDBPath,
		keyGen:                    args.KeyGen,
		blockKeyGen:               args.BlockKeyGen,
		singleSigner:              args.SingleSigner,
		blockSingleSigner:         args.BlockSingleSigner,
		rater:                     args.Rater,
		uint64Converter:           args.Uint64Converter,
		nodeShuffler:              args.NodeShuffler,
		slotManager:               args.SlotManager,
		storageOpenerHandler:      args.StorageUnitOpener,
		latestStorageDataProvider: args.LatestStorageDataProvider,
		addressPubkeyConverter:    args.AddressPubkeyConverter,
		statusHandler:             args.StatusHandler,
		nodeType:                  core.NodeTypeObserver,
		headerIntegrityVerifier:   args.HeaderIntegrityVerifier,
		txSignHasher:              args.TxSignHasher,
		epochNotifier:             args.EpochNotifier,
		numConcurrentTrieSyncers:  args.GeneralConfig.TrieSync.NumConcurrentTrieSyncers,
		maxHardCapForMissingNodes: args.GeneralConfig.TrieSync.MaxHardCapForMissingNodes,
		chRcvHdrHash:              make(chan bool),
		forkController:            args.ForkController,
	}

	whiteListCache, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(epochStartProvider.generalConfig.WhiteListPool))
	if err != nil {
		return nil, err
	}

	epochStartProvider.whiteListHandler, err = interceptors.NewWhiteListDataVerifier(whiteListCache)
	if err != nil {
		return nil, err
	}

	epochStartProvider.whiteListerVerifiedTxs, err = disabledInterceptors.NewDisabledWhiteListDataVerifier()
	if err != nil {
		return nil, err
	}

	epochStartProvider.trieContainer = state.NewDataTriesHolder()
	epochStartProvider.trieStorageManagers = make(map[string]data.StorageManager)

	if epochStartProvider.generalConfig.Hardfork.AfterHardFork {
		epochStartProvider.startEpoch = epochStartProvider.generalConfig.Hardfork.StartEpoch
		epochStartProvider.baseData.lastEpoch = epochStartProvider.startEpoch
		epochStartProvider.startSlot = tools.SafeU64ToI64(epochStartProvider.generalConfig.Hardfork.StartSlot)
		epochStartProvider.baseData.lastSlot = epochStartProvider.startSlot
		epochStartProvider.baseData.epochStartSlot = tools.SafeI64ToU64(epochStartProvider.startSlot)
	}

	return epochStartProvider, nil
}

func (e *epochStartBootstrap) isStartInEpochZero() bool {
	startTime := time.Unix(e.genesisNodesConfig.GetStartTime(), 0)
	isCurrentTimeBeforeGenesis := time.Since(startTime) < 0
	if isCurrentTimeBeforeGenesis {
		return true
	}

	slotsPerEpoch := float64(e.genesisNodesConfig.GetSlotsPerEpoch())
	currentSlot := e.slotManager.Index() - e.startSlot
	epochEndPlusGracePeriod := slotsPerEpoch * (gracePeriodInPercentage + 1.0)
	log.Debug("IsStartInEpochZero", "currentSlot", currentSlot, "epochEndSlot", epochEndPlusGracePeriod)
	return float64(currentSlot) < epochEndPlusGracePeriod
}

func (e *epochStartBootstrap) prepareEpochZero() (Parameters, error) {
	err := e.createTriesComponents()
	if err != nil {
		return Parameters{}, err
	}

	parameters := Parameters{
		Epoch: e.startEpoch,
	}
	return parameters, nil
}

func (e *epochStartBootstrap) IsNodeInGenesisNodesConfig() bool {
	ownPubKey, err := e.publicKey.ToByteArray()
	if err != nil {
		return false
	}

	electedList, eligibleList, err := e.genesisNodesConfig.InitialNodesInfo()
	if err != nil {
		return false
	}

	for _, electedNode := range electedList {
		if bytes.Equal(electedNode.PubKeyBytes(), ownPubKey) {
			return true
		}
	}

	for _, eligibleNode := range eligibleList {
		if bytes.Equal(eligibleNode.PubKeyBytes(), ownPubKey) {
			return true
		}
	}

	return false
}

// GetTriesComponents returns the created tries components according to the shardID for the current epoch
func (e *epochStartBootstrap) GetTriesComponents() (state.TriesHolder, map[string]data.StorageManager) {
	return e.trieContainer, e.trieStorageManagers
}

// Bootstrap runs the fast bootstrap method from the network or local storage
func (e *epochStartBootstrap) Bootstrap() (Parameters, error) {

	if !e.generalConfig.StartInEpoch {
		log.Warn("fast bootstrap is disabled")

		e.initializeFromLocalStorage()
		if !e.baseData.storageExists {
			err := e.createTriesComponents()
			if err != nil {
				return Parameters{}, err
			}

			return Parameters{
				Epoch: e.startEpoch,
			}, nil
		}

		err := e.createTriesComponents()
		if err != nil {
			return Parameters{}, err
		}
		epochToStart := e.baseData.lastEpoch

		return Parameters{
			Epoch:       epochToStart,
			NodesConfig: e.nodesConfig,
		}, nil
	}

	defer func() {
		log.Debug("unregistering all message processor and un-joining all topics")
		errMessenger := e.messenger.UnregisterAllMessageProcessors()
		log.LogIfError(errMessenger)

		errMessenger = e.messenger.UnjoinAllTopics()
		log.LogIfError(errMessenger)
	}()

	var err error
	e.dataPool, err = factoryDataPool.NewDataPoolFromConfig(
		factoryDataPool.ArgsDataPool{
			Config: &e.generalConfig,
		},
	)
	if err != nil {
		return Parameters{}, err
	}

	params, shouldContinue, err := e.startFromSavedEpoch()
	if !shouldContinue {
		return params, err
	}

	err = e.prepareComponentsToSyncFromNetwork()
	if err != nil {
		return Parameters{}, err
	}

	e.epochStartMeta, err = e.epochStartMetaBlockSyncer.SyncEpochStartMeta(timeToWait)
	if err != nil {
		return Parameters{}, err
	}

	e.prevEpochStartMeta, err = e.epochStartMetaBlockSyncer.SyncPrevEpochStartMeta(timeToWait, e.epochStartMeta.GetEpoch()-1)
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("start in epoch bootstrap: got epoch start meta header", "epoch", e.epochStartMeta.GetEpoch(), "nonce", e.epochStartMeta.GetNonce())
	log.Debug("start in epoch bootstrap: got prev epoch start meta header", "epoch", e.prevEpochStartMeta.GetEpoch(), "nonce", e.prevEpochStartMeta.GetNonce())
	e.setEpochStartMetrics()

	err = e.createSyncers()
	if err != nil {
		return Parameters{}, err
	}

	params, err = e.requestAndProcessing()
	if err != nil {
		return Parameters{}, err
	}

	return params, nil
}

func (e *epochStartBootstrap) computeIfCurrentEpochIsSaved() bool {
	e.initializeFromLocalStorage()
	if !e.baseData.storageExists {
		return false
	}

	computedSlot := e.slotManager.Index()
	log.Debug("computed slot", "slot", computedSlot, "lastSlot", e.baseData.lastSlot)
	if computedSlot-e.baseData.lastSlot < slotGracePeriod {
		return true
	}

	slotsSinceEpochStart := computedSlot - tools.SafeU64ToI64(e.baseData.epochStartSlot)
	log.Debug("epoch start slot", "slot", e.baseData.epochStartSlot, "slotsSinceEpochStart", slotsSinceEpochStart)
	slotsPerEpoch := float64(e.genesisNodesConfig.GetSlotsPerEpoch())
	epochEndPlusGracePeriod := slotsPerEpoch * (gracePeriodInPercentage + 1.0)
	return float64(slotsSinceEpochStart) < epochEndPlusGracePeriod
}

func (e *epochStartBootstrap) prepareComponentsToSyncFromNetwork() error {
	e.closeTrieComponents()
	e.storageService = disabled.NewChainStorer()

	err := e.createTriesComponents()
	if err != nil {
		return err
	}

	err = e.createRequestHandler()
	if err != nil {
		return err
	}

	epochStartConfig := e.generalConfig.EpochStart
	blockProcessor, err := NewEpochStartBlockProcessor(
		e.messenger,
		e.requestHandler,
		e.marshalizer,
		e.hasher,
		thresholdForConsideringMetaBlockCorrect,
		epochStartConfig.MinNumConnectedPeersToStart,
		epochStartConfig.MinNumOfPeersToConsiderBlockValid,
	)
	if err != nil {
		return err
	}

	argsEpochStartSyncer := ArgsNewEpochStartMetaSyncer{
		RequestHandler:          e.requestHandler,
		Messenger:               e.messenger,
		Marshalizer:             e.marshalizer,
		TxSignMarshalizer:       e.txSignMarshalizer,
		Hasher:                  e.hasher,
		ChainID:                 []byte(e.genesisNodesConfig.GetChainID()),
		KeyGen:                  e.keyGen,
		BlockKeyGen:             e.blockKeyGen,
		Signer:                  e.singleSigner,
		BlockSigner:             e.blockSingleSigner,
		WhitelistHandler:        e.whiteListHandler,
		AddressPubkeyConv:       e.addressPubkeyConverter,
		NonceConverter:          e.uint64Converter,
		HeaderIntegrityVerifier: e.headerIntegrityVerifier,
		BlockProcessor:          blockProcessor,
	}
	e.epochStartMetaBlockSyncer, err = NewEpochStartMetaSyncer(argsEpochStartSyncer)
	if err != nil {
		return err
	}

	return nil
}

func (e *epochStartBootstrap) createSyncers() error {

	var err error

	args := factoryInterceptors.ArgsEpochStartInterceptorContainer{
		Config:                  e.generalConfig,
		ProtoMarshalizer:        e.marshalizer,
		TxSignMarshalizer:       e.txSignMarshalizer,
		Hasher:                  e.hasher,
		Messenger:               e.messenger,
		DataPool:                e.dataPool,
		SingleSigner:            e.singleSigner,
		BlockSingleSigner:       e.blockSingleSigner,
		KeyGen:                  e.keyGen,
		BlockKeyGen:             e.blockKeyGen,
		WhiteListHandler:        e.whiteListHandler,
		WhiteListerVerifiedTxs:  e.whiteListerVerifiedTxs,
		AddressPubkeyConv:       e.addressPubkeyConverter,
		NonceConverter:          e.uint64Converter,
		ChainID:                 []byte(e.genesisNodesConfig.GetChainID()),
		MinTransactionVersion:   e.genesisNodesConfig.GetMinTransactionVersion(),
		HeaderIntegrityVerifier: e.headerIntegrityVerifier,
		TxSignHasher:            e.txSignHasher,
		EpochNotifier:           e.epochNotifier,
		ForkController:          e.forkController,
	}

	e.interceptorContainer, err = factoryInterceptors.NewEpochStartInterceptorsContainer(args)
	if err != nil {
		return err
	}

	return nil
}

// Bootstrap will handle requesting and receiving the needed information the node will bootstrap from
func (e *epochStartBootstrap) requestAndProcessing() (Parameters, error) {
	var err error
	e.baseData.lastEpoch = e.epochStartMeta.Header.Epoch

	epochStartValidators, prevEpochStartValidators, err := e.requestAndProcess()
	if err != nil {
		return Parameters{}, err
	}

	log.Debug("removing cached received trie nodes")
	e.dataPool.TrieNodes().Clear()

	parameters := Parameters{
		Epoch:                   e.baseData.lastEpoch,
		NodesConfig:             e.nodesConfig,
		CurrEpochValidatorsInfo: epochStartValidators,
		PrevEpochValidatorsInfo: prevEpochStartValidators,
	}

	return parameters, nil
}

func (e *epochStartBootstrap) processNodesConfig(pubKey []byte, epochStartValidators []*state.ValidatorInfo, prevEpochStartValidators []*state.ValidatorInfo) error {
	var err error

	argsNewValidatorStatusSyncers := ArgsNewSyncValidatorStatus{
		DataPool:           e.dataPool,
		Marshalizer:        e.marshalizer,
		RequestHandler:     e.requestHandler,
		GenesisNodesConfig: e.genesisNodesConfig,
		NodeShuffler:       e.nodeShuffler,
		Hasher:             e.hasher,
		PubKey:             pubKey,
	}
	e.nodesConfigHandler, err = NewSyncValidatorStatus(argsNewValidatorStatusSyncers)
	if err != nil {
		return err
	}

	e.nodesConfig, err = e.nodesConfigHandler.NodesConfigFromMetaBlock(e.epochStartMeta, e.prevEpochStartMeta, epochStartValidators, prevEpochStartValidators)

	return err
}

func (e *epochStartBootstrap) requestAndProcess() ([]*state.ValidatorInfo, []*state.ValidatorInfo, error) {
	var err error

	epochStartValidators, err := e.GetEpochValidatorsFromTrie(e.epochStartMeta.GetValidatorsTrieRoot())
	if err != nil {
		return nil, nil, err
	}

	prevEpochStartValidators, err := e.GetEpochValidatorsFromTrie(e.prevEpochStartMeta.GetValidatorsTrieRoot())
	if err != nil {
		return nil, nil, err
	}

	pubKeyBytes, err := e.publicKey.ToByteArray()
	if err != nil {
		return nil, nil, err
	}

	log.Debug("start in epoch bootstrap: processNodesConfig")
	err = e.processNodesConfig(pubKeyBytes, epochStartValidators, prevEpochStartValidators)
	if err != nil {
		return nil, nil, err
	}

	err = e.messenger.CreateTopic(common.ConsensusTopic, true)
	if err != nil {
		return nil, nil, err
	}

	storageHandlerComponent, err := NewMetaStorageHandler(
		e.generalConfig,
		e.pathManager,
		e.marshalizer,
		e.hasher,
		e.epochStartMeta.Header.Epoch,
		e.uint64Converter,
	)
	if err != nil {
		return nil, nil, err
	}

	defer storageHandlerComponent.CloseStorageService()
	e.closeTrieComponents()
	err = e.createTriesComponents()
	if err != nil {
		return nil, nil, err
	}

	log.Debug("start in epoch bootstrap: syncValidatorAccountsState")
	err = e.syncValidatorAccountsState(e.epochStartMeta.Header.ValidatorsTrieRoot)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("start in epoch bootstrap: syncAccountsState")
	err = e.syncUserAccountsState(e.epochStartMeta.Header.TrieRoot)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("start in epoch bootstrap: syncKappAccountsState")
	err = e.syncKappAccountsState(e.epochStartMeta.Header.KAppsTrieRoot)
	if err != nil {
		return nil, nil, err
	}

	components := &ComponentsNeededForBootstrap{
		EpochStartBlock:    e.epochStartMeta,
		PreviousEpochStart: e.prevEpochStartMeta,
		NodesConfig:        e.nodesConfig,
		Headers:            e.syncedHeaders,
		UserAccountTries:   e.userAccountTries,
		PeerAccountTries:   e.peerAccountTries,
		KAppAccountTries:   e.kappAccountTries,
	}

	errSavingToStorage := storageHandlerComponent.SaveDataToStorage(components)
	if errSavingToStorage != nil {
		return nil, nil, errSavingToStorage
	}

	return epochStartValidators, prevEpochStartValidators, nil
}

func (e *epochStartBootstrap) syncUserAccountsState(rootHash []byte) error {
	thr, err := throttler.NewNumGoRoutinesThrottler(int32(e.numConcurrentTrieSyncers)) // #nosec G115
	if err != nil {
		return err
	}

	e.mutTrieStorageManagers.RLock()
	trieStorageManager := e.trieStorageManagers[factory.UserAccountTrie]
	e.mutTrieStorageManagers.RUnlock()

	argsUserAccountsSyncer := syncer.ArgsNewUserAccountsSyncer{
		ArgsNewBaseAccountsSyncer: syncer.ArgsNewBaseAccountsSyncer{
			Hasher:                    e.hasher,
			Marshalizer:               e.marshalizer,
			TrieStorageManager:        trieStorageManager,
			RequestHandler:            e.requestHandler,
			Timeout:                   timeoutGettingTrieNode,
			Cacher:                    e.dataPool.TrieNodes(),
			MaxTrieLevelInMemory:      e.generalConfig.StateTriesConfig.MaxStateTrieLevelInMemory,
			MaxHardCapForMissingNodes: e.maxHardCapForMissingNodes,
		},
		Throttler: thr,
	}
	accountsDBSyncer, err := syncer.NewUserAccountsSyncer(argsUserAccountsSyncer)
	if err != nil {
		return err
	}

	err = accountsDBSyncer.SyncAccounts(rootHash)
	if err != nil {
		return err
	}

	e.userAccountTries = accountsDBSyncer.GetSyncedTries()
	return nil
}

func (e *epochStartBootstrap) syncValidatorAccountsState(rootHash []byte) error {
	e.mutTrieStorageManagers.RLock()
	peerTrieStorageManager := e.trieStorageManagers[factory.PeerAccountTrie]
	e.mutTrieStorageManagers.RUnlock()

	args := syncer.ArgsNewValidatorAccountsSyncer{
		ArgsNewBaseAccountsSyncer: syncer.ArgsNewBaseAccountsSyncer{
			Hasher:                    e.hasher,
			Marshalizer:               e.marshalizer,
			TrieStorageManager:        peerTrieStorageManager,
			RequestHandler:            e.requestHandler,
			Timeout:                   timeoutGettingTrieNode,
			Cacher:                    e.dataPool.TrieNodes(),
			MaxTrieLevelInMemory:      e.generalConfig.StateTriesConfig.MaxStateTrieLevelInMemory,
			MaxHardCapForMissingNodes: e.maxHardCapForMissingNodes,
		},
	}
	accountsDBSyncer, err := syncer.NewValidatorAccountsSyncer(args)
	if err != nil {
		return err
	}

	err = accountsDBSyncer.SyncAccounts(rootHash)
	if err != nil {
		return err
	}

	e.peerAccountTries = accountsDBSyncer.GetSyncedTries()
	return nil
}

func (e *epochStartBootstrap) syncKappAccountsState(rootHash []byte) error {
	thr, err := throttler.NewNumGoRoutinesThrottler(int32(e.numConcurrentTrieSyncers)) // #nosec G115
	if err != nil {
		return err
	}

	e.mutTrieStorageManagers.RLock()
	kappTrieStorageManager := e.trieStorageManagers[factory.KAppAccountTrie]
	e.mutTrieStorageManagers.RUnlock()

	args := syncer.ArgsNewKappAccountsSyncer{
		ArgsNewBaseAccountsSyncer: syncer.ArgsNewBaseAccountsSyncer{
			Hasher:                    e.hasher,
			Marshalizer:               e.marshalizer,
			TrieStorageManager:        kappTrieStorageManager,
			RequestHandler:            e.requestHandler,
			Timeout:                   timeoutGettingTrieNode,
			Cacher:                    e.dataPool.TrieNodes(),
			MaxTrieLevelInMemory:      e.generalConfig.StateTriesConfig.MaxStateTrieLevelInMemory,
			MaxHardCapForMissingNodes: e.maxHardCapForMissingNodes,
		},
		Throttler: thr,
	}
	accountsDBSyncer, err := syncer.NewKappAccountsSyncer(args)
	if err != nil {
		return err
	}

	err = accountsDBSyncer.SyncAccounts(rootHash)
	if err != nil {
		return err
	}

	e.kappAccountTries = accountsDBSyncer.GetSyncedTries()
	return nil
}

func (e *epochStartBootstrap) createTriesComponents() error {

	trieFactoryArgs := factory.TrieFactoryArgs{
		EvictionWaitingListCfg:   e.generalConfig.EvictionWaitingList,
		SnapshotDbCfg:            e.generalConfig.TrieSnapshotDB,
		Marshalizer:              e.marshalizer,
		Hasher:                   e.hasher,
		PathManager:              e.pathManager,
		TrieStorageManagerConfig: e.generalConfig.TrieStorageManagerConfig,
	}
	trieFactory, err := factory.NewTrieFactory(trieFactoryArgs)
	if err != nil {
		return err
	}

	userStorageManager, userAccountTrie, err := trieFactory.Create(
		e.generalConfig.AccountsTrieStorage,
		e.generalConfig.StateTriesConfig.AccountsStatePruningEnabled,
		e.generalConfig.StateTriesConfig.MaxStateTrieLevelInMemory,
	)
	if err != nil {
		return err
	}
	e.trieContainer.Put([]byte(factory.UserAccountTrie), userAccountTrie)
	e.trieStorageManagers[factory.UserAccountTrie] = userStorageManager

	peerStorageManager, peerAccountsTrie, err := trieFactory.Create(
		e.generalConfig.PeerAccountsTrieStorage,
		e.generalConfig.StateTriesConfig.PeerStatePruningEnabled,
		e.generalConfig.StateTriesConfig.MaxPeerTrieLevelInMemory,
	)
	if err != nil {
		return err
	}
	e.trieContainer.Put([]byte(factory.PeerAccountTrie), peerAccountsTrie)
	e.trieStorageManagers[factory.PeerAccountTrie] = peerStorageManager

	kappStorageManager, kappAccountsTrie, err := trieFactory.Create(
		e.generalConfig.KAppAccountsTrieStorage,
		e.generalConfig.StateTriesConfig.KAppStatePruningEnabled,
		e.generalConfig.StateTriesConfig.MaxKAppTrieLevelInMemory,
	)
	if err != nil {
		return err
	}
	e.trieContainer.Put([]byte(factory.KAppAccountTrie), kappAccountsTrie)
	e.trieStorageManagers[factory.KAppAccountTrie] = kappStorageManager

	return nil
}

func (e *epochStartBootstrap) createRequestHandler() error {
	dataPacker, err := partitioning.NewSimpleDataPacker(e.marshalizer)
	if err != nil {
		return err
	}

	storageService := disabled.NewChainStorer()

	resolversContainerArgs := resolverscontainer.FactoryArgs{
		Messenger:                  e.messenger,
		Store:                      storageService,
		Marshalizer:                e.marshalizer,
		DataPools:                  e.dataPool,
		Uint64ByteSliceConverter:   uint64ByteSlice.NewBigEndianConverter(),
		NumConcurrentResolvingJobs: 10,
		DataPacker:                 dataPacker,
		TriesContainer:             e.trieContainer,
		InputAntifloodHandler:      disabled.NewAntiFloodHandler(),
		OutputAntifloodHandler:     disabled.NewAntiFloodHandler(),
	}
	resolverFactory, err := resolverscontainer.NewMetaResolversContainerFactory(resolversContainerArgs)
	if err != nil {
		return err
	}

	container, err := resolverFactory.Create()
	if err != nil {
		return err
	}

	finder, err := containers.NewResolversFinder(container)
	if err != nil {
		return err
	}

	requestedItemsHandler := timecache.NewTimeCache(timeBetweenRequests)
	e.requestHandler, err = requestHandlers.NewResolverRequestHandler(
		finder,
		requestedItemsHandler,
		e.whiteListHandler,
		maxToRequest,
		timeBetweenRequests,
	)
	return err
}

func (e *epochStartBootstrap) startFromSavedEpoch() (Parameters, bool, error) {

	isStartInEpochZero := e.isStartInEpochZero()
	isCurrentEpochSaved := e.computeIfCurrentEpochIsSaved()

	if isStartInEpochZero || isCurrentEpochSaved {
		if e.baseData.lastEpoch <= e.startEpoch {
			params, err := e.prepareEpochZero()
			return params, false, err
		}

		parameters, errPrepare := e.prepareEpochFromStorage()
		if errPrepare == nil {
			return parameters, false, nil
		}

		log.Debug("could not start from storage - will try sync for start in epoch", "error", errPrepare)
	}

	return Parameters{}, true, nil
}

func (e *epochStartBootstrap) setEpochStartMetrics() {
	if e.epochStartMeta != nil {
		e.statusHandler.SetUInt64Value(core.MetricNonceAtEpochStart, e.epochStartMeta.Header.Nonce)
		e.statusHandler.SetUInt64Value(core.MetricSlotAtEpochStart, e.epochStartMeta.Header.Slot)
	}
}

func (e *epochStartBootstrap) closeTrieComponents() {
	if e.trieStorageManagers != nil {
		log.Debug("closing all trieStorageManagers", "num", len(e.trieStorageManagers))
		for _, tsm := range e.trieStorageManagers {
			err := tsm.Close()
			log.LogIfError(err)
		}
	}

	if !check.IfNil(e.trieContainer) {
		tries := e.trieContainer.GetAll()
		log.Debug("closing all tries", "num", len(tries))
		for _, trie := range tries {
			err := trie.ClosePersister()
			log.LogIfError(err)
		}
	}

	if !check.IfNil(e.storageService) {
		err := e.storageService.Destroy()
		log.LogIfError(err)
	}
}

func (e *epochStartBootstrap) GetEpochValidatorsFromTrie(validatorsTrieRoot []byte) ([]*state.ValidatorInfo, error) {

	log.Debug("start in epoch bootstrap: syncValidatorAccountsState")
	err := e.syncValidatorAccountsState(validatorsTrieRoot)
	if err != nil {
		return nil, err
	}

	peerFactory := factoryState.NewPeerAccountCreator()
	merkleTriePeer := e.trieContainer.Get([]byte(factory.PeerAccountTrie))

	peerAdapter, err := state.NewPeerAccountsDB(merkleTriePeer, e.hasher, e.marshalizer, peerFactory, core.Normal)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	leavesChannel, err := peerAdapter.GetAllLeaves(validatorsTrieRoot, ctx)
	if err != nil {
		return nil, err
	}

	validators := make([]*state.ValidatorInfo, 0)
	for pa := range leavesChannel {
		peerAccount := state.NewEmptyPeerAccount()
		err := e.marshalizer.Unmarshal(peerAccount, pa.Value())
		if err != nil {
			return nil, err
		}

		if peerAccount.GetRevoked() {
			continue
		}

		validatorInfoData := e.PeerAccountToValidatorInfo(peerAccount)
		validators = append(validators, validatorInfoData)
	}

	return validators, nil
}

func (e *epochStartBootstrap) PeerAccountToValidatorInfo(peer state.PeerAccountHandler) *state.ValidatorInfo {
	return &state.ValidatorInfo{
		OwnerAddress:                    peer.GetOwnerAddress(),
		PublicKey:                       peer.GetBLSPublicKey(),
		List:                            peer.GetListString(),
		TempRating:                      peer.GetTempRating(),
		Rating:                          peer.GetRating(),
		RatingModifier:                  0,
		LeaderSuccess:                   peer.GetLeaderSuccessRateSuccess(),
		LeaderFailure:                   peer.GetLeaderSuccessRateFailure(),
		ValidatorSuccess:                peer.GetValidatorSuccessRateSuccess(),
		ValidatorFailure:                peer.GetValidatorSuccessRateFailure(),
		ValidatorIgnoredSignatures:      peer.GetValidatorIgnoredSignaturesRate(),
		TotalLeaderSuccess:              peer.GetTotalLeaderSuccessRateSuccess(),
		TotalLeaderFailure:              peer.GetTotalLeaderSuccessRateFailure(),
		TotalValidatorSuccess:           peer.GetTotalValidatorSuccessRateSuccess(),
		TotalValidatorFailure:           peer.GetTotalValidatorSuccessRateFailure(),
		TotalValidatorIgnoredSignatures: peer.GetTotalValidatorIgnoredSignaturesRate(),
		NumSelectedInSuccessBlocks:      peer.GetNumSelectedInSuccessBlocks(),
		IsPubKeyRevoked:                 peer.GetRevoked(),
	}
}

func (e *epochStartBootstrap) createStorageService(
	pathManager storage.PathManagerHandler,
	epochStartNotifier storage.EpochStartNotifier,
	startEpoch uint32,
) (retriever.StorageService, error) {
	storageServiceCreator, err := storageFactory.NewStorageServiceFactory(
		&e.generalConfig,
		pathManager,
		epochStartNotifier,
		startEpoch,
	)
	if err != nil {
		return nil, err
	}

	return storageServiceCreator.CreateForMeta()
}

// IsInterfaceNil returns true if there is no value under the interface
func (e *epochStartBootstrap) IsInterfaceNil() bool {
	return e == nil
}
