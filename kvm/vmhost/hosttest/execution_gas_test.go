//nolint:all
package hostCoretest

import (
	"errors"
	"math/big"
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gasUsedByBuiltinClaim = uint64(120)

var LegacyAsyncCallType = []byte{0}
var NewAsyncCallType = []byte{1}

func makeTestConfig() *test.TestConfig {
	return &test.TestConfig{
		GasProvided:        2000,
		GasProvidedToChild: 300,
		GasUsedByParent:    400,
		GasUsedByChild:     200,

		TransferFromParentToChild: 7,

		ParentBalance:        1000,
		ChildBalance:         1000,
		TransferToThirdParty: 3,
		TransferToVault:      4,
		KDATokensToTransfer:  0,
	}
}

func TestGasUsed_SingleContract(t *testing.T) {
	testConfig := makeTestConfig()

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.WasteGasParentMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("wasteGas").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasRemaining(testConfig.GasProvided-testConfig.GasUsedByParent).
				GasUsed(test.ParentAddress, testConfig.GasUsedByParent)
		})
	assert.Nil(t, err)
}

func TestGasUsed_SingleContract_BuiltinCall(t *testing.T) {
	testConfig := makeTestConfig()

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.ExecOnDestCtxParentMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execOnDestCtx").
			WithArguments(test.ParentAddress, []byte("builtinClaim"), vmhost.One.Bytes()).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasRemaining(testConfig.GasProvided-testConfig.GasUsedByParent-gasUsedByBuiltinClaim).
				GasUsed(test.ParentAddress, testConfig.GasUsedByParent+gasUsedByBuiltinClaim)
		})
	assert.Nil(t, err)
}

func TestGasUsed_SingleContract_BuiltinCallFail(t *testing.T) {
	testConfig := makeTestConfig()

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.ExecOnDestCtxSingleCallParentMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execOnDestCtxSingleCall").
			WithArguments(test.ParentAddress, []byte("builtinFail")).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				ReturnMessage("return value 1").
				HasRuntimeErrors("buildMockErr").
				GasRemaining(0)
		})
	assert.Nil(t, err)
}

func TestGasUsed_ExecuteOnDestChain(t *testing.T) {
	alphaAddress := test.MakeTestSCAddress("alpha")
	betaAddress := test.MakeTestSCAddress("beta")
	gammaAddress := test.MakeTestSCAddress("gamma")

	testConfig := &test.TestConfig{
		GasUsedByParent:    uint64(400),
		GasProvidedToChild: uint64(1000),
		GasProvided:        uint64(2000),
		GasUsedByChild:     uint64(200),
	}

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(alphaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.ExecOnDestCtxSingleCallParentMock),
			test.CreateMockContract(betaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.ExecOnDestCtxSingleCallParentMock),
			test.CreateMockContract(gammaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.ReportOriginalCaller),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(alphaAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execOnDestCtxSingleCall").
			WithArguments(betaAddress, []byte("execOnDestCtxSingleCall"),
				gammaAddress, []byte("reportOriginalCaller")).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData(test.UserAddress, test.UserAddress, test.UserAddress)
		})

	assert.Nil(t, err)
}

func TestGasUsed_TwoContracts_ExecuteOnSameCtx(t *testing.T) {
	testConfig := makeTestConfig()

	for numCalls := uint64(0); numCalls < 3; numCalls++ {
		expectedGasRemaining := testConfig.GasProvided - testConfig.GasUsedByParent - testConfig.GasUsedByChild*numCalls
		numCallsBytes := big.NewInt(0).SetUint64(numCalls).Bytes()

		_, err := test.BuildMockInstanceCallTest(t).
			WithContracts(
				test.CreateMockContract(test.ParentAddress).
					WithBalance(testConfig.ParentBalance).
					WithConfig(testConfig).
					WithMethods(contracts.ExecOnSameCtxParentMock, contracts.ExecOnDestCtxParentMock, contracts.WasteGasParentMock),
				test.CreateMockContract(test.ChildAddress).
					WithBalance(testConfig.ChildBalance).
					WithConfig(testConfig).
					WithMethods(contracts.WasteGasChildMock),
			).
			WithInput(test.CreateTestContractCallInputBuilder().
				WithRecipientAddr(test.ParentAddress).
				WithGasProvided(testConfig.GasProvided).
				WithFunction("execOnSameCtx").
				WithArguments(test.ChildAddress, []byte("wasteGas"), numCallsBytes).
				Build()).
			WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
				setZeroCodeCosts(host)
			}).
			AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok().
					GasRemaining(expectedGasRemaining).
					GasUsed(test.ParentAddress, testConfig.GasUsedByParent+testConfig.GasUsedByChild*numCalls)
				if numCalls > 0 {
					verify.GasUsed(test.ChildAddress, 0)
				}
			})
		assert.Nil(t, err)
	}
}

