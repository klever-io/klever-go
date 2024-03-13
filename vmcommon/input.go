package vmcommon

import (
	"bytes"
	"math/big"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/vm"
)

// VMInput contains the common fields between the 2 types of SC call.
type VMInput struct {
	// CallerAddr is the public key of the wallet initiating the transaction, "from".
	CallerAddr []byte

	// Arguments are the call parameters to the smart contract function call
	// For contract creation, these are the parameters to the @init function.
	// For contract call, these are the parameters to the function referenced in ContractCallInput.Function.
	// If the number of arguments does not match the function arity,
	// the transaction will return FunctionWrongSignature ReturnCode.
	Arguments [][]byte

	// CallType is the type of SmartContract call
	// Based on this value, the VM is informed of whether the call is direct,
	// asynchronous, or asynchronous callback.
	CallType vm.CallType

	// GasProvided is the maximum gas allowed for the smart contract execution.
	// If the transaction consumes more gas than this value, it will immediately terminate
	// and return OutOfGas ReturnCode.
	// The sender will not be charged based on GasProvided, only on the gas burned,
	// so it doesn't cost the sender more to have a higher gas limit.
	GasProvided uint64

	// OriginalTxHash
	OriginalTxHash []byte

	// CurrentTxHash
	CurrentTxHash []byte

	// PrevTxHash
	PrevTxHash []byte

	// KDATransfers is the map of KDA and amount of tokens transferred by the transaction.
	// Before reaching the VM this value is subtracted from sender balance (CallerAddr)
	// and to added to the smart contract balance.
	// It is often, but not always zero in SC calls.
	KDATransfers []*KDATransfer

	// ReturnCallAfterError
	ReturnCallAfterError bool

	// OriginalCallerAddr is the public key of the wallet originally initiating the transaction
	OriginalCallerAddr []byte
}

func checkKLVName(id []byte) []byte {
	if len(id) == 0 {
		return kdautils.KLVIdentifier
	}
	return id
}

// GetKDACallValue Get KDA Call Value by Token Name if any or zero if not
// Nil or empty id will return KLV value
func (input *VMInput) GetKDACallValue(id []byte) *big.Int {
	// check KLV identifier if nil or empty
	id = checkKLVName(id)

	value := big.NewInt(0)
	for _, transfer := range input.KDATransfers {
		if bytes.Equal(checkKLVName(transfer.KDATokenName), id) {
			value = value.Add(value, transfer.KDAValue)
		}
	}

	return value
}

// KDATransfer defines the structure for and KDA / NFT transfer
type KDATransfer struct {
	// KDAValue is the value (amount of tokens) transferred by the transaction.
	// Before reaching the VM this value is subtracted from sender balance (CallerAddr)
	// and to added to the smart contract balance.
	// It is often, but not always zero in SC calls.
	KDAValue *big.Int

	// KDATokenName is the name of the token which was transferred by the transaction to the SC
	KDATokenName []byte

	// KDATokenType is the type of the transferred token
	KDATokenType uint32

	// KDATokenNonce is the nonce for the given NFT token
	KDATokenNonce uint64

	// executed is a flag to indicate if the transfer was executed
	executed bool

	// KDARoyalties is the royalties for the KDA transfer percentage (used only with TX from user)
	KDARoyalties int64
	// KLVRoyalties is the royalties for the KDA transfer fixed (used only with TX from user)
	KLVRoyalties int64
}

func (kdaTransfer *KDATransfer) SetExecuted() *KDATransfer {
	kdaTransfer.executed = true
	return kdaTransfer
}

func (kdaTransfer *KDATransfer) IsExecuted() bool {
	return kdaTransfer.executed
}

func (kdaTransfer *KDATransfer) Clone() KDATransfer {
	transfer := KDATransfer{
		KDAValue:      big.NewInt(0),
		KDATokenName:  make([]byte, len(kdaTransfer.KDATokenName)),
		KDATokenType:  kdaTransfer.KDATokenType,
		KDATokenNonce: kdaTransfer.KDATokenNonce,
		executed:      kdaTransfer.executed,
		KDARoyalties:  kdaTransfer.KDARoyalties,
		KLVRoyalties:  kdaTransfer.KLVRoyalties,
	}

	copy(transfer.KDATokenName, kdaTransfer.KDATokenName)

	if kdaTransfer.KDAValue != nil {
		transfer.KDAValue.Set(kdaTransfer.KDAValue)
	}

	return transfer
}

// ContractCreateInput VM input when creating a new contract.
// Here we have no RecipientAddr because
// the address (PK) of the created account will be provided by the vmcommon.
// We also do not need to specify a Function field,
// because on creation `init` is always called.
type ContractCreateInput struct {
	VMInput

	// ContractCode is the code of the contract being created, assembled into a byte array.
	ContractCode []byte

	// ContractCodeMetadata is the code metadata of the contract being created.
	ContractCodeMetadata []byte
}

// ParsedKDATransfers defines the struct for the parsed kda transfers
type ParsedKDATransfers struct {
	KDATransfers []*KDATransfer
	RcvAddr      []byte
	// used to pass the call function and args to the VM after a built-in function call
	CallFunction string
	CallArgs     [][]byte
}

type ArgsIterator struct {
	Arguments [][]byte
	position  int
}

type Argument []byte

// ContractCallInput VM input when calling a function from an existing contract
type ContractCallInput struct {
	VMInput

	// RecipientAddr is the smart contract public key, "to".
	RecipientAddr []byte

	// Function is the name of the smart contract function that will be called.
	// The function must be public
	Function string

	// AllowInitFunction specifies whether calling the initialization method of
	// the smart contract is allowed or not
	AllowInitFunction bool

	//argsIterator handle the arguments iteration
	argsIterator *ArgsIterator
}

// Initialize iterator when needed
func (ccInput *ContractCallInput) initIterator() {
	if ccInput.argsIterator == nil {
		ccInput.argsIterator = &ArgsIterator{
			Arguments: ccInput.Arguments,
		}
	}
}

// NextArg returns the next argument from the iterator, Argument is an alias for []byte
func (ccInput *ContractCallInput) NextArg() Argument {
	ccInput.initIterator()
	return ccInput.argsIterator.NextArg()
}

func (ai *ArgsIterator) NextArg() Argument {
	if ai.position < len(ai.Arguments) {
		arg := ai.Arguments[ai.position]
		ai.position++
		return Argument(arg)
	}
	return Argument{}
}

func (arg Argument) String() string {
	if arg == nil {
		return ""
	}
	return string(arg)
}

func (arg Argument) Uint64() uint64 {
	if arg == nil {
		return 0
	}
	return big.NewInt(0).SetBytes(arg).Uint64()
}

func (arg Argument) Uint32() uint32 {
	if arg == nil {
		return 0
	}
	return uint32(big.NewInt(0).SetBytes(arg).Uint64())
}

func (arg Argument) Int64() int64 {
	if arg == nil {
		return 0
	}
	return big.NewInt(0).SetBytes(arg).Int64()
}

func (arg Argument) Int32() int32 {
	if arg == nil {
		return 0
	}
	return int32(big.NewInt(0).SetBytes(arg).Int64())
}

func (arg Argument) Bool() bool {
	if len(arg) == 0 || arg[0] == 0 {
		return false
	}

	return true
}
