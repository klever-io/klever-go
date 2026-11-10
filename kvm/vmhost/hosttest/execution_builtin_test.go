package hostCoretest

import (
	"encoding/hex"
	"errors"
	"math/big"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/require"
)

func TestExecution_ExecuteOnDestContext_MockBuiltinFunctions_Claim(t *testing.T) {
	forEachFixAuditChangesV6(t, 12, func(t *testing.T, epochs blockchainConfig.EnableEpochs, copyGas uint64) {
		parentGasUsed := uint64(1973) + copyGas
		transferGas := uint64(2005)
		parentGasUsed += transferGas
		test.BuildInstanceCallTest(t).
			WithEnableEpochs(epochs).
			WithContracts(
				test.CreateInstanceContract(test.ParentAddress).
					WithCode(test.GetTestSCCode("exec-dest-ctx-builtin", "../../")).
					WithBalance(1000)).
			WithInput(test.CreateTestContractCallInputBuilder().
				WithRecipientAddr(test.ParentAddress).
				WithGasProvided(test.GasProvided).
				WithFunction("callBuiltinClaim").
				Build()).
			WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
				stubBlockchainHook.ProcessBuiltInFunctionCalled = dummyProcessBuiltInFunction
				host.SetBuiltInFunctionsContainer(getDummyBuiltinFunctionsContainer())
			}).
			AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
				verify.Ok().
					GasUsed(test.ParentAddress, parentGasUsed).
					GasRemaining(test.GasProvided - parentGasUsed).
					ReturnData([]byte("succ"))
			})
	})
}

func TestExecution_ExecuteOnDestContext_MockBuiltinFunctions_DoSomething(t *testing.T) {
	forEachFixAuditChangesV6(t, 18, func(t *testing.T, epochs blockchainConfig.EnableEpochs, copyGas uint64) {
		parentGasUsed := uint64(1977) + copyGas
		transferGas := uint64(2005)
		parentGasUsed += transferGas
		test.BuildInstanceCallTest(t).
			WithEnableEpochs(epochs).
			WithContracts(
				test.CreateInstanceContract(test.ParentAddress).
					WithCode(test.GetTestSCCode("exec-dest-ctx-builtin", "../../")).
					WithBalance(1000)).
			WithInput(test.CreateTestContractCallInputBuilder().
				WithRecipientAddr(test.ParentAddress).
				WithGasProvided(test.GasProvided).
				WithFunction("callBuiltinDoSomething").
				Build()).
			WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
				stubBlockchainHook.ProcessBuiltInFunctionCalled = dummyProcessBuiltInFunction
				host.SetBuiltInFunctionsContainer(getDummyBuiltinFunctionsContainer())
			}).
			AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
				verify.Ok().
					GasUsed(test.ParentAddress, parentGasUsed).
					GasRemaining(test.GasProvided - parentGasUsed).
					ReturnData([]byte("succ"))
			})
	})
}

func TestExecution_ExecuteOnDestContext_MockBuiltinFunctions_Nonexistent(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-dest-ctx-builtin", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(test.GasProvided).
			WithFunction("callNonexistingBuiltin").
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.ProcessBuiltInFunctionCalled = dummyProcessBuiltInFunction
			host.SetBuiltInFunctionsContainer(getDummyBuiltinFunctionsContainer())
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				ReturnMessage(executor.ErrFuncNotFound.Error()).
				GasRemaining(0)
		})
}

func TestExecution_ExecuteOnDestContext_MockBuiltinFunctions_Fail(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exec-dest-ctx-builtin", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(test.GasProvided).
			WithFunction("callBuiltinFail").
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.ProcessBuiltInFunctionCalled = dummyProcessBuiltInFunction
			host.SetBuiltInFunctionsContainer(getDummyBuiltinFunctionsContainer())
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				ReturnMessage("buildMockErr").
				GasRemaining(0)
		})
}

func TestKDA_GettersAPI(t *testing.T) {
	t.Skip("TODO: create a contract to test this")
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("exchange", "../../")).
				WithBalance(1000)).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(test.GasProvided).
			WithFunction("validateGetters").
			WithKDAValue(big.NewInt(5)).
			WithKDATokenName([]byte("TT")).
			Build()).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.ProcessBuiltInFunctionCalled = dummyProcessBuiltInFunction
			host.SetBuiltInFunctionsContainer(getDummyBuiltinFunctionsContainer())
		}).
		AndAssertResults(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()
		})
}
func dummyProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	outputAccounts := make(map[string]*vmcommon.OutputAccount)
	outputAccounts[string(test.ParentAddress)] = &vmcommon.OutputAccount{
		Address: test.ParentAddress}

	if input.Function == "builtinClaim" {
		return &vmcommon.VMOutput{
			GasRemaining:   400,
			OutputAccounts: outputAccounts,
		}, nil
	}
	if input.Function == "builtinDoSomething" {
		return &vmcommon.VMOutput{
			GasRemaining:   400,
			OutputAccounts: outputAccounts,
		}, nil
	}
	if input.Function == "builtinFail" {
		return nil, errors.New("buildMockErr")
	}
	if input.Function == core.BuiltInFunctionTransfer {
		vmOutput := &vmcommon.VMOutput{
			GasRemaining: 0,
		}
		function := string(input.Arguments[2])
		kdaTransferTxData := function
		for _, arg := range input.Arguments[3:] {
			kdaTransferTxData += "@" + hex.EncodeToString(arg)
		}
		outTransfer := vmcommon.OutputTransfer{
			SenderAddress: input.CallerAddr,
		}
		vmOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
		vmOutput.OutputAccounts[string(input.RecipientAddr)] = &vmcommon.OutputAccount{
			Address:         input.RecipientAddr,
			OutputTransfers: []vmcommon.OutputTransfer{outTransfer},
		}
		// FIXME when KDA token balance querying is implemented, ensure the
		// transfers that happen here are persisted in the mock accounts
		return vmOutput, nil
	}

	return nil, executor.ErrFuncNotFound
}

