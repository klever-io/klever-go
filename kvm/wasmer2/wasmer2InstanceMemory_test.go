package wasmer2

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWasmer2Memory_DataRoundTrip characterises the []byte view Data() builds
// over wasmer-owned linear memory: it aliases live memory, spans it exactly,
// and survives the no-op Destroy(). It checks the view through MemStore and
// MemLoad rather than pinning unsafe.Slice itself.
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

	want := make([]byte, len(payload))
	func() {
		instance.mutex.Lock()
		defer instance.mutex.Unlock()

		view := instance.memory.Data()
		require.Equal(t, int(memoryLength), len(view))
		require.Equal(t, int(memoryLength), cap(view))
		assert.Equal(t, payload, view[:len(payload)])

		// Prove the returned view aliases live linear memory rather than copying
		// it: a write through the view must be observable via MemLoad.
		view[0] ^= 0xFF
		copy(want, view)
	}()

	loaded, err = instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, want, loaded)

	// Destroy is a documented no-op; the direct call (bypassing the instance
	// mutex) is safe because the instance is local to this test goroutine and
	// the body is empty. Memory must stay readable until Clean destroys it.
	instance.memory.Destroy()

	reloaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, want, reloaded)
}
