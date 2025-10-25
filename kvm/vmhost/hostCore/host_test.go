package hostCore_test

import (
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/kapps"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/require"
)

func makeHostParameters() *vmhost.VMHostParameters {
	kdaTransferParser, _ := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)
	epochNotifier := &commonMock.EpochNotifierStub{}

	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	protectedKeys := [][]byte{
		[]byte(kapps.ProtectedKleverKeyPrefix),
		[]byte(kapps.ProtectedKLVKeyPrefix),
		[]byte(kapps.ProtectedKFIKeyPrefix),
		[]byte(kapps.KDAPrefix),
	}

	return &vmhost.VMHostParameters{
		VMType:               testcommon.DefaultVMType,
		KDATransferParser:    kdaTransferParser,
		BuiltInFuncContainer: builtInFunctions.NewBuiltInFunctionContainer(),
		EpochNotifier:        epochNotifier,
		Hasher:               worldmock.DefaultHasher,
		ForkController:       forkController,
		GasSchedule:          kvmConfig.MakeGasMapForTests(),
		ProtectedKeyPrefix:   protectedKeys,
	}
}

func TestNewVMHost(t *testing.T) {
	blockchainHook := worldmock.NewMockWorld()
	bfc := builtInFunctions.NewBuiltInFunctionContainer()
	vmType := []byte("vmType")
	kdaTransferParser, err := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)
	require.Nil(t, err)

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	makeHostParameters := func() *vmhost.VMHostParameters {
		return &vmhost.VMHostParameters{
			VMType:               vmType,
			KDATransferParser:    kdaTransferParser,
			BuiltInFuncContainer: bfc,
			EpochNotifier:        epochNotifier,
			Hasher:               worldmock.DefaultHasher,
			ForkController:       forkController,
		}
	}

	t.Run("NilBlockchainHook", func(t *testing.T) {
		host, err := hostCore.NewVMHost(nil, makeHostParameters())
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilBlockChainHook)
	})
	t.Run("NilHostParameters", func(t *testing.T) {
		host, err := hostCore.NewVMHost(blockchainHook, nil)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilHostParameters)
	})
	t.Run("NilKDATransferParser", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.KDATransferParser = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilKDATransferParser)
	})
	t.Run("NilBuiltInFunctionsContainer", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.BuiltInFuncContainer = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilBuiltInFunctionsContainer)
	})
	t.Run("NilEpochNotifier", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.EpochNotifier = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilEpochNotifier)
	})
	t.Run("NilForkController", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.ForkController = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilEnableEpochsHandler)
	})
	t.Run("NilHasher", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.Hasher = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilHasher)
	})
	t.Run("NilVMType", func(t *testing.T) {
		hostParameters := makeHostParameters()
		hostParameters.VMType = nil
		host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
		require.Nil(t, host)
		require.ErrorIs(t, err, vmhost.ErrNilVMType)
	})
}

func TestSetGetExecutionMode(t *testing.T) {
	blockchainHook := worldmock.NewMockWorld()
	hostParameters := makeHostParameters()

	host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
	require.Nil(t, err)
	require.NotNil(t, host)

	t.Run("sets and gets validator mode", func(t *testing.T) {
		host.SetExecutionMode(vmcommon.ExecutionModeValidator)
		mode := host.GetExecutionMode()
		require.Equal(t, vmcommon.ExecutionModeValidator, mode)
	})

	t.Run("sets and gets leader mode", func(t *testing.T) {
		host.SetExecutionMode(vmcommon.ExecutionModeLeader)
		mode := host.GetExecutionMode()
		require.Equal(t, vmcommon.ExecutionModeLeader, mode)
	})

	t.Run("sets and gets query mode", func(t *testing.T) {
		host.SetExecutionMode(vmcommon.ExecutionModeQuery)
		mode := host.GetExecutionMode()
		require.Equal(t, vmcommon.ExecutionModeQuery, mode)
	})
}

func TestGetters(t *testing.T) {
	blockchainHook := worldmock.NewMockWorld()
	hostParameters := makeHostParameters()

	host, err := hostCore.NewVMHost(blockchainHook, hostParameters)
	require.Nil(t, err)
	require.NotNil(t, host)

	t.Run("GetVersion returns non-empty string", func(t *testing.T) {
		version := host.GetVersion()
		require.NotEmpty(t, version)
	})

	t.Run("Crypto returns non-nil", func(t *testing.T) {
		crypto := host.Crypto()
		require.NotNil(t, crypto)
	})

	t.Run("ForkController returns non-nil", func(t *testing.T) {
		fc := host.ForkController()
		require.NotNil(t, fc)
	})

	t.Run("GetGasScheduleMap returns non-nil", func(t *testing.T) {
		gasSchedule := host.GetGasScheduleMap()
		require.NotNil(t, gasSchedule)
	})
}
