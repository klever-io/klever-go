package genesis

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/update"
)

// EpochStartMetaBlockIdentifier is the constant which defines the export/import identifier for epoch start metaBlock
const EpochStartMetaBlockIdentifier = "epochStartMetaBlock"

// TransactionsIdentifier is the constant which defines the export/import identifier for transactions
const TransactionsIdentifier = "transactions"

// TrieIdentifier is the constant which defines the export/import identifier for tries
const TrieIdentifier = "trie"

// Type identifies the type of the export / import
type Type uint8

const (
	// Unknown is an export/import type which is not known by the system
	Unknown Type = iota
	// Transaction is the export/import type for pending transactions
	Transaction
	// Header is the export/import type for pending headers
	Header
	// RootHash is the export/import type for byte array which has to be treated as rootHash
	RootHash
	// UserAccount is the export/import type for an account of type user account
	UserAccount
	// ValidatorAccount is the export/import type for peer account
	ValidatorAccount
	// DataTrie identifies the data trie kept under a specific account
	DataTrie
)

// atSep is a separator used for export and import to decipher needed types
const atSep = "@"
const accTypeIDX = 2

// NewObject creates an object according to the given type
func NewObject(objType Type) (interface{}, error) {
	switch objType {
	case Transaction:
		return &transaction.Transaction{}, nil
	case Header:
		return &block.Block{Header: &block.BlockHeader{}}, nil
	case RootHash:
		return make([]byte, 0), nil
	}
	return nil, update.ErrUnknownType
}

// NewEmptyAccount returns a new account according to the given type
func NewEmptyAccount(accType Type, address []byte) (state.AccountHandler, error) {
	switch accType {
	case UserAccount:
		return state.NewUserAccount(address)
	case ValidatorAccount:
		return state.NewPeerAccount(address)
	case DataTrie:
		return nil, nil
	}
	return nil, update.ErrUnknownType
}

// GetTrieType returns the type for a given account according to the saved key
func GetTrieType(key string) (Type, error) {
	splitString := strings.Split(key, atSep)
	if len(splitString) < 3 {
		return UserAccount, update.ErrUnknownType
	}

	accTypeInt64, err := strconv.ParseInt(splitString[accTypeIDX], 10, 0)
	if err != nil {
		return UserAccount, err
	}
	accType := Type(accTypeInt64) // #nosec G115

	return accType, nil
}

func getTransactionKeyTypeAndHash(splitString []string) (Type, []byte, error) {
	if len(splitString) < 2 {
		return Unknown, nil, update.ErrUnknownType
	}

	decodedHash, err := hex.DecodeString(splitString[1])
	if err != nil {
		return Unknown, nil, update.ErrUnknownType
	}

	switch splitString[0] {
	case "nrm":
		return Transaction, decodedHash, nil
	}

	return Unknown, nil, update.ErrUnknownType
}

func getTrieTypeAndHash(splitString []string) (Type, []byte, error) {
	if len(splitString) < 3 {
		return Unknown, nil, update.ErrUnknownType
	}

	accTypeInt64, err := strconv.ParseInt(splitString[1], 10, 0)
	if err != nil {
		return Unknown, nil, err
	}
	accType := Type(accTypeInt64) // #nosec G115

	decodedHash, err := hex.DecodeString(splitString[2])
	if err != nil {
		return Unknown, nil, err
	}

	return accType, decodedHash, nil
}

// GetKeyTypeAndHash returns the type of the key by splitting it up and deciphering it
func GetKeyTypeAndHash(key string) (Type, []byte, error) {
	splitString := strings.Split(key, atSep)

	if len(splitString) < 2 {
		return Unknown, nil, update.ErrUnknownType
	}

	switch splitString[0] {
	case "meta":
		return getHeaderTypeAndHash(splitString)
	case "tx":
		return getTransactionKeyTypeAndHash(splitString[1:])
	case "tr":
		return getTrieTypeAndHash(splitString[1:])
	case "rt":
		return RootHash, []byte(key), nil
	}

	return Unknown, nil, update.ErrUnknownType
}

func getHeaderTypeAndHash(splitString []string) (Type, []byte, error) {
	if len(splitString) < 3 {
		return Unknown, nil, update.ErrUnknownType
	}

	hash, err := hex.DecodeString(splitString[2])
	if err != nil {
		return Unknown, nil, err
	}

	return Header, hash, nil
}

// CreateVersionKey creates a version key from the given metaBlock
func CreateVersionKey(meta *block.Block, hash []byte) string {
	return "meta" + atSep + string(meta.Header.ChainID) + atSep + hex.EncodeToString(hash)
}

// CreateAccountKey creates a key for an account according to its type, shard ID and address
func CreateAccountKey(accType Type, address []byte) string {
	key := CreateTrieIdentifier(accType)
	return key + atSep + hex.EncodeToString(address)
}

// CreateRootHashKey creates a key of type roothash for a given trie identifier
func CreateRootHashKey(trieIdentifier string) string {
	return "rt" + atSep + hex.EncodeToString([]byte(trieIdentifier))
}

// CreateTrieIdentifier creates a trie identifier according to trie type and shard id
func CreateTrieIdentifier(accountType Type) string {
	return fmt.Sprint("tr", atSep, atSep, accountType)
}

// AddRootHashToIdentifier adds the roothash to the current identifier
func AddRootHashToIdentifier(identifier string, hash string) string {
	return identifier + atSep + hex.EncodeToString([]byte(hash))
}

// CreateTransactionKey create a transaction key according to its type
func CreateTransactionKey(key string, tx data.TransactionHandler) string {
	switch tx.(type) {
	case *transaction.Transaction:
		return "tx" + atSep + "nrm" + atSep + hex.EncodeToString([]byte(key))
	default:
		return "tx" + atSep + "ukw" + atSep + hex.EncodeToString([]byte(key))
	}
}
