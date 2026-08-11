package testcommon

import (
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	blockchainConfig "github.com/klever-io/klever-go/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

var logMock = logger.GetOrCreate("vm/mock")

// TestType indicates whether the test is a SC call test or a SC creation test
type TestType int

const (
	// RunTest indicates a test with SC calls
	RunTest TestType = iota

	// CreateTest indicates a test with SC creation
	CreateTest
)

// SetupFunction -
type SetupFunction func(vmhost.VMHost, *worldmock.MockWorld)

// AssertResultsFunc -
type AssertResultsFunc func(world *worldmock.MockWorld, verify *VMOutputVerifier)

// AssertResultsWithStartNodeFunc -
type AssertResultsWithStartNodeFunc func(startNode *TestCallNode, world *worldmock.MockWorld, verify *VMOutputVerifier, expectedErrorsForRound []string)

type testTemplateConfig struct {
	tb                       testing.TB
	input                    *vmcommon.ContractCallInput
	useMocks                 bool
	wasmerSIGSEGVPassthrough bool
}

// MockInstancesTestTemplate holds the data to build a mock contract call test
type MockInstancesTestTemplate struct {
	testTemplateConfig
	contracts     *[]MockTestSmartContract
	setup         SetupFunction
	assertResults func(*TestCallNode, *worldmock.MockWorld, *VMOutputVerifier, []string)
	enableEpochs  *blockchainConfig.EnableEpochs
}

// BuildMockInstanceCallTest starts the building process for a mock contract call test
func BuildMockInstanceCallTest(tb testing.TB) *MockInstancesTestTemplate {
	return &MockInstancesTestTemplate{
		testTemplateConfig: testTemplateConfig{
			tb:                       tb,
			useMocks:                 true,
			wasmerSIGSEGVPassthrough: false,
		},
		setup: func(vmhost.VMHost, *worldmock.MockWorld) {},
	}
}

// WithContracts provides the contracts to be used by the mock contract call test
func (callerTest *MockInstancesTestTemplate) WithContracts(usedContracts ...MockTestSmartContract) *MockInstancesTestTemplate {
	callerTest.contracts = &usedContracts
	return callerTest
}

// WithInput provides the ContractCallInput to be used by the mock contract call test
func (callerTest *MockInstancesTestTemplate) WithInput(input *vmcommon.ContractCallInput) *MockInstancesTestTemplate {
	callerTest.input = input
	return callerTest
}

// WithSetup provides the setup function to be used by the mock contract call test
func (callerTest *MockInstancesTestTemplate) WithSetup(setup SetupFunction) *MockInstancesTestTemplate {
	callerTest.setup = setup
	return callerTest
}

// WithEnableEpochs overrides the default fork-flag config (all flags enabled at epoch 0)
// with the supplied EnableEpochs. Use to exercise pre-activation behavior of fork-gated
// changes (e.g. set FixAuditChangesV2 to a high epoch to assert old behavior).
func (callerTest *MockInstancesTestTemplate) WithEnableEpochs(epochs blockchainConfig.EnableEpochs) *MockInstancesTestTemplate {
	callerTest.enableEpochs = &epochs
	return callerTest
}

// WithWasmerSIGSEGVPassthrough sets the wasmerSIGSEGVPassthrough flag
func (callerTest *MockInstancesTestTemplate) WithWasmerSIGSEGVPassthrough(wasmerSIGSEGVPassthrough bool) *MockInstancesTestTemplate {
	callerTest.wasmerSIGSEGVPassthrough = wasmerSIGSEGVPassthrough
	return callerTest
}

// AndAssertResults provides the function that will aserts the results
func (callerTest *MockInstancesTestTemplate) AndAssertResults(assertResults AssertResultsFunc) (*vmcommon.VMOutput, error) {
	return callerTest.andAssertResultsWithWorld(nil, true, nil, RunTest, nil, func(startNode *TestCallNode, world *worldmock.MockWorld, verify *VMOutputVerifier, expectedErrorsForRound []string) {
		// update cached values if success or reset on error
		if verify.VmOutput.ReturnCode == vmcommon.Ok {
			if err := world.AccountsCacher.SaveAll(); err != nil {
				callerTest.tb.Errorf("failed to save accounts: %v", err)
			}
		} else {
			world.AccountsCacher.ResetAll(true)
		}
		// assert results
		assertResults(world, verify)
	})
}

// AndCreateAndAssertResults provides the function that will create the contract and aserts the results
func (callerTest *MockInstancesTestTemplate) AndCreateAndAssertResults(assertResults AssertResultsFunc) (*vmcommon.VMOutput, error) {
	return callerTest.andAssertResultsWithWorld(nil, true, nil, CreateTest, nil, func(startNode *TestCallNode, world *worldmock.MockWorld, verify *VMOutputVerifier, expectedErrorsForRound []string) {
		assertResults(world, verify)
	})
}

// AndAssertResultsWithWorld provides the function that will aserts the results
func (callerTest *MockInstancesTestTemplate) AndAssertResultsWithWorld(
	world *worldmock.MockWorld,
	createAccount bool,
	startNode *TestCallNode,
	expectedErrorsForRound []string,
	assertResults AssertResultsWithStartNodeFunc) (*vmcommon.VMOutput, error) {
	return callerTest.andAssertResultsWithWorld(world, createAccount, startNode, RunTest, expectedErrorsForRound, assertResults)
}

func (callerTest *MockInstancesTestTemplate) andAssertResultsWithWorld(
	world *worldmock.MockWorld,
	createAccount bool,
	startNode *TestCallNode,
	testType TestType,
	expectedErrorsForRound []string,
	assertResults AssertResultsWithStartNodeFunc) (*vmcommon.VMOutput, error) {
	callerTest.assertResults = assertResults
	if world == nil {
		world = worldmock.NewMockWorld()
	}
	return callerTest.runTest(startNode, world, createAccount, testType, expectedErrorsForRound)
}

func (callerTest *MockInstancesTestTemplate) runTest(
	startNode *TestCallNode,
	world *worldmock.MockWorld,
	createContractAccounts bool,
	testType TestType,
	expectedErrorsForRound []string) (*vmcommon.VMOutput, error) {
	if world == nil {
		world = worldmock.NewMockWorld()
	}

	var err error
	user, err := world.AccountsCacher.LoadUser(UserAddress)
	if err != nil {
		return nil, err
	}
	err = world.AccountsCacher.SaveUser(user)
	if err != nil {
		return nil, err
	}

	executorFactory := mock.NewExecutorMockFactory(world)
	hostBuilder := NewTestHostBuilder(callerTest.tb).
		WithExecutorFactory(executorFactory).
		WithBlockchainHook(world)
	if callerTest.enableEpochs != nil {
		hostBuilder = hostBuilder.WithEnableEpochs(*callerTest.enableEpochs)
	}
	host := hostBuilder.Build()

	defer func() {
		host.Reset()
	}()

	for _, mockSC := range *callerTest.contracts {
		mockSC.Initialize(callerTest.tb, host, executorFactory.LastCreatedExecutor, createContractAccounts)
	}

	callerTest.setup(host, world)
	// create snapshot (normaly done by node)
	world.CreateStateBackup()

	var vmOutput *vmcommon.VMOutput
	switch testType {
	case RunTest:
		vmOutput, err = host.RunSmartContractCall(callerTest.input)
	case CreateTest:
		vmOutput, err = host.RunSmartContractCreate(&vmcommon.ContractCreateInput{
			VMInput:      callerTest.input.VMInput,
			ContractCode: mock.MockContractCode(callerTest.input.RecipientAddr),
		})
	}

	allErrors := host.Runtime().GetAllErrors()
	verify := NewVMOutputVerifierWithAllErrors(callerTest.tb, vmOutput, err, allErrors)
	if callerTest.assertResults != nil {
		callerTest.assertResults(startNode, world, verify, expectedErrorsForRound)
	}

	return vmOutput, err
}

// SimpleWasteGasMockMethod is a simple waste gas mock method
func SimpleWasteGasMockMethod(instanceMock *mock.InstanceMock, gas uint64) func() *mock.InstanceMock {
	return func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		err := host.Metering().UseGasBounded(gas)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
		}

		return instance
	}
}

// WasteGasWithReturnDataMockMethod is a simple waste gas mock method
func WasteGasWithReturnDataMockMethod(instanceMock *mock.InstanceMock, gas uint64, returnData []byte) func() *mock.InstanceMock {
	return func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		logMock.Trace("instance mock waste gas", "sc", string(host.Runtime().GetContextAddress()), "func", host.Runtime().FunctionName(), "gas", gas)
		err := host.Metering().UseGasBounded(gas)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
			return instance
		}

		host.Output().Finish(returnData)
		return instance
	}
}