func getDummyBuiltinFunctionsContainer() vmcommon.BuiltInFunctionContainer {
	builtInContainer := builtInFunctions.NewBuiltInFunctionContainer()
	_ = builtInContainer.Add("builtinClaim", &test.MockBuiltin{})
	_ = builtInContainer.Add("builtinDoSomething", &test.MockBuiltin{})
	_ = builtInContainer.Add("builtinFail", &test.MockBuiltin{})
	_ = builtInContainer.Add(core.BuiltInFunctionTransfer, &test.MockBuiltin{})

	return builtInContainer
}

type countedTransferBuiltin struct {
	vmcommon.BuiltinFunction
	counter interface {
		ProcessMaxBuiltInCounters(input *vmcommon.ContractCallInput) error
	}
	counterErr error
}

func (builtin *countedTransferBuiltin) ProcessBuiltinFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if err := builtin.counter.ProcessMaxBuiltInCounters(input); err != nil {
		builtin.counterErr = err
		return nil, err
	}

	return builtin.BuiltinFunction.ProcessBuiltinFunction(input)
}

func countTransfersWithUsageCounter(t *testing.T, host vmhost.VMHost, world *worldmock.MockWorld) *countedTransferBuiltin {
	parser, err := parsers.NewKDATransferParser(&marshal.ProtoMarshalizer{})
	require.NoError(t, err)
	counter, err := counters.NewUsageCounter(parser, host.ForkController())
	require.NoError(t, err)
	counter.SetMaximumValues(map[string]uint64{
		"MaxBuiltInCallsPerTx":      100,
		"MaxNumberOfTransfersPerTx": 250,
		"MaxNumberOfTrieReadsPerTx": 1500,
	})

	transfer, err := world.BuiltinFuncs.Container.Get(core.BuiltInFunctionTransfer)
	require.NoError(t, err)
	counted := &countedTransferBuiltin{
		BuiltinFunction: transfer,
		counter:         counter,
	}
	require.NoError(t, world.BuiltinFuncs.Container.Replace(core.BuiltInFunctionTransfer, counted))

	return counted
}

func unitTransfers(numTransfers int) []zeroValueTransferSpec {
	specs := make([]zeroValueTransferSpec, numTransfers)
	for i := range specs {
		specs[i] = zeroValueTransferSpec{token: test.KDATestTokenName, value: 1}
	}

	return specs
}

func runTransfersUnderUsageCounter(
	t *testing.T,
	callerBody func(vmhost.VMHost),
	epochs blockchainConfig.EnableEpochs,
	assertResults func(*worldmock.MockWorld, *test.VMOutputVerifier),
) error {
	testConfig := makeTestConfig()
	var counted *countedTransferBuiltin

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(instance *contextmock.InstanceMock, _ interface{}) {
					instance.AddMockMethod("transferAndCall", func() *contextmock.InstanceMock {
						callerBody(instance.Host)
						return instance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(recordCallValueMock(&observedCallValue{})),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("transferAndCall").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
			host.Metering().GasSchedule().BuiltInCost.Transfer = 1
			host.Metering().GasSchedule().BaseOpsAPICost.TransferValue = 0

			caller, _ := world.AccountsCacher.LoadUser(test.ParentAddress)
			require.NoError(t, caller.AddToBalance(retargetInitialBalance, test.KDATestTokenName, true))

			counted = countTransfersWithUsageCounter(t, host, world)
		}).
		WithEnableEpochs(epochs).
		AndAssertResults(assertResults)
	require.NoError(t, err)

	return counted.counterErr
}

func TestKDA_TypedTransfersCountTowardTransferLimit(t *testing.T) {
	preForkV7 := blockchainConfig.EnableEpochs{FixAuditChangesV7: 1_000_000}
	postForkV7 := blockchainConfig.EnableEpochs{FixAuditChangesV7: 0}

	t.Run("atLimit", func(t *testing.T) {
		counterErr := runTransfersUnderUsageCounter(t, viaManagedMultiTransfer(unitTransfers(250)), postForkV7,
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
				require.Equal(t, int64(250), kdaBalanceOf(t, world, test.ChildAddress))
			})
		require.NoError(t, counterErr)
	})

	t.Run("pastLimit/postFork", func(t *testing.T) {
		counterErr := runTransfersUnderUsageCounter(t, viaManagedMultiTransfer(unitTransfers(251)), postForkV7,
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.ExecutionFailed()
				require.Zero(t, kdaBalanceOf(t, world, test.ChildAddress))
			})
		require.ErrorIs(t, counterErr, process.ErrMaxCallsReached)
	})

	t.Run("pastLimit/preFork", func(t *testing.T) {
		counterErr := runTransfersUnderUsageCounter(t, viaManagedMultiTransfer(unitTransfers(251)), preForkV7,
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
				require.Equal(t, int64(251), kdaBalanceOf(t, world, test.ChildAddress))
			})
		require.NoError(t, counterErr)
	})
}

func TestKDA_ArgumentTransfersCountOnceTowardTransferLimit(t *testing.T) {
	counterErr := runTransfersUnderUsageCounter(t, viaBuiltinFunction(unitTransfers(126)), blockchainConfig.EnableEpochs{FixAuditChangesV7: 0},
		func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok()
			require.Equal(t, int64(126), kdaBalanceOf(t, world, test.ChildAddress))
		})
	require.NoError(t, counterErr)
}
