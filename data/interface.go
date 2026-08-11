package data

import (
	"context"

	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/transaction"
)

// HeaderHandler defines getters and setters for header data holder
type HeaderHandler interface {
	GetNonce() uint64
	GetParentHash() []byte
	SetParentHash(parentHash []byte)
	GetTimestamp() int64
	SetTimestamp(timestamp int64)
	GetSlot() uint64
	GetEpoch() uint32
	GetIsEpochStart() bool

	GetTrieRoot() []byte
	SetTrieRoot(hash []byte)
	GetTxRootHash() []byte
	GetValidatorsTrieRoot() []byte
	GetKAppsTrieRoot() []byte
	SetKAppsTrieRoot([]byte)

	GetRandSeed() []byte
	SetRandSeed(randSeed []byte)
	GetPrevRandSeed() []byte
	SetPrevRandSeed(prevRandSeed []byte)

	GetTxCount() uint32
	GetSoftwareVersion() []byte
	SetSoftwareVersion(softwareVersion []byte)
	GetChainID() []byte
	SetChainID(chainID []byte)

	GetProducerSignature() []byte
	SetProducerSignature(signature []byte)
	GetPubKeysBitmap() []byte
	SetPubKeysBitmap(signature []byte)
	GetSignature() []byte
	SetSignature(signature []byte)
	GetTxFees() int64
	GetTxBurnedFees() int64
	GetKAppFees() int64
	GetBlockRewards() int64
	GetBurnedUnclaimed() int64
	SetBurnedUnclaimed(burned int64)
	GetStakingRewards() int64
	GetReserved() []byte
	GetPrevEpochStartSlot() uint64

	GetBlockHeader() interface{}
	GetTxHashes() [][]byte
	ComputeRootHash(hasher hashing.Hasher) ([]byte, error)

	IsInterfaceNil() bool
	Clone() HeaderHandler
}

// TransactionHandler defines the type of executable transaction
type TransactionHandler interface {
	GetSender() []byte
	GetRaw() *transaction.Transaction_Raw
	GetData() [][]byte
	GetDataWithIdx(int) []byte
	GetTotalFees() int64
	GetBandwidthFee() int64
	GetNonce() uint64
	IsInterfaceNil() bool

	Size() int
	GetGasLimit() uint64
	GetGasMultiplier() uint64
	CheckIntegrity() error
}

// ContractHandler defines the type of executable contract
type SmartContractHandler interface {
	GetType() transaction.SmartContract_SCType
	GetAddress() []byte
	GetCallValue() map[string]*transaction.CallValue
	IsInterfaceNil() bool
}

type KDAFeeHandler interface {
	GetKDA() []byte
	GetAmount() int64
	IsInterfaceNil() bool
}

// ChainHandler is the interface defining the functionality a blockchain should implement
type ChainHandler interface {
	GetGenesisHeader() HeaderHandler
	SetGenesisHeader(gb HeaderHandler) error
	GetGenesisHeaderHash() []byte
	SetGenesisHeaderHash(hash []byte)
	GetCurrentBlockHeader() HeaderHandler
	SetCurrentBlockHeader(bh HeaderHandler) error
	GetCurrentBlockHeaderHash() []byte
	SetCurrentBlockHeaderHash(hash []byte)
	SetCurrentBlockHeaderAndHash(bh HeaderHandler, hash []byte) error
	GetCurrentBlockHeaderAndHash() (HeaderHandler, []byte)
	GetCurrentBlockRootHash() []byte
	CreateNewHeader() HeaderHandler
	IsInterfaceNil() bool
}

// TriePruningIdentifier is the type for trie pruning identifiers
type TriePruningIdentifier byte

const (
	// OldRoot is appended to the key when oldHashes are added to the evictionWaitingList
	OldRoot TriePruningIdentifier = 0
	// NewRoot is appended to the key when newHashes are added to the evictionWaitingList
	NewRoot TriePruningIdentifier = 1
)

// ModifiedHashes is used to memorize all old hashes and new hashes from when a trie is committed
type ModifiedHashes map[string]struct{}

