package wasmer2

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWasmer2Memory_LoadStoreRoundTrip checks that a store is visible to a
// later load and that neither the loaded bytes nor MemDump alias live memory.
func TestWasmer2Memory_LoadStoreRoundTrip(t *testing.T) {
	t.Parallel()

	instance := newLiveInstance(t)
	defer func() {
		require.True(t, instance.Clean())
	}()

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	require.NoError(t, instance.MemStore(executor.MemPtr(0), payload))

	loaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, loaded)

	loaded[0] ^= 0xFF
	reloaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, reloaded)

	dump := instance.MemDump()
	require.Equal(t, int(instance.MemLength()), len(dump))
	assert.Equal(t, payload, dump[:len(payload)])
	dump[0] ^= 0xFF
	reloaded, err = instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, reloaded)
}

func TestWasmer2Memory_LoadStoreEdges(t *testing.T) {
	t.Parallel()

	instance := newLiveInstance(t)
	defer instance.Clean()

	memLen := instance.MemLength()
	require.NotZero(t, memLen)

	payload := []byte("test data")
	tail := executor.MemPtr(memLen - uint32(len(payload)))
	require.NoError(t, instance.MemStore(tail, payload))

	t.Run("truncated", func(t *testing.T) {
		got, err := instance.MemLoad(executor.MemPtr(memLen-4), 9)
		require.NoError(t, err)
		assert.Equal(t, []byte("data"), got)
	})

	t.Run("starts at the end", func(t *testing.T) {
		got, err := instance.MemLoad(executor.MemPtr(memLen), 4)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, got)
	})

	t.Run("negative length", func(t *testing.T) {
		got, err := instance.MemLoad(10, -2)
		require.ErrorIs(t, err, executor.ErrMemoryNegativeLength)
		assert.Nil(t, got)
	})

	t.Run("negative offset wins over negative length", func(t *testing.T) {
		got, err := instance.MemLoad(-1, -2)
		require.ErrorIs(t, err, executor.ErrMemoryBadBounds)
		assert.NotErrorIs(t, err, executor.ErrMemoryNegativeLength)
		assert.Nil(t, got)
	})

	t.Run("load lower bound", func(t *testing.T) {
		got, err := instance.MemLoad(-3, 4)
		require.ErrorIs(t, err, executor.ErrMemoryBadBounds)
		assert.Nil(t, got)
	})

	t.Run("load upper bound", func(t *testing.T) {
		got, err := instance.MemLoad(executor.MemPtr(memLen+1), 1)
		require.ErrorIs(t, err, executor.ErrMemoryBadBounds)
		assert.Nil(t, got)
	})

	t.Run("zero length load ignores the offset", func(t *testing.T) {
		got, err := instance.MemLoad(-100, 0)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, got)
	})

	t.Run("store lower bound", func(t *testing.T) {
		err := instance.MemStore(-2, []byte("nope"))
		require.ErrorIs(t, err, executor.ErrMemoryBadBoundsLower)
		got, loadErr := instance.MemLoad(tail, executor.MemLength(len(payload)))
		require.NoError(t, loadErr)
		assert.Equal(t, payload, got)
	})

	t.Run("store upper bound", func(t *testing.T) {
		err := instance.MemStore(executor.MemPtr(memLen-4), []byte("too-long"))
		require.ErrorIs(t, err, executor.ErrMemoryBadBoundsUpper)
		got, loadErr := instance.MemLoad(tail, executor.MemLength(len(payload)))
		require.NoError(t, loadErr)
		assert.Equal(t, payload, got)
	})

	t.Run("zero length store is a no-op", func(t *testing.T) {
		require.NoError(t, instance.MemStore(-5, nil))
		require.NoError(t, instance.MemStore(executor.MemPtr(memLen+10), []byte{}))
		got, err := instance.MemLoad(tail, executor.MemLength(len(payload)))
		require.NoError(t, err)
		assert.Equal(t, payload, got)
	})
}

