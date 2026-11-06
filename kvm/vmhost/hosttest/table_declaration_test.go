package hostCoretest

import (
	"encoding/hex"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/kvm/config"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/wasmbytes"
	"github.com/klever-io/klever-go/kvm/wasmer2"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// tableGrowOversizedNonMutatingWasmHex declares a funcref table with a declared
// maximum of 200,000,000 (limits flag = 1, so it does have a maximum - it is
// bounded, just absurdly so) and minimum 0, and exports empty "init" and "main".
// It exercises the "has a maximum, but it's still too large" branch of the
// check, not the "no maximum at all" branch: see tableGrowUnboundedWasmHex
// below for that.
//
// It contains no table instruction. No table instruction compiles (see
// TestForbiddenOps_Tables), so a module that used one would be rejected for
// that alone and could not show the declared-size check doing anything. It
// also keeps the warm path reachable: useWarmInstanceIfExists refuses a warm
// instance outright for any module wasmbytes.MutatesTables reports true for.
//
// Duplicated from oversizedTableNonMutatingWasmHex in
// contexts/runtime_test.go: the two live in different packages, and a shared
// export for one hex string is not worth the plumbing.
const tableGrowOversizedNonMutatingWasmHex = "0061736d0100000001040160000003030200000408017001008084af5f0503010001071803066d656d6f7279020004696e69740000046d61696e00010a070202000b02000b000b046e616d65050401000174"

// tableGrowUnboundedWasmHex declares a funcref table with limits flag = 0:
// no maximum at all, not even a large one. Wasmer reports this as u32::MAX
// (see wasmer2Instance.go:MaxDeclaredTableSize), which is also the value a
// declared maximum could never legitimately reach - Wasmer would report a
// module with limits flag = 1 and max = u32::MAX identically. This is the
// module that exercises that sentinel end to end, through the real deploy
// path and the real executor rather than a mocked instance.
const tableGrowUnboundedWasmHex = "" +
	"0061736d01000000010401600000030201000404017000000503010001071102066d656d6f7279020004696e697400000a040102000b"

// tableAtCapWasmHex / tableOverCapWasmHex declare tables of exactly the
// default cap (1,000) and exactly one over it (1,001). They pin the boundary
// against the real config/node/gasScheduleV1.yaml rather than a
// test-supplied cap, so loosening that YAML value - by an edit or a bad merge
// resolution - fails the suite instead of silently widening what is accepted.
const tableAtCapWasmHex = "" +
	"0061736d0100000001040160000003020100040601700100e8070503010001071102066d656d6f7279020004696e697400000a040102000b"

const tableOverCapWasmHex = "" +
	"0061736d0100000001040160000003020100040601700100e9070503010001071102066d656d6f7279020004696e697400000a040102000b"

// TestDeploy_RejectsOversizedDeclaredTableMaximum verifies the KLC-2526 /
// KLR-19 mitigation: a contract declaring a table with no protocol-bounded
// maximum must be rejected at deploy time.
func TestDeploy_RejectsOversizedDeclaredTableMaximum(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedNonMutatingWasmHex)
	if err != nil {
		t.Fatal(err)
	}

	deployInput := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1_000_000).
		WithContractCode(contractCode).
		WithCallerAddr(test.UserAddress).
		Build()

	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		Build()
	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: test.UserAddress,
		CreatorNonce:   0,
		NewAddress:     test.ParentAddress,
	})
	world.CreateAccount(test.UserAddress, world)

	vmOutput, err := host.RunSmartContractCreate(deployInput)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.ContractInvalid()
}

// TestDeploy_RejectsTableWithNoDeclaredMaximum covers the branch
// TestDeploy_RejectsOversizedDeclaredTableMaximum does not: a table with no
// maximum at all, not merely a very large one. The two are not
// interchangeable - correctness of the whole check rests on Wasmer actually
// reporting u32::MAX for "no maximum", a contract asserted in doc comments
// and covered by a mocked instance (TestTableDeclaration_verifyTableDeclaration
// in validator_test.go) but, before this test, never exercised through the
// real executor and the real deploy path.
func TestDeploy_RejectsTableWithNoDeclaredMaximum(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowUnboundedWasmHex)
	if err != nil {
		t.Fatal(err)
	}

	deployInput := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1_000_000).
		WithContractCode(contractCode).
		WithCallerAddr(test.UserAddress).
		Build()

	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		Build()
	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: test.UserAddress,
		CreatorNonce:   0,
		NewAddress:     test.ParentAddress,
	})
	world.CreateAccount(test.UserAddress, world)

	vmOutput, err := host.RunSmartContractCreate(deployInput)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.ContractInvalid()
}

