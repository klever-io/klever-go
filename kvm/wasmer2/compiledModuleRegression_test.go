package wasmer2

// KLC-3001 regression tests for the properties the compiled-code warmer relies on.
// The warmer compiles modules on its own executor, off the block path, and stores
// the Cache() output where the runtime restores it from. That is only consensus-safe
// while the properties below hold.

import (
	"math"
	"os"
	"testing"

	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/require"
)

var compiledModuleFixtures = []struct {
	name string
	path string
}{
	{"answer", "../test/contracts/answer/output/answer.wasm"},
	{"alloc-features", "../test/features/alloc-features/output/alloc-features.wasm"},
	{"basic-features", "../test/features/basic-features/output/basic-features.wasm"},
	{"features-no-small-int-api", "../test/features/basic-features-no-small-int-api/output/features-no-small-int-api.wasm"},
}

func compiledModuleTestOptions(gasLimit uint64) executor.CompilationOptions {
	gasMap := config.MakeGasMapForTests()
	opcodes := gasMap["WASMOpcodeCost"]
	return executor.CompilationOptions{
		GasLimit:             gasLimit,
		UnmeteredLocals:      opcodes["LocalsUnmetered"],
		MaxMemoryGrow:        opcodes["MaxMemoryGrow"],
		MaxMemoryGrowDelta:   opcodes["MaxMemoryGrowDelta"],
		MaxDeclaredTableSize: math.MaxUint64,
		Metering:             true,
		RuntimeBreakpoints:   true,
	}
}

// newCompiledModuleTestExecutor creates an executor the way the warmer does: no VM
// hooks, only opcode costs.
func newCompiledModuleTestExecutor(tb testing.TB, opcodeCost uint64) *Wasmer2Executor {
	tb.Helper()

	exec, err := CreateExecutor()
	require.NoError(tb, err)

	gasCost, err := config.CreateGasConfig(config.MakeGasMap(opcodeCost))
	require.NoError(tb, err)
	exec.SetOpcodeCosts(gasCost.WASMOpcodeCost)

	return exec
}

// buildProbeModule builds, by hand, a module with no imports whose exported "probe"
// function runs `reps` blocks of i64.const/i64.const/i64.add/drop. Its gas use is a
// pure function of the opcode costs, so it shows which costs a module charges.
func buildProbeModule(reps int) []byte {
	leb := func(v int) []byte {
		var out []byte
		for {
			b := byte(v & 0x7f)
			v >>= 7
			if v != 0 {
				out = append(out, b|0x80)
				continue
			}
			return append(out, b)
		}
	}
	section := func(id byte, content []byte) []byte {
		return append(append([]byte{id}, leb(len(content))...), content...)
	}
	name := func(s string) []byte {
		return append(leb(len(s)), []byte(s)...)
	}

	var body []byte
	body = append(body, 0x00) // no locals
	for range reps {
		body = append(body, 0x42, 0x01, 0x42, 0x02, 0x7c, 0x1a)
	}
	body = append(body, 0x0b)

	var exports []byte
	exports = append(exports, 0x02)
	exports = append(exports, name("memory")...)
	exports = append(exports, 0x02, 0x00)
	exports = append(exports, name("probe")...)
	exports = append(exports, 0x00, 0x00)

	module := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	module = append(module, section(0x01, []byte{0x01, 0x60, 0x00, 0x00})...)
	module = append(module, section(0x03, []byte{0x01, 0x00})...)
	module = append(module, section(0x05, []byte{0x01, 0x00, 0x01})...)
	module = append(module, section(0x07, exports)...)
	module = append(module, section(0x0a, append(append([]byte{0x01}, leb(len(body))...), body...))...)
	return module
}

func probeModulePointsUsed(t *testing.T, inst executor.Instance) uint64 {
	t.Helper()

	inst.SetPointsUsed(0)
	require.NoError(t, inst.CallFunction("probe"))
	return inst.GetPointsUsed()
}

func compileProbe(t *testing.T, exec *Wasmer2Executor, code []byte) uint64 {
	t.Helper()

	inst, err := exec.NewInstanceWithOptions(code, compiledModuleTestOptions(math.MaxUint64))
	require.NoError(t, err)
	defer inst.Clean()

	return probeModulePointsUsed(t, inst)
}