func TestWasmer2Memory_GrowThenLoadStore(t *testing.T) {
	t.Parallel()

	instance := newLiveInstance(t)
	defer instance.Clean()

	before := instance.MemLength()
	require.NotZero(t, before)

	payload := []byte{0x11, 0x22, 0x33, 0x44}
	require.NoError(t, instance.MemStore(0, payload))

	require.NoError(t, instance.MemGrow(1))
	require.Equal(t, before+65536, instance.MemLength())

	// The page that did not exist before the grow accepts a store, and the
	// bytes written before the grow are still there.
	grownAt := executor.MemPtr(before)
	grown := []byte("grown")
	require.NoError(t, instance.MemStore(grownAt, grown))

	got, err := instance.MemLoad(grownAt, executor.MemLength(len(grown)))
	require.NoError(t, err)
	assert.Equal(t, grown, got)

	got, err = instance.MemLoad(0, executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	// A store that runs past the new end is still rejected, and it does not
	// change the byte at the end of the grown memory.
	end := instance.MemLength()
	require.ErrorIs(t, instance.MemStore(executor.MemPtr(end-2), []byte{1, 2, 3, 4}), executor.ErrMemoryBadBoundsUpper)
	got, err = instance.MemLoad(executor.MemPtr(end-2), 2)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0}, got)
}

// wasmMemoryModule is a module whose only export is a memory with the given
// minimum and, when hasMax is set, maximum. Page counts must be small enough
// that every LEB128 and section length fits in one byte.
func wasmMemoryModule(minPages, maxPages byte, hasMax bool) []byte {
	body := []byte{1} // one memory
	if hasMax {
		body = append(body, 1, minPages, maxPages)
	} else {
		body = append(body, 0, minPages)
	}
	memorySection := append([]byte{0x05, byte(len(body))}, body...)

	exportBody := append([]byte{1, 6}, []byte("memory")...)
	exportBody = append(exportBody, 0x02, 0x00) // kind memory, index 0
	exportSection := append([]byte{0x07, byte(len(exportBody))}, exportBody...)

	module := append([]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}, memorySection...)
	return append(module, exportSection...)
}

// errNativeMemoryCap is the bundled native executor's rejection of a memory
// above its page cap. The text comes from the Rust library, not from this
// repo, so a reworded native release fails here first. It is asserted rather
// than relying on require.Error alone: a hand-assembled module that is merely
// malformed would also fail to instantiate and make the cap tests pass
// without checking the cap.
const errNativeMemoryCap = "memory size exceeds maximum allowed"

// TestWasmer2_DeclaredMemoryPageCap pins the executor limit that keeps a
// 4 GiB memory unreachable. The C API reports the length as uint64_t and the
// bridge narrows it to uint32, which would wrap to 0 at exactly 65536 pages.
// The bundled executor rejects a declared minimum or maximum above 20 pages,
// and TestWasmer2_MemoryGrowCap pins the growth on top, so the narrowing
// never drops a bit.
func TestWasmer2_DeclaredMemoryPageCap(t *testing.T) {
	exec, err := CreateExecutor()
	require.NoError(t, err)

	options := executor.CompilationOptions{
		GasLimit:             100_000_000,
		RuntimeBreakpoints:   true,
		MaxDeclaredTableSize: math.MaxUint64,
	}

	allowed, err := exec.NewInstanceWithOptions(wasmMemoryModule(1, 20, true), options)
	require.NoError(t, err)
	require.Equal(t, uint32(65536), allowed.MemLength())
	require.True(t, allowed.Clean())

	_, err = exec.NewInstanceWithOptions(wasmMemoryModule(1, 21, true), options)
	require.Error(t, err)
	assert.ErrorContains(t, err, errNativeMemoryCap)

	// Without a declared maximum the minimum is held to the same cap.
	allowed, err = exec.NewInstanceWithOptions(wasmMemoryModule(20, 0, false), options)
	require.NoError(t, err)
	require.Equal(t, uint32(20*65536), allowed.MemLength())
	require.True(t, allowed.Clean())

	_, err = exec.NewInstanceWithOptions(wasmMemoryModule(21, 0, false), options)
	require.Error(t, err)
	assert.ErrorContains(t, err, errNativeMemoryCap)

	// 65536 pages is exactly 4 GiB, the size at which the uint32 length
	// would wrap. The minimum stays 1 page, so instantiation does not
	// allocate 4 GiB; the declared maximum fails the same 20-page check as 21.
	over, err := hex.DecodeString("0061736d010000000506010101808004070a01066d656d6f72790200")
	require.NoError(t, err)
	_, err = exec.NewInstanceWithOptions(over, options)
	require.Error(t, err)
	assert.ErrorContains(t, err, errNativeMemoryCap)
}

