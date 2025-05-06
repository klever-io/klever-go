package processorNode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	syncGo "sync"
	"sync/atomic"
	"time"

	indexerFactory "github.com/klever-io/klever-go/indexer/factory"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/alarm"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/consensus"
	broadcastFactory "github.com/klever-io/klever-go/core/consensus/broadcast"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/kapp"
	kappDisabled "github.com/klever-io/klever-go/core/kapp/disabled"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/core/process"
	processBlock "github.com/klever-io/klever-go/core/process/block"
	"github.com/klever-io/klever-go/core/process/block/coordinator"
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/block/preprocess"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/factory/interceptorscontainer"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/core/process/interceptors"
	"github.com/klever-io/klever-go/core/process/peer"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/klever-io/klever-go/core/process/smartContract/builtInFunctions"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/core/process/throttle"
	procTx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/crypto/peerSignatureHandler"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	mclsinglesig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/factory/resolverscontainer"
	"github.com/klever-io/klever-go/data/retriever/requestHandlers"
	"github.com/klever-io/klever-go/data/state"
	stateFactory "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	trieFactory "github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/eventNotifier/epochStart"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/genesis"
	genesisData "github.com/klever-io/klever-go/genesis/data"
	genesisMock "github.com/klever-io/klever-go/genesis/mock"
	genesisProcess "github.com/klever-io/klever-go/genesis/process"
	integrationTestsMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/node/nodeDebugFactory"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// SendTransactionsPipe is the pipe used for sending new transactions
const SendTransactionsPipe = "send transactions pipe"

// SoftRestartMessage is the custom message used when the node does a soft restart operation
var SoftRestartMessage = "Shuffled out - soft restart"

// Option represents a functional configuration parameter that can operate
// over the None struct.
type Option func(*ProcessorNode) error

// TestValidatorPubkeyConverter represents an address public key converter
var TestValidatorPubkeyConverter, _ = pubkeyConverter.NewHexPubkeyConverter(96)

// TestAddressPubkeyConverter represents an address public key converter
var TestAddressPubkeyConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)

// TestHasher represents a sha256 hasher
var TestHasher = sha256.Sha256{}

var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")

var log = logger.GetOrCreate("integrationtests/processorNode")

var InitialRating = uint32(50)
var TestMarshalizer = &mock.ProtoMarshalizerMock{}
var WaitTime = time.Second

// ChainID is the chain ID identifier used in integration tests, processing nodes
var ChainID = []byte("integration tests chain ID")

// MinTransactionVersion is the minimum transaction version used in integration tests, processing nodes
var MinTransactionVersion = uint32(1)

const defaultChancesSelection = 1

// TestWalletAccount creates and account with balance and crypto necessary to sign transactions
type NodeAccount struct {
	SingleSigner      crypto.SingleSigner
	BlockSingleSigner crypto.SingleSigner
	TxSingleSigner    crypto.SingleSigner
	SkTxSign          crypto.PrivateKey
	PkTxSign          crypto.PublicKey
	PkTxSignBytes     []byte
	KeygenTxSign      crypto.KeyGenerator
	KeygenBlockSign   crypto.KeyGenerator
	PeerSigHandler    crypto.PeerSignatureHandler

	Address []byte
	Nonce   uint64
	Balance *big.Int
}

type NodeKeyPair struct {
	Sk crypto.PrivateKey
	Pk crypto.PublicKey
}

// Connectable defines the operations for a struct to become connectable by other struct
// In other words, all instances that implement this interface are able to connect with each other
type Connectable interface {
	ConnectTo(connectable Connectable) error
	GetConnectableAddress() string
	IsInterfaceNil() bool
}

type ProcessorNode struct {
	// Validators Keys
	PubkeyTxSignList     []string
	PubkeyBlockSignList  []string
	NodeTxSignKeyPair    *NodeKeyPair
	NodeBlockSignKeyPair *NodeKeyPair
	InitialNodesPubkeys  []string
	PubKey               crypto.PublicKey
	PrivKey              crypto.PrivateKey

	// Test components
	TxSentCounter           uint32
	CounterHdrRecv          int32
	NodeAccount             *NodeAccount
	Node                    *node.Node
	Ctx                     context.Context
	AddressSignatureSize    int
	AddressSignatureHexSize int
	EncodedAddressLength    int
	ValidatorSignatureSize  int
	PublicKeySize           int

	// ----------------------

	// Indexer
	Indexer process.Indexer

	// API
	QueryHandlers    map[string]debug.QueryHandler
	MutQueryHandlers syncGo.RWMutex

	// Config
	MainConfig         config.Config
	NodesSetup         sharding.GenesisNodesSetupHandler
	GenesisTime        time.Time
	EnableEpochsConfig config.EnableEpochsConfig
	IsInImportMode     bool

	// State Components
	AccountsDB          map[state.AccountsDbIdentifier]state.AccountsAdapter
	PeersAdapter        state.AccountsAdapter
	AccountsAdapter     state.AccountsAdapter
	KappsAdapter        state.AccountsAdapter
	AccountsCacher      state.AccountsCacher
	AccountsCacherRO    state.AccountsCacher
	TrieContainer       state.TriesHolder
	TrieStorageManagers map[string]data.StorageManager

	// Data Components
	GenesisBlock data.HeaderHandler
	Blkc         data.ChainHandler

	// Consensus Components + Bootstrap
	ConsensusGroupSize      int
	ConsensusTopic          string
	ConsensusType           string
	BroadcastMessenger      consensus.BroadcastMessenger
	PeerHonestyHandler      consensus.PeerHonestyHandler
	FallbackHeaderValidator consensus.FallbackHeaderValidator
	NodeRedundancyHandler   consensus.NodeRedundancyHandler
	ChanStopNodeProcess     chan endProcess.ArgEndProcess
	BootstrapSlotIndex      uint64

	// Network Components
	PeerDenialEvaluator      p2p.PeerDenialEvaluator
	Messenger                node.P2PMessenger
	InputAntifloodHandler    node.P2PAntifloodHandler
	TxAccumulator            node.Accumulator
	NetworkShardingCollector node.NetworkShardingCollector

	// Crypto Components
	KeyGenForBlock    crypto.KeyGenerator
	KeyGenForAccounts crypto.KeyGenerator
	SingleSigner      crypto.SingleSigner
	TxSingleSigner    crypto.SingleSigner
	MultiSigner       crypto.MultiSigner
	PeerSigHandler    crypto.PeerSignatureHandler

	// Core Components
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	AppStatusHandler         core.AppStatusHandler
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	Watchdog                 core.WatchdogTimer
	HeartbeatHandler         node.HeartbeatHandler
	MinTransactionVersion    uint32
	ChainID                  []byte
	CurrentSendingGoRoutines int32
	Hasher                   hashing.Hasher
	TxSignHasher             hashing.Hasher

	// Process Components
	InternalMarshalizer           marshal.Marshalizer
	TxSignMarshalizer             marshal.Marshalizer
	BlockProcessor                process.BlockProcessor
	TxProcessor                   process.TransactionProcessor
	KappController                kapp.KAppController
	KappControllerSimulator       kapp.KAppController
	ProposalController            kapps.ActiveProposalController
	FeeHandler                    process.EconomicsDataHandler
	ForkDetector                  process.ForkDetector
	ForkController                core.ForkController
	HeaderIntegrityVerifier       process.HeaderIntegrityVerifier
	HeaderSigVerifier             consensus.HeaderSigVerifier
	EpochStartRegistrationHandler eventNotifier.RegistrationHandler
	InterceptorsContainer         process.InterceptorsContainer
	ValidatorsProvider            process.ValidatorsProvider
	WhiteListHandler              process.WhiteListHandler
	WhiteListerVerifiedTxs        process.WhiteListHandler
	SyncTimer                     ntp.SyncTimer
	RequestHandler                process.RequestHandler
	BootStorer                    process.BootStorer
	ResolversContainer            retriever.ResolversContainer
	ResolversFinder               retriever.ResolversFinder
	RequestedItemsHandler         retriever.RequestedItemsHandler
	DataPool                      retriever.PoolsHolder
	Store                         retriever.StorageService
	ValidatorStatistics           process.ValidatorStatisticsProcessor
	EconomicsData                 *economics.EconomicsData
	RatingsData                   *rating.RatingsData
	NodesCoordinator              sharding.NodesCoordinator
	SlotManager                   *integrationTestsMock.SlotManagerMock
	EpochNotifier                 process.EpochNotifier
	EpochStartNotifier            notifier.EpochStartNotifier
	EpochStartTrigger             process.EpochStartTriggerHandler
	BlocksBlackListHandler        process.TimeCacher
	BlockChainHook                process.BlockChainHookHandler
	WorkingDir                    string
	LogProcessor                  process.TransactionLogProcessor
	OnRequestTransactionsHandler  func(hashes [][]byte)
	Bootstrapper                  process.Bootstrapper
}

