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

// tableGrowOversizedWasmHex declares a funcref table with a declared maximum
// of 200,000,000 (limits flag = 1, so it does have a maximum - it is bounded,
// just absurdly so). It exercises the "has a maximum, but it's still too
// large" branch of the check, not the "no maximum at all" branch: see
// tableGrowUnboundedWasmHex below for that.
// tableGrowOversizedNonMutatingWasmHex declares the same over-cap table
// maximum as tableGrowOversizedWasmHex, but contains no table-mutating opcode.
// The distinction decides whether the warm path is reached at all:
// useWarmInstanceIfExists refuses a warm instance outright for any module
// wasmbytes.MutatesTables reports true for, short-circuiting before
// verifyTableDeclarationIfActive. A fixture using table.grow therefore cannot
// exercise warm-instance revalidation - the test would still pass, but because
// both calls went cold and the executor rejected them, not because the warm
// path re-checked anything.
//
// Duplicated from oversizedTableNonMutatingWasmHex in
// contexts/runtime_test.go: the two live in different packages, and a shared
// export for one hex string is not worth the plumbing.
const tableGrowOversizedNonMutatingWasmHex = "0061736d0100000001040160000003030200000408017001008084af5f0503010001071803066d656d6f7279020004696e69740000046d61696e00010a070202000b02000b000b046e616d65050401000174"

const tableGrowOversizedWasmHex = "" +
	"0061736d010000000104016000000307060000000000000408017001008084af5f0503010001077b07066d656d6f727902000867726f775f6f6e6500000d67726f775f74686f7573616e6400011567726f775f68756e647265645f74686f7573616e6400020c67726f775f6d696c6c696f6e00031167726f775f666976655f6d696c6c696f6e0004186173736572745f73697a655f666976655f6d696c6c696f6e00050a50060a00d0704101fc0f001a0b0b00d07041e807fc0f001a0b0c00d07041a08d06fc0f001a0b0c00d07041c0843dfc0f001a0b0d00d07041c096b102fc0f001a0b0f00fc100041c096b102470440000b0b"

// tableGrowUnboundedWasmHex declares a funcref table with limits flag = 0:
// no maximum at all, not even a large one. Wasmer reports this as u32::MAX
// (see wasmer2Instance.go:MaxDeclaredTableSize), which is also the value a
// declared maximum could never legitimately reach - Wasmer would report a
// module with limits flag = 1 and max = u32::MAX identically. This is the
// module that exercises that sentinel end to end, through the real deploy
// path and the real executor rather than a mocked instance.
const tableGrowUnboundedWasmHex = "" +
	"0061736d01000000010401600000030201000404017000000503010001071102066d656d6f7279020004696e697400000a040102000b"

// tableGrowBoundedWasmHex declares a funcref table with max=1,000, matching
// the default WASMOpcodeCost.MaxDeclaredTableSize gas schedule cap. Unlike
// tableGrowOversizedWasmHex, this module deploys and instantiates successfully
// post-fork: the point of the fix is to reject the *declaration*, not
// table.grow itself, so a well-formed module using its table right up to a
// sane cap must keep working.
//
// Exports two different growth amounts (grow_one and grow_one_thousand) so a
// test can compare their gas costs against each other rather than against a
// single hardcoded number.
const tableGrowBoundedWasmHex = "" +
	"0061736d0100000001040160000003050400000000040601700100e8070503010001074b05066d656d6f7279020004696e697400000867726f775f6f6e6500011167726f775f6f6e655f74686f7573616e640002186173736572745f73697a655f6f6e655f74686f7573616e6400030a290402000b0a00d0704101fc0f001a0b0b00d07041e807fc0f001a0b0d00fc100041e807470440000b0b"

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
// maximum must be rejected at deploy time, before it can ever reach
// table.grow to force unbounded validator memory allocations.
func TestDeploy_RejectsOversizedDeclaredTableMaximum(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedWasmHex)
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
// activates. The module exports no "init" function (it was only ever built
// to demonstrate table.grow), so a deploy that gets past the table check
// still fails afterwards with FunctionNotFound; ContractInvalid would mean
// the table check fired anyway, which is what this test guards against.
func TestDeploy_AllowsOversizedDeclaredTableMaximum_PreFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedWasmHex)
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
	verify.FunctionNotFound()
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