func TestGasUsed_TwoContracts_ExecuteOnDestCtx(t *testing.T) {
	testConfig := makeTestConfig()

	for numCalls := uint64(0); numCalls < 3; numCalls++ {
		expectedGasRemaining := testConfig.GasProvided - testConfig.GasUsedByParent - testConfig.GasUsedByChild*numCalls
		numCallsBytes := big.NewInt(0).SetUint64(numCalls).Bytes()

		_, err := test.BuildMockInstanceCallTest(t).
			WithContracts(
				test.CreateMockContract(test.ParentAddress).
					WithBalance(testConfig.ParentBalance).
					WithConfig(testConfig).
					WithMethods(contracts.ExecOnSameCtxParentMock, contracts.ExecOnDestCtxParentMock, contracts.WasteGasParentMock),
				test.CreateMockContract(test.ChildAddress).
					WithBalance(testConfig.ChildBalance).
					WithConfig(testConfig).
					WithMethods(contracts.WasteGasChildMock),
			).
			WithInput(test.CreateTestContractCallInputBuilder().
				WithRecipientAddr(test.ParentAddress).
				WithGasProvided(testConfig.GasProvided).
				WithFunction("execOnDestCtx").
				WithArguments(test.ChildAddress, []byte("wasteGas"), numCallsBytes).
				Build()).
			WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
				setZeroCodeCosts(host)
			}).
			AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok().
					GasRemaining(expectedGasRemaining).
					GasUsed(test.ParentAddress, testConfig.GasUsedByParent)
				if numCalls > 0 {
					verify.GasUsed(test.ChildAddress, testConfig.GasUsedByChild*numCalls)
				}
			})
		assert.Nil(t, err)
	}
}

func TestGasUsed_ThreeContracts_ExecuteOnDestCtx(t *testing.T) {
	alphaAddress := test.MakeTestSCAddress("alpha")
	betaAddress := test.MakeTestSCAddress("beta")
	gammaAddress := test.MakeTestSCAddress("gamma")

	testConfig := &test.TestConfig{
		GasUsedByParent:    uint64(400),
		GasProvidedToChild: uint64(300),
		GasProvided:        uint64(1000),
		GasUsedByChild:     uint64(200),
	}

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(alphaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.ExecOnSameCtxParentMock, contracts.ExecOnDestCtxParentMock, contracts.WasteGasParentMock),
			test.CreateMockContract(betaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.WasteGasChildMock),
			test.CreateMockContract(gammaAddress).
				WithBalance(0).
				WithConfig(testConfig).
				WithMethods(contracts.WasteGasChildMock),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(alphaAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execOnDestCtx").
			WithArguments(betaAddress, []byte("wasteGas"), vmhost.One.Bytes(),
				gammaAddress, []byte("wasteGas"), vmhost.One.Bytes()).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasUsed(alphaAddress, testConfig.GasUsedByParent).
				GasUsed(betaAddress, testConfig.GasUsedByChild).
				GasUsed(gammaAddress, testConfig.GasUsedByChild).
				GasRemaining(testConfig.GasProvided - testConfig.GasUsedByParent - 2*testConfig.GasUsedByChild)
		})
	assert.Nil(t, err)
}

