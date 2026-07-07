package hostCoretest

import (
	"testing"

	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

// A direct external call to the reserved "upgrade" lifecycle hook must be rejected
// by the direct-call guard (verifyAllowedFunctionCall).
func TestDirectCallToUpgradeHookIsRejected(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ChildAddress).
				WithOwner(test.UserAddress).
				WithCode(test.GetTestSCCode("counter", "../../")),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithCallerAddr(test.UserAddress2).
			WithRecipientAddr(test.ChildAddress).
			WithFunction(vmhost.ContractsUpgradeFunctionName).
			WithGasProvided(2_000_000).
			Build()).
		AndAssertResults(func(_ vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.UserError().
				ReturnMessageContains(vmhost.ErrInitFuncCalledInRun.Error())
		})
}

// KLR-07: an indirect executeOnDestContext(target, function="upgrade") must not run the
// target's "upgrade" lifecycle hook as an ordinary endpoint. Before the fix this returned
// Ok and executed the victim's upgrade() (writing COUNTER=1 out of band, bypassing
// checkUpgradePermission). It must now be rejected.
func TestIndirectExecuteOnDestContextUpgradeHookIsRejected(t *testing.T) {
	test.BuildInstanceCallTest(t).
		WithContracts(
			test.CreateInstanceContract(test.ParentAddress).
				WithCode(test.GetTestSCCode("upgrade-forwarder", "../../")),
			test.CreateInstanceContract(test.ChildAddress).
				WithOwner(test.UserAddress).
				WithCode(test.GetTestSCCode("counter", "../../")),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithCallerAddr(test.UserAddress2).
			WithRecipientAddr(test.ParentAddress).
			WithFunction("attack").
			WithGasProvided(2_000_000).
			WithArguments(test.ChildAddress).
			Build()).
		AndAssertResults(func(_ vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().
				ReturnMessageContains(vmhost.ErrInitFuncCalledInRun.Error())
		})
}
