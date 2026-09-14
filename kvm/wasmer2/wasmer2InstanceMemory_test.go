package wasmer2

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWasmer2Memory_DataRoundTrip pins the behaviour of the []byte view that
// Data() builds over the wasmer-owned linear memory: bytes stored through the
// view must read back identically, and the view must span the full memory.
func TestWasmer2Memory_DataRoundTrip(t *testing.T) {
	t.Parallel()

	instance := newLiveInstance(t)
	defer func() {
		require.True(t, instance.Clean())
	}()

	memoryLength := instance.MemLength()
	require.NotZero(t, memoryLength)

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	require.NoError(t, instance.MemStore(executor.MemPtr(0), payload))

	loaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, loaded)

	dump := instance.MemDump()
	require.Equal(t, int(memoryLength), len(dump))
	require.Equal(t, int(memoryLength), cap(dump))
	assert.Equal(t, payload, dump[:len(payload)])

	// Prove the returned view aliases live linear memory rather than copying
	// it: a write through the view must be observable via MemLoad.
	dump[0] ^= 0xFF
	loaded, err = instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, dump[:len(payload)], loaded)

	// Destroy is a documented no-op; the direct call (bypassing the instance
	// mutex) is safe because the instance is local to this test goroutine and
	// the body is empty. Memory must stay readable until Clean destroys it.
	instance.memory.Destroy()

	reloaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, dump[:len(payload)], reloaded)
}
