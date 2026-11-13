package hostCoretest

import (
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	gasSchedules "github.com/klever-io/klever-go/kvm/scenarioexec/gasSchedules"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// contractGasBudget is generous on purpose: these cases measure what a call costs, so none of them
// may run out of gas on any of the paths being compared
const contractGasBudget = uint64(500_000_000)

// runContractForGas deploys one of the real test contracts, calls one of its endpoints and returns
// the gas the call consumed. fixAuditChangesV5 selects the activation path: false keeps the flag
// off, which is the pre-fork charging, true is the post-fork charging.
func runContractForGas(
	t *testing.T,
	contract string,
	function string,
	arguments [][]byte,
	fixAuditChangesV5 bool,
	gasSchedule kvmConfig.GasScheduleMap,
) uint64 {
	t.Helper()

	code := testcommon.GetTestSCCode(contract, "../../")
	blockchain := testcommon.BlockchainHookStubForContracts([]*testcommon.InstanceTestSmartContract{
		testcommon.CreateInstanceContract(testcommon.ParentAddress).
			WithCode(code).
			WithBalance(1000),
	})

	// every other flag defaults to epoch 0, so only the activation under test moves
	enableEpochs := blockchainConfig.EnableEpochs{}
	if !fixAuditChangesV5 {
		enableEpochs.FixAuditChangesV5 = 1000
	}

	host := testcommon.NewTestHostBuilder(t).
		WithBlockchainHook(blockchain).
		WithEnableEpochs(enableEpochs).
		WithGasSchedule(gasSchedule).
		Build()
	defer host.Reset()

	input := testcommon.CreateTestContractCallInputBuilder().
		WithRecipientAddr(testcommon.ParentAddress).
		WithGasProvided(contractGasBudget).
		WithFunction(function).
		WithArguments(arguments...).
		Build()

	vmOutput, err := host.RunSmartContractCall(input)
	require.NoError(t, err)
	require.Equalf(t, vmcommon.Ok, vmOutput.ReturnCode, "%s/%s: %s", contract, function, vmOutput.ReturnMessage)

	return contractGasBudget - vmOutput.GasRemaining
}

// TestRealContractGasPreForkVersusPostFork runs real compiled contracts on both sides of the
// activation and reports what the bounded-gas change costs them. The cases are split into the ones
// that build a managed buffer through repeated appends - the only shape the change can move - and
// the ones that do not, which are there to show the charge is not leaking into ordinary contracts.
//
// mBufferAppendTest(n) performs n appends of n bytes each, so the buffer it builds is n² bytes; it
// is the closest thing in the test corpus to a contract accumulating a result in a loop.
func TestRealContractGasPreForkVersusPostFork(t *testing.T) {
	// the mainnet schedule is what the numbers have to be read on: DataCopyPerByte is 50 there
	// against StorePerByte's 10000, a ratio the flat test schedule (everything priced at 1) hides
	gasSchedule, err := gasSchedules.LoadGasScheduleConfig(gasSchedules.GetV1())
	require.NoError(t, err)

	cases := []struct {
		name          string
		contract      string
		function      string
		arguments     [][]byte
		appendsInLoop bool
	}{
		{name: "managed-buffers appends 10x10B", contract: "managed-buffers", function: "mBufferAppendTest", arguments: [][]byte{{10}}, appendsInLoop: true},
		{name: "managed-buffers appends 50x50B", contract: "managed-buffers", function: "mBufferAppendTest", arguments: [][]byte{{50}}, appendsInLoop: true},
		{name: "managed-buffers appends 100x100B", contract: "managed-buffers", function: "mBufferAppendTest", arguments: [][]byte{{100}}, appendsInLoop: true},
		// the reps argument is read as a signed byte, so anything above 127 arrives negative and
		// the loop never runs
		{name: "managed-buffers appends 120x120B", contract: "managed-buffers", function: "mBufferAppendTest", arguments: [][]byte{{120}}, appendsInLoop: true},
		{name: "timeout run", contract: "timeout", function: "run", appendsInLoop: true},

		{name: "managed-buffers getBytes", contract: "managed-buffers", function: "mBufferGetBytesTest", arguments: [][]byte{{100}}},
		{name: "managed-buffers setByteSlice", contract: "managed-buffers", function: "mBufferSetByteSliceTest", arguments: [][]byte{{100}, {30}}},
		{name: "managed-buffers storageStore", contract: "managed-buffers", function: "mBufferStorageStoreTest", arguments: [][]byte{{100}}},
		{name: "managed-buffers toBigIntUnsigned", contract: "managed-buffers", function: "mBufferToBigIntUnsignedTest", arguments: [][]byte{{100}}},
		{name: "counter increment", contract: "counter", function: "increment"},
		{name: "erc20 totalSupply", contract: "erc20", function: "totalSupply"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testcommon.RaceDetectorEnabled && testCase.contract == "timeout" {
				t.Skip("the race detector slows execution past the smart contract execution timeout")
			}

			preFork := runContractForGas(t, testCase.contract, testCase.function, testCase.arguments, false, gasSchedule)
			postFork := runContractForGas(t, testCase.contract, testCase.function, testCase.arguments, true, gasSchedule)

			t.Logf("REALGAS | %-36s | pre-fork %10d | post-fork %10d | %+d (%.3fx)",
				testCase.name, preFork, postFork, int64(postFork)-int64(preFork),
				float64(postFork)/float64(preFork))

			require.GreaterOrEqual(t, postFork, preFork, "the post-fork path must never charge less")

			if !testCase.appendsInLoop {
				// These cases do not append in a loop, so nothing here moves with the change to
				// the append hooks - the deltas they do show are the other managed-buffer hooks
				// being repriced by the earlier commits of this branch (charging for the buffer
				// about to be read, before reading it), and they are per call, not per iteration.
				require.Less(t, float64(postFork)/float64(preFork), 2.0,
					"a contract that does not append in a loop must not move by more than the per-call repricing")
				return
			}

			// the accumulator copies are real work and are paid for, but geometric growth keeps
			// the total linear in the buffer's final length, so the call stays within a small
			// constant factor of what it cost before the fork
			require.Less(t, float64(postFork)/float64(preFork), 1.5,
				"a buffer built in a loop must stay within a constant factor of the pre-fork cost")
		})
	}
}