func TestGasUsed_KDATransfer_ThenExecuteCall_Success(t *testing.T) {
	initialKDATokenBalance := int64(100)
	kdaTransferGasCost := uint64(1)

	vmhost.SetLoggingForTestsWithLogger("VMOutputVerifier")

	testConfig := makeTestConfig()
	testConfig.KDATokensToTransfer = 5

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.ExecKDATransferAndCallChild),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(contracts.WasteGasChildMock),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execKDATransferAndCall").
			WithArguments(test.ChildAddress, []byte("KleverTransfer"), []byte("wasteGas")).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			// AddBalance
			acc, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			_ = acc.AddToBalance(initialKDATokenBalance, test.KDATestTokenName, true)

			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
			host.Metering().GasSchedule().BuiltInCost.Transfer = kdaTransferGasCost

		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasUsed(test.ParentAddress, testConfig.GasUsedByParent+kdaTransferGasCost).
				GasUsed(test.ChildAddress, testConfig.GasUsedByChild).
				GasRemaining(testConfig.GasProvided - testConfig.GasUsedByParent - kdaTransferGasCost - testConfig.GasUsedByChild)

			user, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			userKda, _ := user.GetUserKDA(test.KDATestTokenName, nil, true)
			require.Equal(t, initialKDATokenBalance-testConfig.KDATokensToTransfer, userKda.Balance)

			child, _ := world.AccountsCacher.LoadUser(test.ChildAddress)
			childUserKda, _ := child.GetUserKDA(test.KDATestTokenName, nil, true)
			require.Equal(t, testConfig.KDATokensToTransfer, childUserKda.Balance)
		})
	assert.Nil(t, err)
}

func TestGasUsed_KDATransfer_ThenExecuteCall_Fail(t *testing.T) {
	initialKDATokenBalance := uint64(100)

	testConfig := makeTestConfig()
	testConfig.KDATokensToTransfer = 5

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.ExecKDATransferAndCallChild),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(contracts.FailChildMock),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execKDATransferAndCall").
			WithArguments(test.ChildAddress, []byte("KleverTransfer"), []byte("fail")).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			// AddBalance
			acc, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			_ = acc.AddToBalance(int64(initialKDATokenBalance), test.KDATestTokenName, true)

			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				HasRuntimeErrors("forced fail").
				GasRemaining(0)

			user, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			require.Equal(t, initialKDATokenBalance, uint64(user.GetBalance(test.KDATestTokenName, true)))

			child, _ := world.AccountsCacher.LoadUser(test.ChildAddress)
			require.Equal(t, uint64(0), uint64(child.GetBalance(test.KDATestTokenName, true)))
		})
	assert.Nil(t, err)
}

func TestGasUsed_KDATransferFailed(t *testing.T) {
	initialKDATokenBalance := int64(100)

	testConfig := makeTestConfig()
	testConfig.KDATokensToTransfer = 2 * initialKDATokenBalance

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.ExecKDATransferAndCallChild),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(contracts.FailChildMock),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execKDATransferAndCall").
			WithArguments(test.ChildAddress, []byte("KleverTransfer"), []byte("fail")).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			// AddBalance
			acc, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			_ = acc.AddToBalance(int64(initialKDATokenBalance), test.KDATestTokenName, true)

			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				HasRuntimeErrors("insufficient funds").
				GasRemaining(0)

			user, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			userKda, _ := user.GetUserKDA(test.KDATestTokenName, nil, true)
			require.Equal(t, initialKDATokenBalance, userKda.Balance)

			child, _ := world.AccountsCacher.LoadUser(test.ChildAddress)
			childUserKda, _ := child.GetUserKDA(test.KDATestTokenName, nil, true)
			require.Equal(t, int64(0), childUserKda.Balance)
		})
	assert.Nil(t, err)
}

func TestGasUsed_BuiltinMultiContractChainCall(t *testing.T) {
	// FIXME matei-p enable this test for R2
	t.Skip()

	testConfig := makeTestConfig()
	testConfig.TransferFromChildToParent = 5

	expectedGasUsedByParent := testConfig.GasUsedByParent
	expectedGasUsedByChild :=
		testConfig.GasUsedByParent /* due to ForwardAsyncCallParentBuiltinMock */ +
			gasUsedByBuiltinClaim

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("forwardAsyncCall").
			WithArguments(test.ChildAddress, []byte("forwardAsyncCall"), []byte("builtinClaim"), vmhost.One.Bytes()).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			world.CreateAccount(test.UserAddress, world)
			setZeroCodeCosts(host)
			createMockBuiltinFunctions(t, host, world)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.WithTrace().Ok().
				GasUsed(test.ParentAddress, expectedGasUsedByParent).
				GasUsed(test.ChildAddress, expectedGasUsedByChild).
				GasRemaining(testConfig.GasProvided - expectedGasUsedByParent - expectedGasUsedByChild)
		})
	assert.Nil(t, err)
}