// ApplyOptions can set up different configurable options of a Node instance
func (n *ProcessorNode) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		err := opt(n)
		if err != nil {
			return errors.New("error applying option: " + err.Error())
		}
	}
	return nil
}

// GetAppStatusHandler will return the current status handler
func (n *ProcessorNode) GetAppStatusHandler() core.AppStatusHandler {
	return n.AppStatusHandler
}

// CreateMemUnit returns an in-memory storer implementation (the vast majority of tests do not require effective
// disk I/O)
func CreateMemUnit() storage.Storer {
	capacity := uint32(10)
	shards := uint32(1)
	sizeInBytes := uint64(0)
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: capacity, Shards: shards, SizeInBytes: sizeInBytes})
	persist, _ := memorydb.NewlruDB(10000000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)

	return unit
}

// CreateStore creates a storage service for shard nodes
func CreateStore() retriever.StorageService {
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.BlockUnit, CreateMemUnit())
	store.AddStorer(retriever.HdrNonceHashDataUnit, CreateMemUnit())
	store.AddStorer(retriever.TransactionUnit, CreateMemUnit())
	store.AddStorer(retriever.HeartbeatUnit, CreateMemUnit())
	store.AddStorer(retriever.BootstrapUnit, CreateMemUnit())
	store.AddStorer(retriever.StatusMetricsUnit, CreateMemUnit())
	store.AddStorer(retriever.TxLogsUnit, CreateMemUnit())

	return store
}

func (n *ProcessorNode) addTransactionsToSendPipe(txs []*transaction.Transaction) {
	if check.IfNil(n.TxAccumulator) {
		return
	}

	for _, tx := range txs {
		n.TxAccumulator.AddData(tx)
	}
}

// GetPeerInfo returns information about a peer id
func (n *ProcessorNode) GetPeerInfo(pid string) ([]core.QueryP2PPeerInfo, error) {
	peers := n.Messenger.Peers()
	pidsFound := make([]core.PeerID, 0)
	for _, p := range peers {
		if strings.Contains(p.Pretty(), pid) {
			pidsFound = append(pidsFound, p)
		}
	}

	if len(pidsFound) == 0 {
		return nil, fmt.Errorf("%w for provided peer %s", common.ErrUnknownPeerID, pid)
	}

	sort.Slice(pidsFound, func(i, j int) bool {
		return pidsFound[i].Pretty() < pidsFound[j].Pretty()
	})

	peerInfoSlice := make([]core.QueryP2PPeerInfo, 0, len(pidsFound))
	for _, p := range pidsFound {
		pidInfo := n.createPidInfo(p)
		peerInfoSlice = append(peerInfoSlice, pidInfo)
	}

	return peerInfoSlice, nil
}

func (n *ProcessorNode) GetProposalParameters() (map[int32]*kapps.Parameter, error) {
	return n.ProposalController.GetActiveParameters(), nil
}

func (n *ProcessorNode) createPidInfo(p core.PeerID) core.QueryP2PPeerInfo {
	result := core.QueryP2PPeerInfo{
		Pid:           p.Pretty(),
		Addresses:     n.Messenger.PeerAddresses(p),
		IsBlacklisted: n.PeerDenialEvaluator.IsDenied(p),
	}

	peerInfo := n.NetworkShardingCollector.GetPeerInfo(p)
	result.PeerType = peerInfo.PeerType.String()
	if len(peerInfo.PkBytes) == 0 {
		result.Pk = ""
	} else {
		result.Pk = n.ValidatorPubkeyConverter.Encode(peerInfo.PkBytes)
	}

	return result
}

// // createChronologyHandler method creates a chronology object
// func (n *ProcessorNode) createChronologyHandler(
// 	appStatusHandler core.AppStatusHandler,
// 	broadcastMessenger consensus.BroadcastMessenger,
// 	watchdog core.WatchdogTimer,
// ) (consensus.ChronologyHandler, error) {
// 	chr, err := chronology.NewChronology(
// 		n.SlotManager,
// 		n.Blkc,
// 		n.NodesCoordinator,
// 		n.BlockProcessor,
// 		broadcastMessenger,
// 		n.SyncTimer,
// 		watchdog,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	err = chr.SetAppStatusHandler(appStatusHandler)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return chr, nil
// }

// IsInterfaceNil returns true if there is no value under the interface
func (n *ProcessorNode) IsInterfaceNil() bool {
	return n == nil
}

//---------------------- INIT TEST NODE

func (n *ProcessorNode) initDataPools() error {
	n.DataPool = mock.NewPoolsHolderMock()
	n.addHandlersForCounters()

	cacherCfg := storageUnit.CacheConfig{Capacity: 10000, Type: storageUnit.LRUCache, Shards: 1}
	whiteListCache, err := storageUnit.NewCache(cacherCfg)
	if err != nil {
		return err
	}

	cacherVerifiedCfg := storageUnit.CacheConfig{Capacity: 5000, Type: storageUnit.LRUCache, Shards: 1}
	whiteListVerifiedCache, err := storageUnit.NewCache(cacherVerifiedCfg)
	if err != nil {
		return err
	}

	n.WhiteListHandler, err = interceptors.NewWhiteListDataVerifier(whiteListCache)
	if err != nil {
		return err
	}
	n.WhiteListerVerifiedTxs, err = interceptors.NewWhiteListDataVerifier(whiteListVerifiedCache)
	if err != nil {
		return err
	}
	return nil
}

