package factory

import (
	"fmt"
	"math"
	"os"
	"path"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/genesis/process/disabled"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug/factory"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/genesis"
	"github.com/klever-io/klever-go/update/storing"
	"github.com/klever-io/klever-go/update/sync"
)

var log = logger.GetOrCreate("update/factory")

// ArgsExporter is the argument structure to create a new exporter
type ArgsExporter struct {
	TxSignMarshalizer marshal.Marshalizer
	Marshalizer       marshal.Marshalizer
	Hasher            hashing.Hasher
	//HeaderValidator          epochStart.HeaderValidator
	Uint64Converter          typeConverters.Uint64ByteSliceConverter
	DataPool                 retriever.PoolsHolder
	StorageService           retriever.StorageService
	RequestHandler           process.RequestHandler
	Messenger                p2p.Messenger
	ActiveAccountsDBs        map[state.AccountsDbIdentifier]state.AccountsAdapter
	ExistingResolvers        retriever.ResolversContainer
	ExportFolder             string
	ExportTriesStorageConfig config.StorageConfig
	ExportStateStorageConfig config.StorageConfig
	ExportStateKeysConfig    config.StorageConfig
	MaxTrieLevelInMemory     uint
	WhiteListHandler         process.WhiteListHandler
	WhiteListerVerifiedTxs   process.WhiteListHandler
	InterceptorsContainer    process.InterceptorsContainer
	MultiSigner              crypto.MultiSigner
	NodesCoordinator         sharding.NodesCoordinator
	SingleSigner             crypto.SingleSigner
	AddressPubKeyConverter   core.PubkeyConverter
	ValidatorPubKeyConverter core.PubkeyConverter
	BlockKeyGen              crypto.KeyGenerator
	KeyGen                   crypto.KeyGenerator
	BlockSigner              crypto.SingleSigner
	HeaderSigVerifier        process.InterceptedHeaderSigVerifier
	HeaderIntegrityVerifier  process.HeaderIntegrityVerifier
	//ValidityAttester          process.ValidityAttester
	InputAntifloodHandler     process.P2PAntifloodHandler
	OutputAntifloodHandler    process.P2PAntifloodHandler
	ChainID                   []byte
	SlotManager               update.SlotManager
	GenesisNodesSetupHandler  update.GenesisNodesSetupHandler
	InterceptorDebugConfig    config.InterceptorResolverDebugConfig
	MinTxVersion              uint32
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	EpochStartTrigger         process.TriggerHandler
	MaxHardCapForMissingNodes int
	NumConcurrentTrieSyncers  int
	KAppController            kapp.KAppController
}

type exportHandlerFactory struct {
	txSignMarshalizer        marshal.Marshalizer
	marshalizer              marshal.Marshalizer
	hasher                   hashing.Hasher
	uint64Converter          typeConverters.Uint64ByteSliceConverter
	dataPool                 retriever.PoolsHolder
	storageService           retriever.StorageService
	requestHandler           process.RequestHandler
	messenger                p2p.Messenger
	activeAccountsDBs        map[state.AccountsDbIdentifier]state.AccountsAdapter
	exportFolder             string
	exportTriesStorageConfig config.StorageConfig
	exportStateStorageConfig config.StorageConfig
	exportStateKeysConfig    config.StorageConfig
	maxTrieLevelInMemory     uint
	whiteListHandler         process.WhiteListHandler
	whiteListerVerifiedTxs   process.WhiteListHandler
	interceptorsContainer    process.InterceptorsContainer
	existingResolvers        retriever.ResolversContainer
	epochStartTrigger        eventNotifier.TriggerHandler
	accounts                 state.AccountsAdapter
	multiSigner              crypto.MultiSigner
	nodesCoordinator         sharding.NodesCoordinator
	singleSigner             crypto.SingleSigner
	blockKeyGen              crypto.KeyGenerator
	keyGen                   crypto.KeyGenerator
	blockSigner              crypto.SingleSigner
	addressPubKeyConverter   core.PubkeyConverter
	validatorPubKeyConverter core.PubkeyConverter
	headerSigVerifier        process.InterceptedHeaderSigVerifier
	headerIntegrityVerifier  process.HeaderIntegrityVerifier
	//validityAttester          process.ValidityAttester
	resolverContainer         retriever.ResolversContainer
	inputAntifloodHandler     process.P2PAntifloodHandler
	outputAntifloodHandler    process.P2PAntifloodHandler
	chainID                   []byte
	slotManager               process.SlotManager
	genesisNodesSetupHandler  update.GenesisNodesSetupHandler
	interceptorDebugConfig    config.InterceptorResolverDebugConfig
	minTxVersion              uint32
	enableSignTxWithHashEpoch uint32
	txSignHasher              hashing.Hasher
	epochNotifier             process.EpochNotifier
	maxHardCapForMissingNodes int
	numConcurrentTrieSyncers  int
	kAppController            kapp.KAppController
}

