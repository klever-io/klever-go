package testcommon

import (
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	executorwrapper "github.com/klever-io/klever-go/kvm/executor/wrapper"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon/testexecutor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/require"
)

// TestHostBuilder allows tests to configure and initialize the VM host and blockhain mock on which they operate.
type TestHostBuilder struct {
	tb               testing.TB
	blockchainHook   vmcommon.BlockchainHook
	vmHostParameters *vmhost.VMHostParameters
	host             vmhost.VMHost
}

// NewTestHostBuilder commences a test host builder pattern.
func NewTestHostBuilder(tb testing.TB) *TestHostBuilder {
	kdaTransferParser, _ := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)
	protectedKeys := [][]byte{
		[]byte(kapps.ProtectedKleverKeyPrefix),
		[]byte(kapps.ProtectedKLVKeyPrefix),
		[]byte(kapps.ProtectedKFIKeyPrefix),
		[]byte(kapps.KDAPrefix),
	}
	epochNotifier := &commonMock.EpochNotifierStub{}

	forkController, _ := fork.NewForkController(blockchainConfig.EnableEpochs{
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
	}, epochNotifier)

	return &TestHostBuilder{
		tb: tb,
		vmHostParameters: &vmhost.VMHostParameters{
			VMType:                   DefaultVMType,
			GasSchedule:              nil,
			BuiltInFuncContainer:     nil,
			ProtectedKeyPrefix:       protectedKeys,
			KDATransferParser:        kdaTransferParser,
			EpochNotifier:            epochNotifier,
			OverrideVMExecutor:       nil,
			WasmerSIGSEGVPassthrough: false,
			Hasher:                   defaultHasher,
			ForkController:           forkController,
		},
	}
}

// WithEnableEpochs replaces the ForkController on this host with one constructed from
// the supplied EnableEpochs config. Useful for tests that need to exercise pre-activation
// behavior of fork-gated changes (e.g. set FixAuditChangesV2 to a high epoch to keep the
// flag disabled at epoch 0).
func (thb *TestHostBuilder) WithEnableEpochs(epochs blockchainConfig.EnableEpochs) *TestHostBuilder {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(epochs, epochNotifier)
	thb.vmHostParameters.EpochNotifier = epochNotifier
	thb.vmHostParameters.ForkController = forkController
	return thb
}

// Ensures gas costs are initialized.
func (thb *TestHostBuilder) initializeGasCosts() {
	if thb.vmHostParameters.GasSchedule == nil {
		thb.vmHostParameters.GasSchedule = config.MakeGasMapForTests()
	}
}

// Ensures the built-in function container is initialized.
func (thb *TestHostBuilder) initializeBuiltInFuncContainer() {
	if thb.vmHostParameters.BuiltInFuncContainer == nil {
		thb.vmHostParameters.BuiltInFuncContainer = builtInFunctions.NewBuiltInFunctionContainer()
	}

}

// WithBlockchainHook sets a pre-built blockchain hook for the VM to work with.
func (thb *TestHostBuilder) WithBlockchainHook(blockchainHook vmcommon.BlockchainHook) *TestHostBuilder {
	thb.blockchainHook = blockchainHook
	return thb
}

// WithBuiltinFunctions sets up builtin functions in the blockchain hook.
// Only works if the blockchain hook is of type worldmock.MockWorld.
func (thb *TestHostBuilder) WithBuiltinFunctions() *TestHostBuilder {
	thb.initializeGasCosts()
	mockWorld, ok := thb.blockchainHook.(*worldmock.MockWorld)
	require.True(thb.tb, ok, "builtin functions can only be injected into blockchain hooks of type MockWorld")
	err := mockWorld.InitBuiltinFunctions(thb.vmHostParameters.GasSchedule, thb.vmHostParameters.ForkController)
	require.Nil(thb.tb, err)
	thb.vmHostParameters.BuiltInFuncContainer = mockWorld.BuiltinFuncs.Container
	return thb
}

// WithExecutorFactory allows tests to choose what executor to use.
func (thb *TestHostBuilder) WithExecutorFactory(executorFactory executor.ExecutorAbstractFactory) *TestHostBuilder {
	thb.vmHostParameters.OverrideVMExecutor = executorFactory
	return thb
}

// WithExecutorLogs sets an ExecutorLogger, which wraps the existing OverrideVMExecutor
func (thb *TestHostBuilder) WithExecutorLogs(executorLogger executorwrapper.ExecutorLogger) *TestHostBuilder {
	if thb.vmHostParameters.OverrideVMExecutor == nil {
		thb.tb.Fatal("WithExecutorLogs() requires WithExecutorFactory()")
	}

	wrapper := executorwrapper.NewWrappedExecutorFactory(
		executorLogger,
		thb.vmHostParameters.OverrideVMExecutor)

	return thb.WithExecutorFactory(wrapper)
}

// WithWasmerSIGSEGVPassthrough allows tests to configure the WasmerSIGSEGVPassthrough flag.
func (thb *TestHostBuilder) WithWasmerSIGSEGVPassthrough(wasmerSIGSEGVPassthrough bool) *TestHostBuilder {
	thb.vmHostParameters.WasmerSIGSEGVPassthrough = wasmerSIGSEGVPassthrough
	return thb
}

// WithGasSchedule allows tests to use the gas costs. The default is config.MakeGasMapForTests().
func (thb *TestHostBuilder) WithGasSchedule(gasSchedule config.GasScheduleMap) *TestHostBuilder {
	thb.vmHostParameters.GasSchedule = gasSchedule
	return thb
}

// Build initializes the VM host with all configured options.
func (thb *TestHostBuilder) Build() vmhost.VMHost {
	thb.initializeHost()
	return thb.host
}

func (thb *TestHostBuilder) initializeHost() {
	thb.initializeGasCosts()
	if thb.host == nil {
		thb.host = thb.newHost()
	}
}

func (thb *TestHostBuilder) newHost() vmhost.VMHost {
	if check.IfNil(thb.vmHostParameters.OverrideVMExecutor) {
		exec := testexecutor.NewDefaultTestExecutorFactory(thb.tb)
		thb.vmHostParameters.OverrideVMExecutor = exec
	}

	thb.initializeBuiltInFuncContainer()
	host, err := hostCore.NewVMHost(
		thb.blockchainHook,
		thb.vmHostParameters,
	)
	require.Nil(thb.tb, err)
	require.NotNil(thb.tb, host)

	return host
}
