package testcommon

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/vm"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/vmcommon"
)

var defaultHasher = &blake2b.Blake2b{}

// DefaultVMType is an exposed value to use in tests
var DefaultVMType = []byte{0xF, 0xF}

// ErrAccountNotFound is an exposed value to use in tests
var ErrAccountNotFound = errors.New("account not found")

// UserAddress is an exposed value to use in tests
var UserAddress = MakeTestSCAddressWithDefaultVM("userAccount")

// UserAddress2 is an exposed value to use in tests
var UserAddress2 = []byte("userAccount2....................")

// AddressSize is the size of an account address, in bytes.
const AddressSize = 32

// SCAddressPrefix is the prefix of any smart contract address used for testing.
var SCAddressPrefix = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0f")

// ParentAddress is an exposed value to use in tests
var ParentAddress = MakeTestSCAddressWithDefaultVM("parentSC")

// ChildAddress is an exposed value to use in tests
var ChildAddress = MakeTestSCAddressWithDefaultVM("childSC")

// NephewAddress is an exposed value to use in tests
var NephewAddress = MakeTestSCAddressWithDefaultVM("NephewAddress")

// KDATransferGasCost is an exposed value to use in tests
var KDATransferGasCost = uint64(1)

// KDATestTokenName is an exposed value to use in tests
var KDATestTokenName = []byte("TTT-0101")

// DefaultCodeMetadata is an exposed value to use in tests
var DefaultCodeMetadata = []byte{3, 0}

// MakeTestSCAddress generates a new smart contract address to be used for
// testing based on the given identifier.
func MakeTestSCAddress(identifier string) []byte {
	numberOfTrailingDots := AddressSize - len(SCAddressPrefix) - len(identifier)
	leftBytes := SCAddressPrefix
	rightBytes := []byte(identifier + strings.Repeat(".", numberOfTrailingDots))
	return append(leftBytes, rightBytes...)
}

// MakeTestSCAddressWithDefaultVM generates a new smart contract address to be used for
// testing based on the given identifier.
func MakeTestSCAddressWithDefaultVM(identifier string) []byte {
	return MakeTestSCAddressWithVMType(identifier, worldmock.DefaultVMType)
}

// MakeTestSCAddressWithVMType generates a new smart contract address to be used for
// testing based on the given identifier.
func MakeTestSCAddressWithVMType(identifier string, vmType []byte) []byte {
	address := MakeTestSCAddress(identifier)
	copy(address[vmcommon.NumInitCharactersForScAddress-core.VMTypeLen:], vmType)
	return address
}

// GetSCCode retrieves the bytecode of a WASM module from a file
func GetSCCode(fileName string) []byte {
	code, err := os.ReadFile(filepath.Clean(fileName))
	if err != nil {
		panic(fmt.Sprintf("GetSCCode(): %s", fileName))
	}

	return code
}

// GetTestSCCode retrieves the bytecode of a WASM testing contract
func GetTestSCCode(scName string, prefixToTestSCs ...string) []byte {
	var searchedPaths []string
	for _, prefixToTestSC := range prefixToTestSCs {
		pathToSC := prefixToTestSC + "test/contracts/" + scName + "/output/" + scName + ".wasm"
		searchedPaths = append(searchedPaths, pathToSC)
		code, err := os.ReadFile(filepath.Clean(pathToSC))
		if err == nil {
			return code
		}
	}
	panic(fmt.Sprintf("GetSCCode(): %s", searchedPaths))
}

// GetTestSCCodeModule retrieves the bytecode of a WASM testing contract, given
// a specific name of the WASM module
func GetTestSCCodeModule(scName string, moduleName string, prefixToTestSCs string) []byte {
	pathToSC := prefixToTestSCs + "test/contracts/" + scName + "/output/" + moduleName + ".wasm"
	return GetSCCode(pathToSC)
}

// BlockchainHookStubForCallSigSegv -
func BlockchainHookStubForCallSigSegv(code []byte, balance *big.Int) *contextmock.BlockchainHookStub {
	stubBlockchainHook := &contextmock.BlockchainHookStub{}
	stubBlockchainHook.GetUserAccountCalled = func(scAddress []byte) (state.UserAccountHandler, error) {
		if bytes.Equal(scAddress, ParentAddress) {
			return &worldmock.Account{
				Balance: balance,
			}, nil
		}
		return nil, ErrAccountNotFound
	}
	stubBlockchainHook.GetCodeCalled = func(account state.UserAccountHandler) []byte {
		return code
	}
	return stubBlockchainHook
}

// BlockchainHookStubForCall creates a BlockchainHookStub
func BlockchainHookStubForCall(code []byte, balance *big.Int) *contextmock.BlockchainHookStub {
	stubBlockchainHook := &contextmock.BlockchainHookStub{}
	stubBlockchainHook.GetUserAccountCalled = func(scAddress []byte) (state.UserAccountHandler, error) {
		if bytes.Equal(scAddress, ParentAddress) {
			return &worldmock.Account{
				Balance: balance,
			}, nil
		}
		return nil, ErrAccountNotFound
	}
	stubBlockchainHook.GetCodeCalled = func(account state.UserAccountHandler) []byte {
		return code
	}

	return stubBlockchainHook
}

