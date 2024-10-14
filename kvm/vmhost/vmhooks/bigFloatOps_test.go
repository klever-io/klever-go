package vmhooks_test

import (
	"math/big"
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
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
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

func TestBigFloatPow(t *testing.T) {
	t.Run("Positive base, positive exponent", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		base := big.NewFloat(2.0)
		exponent := int32(3)
		baseHandle, _ := hooks.GetManagedTypesContext().PutBigFloat(base)
		destHandle := int32(100)

		hooks.BigFloatPow(destHandle, baseHandle, exponent)

		result, _ := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		expected := big.NewFloat(8.0)
		require.Equal(t, 0, expected.Cmp(result), "the result should be 8.0")
	})

	t.Run("Negative exponent", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		base := big.NewFloat(2.0)
		exponent := int32(-1)
		baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(base)
		destHandle := int32(101)

		hooks.BigFloatPow(destHandle, baseHandle, exponent)

		_, err = hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		require.Error(t, err, "should return an error for negative exponent")
	})

	t.Run("Zero base, positive exponent", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		base := big.NewFloat(0.0)
		exponent := int32(5)
		baseHandle, _ := hooks.GetManagedTypesContext().PutBigFloat(base)
		destHandle := int32(102)

		hooks.BigFloatPow(destHandle, baseHandle, exponent)

		result, _ := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		expected := big.NewFloat(0.0)
		require.Equal(t, 0, expected.Cmp(result), "the result should be 0.0")
	})
}
