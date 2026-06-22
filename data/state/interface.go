package state

import (
	"context"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// AccountsDbIdentifier is the type of accounts db
type AccountsDbIdentifier byte

const (
	// UserAccountsState is the user accounts
	UserAccountsState AccountsDbIdentifier = 0
	// KAppAccountsState is the peer accounts
	KAppAccountsState AccountsDbIdentifier = 1
	// PeerAccountsState is the peer accounts
	PeerAccountsState AccountsDbIdentifier = 2
)

// UserAccountHandler models a user account, which can journalize account's data with some extra features
// like balance, developer rewards, owner
type UserAccountHandler interface {
	SetCode(code []byte)
	SetCodeMetadata(codeMetadata []byte)
	GetCodeMetadata() []byte
	SetCodeHash([]byte)
	GetCodeHash() []byte
	GetOwnerAddress() []byte
	SetOwnerAddress([]byte)
	SaveKeyValue(key []byte, value []byte) error
	RetrieveValue(key []byte) ([]byte, error)
	GetNonce() uint64
	GetUserKDA(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error)
	SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error
	SetRootHash([]byte)
	GetRootHash() []byte
	AddToBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error
	AddToBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error
	SubFromBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error
	SubFromBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error
	AddInternalKDA(assetID []byte, internalID []byte, data []byte) error
	SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error)
	AddToAllowance(value int64) error
	GetBalance(assetID []byte, cdd bool) int64
	GetBalanceWithNonce(assetID []byte, nonce []byte, cdd bool) int64
	GetAllowance() int64
	GetFrozenBalance(assetID []byte, cdd bool) int64
	GetBuckets(assetID []byte, cdd bool) map[string]*kapps.UserBucket
	Freeze(assetID, bucketID []byte, value int64, blockEpoch uint32, blockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) error
	Unfreeze(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error)
	Delegate(bucketID, delegation []byte, userKDA *kapps.UserKDA) (int64, error)
	Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error)
	Claim(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error)
	ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error)
	Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error)
	GetPermission(id int32) (*Permission, bool, error)
	SetPermissions([]*Permission)
	GetPermissions() []*Permission
	SetName(name []byte)
	GetName() []byte
	SetDataTrie(trie data.Trie)
	DataTrie() data.Trie
	HasNewCode() bool
	DataTrieTracker() DataTrieTracker
	AccountDataHandler() AccountDataHandler
	Equal(UserAccountHandler) bool
	AccountHandler
}

// AccountDataHandler models what how to manipulate data held by a SC account
type AccountDataHandler interface {
	RetrieveValue(key []byte) ([]byte, error)
	SaveKeyValue(key []byte, value []byte) error
	IsInterfaceNil() bool
}

// KAppAccountHandler models a KApp account, which can journalize account's data with some extra features
type KAppAccountHandler interface {
	SetRootHash([]byte)
	GetRootHash() []byte
	SetName(userName []byte)
	GetName() []byte
	StartProposalsKApp(forks core.ForkController) (kapps.ActiveProposalController, error)
	GetNonce() uint64
	IncreaseNonce(nonce uint64)
	GetUserKDA(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error)
	AddInternalKDA(assetID []byte, internalID []byte, data []byte) error
	SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error)
	SetDataTrie(trie data.Trie)
	DataTrie() data.Trie
	DataTrieTracker() DataTrieTracker
	GetStorage(key []byte) []byte
	SetStorage(key []byte, value []byte) error
	AccountHandler
}

// AccountHandler models a state account, which can journalize and revert
// It knows about code and data, as data structures not hashes
type AccountHandler interface {
	AddressBytes() []byte
	IncreaseNonce(nonce uint64)
	GetNonce() uint64
	IsInterfaceNil() bool
}

// AccountHandler models a state account, which can journalize and revert
// It knows about code and data, as data structures not hashes
type AccountHandlerWithGetUserKDA interface {
	AccountHandler
	GetUserKDA(assetID []byte, nonce []byte, checkDirtyData bool) (*kapps.UserKDA, error)
}

// BasicAccountsAdapter is a simplified version of AccountsAdapter with only
// the necessary methods for smart contracts VM
type BasicAccountsAdapter interface {
	GetExistingAccount(address []byte) (AccountHandler, error)
	LoadAccount(address []byte) (AccountHandler, error)
	SaveAccount(account AccountHandler) error
	SaveAccounts(accounts ...AccountHandler) error
	RemoveAccountCode(address []byte) error
	RemoveAccount(address []byte) error
	Commit() ([]byte, error)
	JournalLen() int
	RevertToSnapshot(snapshot int) error
	GetCode(codeHash []byte) []byte

	RootHash() ([]byte, error)
	IsInterfaceNil() bool
}

// AccountsAdapter is used for the structure that manages the accounts on top of a trie.PatriciaMerkleTrie
// implementation
type AccountsAdapter interface {
	BasicAccountsAdapter
	GetNumCheckpoints() uint32

	RecreateTrie(rootHash []byte) error
	PruneTrie(rootHash []byte, identifier data.TriePruningIdentifier)
	CancelPrune(rootHash []byte, identifier data.TriePruningIdentifier)
	SnapshotState(rootHash []byte, ctx context.Context)
	SetStateCheckpoint(rootHash []byte, ctx context.Context)
	IsPruningEnabled() bool
	GetAllLeaves(rootHash []byte, ctx context.Context) (chan data.KeyValueHolder, error)
	RecreateAllTries(rootHash []byte, ctx context.Context) (map[string]data.Trie, error)

	IsInterfaceNil() bool
}

