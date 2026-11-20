package worldmock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// KLC-3001: Clear resets the compiled-code cache, so, like ClearCompiledCodes and the real
// hook, it must bump the generation and notify the cleared handlers.
func TestMockWorld_ClearInvalidatesTheCompiledCodeGeneration(t *testing.T) {
	t.Parallel()

	world := NewMockWorld()
	numCleared := 0
	world.RegisterCompiledCodesClearedHandler(func() { numCleared++ })

	generation := world.CompiledCodeGeneration()
	world.Clear()

	require.NotEqual(t, generation, world.CompiledCodeGeneration())
	require.False(t, world.SaveCompiledCodeIfCurrent(generation, nil, []byte("hash"), []byte("module")))
	found, _ := world.GetCompiledCode([]byte("hash"))
	require.False(t, found)
	require.Equal(t, 1, numCleared)

	require.True(t, world.SaveCompiledCodeIfCurrent(world.CompiledCodeGeneration(), nil, []byte("hash"), []byte("module")))
}