// DBWriteCacher is used to cache changes made to the trie, and only write to the database when it's needed
type DBWriteCacher interface {
	Put(key, val []byte) error
	Get(key []byte) ([]byte, error)
	Remove(key []byte) error
	Close() error
	IsInterfaceNil() bool
}

// KeyValueHolder is used to hold a key and an associated value
type KeyValueHolder interface {
	Key() []byte
	Value() []byte
}

// Trie is an interface for Merkle Trees implementations
type Trie interface {
	Get(key []byte) ([]byte, error)
	Update(key, value []byte) error
	Delete(key []byte) error
	RootHash() ([]byte, error)
	Commit() error
	Recreate(root []byte) (Trie, error)
	String() string
	ResetOldHashes() [][]byte
	AppendToOldHashes([][]byte)
	GetDirtyHashes() (ModifiedHashes, error)
	SetNewHashes(ModifiedHashes)
	GetSerializedNodes([]byte, uint64) ([][]byte, uint64, error)
	GetAllLeavesOnChannel(rootHash []byte, ctx context.Context) (*TrieIteratorChannels, error)
	GetAllHashes() ([][]byte, error)
	IsInterfaceNil() bool
	ClosePersister() error
	GetProof(key []byte) ([][]byte, error)
	VerifyProof(key []byte, proof [][]byte) (bool, error)
	GetStorageManager() StorageManager
}

// SnapshotDbHandler is used to keep track of how many references a snapshot db has
type SnapshotDbHandler interface {
	DBWriteCacher
	IsInUse() bool
	DecreaseNumReferences()
	IncreaseNumReferences()
	MarkForRemoval()
	SetPath(string)
}

// StorageManager manages all trie storage operations
type StorageManager interface {
	Database() DBWriteCacher
	TakeSnapshot([]byte)
	SetCheckpoint([]byte)
	Prune([]byte, TriePruningIdentifier)
	CancelPrune([]byte, TriePruningIdentifier)
	MarkForEviction([]byte, ModifiedHashes) error
	GetSnapshotThatContainsHash(rootHash []byte) SnapshotDbHandler
	IsPruningEnabled() bool
	EnterPruningBufferingMode()
	ExitPruningBufferingMode()
	GetSnapshotDbBatchDelay() int
	Close() error
	IsInterfaceNil() bool
}

// DBRemoveCacher is used to cache keys that will be deleted from the database
type DBRemoveCacher interface {
	Put([]byte, ModifiedHashes) error
	Evict([]byte) (ModifiedHashes, error)
	ShouldKeepHash(hash string, identifier TriePruningIdentifier) (bool, error)
	IsInterfaceNil() bool
	Close() error
}

// TrieSyncer synchronizes the trie, asking on the network for the missing nodes
type TrieSyncer interface {
	StartSyncing(rootHash []byte, ctx context.Context) error
	Trie() Trie
	IsInterfaceNil() bool
}

// SyncStatisticsHandler defines the methods for a component able to store the sync statistics for a trie
type SyncStatisticsHandler interface {
	Reset()
	AddNumReceived(value int)
	SetNumMissing(rootHash []byte, value int)
	NumReceived() int
	NumMissing() int
	IsInterfaceNil() bool
}

// BlockInfo holds information about a cross block referenced in a received block
type BlockInfo struct {
	Hash []byte
	Slot uint64
}

// ValidatorInfoHandler is used to store multiple validatorInfo properties required in shards
type ValidatorInfoHandler interface {
	GetPublicKey() []byte
	GetTempRating() uint32
	String() string
	IsInterfaceNil() bool
}

// GoRoutineThrottler can monitor the number of the currently running go routines
type GoRoutineThrottler interface {
	CanProcess() bool
	StartProcessing()
	EndProcessing()
	IsInterfaceNil() bool
}

// LogHandler defines the type for a log resulted from executing a transaction or smart contract call
type LogHandler interface {
	// GetAddress returns the address of the sc that was originally called by the user
	GetAddress() []byte
	// GetLogEvents returns the events from a transaction log entry
	GetLogEvents() []transaction.EventHandler

	IsInterfaceNil() bool
}

type ProcessResults interface {
	Hashes() [][]byte
	Length() int
	Size() int64
	IsInterfaceNil() bool
}
