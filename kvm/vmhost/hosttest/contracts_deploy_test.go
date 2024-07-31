package hostCoretest

import (
	"math/big"
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
)

var sc1Address = testcommon.MakeTestSCAddress("sc1")
var sc2Address = testcommon.MakeTestSCAddress("sc2")

func getDeployFromSourceTestConfig() testcommon.TestConfig {
	return test.TestConfig{
		DeployedContractAddress: sc1Address,
		GasUsedByInit:           uint64(200),
		GasUsedByChild:          uint64(200),
		GasProvidedForInit:      uint64(300),
		GasProvided:             uint64(1000),
		AoTPreparePerByteCost:   uint64(1),
		CompilePerByteCost:      uint64(2),
	}
}

func getUpdateFromSourceTestConfig() testcommon.TestConfig {
	config := getDeployFromSourceTestConfig()
	config.ContractToBeUpdatedAddress = sc2Address
	config.Owner = test.ParentAddress
	config.IsFlagEnabled = true
	return config
}

func TestDeployFromSource_Success(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	deployedCode := testConfig.DeployedContractAddress /* this is the actual mock code of the deployed contract */
	deployedCodeLen := uint64(len(deployedCode))
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		newContractAddress := verify.VmOutput.ReturnData[0]
		verify.
			Ok().
			Code(newContractAddress, deployedCode).
			GasRemaining(testConfig.GasProvided -
				testConfig.GasUsedByInit -
				deployedCodeLen*testConfig.CompilePerByteCost -
				deployedCodeLen*testConfig.AoTPreparePerByteCost)
	})
}

func TestDeployFromSource_NoGasForInit(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	testConfig.GasProvidedForInit = uint64(10)

	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas()
	})
}

func TestDeployFromSource_NoGasForAoTPrepare(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	testConfig.AoTPreparePerByteCost = uint64(10)
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas()
	})
}

func TestDeployFromSource_NoGasForCompile(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	testConfig.CompilePerByteCost = uint64(100)
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas()
	})
}

func TestDeployFromSource_NoContract(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	testConfig.DeployedContractAddress = nil
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			ExecutionFailed().
			HasRuntimeErrors(vmhost.ErrContractInvalid.Error())
	})
}

func runDeployFromSourceTest(t *testing.T, testConfig *testcommon.TestConfig, asserts func(world *worldmock.MockWorld, verify *test.VMOutputVerifier)) {
	var deployedContract test.MockTestSmartContract
	if testConfig.DeployedContractAddress != nil {
		deployedContract = test.CreateMockContract(testConfig.DeployedContractAddress).
			WithConfig(testConfig).
			WithMethods(contracts.InitMockMethod)
	}
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			deployedContract,
			test.CreateMockContract(test.ParentAddress).
				WithConfig(testConfig).
				WithMethods(contracts.DeployContractFromSourceMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("deployContractFromSource").
			WithArguments(testConfig.DeployedContractAddress, []byte{0, 0}, big.NewInt(int64(testConfig.GasProvidedForInit)).Bytes()).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
			host.Metering().GasSchedule().BaseOperationCost.AoTPreparePerByte = testConfig.AoTPreparePerByteCost
			host.Metering().GasSchedule().BaseOperationCost.CompilePerByte = testConfig.CompilePerByteCost
		}).
		AndAssertResults(asserts)
	assert.Nil(t, err)
}

func TestUpdateFromSource_Success_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	updatedCode := testConfig.DeployedContractAddress
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Ok().
			Code(testConfig.ContractToBeUpdatedAddress, updatedCode)
	})
}

func TestUpdateFromSource_CallbackFails_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	updatedCode := testConfig.DeployedContractAddress
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Code(testConfig.ContractToBeUpdatedAddress, updatedCode)
	})
}

func TestUpdateFromSource_Success_NoEpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.IsFlagEnabled = false
	updatedCode := testConfig.DeployedContractAddress /* this is the actual mock code of the deployed contract */
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Ok().
			Code(testConfig.ContractToBeUpdatedAddress, updatedCode)
	})
}

func TestUpdateFromSource_NoPermission_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.Owner = nil
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			ExecutionFailed().
			HasRuntimeErrors(vmhost.ErrUpgradeNotAllowed.Error())
	})
}

func TestUpdateFromSource_NoPermission_NoEpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.IsFlagEnabled = false
	testConfig.Owner = nil
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			ExecutionFailed().
			HasRuntimeErrors(vmhost.ErrUpgradeNotAllowed.Error())
		//Code(testConfig.ContractToBeUpdatedAddress, nil)
	})
}

func TestUpdateFromSource_NoGasForInit_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.GasUsedByInit = 1000
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
	})
}

func TestUpdateFromSource_NoGasForInit_NoEpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.IsFlagEnabled = false
	testConfig.GasUsedByInit = 1000
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
	})
}

func TestUpdateFromSource_NoGasForCompile_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.CompilePerByteCost = uint64(50)
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
	})
}

func TestUpdateFromSource_NoGasForCompile_NoEpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.IsFlagEnabled = false
	testConfig.CompilePerByteCost = uint64(50)
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
	})
}

func runUpdateFromSourceTest(t *testing.T, testConfig *testcommon.TestConfig, asserts func(world *worldmock.MockWorld, verify *test.VMOutputVerifier)) {
	var deployedContract test.MockTestSmartContract
	var contractToUpdate test.MockTestSmartContract
	if testConfig.DeployedContractAddress != nil {
		// The DeployedContract will be the source of the code to be written in
		// ContractToBeUpdated. Therefore it is the DeployedContract which must
		// initially have UpgradeMockMethod. After the code update, the
		// ContractToBeUpdated will also have the UpgradeMockMethod and it will be
		// called by the VM.
		deployedContract = test.CreateMockContract(testConfig.DeployedContractAddress).
			WithConfig(testConfig).
			WithMethods(contracts.InitMockMethod, contracts.UpgradeMockMethod)
	}
	if testConfig.ContractToBeUpdatedAddress != nil {
		contractToUpdate = test.CreateMockContract(testConfig.ContractToBeUpdatedAddress).
			WithConfig(testConfig).
			WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
			WithOwnerAddress(testConfig.Owner).
			WithMethods(contracts.InitMockMethod)
	}

	methods := []func(*mock.InstanceMock, interface{}){contracts.UpdateContractFromSourceMock}

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			deployedContract,
			contractToUpdate,
			test.CreateMockContract(test.ParentAddress).
				WithConfig(testConfig).
				WithMethods(methods...)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("updateContractFromSource").
			WithArguments(testConfig.DeployedContractAddress, testConfig.ContractToBeUpdatedAddress,
				nil,
				big.NewInt(int64(testConfig.GasProvidedForInit)).Bytes()).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
			gasSchedule := host.Metering().GasSchedule()
			gasSchedule.BaseOperationCost.AoTPreparePerByte = testConfig.AoTPreparePerByteCost
			gasSchedule.BaseOperationCost.CompilePerByte = testConfig.CompilePerByteCost
		}).
		AndAssertResults(asserts)
	assert.Nil(t, err)
}
