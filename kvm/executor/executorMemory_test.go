package executor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemLoadBytes_Edges(t *testing.T) {
	t.Parallel()

	mem := []byte{0, 1, 2, 3, 4, 5, 6, 7}

	t.Run("in range", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, 2, 3)
		require.NoError(t, err)
		assert.Equal(t, []byte{2, 3, 4}, got)
	})

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, 6, 5)
		require.NoError(t, err)
		assert.Equal(t, []byte{6, 7}, got)
	})

	t.Run("starts at the end", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, MemPtr(len(mem)), 4)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, got)
	})

	t.Run("negative length", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, 1, -2)
		require.ErrorIs(t, err, ErrMemoryNegativeLength)
		assert.Nil(t, got)
	})

	t.Run("negative offset wins over negative length", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, -1, -2)
		require.ErrorIs(t, err, ErrMemoryBadBounds)
		assert.NotErrorIs(t, err, ErrMemoryNegativeLength)
		assert.Nil(t, got)
	})

	t.Run("lower bound", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, -3, 4)
		require.ErrorIs(t, err, ErrMemoryBadBounds)
		assert.Nil(t, got)
	})

	t.Run("upper bound", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, MemPtr(len(mem)+1), 1)
		require.ErrorIs(t, err, ErrMemoryBadBounds)
		assert.Nil(t, got)
	})

	t.Run("zero length ignores the offset", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, -100, 0)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, got)

		got, err = MemLoadBytes(mem, MemPtr(len(mem)+50), 0)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, got)
	})

	t.Run("result does not alias", func(t *testing.T) {
		t.Parallel()
		got, err := MemLoadBytes(mem, 1, 2)
		require.NoError(t, err)
		got[0] = 0xFF
		assert.Equal(t, byte(1), mem[1])
	})
}

func TestMemStoreBytes_Edges(t *testing.T) {
	t.Parallel()

	fresh := func() []byte {
		return []byte{0, 1, 2, 3, 4, 5, 6, 7}
	}

	t.Run("in range", func(t *testing.T) {
		t.Parallel()
		mem := fresh()
		require.NoError(t, MemStoreBytes(mem, 2, []byte{9, 8}))
		assert.Equal(t, []byte{0, 1, 9, 8, 4, 5, 6, 7}, mem)
	})

	t.Run("lower bound", func(t *testing.T) {
		t.Parallel()
		mem := fresh()
		err := MemStoreBytes(mem, -1, []byte{9})
		require.ErrorIs(t, err, ErrMemoryBadBoundsLower)
		assert.Equal(t, fresh(), mem)
	})

	t.Run("upper bound", func(t *testing.T) {
		t.Parallel()
		mem := fresh()
		err := MemStoreBytes(mem, 6, []byte{9, 8, 7})
		require.ErrorIs(t, err, ErrMemoryBadBoundsUpper)
		assert.Equal(t, fresh(), mem)
	})

	t.Run("zero length is a no-op at any offset", func(t *testing.T) {
		t.Parallel()
		mem := fresh()
		require.NoError(t, MemStoreBytes(mem, -5, nil))
		require.NoError(t, MemStoreBytes(mem, MemPtr(len(mem)+10), []byte{}))
		assert.Equal(t, fresh(), mem)
	})
}

// TestMemStoreRange_NarrowedLength pins the stores whose length does not fit
// in int32. The data length is narrowed to int32 as it always was. A length
// that narrows to a negative value must be rejected: the span would otherwise
// convert to a large uint32 and the native copy would run past the end of
// memory. The cases use lengths only, so nothing of that size is allocated.
func TestMemStoreRange_NarrowedLength(t *testing.T) {
	t.Parallel()

	const memoryLength = 65536

	tests := []struct {
		name       string
		memPtr     MemPtr
		dataLength int
		wantErr    error
	}{
		{"2 GiB narrows to MinInt32", 100, 1 << 31, ErrMemoryBadBoundsUpper},
		{"narrows to -50 with the end inside memory", 100, 1<<32 - 50, ErrMemoryBadBoundsUpper},
		{"narrows to -1 at offset zero", 0, 1<<32 - 1, ErrMemoryBadBoundsUpper},
		{"negative offset is still checked first", -1, 1 << 31, ErrMemoryBadBoundsLower},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			off, n, err := MemStoreRange(memoryLength, tt.memPtr, tt.dataLength)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, off)
			assert.Zero(t, n)
		})
	}

	t.Run("4 GiB narrows to zero and stays a no-op", func(t *testing.T) {
		t.Parallel()
		off, n, err := MemStoreRange(memoryLength, 100, 1<<32)
		require.NoError(t, err)
		assert.Zero(t, off)
		assert.Zero(t, n)
	})
}

// TestMemRange_OffsetSaturation pins what happens when memPtr + length passes
// MaxInt32. Offset clamps the end to MaxInt32 instead of wrapping. At any real
// memory size that end is past memory, so both checks fail. Only a memory
// larger than 2 GiB, which the page caps rule out, gets a span: the load is
// cut at MaxInt32, and the store reports success while writing only up to
// MaxInt32. Both are the historical behaviour and both stay inside memory.
func TestMemRange_OffsetSaturation(t *testing.T) {
	t.Parallel()

	const nearEnd = MemPtr(math.MaxInt32 - 1)

	t.Run("load past memory at a real size", func(t *testing.T) {
		t.Parallel()
		_, _, err := MemLoadRange(30*65536, nearEnd, 10)
		require.ErrorIs(t, err, ErrMemoryBadBounds)
	})

	t.Run("store past memory at a real size", func(t *testing.T) {
		t.Parallel()
		_, _, err := MemStoreRange(30*65536, nearEnd, 10)
		require.ErrorIs(t, err, ErrMemoryBadBoundsUpper)
	})

	t.Run("load is cut at MaxInt32 above 2 GiB", func(t *testing.T) {
		t.Parallel()
		off, n, err := MemLoadRange(math.MaxUint32, nearEnd, 10)
		require.NoError(t, err)
		assert.Equal(t, uint32(nearEnd), off)
		assert.Equal(t, uint32(1), n)
	})

	t.Run("store spans only up to MaxInt32 above 2 GiB", func(t *testing.T) {
		t.Parallel()
		off, n, err := MemStoreRange(math.MaxUint32, nearEnd, 10)
		require.NoError(t, err)
		assert.Equal(t, uint32(nearEnd), off)
		assert.Equal(t, uint32(1), n)
	})
}
