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

	// Destroy is a documented no-op; the direct call (bypassing the instance
	// mutex) is safe because this test is sequential and the body is empty.
	// The memory must remain readable until Clean destroys the instance.
	instance.memory.Destroy()

	reloaded, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, payload, reloaded)
}
