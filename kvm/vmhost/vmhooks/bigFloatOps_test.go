package vmhooks_test

import (
	"math/big"
	"strconv"
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

	t.Run("Absolute-zero base short-circuits without charging gas", func(t *testing.T) {
		// An absolute-zero base has a trivial, bounded result and must not run the
		// unbounded pow loop nor charge result-size gas: it succeeds even with no
		// gas provided. 0^0 == 1 and 0^n == 0 for n > 0.
		cases := []struct {
			exponent int32
			expected float64
		}{
			{exponent: 0, expected: 1.0},
			{exponent: 1, expected: 0.0},
			{exponent: 64, expected: 0.0},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(strconv.Itoa(int(tc.exponent)), func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
				require.NoError(t, err)
				hooks := vmhooks.NewVMHooksImpl(vmHost)
				provideGas(hooks, 0) // no gas: proves no result-size charge is made

				baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(0.0))
				require.NoError(t, err)
				destHandle := int32(112)

				hooks.BigFloatPow(destHandle, baseHandle, tc.exponent)

				result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
				require.NoError(t, err, "absolute-zero base must not fault")
				require.Equal(t, 0, big.NewFloat(tc.expected).Cmp(result), "0^%d should be %v", tc.exponent, tc.expected)
			})
		}
	})

	t.Run("Fractional bases charge positive gas", func(t *testing.T) {
		// exponent chosen so the result-size charge (exponent*bitLen/8, bitLen == 1)
		// is a whole, strictly-positive number of gas units.
		const exponent = int32(64)
		const expectedGasCharge = uint64(exponent) / 8 // 8 gas

		for _, base := range []float64{-0.9, -0.1, 0.1, 0.9} {
			base := base
			t.Run(strconv.FormatFloat(base, 'f', -1, 64), func(t *testing.T) {
				t.Run("faults when gas is one unit short", func(t *testing.T) {
					mockWorld := worldmock.NewMockWorld()
					vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
					require.NoError(t, err)
					hooks := vmhooks.NewVMHooksImpl(vmHost)
					// One unit short of the positive result-size charge.
					provideGas(hooks, expectedGasCharge-1)

					baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(base))
					require.NoError(t, err)
					destHandle := int32(110)

					hooks.BigFloatPow(destHandle, baseHandle, exponent)

					_, err = hooks.GetManagedTypesContext().GetBigFloat(destHandle)
					require.Error(t, err, "a positive gas charge must exhaust the one-unit-short budget and leave the destination unset")
				})

				t.Run("runs to completion with sufficient gas", func(t *testing.T) {
					mockWorld := worldmock.NewMockWorld()
					vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
					require.NoError(t, err)
					hooks := vmhooks.NewVMHooksImpl(vmHost)
					provideGas(hooks, 1_000_000)

					baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(base))
					require.NoError(t, err)
					destHandle := int32(111)

					hooks.BigFloatPow(destHandle, baseHandle, exponent)

					result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
					require.NoError(t, err, "base %v must produce a result", base)
					expectedSign := 1
					if base < 0 && exponent%2 != 0 {
						expectedSign = -1
					}
					require.Equal(t, expectedSign, result.Sign(), "sign of %v^%d", base, exponent)
				})
			})
		}
	})
}
