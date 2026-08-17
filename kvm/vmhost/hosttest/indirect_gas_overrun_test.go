package hostCoretest

import (
	"math/big"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Regression tests for KLR-46.
//
// Wasmer's metering only traps at basic-block boundaries, so a VM hook charging
// through unbounded UseGas can leave an instance with more points used than the
// gas it was forwarded, and the contract can return before any trap fires.
// checkFinalGasAfterExit() catches that on exit. Indirect plain calls,
// execute() -> callSCMethodIndirect(), were the one path not running it; the
// indirect delete and upgrade legs already did, ungated. The fix adds it for
// plain calls, gated on FixAuditChangesV4.
//
// These tests mirror TestGasUsed_TwoContracts_ExecuteOnSameCtx in
// execution_gas_test.go: same makeTestConfig() values, same verifier assertions.
// Test_..._Baseline reproduces that test's exact numbers through this file's
// harness, so the numbers below can be read against a known-good anchor.

var overrunStorageKey = []byte("overrunKey")
var overrunStorageValue = []byte{42}

// indirectCall is the shape shared by ExecuteOnSameContextInMockContracts and
// ExecuteOnDestContextInMockContracts.
type indirectCall func(vmhost.VMHost, *vmcommon.ContractCallInput, *big.Int) int32

// Calling the host directly keeps the
// parent alive past the failed call, which is the only way to observe whether
// ExecuteOnSameContext rolled the child's state back or committed it.
func executeOnSameContextDirect(host vmhost.VMHost, input *vmcommon.ContractCallInput, _ *big.Int) int32 {
	if err := host.ExecuteOnSameContext(input); err != nil {
		return -1
	}

	return 0
}

// preForkV4EnableEpochs activates every fork at epoch 0 except FixAuditChangesV4,
// pinning the test to pre-KLR-46-fix behavior.
func preForkV4EnableEpochs() *blockchainConfig.EnableEpochs {
	return &blockchainConfig.EnableEpochs{FixAuditChangesV4: 1_000_000}
}

// postForkV4EnableEpochs activates FixAuditChangesV4. Stated explicitly rather
// than relying on the zero value: newTestHostBuilder's default EnableEpochs
// literal enumerates flags one by one and does not mention V4, so post-fork
// tests would otherwise inherit their subject silently.
func postForkV4EnableEpochs() *blockchainConfig.EnableEpochs {
	return &blockchainConfig.EnableEpochs{FixAuditChangesV4: 0}
}

// wellBehavedChild spends part of its forwarded gas, like WasteGasChildMock.
func wellBehavedChild(gasUsed uint64) func(vmhost.VMHost) {
	return func(host vmhost.VMHost) {
		writeChildMarker(host)
		if err := host.Metering().UseGasBounded(gasUsed); err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
		}
	}
}

// childSpendingBudgetPlus burns its whole forwarded budget plus extra through
// unbounded UseGas and returns normally, which is what a hook overshooting
// between two metering points does to a real instance. extra=0 is the boundary
// case: it spends exactly its budget and must still be accepted, since
// checkFinalGasAfterExit compares with a strict > rather than >=.
func childSpendingBudgetPlus(extra uint64) func(vmhost.VMHost) {
	return func(host vmhost.VMHost) {
		writeChildMarker(host)
		host.Metering().UseGas(host.Metering().GetGasForExecution() + extra)
	}
}

func writeChildMarker(host vmhost.VMHost) {
	_, _ = host.Storage().SetStorage(overrunStorageKey, overrunStorageValue)
}