// TestGasUsed_TableGrow_FlatCostRegardlessOfSize documents a metering
// characteristic this PR does not change: table.grow is metered at a flat
// per-call cost with no regard for how many elements are actually added.
//
// The claim is checked by comparing two different growth amounts against each
// other - growing by 1 and growing by 1,000 - rather than by asserting one
// call against a hardcoded constant. A single measurement would pass unchanged
// if table.grow were correctly metered per element, which is precisely the
// property the name claims to pin.
//
// Each amount runs on its own host so neither measurement can be perturbed by
// table state left behind by the other.
func TestGasUsed_TableGrow_FlatCostRegardlessOfSize(t *testing.T) {
	gasSchedule, err := blockchainConfig.LoadGasScheduleConfig("../../../config/node/gasScheduleV1.yaml")
	require.NoError(t, err)

	contractCode, err := hex.DecodeString(tableGrowBoundedWasmHex)
	require.NoError(t, err)

	gasFor := func(function string) uint64 {
		world := worldmock.NewMockWorld()
		world.CreateAccount(test.UserAddress, world)
		scAccount := world.CreateSmartContractAccount(test.UserAddress, test.ParentAddress, contractCode, world)
		world.PutAccount(scAccount)

		host := test.NewTestHostBuilder(t).
			WithBlockchainHook(world).
			WithGasSchedule(gasSchedule).
			WithBuiltinFunctions().
			WithExecutorFactory(wasmer2.ExecutorFactory()).
			Build()
		defer host.Reset()

		const gasProvided = uint64(2_000_000)
		out := runTableGrowCall(t, host, function, gasProvided)
		require.Equal(t, vmcommon.Ok, out.ReturnCode, "%s should succeed", function)
		return gasProvided - out.GasRemaining
	}

	growOne := gasFor("grow_one")
	growThousand := gasFor("grow_one_thousand")

	t.Logf("grow_one=%d  grow_one_thousand=%d  delta=%d", growOne, growThousand, int64(growThousand)-int64(growOne))
	require.Equal(t, growOne, growThousand,
		"growing by 1,000 costs the same as growing by 1: table.grow is not metered per element")
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
	contractCode, err := hex.DecodeString(tableGrowOversizedWasmHex)
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

	growOutput := runTableGrowCall(t, host, "grow_five_million", 2_000_000)
	require.NotEqual(t, vmcommon.Ok, growOutput.ReturnCode)
}

// TestExecute_PreexistingOversizedTable_PreFork is the pre-fork counterpart
// of TestExecute_PreexistingOversizedTable_RejectedPostFork: before
// FixAuditChangesV4 activates, the same preexisting contract must keep
// behaving exactly as it did before this fix, for the same
// validators-must-agree-until-the-fork reason as the deploy-time PreFork
// test above.
func TestExecute_PreexistingOversizedTable_PreFork(t *testing.T) {
	contractCode, err := hex.DecodeString(tableGrowOversizedWasmHex)
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

	growOutput := runTableGrowCall(t, host, "grow_five_million", 2_000_000)
	require.Equal(t, vmcommon.Ok, growOutput.ReturnCode)
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
	preForkOutput := runTableGrowCall(t, host, "main", 2_000_000)
	require.Equal(t, vmcommon.Ok, preForkOutput.ReturnCode)

	// Epoch 1: fork active. The same warm instance must now be rejected.
	epochConfirmer.EpochConfirmed(1)
	postForkOutput := runTableGrowCall(t, host, "main", 2_000_000)
	require.NotEqual(t, vmcommon.Ok, postForkOutput.ReturnCode,
		"warm instance bypassed the declared-table-size check after the fork activated")

	// Note: the top-level path substitutes ErrContractInvalid for whatever
	// StartWasmerInstance returned, so warm and cold look identical here even
	// when they are not. The assertion that they genuinely agree lives at the
	// StartWasmerInstance boundary, in
	// TestRuntimeContext_WarmAndColdRejectionsAreIdentical.
}

func runTableGrowCall(t *testing.T, host vmcommon.VMExecutionHandler, function string, gasProvided uint64) *vmcommon.VMOutput {
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