func TestGasUsed_MockCreateContract(t *testing.T) {
	testConfig := makeTestConfig()

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(contracts.InitFunctionMock, contracts.WasteGasParentMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("wasteGas").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndCreateAndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte(vmhost.InitFunctionName))
		})
	assert.Nil(t, err)
}

func TestGasUsed_MockUpgradeContract(t *testing.T) {
	testConfig := makeTestConfig()

	codeMetadata := []byte{vmcommon.MetadataUpgradeable, 0}

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithOwnerAddress(test.UserAddress).
				WithCodeMetadata(codeMetadata).
				WithMethods(contracts.UpgradeFunctionMock, contracts.WasteGasParentMock)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction(vmhost.UpgradeFunctionName).
			WithArguments(mock.MockContractCode(test.ParentAddress), codeMetadata).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte(vmhost.UpgradeFunctionName))
		})
	assert.Nil(t, err)
}

type MockClaimBuiltin struct {
	test.MockBuiltin
	AmountToGive int64
	GasCost      uint64
}

var amountToGiveByBuiltinClaim = int64(42)

func createMockBuiltinFunctions(tb testing.TB, host vmhost.VMHost, world *worldmock.MockWorld) {
	err := world.InitBuiltinFunctions(host.GetGasScheduleMap(), host.ForkController())
	require.Nil(tb, err)

	mockClaimBuiltin := &MockClaimBuiltin{
		AmountToGive: amountToGiveByBuiltinClaim,
		GasCost:      gasUsedByBuiltinClaim,
	}

	_ = world.BuiltinFuncs.Container.Add("builtinClaim", &test.MockBuiltin{
		ProcessBuiltinFunctionCall: func(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			vmOutput := test.MakeEmptyVMOutput()
			test.AddNewOutputTransfer(
				vmOutput,
				nil,
				nil,
				mockClaimBuiltin.AmountToGive,
				nil)
			vmOutput.GasRemaining = vmInput.GasProvided - mockClaimBuiltin.GasCost
			return vmOutput, nil
		},
	})

	_ = world.BuiltinFuncs.Container.Add("builtinFail", &test.MockBuiltin{
		ProcessBuiltinFunctionCall: func(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			return nil, errors.New("buildMockErr")
		},
	})

	err = world.BuiltinFuncs.Container.Add("sendMessage", &test.MockBuiltin{
		ProcessBuiltinFunctionCall: func(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			vmOutput := test.MakeEmptyVMOutput()

			account := test.AddNewOutputTransfer(
				vmOutput,
				nil,
				vmInput.RecipientAddr,
				0,
				[]byte("message"),
			)
			_ = account
			vmOutput.GasRemaining = 0
			return vmOutput, nil
		},
	})
	assert.Nil(tb, err)

	host.SetBuiltInFunctionsContainer(world.BuiltinFuncs.Container)
}

func setZeroCodeCosts(host vmhost.VMHost) {
	host.Metering().GasSchedule().BaseOperationCost.CompilePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.AoTPreparePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.GetCode = 0
	host.Metering().GasSchedule().BaseOperationCost.StorePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.DataCopyPerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.PersistPerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.ReleasePerByte = 0
	host.Metering().GasSchedule().BaseOpsAPICost.SignalError = 0
	host.Metering().GasSchedule().BaseOpsAPICost.ExecuteOnSameContext = 0
	host.Metering().GasSchedule().BaseOpsAPICost.ExecuteOnDestContext = 0
	host.Metering().GasSchedule().BaseOpsAPICost.StorageLoad = 0
	host.Metering().GasSchedule().BaseOpsAPICost.StorageStore = 0
	host.Metering().GasSchedule().BaseOpsAPICost.TransferValue = 0
	host.Metering().GasSchedule().BaseOpsAPICost.CreateContract = 0
}
