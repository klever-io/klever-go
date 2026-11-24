package hostCore_test

import (
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// There is no sub-transaction rollback in the KVM (KLC-2686). A transfer settled by a VM hook is
// written straight into the accounts cacher, and nothing undoes it when a later step of the same
// call fails. The only rollback boundary is the transaction: txProcess saves the cacher on success
// and resets it otherwise. That is sound only while every VM error path aborts the whole
// transaction, which these tests pin down for each sub-call that moves value before the callee
// runs.
//
// Most tests also record the balances while the call is still running. They show the transfer is
// still settled after the sub-call failed, which is why the abort matters. If sub-call rollback is
// ever added, those in-flight assertions are expected to change. The output state already rolls
// back with a failed transfer-and-execute (KLC-2687, transfer_execute_output_state_test.go).

const (
	noRollbackKDAAmount      = int64(1000)
	noRollbackKDAFunding     = int64(5000)
	noRollbackKLVAmount      = int64(7)
	noRollbackFollowUpGas    = int64(500)
	noRollbackFailingMethod  = "fail"
	noRollbackPayingMethod   = "payThenFail"
	noRollbackAcceptMethod   = "accept"
	noRollbackPayOutMethod   = "payOut"
	noRollbackPayUnfunded    = "payUnfunded"
	noRollbackBuiltinPay     = "builtinPay"
	noRollbackBuiltinNoFunds = "builtinPayUnfunded"
	noRollbackCallerFunction = "callChild"
)

// setupNoRollbackHost wires the real built-in functions into the mock world and zeroes the code
// and API costs these calls would otherwise charge. The built-ins take their costs from the
// host's test gas map, where every cost is 1, not from the metering schedule changed below.
func setupNoRollbackHost(t *testing.T, host vmhost.VMHost, world *worldmock.MockWorld) {
	require.NoError(t, world.InitBuiltinFunctions(host.GetGasScheduleMap(), host.ForkController()))
	host.SetBuiltInFunctionsContainer(world.BuiltinFuncs.Container)

	zeroCodeCosts(host)
	gasSchedule := host.Metering().GasSchedule()
	gasSchedule.BaseOpsAPICost.SignalError = 0
	gasSchedule.BaseOpsAPICost.ExecuteOnSameContext = 0
	gasSchedule.BaseOpsAPICost.TransferValue = 0
}

// balances holds one token's balance for the three accounts these tests move value between.
type balances struct {
	parent     int64
	child      int64
	thirdParty int64
}

func tokenBalanceOf(t *testing.T, world *worldmock.MockWorld, address []byte, token []byte) int64 {
	account, err := world.AccountsCacher.LoadUser(address)
	require.NoError(t, err)

	return account.GetBalance(token, true)
}

func balancesOf(t *testing.T, world *worldmock.MockWorld, token []byte) balances {
	return balances{
		parent:     tokenBalanceOf(t, world, test.ParentAddress, token),
		child:      tokenBalanceOf(t, world, test.ChildAddress, token),
		thirdParty: tokenBalanceOf(t, world, test.UserAddress, token),
	}
}

// childMethods gives the child a method that fails outright, one that first pays the third party
// out of the account it runs as and then fails, one that succeeds without doing anything, one that
// pays the parent back through a transfer-and-execute with no follow-up call, and one whose payback
// fails because the child holds none of the token.
func childMethods(t *testing.T) func(*mock.InstanceMock, interface{}) {
	return func(instance *mock.InstanceMock, _ interface{}) {
		instance.AddMockMethod(noRollbackAcceptMethod, func() *mock.InstanceMock {
			return instance
		})
		instance.AddMockMethod(noRollbackFailingMethod, func() *mock.InstanceMock {
			instance.Host.Runtime().SignalUserError("child failed")
			return instance
		})
		instance.AddMockMethod(noRollbackPayOutMethod, func() *mock.InstanceMock {
			transfer := &vmcommon.KDATransfer{
				KDAValue:     big.NewInt(noRollbackKLVAmount),
				KDATokenName: kdautils.KLVIdentifier,
			}
			vmhooks.TransferKDANFTExecuteWithTypedArgs(instance.Host, test.ParentAddress, []*vmcommon.KDATransfer{transfer}, 0, nil, nil)
			return instance
		})
		instance.AddMockMethod(noRollbackPayUnfunded, func() *mock.InstanceMock {
			transfer := &vmcommon.KDATransfer{
				KDAValue:     big.NewInt(1),
				KDATokenName: []byte("UNFUNDED-0000"),
			}
			vmhooks.TransferKDANFTExecuteWithTypedArgs(instance.Host, test.ParentAddress, []*vmcommon.KDATransfer{transfer}, 0, nil, nil)
			return instance
		})
		instance.AddMockMethod(noRollbackBuiltinPay, func() *mock.InstanceMock {
			builtinTransferThenCall(instance.Host, test.UserAddress2, kdautils.KLVIdentifier, noRollbackKLVAmount, "")
			return instance
		})
		instance.AddMockMethod(noRollbackBuiltinNoFunds, func() *mock.InstanceMock {
			builtinTransferThenCall(instance.Host, test.UserAddress2, []byte("UNFUNDED-0000"), 1, "")
			return instance
		})
		instance.AddMockMethod(noRollbackPayingMethod, func() *mock.InstanceMock {
			host := instance.Host
			err := host.Output().TransferValueOnly(
				test.UserAddress,
				host.Runtime().GetContextAddress(),
				big.NewInt(noRollbackKLVAmount),
				false,
			)
			require.NoError(t, err)

			host.Runtime().SignalUserError("child failed after paying")
			return instance
		})
	}
}

// runSubCall has the parent run callerBody and asserts whether the whole call succeeded. It
// returns the balances before the call, right after callerBody returned, and after the harness has
// saved (on success) or reset (on failure) the cacher as txProcess would, plus callerBody's return
// value.
func runSubCall(
	t *testing.T,
	token []byte,
	callerBody func(host vmhost.VMHost) int32,
	expectOk bool,
) (before, inFlight, after balances, returnCode int32) {
	return runSubCallWithSetup(t, token, callerBody, expectOk, nil)
}

func runSubCallWithSetup(
	t *testing.T,
	token []byte,
	callerBody func(host vmhost.VMHost) int32,
	expectOk bool,
	extraSetup func(world *worldmock.MockWorld),
) (before, inFlight, after balances, returnCode int32) {
	testConfig := &test.TestConfig{
		GasProvided:   2000,
		ParentBalance: 1000,
		ChildBalance:  1000,
	}

	var world *worldmock.MockWorld
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				// the child's payOut pays the parent back without a follow-up call
				WithCodeMetadata((&vmcommon.CodeMetadata{PayableBySC: true}).ToBytes()).
				WithMethods(func(instance *mock.InstanceMock, _ interface{}) {
					instance.AddMockMethod(noRollbackCallerFunction, func() *mock.InstanceMock {
						returnCode = callerBody(instance.Host)
						inFlight = balancesOf(t, world, token)
						return instance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(childMethods(t)),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction(noRollbackCallerFunction).
			Build()).
		WithSetup(func(host vmhost.VMHost, mockWorld *worldmock.MockWorld) {
			world = mockWorld
			setupNoRollbackHost(t, host, mockWorld)
			recordTransferReceipts(t, mockWorld)
			if extraSetup != nil {
				extraSetup(mockWorld)
			}

			caller, err := mockWorld.AccountsCacher.LoadUser(test.ParentAddress)
			require.NoError(t, err)
			require.NoError(t, caller.AddToBalance(noRollbackKDAFunding, test.KDATestTokenName, true))
			// commit the funding so the reset below restores this state rather than discarding it
			require.NoError(t, mockWorld.AccountsCacher.SaveAll())

			before = balancesOf(t, mockWorld, token)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			if expectOk {
				require.Equal(t, vmcommon.Ok, verify.VmOutput.ReturnCode)
			} else {
				require.NotEqual(t, vmcommon.Ok, verify.VmOutput.ReturnCode)
			}
			after = balancesOf(t, world, token)
		})
	require.NoError(t, err)

	return before, inFlight, after, returnCode
}

// runFailingSubCall runs callerBody, which must end in a failed sub-call, and asserts that the
// whole call failed and that the transaction-level reset restored the starting balances.
func runFailingSubCall(
	t *testing.T,
	token []byte,
	callerBody func(host vmhost.VMHost) int32,
) (before, inFlight balances, returnCode int32) {
	before, inFlight, after, returnCode := runSubCall(t, token, callerBody, false)
	require.Equal(t, before, after, "the transaction reset must restore the starting balances")

	return before, inFlight, returnCode
}

func Test_NoSubTxRollback_TransferKDANFTExecute(t *testing.T) {
	before, inFlight, returnCode := runFailingSubCall(t, test.KDATestTokenName, func(host vmhost.VMHost) int32 {
		transfer := &vmcommon.KDATransfer{
			KDAValue:     big.NewInt(noRollbackKDAAmount),
			KDATokenName: test.KDATestTokenName,
		}

		return transferToChildAndCall(host, []*vmcommon.KDATransfer{transfer}, noRollbackFailingMethod)
	})

	require.Equal(t, int32(1), returnCode)
	require.Equal(t, before.parent-noRollbackKDAAmount, inFlight.parent, "transfer still settled after the follow-up failed")
	require.Equal(t, before.child+noRollbackKDAAmount, inFlight.child)
}

func Test_NoSubTxRollback_ExecuteOnDestContextWithValue(t *testing.T) {
	before, inFlight, returnCode := runFailingSubCall(t, kdautils.KLVIdentifier, func(host vmhost.VMHost) int32 {
		return vmhooks.ExecuteOnDestContextWithTypedArgs(
			host,
			noRollbackFollowUpGas,
			big.NewInt(noRollbackKLVAmount),
			[]byte(noRollbackFailingMethod),
			test.ChildAddress,
			nil,
		)
	})

	require.Equal(t, int32(1), returnCode)
	require.Equal(t, before.parent-noRollbackKLVAmount, inFlight.parent, "call value still settled after the callee failed")
	require.Equal(t, before.child+noRollbackKLVAmount, inFlight.child)
}

// ExecuteOnSameContext runs the child's code as the parent, so the payment the child makes comes
// out of the parent's account.
func Test_NoSubTxRollback_ExecuteOnSameContext(t *testing.T) {
	before, inFlight, returnCode := runFailingSubCall(t, kdautils.KLVIdentifier, func(host vmhost.VMHost) int32 {
		return vmhooks.ExecuteOnSameContextWithTypedArgs(
			host,
			noRollbackFollowUpGas,
			big.NewInt(0),
			[]byte(noRollbackPayingMethod),
			test.ChildAddress,
			nil,
		)
	})

	require.NotZero(t, returnCode)
	require.Equal(t, before.parent-noRollbackKLVAmount, inFlight.parent, "payment still settled after the library code failed")
	require.Equal(t, before.thirdParty+noRollbackKLVAmount, inFlight.thirdParty)
}

// multiTransferThenAccept pays the child the funded amount and then 1 of secondToken in a single
// multi-transfer, followed by a call to a method that always succeeds.
func multiTransferThenAccept(secondToken []byte) func(host vmhost.VMHost) int32 {
	return func(host vmhost.VMHost) int32 {
		transfers := []*vmcommon.KDATransfer{
			{KDAValue: big.NewInt(noRollbackKDAAmount), KDATokenName: test.KDATestTokenName},
			{KDAValue: big.NewInt(1), KDATokenName: secondToken},
		}

		return transferToChildAndCall(host, transfers, noRollbackAcceptMethod)
	}
}

// A multi-transfer whose later transfer fails must abort the whole call like any other failed
// sub-call. Unlike the tests above, this one does not assert the in-flight balances: the built-in
// currently leaves the earlier transfers applied until the transaction resets, and KLC-2688 makes
// that loop all-or-nothing, so only the outcome is pinned here.
//
// Test_NoSubTxRollback_MultiTransferControlSucceeds runs the same call with the second transfer
// funded. While it passes, the unfunded second transfer is the only thing failing this one.
func Test_NoSubTxRollback_MultiTransferAbortsTransaction(t *testing.T) {
	_, _, returnCode := runFailingSubCall(t, test.KDATestTokenName, multiTransferThenAccept([]byte("UNFUNDED-0000")))

	require.Equal(t, int32(1), returnCode)
}

func Test_NoSubTxRollback_MultiTransferControlSucceeds(t *testing.T) {
	before, _, after, returnCode := runSubCall(t, test.KDATestTokenName, multiTransferThenAccept(test.KDATestTokenName), true)

	require.Zero(t, returnCode)
	require.Equal(t, before.parent-noRollbackKDAAmount-1, after.parent)
	require.Equal(t, before.child+noRollbackKDAAmount+1, after.child, "the child is credited both transfers")
}
