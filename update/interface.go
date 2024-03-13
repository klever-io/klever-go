package update

import (
	"context"
	"time"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// StateSyncer interface defines the methods needed to sync and get all states
type StateSyncer interface {
	GetEpochStartMetaBlock() (*block.Block, error)
	GetUnFinishedMetaBlocks() (map[string]*block.Block, error)
	SyncAllState(epoch uint32) error
	GetAllTries() (map[string]data.Trie, error)
	GetAllTransactions() (map[string]data.TransactionHandler, error)
	IsInterfaceNil() bool
}

// TrieSyncer synchronizes the trie, asking on the network for the missing nodes
type TrieSyncer interface {
	StartSyncing(rootHash []byte, ctx context.Context) error
	Trie() data.Trie
	IsInterfaceNil() bool
}

// TrieSyncContainer keep a list of TrieSyncer
type TrieSyncContainer interface {
	Get(key string) (TrieSyncer, error)
	Add(key string, val TrieSyncer) error
	AddMultiple(keys []string, interceptors []TrieSyncer) error
	Replace(key string, val TrieSyncer) error
	Remove(key string)
	Len() int
	IsInterfaceNil() bool
}

// EpochStartVerifier defines the functionality needed by sync all state from epochTrigger
type EpochStartVerifier interface {
	IsEpochStart() bool
	Epoch() uint32
	EpochStartHdrHash() []byte
	IsInterfaceNil() bool
}

// HistoryStorer provides storage services in a two layered storage construct, where the first layer is
// represented by a cache and second layer by a persitent storage (DB-like)
type HistoryStorer interface {
	Put(key, data []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) error
	Remove(key []byte) error
	ClearCache()
	DestroyUnit() error
	GetFromEpoch(key []byte, epoch uint32) ([]byte, error)
	HasInEpoch(key []byte, epoch uint32) error

	IsInterfaceNil() bool
}

// RequestHandler defines the methods through which request to data can be made
type RequestHandler interface {
	RequestTransaction(txHashes [][]byte)
	RequestStartOfEpochBlock(epoch uint32)
	RequestHeader(hash []byte)
	RequestHeaderByNonce(nonce uint64)
	RequestTrieNodes(hashes [][]byte, topic string)
	RequestInterval() time.Duration
	SetNumPeersToQuery(key string, intra int, cross int) error
	GetNumPeersToQuery(key string) (int, int, error)
	IsInterfaceNil() bool
}

// ExportHandler defines the methods to export the current state of the blockchain
type ExportHandler interface {
	ExportAll(epoch uint32) error
	IsInterfaceNil() bool
}

// ImportHandler defines the methods to import the full state of the blockchain
type ImportHandler interface {
	ImportAll() error
	GetValidatorAccountsDB() state.AccountsAdapter
	GetHardForkMetaBlock() *block.Block
	GetUnFinishedMetaBlocks() map[string]*block.Block
	GetTransactions() map[string]data.TransactionHandler
	GetAccountsDB() state.AccountsAdapter
	IsInterfaceNil() bool
}

// HardForkBlockProcessor defines the methods to process after hardfork
type HardForkBlockProcessor interface {
	CreateBlock(body *block.Block, chainID string, slot uint64, nonce uint64, epoch uint32) (data.HeaderHandler, error)
	CreateBody() (*block.Block, error)
	CreatePostMiniBlocks(mbsInfo []*MbInfo) (*block.BlockHeader, []*MbInfo, error)
	IsInterfaceNil() bool
}

// PendingTransactionProcessor defines the methods to process a transaction destination me
type PendingTransactionProcessor interface {
	ProcessTransactions(mbInfo *MbInfo) (*block.Block, error)
	RootHash() ([]byte, error)
	Commit() ([]byte, error)
	IsInterfaceNil() bool
}

// HeaderSyncHandler defines the methods to sync and get the epoch start metablock
type HeaderSyncHandler interface {
	SyncUnFinishedMetaHeaders(epoch uint32) error
	GetEpochStartMetaBlock() (*block.Block, error)
	GetUnFinishedMetaBlocks() (map[string]*block.Block, error)
	IsInterfaceNil() bool
}

// EpochStartTriesSyncHandler defines the methods to sync all tries from a given epoch start metablock
type EpochStartTriesSyncHandler interface {
	SyncTriesFrom(meta *block.Block) error
	GetTries() (map[string]data.Trie, error)
	IsInterfaceNil() bool
}

// PendingTransactionsSyncHandler defines the methods to sync all transactions from a set of miniblocks
type PendingTransactionsSyncHandler interface {
	SyncPendingTransactions(block *block.Block, epoch uint32, ctx context.Context) error
	GetTransactions() (map[string]data.TransactionHandler, error)
	IsInterfaceNil() bool
}

// DataWriter defines the methods to write data
type DataWriter interface {
	WriteString(s string) (int, error)
	Flush() error
}

// WhiteListHandler is the interface needed to add whitelisted data
type WhiteListHandler interface {
	Remove(keys [][]byte)
	Add(keys [][]byte)
	IsWhiteListed(interceptedData process.InterceptedData) bool
	IsInterfaceNil() bool
}

// AccountsDBSyncer defines the methods for the accounts db syncer
type AccountsDBSyncer interface {
	GetSyncedTries() map[string]data.Trie
	SyncAccounts(rootHash []byte) error
	IsInterfaceNil() bool
}

// AccountsDBSyncContainer keep a list of TrieSyncer
type AccountsDBSyncContainer interface {
	Get(key string) (AccountsDBSyncer, error)
	Add(key string, val AccountsDBSyncer) error
	AddMultiple(keys []string, values []AccountsDBSyncer) error
	Replace(key string, val AccountsDBSyncer) error
	Remove(key string)
	Len() int
	IsInterfaceNil() bool
}

// SigVerifier is used to verify the signature on a provided message
type SigVerifier interface {
	Verify(message []byte, sig []byte, pk []byte) error
	IsInterfaceNil() bool
}

// EpochHandler defines the functionality to get the current epoch
type EpochHandler interface {
	Epoch() uint32
	ForceEpochStart(slot uint64)
	IsInterfaceNil() bool
}

// ExportFactoryHandler defines the functionality to create and export handler
type ExportFactoryHandler interface {
	Create() (ExportHandler, error)
	IsInterfaceNil() bool
}

// EpochChangeConfirmedNotifier defines the functionality needed to register for the epoch change confirmed event
type EpochChangeConfirmedNotifier interface {
	RegisterForEpochChangeConfirmed(handler func(epoch uint32))
	IsInterfaceNil() bool
}

// ImportStartHandler can manage the process of starting the import after the hardfork event
type ImportStartHandler interface {
	IsAfterExportBeforeImport() bool
	ShouldStartImport() bool
	ResetStartImport() error
	SetStartImport() error
	IsInterfaceNil() bool
}

// HardforkStorer manages the export and import of data
type HardforkStorer interface {
	Write(identifier string, key []byte, value []byte) error
	FinishedIdentifier(identifier string) error
	RangeKeys(handler func(identifier string, keys [][]byte) bool)
	Get(identifier string, key []byte) ([]byte, error)
	Close() error
	IsInterfaceNil() bool
}

// GenesisNodesSetupHandler returns the genesis nodes info
type GenesisNodesSetupHandler interface {
	InitialNodesInfo() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error)
	GetStartTime() int64
	GetSlotInterval() uint64
	GetChainID() string
	GetMinTransactionVersion() uint32
	GetConsensusGroupSize() uint32
	MinNumberOfNodes() uint32
	IsInterfaceNil() bool
}

// SlotManager defines the actions which should be handled by a slot implementation
type SlotManager interface {
	Index() int64
	Timestamp() time.Time
	IsInterfaceNil() bool
}
