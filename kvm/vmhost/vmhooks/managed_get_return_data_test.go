package vmhooks_test

import (
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Copying a return item into a managed buffer through managedGetReturnData.
//
// Pre-fork the hook charges the flat GetReturnData cost through a charge that cannot fail and then
// copies the whole item, into a new handle whenever resultHandle has none, so one item paid once
// through finish can be copied into any number of handles for the flat cost each. Post-fork
// (FixAuditChangesV6) the flat cost is bounded, a new handle costs MBufferNewFromBytes and the
// item's bytes are paid, bounded, before anything is copied.

// existingHandleBytes is what an existing result handle holds before the calls.
var existingHandleBytes = []byte("keep")

// returnDataCall is what the managedGetReturnData calls left behind, plus the gas costs they ran on.
type returnDataCall struct {
	getReturnDataCost       uint64
	mBufferNewFromBytesCost uint64
	// resultLens is the length of each result handle after the calls, -1 when it has no buffer
	resultLens []int32
}

// prepareGetReturnData finishes one itemLen-byte return item and copies it, through
// managedGetReturnData, into each of resultHandles in turn. reuseHandle makes resultHandles[0] an
// existing buffer holding existingHandleBytes before the calls, so its copy makes no new handle. The
// calls stop at the first one that leaves no buffer, as a contract's execution stops at the hook's
// fault. The flat cost and the new-handle cost are pinned to their distinct mainnet prices, so an
// assertion cannot pass with one charged in place of the other.
func prepareGetReturnData(itemLen int, resultHandles []int32, reuseHandle bool, call *returnDataCall) func(vmhost.VMHost) func() {
	return func(host vmhost.VMHost) func() {
		gasSchedule := host.Metering().GasSchedule()
		gasSchedule.BaseOpsAPICost.GetReturnData = 100
		gasSchedule.ManagedBufferAPICost.MBufferNewFromBytes = 2000
		call.getReturnDataCost = gasSchedule.BaseOpsAPICost.GetReturnData
		call.mBufferNewFromBytesCost = gasSchedule.ManagedBufferAPICost.MBufferNewFromBytes

		managedType := host.ManagedTypes()
		if reuseHandle {
			resultHandles[0] = managedType.NewManagedBufferFromBytes(existingHandleBytes)
		}

		output := host.Output()
		output.Finish(make([]byte, itemLen))
		// #nosec G115
		resultID := int32(len(output.ReturnData()) - 1)

		hooks := vmhooks.NewVMHooksImpl(host)
		return func() {
			for _, resultHandle := range resultHandles {
				hooks.ManagedGetReturnData(resultID, resultHandle)
				if managedType.GetLength(resultHandle) < 0 {
					break
				}
			}
			call.resultLens = make([]int32, len(resultHandles))
			for i, resultHandle := range resultHandles {
				call.resultLens[i] = managedType.GetLength(resultHandle)
			}
		}
	}
}

// unusedHandles returns count handles no buffer has, well above the ones the test makes.
func unusedHandles(count int) []int32 {
	handles := make([]int32, count)
	for i := range handles {
		handles[i] = int32(1_000_000 + i) // #nosec G115
	}
	return handles
}

func TestManagedGetReturnData_PreForkKeepsTheLegacyGas(t *testing.T) {
	const gasLeft = uint64(10_000_000)
	const numHandles, itemLen = 1000, 1024

	var call returnDataCall
	pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(numHandles), false, &call))

	t.Logf("pre-fork %s gasUsed=%d allocated=%d", pre.returnCode, pre.gasUsed, pre.allocated)

	// the flat cost per call and nothing else, whatever the item's size or the handle being new
	require.Equal(t, vmcommon.Ok, pre.returnCode)
	require.Equal(t, numHandles*call.getReturnDataCost, pre.gasUsed)
	for _, resultLen := range call.resultLens {
		require.Equal(t, int32(itemLen), resultLen)
	}
}

func TestManagedGetReturnData_PostForkChargesEachHandleAndByte(t *testing.T) {
	const gasLeft = uint64(10_000_000)
	const numHandles, itemLen = 100, 1024

	var call returnDataCall
	post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(numHandles), false, &call))

	t.Logf("post-fork %s gasUsed=%d", post.returnCode, post.gasUsed)

	require.Equal(t, vmcommon.Ok, post.returnCode)
	perCopy := call.getReturnDataCost + call.mBufferNewFromBytesCost + itemLen*mainnetDataCopyPerByte
	require.Equal(t, numHandles*perCopy, post.gasUsed)
	for _, resultLen := range call.resultLens {
		require.Equal(t, int32(itemLen), resultLen)
	}
}

