package vmhost

import (
	"errors"
	"math/big"
	"strings"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/vmcommon"
)

// DefaultVMType is an exposed value to use in tests
var DefaultVMType = []byte{0xF, 0xF}

// ErrAccountNotFound is an exposed value to use in tests
var ErrAccountNotFound = errors.New("account not found")

// UserAddress is an exposed value to use in tests
var UserAddress = []byte("userAccount.....................")

// AddressSize is the size of an account address, in bytes.
const AddressSize = 32

// SCAddressPrefix is the prefix of any smart contract address used for testing.
var SCAddressPrefix = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0f")

// ParentAddress is an exposed value to use in tests
var ParentAddress = MakeTestSCAddress("parentSC")

// MakeTestSCAddress generates a new smart contract address to be used for
// testing based on the given identifier.
func MakeTestSCAddress(identifier string) []byte {
	return makeTestAddress(SCAddressPrefix, identifier)
}

func makeTestAddress(_ []byte, identifier string) []byte {
	numberOfTrailingDots := AddressSize - len(SCAddressPrefix) - len(identifier)
	leftBytes := SCAddressPrefix
	rightBytes := []byte(identifier + strings.Repeat(".", numberOfTrailingDots))
	return append(leftBytes, rightBytes...)
}

// MakeEmptyContractCallInput instantiates an empty ContractCallInput
func MakeEmptyContractCallInput() *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:           nil,
			Arguments:            make([][]byte, 0),
			CallType:             vm.DirectCall,
			GasProvided:          0,
			ReturnCallAfterError: false,
			KDATransfers:         make([]*vmcommon.KDATransfer, 0),
		},
		RecipientAddr: nil,
		Function:      "",
	}
}

// SetCallParties sets the caller and recipient of the given ContractCallInput
func SetCallParties(input *vmcommon.ContractCallInput, caller []byte, recipient []byte) {
	input.OriginalCallerAddr = []byte("address_original_caller")
	input.CallerAddr = caller
	input.RecipientAddr = recipient
}

// AddArgument adds the provided argument to the ContractCallInput
func AddArgument(input *vmcommon.ContractCallInput, argument []byte) {
	if input.Arguments == nil {
		input.Arguments = make([][]byte, 0)
	}
	input.Arguments = append(input.Arguments, argument)
}

// CopyTxHashes copies the tx hashes from a source ContractCallInput into another
func CopyTxHashes(input *vmcommon.ContractCallInput, sourceInput *vmcommon.ContractCallInput) {
	input.CurrentTxHash = sourceInput.CurrentTxHash
	input.OriginalTxHash = sourceInput.OriginalTxHash
}

// DefaultTestContractCallInput creates a vmcommon.ContractCallInput struct
// with default values.
func DefaultTestContractCallInput() *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:   UserAddress,
			Arguments:    make([][]byte, 0),
			CallType:     vm.DirectCall,
			GasProvided:  0,
			KDATransfers: make([]*vmcommon.KDATransfer, 0),
		},
		RecipientAddr: ParentAddress,
		Function:      "function",
	}
}

// ContractCallInputBuilder extends a ContractCallInput for extra building functionality during testing
type ContractCallInputBuilder struct {
	vmcommon.ContractCallInput
}

// CreateTestContractCallInputBuilder is a builder for ContractCallInputBuilder
func CreateTestContractCallInputBuilder() *ContractCallInputBuilder {
	return &ContractCallInputBuilder{
		ContractCallInput: *DefaultTestContractCallInput(),
	}
}

// WithRecipientAddr provides the recepient address of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithRecipientAddr(address []byte) *ContractCallInputBuilder {
	contractInput.RecipientAddr = address
	return contractInput
}

// WithCallerAddr provides the caller address of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithCallerAddr(address []byte) *ContractCallInputBuilder {
	contractInput.CallerAddr = address
	return contractInput
}

// WithGasProvided provides the gas of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithGasProvided(gas uint64) *ContractCallInputBuilder {
	contractInput.GasProvided = gas
	return contractInput
}

// WithFunction provides the function to be called for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithFunction(function string) *ContractCallInputBuilder {
	contractInput.Function = function
	return contractInput
}

// WithArguments provides the arguments to be called for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithArguments(arguments ...[]byte) *ContractCallInputBuilder {
	contractInput.Arguments = arguments
	return contractInput
}

// WithCurrentTxHash provides the CurrentTxHash for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithCurrentTxHash(txHash []byte) *ContractCallInputBuilder {
	contractInput.CurrentTxHash = txHash
	return contractInput
}

func (contractInput *ContractCallInputBuilder) initKDATransferIfNeeded() {
	if len(contractInput.KDATransfers) == 0 {
		contractInput.KDATransfers = make([]*vmcommon.KDATransfer, 1)
		contractInput.KDATransfers[0] = &vmcommon.KDATransfer{}
	}
}

// WithKDAValue provides the KDAValue for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithKDAValue(kdaValue *big.Int) *ContractCallInputBuilder {
	contractInput.initKDATransferIfNeeded()
	contractInput.KDATransfers[0].KDAValue = kdaValue
	return contractInput
}

// WithKDATokenName provides the KDATokenName for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithKDATokenName(kdaTokenName []byte) *ContractCallInputBuilder {
	contractInput.initKDATransferIfNeeded()
	contractInput.KDATransfers[0].KDATokenName = kdaTokenName
	return contractInput
}

// Build completes the build of a ContractCallInput
func (contractInput *ContractCallInputBuilder) Build() *vmcommon.ContractCallInput {
	return &contractInput.ContractCallInput
}

// ContractCreateInputBuilder extends a ContractCreateInput for extra building functionality during testing
type ContractCreateInputBuilder struct {
	vmcommon.ContractCreateInput
}

// WithGasProvided provides the GasProvided for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithGasProvided(gas uint64) *ContractCreateInputBuilder {
	contractInput.GasProvided = gas
	return contractInput
}

// WithContractCode provides the ContractCode for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithContractCode(code []byte) *ContractCreateInputBuilder {
	contractInput.ContractCode = code
	return contractInput
}

// WithCallerAddr provides the CallerAddr for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithCallerAddr(address []byte) *ContractCreateInputBuilder {
	contractInput.CallerAddr = address
	return contractInput
}

// WithCallValue provides the CallValue for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithCallValue(callValue int64) *ContractCreateInputBuilder {
	contractInput.KDATransfers = append(contractInput.KDATransfers,
		&vmcommon.KDATransfer{
			KDAValue:     big.NewInt(callValue),
			KDATokenName: kdautils.KLVIdentifier,
		})

	return contractInput
}

// WithArguments provides the Arguments for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithArguments(arguments ...[]byte) *ContractCreateInputBuilder {
	contractInput.Arguments = arguments
	return contractInput
}

// Build completes the build of a ContractCreateInput
func (contractInput *ContractCreateInputBuilder) Build() *vmcommon.ContractCreateInput {
	return &contractInput.ContractCreateInput
}
