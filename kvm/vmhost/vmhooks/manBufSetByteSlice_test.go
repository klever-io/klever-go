package vmhooks_test

import (
	"testing"

	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

// TestManagedBufferSetByteSliceWithTypedArgs covers the branches added when this function was
// switched from a GetBytes+SetBytes pair to the single-copy SetByteSlice: the handle-not-found
// fault path, the out-of-bounds non-fault path, and the write path.
func TestManagedBufferSetByteSliceWithTypedArgs(t *testing.T) {
	t.Run("handle not found faults", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		ret := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(vmHost, 999, 0, 1, []byte{0xAA})

		require.Equal(t, int32(1), ret)
	})

	t.Run("out of bounds does not fail execution", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes([]byte{1, 2, 3, 4})

		ret := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(vmHost, mBufferHandle, 2, 10, []byte{0xAA})

		require.Equal(t, int32(1), ret)
	})

	t.Run("overwrites the slice", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes([]byte{1, 2, 3, 4})

		ret := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(vmHost, mBufferHandle, 1, 2, []byte{0xAA, 0xBB})

		require.Equal(t, int32(0), ret)
		result, err := hooks.GetManagedTypesContext().GetBytes(mBufferHandle)
		require.NoError(t, err)
		require.Equal(t, []byte{1, 0xAA, 0xBB, 4}, result)
	})
}