// TestDeploy_AllowsOversizedDeclaredTableMaximum_PreFork verifies the
// KLC-2526 mitigation is gated behind FixAuditChangesV4: before the fork
// epoch is reached, the same module from
// TestDeploy_RejectsOversizedDeclaredTableMaximum must pass the table check
// exactly as it did before this fix, since two validators on different code
// versions must agree on the outcome of the same transaction until the fork
// activates. ContractInvalid would mean the table check fired anyway, which
// is what this test guards against.
func TestDeploy_AllowsOversizedDeclaredTableMaximum_PreFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedNonMutatingWasmHex)
	if err != nil {
		t.Fatal(err)
	}

	deployInput := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1_000_000).
		WithContractCode(contractCode).
		WithCallerAddr(test.UserAddress).
		Build()

	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		WithEnableEpochs(blockchainConfig.EnableEpochs{
			FixAuditChangesV4: 10, // not yet reached at epoch 0
		}).
		Build()
	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: test.UserAddress,
		CreatorNonce:   0,
		NewAddress:     test.ParentAddress,
	})
	world.CreateAccount(test.UserAddress, world)

	vmOutput, err := host.RunSmartContractCreate(deployInput)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
}

// TestDeploy_AcceptsSmallDeclaredTableMaximum is a control test confirming
// the mitigation does not break real contracts: the "adder" test contract
// declares a table with min=max=1 (well under the protocol cap) and must
// still deploy successfully.
func TestDeploy_AcceptsSmallDeclaredTableMaximum(t *testing.T) {
	deployInput := test.CreateTestContractCreateInputBuilder().
		WithGasProvided(1_000_000).
		WithContractCode(test.GetSCCode("../../test/adder/output/adder.wasm")).
		WithCallerAddr(test.UserAddress).
		WithArguments([]byte{0}).
		Build()

	world := worldmock.NewMockWorld()
	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		Build()
	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: test.UserAddress,
		CreatorNonce:   0,
		NewAddress:     test.ParentAddress,
	})
	world.CreateAccount(test.UserAddress, world)

	vmOutput, err := host.RunSmartContractCreate(deployInput)
	verify := test.NewVMOutputVerifier(t, vmOutput, err)
	verify.Ok()
}

// TestExecute_PreexistingOversizedTable_RejectedPostFork verifies the
// KLC-2526 fix closes the gap flagged in review: VerifyContractCode only
// ever ran on fresh deploys, so a contract that reached state before this
// fix - or before FixAuditChangesV4 activates - would keep its unbounded
// table forever, since nothing re-validates code already in storage. The
// real fix moved enforcement into instantiation itself (see
// klever-vm-executor-rs, validate_tables), which runs on every
// instantiation regardless of how the contract got into state, including
// this one injected directly into the mock world, bypassing deploy
// entirely, to simulate a contract that was already there before the fork.
func TestExecute_PreexistingOversizedTable_RejectedPostFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedNonMutatingWasmHex)
	require.NoError(t, err)

	world := worldmock.NewMockWorld()
	world.CreateAccount(test.UserAddress, world)
	scAccount := world.CreateSmartContractAccount(test.UserAddress, test.ParentAddress, contractCode, world)
	world.PutAccount(scAccount)

	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		Build()
	defer host.Reset()

	callOutput := runContractCall(t, host, "main", 2_000_000)
	require.NotEqual(t, vmcommon.Ok, callOutput.ReturnCode)
}

// TestExecute_PreexistingOversizedTable_PreFork is the pre-fork counterpart
// of TestExecute_PreexistingOversizedTable_RejectedPostFork: before
// FixAuditChangesV4 activates, the same preexisting contract must keep
// behaving exactly as it did before this fix, for the same
// validators-must-agree-until-the-fork reason as the deploy-time PreFork
// test above.
func TestExecute_PreexistingOversizedTable_PreFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedNonMutatingWasmHex)
	require.NoError(t, err)

	world := worldmock.NewMockWorld()
	world.CreateAccount(test.UserAddress, world)
	scAccount := world.CreateSmartContractAccount(test.UserAddress, test.ParentAddress, contractCode, world)
	world.PutAccount(scAccount)

	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithEnableEpochs(blockchainConfig.EnableEpochs{
			FixAuditChangesV4: 10, // not yet reached at epoch 0
		}).
		Build()
	defer host.Reset()

	callOutput := runContractCall(t, host, "main", 2_000_000)
	require.Equal(t, vmcommon.Ok, callOutput.ReturnCode)
}