// wasmGrowModule is a 1-page memory module exporting "grow", which runs
// memory.grow(delta) the given number of times and drops each result.
// Every immediate and size is one LEB128 byte: delta is a signed i32.const,
// so it must be at most 63, and the code section stays within 127 bytes only
// up to 24 repetitions.
func wasmGrowModule(delta byte, times int) []byte {
	if delta > 63 || times < 0 || times > 24 {
		panic("wasmGrowModule: delta must be <= 63 and times in [0, 24]")
	}

	module := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	module = append(module, 0x01, 0x04, 0x01, 0x60, 0x00, 0x00) // type () -> ()
	module = append(module, 0x03, 0x02, 0x01, 0x00)             // one function of type 0
	module = append(module, 0x05, 0x03, 0x01, 0x00, 0x01)       // memory: min 1, no max

	exports := append([]byte{0x02, 0x06}, []byte("memory")...)
	exports = append(exports, 0x02, 0x00, 0x04)
	exports = append(exports, []byte("grow")...)
	exports = append(exports, 0x00, 0x00)
	module = append(module, 0x07, byte(len(exports)))
	module = append(module, exports...)

	body := []byte{0x00} // no locals
	for range times {
		body = append(body, 0x41, delta, 0x40, 0x00, 0x1a) // i32.const delta; memory.grow; drop
	}
	body = append(body, 0x0b)
	code := append([]byte{0x01, byte(len(body))}, body...)
	module = append(module, 0x0a, byte(len(code)))
	return append(module, code...)
}

// TestWasmer2_MemoryGrowCap pins the other half of the 4 GiB argument: with
// the consensus gas schedule (MaxMemoryGrow 10, MaxMemoryGrowDelta 1) a
// contract grows its memory by at most 10 pages over the instance's lifetime,
// a larger delta is refused outright, and Reset restores the declared size.
func TestWasmer2_MemoryGrowCap(t *testing.T) {
	exec, err := CreateExecutor()
	require.NoError(t, err)

	options := executor.CompilationOptions{
		GasLimit:             100_000_000,
		RuntimeBreakpoints:   true,
		Metering:             true,
		MaxMemoryGrow:        10,
		MaxMemoryGrowDelta:   1,
		MaxDeclaredTableSize: math.MaxUint64,
	}

	call := func(instance executor.Instance) error {
		instance.SetGasLimit(options.GasLimit)
		instance.SetPointsUsed(0)
		return instance.CallFunction("grow")
	}

	// Two calls of 5 grows succeed; the third would pass the lifetime cap.
	instance, err := exec.NewInstanceWithOptions(wasmGrowModule(1, 5), options)
	require.NoError(t, err)
	require.NoError(t, call(instance))
	require.NoError(t, call(instance))
	require.Equal(t, uint32(11*65536), instance.MemLength())
	require.Error(t, call(instance))
	require.Equal(t, uint32(11*65536), instance.MemLength())
	require.True(t, instance.Reset())
	require.Equal(t, uint32(65536), instance.MemLength())
	require.True(t, instance.Clean())

	instance, err = exec.NewInstanceWithOptions(wasmGrowModule(5, 1), options)
	require.NoError(t, err)
	require.Error(t, call(instance))
	require.Equal(t, uint32(65536), instance.MemLength())
	require.True(t, instance.Clean())
}

// TestBytesAt_Bounds checks that bytesAt views exactly the requested span and
// panics, rather than reading or writing outside memory, when a caller hands
// it a span the range helpers would never produce.
func TestBytesAt_Bounds(t *testing.T) {
	t.Parallel()

	buf := [8]byte{0, 1, 2, 3, 4, 5, 6, 7}
	base := &buf[0]
	const memLen = uint32(len(buf))

	view := bytesAt(base, memLen, 2, 3)
	assert.Equal(t, []byte{2, 3, 4}, view)
	assert.Equal(t, 3, cap(view))
	view[0] = 0xFF
	assert.Equal(t, byte(0xFF), buf[2], "the view aliases the memory it was built over")

	assert.Equal(t, []byte{5, 6, 7}, bytesAt(base, memLen, 5, 3), "a span ending exactly at the end is allowed")
	assert.Equal(t, []byte{}, bytesAt(base, memLen, memLen, 0), "an empty span at the end is allowed")
	assert.Equal(t, []byte{}, bytesAt(nil, 0, 0, 0), "an empty span needs no base")

	assert.Panics(t, func() { bytesAt(base, memLen, 6, 3) }, "end past memory")
	assert.Panics(t, func() { bytesAt(base, memLen, memLen+1, 0) }, "offset past memory")
	assert.Panics(t, func() { bytesAt(base, memLen, 1, math.MaxUint32) }, "length that would wrap the end")
	assert.Panics(t, func() { bytesAt(nil, memLen, 0, 1) }, "nil base")
}
