package wasmer2

import (
	"os"
	"sync"
	"testing"
	"unsafe"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedOpcodeCostSize is 123 uint32 fields. The executor's OpcodeCost is
// #[repr(C)] with the same fields, and its own test pins the same size.
const expectedOpcodeCostSize = 123 * 4

func TestOpcodeCost_SizeMatchesExecutorLibrary(t *testing.T) {
	require.Equal(t, uintptr(expectedOpcodeCostSize), unsafe.Sizeof(OpcodeCost{}))
	require.Equal(t, uint32(expectedOpcodeCostSize), cWasmerOpcodeCostSize(),
		"the shipped executor libraries must read the same opcode cost struct as OpcodeCost")
}

func TestCreateExecutor_RefusesMismatchedLibrary(t *testing.T) {
	original := libraryOpcodeCostSize
	t.Cleanup(func() { libraryOpcodeCostSize = original })
	libraryOpcodeCostSize = func() uint32 { return expectedOpcodeCostSize + 5*4 }

	executor, err := CreateExecutor()
	require.ErrorIs(t, err, ErrOpcodeCostLayoutMismatch)
	require.Nil(t, executor)
}

// expectedCompilationOptionsSize is five uint64 and three bool, padded to the
// 8-byte alignment. The executor's CompilationOptions is #[repr(C)] with the
// same fields, and its own test pins the same size.
const expectedCompilationOptionsSize = 48

func TestCompilationOptions_SizeMatchesExecutorLibrary(t *testing.T) {
	require.Equal(t, uintptr(expectedCompilationOptionsSize), unsafe.Sizeof(executor.CompilationOptions{}))
	require.Equal(t, uint32(expectedCompilationOptionsSize), cWasmerCompilationOptionsSize(),
		"the shipped executor libraries must read the same compilation options struct as CompilationOptions")
}

func TestCheckStructLayout(t *testing.T) {
	opcodeCostSize := unsafe.Sizeof(OpcodeCost{})
	require.NoError(t, checkStructLayout(ErrOpcodeCostLayoutMismatch, expectedOpcodeCostSize, opcodeCostSize))

	// The pre-KLC-2595 libraries read five more table opcode fields.
	err := checkStructLayout(ErrOpcodeCostLayoutMismatch, expectedOpcodeCostSize+5*4, opcodeCostSize)
	require.ErrorIs(t, err, ErrOpcodeCostLayoutMismatch)
	require.Contains(t, err.Error(), "library 512 bytes, Go 492 bytes")

	optionsSize := unsafe.Sizeof(executor.CompilationOptions{})
	require.NoError(t, checkStructLayout(ErrCompilationOptionsLayoutMismatch, expectedCompilationOptionsSize, optionsSize))

	// The pre-KLC-2958 libraries read one more uint64, the table.init per-element cost.
	err = checkStructLayout(ErrCompilationOptionsLayoutMismatch, expectedCompilationOptionsSize+8, optionsSize)
	require.ErrorIs(t, err, ErrCompilationOptionsLayoutMismatch)
	require.Contains(t, err.Error(), "library 56 bytes, Go 48 bytes")
}

func TestCreateExecutor_RefusesMismatchedCompilationOptions(t *testing.T) {
	original := libraryCompilationOptionsSize
	t.Cleanup(func() { libraryCompilationOptionsSize = original })
	libraryCompilationOptionsSize = func() uint32 { return expectedCompilationOptionsSize + 8 }

	wasmerExecutor, err := CreateExecutor()
	require.ErrorIs(t, err, ErrCompilationOptionsLayoutMismatch)
	require.Nil(t, wasmerExecutor)
}

type vmHooksStub struct {
	executor.VMHooks
}

func TestWasmer2Executor_ResolvesItsOwnVMHooks(t *testing.T) {
	firstHooks := &vmHooksStub{}
	secondHooks := &vmHooksStub{}

	firstExecutor, err := CreateExecutor()
	require.NoError(t, err)
	require.NoError(t, firstExecutor.initVMHooks(firstHooks))

	secondExecutor, err := CreateExecutor()
	require.NoError(t, err)
	require.NoError(t, secondExecutor.initVMHooks(secondHooks))

	require.Same(t, firstHooks, getVMHooksFromContextRawPtr(firstExecutor.vmHooksCell))
	require.Same(t, secondHooks, getVMHooksFromContextRawPtr(secondExecutor.vmHooksCell))
}

func TestWasmer2Executor_InitVMHooksFailsWhileAnInstanceExists(t *testing.T) {
	wasmerExecutor, err := CreateExecutor()
	require.NoError(t, err)

	code, err := os.ReadFile("../test/contracts/answer/output/answer.wasm")
	require.NoError(t, err)
	instance, err := wasmerExecutor.NewInstanceWithOptions(code, executor.CompilationOptions{
		GasLimit:             100_000_000,
		RuntimeBreakpoints:   true,
		MaxDeclaredTableSize: 1024,
	})
	require.NoError(t, err)
	defer instance.Clean()

	err = wasmerExecutor.initVMHooks(&vmHooksStub{})
	require.ErrorIs(t, err, ErrFailedVMHooksSetup)
	require.Nil(t, wasmerExecutor.vmHooksCell)
}

func TestVMHooksRegistry_ConcurrentAddAndGet(t *testing.T) {
	registry := &vmHooksRegistry{}
	numGoroutines := 16
	numHooksPerGoroutine := 200

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			for range numHooksPerGoroutine {
				hooks := &vmHooksStub{}
				index := registry.add(hooks)
				assert.Same(t, hooks, registry.get(index))
			}
		})
	}
	wg.Wait()

	require.Len(t, *registry.entries.Load(), numGoroutines*numHooksPerGoroutine)
}

func BenchmarkGetVMHooksFromContextRawPtr(b *testing.B) {
	wasmerExecutor, err := CreateExecutor()
	require.NoError(b, err)
	require.NoError(b, wasmerExecutor.initVMHooks(&vmHooksStub{}))

	var vmHooks executor.VMHooks
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		vmHooks = getVMHooksFromContextRawPtr(wasmerExecutor.vmHooksCell)
	}
	require.NotNil(b, vmHooks)
}
