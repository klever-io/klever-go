package hostCoretest

import (
	"math/big"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sc1Address = testcommon.MakeTestSCAddress("sc1")
var sc2Address = testcommon.MakeTestSCAddress("sc2")
var nestedDeploySourceAddress = testcommon.MakeTestSCAddress("nestedDeploySource")

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

func preForkAuditV4EnableEpochs() blockchainConfig.EnableEpochs {
	return blockchainConfig.EnableEpochs{
		ClaimKFI:                0,
		ProcessorFlowITOPrice:   0,
		FixStakingBuckets:       0,
		KdaFpr:                  0,
		BigBucketsCompute:       0,
		FPRComputeAndKdaFeeFlow: 0,
		FixDelegationSameEpoch:  0,
		SmartContracts:          0,
		FixAuditChanges:         0,
		EpochRewardsV2:          0,
		FixAuditChangesV2:       0,
		FixMarketBuyOverflow:    0,
		FixAuditChangesV3:       0,
		FixAuditChangesV4:       1_000_000,
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
	deployedCode := mock.MockContractCode(testConfig.DeployedContractAddress) /* this is the actual mock code of the deployed contract */
	deployedCodeLen := uint64(len(deployedCode))
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		newContractAddress := verify.VmOutput.ReturnData[0]
		verify.
			Ok().
			Code(newContractAddress, deployedCode).
			GasRemaining(testConfig.GasProvided -
				testConfig.GasUsedByInit -
				deployedCodeLen*testConfig.CompilePerByteCost -
				deployedCodeLen*testConfig.AoTPreparePerByteCost -
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

func TestDeployFromSource_CodeDeclaringStartSection(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	runDeployFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			ExecutionFailed().
			HasRuntimeErrors(vmhost.ErrContractHasStartSection.Error())
	}, func(_ vmhost.VMHost, world *worldmock.MockWorld) {
		account := world.GetAccount(testConfig.DeployedContractAddress)
		account.SetCodeAndMetadata(mock.WasmCodeWithStartSection(), &vmcommon.CodeMetadata{Payable: true})
		world.PutAccount(account)
	})
}

func TestDeployFromSource_CodeDeclaringStartSection_PreFork(t *testing.T) {
	testConfig := getDeployFromSourceTestConfig()
	epochs := preForkAuditV4EnableEpochs()
	runDeployFromSourceTestWithEpochs(t, &testConfig, &epochs, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Ok().
			Code(verify.VmOutput.ReturnData[0], mock.WasmCodeWithStartSection())
	}, func(_ vmhost.VMHost, world *worldmock.MockWorld) {
		account := world.GetAccount(testConfig.DeployedContractAddress)
		account.SetCodeAndMetadata(mock.WasmCodeWithStartSection(), &vmcommon.CodeMetadata{Payable: true})
		world.PutAccount(account)
	})
}

func runDeployFromSourceTest(
	t *testing.T,
	testConfig *testcommon.TestConfig,
	asserts func(world *worldmock.MockWorld, verify *test.VMOutputVerifier),
	extraSetup ...func(host vmhost.VMHost, world *worldmock.MockWorld),
) {
	runDeployFromSourceTestWithEpochs(t, testConfig, nil, asserts, extraSetup...)
}

func runDeployFromSourceTestWithEpochs(
	t *testing.T,
	testConfig *testcommon.TestConfig,
	epochs *blockchainConfig.EnableEpochs,
	asserts func(world *worldmock.MockWorld, verify *test.VMOutputVerifier),
	extraSetup ...func(host vmhost.VMHost, world *worldmock.MockWorld),
) {
	var deployedContract test.MockTestSmartContract
	if testConfig.DeployedContractAddress != nil {
		deployedContract = test.CreateMockContract(testConfig.DeployedContractAddress).
			WithConfig(testConfig).
			WithMethods(contracts.InitMockMethod)
	}
	callTest := test.BuildMockInstanceCallTest(t)
	if epochs != nil {
		callTest = callTest.WithEnableEpochs(*epochs)
	}
	_, err := callTest.
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
			for _, setup := range extraSetup {
				setup(host, world)
			}
		}).
		AndAssertResults(asserts)
	assert.Nil(t, err)
}

func TestUpdateFromSource_Success_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	updatedCode := mock.MockContractCode(testConfig.DeployedContractAddress)
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Ok().
			Code(testConfig.ContractToBeUpdatedAddress, updatedCode)
	})
}

func TestUpdateFromSource_CallbackFails_EpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	updatedCode := mock.MockContractCode(testConfig.DeployedContractAddress)
	runUpdateFromSourceTest(t, &testConfig, func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			Code(testConfig.ContractToBeUpdatedAddress, updatedCode)
	})
}