// NewExportHandlerFactory creates an exporter factory
func NewExportHandlerFactory(args ArgsExporter) (*exportHandlerFactory, error) {
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.Uint64Converter) {
		return nil, common.ErrNilUint64Converter
	}
	if check.IfNil(args.DataPool) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(args.StorageService) {
		return nil, common.ErrNilStorage
	}
	if check.IfNil(args.RequestHandler) {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(args.Messenger) {
		return nil, common.ErrNilMessenger
	}
	if args.ActiveAccountsDBs == nil {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(args.WhiteListHandler) {
		return nil, common.ErrNilWhiteListHandler
	}
	if check.IfNil(args.WhiteListerVerifiedTxs) {
		return nil, common.ErrNilWhiteListHandler
	}
	if check.IfNil(args.InterceptorsContainer) {
		return nil, common.ErrNilInterceptorsContainer
	}
	if check.IfNil(args.ExistingResolvers) {
		return nil, common.ErrNilResolverContainer
	}
	if check.IfNil(args.MultiSigner) {
		return nil, update.ErrNilMultiSigner
	}
	if check.IfNil(args.NodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(args.SingleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(args.AddressPubKeyConverter) {
		return nil, fmt.Errorf("%w for addresses", common.ErrNilPubKeyConverter)
	}
	if check.IfNil(args.ValidatorPubKeyConverter) {
		return nil, fmt.Errorf("%w for validators", common.ErrNilPubKeyConverter)
	}
	if check.IfNil(args.BlockKeyGen) {
		return nil, common.ErrNilBlockKeyGen
	}
	if check.IfNil(args.KeyGen) {
		return nil, crypto.ErrNilKeyGenerator
	}
	if check.IfNil(args.BlockSigner) {
		return nil, common.ErrNilBlockSigner
	}
	if check.IfNil(args.HeaderSigVerifier) {
		return nil, common.ErrNilHeaderSigVerifier
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return nil, common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(args.TxSignMarshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.InputAntifloodHandler) {
		return nil, update.ErrNilAntiFloodHandler
	}
	if check.IfNil(args.OutputAntifloodHandler) {
		return nil, update.ErrNilAntiFloodHandler
	}
	if check.IfNil(args.SlotManager) {
		return nil, common.ErrNilSlotManager
	}
	if check.IfNil(args.GenesisNodesSetupHandler) {
		return nil, update.ErrNilGenesisNodesSetupHandler
	}
	if check.IfNil(args.TxSignHasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, update.ErrNilEpochNotifier
	}
	if args.MaxHardCapForMissingNodes < 1 {
		return nil, update.ErrInvalidMaxHardCapForMissingNodes
	}
	if args.NumConcurrentTrieSyncers < 1 {
		return nil, update.ErrInvalidNumConcurrentTrieSyncers
	}
	if check.IfNil(args.KAppController) {
		return nil, common.ErrNilKAppController
	}

	e := &exportHandlerFactory{
		txSignMarshalizer:        args.TxSignMarshalizer,
		marshalizer:              args.Marshalizer,
		hasher:                   args.Hasher,
		uint64Converter:          args.Uint64Converter,
		dataPool:                 args.DataPool,
		storageService:           args.StorageService,
		requestHandler:           args.RequestHandler,
		messenger:                args.Messenger,
		activeAccountsDBs:        args.ActiveAccountsDBs,
		exportFolder:             args.ExportFolder,
		exportTriesStorageConfig: args.ExportTriesStorageConfig,
		exportStateStorageConfig: args.ExportStateStorageConfig,
		exportStateKeysConfig:    args.ExportStateKeysConfig,
		interceptorsContainer:    args.InterceptorsContainer,
		whiteListHandler:         args.WhiteListHandler,
		whiteListerVerifiedTxs:   args.WhiteListerVerifiedTxs,
		existingResolvers:        args.ExistingResolvers,
		accounts:                 args.ActiveAccountsDBs[state.UserAccountsState],
		multiSigner:              args.MultiSigner,
		nodesCoordinator:         args.NodesCoordinator,
		singleSigner:             args.SingleSigner,
		addressPubKeyConverter:   args.AddressPubKeyConverter,
		validatorPubKeyConverter: args.ValidatorPubKeyConverter,
		blockKeyGen:              args.BlockKeyGen,
		keyGen:                   args.KeyGen,
		blockSigner:              args.BlockSigner,
		headerSigVerifier:        args.HeaderSigVerifier,
		headerIntegrityVerifier:  args.HeaderIntegrityVerifier,
		//validityAttester:          args.ValidityAttester,
		inputAntifloodHandler:     args.InputAntifloodHandler,
		outputAntifloodHandler:    args.OutputAntifloodHandler,
		maxTrieLevelInMemory:      args.MaxTrieLevelInMemory,
		chainID:                   args.ChainID,
		slotManager:               args.SlotManager,
		genesisNodesSetupHandler:  args.GenesisNodesSetupHandler,
		interceptorDebugConfig:    args.InterceptorDebugConfig,
		minTxVersion:              args.MinTxVersion,
		enableSignTxWithHashEpoch: args.EnableSignTxWithHashEpoch,
		txSignHasher:              args.TxSignHasher,
		epochNotifier:             args.EpochNotifier,
		epochStartTrigger:         args.EpochStartTrigger,
		maxHardCapForMissingNodes: args.MaxHardCapForMissingNodes,
		numConcurrentTrieSyncers:  args.NumConcurrentTrieSyncers,
	}

	return e, nil
}

// Create makes a new export handler
func (e *exportHandlerFactory) Create() (update.ExportHandler, error) {
	err := e.prepareFolders(e.exportFolder)
	if err != nil {
		return nil, err
	}

	// TODO reuse the debugger when the one used for regular resolvers & interceptors will be moved inside the status components
	debugger, errNotCritical := factory.NewInterceptorResolverDebuggerFactory(e.interceptorDebugConfig)
	if errNotCritical != nil {
		log.Warn("error creating hardfork debugger", "error", errNotCritical)
	}

	argsDataTrieFactory := ArgsNewDataTrieFactory{
		StorageConfig:        e.exportTriesStorageConfig,
		SyncFolder:           e.exportFolder,
		Marshalizer:          e.marshalizer,
		Hasher:               e.hasher,
		MaxTrieLevelInMemory: e.maxTrieLevelInMemory,
	}
	dataTriesContainerFactory, err := NewDataTrieFactory(argsDataTrieFactory)
	if err != nil {
		return nil, err
	}
	dataTries, err := dataTriesContainerFactory.Create()
	if err != nil {
		return nil, err
	}

	argsResolvers := ArgsNewResolversContainerFactory{
		Messenger:                  e.messenger,
		Marshalizer:                e.marshalizer,
		DataTrieContainer:          dataTries,
		ExistingResolvers:          e.existingResolvers,
		NumConcurrentResolvingJobs: 100,
		InputAntifloodHandler:      e.inputAntifloodHandler,
		OutputAntifloodHandler:     e.outputAntifloodHandler,
	}
	resolversFactory, err := NewResolversContainerFactory(argsResolvers)
	if err != nil {
		return nil, err
	}
	e.resolverContainer, err = resolversFactory.Create()
	if err != nil {
		return nil, err
	}

	e.resolverContainer.Iterate(func(key string, resolver retriever.Resolver) bool {
		errNotCritical = resolver.SetResolverDebugHandler(debugger)
		if errNotCritical != nil {
			log.Warn("error setting debugger", "resolver", key, "error", errNotCritical)
		}

		return true
	})

	argsAccountsSyncers := ArgsNewAccountsDBSyncersContainerFactory{
		TrieCacher:                e.dataPool.TrieNodes(),
		RequestHandler:            e.requestHandler,
		Hasher:                    e.hasher,
		Marshalizer:               e.marshalizer,
		TrieStorageManager:        dataTriesContainerFactory.TrieStorageManager(),
		TimoutGettingTrieNode:     update.TimeoutGettingTrieNodes,
		MaxTrieLevelInMemory:      e.maxTrieLevelInMemory,
		MaxHardCapForMissingNodes: e.maxHardCapForMissingNodes,
		NumConcurrentTrieSyncers:  e.numConcurrentTrieSyncers,
	}
	accountsDBSyncerFactory, err := NewAccountsDBSContainerFactory(argsAccountsSyncers)
	if err != nil {
		return nil, err
	}
	accountsDBSyncerContainer, err := accountsDBSyncerFactory.Create()
	if err != nil {
		return nil, err
	}

	argsNewHeadersSync := sync.ArgsNewHeadersSyncHandler{
		StorageService:  e.storageService,
		Cache:           e.dataPool.Headers(),
		Marshalizer:     e.marshalizer,
		Hasher:          e.hasher,
		EpochHandler:    e.epochStartTrigger, // TODO: EpochHandler:    epochHandler,
		RequestHandler:  e.requestHandler,
		Uint64Converter: e.uint64Converter,
	}
	epochStartHeadersSyncer, err := sync.NewHeadersSyncHandler(argsNewHeadersSync)
	if err != nil {
		return nil, err
	}

	argsNewSyncAccountsDBsHandler := sync.ArgsNewSyncAccountsDBsHandler{
		AccountsDBsSyncers: accountsDBSyncerContainer,
		ActiveAccountsDBs:  e.activeAccountsDBs,
	}
	epochStartTrieSyncer, err := sync.NewSyncAccountsDBsHandler(argsNewSyncAccountsDBsHandler)
	if err != nil {
		return nil, err
	}

	argsPendingTransactions := sync.ArgsNewPendingTransactionsSyncer{
		DataPools:      e.dataPool,
		Storages:       e.storageService,
		Marshalizer:    e.marshalizer,
		RequestHandler: e.requestHandler,
	}
	epochStartTransactionsSyncer, err := sync.NewPendingTransactionsSyncer(argsPendingTransactions)
	if err != nil {
		return nil, err
	}

	argsSyncState := sync.ArgsNewSyncState{
		Headers:      epochStartHeadersSyncer,
		Tries:        epochStartTrieSyncer,
		Transactions: epochStartTransactionsSyncer,
	}
	stateSyncer, err := sync.NewSyncState(argsSyncState)
	if err != nil {
		return nil, err
	}

	keysStorer, err := createStorer(e.exportStateKeysConfig, e.exportFolder)
	if err != nil {
		return nil, fmt.Errorf("%w while creating keys storer", err)
	}
	keysVals, err := createStorer(e.exportStateStorageConfig, e.exportFolder)
	if err != nil {
		return nil, fmt.Errorf("%w while creating keys-values storer", err)
	}

	arg := storing.ArgHardforkStorer{
		KeysStore:   keysStorer,
		KeyValue:    keysVals,
		Marshalizer: e.marshalizer,
	}
	hs, err := storing.NewHardforkStorer(arg)
	if err != nil {
		return nil, err
	}

	argsExporter := genesis.ArgsNewStateExporter{
		StateSyncer:              stateSyncer,
		Marshalizer:              e.marshalizer,
		HardforkStorer:           hs,
		Hasher:                   e.hasher,
		ExportFolder:             e.exportFolder,
		ValidatorPubKeyConverter: e.validatorPubKeyConverter,
		AddressPubKeyConverter:   e.addressPubKeyConverter,
		GenesisNodesSetupHandler: e.genesisNodesSetupHandler,
	}
	exportHandler, err := genesis.NewStateExporter(argsExporter)
	if err != nil {
		return nil, err
	}

	// TODO: e.epochStartTrigger = epochHandler
	err = e.createInterceptors()
	if err != nil {
		return nil, err
	}

	e.interceptorsContainer.Iterate(func(key string, interceptor process.Interceptor) bool {
		errNotCritical = interceptor.SetInterceptedDebugHandler(debugger)
		if errNotCritical != nil {
			log.Warn("error setting debugger", "interceptor", key, "error", errNotCritical)
		}

		return true
	})

	return exportHandler, nil
}

func (e *exportHandlerFactory) prepareFolders(folder string) error {
	err := os.RemoveAll(folder)
	if err != nil {
		return err
	}

	return os.MkdirAll(folder, os.ModePerm)
}

func (e *exportHandlerFactory) createInterceptors() error {
	argsInterceptors := ArgsNewFullSyncInterceptorsContainerFactory{
		Accounts:                e.accounts,
		NodesCoordinator:        e.nodesCoordinator,
		Messenger:               e.messenger,
		Store:                   e.storageService,
		Marshalizer:             e.marshalizer,
		TxSignMarshalizer:       e.txSignMarshalizer,
		Hasher:                  e.hasher,
		KeyGen:                  e.keyGen,
		BlockSignKeyGen:         e.blockKeyGen,
		SingleSigner:            e.singleSigner,
		BlockSingleSigner:       e.blockSigner,
		MultiSigner:             e.multiSigner,
		DataPool:                e.dataPool,
		AddressPubkeyConverter:  e.addressPubKeyConverter,
		MaxTxNonceDeltaAllowed:  math.MaxInt32,
		TxFeeHandler:            &disabled.FeeHandler{},
		BlockBlackList:          timecache.NewTimeCache(time.Second),
		HeaderSigVerifier:       e.headerSigVerifier,
		HeaderIntegrityVerifier: e.headerIntegrityVerifier,
		//ValidityAttester:          e.validityAttester,
		EpochStartTrigger:         e.epochStartTrigger,
		WhiteListHandler:          e.whiteListHandler,
		WhiteListerVerifiedTxs:    e.whiteListerVerifiedTxs,
		InterceptorsContainer:     e.interceptorsContainer,
		AntifloodHandler:          e.inputAntifloodHandler,
		NonceConverter:            e.uint64Converter,
		ChainID:                   e.chainID,
		MinTxVersion:              e.minTxVersion,
		EnableSignTxWithHashEpoch: e.enableSignTxWithHashEpoch,
		TxSignHasher:              e.txSignHasher,
		EpochNotifier:             e.epochNotifier,
		KAppController:            e.kAppController,
	}
	fullSyncInterceptors, err := NewFullSyncInterceptorsContainerFactory(argsInterceptors)
	if err != nil {
		return err
	}

	interceptorsContainer, err := fullSyncInterceptors.Create()
	if err != nil {
		return err
	}

	e.interceptorsContainer = interceptorsContainer
	return nil
}

func createStorer(storageConfig config.StorageConfig, folder string) (storage.Storer, error) {
	dbConfig := storageFactory.GetDBFromConfig(storageConfig.DB)
	dbConfig.FilePath = path.Join(folder, storageConfig.DB.FilePath)
	accountsTrieStorage, err := storageUnit.NewStorageUnitFromConf(
		storageFactory.GetCacherFromConfig(storageConfig.Cache),
		dbConfig,
		storageFactory.GetBloomFromConfig(storageConfig.Bloom),
	)
	if err != nil {
		return nil, err
	}

	return accountsTrieStorage, nil
}

// IsInterfaceNil returns true if underlying object is nil
func (e *exportHandlerFactory) IsInterfaceNil() bool {
	return e == nil
}
