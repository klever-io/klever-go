package vmhooks_test

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
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

func provideGas(hooks *vmhooks.VMHooksImpl, gas uint64) {
	hooks.GetMeteringContext().InitStateFromContractCallInput(&vmcommon.VMInput{GasProvided: gas})
}

func makeHostParameters() *vmhost.VMHostParameters {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	return makeHostParametersWithFork(forkController)
}

func makeHostParametersWithFork(forkController core.ForkController) *vmhost.VMHostParameters {
	kdaTransferParser, _ := parsers.NewKDATransferParser(worldmock.WorldMarshalizer)

	return &vmhost.VMHostParameters{
		VMType:               testcommon.DefaultVMType,
		KDATransferParser:    kdaTransferParser,
		BuiltInFuncContainer: builtInFunctions.NewBuiltInFunctionContainer(),
		EpochNotifier:        &commonMock.EpochNotifierStub{},
		Hasher:               worldmock.DefaultHasher,
		ForkController:       forkController,
		GasSchedule:          kvmConfig.MakeGasMapForTests(),
		ProtectedKeyPrefix: [][]byte{
			[]byte(kapps.ProtectedKleverKeyPrefix),
			[]byte(kapps.ProtectedKLVKeyPrefix),
			[]byte(kapps.ProtectedKFIKeyPrefix),
			[]byte(kapps.KDAPrefix),
		},
	}
}