func TestUpdateFromSource_Success_NoEpochFlag(t *testing.T) {
	testConfig := getUpdateFromSourceTestConfig()
	testConfig.IsFlagEnabled = false
	updatedCode := mock.MockContractCode(testConfig.DeployedContractAddress) /* this is the actual mock code of the deployed contract */
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

type nestedDeployResult struct {
	outerAddress []byte
	outerErr     error
	innerAddress []byte
	innerErr     error
	initCalls    int
	vmOutput     *vmcommon.VMOutput
}

func runNestedDeployTest(t *testing.T, fixAuditChangesV4 uint32, propagateFault bool, asserts func(world *worldmock.MockWorld, verify *test.VMOutputVerifier)) nestedDeployResult {
	result := nestedDeployResult{}

	deployFromSource := func(host vmhost.VMHost) ([]byte, error) {
		return vmhooks.DeployFromSourceContractWithTypedArgs(
			host,
			nestedDeploySourceAddress,
			[]byte{0, 0},
			big.NewInt(0),
			[][]byte{},
			100000,
		)
	}

	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithEnableEpochs(blockchainConfig.EnableEpochs{FixAuditChangesV4: fixAuditChangesV4}).
		WithContracts(
			test.CreateMockContract(nestedDeploySourceAddress).
				WithMethods(func(sourceInstance *mock.InstanceMock, _ any) {
					sourceInstance.AddMockMethod("init", func() *mock.InstanceMock {
						result.initCalls++
						if result.initCalls == 1 {
							vmhooks.ExecuteOnDestContextWithTypedArgs(
								sourceInstance.Host,
								100000,
								big.NewInt(0),
								[]byte("deployAgain"),
								test.ParentAddress,
								[][]byte{},
							)
						}
						return sourceInstance
					})
				}),
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("deployOnce", func() *mock.InstanceMock {
						result.outerAddress, result.outerErr = deployFromSource(parentInstance.Host)
						if propagateFault && result.outerErr != nil {
							parentInstance.Host.Runtime().FailExecution(result.outerErr)
						}
						return parentInstance
					})
					parentInstance.AddMockMethod("deployAgain", func() *mock.InstanceMock {
						result.innerAddress, result.innerErr = deployFromSource(parentInstance.Host)
						if propagateFault && result.innerErr != nil {
							parentInstance.Host.Runtime().FailExecution(result.innerErr)
						}
						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000000).
			WithFunction("deployOnce").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(asserts)

	assert.Nil(t, err)

	result.vmOutput = vmOutput

	return result
}

func TestDeployFromSource_NestedDeployRejectsDuplicateAddressWithinTransaction(t *testing.T) {
	result := runNestedDeployTest(t, 0, false, func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.Ok()
	})

	require.Equal(t, 1, result.initCalls)
	require.NoError(t, result.outerErr)
	require.Equal(t, vmhost.ErrDeploymentOverExistingAccount, result.innerErr)
	require.Nil(t, result.innerAddress)

	requireDeployedFromNestedDeploySource(t, result)
}

func TestDeployFromSource_NestedDeployReusesAddressBeforeAuditV4(t *testing.T) {
	result := runNestedDeployTest(t, 1_000_000, false, func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.Ok()
	})

	require.Equal(t, 2, result.initCalls)
	require.NoError(t, result.outerErr)
	require.NoError(t, result.innerErr)
	require.Equal(t, result.outerAddress, result.innerAddress)

	requireDeployedFromNestedDeploySource(t, result)
}

func TestDeployFromSource_NestedDeployWithDuplicateAddressAbortsTransaction(t *testing.T) {
	result := runNestedDeployTest(t, 0, true, func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
		verify.
			ExecutionFailed().
			HasRuntimeErrors(vmhost.ErrDeploymentOverExistingAccount.Error())
	})

	require.Equal(t, 1, result.initCalls)
	require.Empty(t, result.vmOutput.OutputAccounts)
}

func requireDeployedFromNestedDeploySource(t *testing.T, result nestedDeployResult) {
	sourceCode := mock.MockContractCode(nestedDeploySourceAddress)

	deployedAccount := result.vmOutput.OutputAccounts[string(result.outerAddress)]
	require.NotNil(t, deployedAccount)
	require.Equal(t, sourceCode, deployedAccount.Code)
	require.Equal(t, []byte{0, 0}, deployedAccount.CodeMetadata)
	require.Equal(t, test.ParentAddress, deployedAccount.CodeDeployerAddress)
}