func TestManagedGetReturnData_PostForkExistingHandleIsNotChargedAsNew(t *testing.T) {
	const gasLeft = uint64(10_000_000)
	const itemLen = 1024

	var call returnDataCall
	post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, make([]int32, 1), true, &call))

	require.Equal(t, vmcommon.Ok, post.returnCode)
	require.Equal(t, call.getReturnDataCost+itemLen*mainnetDataCopyPerByte, post.gasUsed)
	require.Equal(t, []int32{itemLen}, call.resultLens)
}

func TestManagedGetReturnData_PostForkUnderfundedFlatCostFails(t *testing.T) {
	// less gas left than the flat cost alone, copying an empty item into an existing handle, so
	// only the flat charge can fail
	const gasLeft = uint64(50)

	var preCall, postCall returnDataCall
	pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(0, make([]int32, 1), true, &preCall))
	post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(0, make([]int32, 1), true, &postCall))

	t.Logf("pre-fork %s gasUsed=%d, post-fork %s %q gasUsed=%d",
		pre.returnCode, pre.gasUsed, post.returnCode, post.returnMessage, post.gasUsed)

	// pre-fork the flat cost is overdrawn and the empty item still overwrites the handle
	require.Equal(t, preCall.getReturnDataCost, pre.gasUsed)
	require.Equal(t, []int32{0}, preCall.resultLens)

	// post-fork the flat charge fails without overdrawing and the handle is left as it was
	require.Equal(t, vmcommon.VMOutOfGas, post.returnCode)
	require.Contains(t, post.returnMessage, vmhost.ErrNotEnoughGas.Error())
	require.LessOrEqual(t, post.gasUsed, gasLeft)
	require.Equal(t, []int32{int32(len(existingHandleBytes))}, postCall.resultLens)
}

func TestManagedGetReturnData_PostForkUnderfundedNewHandleChargeFails(t *testing.T) {
	// an empty item copied into an unused handle costs the flat cost plus the new handle and no
	// bytes, so with one gas less than that only the new-handle charge can fail
	const exactGas = uint64(100 + 2000)

	var underfundedCall, fundedCall returnDataCall
	underfunded := runManagedHook(t, postForkV6EnableEpochs(), exactGas-1,
		prepareGetReturnData(0, unusedHandles(1), false, &underfundedCall))
	funded := runManagedHook(t, postForkV6EnableEpochs(), exactGas,
		prepareGetReturnData(0, unusedHandles(1), false, &fundedCall))

	t.Logf("underfunded %s %q gasUsed=%d, funded %s gasUsed=%d",
		underfunded.returnCode, underfunded.returnMessage, underfunded.gasUsed, funded.returnCode, funded.gasUsed)

	require.Equal(t, exactGas, underfundedCall.getReturnDataCost+underfundedCall.mBufferNewFromBytesCost)

	// the new-handle charge fails without overdrawing and no handle is made
	require.Equal(t, vmcommon.VMOutOfGas, underfunded.returnCode)
	require.Contains(t, underfunded.returnMessage, vmhost.ErrNotEnoughGas.Error())
	require.LessOrEqual(t, underfunded.gasUsed, exactGas-1)
	require.Equal(t, []int32{-1}, underfundedCall.resultLens)

	// one more gas pays for it, so the failure above is the new-handle charge and nothing before it
	require.Equal(t, vmcommon.Ok, funded.returnCode)
	require.Equal(t, exactGas, funded.gasUsed)
	require.Equal(t, []int32{0}, fundedCall.resultLens)
}

func TestManagedGetReturnData_PostForkUnderfundedCopyFails(t *testing.T) {
	// one 1 MiB item needs 52M gas post-fork, over five times what the parent has left
	const gasLeft = uint64(10_000_000)
	const itemLen = 1024 * 1024

	var preCall, postCall returnDataCall
	pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(1), false, &preCall))
	post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(1), false, &postCall))

	t.Logf("pre-fork %s allocated=%d, post-fork %s %q allocated=%d",
		pre.returnCode, pre.allocated, post.returnCode, post.returnMessage, post.allocated)

	// pre-fork the item is copied into a new handle for the flat cost
	require.Equal(t, vmcommon.Ok, pre.returnCode)
	require.Equal(t, []int32{itemLen}, preCall.resultLens)
	require.GreaterOrEqual(t, pre.allocated, uint64(itemLen))

	// post-fork the copy fails before the item is copied or the handle is made
	require.Equal(t, vmcommon.VMOutOfGas, post.returnCode)
	require.Contains(t, post.returnMessage, vmhost.ErrNotEnoughGas.Error())
	require.Equal(t, []int32{-1}, postCall.resultLens)
	require.Less(t, post.allocated, uint64(itemLen))
}

