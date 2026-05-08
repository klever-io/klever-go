package contexts

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto/factory"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon/testexecutor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

const (
	smallWasmPath = "./../../test/contracts/counter/output/counter.wasm"
	bigWasmPath   = "./../../test/digital-cash/output/digital-cash.wasm"

	benchGasLimit = uint64(100_000_000)
)

func setupExecutorForBench(b testing.TB) executor.Executor {
	gasSchedule := config.MakeGasMapForTests()
	host := &contextmock.VMHostMock{}

	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(gasSchedule)
	host.MeteringContext = mockMetering
	host.BlockchainContext, _ = NewBlockchainContext(host, worldmock.NewMockWorld())
	host.OutputContext, _ = NewOutputContext(host)
	host.CryptoHook = factory.NewVMCrypto()

	execFactory := testexecutor.NewDefaultTestExecutorFactory(b)
	exec, err := execFactory.CreateExecutor(executor.ExecutorFactoryArgs{
		VMHooks:          vmhooks.NewVMHooksImpl(host),
		ExecutionTimeout: time.Minute,
	})
	require.Nil(b, err)
	return exec
}

func setupRuntimeForBench(b testing.TB) *runtimeContext {
	gasSchedule := config.MakeGasMapForTests()
	host := &contextmock.VMHostMock{}

	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(gasSchedule)
	host.MeteringContext = mockMetering
	host.BlockchainContext, _ = NewBlockchainContext(host, worldmock.NewMockWorld())
	host.OutputContext, _ = NewOutputContext(host)
	host.CryptoHook = factory.NewVMCrypto()

	execFactory := testexecutor.NewDefaultTestExecutorFactory(b)
	exec, err := execFactory.CreateExecutor(executor.ExecutorFactoryArgs{
		VMHooks:          vmhooks.NewVMHooksImpl(host),
		ExecutionTimeout: time.Minute,
	})
	require.Nil(b, err)

	rt, err := NewRuntimeContext(
		host,
		vmType,
		builtInFunctions.NewBuiltInFunctionContainer(),
		exec,
		defaultHasher,
	)
	require.Nil(b, err)
	rt.SetMaxInstanceStackSize(1)
	return rt
}

func compileOptions() executor.CompilationOptions {
	wasmCost := config.MakeGasMapForTests()["WASMOpcodeCost"]
	return executor.CompilationOptions{
		GasLimit:           benchGasLimit,
		UnmeteredLocals:    wasmCost["LocalsUnmetered"],
		MaxMemoryGrow:      wasmCost["MaxMemoryGrow"],
		MaxMemoryGrowDelta: wasmCost["MaxMemoryGrowDelta"],
		OpcodeTrace:        false,
		Metering:           true,
		RuntimeBreakpoints: true,
	}
}

// benchJITCompile measures the cost of a full Wasmer Singlepass JIT compile —
// the cold path that fires when neither the warm-instance LRU nor the
// compiled-code pool can serve the contract.
func benchJITCompile(b *testing.B, wasmPath string) {
	exec := setupExecutorForBench(b)
	code := vmhost.GetSCCode(wasmPath)
	options := compileOptions()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		inst, err := exec.NewInstanceWithOptions(code, options)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		inst.Clean()
		b.StartTimer()
	}
}

// benchAOTDeserialize measures the cost of restoring a Wasmer instance from
// already-compiled AOT bytes — the path that fires when the warm-instance LRU
// misses but the compiled-code pool has the bytes. This is the suspected ~10ms
// floor for first-of-contract calls in steady state.
func benchAOTDeserialize(b *testing.B, wasmPath string) {
	exec := setupExecutorForBench(b)
	code := vmhost.GetSCCode(wasmPath)
	options := compileOptions()

	seed, err := exec.NewInstanceWithOptions(code, options)
	require.Nil(b, err)
	cached, err := seed.Cache()
	require.Nil(b, err)
	require.NotEmpty(b, cached)
	seed.Clean()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		inst, err := exec.NewInstanceFromCompiledCodeWithOptions(cached, options)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		inst.Clean()
		b.StartTimer()
	}
}

// benchWarmReset measures the cost of reusing a warm instance via
// instance.Reset() — the fast path that fires when the warm-instance LRU
// already holds the contract.
func benchWarmReset(b *testing.B, wasmPath string) {
	exec := setupExecutorForBench(b)
	code := vmhost.GetSCCode(wasmPath)
	options := compileOptions()

	inst, err := exec.NewInstanceWithOptions(code, options)
	require.Nil(b, err)
	defer inst.Clean()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if !inst.Reset() {
			b.Fatal("instance.Reset returned false")
		}
	}
}

func BenchmarkJITCompile_Small(b *testing.B)     { benchJITCompile(b, smallWasmPath) }
func BenchmarkJITCompile_Big(b *testing.B)       { benchJITCompile(b, bigWasmPath) }
func BenchmarkAOTDeserialize_Small(b *testing.B) { benchAOTDeserialize(b, smallWasmPath) }
func BenchmarkAOTDeserialize_Big(b *testing.B)   { benchAOTDeserialize(b, bigWasmPath) }
func BenchmarkWarmReset_Small(b *testing.B)      { benchWarmReset(b, smallWasmPath) }
func BenchmarkWarmReset_Big(b *testing.B)        { benchWarmReset(b, bigWasmPath) }