// TriesHolder is used to store multiple tries
type TriesHolder interface {
	Put([]byte, data.Trie)
	Replace(key []byte, tr data.Trie)
	Get([]byte) data.Trie
	GetAll() []data.Trie
	Reset()
	IsInterfaceNil() bool
}

// JournalAccountEntry will be used to implement different state changes to be able to easily revert them
type JournalAccountEntry interface {
	Revert() (AccountHandler, error)
	IsInterfaceNil() bool
}

type baseAccountHandler interface {
	AddressBytes() []byte
	IncreaseNonce(nonce uint64)
	GetNonce() uint64
	HasNewCode() bool
	SetRootHash([]byte)
	GetRootHash() []byte
	SetDataTrie(trie data.Trie)
	DataTrie() data.Trie
	DataTrieTracker() DataTrieTracker
	IsInterfaceNil() bool
}

// AccountsDBImporter is used in importing accounts
type AccountsDBImporter interface {
	ImportAccount(account AccountHandler) error
	Commit() ([]byte, error)
	IsInterfaceNil() bool
}

// Updater set a new value for a key, implemented by trie
type Updater interface {
	Get(key []byte) ([]byte, error)
	Update(key, value []byte) error
	IsInterfaceNil() bool
}

// DataTrieTracker models what how to manipulate data held by a SC account
type DataTrieTracker interface {
	ClearDataCaches()
	DirtyData() map[string][]byte
	RetrieveValue(key []byte) ([]byte, error)
	SaveKeyValue(key []byte, value []byte) error
	SetDataTrie(tr data.Trie)
	DataTrie() data.Trie
	IsInterfaceNil() bool
}

// AccountFactory creates an account of different types
type AccountFactory interface {
	CreateAccount(address []byte) (AccountHandler, error)
	IsInterfaceNil() bool
}

// PeerAccountHandler models a peer state account, which can journalize a normal account's data
// with some extra features like signing statistics or rating information
type PeerAccountHandler interface {
	GetBLSPublicKey() []byte
	SetBLSPublicKey([]byte) error
	GetOwnerAddress() []byte
	SetOwnerAddress([]byte) error
	SetRevoked()
	GetRevoked() bool
	GetAccumulatedFees() int64
	GetList() List
	GetIndex() uint32
	GetListString() string
	GetLeaderSuccessRateSuccess() uint32
	GetTotalLeaderSuccessRateSuccess() uint32
	GetValidatorSuccessRateSuccess() uint32
	GetTotalValidatorSuccessRateSuccess() uint32
	GetLeaderSuccessRateFailure() uint32
	GetTotalLeaderSuccessRateFailure() uint32
	GetValidatorSuccessRateFailure() uint32
	GetTotalValidatorSuccessRateFailure() uint32
	IncreaseLeaderSuccessRate(uint32)
	DecreaseLeaderSuccessRate(uint32)
	IncreaseValidatorSuccessRate(uint32)
	DecreaseValidatorSuccessRate(uint32)
	IncreaseValidatorIgnoredSignaturesRate(uint32)
	GetNumSelectedInSuccessBlocks() uint32
	IncreaseNumSelectedInSuccessBlocks()
	GetLeaderSuccessRate() *SignRate
	GetValidatorSuccessRate() *SignRate
	GetValidatorIgnoredSignaturesRate() uint32
	GetTotalLeaderSuccessRate() *SignRate
	GetTotalValidatorSuccessRate() *SignRate
	GetTotalValidatorIgnoredSignaturesRate() uint32
	SetList(list List)
	SetListAndIndex(list List, index uint32)
	GetRating() uint32
	SetRating(uint32)
	GetTempRating() uint32
	SetTempRating(uint32)
	GetConsecutiveProposerMisses() uint32
	SetConsecutiveProposerMisses(uint322 uint32)
	AddToAccumulatedFees(value int64)
	ResetAtNewEpoch()
	CopyFrom(old PeerAccountHandler) error
	AccountHandler
}

// AccountsCacher holds accounts instances to be saved at the end of operations
type AccountsCacher interface {
	GetExistingUser(address []byte) (UserAccountHandler, error)
	GetExistingKapp(address []byte) (KAppAccountHandler, error)
	GetExistingPeer(address []byte) (PeerAccountHandler, error)
	LoadUser(address []byte) (UserAccountHandler, error)
	LoadKApp(address []byte) (KAppAccountHandler, error)
	LoadKAppUncached(address []byte) (KAppAccountHandler, error)
	LoadPeer(address []byte) (PeerAccountHandler, error)
	ResetAll(enableCache bool)
	SaveAll() error
	SaveUser(account AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdateUser(account AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdateKapp(account AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdatePeer(account AccountHandler) error
	GetCode(codeHash []byte) []byte
	RemoveCode(address []byte) error
	IsInterfaceNil() bool
}
