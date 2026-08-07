package hostCoretest

import (
	"testing"

	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

// TestManagedExecAliasDoesNotRetargetChildStorageWasm is a regression test,
// using REAL, compiled WASM contracts
// (kvm/test/contracts/managed-exec-alias-parent and
// kvm/test/contracts/managed-exec-alias-child), for a managed-buffer
// aliasing bug in ManagedExecuteOnDestContext.
//
// Parent contract ("managed-exec-alias-parent") calls the child's
// writeStorage endpoint through the low-level managedExecuteOnDestContext EI
// (using managed-buffer handles), then - AFTER the call returns - overwrites
// the bytes underlying the destination-address managed buffer handle with
// the victim contract's address, attempting to retarget the child's storage
// write onto the victim account.
//
// Before the fix, managedTypesContext.GetBytes() returned the live internal
// buffer slice instead of a copy, so this post-call mutation retroactively
// corrupted the already-merged child OutputAccount's Address field. GetBytes
// now always copies (symmetric with SetBytes), so the retarget attempt below
// must have no effect: the storage update must stay attached to the child.
func TestManagedExecAliasDoesNotRetargetChildStorageWasm(t *testing.T) {
	victimAddress := test.MakeTestSCAddress("alias-victim-wasm")
	storageKey := []byte("managed-exec-alias-key")

	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("managed-exec-alias-parent", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(test.ChildAddress).
				WithCode(test.GetTestSCCode("managed-exec-alias-child", "../../")).
				WithBalance(1000),
			test.CreateInstanceContract(victimAddress).
				WithCode(test.GetTestSCCode("managed-exec-alias-child", "../../")).
				WithBalance(1000),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction("retargetChildStorage").
			WithArguments(test.ChildAddress, victimAddress).
			WithGasProvided(test.GasProvided).
			Build()).
		AndAssertResults(func(host vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.Ok()

			outAcc, ok := verify.VmOutput.OutputAccounts[string(test.ChildAddress)]
			if !ok {
				t.Fatalf("expected an OutputAccount keyed by the child address, found none")
			}

			// The OutputAccount keyed by the child's address must keep the
			// child's own address: the post-call buffer mutation in the
			// parent contract must not be able to retarget it.
			require.Equal(t, test.ChildAddress, outAcc.Address,
				"OutputAccount keyed by child address must not be retargeted to the victim address")

			require.Contains(t, outAcc.StorageUpdates, string(storageKey),
				"the storage update for storageKey must stay attached to the child's own output account")

			_, victimHasOutput := verify.VmOutput.OutputAccounts[string(victimAddress)]
			require.False(t, victimHasOutput, "the victim account must not appear in OutputAccounts at all")
		})
}
