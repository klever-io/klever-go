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
		BlockGasLimit:        uint64(1000),
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