func (n *ProcessorNode) InitTestNode() error {
	err := n.initChainHandler()
	if err != nil {
		return err
	}
	err = n.initSlotHandler()
	if err != nil {
		return err
	}
	n.NetworkShardingCollector = &consensusMock.NetworkShardingCollectorStub{
		UpdatePeerIDPublicKeyCalled: func(peerID core.PeerID, pkBytes []byte) {
			log.Debug("UpdatePeerIDPublicKey", "peerID", peerID.Pretty(), "pkBytes", pkBytes)
		},
	}
	n.initStorage()
	n.initRequestedItemsHandler()
	n.initRatingsData()
	err = n.initEconomicsData()
	if err != nil {
		return err
	}
	err = n.initAccountDBsWithoutPruningStorer()
	if err != nil {
		return err
	}
	err = n.initResolvers()
	if err != nil {
		return err
	}
	err = n.initValidatorStatistics()
	if err != nil {
		return err
	}

	if check.IfNil(n.InterceptorsContainer) {
		n.InterceptorsContainer = &mock.InterceptorsContainerStub{}
	}

	alarmScheduler := alarm.NewAlarmScheduler()

	n.BroadcastMessenger, err = broadcastFactory.GetBroadcastMessenger(
		&broadcastFactory.GetBroadcastMessengerArgs{
			Marshalizer:           getMarshalizer(),
			Hasher:                getHasher(),
			Messenger:             n.Messenger,
			PrivateKey:            n.NodeBlockSignKeyPair.Sk,
			PeerSignatureHandler:  n.PeerSigHandler,
			HeadersSubscriber:     n.DataPool.Headers(),
			InterceptorsContainer: n.InterceptorsContainer,
			AlarmScheduler:        alarmScheduler,
		},
	)
	if err != nil {
		return err
	}

	if elastcEnabled {
		idx, err := n.createIndexer()
		if err != nil {
			return err
		}

		n.Indexer = idx
	}

	err = n.CreateGenesisBlock()
	if err != nil {
		return err
	}
	err = n.initInterceptors("")
	if err != nil {
		return err
	}
	err = n.initBlockProcessor()
	if err != nil {
		return err
	}
	err = n.initNode()
	if err != nil {
		return err
	}
	return nil
}

func (n *ProcessorNode) initChainHandler() error {
	blockChain := blockchain.NewBlockChain()

	_ = blockChain.SetAppStatusHandler(n.AppStatusHandler)
	_ = blockChain.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{Nonce: 0},
	})

	genesisHeaderHash, err := tools.CalculateHash(getMarshalizer(), getHasher(), blockChain.GetGenesisHeader())
	if err != nil {
		return err
	}

	blockChain.SetGenesisHeaderHash(genesisHeaderHash)
	n.Blkc = blockChain

	return nil
}

func (n *ProcessorNode) initSlotHandler() error {

	n.SlotManager = &integrationTestsMock.SlotManagerMock{
		GenesisTimeField: n.GenesisTime,
		TimestampField:   time.Now(),
		SyncTimer:        n.SyncTimer,
	}
	return nil
}

func (n *ProcessorNode) initStorage() {
	n.Store = CreateStore()
}

func (n *ProcessorNode) initEconomicsData() error {
	argsNewEconomicsData := economics.ArgsNewEconomicsData{
		EpochNotifier: n.EpochNotifier,
	}
	economicsData, err := economics.NewEconomicsData(argsNewEconomicsData)
	if err != nil {
		return err
	}

	_ = economicsData.SetProposalController(n.ProposalController)

	n.EconomicsData = economicsData
	return nil
}

func (n *ProcessorNode) initRatingsData() {
	n.RatingsData = CreateRatingsData()
}

func (n *ProcessorNode) initRequestedItemsHandler() {
	n.RequestedItemsHandler = timecache.NewTimeCache(time.Second)
}

func (n *ProcessorNode) initResolvers() error {
	var err error

	dataPacker, err := partitioning.NewSimpleDataPacker(getMarshalizer())
	if err != nil {
		return err
	}

	err = n.Messenger.CreateTopic(common.ConsensusTopic, true)
	if err != nil {
		return err
	}

	resolverContainerFactory := resolverscontainer.FactoryArgs{
		Messenger:                  n.Messenger,
		Store:                      n.Store,
		Marshalizer:                n.InternalMarshalizer,
		DataPools:                  n.DataPool,
		Uint64ByteSliceConverter:   n.Uint64ByteSliceConverter,
		DataPacker:                 dataPacker,
		TriesContainer:             n.TrieContainer,
		InputAntifloodHandler:      disabled.NewAntiFloodHandler(),
		OutputAntifloodHandler:     disabled.NewAntiFloodHandler(),
		NumConcurrentResolvingJobs: 10,
	}

	resolversContainerFactory, err := resolverscontainer.NewMetaResolversContainerFactory(resolverContainerFactory)
	if err != nil {
		return err
	}

	n.ResolversContainer, err = resolversContainerFactory.Create()
	if err != nil {
		return err
	}

	n.ResolversFinder, err = containers.NewResolversFinder(n.ResolversContainer)
	if err != nil {
		return err
	}
	n.RequestHandler, err = requestHandlers.NewResolverRequestHandler(
		n.ResolversFinder,
		n.RequestedItemsHandler,
		n.WhiteListHandler,
		100,
		time.Second,
	)
	if err != nil {
		return err
	}
	return nil

}

func (n *ProcessorNode) CreateGenesisBlock() error {
	var initialAccounts []genesis.InitialAccountHandler
	for _, pubkey := range n.PubkeyTxSignList {
		address := TestAddressPubkeyConverter.Encode([]byte(pubkey))
		initAcc := &genesisData.InitialAccount{
			Address:    address,
			Balance:    190000000000000,
			KFIBalance: 100000000000,
			Delegation: &genesisData.DelegationData{
				Address: address,
				Value:   10000000000000,
			},
		}

		initAcc.SetAddressBytes([]byte(pubkey))
		initAcc.Delegation.SetAddressBytes([]byte(pubkey))
		initialAccounts = append(initialAccounts, initAcc)
	}

	accountsParser := &genesisMock.AccountsParserStub{
		InitialAccountsCalled: func() []genesis.InitialAccountHandler {
			return initialAccounts
		},
		GetTotalStakedForDelegationAddressCalled: func(delegationAddress string) int64 {
			sum := int64(0)
			for _, in := range initialAccounts {
				if in.GetDelegationHandler().GetAddress() == delegationAddress {
					sum += in.GetDelegationHandler().GetValue()
				}
			}
			return sum
		},
		GetInitialAccountsForDelegatedCalled: func(addressBytes []byte) []genesis.InitialAccountHandler {
			list := make([]genesis.InitialAccountHandler, 0)
			for _, ia := range initialAccounts {
				if bytes.Equal(ia.GetDelegationHandler().AddressBytes(), addressBytes) {
					list = append(list, ia)
				}
			}

			return list
		},
	}

	// INIT KLV AND KFI AND DELEGATE VALIDATORS ACCOUNTS
	arg := genesisProcess.ArgsGenesisBlockCreator{
		GenesisTime:              0,
		StartEpochNum:            0,
		PubkeyConv:               n.AddressPubkeyConverter,
		InitialNodesSetup:        n.NodesSetup,
		Economics:                n.EconomicsData,
		Store:                    n.Store,
		Blkc:                     n.Blkc,
		Marshalizer:              n.InternalMarshalizer,
		SignMarshalizer:          n.TxSignMarshalizer,
		Hasher:                   n.Hasher,
		Uint64ByteSliceConverter: n.Uint64ByteSliceConverter,
		DataPool:                 n.DataPool,
		AccountsParser:           accountsParser,
		Accounts:                 n.AccountsAdapter,
		PeerAccounts:             n.PeersAdapter,
		KAppAccounts:             n.KappsAdapter,
		KAppController:           n.KappController,
		Indexer:                  n.Indexer,
		TrieStorageManagers:      n.TrieStorageManagers,
		ChainID:                  string(ChainID),
		BlockSignKeyGen:          n.NodeAccount.KeygenBlockSign,
		TxLogsProcessor:          n.LogProcessor,
		WorkingDir:               n.WorkingDir,
	}

	gbc, err := genesisProcess.NewGenesisBlockCreator(arg)
	if err != nil {
		return err
	}

	genesisBlock, err := gbc.CreateGenesisBlock()
	if err != nil {
		return err
	}
	n.GenesisBlock = genesisBlock
	err = n.Blkc.SetGenesisHeader(genesisBlock)
	if err != nil {
		return err
	}
	hash, err := tools.CalculateHash(TestMarshalizer, TestHasher, genesisBlock.GetBlockHeader())
	if err != nil {
		return err
	}
	n.Blkc.SetGenesisHeaderHash(hash)

	return nil
}

