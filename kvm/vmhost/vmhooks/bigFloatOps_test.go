package vmhooks_test

import (
	"context"
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
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/require"
)

// provideGas seeds the metering context so gas-charging hooks (BigFloatPow /
// BigIntPow pre-charge on result size) can reach their compute path in a direct
// unit test that does not go through RunSmartContractCall.
func provideGas(hooks *vmhooks.VMHooksImpl, gas uint64) {
	hooks.GetMeteringContext().InitStateFromContractCallInput(&vmcommon.VMInput{GasProvided: gas})
}

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
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
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
		require.NoError(t, err)
		destHandle := int32(101)

		hooks.BigFloatPow(destHandle, baseHandle, exponent)

		_, err = hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		require.Error(t, err, "should return an error for negative exponent")
	})

	t.Run("Zero base rejected", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		base := big.NewFloat(0.0)
		exponent := int32(5)
		baseHandle, _ := hooks.GetManagedTypesContext().PutBigFloat(base)
		destHandle := int32(102)

		hooks.BigFloatPow(destHandle, baseHandle, exponent)

		// The guard faults before writing a result, so the destination stays unset.
		_, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		require.Error(t, err, "zero base should be rejected, leaving destination unset")
	})
}

type hostWithExecutionContext struct {
	vmhost.VMHost
	ctx context.Context
}

func (h *hostWithExecutionContext) GetExecutionContext() context.Context {
	return h.ctx
}

func TestBigFloatPow_Timeout(t *testing.T) {
	t.Run("cancelled context panics before writing result", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already timed out
		hooks := vmhooks.NewVMHooksImpl(&hostWithExecutionContext{VMHost: vmHost, ctx: ctx})
		provideGas(hooks, 1_000_000)

		base := big.NewFloat(2.0)
		exponent := int32(1000)
		baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(base)
		require.NoError(t, err)
		destHandle := int32(200)

		require.PanicsWithValue(t, vmhost.ErrExecutionFailedWithTimeout, func() {
			hooks.BigFloatPow(destHandle, baseHandle, exponent)
		}, "cancelled context must panic ErrExecutionFailedWithTimeout")

		_, err = hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		require.Error(t, err, "destination must stay unset on timeout")
	})

	t.Run("nil context runs to completion", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost) // real host => GetExecutionContext() == nil

		base := big.NewFloat(2.0)
		exponent := int32(3)
		baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(base)
		require.NoError(t, err)
		destHandle := int32(201)

		require.NotPanics(t, func() {
			hooks.BigFloatPow(destHandle, baseHandle, exponent)
		}, "nil context must not panic")

		result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(8.0).Cmp(result), "2^3 should be 8.0")
	})
}