func TestManagedGetReturnData_PostForkReusedItemIsBoundedByGas(t *testing.T) {
	// the attack shape: one 100 KiB item copied into many new handles. Pre-fork 1000 copies cost the
	// flat cost each and hold ~100 MB; post-fork each costs ~5.1M on the item's bytes alone, so the
	// gas left pays for under two of them
	const gasLeft = uint64(10_000_000)
	const itemLen = 100 * 1024
	numHandles := 1000
	if test.RaceDetectorEnabled {
		numHandles = 100
	}

	var preCall, postCall returnDataCall
	pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(numHandles), false, &preCall))
	post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft,
		prepareGetReturnData(itemLen, unusedHandles(numHandles), false, &postCall))

	t.Logf("pre-fork %s gasUsed=%d allocated=%d, post-fork %s allocated=%d",
		pre.returnCode, pre.gasUsed, pre.allocated, post.returnCode, post.allocated)

	require.Equal(t, vmcommon.Ok, pre.returnCode)
	require.Equal(t, uint64(numHandles)*preCall.getReturnDataCost, pre.gasUsed)
	require.GreaterOrEqual(t, pre.allocated, uint64(numHandles*itemLen))

	require.Equal(t, vmcommon.VMOutOfGas, post.returnCode)
	perCopy := postCall.getReturnDataCost + postCall.mBufferNewFromBytesCost + itemLen*mainnetDataCopyPerByte
	paidCopies := int(gasLeft / perCopy)
	for i, resultLen := range postCall.resultLens {
		if i < paidCopies {
			require.Equal(t, int32(itemLen), resultLen, i)
		} else {
			require.Equal(t, int32(-1), resultLen, i)
		}
	}
	require.Less(t, post.allocated, uint64((paidCopies+1)*itemLen))
}

// prepareGetReturnDataOutOfRange finishes one return item and asks managedGetReturnData for the
// item at resultIDOf(number of return items), which is out of range, into resultHandle. resultLen
// is the length of resultHandle after the call, -1 when it has no buffer.
func prepareGetReturnDataOutOfRange(
	resultIDOf func(numItems int32) int32,
	resultHandle int32,
	resultLen *int32,
) func(vmhost.VMHost) func() {
	return func(host vmhost.VMHost) func() {
		output := host.Output()
		output.Finish(make([]byte, 1024))
		// #nosec G115
		resultID := resultIDOf(int32(len(output.ReturnData())))

		managedType := host.ManagedTypes()
		hooks := vmhooks.NewVMHooksImpl(host)
		return func() {
			hooks.ManagedGetReturnData(resultID, resultHandle)
			*resultLen = managedType.GetLength(resultHandle)
		}
	}
}

func TestManagedGetReturnData_OutOfRangeResultIDFails(t *testing.T) {
	// the range check has to run before the post-fork charges, which read the item at resultID
	const gasLeft = uint64(10_000_000)

	forks := map[string]blockchainConfig.EnableEpochs{
		"pre-fork":  preForkV6EnableEpochs(),
		"post-fork": postForkV6EnableEpochs(),
	}
	resultIDs := map[string]func(numItems int32) int32{
		"past the end": func(numItems int32) int32 { return numItems },
		"negative":     func(int32) int32 { return -1 },
	}

	for forkName, epochs := range forks {
		for resultIDName, resultIDOf := range resultIDs {
			t.Run(forkName+" "+resultIDName, func(t *testing.T) {
				resultLen := int32(0)
				run := runManagedHook(t, epochs, gasLeft,
					prepareGetReturnDataOutOfRange(resultIDOf, unusedHandles(1)[0], &resultLen))

				t.Logf("%s %q gasUsed=%d", run.returnCode, run.returnMessage, run.gasUsed)

				// the call fails on the index, nothing is copied and no handle is made
				require.Equal(t, vmcommon.VMExecutionFailed, run.returnCode)
				require.Contains(t, run.returnMessage, vmhost.ErrArgOutOfRange.Error())
				require.Equal(t, int32(-1), resultLen)
				require.LessOrEqual(t, run.gasUsed, gasLeft)
			})
		}
	}
}