// TestDeploy_TableSizeBoundaryAgainstShippedGasSchedule pins both sides of the
// cap against the gas schedule that actually ships, rather than a cap supplied
// by the test. The reject-side tests above use a table of 200,000,000, which
// fails under any plausible cap and so would keep passing even if the shipped
// value were loosened; these two fail if the boundary moves in either
// direction.
func TestDeploy_TableSizeBoundaryAgainstShippedGasSchedule(t *testing.T) {
	gasSchedule, err := blockchainConfig.LoadGasScheduleConfig("../../../config/node/gasScheduleV1.yaml")
	require.NoError(t, err)
	require.Equal(t, uint64(1000), gasSchedule["WASMOpcodeCost"]["MaxDeclaredTableSize"],
		"the boundary modules below encode 1000/1001; regenerate them if the shipped cap changes")

	for _, tc := range []struct {
		name    string
		codeHex string
		accept  bool
	}{
		{"table max exactly at cap is accepted", tableAtCapWasmHex, true},
		{"table max one over cap is rejected", tableOverCapWasmHex, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contractCode, err := hex.DecodeString(tc.codeHex)
			require.NoError(t, err)

			deployInput := test.CreateTestContractCreateInputBuilder().
				WithGasProvided(1_000_000).
				WithContractCode(contractCode).
				WithCallerAddr(test.UserAddress).
				Build()

			world := worldmock.NewMockWorld()
			host := test.NewTestHostBuilder(t).
				WithBlockchainHook(world).
				WithGasSchedule(gasSchedule).
				WithBuiltinFunctions().
				WithExecutorFactory(wasmer2.ExecutorFactory()).
				Build()
			defer host.Reset()
			world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
				CreatorAddress: test.UserAddress,
				CreatorNonce:   0,
				NewAddress:     test.ParentAddress,
			})
			world.CreateAccount(test.UserAddress, world)

			vmOutput, err := host.RunSmartContractCreate(deployInput)
			verify := test.NewVMOutputVerifier(t, vmOutput, err)
			if tc.accept {
				verify.Ok()
			} else {
				verify.ContractInvalid()
			}
		})
	}
}

// TestExecute_WarmInstanceIsRevalidatedPostFork is the regression test for the
// warm-instance bypass. A warm instance reaches execution without passing
// either enforcement layer: the executor validates declared tables only when
// it actually compiles or instantiates, and VerifyContractCode is skipped on
// the warm path. Reproduced live on a local node before the fix - a contract
// over the cap, warmed pre-fork, kept executing successfully after the fork
// activated.
//
// It matters for consensus rather than only for the missed rejection:
// warm-cache occupancy is node-local, so nodes holding the contract warm would
// accept a transaction that nodes reinstantiating it reject.
//
// The first call (pre-fork) both succeeds and leaves the instance warm; the
// second runs with the fork active against that same warm instance.
func TestExecute_WarmInstanceIsRevalidatedPostFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedNonMutatingWasmHex)
	require.NoError(t, err)
	require.False(t, wasmbytes.MutatesTables(contractCode),
		"fixture must not mutate tables, or useWarmInstanceIfExists refuses the warm instance before the check this test exists to cover")

	world := worldmock.NewMockWorld()
	world.CreateAccount(test.UserAddress, world)
	scAccount := world.CreateSmartContractAccount(test.UserAddress, test.ParentAddress, contractCode, world)
	world.PutAccount(scAccount)

	host := test.NewTestHostBuilder(t).
		WithBlockchainHook(world).
		WithGasSchedule(config.MakeGasMapForTests()).
		WithBuiltinFunctions().
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithEnableEpochs(blockchainConfig.EnableEpochs{
			FixAuditChangesV4: 1, // not active at epoch 0, active from epoch 1
		}).
		Build()
	defer host.Reset()

	// The concrete ForkController exposes EpochConfirmed; the core interface
	// does not, so drive the epoch through a narrow local assertion rather
	// than rebuilding the host between the two calls (which would discard the
	// warm cache and defeat the point of the test).
	epochConfirmer, ok := host.ForkController().(interface{ EpochConfirmed(epoch uint32) })
	require.True(t, ok, "ForkController must expose EpochConfirmed to drive the fork in-test")

	// Epoch 0: pre-fork. Succeeds, and leaves the instance in the warm cache.
	epochConfirmer.EpochConfirmed(0)
	preForkOutput := runContractCall(t, host, "main", 2_000_000)
	require.Equal(t, vmcommon.Ok, preForkOutput.ReturnCode)

	// Epoch 1: fork active. The same warm instance must now be rejected.
	epochConfirmer.EpochConfirmed(1)
	postForkOutput := runContractCall(t, host, "main", 2_000_000)
	require.NotEqual(t, vmcommon.Ok, postForkOutput.ReturnCode,
		"warm instance bypassed the declared-table-size check after the fork activated")

	// Note: the top-level path substitutes ErrContractInvalid for whatever
	// StartWasmerInstance returned, so warm and cold look identical here even
	// when they are not. The assertion that they genuinely agree lives at the
	// StartWasmerInstance boundary, in
	// TestRuntimeContext_WarmAndColdRejectionsAreIdentical.
}

// runContractCall invokes an exported endpoint on an already-built host and returns the
// raw output, so callers can assert on failures as well as successes.
func runContractCall(t *testing.T, host vmcommon.VMExecutionHandler, function string, gasProvided uint64) *vmcommon.VMOutput {
	t.Helper()
	input := test.CreateTestContractCallInputBuilder().
		WithRecipientAddr(test.ParentAddress).
		WithCallerAddr(test.UserAddress).
		WithFunction(function).
		WithGasProvided(gasProvided).
		Build()
	vmOutput, err := host.RunSmartContractCall(input)
	require.NoError(t, err)
	require.NotNil(t, vmOutput)
	return vmOutput
}