func newForkStub(fixAuditV3 bool) *commonMock.ForkControllerStub {
	fc := commonMock.NewForkControllerStub()
	fc.FixAuditChangesV3Value = fixAuditV3
	return fc
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

	// A zero base always short-circuits to the same trivial value (0^0 == 1,
	// 0^n == 0 for n > 0). The FixAuditChangesV3 fork only changes the result-size
	// gas charged before that short-circuit: pre-fork the bit length collapses to 0
	// and nothing is charged, post-fork it is bumped to 1 and a positive amount is
	// charged.
	t.Run("Zero base short-circuits to same value regardless of fork", func(t *testing.T) {
		cases := []struct {
			exponent int32
			expected float64
		}{
			{exponent: 0, expected: 1.0},
			{exponent: 1, expected: 0.0},
			{exponent: 64, expected: 0.0},
		}
		for _, fixAuditV3 := range []bool{false, true} {
			fixAuditV3 := fixAuditV3
			t.Run(fmt.Sprintf("FixAuditChangesV3=%t", fixAuditV3), func(t *testing.T) {
				for _, tc := range cases {
					tc := tc
					t.Run(strconv.Itoa(int(tc.exponent)), func(t *testing.T) {
						mockWorld := worldmock.NewMockWorld()
						vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(fixAuditV3)))
						require.NoError(t, err)
						hooks := vmhooks.NewVMHooksImpl(vmHost)
						provideGas(hooks, 1_000_000)

						baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(0.0))
						require.NoError(t, err)
						destHandle := int32(112)

						hooks.BigFloatPow(destHandle, baseHandle, tc.exponent)

						result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
						require.NoError(t, err, "zero base must not fault")
						require.Equal(t, 0, big.NewFloat(tc.expected).Cmp(result), "0^%d should be %v", tc.exponent, tc.expected)
					})
				}
			})
		}
	})

	// The fork-gated difference is purely in the result-size gas charged for a zero
	// base before the short-circuit. exponent 64 gives a post-fork charge of
	// 64*1/8 == 8 gas (bit length bumped to 1); pre-fork the bit length is 0 so no
	// result-size gas is charged and the hook succeeds even with a budget of 0.
	t.Run("Zero base result-size gas is fork-gated", func(t *testing.T) {
		const exponent = int32(64)
		const postForkCharge = uint64(exponent) / 8 // 8 gas

		t.Run("pre-fork charges no result-size gas", func(t *testing.T) {
			mockWorld := worldmock.NewMockWorld()
			vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(false)))
			require.NoError(t, err)
			hooks := vmhooks.NewVMHooksImpl(vmHost)
			provideGas(hooks, 0) // no gas: proves no result-size charge is made

			baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(0.0))
			require.NoError(t, err)
			destHandle := int32(113)

			hooks.BigFloatPow(destHandle, baseHandle, exponent)

			result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
			require.NoError(t, err, "pre-fork zero base must not charge result-size gas")
			require.Equal(t, 0, big.NewFloat(0.0).Cmp(result), "0^64 should be 0")
		})

		t.Run("post-fork faults when gas is one unit short", func(t *testing.T) {
			mockWorld := worldmock.NewMockWorld()
			vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(true)))
			require.NoError(t, err)
			hooks := vmhooks.NewVMHooksImpl(vmHost)
			// One unit short of the positive post-fork result-size charge.
			provideGas(hooks, postForkCharge-1)

			baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(0.0))
			require.NoError(t, err)
			destHandle := int32(114)

			hooks.BigFloatPow(destHandle, baseHandle, exponent)

			_, err = hooks.GetManagedTypesContext().GetBigFloat(destHandle)
			require.Error(t, err, "post-fork zero base charges a positive amount that must exhaust the one-unit-short budget")
		})

		t.Run("post-fork runs to completion with the exact charge", func(t *testing.T) {
			mockWorld := worldmock.NewMockWorld()
			vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(true)))
			require.NoError(t, err)
			hooks := vmhooks.NewVMHooksImpl(vmHost)
			provideGas(hooks, postForkCharge)

			baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(big.NewFloat(0.0))
			require.NoError(t, err)
			destHandle := int32(115)

			hooks.BigFloatPow(destHandle, baseHandle, exponent)

			result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
			require.NoError(t, err, "post-fork zero base must succeed with exactly the result-size charge")
			require.Equal(t, 0, big.NewFloat(0.0).Cmp(result), "0^64 should be 0")
		})
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

	// The legacy multiply loop alternates the sign of a zero result each
	// iteration, so (-0)^odd is -0 and (-0)^even is +0. The zero-base
	// short-circuit is not fork-gated, so it must reproduce those signs exactly:
	// the sign bit survives big.Float serialization, and flipping it would
	// diverge historical replay.
	t.Run("Negative-zero base keeps the legacy sign of zero", func(t *testing.T) {
		cases := []struct {
			exponent    int32
			wantSignbit bool
		}{
			{exponent: 1, wantSignbit: true},
			{exponent: 2, wantSignbit: false},
			{exponent: 3, wantSignbit: true},
		}
		for _, fixAuditV3 := range []bool{false, true} {
			t.Run(fmt.Sprintf("FixAuditChangesV3=%t", fixAuditV3), func(t *testing.T) {
				for _, tc := range cases {
					t.Run(strconv.Itoa(int(tc.exponent)), func(t *testing.T) {
						mockWorld := worldmock.NewMockWorld()
						vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(fixAuditV3)))
						require.NoError(t, err)
						hooks := vmhooks.NewVMHooksImpl(vmHost)
						provideGas(hooks, 1_000_000)

						negZero := new(big.Float).Neg(big.NewFloat(0))
						baseHandle, err := hooks.GetManagedTypesContext().PutBigFloat(negZero)
						require.NoError(t, err)
						destHandle := int32(116)

						hooks.BigFloatPow(destHandle, baseHandle, tc.exponent)

						result, err := hooks.GetManagedTypesContext().GetBigFloat(destHandle)
						require.NoError(t, err, "negative-zero base must not fault")
						require.Equal(t, 0, result.Cmp(big.NewFloat(0)), "(-0)^%d must be zero", tc.exponent)
						require.Equal(t, tc.wantSignbit, result.Signbit(), "sign bit of (-0)^%d", tc.exponent)
					})
				}
			})
		}
	})
}
