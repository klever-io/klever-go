package hostCoretest

import (
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

// preForkEnableEpochs returns an EnableEpochs config where every fork is active at
// epoch 0 except FixAuditChangesV2, which activates at a far-future epoch. This pins
// the test to pre-KLC-2347-fix behavior so we can assert the old buggy invariants
// without disabling unrelated forks.
func preForkEnableEpochs() blockchainConfig.EnableEpochs {
	return blockchainConfig.EnableEpochs{
		ClaimKFI:                0,
		ProcessorFlowITOPrice:   0,
		FixStakingBuckets:       0,
		KdaFpr:                  0,
		BigBucketsCompute:       0,
		FPRComputeAndKdaFeeFlow: 0,
		FixDelegationSameEpoch:  0,
		SmartContracts:          0,
		FixAuditChanges:         0,
		EpochRewardsV2:          0,
		FixAuditChangesV2:       1_000_000,
	}
}

// Test_ReadOnly_DoesNotCommitDelete and Test_ReadOnly_BlocksUpgradeDispatch are
// regression tests for KLC-2347.
//
// A contract reached via ExecuteReadOnlyWithTypedArguments must not be able to
// produce contract-delete or contract-upgrade side effects in the merged
// parent VM output, even when the read-only callee owns the target contract.
//
// Without the fix in execute(), `executeDelete` runs without checking
// runtime.ReadOnly() and appends the target to vmOutput.DeletedAccounts;
// scProcessor.processVMOutput would then commit that deletion to chain state.
func Test_ReadOnly_DoesNotCommitDelete(t *testing.T) {
	targetAddress := test.MakeTestSCAddressWithDefaultVM("readonlyDeleteTarget")

	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnlyChild", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("deleteTarget"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("deleteTarget", func() *mock.InstanceMock {
						host := childInstance.Host
						managed := host.ManagedTypes()

						destHandle := managed.NewManagedBufferFromBytes(targetAddress)
						argsHandle := managed.NewManagedBuffer()
						managed.WriteManagedVecOfManagedBuffers(nil, argsHandle)

						vmhooks.ManagedDeleteContractWithHost(host, destHandle, 100000, argsHandle)
						return childInstance
					})
				}),
			test.CreateMockContract(targetAddress).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithOwnerAddress(test.ChildAddress).
				WithMethods(),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnlyChild").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, _ *test.VMOutputVerifier) {})

	require.NoError(t, err)
	require.NotNil(t, vmOutput)
	require.NotContains(t, vmOutput.DeletedAccounts, targetAddress,
		"read-only nested execution must not commit contract-delete side effects")
	for _, logEntry := range vmOutput.Logs {
		require.NotEqual(t, []byte(vmhost.DeleteContractString), logEntry.Identifier,
			"read-only nested execution must not emit a delete-contract log")
	}
	if outAcc, ok := vmOutput.OutputAccounts[string(targetAddress)]; ok {
		require.Empty(t, outAcc.Code,
			"read-only nested execution must not mutate target contract code")
	}
}

// Test_ReadOnly_BlocksUpgradeDispatch confirms the upgrade-side leg of the
// shared dispatcher in execute() rejects read-only invocation. The parent
// invokes ExecuteReadOnlyWithTypedArguments with function = UpgradeFunctionName
// against a target it owns; the inner call must surface the read-only error in
// the parent runtime errors instead of reaching executeUpgrade.
func Test_ReadOnly_BlocksUpgradeDispatch(t *testing.T) {
	targetAddress := test.MakeTestSCAddressWithDefaultVM("readonlyUpgradeTarget")

	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("upgradeReadOnly", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte(vmhost.UpgradeFunctionName),
							targetAddress,
							[][]byte{[]byte("dummy"), {0, 0}},
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(targetAddress).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithOwnerAddress(test.ParentAddress).
				WithMethods(),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("upgradeReadOnly").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.HasRuntimeErrors(vmhost.ErrInvalidCallOnReadOnlyMode.Error())
		})

	require.NoError(t, err)
	require.NotNil(t, vmOutput)
	if outAcc, ok := vmOutput.OutputAccounts[string(targetAddress)]; ok {
		require.Empty(t, outAcc.Code,
			"read-only nested execution must not mutate target contract code on upgrade")
	}
}

// Test_ReadOnly_DeleteCommitted_PreFork is the negative-fork regression test for
// KLC-2347. It locks the fork-gating contract: with FixAuditChangesV2 disabled
// (activation epoch in the far future), the chain MUST reproduce the original
// vulnerable behavior — a read-only nested call commits the contract delete into
// vmOutput.DeletedAccounts. This guarantees that:
//
//  1. The four runtime.ReadOnly() guards in execution.go remain fork-gated and
//     cannot silently become always-on (which would diverge consensus).
//  2. A future change that drops the FixAuditChangesV2() check would be caught
//     by CI rather than at validator-rollout time.
//
// At and after the activation epoch, Test_ReadOnly_DoesNotCommitDelete asserts
// the patched (correct) behavior. The pair locks both sides of the fork.
func Test_ReadOnly_DeleteCommitted_PreFork(t *testing.T) {
	targetAddress := test.MakeTestSCAddressWithDefaultVM("readonlyDelPreFork")

	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithEnableEpochs(preForkEnableEpochs()).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnlyChild", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("deleteTarget"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("deleteTarget", func() *mock.InstanceMock {
						host := childInstance.Host
						managed := host.ManagedTypes()

						destHandle := managed.NewManagedBufferFromBytes(targetAddress)
						argsHandle := managed.NewManagedBuffer()
						managed.WriteManagedVecOfManagedBuffers(nil, argsHandle)

						vmhooks.ManagedDeleteContractWithHost(host, destHandle, 100000, argsHandle)
						return childInstance
					})
				}),
			test.CreateMockContract(targetAddress).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithOwnerAddress(test.ChildAddress).
				WithMethods(),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnlyChild").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, _ *test.VMOutputVerifier) {})

	require.NoError(t, err)
	require.NotNil(t, vmOutput)
	require.Contains(t, vmOutput.DeletedAccounts, targetAddress,
		"pre-fork (FixAuditChangesV2 disabled) must reproduce the original vulnerable behavior; "+
			"if this assertion starts failing, the read-only guards are no longer fork-gated and consensus has shifted")
}
