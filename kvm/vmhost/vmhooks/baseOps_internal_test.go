package vmhooks

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInternalMockVMHost() *contextmock.VMHostMock {
	gasSchedule := config.MakeGasMapForTests()

	mockMetering := &contextmock.MeteringContextMock{
		GasProvidedMock: 1000,
		GasLeftMock:     1000,
	}
	mockMetering.SetGasSchedule(gasSchedule)

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockRuntime.InitState()

	return &contextmock.VMHostMock{
		RuntimeContext:  mockRuntime,
		MeteringContext: mockMetering,
	}
}

func encodeArgLengths(lengths []int32) []byte {
	data := make([]byte, len(lengths)*4)
	for i, l := range lengths {
		binary.LittleEndian.PutUint32(data[i*4:], uint32(l)) // #nosec G115
	}
	return data
}

func TestArgumentsLengthByteCount(t *testing.T) {
	t.Parallel()

	// numArguments * 4 overflows int32 for any numArguments > math.MaxInt32/4.
	overflowBoundary := int32(math.MaxInt32/4 + 1)

	// values that must NOT overflow
	n, err := argumentsLengthByteCount(0)
	require.NoError(t, err)
	assert.Equal(t, int32(0), n)

	n, err = argumentsLengthByteCount(1)
	require.NoError(t, err)
	assert.Equal(t, int32(4), n)

	n, err = argumentsLengthByteCount(math.MaxInt32 / 4)
	require.NoError(t, err)
	assert.Equal(t, int32(math.MaxInt32/4)*4, n)

	// values that MUST overflow: boundary, midpoint, and max
	_, err = argumentsLengthByteCount(overflowBoundary)
	require.Error(t, err)

	_, err = argumentsLengthByteCount(math.MaxInt32 / 2)
	require.Error(t, err)

	_, err = argumentsLengthByteCount(math.MaxInt32)
	require.Error(t, err)
}

func TestVMHooksImpl_getArgumentsFromMemory(t *testing.T) {
	t.Parallel()

	t.Run("valid arguments", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		args := [][]byte{[]byte("arg1"), []byte("argument2")}
		lengths := []int32{int32(len(args[0])), int32(len(args[1]))}

		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))
		dataOffset := executor.MemPtr(100)
		offset := dataOffset
		for _, arg := range args {
			require.NoError(t, host.Runtime().GetInstance().MemStore(offset, arg))
			offset = offset.Offset(int32(len(arg)))
		}

		result, totalLen, err := context.getArgumentsFromMemory(host, int32(len(args)), executor.MemPtr(0), dataOffset)

		require.NoError(t, err)
		assert.Equal(t, args, result)
		assert.Equal(t, int32(len(args[0])+len(args[1])), totalLen)
	})

	t.Run("zero arguments", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		result, totalLen, err := context.getArgumentsFromMemory(host, 0, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Equal(t, [][]byte{}, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("negative numArguments is rejected", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		result, totalLen, err := context.getArgumentsFromMemory(host, -1, executor.MemPtr(0), executor.MemPtr(100))

		require.Error(t, err)
		assert.EqualError(t, err, "invalid numArguments (-1)")
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("numArguments above max is rejected", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		result, totalLen, err := context.getArgumentsFromMemory(host, maxNumArgumentsFromMemory+1, executor.MemPtr(0), executor.MemPtr(100))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("MemLoad failure for argument lengths is propagated", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		// Don't store anything; request lengths far beyond memory bounds.
		result, totalLen, err := context.getArgumentsFromMemory(host, 1, executor.MemPtr(1<<20), executor.MemPtr(0))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})
}