// BlockchainHookStubForTwoSCs creates a world stub configured for testing calls between 2 SmartContracts
func BlockchainHookStubForTwoSCs(
	parentCode []byte,
	childCode []byte,
	parentSCBalance *big.Int,
	childSCBalance *big.Int,
) *contextmock.BlockchainHookStub {
	stubBlockchainHook := &contextmock.BlockchainHookStub{}

	if parentSCBalance == nil {
		parentSCBalance = big.NewInt(1000)
	}

	if childSCBalance == nil {
		childSCBalance = big.NewInt(1000)
	}

	stubBlockchainHook.GetUserAccountCalled = func(scAddress []byte) (state.UserAccountHandler, error) {
		if bytes.Equal(scAddress, ParentAddress) {
			return &worldmock.Account{
				Address: ParentAddress,
				Balance: parentSCBalance,
			}, nil
		}
		if bytes.Equal(scAddress, ChildAddress) {
			return &worldmock.Account{
				Address: ChildAddress,
				Balance: childSCBalance,
			}, nil
		}

		return nil, ErrAccountNotFound
	}
	stubBlockchainHook.GetCodeCalled = func(account state.UserAccountHandler) []byte {
		if bytes.Equal(account.AddressBytes(), ParentAddress) {
			return parentCode
		}
		if bytes.Equal(account.AddressBytes(), ChildAddress) {
			return childCode
		}
		return nil
	}

	return stubBlockchainHook
}

// BlockchainHookStubForContracts -
func BlockchainHookStubForContracts(
	contracts []*InstanceTestSmartContract,
) *contextmock.BlockchainHookStub {

	stubBlockchainHook := &contextmock.BlockchainHookStub{}

	contractsMap := make(map[string]*worldmock.Account)
	codeMap := make(map[string]*[]byte)

	for _, contract := range contracts {
		codeHash := defaultHasher.Compute(string(contract.code))
		contractsMap[string(contract.address)] = &worldmock.Account{
			Address:      contract.address,
			Balance:      big.NewInt(contract.balance),
			CodeHash:     codeHash,
			CodeMetadata: DefaultCodeMetadata,
			OwnerAddress: contract.ownerAddress,
		}
		codeMap[string(contract.address)] = &contract.code
	}

	stubBlockchainHook.GetUserAccountCalled = func(scAddress []byte) (state.UserAccountHandler, error) {
		contract, found := contractsMap[string(scAddress)]
		if found {
			return contract, nil
		}
		return nil, ErrAccountNotFound
	}
	stubBlockchainHook.GetCodeCalled = func(account state.UserAccountHandler) []byte {
		code, found := codeMap[string(account.AddressBytes())]
		if found {
			return *code
		}
		return nil
	}

	return stubBlockchainHook
}

// AddTestSmartContractToWorld directly deploys the provided code into the
// given MockWorld under a SC address built with the given identifier.
func AddTestSmartContractToWorld(world *worldmock.MockWorld, identifier string, code []byte) *worldmock.Account {
	address := MakeTestSCAddress(identifier)

	acct := world.CreateSmartContractAccount(UserAddress, address, code, world)
	world.PutAccount(acct)

	return acct
}

// DefaultTestContractCreateInput creates a vmcommon.ContractCreateInput struct
// with default values.
func DefaultTestContractCreateInput() *vmcommon.ContractCreateInput {
	return &vmcommon.ContractCreateInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: []byte("caller"),
			Arguments: [][]byte{
				[]byte("argument 1"),
				[]byte("argument 2"),
			},
			KDATransfers: []*vmcommon.KDATransfer{},
			CallType:     vm.DirectCall,
			GasProvided:  0,
		},
		ContractCode: []byte("contract"),
	}
}

// DefaultTestContractCallInput creates a vmcommon.ContractCallInput struct
// with default values.
func DefaultTestContractCallInput() *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			OriginalCallerAddr: UserAddress,
			CallerAddr:         UserAddress,
			Arguments:          make([][]byte, 0),
			KDATransfers:       []*vmcommon.KDATransfer{},
			CallType:           vm.DirectCall,
			GasProvided:        0,
		},
		RecipientAddr: ParentAddress,
		Function:      "function",
	}
}

// ContractCallInputBuilder extends a ContractCallInput for extra building functionality during testing
type ContractCallInputBuilder struct {
	vmcommon.ContractCallInput
	CurrentKDATransferIndex int
}

// CreateTestContractCallInputBuilder is a builder for ContractCallInputBuilder
func CreateTestContractCallInputBuilder() *ContractCallInputBuilder {
	return &ContractCallInputBuilder{
		ContractCallInput:       *DefaultTestContractCallInput(),
		CurrentKDATransferIndex: 0,
	}
}

