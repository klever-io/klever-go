package retriever

import (
	"fmt"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/counting"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
)

// UnitType is the type for Storage unit identifiers
type UnitType uint8

// String returns the friendly name of the unit
func (ut UnitType) String() string {
	switch ut {
	case TransactionUnit:
		return "TransactionUnit"
	case BlockUnit:
		return "BlockUnit"
	case HdrNonceHashDataUnit:
		return "HdrNonceHashDataUnit"
	case HeartbeatUnit:
		return "HeartbeatUnit"
	case BootstrapUnit:
		return "BootstrapUnit"
	case StatusMetricsUnit:
		return "StatusMetricsUnit"
	}

	return fmt.Sprintf("unknown type %d", ut)
}

const (
	// TransactionUnit is the transactions storage unit identifier
	TransactionUnit UnitType = 0
	// BlockUnit is the transaction block header storage unit identifier
	BlockUnit UnitType = 1
	// HdrNonceHashDataUnit is the header nonce-hash pair data unit identifier
	HdrNonceHashDataUnit UnitType = 2
	// HeartbeatUnit is the heartbeat storage unit identifier
	HeartbeatUnit UnitType = 3
	// BootstrapUnit is the bootstrap storage unit identifier
	BootstrapUnit UnitType = 4
	//StatusMetricsUnit is the status metrics storage unit identifier
	StatusMetricsUnit UnitType = 5
	// TxLogsUnit is the transactions logs storage unit identifier
	TxLogsUnit UnitType = 6
	// EpochByHashUnit is the epoch by hash storage unit identifier
	EpochByHashUnit UnitType = 7
	// BlockHashByTxHashUnit is the blocks hash by tx hash storage unit identifier
	BlockHashByTxHashUnit UnitType = 8
)

// HeadersPool defines what a headers pool structure can perform
type HeadersPool interface {
	Clear()
	AddHeader(headerHash []byte, header data.HeaderHandler)
	RemoveHeaderByHash(headerHash []byte)
	RemoveHeaderByNonce(headerNonce uint64)
	GetHeadersByNonce(headerNonce uint64) ([]data.HeaderHandler, [][]byte, error)
	GetHeaderByHash(hash []byte) (data.HeaderHandler, error)
	RegisterHandler(handler func(headerHandler data.HeaderHandler, headerHash []byte))
	Nonces() []uint64
	Len() int
	MaxSize() int
	IsInterfaceNil() bool
	GetNumHeaders() int
}

// ShardedDataCacherNotifier defines what a sharded-data structure can perform
type ShardedDataCacherNotifier interface {
	RegisterOnAdded(func(key []byte, value interface{}))
	ShardDataStore(cacheID string) (c storage.Cacher)
	AddData(key []byte, data interface{}, sizeInBytes int, cacheID string)
	Notify(txHash []byte, value interface{}, sizeInBytes int)
	SearchFirstData(key []byte) (value interface{}, ok bool)
	RemoveData(key []byte, cacheID string)
	RemoveSetOfDataFromPool(keys [][]byte, cacheID string)
	ImmunizeSetOfDataAgainstEviction(keys [][]byte, cacheID string)
	RemoveDataFromAllShards(key []byte)
	Clear()
	ClearShardStore(cacheID string)
	GetCounts() counting.CountsWithSize
	GetPaginated(cacheID string, page int, pageSize int) ([]interface{}, int)
	GetSenderPaginated(cacheID string, sender []byte, page int, pageSize int) ([]interface{}, int)
	IsInterfaceNil() bool
}

// TransactionCacher defines the methods for the local cacher, info for current slot
type TransactionCacher interface {
	Clean()
	GetTx(txHash []byte) (data.TransactionHandler, error)
	AddTx(txHash []byte, tx data.TransactionHandler)
	IsInterfaceNil() bool
}

// PoolsHolder defines getters for data pools
type PoolsHolder interface {
	Transactions() ShardedDataCacherNotifier
	SmartContracts() storage.Cacher
	Headers() HeadersPool
	TrieNodes() storage.Cacher
	CurrentBlockTxs() TransactionCacher
	IsInterfaceNil() bool
}

