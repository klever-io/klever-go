package vmcommon

import (
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/closing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// FunctionNames (alias) is a map of function names
type FunctionNames = map[string]struct{}

// BlockchainHook is the interface for VM blockchain callbacks
type BlockchainHook interface {
	// NewAddress yields the address of a new SC account, when one such account is created.
	// The result should only depend on the creator address and nonce.
	// Returning an empty address lets the VM decide what the new address should be.
	NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte) ([]byte, error)

	// GetStorageData should yield the storage value for a certain account and index.
	// Should return an empty byte array if the key is missing from the account storage,
	// or if account does not exist.
	GetStorageData(accountAddress []byte, index []byte) ([]byte, uint32, error)

	// GetBlockHash returns the hash of the block with the asked nonce if available
	GetBlockHash(nonce uint64) ([]byte, error)

	// LastNonce returns the nonce from from the last committed block
	LastNonce() uint64

	// LastSlot returns the round from the last committed block
	LastSlot() uint64

	// LastTimeStamp returns the timeStamp from the last committed block
	LastTimeStamp() int64

	// LastRandomSeed returns the random seed from the last committed block
	LastRandomSeed() []byte

	// LastEpoch returns the epoch from the last committed block
	LastEpoch() uint32

	// GetStateRootHash returns the state root hash from the last committed block
	GetStateRootHash() []byte

	// CurrentNonce returns the nonce from the current block
	CurrentNonce() uint64

	// CurrentSlot returns the round from the current block
	CurrentSlot() uint64

	// CurrentTimeStamp return the timestamp from the current block
	CurrentTimeStamp() int64

	// CurrentRandomSeed returns the random seed from the current header
	CurrentRandomSeed() []byte

	// CurrentEpoch returns the current epoch
	CurrentEpoch() uint32

	// ProcessBuiltInFunction will process the builtIn function for the created input
	ProcessBuiltInFunction(input *ContractCallInput) (*VMOutput, error)

	// GetBuiltinFunctionNames returns the names of protocol built-in functions
	GetBuiltinFunctionNames() FunctionNames

	// GetAllState returns the full state of the account, all the key-value saved
	GetAllState(address []byte) (map[string][]byte, error)

	// GetUserAccount returns a user account
	GetUserAccount(address []byte) (state.UserAccountHandler, error)

	// GetCode returns the code for the given account
	GetCode(state.UserAccountHandler) []byte

	// IsSmartContract returns whether the address points to a smart contract
	IsSmartContract(address []byte) bool

	// IsPayable checks weather the provided address can receive ERD or not
	IsPayable(sndAddress []byte, recvAddress []byte) (bool, error)

	// SaveCompiledCode saves to cache and storage the compiled code
	SaveCompiledCode(codeHash []byte, code []byte)

	// GetCompiledCode returns the compiled code if it finds in the cache or storage
	GetCompiledCode(codeHash []byte) (bool, []byte)

	// ClearCompiledCodes clears the cache and storage of compiled codes
	ClearCompiledCodes()

	// GetKDAToken loads the KDA digital token for the given key
	GetKDAToken(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error)

	// GetSFTMeta loads the SFT metadata for the given key
	GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error)

	// GetSnapshot gets the number of entries in the journal as a snapshot id
	GetSnapshot() int

	// RevertToSnapshot reverts snapshots up to the specified one
	RevertToSnapshot(snapshot int) error

	// ExecuteSmartContractCallOnOtherVM executes a smart contract call on another VM
	ExecuteSmartContractCallOnOtherVM(input *ContractCallInput) (*VMOutput, error)

	// TransferValueOnly transfers KLV from one account to another
	TransferValueOnly(destination []byte, sender []byte, value *big.Int) error

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// VMExecutionHandler interface for any Elrond VM endpoint
type VMExecutionHandler interface {
	closing.Closer

	// RunSmartContractCreate computes how a smart contract creation should be performed
	RunSmartContractCreate(input *ContractCreateInput) (*VMOutput, error)

	// RunSmartContractCall computes the result of a smart contract call and how the system must change after the execution
	RunSmartContractCall(input *ContractCallInput) (*VMOutput, error)

	// GasScheduleChange sets a new gas schedule for the VM
	GasScheduleChange(newGasSchedule map[string]map[string]uint64)

	// GetVersion returns the version of the VM instance
	GetVersion() string

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// CryptoHook interface for VM krypto functions
type CryptoHook interface {
	// Sha256 cryptographic function
	Sha256(data []byte) ([]byte, error)

	// Keccak256 cryptographic function
	Keccak256(data []byte) ([]byte, error)

	// Ripemd160 cryptographic function
	Ripemd160(data []byte) ([]byte, error)

	// Ecrecover calculates the corresponding Ethereum address for the public key which created the given signature
	// https://ewasm.readthedocs.io/en/mkdocs/system_contracts/
	Ecrecover(hash []byte, recoveryID []byte, r []byte, s []byte) ([]byte, error)

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// Marshalizer defines the 2 basic operations: serialize (marshal) and deserialize (unmarshal)
type Marshalizer interface {
	Marshal(obj interface{}) ([]byte, error)
	Unmarshal(obj interface{}, buff []byte) error
	IsInterfaceNil() bool
}

// KDARoleHandler provides IsAllowedToExecute function for an KDA
type KDARoleHandler interface {
	CheckAllowedToExecute(account state.UserAccountHandler, tokenID []byte, action []byte) error
	IsInterfaceNil() bool
}

// PayableHandler provides IsPayable function which returns if an account is payable or not
type PayableHandler interface {
	IsPayable(sndAddress, rcvAddress []byte) (bool, error)
	IsInterfaceNil() bool
}

// EpochSubscriberHandler defines the behavior of a component that can be notified if a new epoch was confirmed
type EpochSubscriberHandler interface {
	EpochConfirmed(epoch uint32, timestamp uint64)
	IsInterfaceNil() bool
}

// BuiltinFunction defines the methods for the built-in protocol smart contract functions
type BuiltinFunction interface {
	ProcessBuiltinFunction(vmInput *ContractCallInput) (*VMOutput, error)
	SetNewGasConfig(gasCost *GasCost)
	IsActive() bool
	IsInterfaceNil() bool
}

// BuiltInFunctionContainer defines the methods for the built-in protocol container
type BuiltInFunctionContainer interface {
	Get(key string) (BuiltinFunction, error)
	Add(key string, function BuiltinFunction) error
	Replace(key string, function BuiltinFunction) error
	Remove(key string)
	Len() int
	Keys() map[string]struct{}
	IsInterfaceNil() bool
}

// EpochNotifier can notify upon an epoch change and provide the current epoch
type EpochNotifier interface {
	RegisterNotifyHandler(handler core.EpochSubscriberHandler)
	IsInterfaceNil() bool
}

// KDATransferParser can parse single and multi KDA / NFT transfers
type KDATransferParser interface {
	ParseKDATransfers(rcvAddr []byte, args [][]byte) (*ParsedKDATransfers, error)
	IsInterfaceNil() bool
}

// CallArgsParser will handle parsing transaction data to function and arguments
type CallArgsParser interface {
	ParseData(data string) (string, [][]byte, error)
	ParseArguments(data string) ([][]byte, error)
	IsInterfaceNil() bool
}

// BuiltInFunctionFactory will handle built-in functions and components
type BuiltInFunctionFactory interface {
	BuiltInFunctionContainer() BuiltInFunctionContainer
	SetPayableHandler(handler PayableHandler) error
	CreateBuiltInFunctionContainer() error
	IsInterfaceNil() bool
}

// PayableChecker will handle checking if transfer can happen of KDA tokens towards destination
type PayableChecker interface {
	CheckPayable(vmInput *ContractCallInput, dstAddress []byte, minLenArguments int) error
	IsInterfaceNil() bool
}

// AcceptPayableChecker defines the methods to accept a payable handler through a set function
type AcceptPayableChecker interface {
	SetPayableChecker(payableHandler PayableChecker) error
	IsInterfaceNil() bool
}

// NextOutputTransferIndexProvider interface abstracts a type that manages a transfer index counter
type NextOutputTransferIndexProvider interface {
	NextOutputTransferIndex() uint32
	GetCrtTransferIndex() uint32
	SetCrtTransferIndex(index uint32)
	IsInterfaceNil() bool
}