// TestCompiledModuleKeepsCompileTimeOpcodeCosts pins that opcode costs are baked into
// Cache() output: a module compiled under one schedule keeps charging that schedule
// after the executor moves to another. This is why the warmer must never insert a
// module compiled under a stale gas schedule.
func TestCompiledModuleKeepsCompileTimeOpcodeCosts(t *testing.T) {
	code := buildProbeModule(50)
	options := compiledModuleTestOptions(math.MaxUint64)

	exec := newCompiledModuleTestExecutor(t, 1)
	fresh1, err := exec.NewInstanceWithOptions(code, options)
	require.NoError(t, err)
	defer fresh1.Clean()
	pointsCost1 := probeModulePointsUsed(t, fresh1)

	compiledUnderCost1, err := fresh1.Cache()
	require.NoError(t, err)

	// Change the schedule on the same executor, as vmHost.GasScheduleChange does.
	gasCost5, err := config.CreateGasConfig(config.MakeGasMap(5))
	require.NoError(t, err)
	exec.SetOpcodeCosts(gasCost5.WASMOpcodeCost)

	fresh5, err := exec.NewInstanceWithOptions(code, options)
	require.NoError(t, err)
	defer fresh5.Clean()
	pointsCost5 := probeModulePointsUsed(t, fresh5)

	fromStale, err := exec.NewInstanceFromCompiledCodeWithOptions(compiledUnderCost1, options)
	require.NoError(t, err)
	defer fromStale.Clean()
	pointsStale := probeModulePointsUsed(t, fromStale)

	require.NotEqual(t, pointsCost1, pointsCost5, "the probe must be sensitive to opcode costs")
	require.Equal(t, pointsCost1, pointsStale, "a compiled module must keep the opcode costs it was compiled with")
}

// TestOpcodeCostsArePerExecutor pins that SetOpcodeCosts on one executor does not
// change what another executor compiles. The warmer sets costs on its own executor;
// if costs were process-global it would change the block-processing VM's metering.
func TestOpcodeCostsArePerExecutor(t *testing.T) {
	code := buildProbeModule(50)

	execA := newCompiledModuleTestExecutor(t, 1)
	pointsA := compileProbe(t, execA, code)

	execB := newCompiledModuleTestExecutor(t, 5)
	pointsB := compileProbe(t, execB, code)
	require.NotEqual(t, pointsA, pointsB)

	require.Equal(t, pointsA, compileProbe(t, execA, code), "executor A must keep its own costs")

	gasCost7, err := config.CreateGasConfig(config.MakeGasMap(7))
	require.NoError(t, err)
	execB.SetOpcodeCosts(gasCost7.WASMOpcodeCost)
	require.Equal(t, pointsA, compileProbe(t, execA, code), "changing B's costs must not affect A")
}

// TestCompiledModuleGasLimitIsAppliedOnRestore pins how the GasLimit compilation option
// relates to Cache() output. Cache() is deterministic for identical options, but the
// GasLimit is serialized into it, so modules compiled under different limits are NOT
// byte-identical. What the compiled-code pool relies on (it is shared by calls with
// different gas limits) is that the limit passed when restoring is the one enforced,
// whatever limit the module was compiled under.
func TestCompiledModuleGasLimitIsAppliedOnRestore(t *testing.T) {
	exec := newCompiledModuleTestExecutor(t, 1)

	t.Run("Cache output is deterministic but embeds the gas limit", func(t *testing.T) {
		for _, fixture := range compiledModuleFixtures {
			code, err := os.ReadFile(fixture.path)
			require.NoError(t, err)

			cacheFor := func(gasLimit uint64) []byte {
				inst, errInst := exec.NewInstanceWithOptions(code, compiledModuleTestOptions(gasLimit))
				require.NoError(t, errInst)
				defer inst.Clean()

				compiled, errCache := inst.Cache()
				require.NoError(t, errCache)
				require.NotEmpty(t, compiled)
				return compiled
			}

			high := cacheFor(math.MaxUint64)
			again := cacheFor(math.MaxUint64)
			low := cacheFor(1_000_000)

			require.Equal(t, high, again, "%s: Cache() output must be deterministic", fixture.name)
			require.Equal(t, len(high), len(low), "%s", fixture.name)
			require.NotEqual(t, high, low, "%s: the gas limit is part of Cache() output", fixture.name)
		}
	})

	t.Run("restore-time gas limit is enforced", func(t *testing.T) {
		code := buildProbeModule(50)
		need := compileProbe(t, exec, code)

		run := func(compileLimit uint64, restoreLimit uint64) error {
			seed, err := exec.NewInstanceWithOptions(code, compiledModuleTestOptions(compileLimit))
			require.NoError(t, err)
			compiled, err := seed.Cache()
			seed.Clean()
			require.NoError(t, err)

			inst, err := exec.NewInstanceFromCompiledCodeWithOptions(compiled, compiledModuleTestOptions(restoreLimit))
			require.NoError(t, err)
			defer inst.Clean()

			inst.SetPointsUsed(0)
			return inst.CallFunction("probe")
		}

		require.Error(t, run(math.MaxUint64, need-1), "restored with too little gas must run out")
		require.NoError(t, run(need-1, math.MaxUint64), "a small compile-time limit must not cap the restored call")
	})
}
