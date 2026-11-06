package wasmer2

import (
	"testing"
	"unsafe"

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

func TestCheckOpcodeCostLayout(t *testing.T) {
	require.NoError(t, checkOpcodeCostLayout(expectedOpcodeCostSize))

	// The pre-KLC-2595 libraries read five more table opcode fields.
	err := checkOpcodeCostLayout(expectedOpcodeCostSize + 5*4)
	require.ErrorIs(t, err, ErrOpcodeCostLayoutMismatch)
	require.Contains(t, err.Error(), "library 512 bytes, Go 492 bytes")
}

func TestCreateExecutor_RefusesMismatchedLibrary(t *testing.T) {
	original := libraryOpcodeCostSize
	t.Cleanup(func() { libraryOpcodeCostSize = original })
	libraryOpcodeCostSize = func() uint32 { return expectedOpcodeCostSize + 5*4 }

	executor, err := CreateExecutor()
	require.ErrorIs(t, err, ErrOpcodeCostLayoutMismatch)
	require.Nil(t, executor)
}
