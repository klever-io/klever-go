package hostCoretest

import (
	"math/big"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// V6 stays enabled: V7 alone changes whether a zero-gas built-in can mutate state.
func TestExecution_BuiltinInsufficientGasV7(t *testing.T) {
	for _, tc := range []struct {
		name      string
		epochs    blockchainConfig.EnableEpochs
		forwarded int64
		cost      uint64
		succeeds  bool
	}{
		{"before V7/zero", blockchainConfig.EnableEpochs{FixAuditChangesV7: 1_000_000}, 0, 100, true},
		{"V7/zero", blockchainConfig.EnableEpochs{}, 0, 100, false},
		{"V7/below cost", blockchainConfig.EnableEpochs{}, 99, 100, false},
		{"V7/exact cost", blockchainConfig.EnableEpochs{}, 100, 100, true},
		{"V7/above cost", blockchainConfig.EnableEpochs{}, 101, 100, true},
		{"V7/zero cost", blockchainConfig.EnableEpochs{}, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var callResult int32
			_, err := test.BuildMockInstanceCallTest(t).
				WithEnableEpochs(tc.epochs).
				WithContracts(test.CreateMockContract(test.ParentAddress).WithBalance(1000).
					WithMethods(func(instance *mock.InstanceMock, _ interface{}) {
						instance.AddMockMethod("rename", func() *mock.InstanceMock {
							callResult = vmhooks.ExecuteOnDestContextWithTypedArgs(instance.Host, tc.forwarded,
								big.NewInt(0), []byte(core.BuiltInFunctionSetAccountName), test.ParentAddress,
								[][]byte{[]byte("paid-name")})
							return instance
						})
					})).
				WithInput(test.CreateTestContractCallInputBuilder().WithRecipientAddr(test.ParentAddress).
					WithGasProvided(100000).WithFunction("rename").Build()).
				WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
					require.NoError(t, world.InitBuiltinFunctions(host.GetGasScheduleMap(), host.ForkController()))
					host.SetBuiltInFunctionsContainer(world.BuiltinFuncs.Container)
					// The world wrapper shadows its fork controller; replace only our subject.
					function, err := builtInFunctions.NewKleverSetAccountNameFunc(100,
						worldmock.WorldMarshalizer, world.AccountsCacher, host.ForkController(), world.KAppController)
					require.NoError(t, err)
					function.SetNewGasConfig(&vmcommon.GasCost{BuiltInCost: vmcommon.BuiltInCost{SetAccountName: tc.cost}})
					require.NoError(t, world.BuiltinFuncs.Container.Replace(core.BuiltInFunctionSetAccountName, function))
					host.Metering().GasSchedule().BuiltInCost.SetAccountName = tc.cost
				}).
				AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
					if tc.succeeds {
						verify.Ok()
					} else {
						verify.ExecutionFailed()
					}
					account, err := world.AccountsCacher.LoadUser(test.ParentAddress)
					require.NoError(t, err)
					receipts := world.KAppController.GetCurrentKAppContext().Receipts().Get()
					if tc.succeeds {
						require.Zero(t, callResult)
						require.Equal(t, []byte("paid-name"), account.GetName())
						require.Len(t, receipts, 1)
					} else {
						require.NotZero(t, callResult)
						require.Empty(t, account.GetName())
						require.Empty(t, receipts)
					}
				})
			require.NoError(t, err)
		})
	}
}