var TimeSpanForBadHeaders = time.Second * 30

func (n *ProcessorNode) initInterceptors(heartbeatPk string) error {
	n.BlocksBlackListHandler = timecache.NewTimeCache(TimeSpanForBadHeaders)

	if check.IfNil(n.EpochStartNotifier) {
		n.EpochStartNotifier = notifier.NewEpochStartSubscriptionHandler()
	}

	epochStartTriggerArgs := &epochStart.ArgsNewEpochStartTrigger{
		GenesisTime:        n.SlotManager.GenesisTimestamp(),
		Epoch:              0,
		EpochStartSlot:     0,
		EpochStartNotifier: n.EpochStartNotifier,
		SlotsPerEpoch:      6,
		Marshalizer:        getMarshalizer(),
		Hasher:             getHasher(),
		Storage:            n.Store,
		ForkController:     n.ForkController,
	}

	epochStartTrigger, err := epochStart.NewEpochStartTrigger(epochStartTriggerArgs)
	if err != nil {
		return err
	}
	n.EpochStartTrigger = epochStartTrigger

	headerIntegrity, err := CreateHeaderIntegrityVerifier(n.MainConfig.Versions)
	if err != nil {
		return err
	}
	n.HeaderIntegrityVerifier = headerIntegrity

	interceptorContainerArgs := interceptorscontainer.MetaInterceptorsContainerFactoryArgs{
		AddressPubkeyConverter:    n.AddressPubkeyConverter,
		NodesCoordinator:          n.NodesCoordinator,
		Messenger:                 n.Messenger,
		Store:                     n.Store,
		ProtoMarshalizer:          getMarshalizer(),
		TxSignMarshalizer:         getMarshalizer(),
		Hasher:                    getHasher(),
		MultiSigner:               n.MultiSigner,
		DataPool:                  n.DataPool,
		Accounts:                  n.AccountsAdapter,
		MaxTxNonceDeltaAllowed:    15000,
		SingleSigner:              n.TxSingleSigner,
		BlockSingleSigner:         n.NodeAccount.BlockSingleSigner,
		KeyGen:                    n.NodeAccount.KeygenTxSign,
		BlockKeyGen:               n.NodeAccount.KeygenBlockSign,
		BlackList:                 &mock.BlackListHandlerStub{},
		HeaderSigVerifier:         n.HeaderSigVerifier,
		HeaderIntegrityVerifier:   headerIntegrity,
		WhiteListHandler:          n.WhiteListHandler,
		WhiteListerVerifiedTxs:    n.WhiteListerVerifiedTxs,
		AntifloodHandler:          disabled.NewAntiFloodHandler(),
		EpochStartTrigger:         epochStartTrigger,
		ChainID:                   n.ChainID,
		MinTransactionVersion:     n.MinTransactionVersion,
		EnableSignTxWithHashEpoch: 1,
		TxSignHasher:              getHasher(),
		EpochNotifier:             n.EpochNotifier,
		KAppController:            n.KappController,
		TxFeeHandler: &mock.FeeHandlerStub{
			CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return nil, nil
			},
		},
		ForkController:        n.ForkController,
		RequestedItemsHandler: n.RequestedItemsHandler,
	}

	interceptorContainer, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(interceptorContainerArgs)
	if err != nil {
		return err
	}
	n.InterceptorsContainer, err = interceptorContainer.Create()

	return err
}

func (n *ProcessorNode) initValidatorStatistics() error {
	rater, err := rating.NewBlockSigningRater(n.RatingsData)
	if err != nil {
		return err
	}

	if check.IfNil(n.NodesSetup) {
		n.NodesSetup = &mock.NodesSetupStub{
			MinNumberOfNodesCalled: func() uint32 {
				return 2
			},
		}
	}

	kappArgs := kappcontroller.ArgsNewKApp{
		Hasher:         getHasher(),
		Marshalizer:    getMarshalizer(),
		PubkeyConv:     TestAddressPubkeyConverter,
		ForkController: n.ForkController,
		AccountsCacher: n.AccountsCacher,
		RatingsData:    n.RatingsData,
	}

	n.KappController, err = kappcontroller.NewKappController(kappArgs)
	if err != nil {
		return err
	}

	kappArgsSimulator := kappcontroller.ArgsNewKApp{
		Hasher:         getHasher(),
		Marshalizer:    getMarshalizer(),
		PubkeyConv:     TestAddressPubkeyConverter,
		ForkController: n.ForkController,
		AccountsCacher: n.AccountsCacherRO,
		RatingsData:    n.RatingsData,
	}

	n.KappControllerSimulator, err = kappcontroller.NewKappController(kappArgsSimulator)
	if err != nil {
		return err
	}

	//Set Accounts Cacher in validator Kapp
	_ = n.KappController.GetValidatorsKApp().SetAccountsCacher(n.AccountsCacher)

	// Set KappContext
	n.KappController.SetCurrentKAppContext(kappDisabled.NewDisabledKappContext())

	// Set KappController in Validators Kapp
	_ = n.KappController.GetValidatorsKApp().SetKAppController(n.KappController)

	// Add Proposal Controller to Kapps
	_ = n.KappController.SetProposalController(n.ProposalController)

	arguments := peer.ArgValidatorStatisticsProcessor{
		PeerAdapter: n.PeersAdapter,
		KAppAdapter: n.KappsAdapter,
		PubkeyConv:  TestValidatorPubkeyConverter,
		//Use Nodes Coordinator from Bootstrap
		NodesCoordinator:   n.NodesCoordinator,
		DataPool:           n.DataPool,
		StorageService:     n.Store,
		Marshalizer:        n.InternalMarshalizer,
		MaxComputableSlots: n.MainConfig.Preferences.MaxComputableSlots,
		Rater:              rater,
		RewardsHandler:     n.EconomicsData,
		NodesSetup:         n.NodesSetup,
		GenesisNonce:       n.Blkc.GetGenesisHeader().GetNonce(),
		EpochNotifier:      n.EpochNotifier,
		VKApp:              n.KappController.GetValidatorsKApp(),
		ForkController:     n.ForkController,
	}

	n.ValidatorStatistics, err = peer.NewValidatorStatisticsProcessor(arguments)
	if err != nil {
		return err
	}

	return nil
}

