package hostCoretest

import (
	"math/big"
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// The tests in this file lock the read-only invariant across every state-changing
// KVM host path: storage writes, value transfers, indirect deploys, and log
// writes. Delete and upgrade dispatch are covered in readonly_delete_test.go.
//
// The reporter of KLC-2347 explicitly requested that the same invariant should
// be regression-tested for delete, upgrade, storage writes, value transfers,
// and any VM output field that can later mutate chain state. These tests
// fulfill the broader ask so any future regression of read-only enforcement on
// any of these paths is caught at CI time rather than via vulnerability report.

// Test_ReadOnly_BlocksStorageWrite asserts storage.SetStorage rejects writes
// when invoked from a read-only nested execution.
func Test_ReadOnly_BlocksStorageWrite(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnly", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("writeStorage"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("writeStorage", func() *mock.InstanceMock {
						host := childInstance.Host
						_ = vmhooks.StorageStoreWithTypedArgs(host, []byte("k"), []byte("v"))
						return childInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnly").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.HasRuntimeErrors(vmhost.ErrCannotWriteOnReadOnly.Error())
		})

	require.NoError(t, err)
}

// Test_ReadOnly_DropsLogWrite asserts WriteLog is silently dropped when invoked
// from a read-only nested execution. Logs feed receipts and are part of
// consensus-visible output, so they must not appear after a read-only call.
func Test_ReadOnly_DropsLogWrite(t *testing.T) {
	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnly", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("emitLog"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("emitLog", func() *mock.InstanceMock {
						childInstance.Host.Output().WriteLog(
							test.ChildAddress,
							[][]byte{[]byte("topic")},
							[][]byte{[]byte("data")},
						)
						return childInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnly").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, _ *test.VMOutputVerifier) {})

	require.NoError(t, err)
	require.NotNil(t, vmOutput)
	require.Empty(t, vmOutput.Logs,
		"read-only nested execution must not commit log entries")
}

// Test_ReadOnly_BlocksValueTransfer asserts TransferValueOnly with positive
// value is rejected when the runtime is in read-only mode.
func Test_ReadOnly_BlocksValueTransfer(t *testing.T) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnly", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("transferKlv"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithBalance(1000).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("transferKlv", func() *mock.InstanceMock {
						host := childInstance.Host
						err := host.Output().TransferValueOnly(
							test.ParentAddress,
							test.ChildAddress,
							big.NewInt(100),
							false,
						)
						if err != nil {
							host.Runtime().AddError(err, "transferKlv")
						}
						return childInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnly").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.HasRuntimeErrors(vmhost.ErrInvalidCallOnReadOnlyMode.Error())
		})

	require.NoError(t, err)
}

// Test_ReadOnly_BlocksDeploy asserts CreateNewContract rejects deployment when
// invoked from a read-only nested execution via DeployFromSourceContract.
func Test_ReadOnly_BlocksDeploy(t *testing.T) {
	sourceAddress := test.MakeTestSCAddressWithDefaultVM("readonlyDeploySource")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, _ any) {
					parentInstance.AddMockMethod("callReadOnly", func() *mock.InstanceMock {
						host := parentInstance.Host
						_ = vmhooks.ExecuteReadOnlyWithTypedArguments(
							host,
							100000,
							[]byte("deployFromSource"),
							test.ChildAddress,
							nil,
						)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ any) {
					childInstance.AddMockMethod("deployFromSource", func() *mock.InstanceMock {
						host := childInstance.Host
						_, deployErr := vmhooks.DeployFromSourceContractWithTypedArgs(
							host,
							sourceAddress,
							[]byte{vmcommon.MetadataUpgradeable, 0},
							big.NewInt(0),
							nil,
							100000,
						)
						if deployErr != nil {
							host.Runtime().AddError(deployErr, "deployFromSource")
						}
						return childInstance
					})
				}),
			test.CreateMockContract(sourceAddress).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithMethods(),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(500000).
			WithFunction("callReadOnly").
			Build()).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.HasRuntimeErrors(vmhost.ErrInvalidCallOnReadOnlyMode.Error())
		})

	require.NoError(t, err)
}