// StorageService is the interface for data storage unit provided services
type StorageService interface {
	// GetStorer returns the storer from the chain map
	GetStorer(unitType UnitType) storage.Storer
	// AddStorer will add a new storer to the chain map
	AddStorer(key UnitType, s storage.Storer)
	// Has returns true if the key is found in the selected Unit or false otherwise
	Has(unitType UnitType, key []byte) error
	// Get returns the value for the given key if found in the selected storage unit, nil otherwise
	Get(unitType UnitType, key []byte) ([]byte, error)
	// Put stores the key, value pair in the selected storage unit
	Put(unitType UnitType, key []byte, value []byte) error
	// SetEpochForPutOperation will set the epoch which will be used for the put operation
	SetEpochForPutOperation(epoch uint32)
	// GetAll gets all the elements with keys in the keys array, from the selected storage unit
	// If there is a missing key in the unit, it returns an error
	GetAll(unitType UnitType, keys [][]byte) (map[string][]byte, error)
	// GetAllStorers returns all the storers
	GetAllStorers() map[UnitType]storage.Storer
	// Destroy removes the underlying files/resources used by the storage service
	Destroy() error
	//CloseAll will close all the units
	CloseAll() error
	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// ResolverDebugHandler defines an interface for debugging the reqested-resolved data
type ResolverDebugHandler interface {
	LogRequestedData(topic string, hashes [][]byte, numReqIntra int, numReqCross int)
	LogFailedToResolveData(topic string, hash []byte, err error)
	LogSucceededToResolveData(topic string, hash []byte)
	IsInterfaceNil() bool
}

// Resolver defines what a data resolver should do
type Resolver interface {
	RequestDataFromHash(hash []byte, epoch uint32) error
	ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	SetResolverDebugHandler(handler ResolverDebugHandler) error
	SetNumPeersToQuery(intra int, cross int)
	NumPeersToQuery() (int, int)
	IsInterfaceNil() bool
}

// ResolversContainer defines a resolvers holder data type with basic functionality
type ResolversContainer interface {
	Get(key string) (Resolver, error)
	Add(key string, val Resolver) error
	AddMultiple(keys []string, resolvers []Resolver) error
	Replace(key string, val Resolver) error
	Remove(key string)
	Len() int
	ResolverKeys() string
	Iterate(handler func(key string, resolver Resolver) bool)
	IsInterfaceNil() bool
}

// ResolversFinder extends a container resolver and have 2 additional functionality
type ResolversFinder interface {
	ResolversContainer
	ChainResolver(baseTopic string) (Resolver, error)
}

// DataPacker can split a large slice of byte slices in smaller packets
type DataPacker interface {
	PackDataInChunks(data [][]byte, limit int) ([][]byte, error)
	IsInterfaceNil() bool
}

// MessageHandler defines the functionality needed by structs to send data to other peers
type MessageHandler interface {
	ConnectedPeersOnTopic(topic string) []core.PeerID
	SendToConnectedPeer(topic string, buff []byte, peerID core.PeerID) error
	ID() core.PeerID
	IsInterfaceNil() bool
}

// TopicHandler defines the functionality needed by structs to manage topics and message processors
type TopicHandler interface {
	HasTopic(name string) bool
	CreateTopic(name string, createChannelForTopic bool) error
	RegisterMessageProcessor(topic string, handler p2p.MessageProcessor) error
}

// TopicMessageHandler defines the functionality needed by structs to manage topics, message processors and to send data
// to other peers
type TopicMessageHandler interface {
	MessageHandler
	TopicHandler
}

// ManualEpochStartNotifier can manually notify an epoch change
type ManualEpochStartNotifier interface {
	NewEpoch(epoch uint32)
	CurrentEpoch() uint32
	IsInterfaceNil() bool
}

// IntRandomizer interface provides functionality over generating integer numbers
type IntRandomizer interface {
	Intn(n int) int
	IsInterfaceNil() bool
}

// ResolverThrottler can monitor the number of the currently running resolver go routines
type ResolverThrottler interface {
	CanProcess() bool
	StartProcessing()
	EndProcessing()
	IsInterfaceNil() bool
}

// TopicResolverSender defines what sending operations are allowed for a topic resolver
type TopicResolverSender interface {
	SendOnRequestTopic(rd *RequestData, originalHashes [][]byte) error
	SendOnRequestTopicTo(rd *RequestData, originalHashes [][]byte, peer core.PeerID) error
	Send(buff []byte, peer core.PeerID) error
	RequestTopic() string
	SetNumPeersToQuery(intra int, cross int)
	SetResolverDebugHandler(handler ResolverDebugHandler) error
	ResolverDebugHandler() ResolverDebugHandler
	NumPeersToQuery() (int, int)
	IsInterfaceNil() bool
}

// ResolversContainerFactory defines the functionality to create a resolvers container
type ResolversContainerFactory interface {
	Create() (ResolversContainer, error)
	IsInterfaceNil() bool
}

// EpochHandler defines the functionality to get the current epoch
type EpochHandler interface {
	Epoch() uint32
	IsInterfaceNil() bool
}

// P2PAntifloodHandler defines the behavior of a component able to signal that the system is too busy (or flooded) processing
// p2p messages
type P2PAntifloodHandler interface {
	CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopic(peer core.PeerID, topic string, numMessages uint32, _ uint64, sequence []byte) error
	BlacklistPeer(peer core.PeerID, reason string, duration time.Duration)
	IsInterfaceNil() bool
}

// HeaderResolver defines what a block header resolver should do
type HeaderResolver interface {
	Resolver
	RequestDataFromNonce(nonce uint64, epoch uint32) error
	RequestDataFromEpoch(identifier []byte) error
	SetEpochHandler(epochHandler EpochHandler) error
}

// EpochProviderByNonce defines the functionality needed for calculating an epoch based on nonce
type EpochProviderByNonce interface {
	EpochForNonce(nonce uint64) (uint32, error)
	IsInterfaceNil() bool
}

// TrieDataGetter returns requested data from the trie
type TrieDataGetter interface {
	GetSerializedNodes([]byte, uint64) ([][]byte, uint64, error)
	IsInterfaceNil() bool
}

// PeerListCreator is used to create a peer list
type PeerListCreator interface {
	PeerList() []core.PeerID
	ConsensusPeerList() []core.PeerID
	IsInterfaceNil() bool
}

// BlocksResolver defines what a blocks resolver should do
type BlocksResolver interface {
	Resolver
	RequestDataFromHashArray(hashes [][]byte, epoch uint32) error
}

// TrieNodesResolver defines what a trie nodes resolver should do
type TrieNodesResolver interface {
	Resolver
	RequestDataFromHashArray(hashes [][]byte, epoch uint32) error
	RequestDataFromHashArrayTo(hashes [][]byte, epoch uint32, _ core.PeerID) error
}

// WhiteListHandler is the interface needed to add whitelisted data
type WhiteListHandler interface {
	Remove(keys [][]byte)
	Add(keys [][]byte)
	IsInterfaceNil() bool
}

// RequestedItemsHandler can determine if a certain key has or not been requested
type RequestedItemsHandler interface {
	Add(key string) error
	Has(key string) bool
	Sweep()
	ResetAll()
	IsInterfaceNil() bool
}
