package testcommon

import (
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

var WasmVirtualMachine = []byte{5, 0}

// TestConfig is configuration for async call tests
type TestConfig struct {
	ChildAddress      []byte
	ThirdPartyAddress []byte
	VaultAddress      []byte

	GasProvided        uint64
	GasProvidedToChild uint64
	GasUsedByParent    uint64
	GasUsedByChild     uint64

	ParentBalance int64
	ChildBalance  int64

	TransferFromParentToChild int64
	TransferToThirdParty      int64
	TransferToVault           int64
	TransferFromChildToParent int64

	KDATokensToTransfer         int64
	CallbackKDATokensToTransfer uint64

	ChildCalls          int
	RecursiveChildCalls int

	DeployedContractAddress []byte
	GasUsedByInit           uint64
	GasProvidedForInit      uint64
	AsyncCallStepCost       uint64
	AoTPreparePerByteCost   uint64
	CompilePerByteCost      uint64

	ContractToBeUpdatedAddress []byte
	Owner                      []byte
	IsFlagEnabled              bool
}

func getAddressOrDefult(address []byte, defaultAddress []byte) []byte {
	if address == nil {
		return defaultAddress
	}
	return address
}

// GetChildAddress -
func (config *TestConfig) GetChildAddress() []byte {
	return getAddressOrDefult(config.ChildAddress, ChildAddress)
}

// GetThirdPartyAddress -
func (config *TestConfig) GetThirdPartyAddress() []byte {
	return getAddressOrDefult(config.ThirdPartyAddress, ThirdPartyAddress)
}

// GetVaultAddress -
func (config *TestConfig) GetVaultAddress() []byte {
	return getAddressOrDefult(config.VaultAddress, VaultAddress)
}

type testSmartContract struct {
	address      []byte
	balance      int64
	config       *TestConfig
	codeHash     []byte
	codeMetadata []byte
	ownerAddress []byte
	vmType       []byte
}

// MockTestSmartContract represents the config data for the mock smart contract instance to be tested
type MockTestSmartContract struct {
	testSmartContract
	initMethods []func(*mock.InstanceMock, interface{})
	// used only temporarly for call graph building
	tempFunctionsList map[string]bool
}

// CreateMockContract build a contract to be used in a test creted with BuildMockInstanceCallTest
func CreateMockContract(address []byte) *MockTestSmartContract {
	return &MockTestSmartContract{
		testSmartContract: testSmartContract{
			address: address,
			vmType:  WasmVirtualMachine,
		},
		tempFunctionsList: make(map[string]bool, 0),
	}
}

// WithBalance provides the balance for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithVMType(vmType []byte) *MockTestSmartContract {
	mockSC.vmType = vmType
	return mockSC
}

// WithBalance provides the balance for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithBalance(balance int64) *MockTestSmartContract {
	mockSC.balance = balance
	return mockSC
}

// WithConfig provides the config object for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithConfig(config *TestConfig) *MockTestSmartContract {
	mockSC.config = config
	return mockSC
}

// WithCodeMetadata provides the code metadata for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithCodeMetadata(codeMetadata []byte) *MockTestSmartContract {
	mockSC.codeMetadata = codeMetadata
	return mockSC
}

// WithCodeHash provides the code hash for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithCodeHash(codeHash []byte) *MockTestSmartContract {
	mockSC.codeHash = codeHash
	return mockSC
}

// WithOwnerAddress provides the owner address for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithOwnerAddress(ownerAddress []byte) *MockTestSmartContract {
	mockSC.ownerAddress = ownerAddress
	return mockSC
}

// WithMethods provides the methods for the MockTestSmartContract
func (mockSC *MockTestSmartContract) WithMethods(initMethods ...func(*mock.InstanceMock, interface{})) MockTestSmartContract {
	mockSC.initMethods = initMethods
	return *mockSC
}

// GetVMType -
func (mockSC *MockTestSmartContract) GetVMType() []byte {
	return mockSC.vmType
}

// Initialize -
func (mockSC *MockTestSmartContract) Initialize(
	t testing.TB,
	host vmhost.VMHost,
	imb *mock.ExecutorMock,
	createContractAccounts bool,
) {
	instance := imb.CreateAndStoreInstanceMock(t, host, mockSC.address, mockSC.codeHash, mockSC.codeMetadata,
		mockSC.ownerAddress, mockSC.balance, createContractAccounts)
	for _, initMethod := range mockSC.initMethods {
		initMethod(instance, mockSC.config)
	}
}
