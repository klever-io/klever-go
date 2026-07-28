package hostCoretest

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

// TestNestedExecutionTimeoutLeavesNoLeakedInstances drives a same-code nested execution
// that runs until the VM timeout, the shape that previously raced the instance cleanup
// path against the still-unwinding slow-hook worker goroutine and could destroy the
// native instance while it was still executing (SIGSEGV / use-after-free).
//
// The test asserts only the two invariants it can actually observe once the timeout
// path joins the worker before unwinding: the call fails with the timeout error, and
// no wasmer instance is leaked afterwards (ValidateInstances). It does not assert the
// absence of the crash directly, because that failure mode aborts the whole test binary
// rather than failing a single case; the invariants below hold precisely because the
// worker is now joined before CleanInstance runs. The loop repeats to keep exercising
// the timing window that made the crash probabilistic.
func TestNestedExecutionTimeoutLeavesNoLeakedInstances(t *testing.T) {
	code := testcommon.GetTestSCCode("nested-timeout", "../../")

	const attempts = 15
	for range attempts {
		blockchain := testcommon.BlockchainHookStubForContracts([]*testcommon.InstanceTestSmartContract{
			testcommon.CreateInstanceContract(testcommon.ParentAddress).
				WithCode(code).
				WithBalance(1000),
		})

		host := testcommon.NewTestHostBuilder(t).
			WithBlockchainHook(blockchain).
			Build()

		input := testcommon.CreateTestContractCallInputBuilder().
			WithRecipientAddr(testcommon.ParentAddress).
			WithGasProvided(1_000_000_000).
			WithFunction("nestedForever").
			Build()

		_, err := host.RunSmartContractCall(input)
		require.ErrorIs(t, err, vmhost.ErrExecutionFailedWithTimeout)
		require.NoError(t, host.Runtime().ValidateInstances())

		host.Reset()
	}
}
