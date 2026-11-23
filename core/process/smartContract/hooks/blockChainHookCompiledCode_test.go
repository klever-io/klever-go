package hooks

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-3001: the compiled-code cache gains a generation counter so a module compiled
// off the block path can be refused when the cache was cleared while it compiled.

func newCompiledCodeTestHook(t *testing.T) *BlockChainHookImpl {
	t.Helper()

	pool, err := lrucache.NewCache(100)
	require.NoError(t, err)

	bh := &BlockChainHookImpl{
		compiledScPool:     pool,
		nilCompiledSCStore: true,
	}
	require.NoError(t, bh.makeCompiledSCStorage())

	return bh
}

func TestBlockChainHookImpl_SaveAndGetCompiledCodeUnchanged(t *testing.T) {
	t.Parallel()

	bh := newCompiledCodeTestHook(t)

	found, code := bh.GetCompiledCode([]byte("hash"))
	assert.False(t, found)
	assert.Nil(t, code)

	bh.SaveCompiledCode([]byte("hash"), []byte("module"))
	found, code = bh.GetCompiledCode([]byte("hash"))
	assert.True(t, found)
	assert.Equal(t, []byte("module"), code)

	bh.ClearCompiledCodes()
	found, _ = bh.GetCompiledCode([]byte("hash"))
	assert.False(t, found)
}

func TestBlockChainHookImpl_ClearCompiledCodesBumpsGeneration(t *testing.T) {
	t.Parallel()

	bh := newCompiledCodeTestHook(t)

	g0 := bh.CompiledCodeGeneration()
	bh.ClearCompiledCodes()
	g1 := bh.CompiledCodeGeneration()
	bh.ClearCompiledCodes()
	g2 := bh.CompiledCodeGeneration()

	assert.Equal(t, g0+1, g1)
	assert.Equal(t, g1+1, g2)
}

func TestBlockChainHookImpl_SaveCompiledCodeIfCurrent(t *testing.T) {
	t.Parallel()

	always := func() bool { return true }

	t.Run("accepted when generation and check hold", func(t *testing.T) {
		t.Parallel()

		bh := newCompiledCodeTestHook(t)
		g := bh.CompiledCodeGeneration()

		saved := bh.SaveCompiledCodeIfCurrent(g, always, []byte("hash"), []byte("module"))
		assert.True(t, saved)

		found, code := bh.GetCompiledCode([]byte("hash"))
		assert.True(t, found)
		assert.Equal(t, []byte("module"), code)
	})

	t.Run("nil check is treated as current", func(t *testing.T) {
		t.Parallel()

		bh := newCompiledCodeTestHook(t)
		saved := bh.SaveCompiledCodeIfCurrent(bh.CompiledCodeGeneration(), nil, []byte("hash"), []byte("module"))
		assert.True(t, saved)
	})

	t.Run("rejected after a clear", func(t *testing.T) {
		t.Parallel()

		bh := newCompiledCodeTestHook(t)
		g := bh.CompiledCodeGeneration()
		bh.ClearCompiledCodes()

		saved := bh.SaveCompiledCodeIfCurrent(g, always, []byte("hash"), []byte("module"))
		assert.False(t, saved)

		found, _ := bh.GetCompiledCode([]byte("hash"))
		assert.False(t, found)
	})

	t.Run("rejected when the fingerprint changed", func(t *testing.T) {
		t.Parallel()

		bh := newCompiledCodeTestHook(t)
		g := bh.CompiledCodeGeneration()

		saved := bh.SaveCompiledCodeIfCurrent(g, func() bool { return false }, []byte("hash"), []byte("module"))
		assert.False(t, saved)

		found, _ := bh.GetCompiledCode([]byte("hash"))
		assert.False(t, found)
	})
}

func TestBlockChainHookImpl_CompiledCodesClearedHandler(t *testing.T) {
	t.Parallel()

	bh := newCompiledCodeTestHook(t)

	var calls atomic.Int32
	var generationSeen atomic.Uint64
	bh.RegisterCompiledCodesClearedHandler(func() {
		calls.Add(1)
		// Runs after the lock is released, so it may read the hook without deadlocking.
		generationSeen.Store(bh.CompiledCodeGeneration())
		_, _ = bh.GetCompiledCode([]byte("hash"))
	})
	bh.RegisterCompiledCodesClearedHandler(nil)

	bh.ClearCompiledCodes()
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, bh.CompiledCodeGeneration(), generationSeen.Load())

	bh.EpochConfirmed(0) // not an activation epoch: nil map, no clear
	assert.Equal(t, int32(1), calls.Load())
}

func TestBlockChainHookImpl_CompiledCodeConcurrentAccess(t *testing.T) {
	t.Parallel()

	bh := newCompiledCodeTestHook(t)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bh.ClearCompiledCodes()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bh.SaveCompiledCode([]byte("hash"), []byte("module"))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = bh.SaveCompiledCodeIfCurrent(bh.CompiledCodeGeneration(), nil, []byte("hash"), []byte("module"))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = bh.GetCompiledCode([]byte("hash"))
			}
		}()
	}
	wg.Wait()
}

func TestBlockChainHookImpl_CloseInvalidatesModulesCompiledBeforeIt(t *testing.T) {
	t.Parallel()

	bh := newCompiledCodeTestHook(t)
	generation := bh.CompiledCodeGeneration()

	require.NoError(t, bh.Close())

	assert.Equal(t, generation+1, bh.CompiledCodeGeneration())
	assert.False(t, bh.SaveCompiledCodeIfCurrent(generation, nil, []byte("hash"), []byte("module")),
		"a module compiled before Close must not be saved into the destroyed storage")
}
