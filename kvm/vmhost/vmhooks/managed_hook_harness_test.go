package vmhooks_test

import (
	"math/big"
	"runtime"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// The harness the FixAuditChangesV6 decode tests share: one hook call made from a contract, on
// both sides of the fork, measured on its own.

const mainnetDataCopyPerByte = 50

// testMBufferNewFromBytes is MBufferNewFromBytes in the test gas schedule the harness runs on
const testMBufferNewFromBytes = 1

// managedDeployMetadataCopy is what ManagedCreateContract and ManagedDeployFromSourceContract pay
// post-fork for the code-metadata buffer they clone, on the two-byte upgradeableMetadata both
// tables pass them. ManagedUpgradeFromSourceContract pays the same; ManagedUpgradeContract pays for its code too
const managedDeployMetadataCopy = 2 * mainnetDataCopyPerByte

// managedHookChildFunction is the endpoint of the child contract the calling hooks target.
const managedHookChildFunction = "deposit"

// managedHookEmptyItemsFunction is both the child endpoint that emits empty return items and the
// marker that makes the child's init and upgrade emit them: called with the arguments
// [managedHookEmptyItemsFunction, n], any of the three finishes n empty return items instead of
// what it would otherwise return. n is read as a big-endian unsigned integer.
const managedHookEmptyItemsFunction = "emitEmptyItems"

// managedHookFailAfterItemsFunction is managedHookEmptyItemsFunction for a child that fails with
// managedHookChildFailure once its items are finished.
const managedHookFailAfterItemsFunction = "emitEmptyItemsAndFail"

// managedHookChildFailure is the error of a child asked for managedHookFailAfterItemsFunction.
const managedHookChildFailure = "child failed after its return items"

// finishEmptyItemsIfAsked finishes the empty return items the arguments ask for, and reports
// whether they asked.
func finishEmptyItemsIfAsked(host vmhost.VMHost) bool {
	arguments := host.Runtime().Arguments()
	if len(arguments) != 2 {
		return false
	}
	function := string(arguments[0])
	if function != managedHookEmptyItemsFunction && function != managedHookFailAfterItemsFunction {
		return false
	}
	numItems := new(big.Int).SetBytes(arguments[1]).Uint64()
	for i := uint64(0); i < numItems; i++ {
		host.Output().Finish(nil)
	}
	if function == managedHookFailAfterItemsFunction {
		host.Runtime().SignalUserError(managedHookChildFailure)
	}
	return true
}

// childMethodMock adds a child method that finishes the empty items it is asked for, and otherwise
// finishes defaultReturn, as contracts.InitFunctionMock and contracts.UpgradeFunctionMock do.
func childMethodMock(name string, defaultReturn []byte) func(*mock.InstanceMock, any) {
	return func(childInstance *mock.InstanceMock, _ any) {
		childInstance.AddMockMethod(name, func() *mock.InstanceMock {
			host := childInstance.Host
			if !finishEmptyItemsIfAsked(host) && defaultReturn != nil {
				host.Output().Finish(defaultReturn)
			}
			return childInstance
		})
	}
}

// managedBuffer, managedZero and newHooks build hook arguments and hooks without paying gas.
func managedBuffer(host vmhost.VMHost, data []byte) int32 {
	return host.ManagedTypes().NewManagedBufferFromBytes(data)
}

func managedZero(host vmhost.VMHost) int32 {
	return host.ManagedTypes().NewBigInt(big.NewInt(0))
}

func newHooks(host vmhost.VMHost) *vmhooks.VMHooksImpl { return vmhooks.NewVMHooksImpl(host) }

func preForkV6EnableEpochs() blockchainConfig.EnableEpochs {
	return blockchainConfig.EnableEpochs{FixAuditChangesV5: 0, FixAuditChangesV6: 1_000_000}
}

func postForkV6EnableEpochs() blockchainConfig.EnableEpochs {
	return blockchainConfig.EnableEpochs{FixAuditChangesV5: 0, FixAuditChangesV6: 0}
}

// managedHookRun is what one hook call did, plus how the whole execution ended.
type managedHookRun struct {
	gasUsed       uint64
	allocated     uint64
	returnCode    vmcommon.ReturnCode
	returnMessage string
}

// runManagedHook runs one hook call from the parent contract with gasLeftBeforeHook gas left, on
// the mainnet DataCopyPerByte, and measures the call alone. prepare builds the buffers the hook
// reads, directly in the managed types context and so without gas, and returns the call. The
// parent holds test.KDATestTokenName to pay with; the child exposes managedHookChildFunction,
// init and upgrade, is owned by the parent and is upgradeable, so every calling hook has a target.
func runManagedHook(
	t *testing.T,
	epochs blockchainConfig.EnableEpochs,
	gasLeftBeforeHook uint64,
	prepare func(host vmhost.VMHost) func(),
) managedHookRun {
	return runManagedHookInWorld(t, epochs, gasLeftBeforeHook, nil, prepare)
}

// runManagedHookInWorld is runManagedHook with setupWorld, when not nil, run on the host and the
// world once the world's built-in functions are in place.
func runManagedHookInWorld(
	t *testing.T,
	epochs blockchainConfig.EnableEpochs,
	gasLeftBeforeHook uint64,
	setupWorld func(host vmhost.VMHost, world *worldmock.MockWorld),
	prepare func(host vmhost.VMHost) func(),
) managedHookRun {
	var run managedHookRun

	_, err := test.BuildMockInstanceCallTest(t).
		WithEnableEpochs(epochs).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callHook", func() *mock.InstanceMock {
						host := parentInstance.Host
						metering := host.Metering()
						metering.GasSchedule().BaseOperationCost.DataCopyPerByte = mainnetDataCopyPerByte

						callHook := prepare(host)

						metering.UseGas(metering.GasLeft() - gasLeftBeforeHook)
						pointsBefore := host.Runtime().GetPointsUsed()

						var before, after runtime.MemStats
						runtime.GC()
						runtime.ReadMemStats(&before)

						callHook()

						runtime.ReadMemStats(&after)
						run.allocated = after.TotalAlloc - before.TotalAlloc
						run.gasUsed = host.Runtime().GetPointsUsed() - pointsBefore

						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(1000).
				WithOwnerAddress(test.ParentAddress).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithMethods(
					childMethodMock(vmhost.InitFunctionName, []byte(vmhost.InitFunctionName)),
					childMethodMock(vmhost.ContractsUpgradeFunctionName, []byte(vmhost.UpgradeFunctionName)),
					childMethodMock(managedHookEmptyItemsFunction, nil),
					childMethodMock(managedHookFailAfterItemsFunction, nil),
					func(childInstance *mock.InstanceMock, _ any) {
						childInstance.AddMockMethod(managedHookChildFunction, func() *mock.InstanceMock { return childInstance })
					}),
		).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			require.NoError(t, world.InitBuiltinFunctions(host.GetGasScheduleMap(), host.ForkController()))
			caller, err := world.AccountsCacher.LoadUser(test.ParentAddress)
			require.NoError(t, err)
			require.NoError(t, caller.AddToBalance(1000, test.KDATestTokenName, true))
			if setupWorld != nil {
				setupWorld(host, world)
			}
		}).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(gasLeftBeforeHook + 1_000_000).
			WithFunction("callHook").
			Build()).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			run.returnCode = verify.VmOutput.ReturnCode
			run.returnMessage = verify.VmOutput.ReturnMessage
		})
	require.NoError(t, err)

	return run
}