func (n *ProcessorNode) initBlockProcessor() error {
	var err error

	n.ForkDetector, err = sync.NewMetaForkDetector(n.SlotManager, n.BlocksBlackListHandler, n.SlotManager.GenesisTimestamp().Unix())
	if err != nil {
		return err
	}

	accountsDBs := make(map[state.AccountsDbIdentifier]state.AccountsAdapter)
	accountsDBs[state.UserAccountsState] = n.AccountsAdapter
	accountsDBs[state.PeerAccountsState] = n.PeersAdapter
	accountsDBs[state.KAppAccountsState] = n.KappsAdapter
	n.AccountsDB = accountsDBs

	if check.IfNil(n.EpochStartNotifier) {
		n.EpochStartNotifier = notifier.NewEpochStartSubscriptionHandler()
	}

	if check.IfNil(n.EpochStartTrigger) {

		epochStartTriggerArgs := &epochStart.ArgsNewEpochStartTrigger{
			GenesisTime:        n.SlotManager.GenesisTimestamp(),
			Epoch:              0,
			EpochStartSlot:     0,
			EpochStartNotifier: n.EpochStartNotifier,
			SlotsPerEpoch:      6,
			Marshalizer:        getMarshalizer(),
			Hasher:             getHasher(),
			Storage:            n.Store,
			ForkController:     n.ForkController,
		}
		epochStartTrigger, err := epochStart.NewEpochStartTrigger(epochStartTriggerArgs)
		if err != nil {
			return err
		}
		n.EpochStartTrigger = epochStartTrigger
	}

	argsEpochStartData := epochStart.ArgsNewEpochStartData{
		Marshalizer:       getMarshalizer(),
		Hasher:            getHasher(),
		Store:             n.Store,
		DataPool:          n.DataPool,
		EpochStartTrigger: n.EpochStartTrigger,
		RequestHandler:    n.RequestHandler,
		GenesisEpoch:      n.GenesisBlock.GetEpoch(),
	}

	epochStartDataCreator, err := epochStart.NewEpochStartData(argsEpochStartData)
	if err != nil {
		return err
	}

	blockSizeThrottler, err := throttle.NewBlockSizeThrottle(0, 10000000)
	if err != nil {
		return err
	}

	txFeeHandler, err := postprocess.NewFeeAccumulator()
	if err != nil {
		return err
	}

	// reset accounts KApp to use cache based on ProcessorFlowITOPrice fork
	n.AccountsCacher.ResetAll(n.ForkController.ProcessorFlowITOPrice())

	err = n.KappController.InitKApps(n.AccountsCacher)
	if err != nil {
		return err
	}

	argsNewTxProcessor := procTx.ArgsNewTxProcessor{
		Cfg:            n.MainConfig,
		KAppController: n.KappController,
		Hasher:         TestHasher,
		Marshalizer:    TestMarshalizer,
		PubkeyConv:     TestAddressPubkeyConverter,
		KeyGen:         n.NodeAccount.KeygenTxSign,
		SingleSigner:   n.TxSingleSigner,
		EconomicsFee:   n.EconomicsData,
		TxFeeHandler:   txFeeHandler,
		EpochNotifier:  n.EpochNotifier,
		RatingsData:    n.RatingsData,
		AccountsCacher: n.AccountsCacher,
		ForkController: n.ForkController,
		ScProcessor:    &mock.SCProcessorMock{},
	}
	n.TxProcessor, err = procTx.NewTxProcessor(argsNewTxProcessor)
	if err != nil {
		return err
	}

	preprocessor, err := preprocess.NewTransactionPreprocessor(
		n.DataPool.Transactions(),
		n.Store,
		n.Hasher,
		n.InternalMarshalizer,
		n.TxProcessor,
		n.AccountsAdapter,
		n.KappsAdapter,
		n.PeersAdapter,
		n.OnRequestTransaction,
		n.EconomicsData,
		TestAddressPubkeyConverter,
		n.ForkController,
	)
	if err != nil {
		return err
	}

	tc, err := coordinator.NewTransactionCoordinator(
		n.Hasher,
		n.InternalMarshalizer,
		n.AccountsAdapter,
		n.KappsAdapter,
		n.RequestHandler,
		preprocessor,
		txFeeHandler,
		n.EconomicsData,
		n.LogProcessor,
		n.ForkController,
	)
	if err != nil {
		return err
	}

	argsGasScheduleNotifier := notifier.ArgsNewGasScheduleNotifier{
		GasScheduleConfig:  n.MainConfig.GasScheduleConfig,
		EpochNotifier:      n.EpochNotifier,
		WasmVMChangeLocker: &syncGo.RWMutex{},
	}
	gasScheduleNotifier, err := notifier.NewGasScheduleNotifier(argsGasScheduleNotifier)
	if err != nil {
		return err
	}

	argsBuiltIn := builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		GasSchedule:     gasScheduleNotifier,
		MapDNSAddresses: make(map[string]struct{}),
		Marshalizer:     n.InternalMarshalizer,
		AccountsCacher:  n.AccountsCacher,
		KAppController:  n.KappController,
		EpochNotifier:   n.EpochNotifier,
		ForkController:  n.ForkController,
	}

	builtInFuncFactory, err := builtInFunctions.CreateBuiltInFunctionsFactory(argsBuiltIn)
	if err != nil {
		return err
	}

	cacherCfg := storageFactory.GetCacherFromConfig(n.MainConfig.SmartContractDataPool)
	smartContractsCache, err := storageUnit.NewCache(cacherCfg)
	if err != nil {
		return err
	}

	argsHook := hooks.ArgBlockChainHook{
		AccountsCacher:     n.AccountsCacher,
		KAppController:     n.KappController,
		PubkeyConv:         n.AddressPubkeyConverter,
		StorageService:     n.Store,
		BlockChain:         n.Blkc,
		Marshalizer:        n.InternalMarshalizer,
		Uint64Converter:    n.Uint64ByteSliceConverter,
		BuiltInFunctions:   builtInFuncFactory.BuiltInFunctionContainer(),
		DataPool:           n.DataPool,
		ConfigSCStorage:    n.MainConfig.Storages.SmartContractsStorage,
		CompiledSCPool:     smartContractsCache,
		WorkingDir:         n.WorkingDir,
		EpochNotifier:      n.EpochNotifier,
		EnableEpochs:       n.MainConfig.EnableEpochs,
		ForkController:     n.ForkController,
		NilCompiledSCStore: true,
		GasSchedule:        gasScheduleNotifier,
		Counter:            counters.NewDisabledCounter(),
	}

	n.BlockChainHook, err = hooks.NewBlockChainHookImpl(argsHook)
	if err != nil {
		return err
	}

	blockProcessorArgs := &processBlock.ArgMetaProcessor{
		ArgBaseProcessor: processBlock.ArgBaseProcessor{
			AccountsDB:              accountsDBs,
			ForkDetector:            n.ForkDetector,
			Hasher:                  n.Hasher,
			Marshalizer:             n.InternalMarshalizer,
			Store:                   n.Store,
			NodesCoordinator:        n.NodesCoordinator,
			Uint64Converter:         n.Uint64ByteSliceConverter,
			RequestHandler:          n.RequestHandler,
			EpochStartTrigger:       n.EpochStartTrigger,
			SlotManager:             n.SlotManager,
			BootStorer:              n.BootStorer,
			DataPool:                n.DataPool,
			BlockChain:              n.Blkc,
			StateCheckpointModulus:  1,
			BlockSizeThrottler:      blockSizeThrottler,
			TpsBenchmark:            &mock.TpsBenchmarkMock{},
			EpochNotifier:           n.EpochNotifier,
			TxCoordinator:           tc,
			FeeHandler:              txFeeHandler,
			Indexer:                 n.Indexer,
			HeaderIntegrityVerifier: n.HeaderIntegrityVerifier,
			KAppController:          n.KappController,
			BlockChainHook:          n.BlockChainHook,
		},
		EpochStartDataCreator:        epochStartDataCreator,
		EconomicsData:                n.EconomicsData,
		ValidatorStatisticsProcessor: n.ValidatorStatistics,
		ForkController:               n.ForkController,
	}

	n.BlockProcessor, err = processBlock.NewMetaProcessor(*blockProcessorArgs)
	if err != nil {
		return err
	}

	err = n.BlockProcessor.SetProposalController(n.ProposalController)
	if err != nil {
		return err
	}

	err = n.TxProcessor.SetProposalController(n.ProposalController)
	if err != nil {
		return err
	}

	err = n.FeeHandler.SetProposalController(n.ProposalController)
	if err != nil {
		return err
	}

	err = n.KappController.SetProposalController(n.ProposalController)
	if err != nil {
		return err
	}

	err = n.KappControllerSimulator.SetProposalController(n.ProposalController)
	if err != nil {
		return err
	}

	n.LogProcessor, err = transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:               nil,
		Marshalizer:          getMarshalizer(),
		SaveInStorageEnabled: false,
	})
	if err != nil {
		return err
	}

	return nil
}