// runIndirectCall wires a parent that spends GasUsedByParent and then makes one
// indirect call forwarding GasProvidedToChild, matching ExecOnSameCtxParentMock's
// gas behavior. The parent ignores the hook's return value so that a rejected call
// surfaces its own error rather than being masked by a FailExecution("return value
// -1") from the caller.
func runIndirectCall(
	t *testing.T,
	execute indirectCall,
	childBody func(vmhost.VMHost),
	epochs *blockchainConfig.EnableEpochs,
	assertResults test.AssertResultsFunc,
) {
	testConfig := makeTestConfig()

	callerTest := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("execIndirect", func() *mock.InstanceMock {
						host := parentInstance.Host
						if err := host.Metering().UseGasBounded(testConfig.GasUsedByParent); err != nil {
							host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
							return parentInstance
						}

						input := test.DefaultTestContractCallInput()
						input.CallerAddr = parentInstance.Address
						input.RecipientAddr = test.ChildAddress
						input.Function = "childCall"
						input.GasProvided = testConfig.GasProvidedToChild

						execute(host, input, big.NewInt(0))
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(testConfig.ChildBalance).
				WithConfig(testConfig).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("childCall", func() *mock.InstanceMock {
						childBody(childInstance.Host)
						return childInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("execIndirect").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		WithEnableEpochs(*epochs)

	vmOutput, err := callerTest.AndAssertResults(assertResults)
	require.NoError(t, err)
	require.NotNil(t, vmOutput)
}

// childMarkerOn is the storage assertion for a committed child marker, on
// whichever account it lands (the parent's, for same-context).
func childMarkerOn(address []byte) test.StoreEntry {
	return test.CreateStoreEntry(address).WithKey(overrunStorageKey).WithValue(overrunStorageValue)
}

// Test_IndirectGasOverrun_SameContext_Baseline anchors this file to the existing
// gas suite. A well-behaved child must produce exactly the numbers asserted by
// TestGasUsed_TwoContracts_ExecuteOnSameCtx with numCalls=1: the child's gas is
// billed to the parent (same-context forwards then refunds the unused remainder),
// and the child account itself is charged nothing.
func Test_IndirectGasOverrun_SameContext_Baseline(t *testing.T) {
	testConfig := makeTestConfig()
	expectGasUsedByParent := testConfig.GasUsedByParent + testConfig.GasUsedByChild

	runIndirectCall(t, contracts.ExecuteOnSameContextInMockContracts,
		wellBehavedChild(testConfig.GasUsedByChild), postForkV4EnableEpochs(),
		func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasRemaining(testConfig.GasProvided-expectGasUsedByParent).
				GasUsed(test.ParentAddress, expectGasUsedByParent).
				GasUsed(test.ChildAddress, 0)
		})
}

// Test_IndirectGasOverrun_SameContext_ExactBudget pins the comparison in
// checkFinalGasAfterExit as strict >. A child that spends its forwarded gas down
// to the last unit is well-behaved and must still be accepted; flipping the check
// to >= would revert it, and every other test in this file would still pass.
func Test_IndirectGasOverrun_SameContext_ExactBudget(t *testing.T) {
	testConfig := makeTestConfig()
	expectGasUsedByParent := testConfig.GasUsedByParent + testConfig.GasProvidedToChild

	runIndirectCall(t, contracts.ExecuteOnSameContextInMockContracts,
		childSpendingBudgetPlus(0), postForkV4EnableEpochs(),
		func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				GasRemaining(testConfig.GasProvided-expectGasUsedByParent).
				GasUsed(test.ParentAddress, expectGasUsedByParent).
				GasUsed(test.ChildAddress, 0).
				Storage(childMarkerOn(test.ParentAddress))
		})
}

// Test_IndirectGasOverrun_SameContext_PreFork is the vulnerability, stated in the
// gas suite's own vocabulary. Against the baseline above, the only thing that
// changes is that the child spends more than the 300 it was forwarded - yet the
// parent is billed exactly 400+300, the transaction balances to GasProvided and
// returns Ok, and the child's storage write is committed.
//
// The expected numbers are identical for an overrun of 1 and of 100000: whatever
// the child spends past its budget is absorbed, never billed. That is the leak.
//
// This also locks the fork gate - dropping it would diverge consensus.
func Test_IndirectGasOverrun_SameContext_PreFork(t *testing.T) {
	testConfig := makeTestConfig()
	// the parent pre-pays the whole forwarded amount and RestoreGas returns the
	// child's unused gas; on an overrun that remainder is clamped to zero
	expectGasUsedByParent := testConfig.GasUsedByParent + testConfig.GasProvidedToChild

	for _, overrunBy := range []uint64{1, 100000} {
		runIndirectCall(t, contracts.ExecuteOnSameContextInMockContracts,
			childSpendingBudgetPlus(overrunBy), preForkV4EnableEpochs(),
			func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok().
					GasRemaining(testConfig.GasProvided-expectGasUsedByParent).
					GasUsed(test.ParentAddress, expectGasUsedByParent).
					GasUsed(test.ChildAddress, 0).
					Storage(childMarkerOn(test.ParentAddress))
			})
	}
}

func Test_IndirectGasOverrun_SameContext_Rejected(t *testing.T) {
	runIndirectCall(t, contracts.ExecuteOnSameContextInMockContracts,
		childSpendingBudgetPlus(1), postForkV4EnableEpochs(),
		func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.OutOfGas().
				HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error()).
				GasRemaining(0).
				Storage()
		})
}

// Here the parent survives the rejected call, so the child's leftovers are visible:
// the overrun is billed as the full GasProvidedToChild with nothing restored, and
// the child's storage write is gone.
func Test_IndirectGasOverrun_SameContext_RolledBack(t *testing.T) {
	testConfig := makeTestConfig()
	// the forwarded gas is pre-paid by the parent and, the call having failed,
	// never restored - the overrun itself is not billed to anyone
	expectGasUsedByParent := testConfig.GasUsedByParent + testConfig.GasProvidedToChild

	for _, overrunBy := range []uint64{1, 100000} {
		runIndirectCall(t, executeOnSameContextDirect,
			childSpendingBudgetPlus(overrunBy), postForkV4EnableEpochs(),
			func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok().
					HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error()).
					GasRemaining(testConfig.GasProvided-expectGasUsedByParent).
					GasUsed(test.ParentAddress, expectGasUsedByParent).
					Storage()
			})
	}
}

// pins the reclassification on the dest-context leg of execute()
func Test_IndirectGasOverrun_DestContext_Rejected(t *testing.T) {
	runIndirectCall(t, contracts.ExecuteOnDestContextInMockContracts,
		childSpendingBudgetPlus(1), postForkV4EnableEpochs(),
		func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.OutOfGas().
				HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error()).
				GasRemaining(0).
				Storage()
		})
}

func Test_IndirectGasOverrun_DestContext_PreFork(t *testing.T) {
	runIndirectCall(t, contracts.ExecuteOnDestContextInMockContracts,
		childSpendingBudgetPlus(1), preForkV4EnableEpochs(),
		func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				HasRuntimeErrors(vmhost.ErrInputAndOutputGasDoesNotMatch.Error()).
				GasRemaining(0).
				Storage()
		})
}