// TestGasProbeContractPreForkVersusPostFork runs the purpose-built gas-probe contract, which calls
// the managed-buffer hooks directly rather than through klever-sc, so each endpoint's hook sequence
// is exactly the shape it claims to be:
//
//   - accumulate / accumulateViaHandles build one buffer in a loop and return it, writing nothing
//     to storage. This is what ManagedVec::push and ManagedBufferBuilder generate, and it is the
//     only shape the append charge can move by more than a rounding error.
//   - readModifyWrite loads a value, re-encodes it field by field and writes it back on every
//     iteration - the mapper + custom struct pattern. Each append here sits next to a storage
//     write, and StorePerByte (10000) is 200x DataCopyPerByte (50), so the append charge disappears
//     into it.
//
// Read on the mainnet schedule, the split between the two is the whole argument about what this
// change costs deployed contracts.
func TestGasProbeContractPreForkVersusPostFork(t *testing.T) {
	gasSchedule, err := gasSchedules.LoadGasScheduleConfig(gasSchedules.GetV1())
	require.NoError(t, err)

	cases := []struct {
		name          string
		function      string
		arguments     [][]byte
		appendsInLoop bool
	}{
		{name: "accumulate 50x32B", function: "accumulate", arguments: [][]byte{{50}, {32}}, appendsInLoop: true},
		{name: "accumulate 200x32B", function: "accumulate", arguments: [][]byte{{200}, {32}}, appendsInLoop: true},
		{name: "accumulate 500x32B", function: "accumulate", arguments: [][]byte{{0x01, 0xF4}, {32}}, appendsInLoop: true},
		{name: "accumulate 1000x64B", function: "accumulate", arguments: [][]byte{{0x03, 0xE8}, {64}}, appendsInLoop: true},
		{name: "accumulateViaHandles 500x32B", function: "accumulateViaHandles", arguments: [][]byte{{0x01, 0xF4}, {32}}, appendsInLoop: true},

		{name: "readModifyWrite 10x64B struct", function: "readModifyWrite", arguments: [][]byte{{10}, {64}}},
		{name: "readModifyWrite 100x64B struct", function: "readModifyWrite", arguments: [][]byte{{100}, {64}}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			preFork := runContractForGas(t, "gas-probe", testCase.function, testCase.arguments, false, gasSchedule)
			postFork := runContractForGas(t, "gas-probe", testCase.function, testCase.arguments, true, gasSchedule)

			t.Logf("PROBEGAS | %-32s | pre-fork %11d | post-fork %11d | %+d (%.3fx)",
				testCase.name, preFork, postFork, int64(postFork)-int64(preFork),
				float64(postFork)/float64(preFork))

			require.GreaterOrEqual(t, postFork, preFork, "the post-fork path must never charge less")

			if !testCase.appendsInLoop {
				// Most of what this case does move by is not the append charge at all: it is the
				// other managed-buffer hooks being repriced by the earlier commits of this branch
				// (mBufferStorageLoad/Store charging for the buffer before touching it). Measured
				// against the unconditional accumulator charge this same case sits at 1.106x, so
				// the append change itself is worth about two points of the total here.
				require.Less(t, float64(postFork)/float64(preFork), 1.10,
					"a read-modify-write against storage is dominated by StorePerByte, not by the append")
				return
			}

			require.Less(t, float64(postFork)/float64(preFork), 1.5,
				"a buffer built in a loop must stay within a constant factor of the pre-fork cost")
		})
	}
}