func (n *ProcessorNode) OnRequestTransaction(hashes [][]byte) {
	if n.OnRequestTransactionsHandler == nil {
		return
	}

	n.OnRequestTransactionsHandler(hashes)
}

//--------------------------------------------

func getMarshalizer() marshal.Marshalizer {
	return &mock.ProtoMarshalizerMock{}
}

func getHasher() hashing.Hasher {
	return &mock.HasherMock{}
}

func createKeyGenMock() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			if string(b) == "" {
				return nil, errSingleSignKeyGenMock
			}

			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

func CreateMessengerFromP2P(p2pConfig config.P2PConfig) (p2p.Messenger, error) {
	arg := libp2p.ArgsNetworkMessenger{
		Marshalizer:   getMarshalizer(),
		ListenAddress: libp2p.ListenLocalhostAddrWithIp4AndTcp,
		P2pConfig:     p2pConfig,
		SyncTimer:     &libp2p.LocalSyncTimer{},
	}

	return libp2p.NewNetworkMessenger(arg)
}

func CreatePKBytes() []byte {
	pksBytes := make([]byte, 128)

	return pksBytes
}

func CreateNodeAccount() *NodeAccount {
	singleSigner := &mclsinglesig.BlsSingleSigner{}
	blockSigner := &mock.SingleSignerMock{
		VerifyStub: func(public crypto.PublicKey, msg, sig []byte) error {
			return nil
		},
	}
	txSingleSigner := &singlesig.Ed25519Signer{}

	sk, pk, keyGen := GenerateSkAndPk()

	pkBytes, _ := pk.ToByteArray()

	peerSigCache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000})
	peerSignatureHandler, _ := peerSignatureHandler.NewPeerSignatureHandler(peerSigCache, singleSigner, keyGen)
	testNodeAccount := &NodeAccount{
		SingleSigner:      singleSigner,
		BlockSingleSigner: blockSigner,
		TxSingleSigner:    txSingleSigner,
		Balance:           big.NewInt(0),
		KeygenTxSign:      keyGen,
		KeygenBlockSign:   &mock.KeyGenMock{},
		SkTxSign:          sk,
		PkTxSign:          pk,
		PkTxSignBytes:     pkBytes,
		Address:           pkBytes,
		PeerSigHandler:    peerSignatureHandler,
	}

	return testNodeAccount
}

func CreateNodeAccountWithExistingKeys(sk crypto.PrivateKey, pk crypto.PublicKey, singleSigner crypto.SingleSigner, blockSingleSigner crypto.SingleSigner) (*NodeAccount, error) {
	blockKeyGen := signing.NewKeyGenerator(mcl.NewSuiteBLS12())
	suite := ed25519.NewEd25519()
	keygen := signing.NewKeyGenerator(suite)

	pkBytes, _ := pk.ToByteArray()

	peerSigCache, err := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000})
	if err != nil {
		return nil, err
	}
	peerSignatureHandler, err := peerSignatureHandler.NewPeerSignatureHandler(peerSigCache, singleSigner, keygen)
	if err != nil {
		return nil, err
	}
	testNodeAccount := &NodeAccount{
		SingleSigner:      singleSigner,
		BlockSingleSigner: blockSingleSigner,
		Balance:           big.NewInt(0),
		KeygenTxSign:      keygen,
		KeygenBlockSign:   blockKeyGen,
		SkTxSign:          sk,
		PkTxSign:          pk,
		PkTxSignBytes:     pkBytes,
		Address:           pkBytes,
		PeerSigHandler:    peerSignatureHandler,
	}

	return testNodeAccount, nil
}

func CreateHeaderIntegrityVerifier(cfg config.VersionsConfig) (process.HeaderIntegrityVerifier, error) {
	headerVersioning, err := headerCheck.NewHeaderIntegrityVerifier(
		ChainID,
		cfg.VersionsByEpochs,
		cfg.DefaultVersion,
		&mock.CacherStub{},
	)
	if err != nil {
		return nil, err
	}

	return headerVersioning, nil
}

func (n *ProcessorNode) initNode() error {
	var err error
	n.Node, err = node.NewNode(
		node.WithMessenger(n.Messenger),
		node.WithHasher(TestHasher),
		node.WithInternalMarshalizer(TestMarshalizer),
		node.WithTxSignMarshalizer(TestMarshalizer), // CHECK
		// node.WithInitialNodesPubKeys(crypto.InitialPubKeys), // CHECK
		node.WithAddressPubkeyConverter(TestAddressPubkeyConverter),
		node.WithValidatorPubkeyConverter(TestValidatorPubkeyConverter),
		node.WithAccountsAdapter(n.AccountsAdapter),
		node.WithKAppsAdapter(n.KappsAdapter),
		node.WithKAppController(n.KappController),
		node.WithSimulatorKAppController(n.KappControllerSimulator),
		node.WithBlockChain(n.Blkc),
		node.WithDataStore(n.Store),
		node.WithConsensusGroupSize(int(n.ConsensusGroupSize)),
		node.WithSyncer(n.SyncTimer),
		node.WithTxFeeHandler(n.EconomicsData), // CHECK
		node.WithBlockProcessor(n.BlockProcessor),
		node.WithTransactionProcessor(n.TxProcessor),
		node.WithGenesisTime(time.Unix(n.NodesSetup.GetStartTime(), 0)),
		node.WithNodesCoordinator(n.NodesCoordinator),
		node.WithUint64ByteSliceConverter(n.Uint64ByteSliceConverter),
		node.WithSingleSigner(n.SingleSigner),
		node.WithMultiSigner(n.MultiSigner),
		node.WithKeyGen(createKeyGenMock()),
		node.WithKeyGenForAccounts(n.NodeAccount.KeygenTxSign),
		node.WithPubKey(n.NodeBlockSignKeyPair.Pk),
		node.WithPrivKey(n.NodeBlockSignKeyPair.Sk),
		node.WithInterceptorsContainer(n.InterceptorsContainer),
		node.WithResolversFinder(n.ResolversFinder),
		node.WithTxSingleSigner(n.TxSingleSigner),
		node.WithAppStatusHandler(n.AppStatusHandler),
		node.WithIndexer(n.Indexer),
		node.WithEpochStartTrigger(n.EpochStartTrigger),
		node.WithEpochStartEventNotifier(n.EpochStartNotifier),
		node.WithBlockBlackListHandler(n.BlocksBlackListHandler),
		node.WithPeerDenialEvaluator(n.PeerDenialEvaluator),
		node.WithNetworkShardingCollector(n.NetworkShardingCollector),
		node.WithConsensusType("bls"),
		node.WithBootstrapSlotIndex(n.BootstrapSlotIndex),
		node.WithBootStorer(n.BootStorer),
		node.WithRequestedItemsHandler(n.RequestedItemsHandler),
		node.WithValidatorsProvider(n.ValidatorsProvider),
		node.WithChainID(n.ChainID),
		node.WithForkDetector(n.ForkDetector),
		node.WithMinTransactionVersion(n.MinTransactionVersion),
		node.WithRequestHandler(n.RequestHandler),
		node.WithInputAntifloodHandler(n.InputAntifloodHandler),
		node.WithTxAccumulator(n.TxAccumulator),
		node.WithWhiteListHandler(n.WhiteListHandler),
		node.WithWhiteListHandlerVerified(n.WhiteListerVerifiedTxs),
		node.WithPublicKeySize(96),
		node.WithValidatorSignatureSize(48),
		node.WithNodeStopChannel(n.ChanStopNodeProcess),
		node.WithPeerHonestyHandler(n.PeerHonestyHandler),
		node.WithWatchdogTimer(n.Watchdog),
		node.WithPeerSignatureHandler(n.PeerSigHandler),
		node.WithTxSignHasher(n.TxSignHasher),
		node.WithSlotManager(n.SlotManager),
		node.WithHeaderSigVerifier(n.HeaderSigVerifier),
		node.WithHeaderIntegrityVerifier(n.HeaderIntegrityVerifier),
		node.WithFallbackHeaderValidator(n.FallbackHeaderValidator),
		node.WithNodeRedundancyHandler(n.NodeRedundancyHandler),
		node.WithValidatorStatistics(n.ValidatorStatistics),
		node.WithImportMode(false), // CHECK
		node.WithEnableEpochsConfig(&n.EnableEpochsConfig),
		node.WithForkController(n.ForkController),
		node.WithDataPool(n.DataPool),
		node.WithStartInSync(true),
	)
	if err != nil {
		return err
	}

	return nodeDebugFactory.CreateInterceptedDebugHandler(
		n.Node,
		n.InterceptorsContainer,
		n.ResolversFinder,
		config.InterceptorResolverDebugConfig{
			Enabled:                    true,
			CacheSize:                  1000,
			EnablePrint:                true,
			IntervalAutoPrintInSeconds: 1,
			NumRequestsThreshold:       1,
			NumResolveFailureThreshold: 1,
			DebugLineExpiration:        1000,
		},
	)

}

