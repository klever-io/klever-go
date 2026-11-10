package wasmer2

import (
	"testing"
	"unsafe"

	"github.com/klever-io/klever-go/kvm/executor"
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
