package builtInFunctions_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	vmconfig "github.com/klever-io/klever-go/kvm/config"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Every priced non-transfer built-in must reject insufficient gas before mutation.
// Valid parser inputs ensure this test reaches each built-in's charging boundary.
func TestBuiltIns_InsufficientGasV7(t *testing.T) {
	address := make([]byte, 32)
	address[0] = 1
	emptyMap := make([]byte, 4)
	cases := []struct {
		name      string
		arguments [][]byte
	}{
		{core.BuiltInFunctionAssetTrigger, [][]byte{{3}, []byte("TOK-1234")}},
		{core.BuiltInFunctionBuy, [][]byte{{0}, []byte("order"), []byte("KLV"), {1}}},
		{core.BuiltInFunctionCancelMarketOrder, [][]byte{[]byte("order")}},
		{core.BuiltInFunctionChangeOwnerAddress, [][]byte{make([]byte, 32)}},
		{core.BuiltInFunctionClaim, [][]byte{{0}, []byte("KLV")}},
		{core.BuiltInFunctionConfigITO, [][]byte{[]byte("TOK-1234"), address, nil, nil, nil, nil, nil, nil, nil, nil, emptyMap, emptyMap}},
		{core.BuiltInFunctionConfigMarketplace, [][]byte{[]byte("market"), []byte("name"), address, nil}},
		{core.BuiltInFunctionCreateAsset, [][]byte{nil, []byte("name"), []byte("TOK"), nil, address, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, emptyMap, make([]byte, 60)}},
		{core.BuiltInFunctionCreateMarketplace, [][]byte{[]byte("name"), address, nil}},
		{core.BuiltInFunctionCreateValidator, [][]byte{address, address, address, []byte("name"), nil, nil, nil, nil}},
		{core.BuiltInFunctionDelegate, [][]byte{address, []byte("bucket")}},
		{core.BuiltInFunctionDeposit, [][]byte{nil, []byte("id"), []byte("KLV"), {1}}},
		{core.BuiltInFunctionFreeze, [][]byte{[]byte("KLV"), {1}}},
		{core.BuiltInFunctionITOTrigger, [][]byte{{1}, []byte("TOK-1234"), nil}},
		{core.BuiltInFunctionProposal, [][]byte{{1}, []byte("description"), emptyMap}},
		{core.BuiltInFunctionSell, [][]byte{nil, []byte("market"), []byte("TOK-1234"), {1}, []byte("KLV"), {1}, {1}, {1}}},
		{core.BuiltInFunctionSetAccountName, [][]byte{[]byte("name")}},
		{core.BuiltInFunctionUndelegate, [][]byte{[]byte("bucket")}},
		{core.BuiltInFunctionUnfreeze, [][]byte{[]byte("KLV"), []byte("bucket")}},
		{core.BuiltInFunctionUnjail, nil},
		{core.BuiltInFunctionUpdateAccountPermission, [][]byte{address, emptyMap}},
		{core.BuiltInFunctionValidatorConfig, [][]byte{address, address, []byte("name"), nil, nil, nil, nil}},
		{core.BuiltInFunctionVote, [][]byte{{1}, nil, {1}}},
		{core.BuiltInFunctionWithdraw, [][]byte{nil, []byte("KLV"), {1}, nil}},
	}
	// Only fixtures that pass business validation reach the remainder assertions.
	v7Successes := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, v7 := range []bool{false, true} {
				for _, provided := range []uint64{0, 99, 100, 101} {
					t.Run(fmt.Sprintf("v7=%t/gas=%d", v7, provided), func(t *testing.T) {
						forks := mock.NewForkControllerStub()
						forks.FixAuditChangesV7Value = v7
						world := worldmock.NewMockWorld()
						gas := vmconfig.MakeGasMapForTests()
						gas["BuiltInCost"] = vmconfig.FillGasMapBuiltInCosts(100)
						require.NoError(t, world.InitBuiltinFunctions(gas, forks))
						// The world wrapper replaces its supplied fork controller. Build a fresh
						// production container with the explicit gate to exercise both sides.
						factory, err := builtInFunctions.NewBuiltInFunctionsCreator(builtInFunctions.ArgsCreateBuiltInFunctionContainer{
							GasMap: gas, Marshalizer: worldmock.WorldMarshalizer,
							AccountsCacher: world.AccountsCacher, KAppController: world.KAppController,
							MapDNSAddresses: make(map[string]struct{}), ForkController: forks,
						})
						require.NoError(t, err)
						require.NoError(t, factory.CreateBuiltInFunctionContainer())
						function, err := factory.BuiltInFunctionContainer().Get(tc.name)
						require.NoError(t, err)
						output, err := function.ProcessBuiltinFunction(&vmcommon.ContractCallInput{
							VMInput:       vmcommon.VMInput{CallerAddr: address, GasProvided: provided, Arguments: tc.arguments},
							RecipientAddr: address,
						})
						if v7 && provided < 100 {
							require.ErrorIs(t, err, common.ErrNotEnoughGas)
							require.Nil(t, output)
							require.Empty(t, world.KAppController.GetCurrentKAppContext().Receipts().Get())
						} else {
							// Business validation may reject the fixture, but charging must allow it
							// through at the exact cost, above it, and at any gas before V7.
							require.NotErrorIs(t, err, common.ErrNotEnoughGas)
							require.NotErrorIs(t, err, builtInFunctions.ErrInvalidArguments)
							if err == nil {
								require.Equal(t, vmcommon.Ok, output.ReturnCode)
								if v7 {
									v7Successes++
								}
								if provided <= 100 {
									require.Zero(t, output.GasRemaining)
								} else {
									require.Equal(t, uint64(1), output.GasRemaining)
								}
							}
						}
					})
				}
			}
		})
	}
	require.NotZero(t, v7Successes, "no fixture reached the V7 gas remainder assertions")
}