func (n *ProcessorNode) initAccountDBsWithoutPruningStorer() error {
	n.TrieStorageManagers = make(map[string]data.StorageManager)
	n.TrieContainer = state.NewDataTriesHolder()

	userStorageManager, err := trie.NewTrieStorageManagerWithoutPruning(disabled.CreateMemUnit())
	if err != nil {
		return err
	}
	userAccountTrie, err := trie.NewTrie(userStorageManager, n.InternalMarshalizer, n.Hasher, 5)
	if err != nil {
		return err
	}

	accountFactory := stateFactory.NewAccountCreator()
	n.TrieContainer.Put([]byte(trieFactory.UserAccountTrie), userAccountTrie)
	n.TrieStorageManagers[trieFactory.UserAccountTrie] = userStorageManager
	n.AccountsAdapter, err = state.NewAccountsDB(userAccountTrie, n.Hasher, n.InternalMarshalizer, accountFactory, core.Normal)
	if err != nil {
		return err
	}

	peerStorageManager, err := trie.NewTrieStorageManagerWithoutPruning(disabled.CreateMemUnit())
	if err != nil {
		return err
	}
	peerAccountTrie, err := trie.NewTrie(peerStorageManager, n.InternalMarshalizer, n.Hasher, 5)
	if err != nil {
		return err
	}

	peerFactory := stateFactory.NewPeerAccountCreator()
	n.TrieContainer.Put([]byte(trieFactory.PeerAccountTrie), peerAccountTrie)
	n.TrieStorageManagers[trieFactory.PeerAccountTrie] = peerStorageManager
	n.PeersAdapter, err = state.NewPeerAccountsDB(peerAccountTrie, n.Hasher, n.InternalMarshalizer, peerFactory, core.Normal)
	if err != nil {
		return err
	}

	kappStorageManager, err := trie.NewTrieStorageManagerWithoutPruning(disabled.CreateMemUnit())
	if err != nil {
		return err
	}
	kappAccountTrie, err := trie.NewTrie(kappStorageManager, n.InternalMarshalizer, n.Hasher, 5)
	if err != nil {
		return err
	}

	kappFactory := stateFactory.NewKAppAccountCreator()
	n.TrieContainer.Put([]byte(trieFactory.KAppAccountTrie), kappAccountTrie)
	n.TrieStorageManagers[trieFactory.KAppAccountTrie] = kappStorageManager
	n.KappsAdapter, err = state.NewKAppAccountsDB(kappAccountTrie, n.Hasher, n.InternalMarshalizer, kappFactory, core.Normal)
	if err != nil {
		return err
	}

	n.AccountsCacher, err = state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: n.AccountsAdapter,
			Kapps:    n.KappsAdapter,
			Peers:    n.PeersAdapter,
		},
	)
	return err
}

// GetConnectableAddress returns a non circuit, non windows default connectable p2p address
func (n *ProcessorNode) GetConnectableAddress() string {
	if n == nil {
		return "nil"
	}

	return GetConnectableAddress(n.Messenger)
}

// ConnectTo will try to initiate a connection to the provided parameter
func (n *ProcessorNode) ConnectTo(connectable Connectable) error {
	if check.IfNil(connectable) {
		return fmt.Errorf("trying to connect to a nil Connectable parameter")
	}

	return n.Messenger.ConnectToPeer(connectable.GetConnectableAddress())
}

func (n *ProcessorNode) GetCurrentBlockHeaderAndHash() (data.HeaderHandler, []byte) {
	return n.Blkc.GetCurrentBlockHeader(), n.Blkc.GetCurrentBlockHeaderHash()
}

func (n *ProcessorNode) RevertOneBlock(nonce uint64) error {
	// get current block
	currHeader, err := n.GetBlock(nonce)
	if err != nil {
		return err
	}

	// get last block
	prevHeader, err := n.GetBlock(nonce - 1)
	if err != nil {
		return err
	}

	err = n.Blkc.SetCurrentBlockHeader(prevHeader)
	if err != nil {
		return err
	}

	n.Blkc.SetCurrentBlockHeaderHash(prevHeader.GetParentHash())

	err = n.BlockProcessor.RevertStateToBlock(prevHeader)
	if err != nil {
		return err
	}

	n.BlockProcessor.PruneStateOnRollback(currHeader, prevHeader)

	err = n.BlockProcessor.RestoreBlockIntoPools(currHeader)
	if err != nil {
		return err
	}

	return nil
}

// SyncNode tries to process and commit a block already stored in data pool with provided nonce
func (n *ProcessorNode) SyncNode(nonce uint64) error {
	header, err := n.GetBlock(nonce)
	if err != nil {
		return err
	}

	err = n.BlockProcessor.ProcessBlock(
		header,
		func() time.Duration {
			return time.Second * 4
		},
	)
	if err != nil {
		return err
	}

	err = n.BlockProcessor.CommitBlock(header)
	if err != nil {
		return err
	}

	return nil
}

func freeFeeHandlerMock() *mock.FeeHandlerStub {
	return &mock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return nil, nil
		},
		ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return &transaction.CostResponse{
				KAppFee:      0,
				BandwidthFee: 0,
				RetMessage:   "",
			}, nil
		},
	}
}

