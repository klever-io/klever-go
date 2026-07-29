package hostCoretest

import (
	"bytes"
	"math/big"
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

// TestManagedExecAliasDoesNotRetargetChildStorageMock is a regression test,
// using Go mock contracts, for a managed-buffer aliasing bug: a parent
// contract mutates the destination-address managed buffer passed to
// ManagedExecuteOnDestContext after the child call has already returned,
// which could retarget the child's storage update onto an unrelated victim
// account. Before the fix, managedTypesContext.GetBytes() returned the live
// internal buffer slice instead of a copy, so this post-call mutation
// retroactively corrupted the already-merged child OutputAccount's Address
// field. GetBytes now always copies (symmetric with SetBytes), so the
// retarget attempt below must have no effect.
func TestManagedExecAliasDoesNotRetargetChildStorageMock(t *testing.T) {
	victimAddress := test.MakeTestSCAddress("alias-victim")
	storageKey := []byte("ordinary-child-key")
	storageValue := []byte("child-write-test-value")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, _ interface{}) {
					parentInstance.AddMockMethod("retargetChildStorage", func() *mock.InstanceMock {
						host := parentInstance.Host
						instance := mock.GetMockInstance(host)
						managed := host.ManagedTypes()
						hooks := vmhooks.NewVMHooksImpl(host)

						destHandle := managed.NewManagedBufferFromBytes(test.ChildAddress)
						valueHandle := managed.NewBigInt(big.NewInt(0))
						functionHandle := managed.NewManagedBufferFromBytes([]byte("writeStorage"))
						argsHandle := managed.NewManagedBuffer()
						resultHandle := managed.NewManagedBuffer()

						managed.WriteManagedVecOfManagedBuffers([][]byte{storageKey, storageValue}, argsHandle)

						returnValue := hooks.ManagedExecuteOnDestContext(
							100000,
							destHandle,
							valueHandle,
							functionHandle,
							argsHandle,
							resultHandle,
						)
						require.Equal(t, int32(0), returnValue)

						// Reproduce the bug: mutate the destination buffer after the
						// call already returned. With the fix, GetBytes() inside
						// ManagedBufferSetByteSliceWithTypedArgs now operates on a private
						// copy, so this can no longer reach the merged child output account.
						retarget := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
							host,
							destHandle,
							0,
							int32(len(victimAddress)),
							victimAddress,
						)
						require.Equal(t, int32(0), retarget)

						host.Output().Finish([]byte("retargeted"))
						return instance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, _ interface{}) {
					childInstance.AddMockMethod("writeStorage", func() *mock.InstanceMock {
						host := childInstance.Host
						instance := mock.GetMockInstance(host)
						arguments := host.Runtime().Arguments()
						require.Len(t, arguments, 2)

						_, err := host.Storage().SetStorage(arguments[0], arguments[1])
						require.NoError(t, err)
						return instance
					})
				}),
			test.CreateMockContract(victimAddress).
				WithMethods(),
		).
		WithInput(
			test.CreateTestContractCallInputBuilder().
				WithRecipientAddr(test.ParentAddress).
				WithGasProvided(1_000_000).
				WithFunction("retargetChildStorage").
				Build(),
		).
		WithSetup(func(host vmhost.VMHost, _ *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte("retargeted"))

			outAcc, ok := verify.VmOutput.OutputAccounts[string(test.ChildAddress)]
			require.True(t, ok, "map key remains the originally called child")
			require.NotNil(t, outAcc)
			require.Equal(t, test.ChildAddress, outAcc.Address, "output account address must not be retargeted by a post-call buffer mutation")
			require.Contains(t, outAcc.StorageUpdates, string(storageKey))

			require.NoError(t, world.UpdateAccounts(verify.VmOutput.OutputAccounts, nil))

			childAccount, err := world.AccountsCacher.GetExistingUser(test.ChildAddress)
			require.NoError(t, err)
			childValue, err := childAccount.DataTrieTracker().RetrieveValue(storageKey)
			require.NoError(t, err)
			require.True(t, bytes.Equal(storageValue, childValue), "the child must receive its own storage update")

			_, victimHasOutput := verify.VmOutput.OutputAccounts[string(victimAddress)]
			require.False(t, victimHasOutput, "the victim account must not appear in OutputAccounts at all")
		})

	require.NoError(t, err)
}