// WithRecipientAddr provides the recepient address of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithRecipientAddr(address []byte) *ContractCallInputBuilder {
	contractInput.ContractCallInput.RecipientAddr = address
	return contractInput
}

// WithCallerAddr provides the caller address of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithCallerAddr(address []byte) *ContractCallInputBuilder {
	contractInput.ContractCallInput.CallerAddr = address
	return contractInput
}

// WithKDATransfers provides the kda values to the called contract
func (contractInput *ContractCallInputBuilder) WithKDATransfers(values []*vmcommon.KDATransfer) *ContractCallInputBuilder {
	contractInput.ContractCallInput.KDATransfers = values
	return contractInput
}

// WithGasProvided provides the gas of ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithGasProvided(gas uint64) *ContractCallInputBuilder {
	contractInput.ContractCallInput.VMInput.GasProvided = gas
	return contractInput
}

// WithFunction provides the function to be called for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithFunction(function string) *ContractCallInputBuilder {
	contractInput.ContractCallInput.Function = function
	return contractInput
}

// WithArguments provides the arguments to be called for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithArguments(arguments ...[]byte) *ContractCallInputBuilder {
	contractInput.ContractCallInput.VMInput.Arguments = arguments
	return contractInput
}

// WithCurrentTxHash provides the CurrentTxHash for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithCurrentTxHash(txHash []byte) *ContractCallInputBuilder {
	contractInput.ContractCallInput.CurrentTxHash = txHash
	return contractInput
}

// WithPrevTxHash provides the PrevTxHash for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithPrevTxHash(txHash []byte) *ContractCallInputBuilder {
	contractInput.ContractCallInput.PrevTxHash = txHash
	return contractInput
}

func (contractInput *ContractCallInputBuilder) initKDATransferIfNeeded() {
	if len(contractInput.KDATransfers) == 0 {
		contractInput.KDATransfers = make([]*vmcommon.KDATransfer, 1)
		contractInput.KDATransfers[0] = &vmcommon.KDATransfer{}
		contractInput.CurrentKDATransferIndex = 0
	}
}

// WithKDAValue provides the KDAValue for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithKDAValue(kdaValue *big.Int) *ContractCallInputBuilder {
	contractInput.initKDATransferIfNeeded()
	i := contractInput.CurrentKDATransferIndex
	contractInput.ContractCallInput.KDATransfers[i].KDAValue = kdaValue
	return contractInput
}

// WithKDATokenName provides the KDATokenName for ContractCallInputBuilder
func (contractInput *ContractCallInputBuilder) WithKDATokenName(kdaTokenName []byte) *ContractCallInputBuilder {
	contractInput.initKDATransferIfNeeded()
	i := contractInput.CurrentKDATransferIndex
	contractInput.ContractCallInput.KDATransfers[i].KDATokenName = kdaTokenName
	return contractInput
}

func (contractInput *ContractCallInputBuilder) NextKDATransfer() *ContractCallInputBuilder {
	nextTransfer := &vmcommon.KDATransfer{}
	contractInput.KDATransfers = append(contractInput.KDATransfers, nextTransfer)
	contractInput.CurrentKDATransferIndex++
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

// CreateTestContractCreateInputBuilder is a builder for ContractCreateInputBuilder
func CreateTestContractCreateInputBuilder() *ContractCreateInputBuilder {
	return &ContractCreateInputBuilder{
		ContractCreateInput: *DefaultTestContractCreateInput(),
	}
}

// WithGasProvided provides the GasProvided for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithGasProvided(gas uint64) *ContractCreateInputBuilder {
	contractInput.ContractCreateInput.GasProvided = gas
	return contractInput
}

// WithContractCode provides the ContractCode for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithContractCode(code []byte) *ContractCreateInputBuilder {
	contractInput.ContractCreateInput.ContractCode = code
	return contractInput
}

// WithCallerAddr provides the CallerAddr for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithCallerAddr(address []byte) *ContractCreateInputBuilder {
	contractInput.ContractCreateInput.CallerAddr = address
	return contractInput
}

// WithKDATransfers provides the CallValue for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithKDATransfers(values []*vmcommon.KDATransfer) *ContractCreateInputBuilder {
	contractInput.ContractCreateInput.KDATransfers = values
	return contractInput
}

// WithArguments provides the Arguments for a ContractCreateInputBuilder
func (contractInput *ContractCreateInputBuilder) WithArguments(arguments ...[]byte) *ContractCreateInputBuilder {
	contractInput.ContractCreateInput.Arguments = arguments
	return contractInput
}

// Build completes the build of a ContractCreateInput
func (contractInput *ContractCreateInputBuilder) Build() *vmcommon.ContractCreateInput {
	return &contractInput.ContractCreateInput
}