// GetConnectableAddress returns a non circuit, non windows default connectable address for provided messenger
func GetConnectableAddress(mes p2p.Messenger) string {
	for _, addr := range mes.Addresses() {
		if strings.Contains(addr, "circuit") || strings.Contains(addr, "169.254") {
			continue
		}
		return addr
	}
	return ""
}

func (n *ProcessorNode) addHandlersForCounters() {
	hdrHandlers := func(header data.HeaderHandler, key []byte) {
		if err := n.HeaderSigVerifier.VerifySignature(header); err != nil {
			return
		}
		atomic.AddInt32(&n.CounterHdrRecv, 1)
	}

	n.DataPool.Headers().RegisterHandler(hdrHandlers)
}

func GenerateNodeKeys(numOfKeys int) ([]*NodeKeyPair, []string, []*NodeKeyPair, []string) {
	txSignKeys := make([]*NodeKeyPair, numOfKeys)
	blkSignKeys := make([]*NodeKeyPair, numOfKeys)

	txSignPkList := make([]string, numOfKeys)
	blkSignPkList := make([]string, numOfKeys)
	// TX SIGN KEYS
	txSuite := ed25519.NewEd25519()
	txKeyGen := signing.NewKeyGenerator(txSuite)

	// BLOCK SIGN KEYS
	blkSuite := mcl.NewSuiteBLS12()
	blkKeyGen := signing.NewKeyGenerator(blkSuite)

	for i := 0; i < numOfKeys; i++ {

		// GENERATE BLOCK SIGN KEYS
		skBlk, pkBlk := blkKeyGen.GeneratePair()
		blkSignKeys[i] = &NodeKeyPair{
			Sk: skBlk,
			Pk: pkBlk,
		}

		pkBlkBytes, _ := pkBlk.ToByteArray()
		blkSignPkList[i] = string(pkBlkBytes)

		//GENERATE TX SIGN KEYS
		skTx, pkTx := txKeyGen.GeneratePair()
		txSignKeys[i] = &NodeKeyPair{
			Sk: skTx,
			Pk: pkTx,
		}

		pkTxBytes, _ := pkTx.ToByteArray()
		txSignPkList[i] = string(pkTxBytes)

	}

	return txSignKeys, txSignPkList, blkSignKeys, blkSignPkList
}

// GenValidatorsFromPubKeys generates a map of validators per shard out of public keys map
func GenValidatorsFromPubKeys(validatorPubKeysList []string, txSignPubkeysList []string) []sharding.GenesisNodeInfoHandler {
	validatorsList := make([]sharding.GenesisNodeInfoHandler, len(validatorPubKeysList))

	for i, nodePk := range validatorPubKeysList {
		v := mock.NewNodeInfo([]byte(txSignPubkeysList[i]), []byte(nodePk), InitialRating)
		validatorsList[i] = v
	}

	return validatorsList
}

func createHeaderIntegrityVerifier() (process.HeaderIntegrityVerifier, error) {
	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000})
	if err != nil {
		return nil, err
	}
	headerVersioning, err := headerCheck.NewHeaderIntegrityVerifier(
		ChainID,
		[]config.VersionByEpochs{
			{
				StartEpoch: 0,
				Version:    "v1",
			},
			{
				StartEpoch: 1,
				Version:    "*",
			},
		},
		"default",
		cache,
	)
	if err != nil {
		return nil, err
	}

	return headerVersioning, nil
}

// ConnectNodes will try to connect all provided connectable instances in a full mesh fashion
func ConnectNodes(nodes []Connectable) {
	encounteredErrors := make([]error, 0)

	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			src := nodes[i]
			dst := nodes[j]
			err := src.ConnectTo(dst)
			if err != nil {
				encounteredErrors = append(encounteredErrors,
					fmt.Errorf("%w while %s was connecting to %s", err, src.GetConnectableAddress(), dst.GetConnectableAddress()))
			}
		}
	}

	printEncounteredErrors(encounteredErrors)

}

func printEncounteredErrors(encounteredErrors []error) {
	if len(encounteredErrors) == 0 {
		return
	}

	printArguments := make([]interface{}, 0, len(encounteredErrors)*2)
	for i, err := range encounteredErrors {
		if err == nil {
			continue
		}

		printArguments = append(printArguments, fmt.Sprintf("err%d", i))
		printArguments = append(printArguments, err.Error())
	}

	log.Warn("errors encountered while connecting hosts", printArguments...)
}

func LoadConfig(relativePath string) (config.Config, error) {
	return config.LoadFromPath(relativePath)
}

func SelectTestNodesForPubKeys(nodes []*ProcessorNode, pubKeys []string) []*ProcessorNode {
	selectedNodes := make([]*ProcessorNode, len(pubKeys))
	cntNodes := 0

	for i, pk := range pubKeys {
		for _, node := range nodes {
			pubKeyBytes, _ := node.NodeBlockSignKeyPair.Pk.ToByteArray()
			if bytes.Equal(pubKeyBytes, []byte(pk)) {
				selectedNodes[i] = node
				cntNodes++
			}
		}
	}

	if cntNodes != len(pubKeys) {
		fmt.Println("Error selecting nodes from public keys")
	}

	return selectedNodes
}

func (n *ProcessorNode) FillHeaderFields(proposer *ProcessorNode, hdr data.HeaderHandler, signer crypto.SingleSigner) (data.HeaderHandler, error) {
	leaderSk := proposer.NodeBlockSignKeyPair.Sk

	hdrClone := hdr
	hdrClone.SetProducerSignature(nil)

	headerJsonBytes, _ := TestMarshalizer.Marshal(hdrClone.(*block.Block).GetBlockHeader())
	leaderSign, _ := signer.Sign(leaderSk, headerJsonBytes)
	hdr.SetProducerSignature(leaderSign)

	return hdr, nil
}

// CreateP2PConfigWithNoDiscovery creates a new libp2p messenger with no peer discovery
func CreateP2PConfigWithNoDiscovery() config.P2PConfig {
	return config.P2PConfig{
		Node: config.NodeConfig{
			Port: "0",
			Seed: "",
		},
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled: false,
		},
		Sharding: config.ShardingConfig{
			Type: p2p.NilListSharder,
		},
	}
}

func (n *ProcessorNode) createIndexer() (process.Indexer, error) {
	epochStartNotifier := notifier.NewEpochStartSubscriptionHandler()

	indexerFactoryArgs := &indexerFactory.ArgsIndexerFactory{
		Enabled:                  true,
		IndexerCacheSize:         100,
		Url:                      "http://localhost:9200",
		Marshalizer:              n.InternalMarshalizer,
		Hasher:                   n.Hasher,
		EpochStartNotifier:       epochStartNotifier,
		NodesCoordinator:         n.NodesCoordinator,
		AddressPubkeyConverter:   n.AddressPubkeyConverter,
		ValidatorPubkeyConverter: n.ValidatorPubkeyConverter,
		EnabledIndexes:           []string{"transactions", "blocks", "accounts", "accountshistory", "assets", "proposals", "marketplaces", "validators", "rating", "epoch", "accountskda", "peersaccounts"},
		AccountsDB:               n.AccountsAdapter,
		KappsDB:                  n.KappsAdapter,
		KAppController:           n.KappController,
		Denomination:             6,
		UseKibana:                true,
		IsInImportDBMode:         false,
	}

	return indexerFactory.NewIndexer(indexerFactoryArgs)
}
