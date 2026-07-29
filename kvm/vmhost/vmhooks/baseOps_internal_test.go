package vmhooks

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	kvmmath "github.com/klever-io/klever-go/kvm/math"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInternalMockVMHost() *contextmock.VMHostMock {
	return newInternalMockVMHostWithGas(true, 1000)
}

func newInternalMockVMHostWithGas(fixAuditChangesV4 bool, gas uint64) *contextmock.VMHostMock {
	gasSchedule := config.MakeGasMapForTests()

	mockMetering := &contextmock.MeteringContextMock{
		GasProvidedMock: gas,
		GasLeftMock:     gas,
	}
	mockMetering.SetGasSchedule(gasSchedule)

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockRuntime.InitState()

	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV4Value = fixAuditChangesV4

	return &contextmock.VMHostMock{
		RuntimeContext:        mockRuntime,
		MeteringContext:       mockMetering,
		ForkControllerContext: forkController,
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

func TestVMHooksImpl_WriteLogRejectsTopicCountAboveMaximum(t *testing.T) {
	t.Parallel()

	host := newInternalMockVMHost()
	host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: true}
	host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = true

	context := NewVMHooksImpl(host)
	context.WriteLog(executor.MemPtr(0), 0, executor.MemPtr(100), maxNumTopics+1)

	require.EqualError(
		t,
		host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr,
		fmt.Sprintf("invalid numTopics (%d)", maxNumTopics+1),
	)
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

	t.Run("negative argument length is rejected", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		lengths := []int32{4, -1}
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))

		result, totalLen, err := context.getArgumentsFromMemory(host, int32(len(lengths)), executor.MemPtr(0), executor.MemPtr(100))

		require.Error(t, err)
		assert.EqualError(t, err, "invalid argument length (-1)")
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("total argument length above max is rejected", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		// Two lengths that sum to just past the limit without overflowing int32.
		// The limit is checked before any argument data is loaded, so no data
		// needs to be stored.
		lengths := []int32{maxTotalArgumentsBytes/2 + 1, maxTotalArgumentsBytes/2 + 1}
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))

		result, totalLen, err := context.getArgumentsFromMemory(host, int32(len(lengths)), executor.MemPtr(0), executor.MemPtr(100))

		require.Error(t, err)
		assert.EqualError(t, err, "total argument length exceeds maximum")
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("total argument length int32 overflow is rejected", func(t *testing.T) {
		host := newInternalMockVMHost()
		context := NewVMHooksImpl(host)

		// Two lengths whose sum wraps past math.MaxInt32. The per-addition
		// overflow guard fires inside the accumulation loop, before the
		// max-total check and before any data is loaded.
		lengths := []int32{math.MaxInt32, math.MaxInt32}
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))

		result, totalLen, err := context.getArgumentsFromMemory(host, int32(len(lengths)), executor.MemPtr(0), executor.MemPtr(100))

		require.Error(t, err)
		assert.ErrorIs(t, err, kvmmath.ErrAdditionOverflow)
		assert.Contains(t, err.Error(), "total argument length overflow")
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})
}

// The lengths array itself (4 bytes per argument) used to be loaded and parsed
// for free: only the argument data was ever charged for. A contract could pass
// a huge numArguments with almost no gas and still force the host to load and
// parse numArguments*4 bytes. After the fork that read is metered up-front, at
// DataCopyPerByte, before any memory is touched.
func TestVMHooksImpl_getArgumentsFromMemory_argumentLengthsGasCharge(t *testing.T) {
	t.Parallel()

	// DataCopyPerByte is 1 in the test gas schedule, so the expected charge is
	// simply the byte length of the lengths array.
	const numArgs = 2
	const expectedCharge = uint64(numArgs * 4)

	storeArgs := func(t *testing.T, host *contextmock.VMHostMock, dataOffset executor.MemPtr) [][]byte {
		t.Helper()
		args := [][]byte{[]byte("arg1"), []byte("argument2")}
		lengths := []int32{int32(len(args[0])), int32(len(args[1]))}

		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))
		offset := dataOffset
		for _, arg := range args {
			require.NoError(t, host.Runtime().GetInstance().MemStore(offset, arg))
			offset = offset.Offset(int32(len(arg)))
		}
		return args
	}

	t.Run("after fork: the lengths array is charged for", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(true, 1000)
		context := NewVMHooksImpl(host)
		args := storeArgs(t, host, executor.MemPtr(100))

		result, totalLen, err := context.getArgumentsFromMemory(host, numArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Equal(t, args, result)
		assert.Equal(t, int32(len(args[0])+len(args[1])), totalLen)
		assert.Equal(t, 1000-expectedCharge, host.Metering().GasLeft())
	})

	t.Run("before fork: the lengths array is free", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(false, 1000)
		context := NewVMHooksImpl(host)
		args := storeArgs(t, host, executor.MemPtr(100))

		result, totalLen, err := context.getArgumentsFromMemory(host, numArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Equal(t, args, result)
		assert.Equal(t, int32(len(args[0])+len(args[1])), totalLen)
		assert.Equal(t, uint64(1000), host.Metering().GasLeft())
	})

	t.Run("after fork: one gas short of the lengths array fails", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(true, expectedCharge-1)
		context := NewVMHooksImpl(host)
		storeArgs(t, host, executor.MemPtr(100))

		result, totalLen, err := context.getArgumentsFromMemory(host, numArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.ErrorIs(t, err, vmhost.ErrNotEnoughGas)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)
	})

	t.Run("after fork: exactly the charge succeeds and leaves no gas", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(true, expectedCharge)
		context := NewVMHooksImpl(host)
		args := storeArgs(t, host, executor.MemPtr(100))

		result, totalLen, err := context.getArgumentsFromMemory(host, numArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Equal(t, args, result)
		assert.Equal(t, int32(len(args[0])+len(args[1])), totalLen)
		assert.Equal(t, uint64(0), host.Metering().GasLeft())
	})

	// The whole point of the charge: a large numArguments must be rejected on
	// gas before the host loads and parses the lengths array, not after.
	t.Run("after fork: a large numArguments runs out of gas, before fork it does not", func(t *testing.T) {
		const manyArgs = int32(1000) // 4000 bytes of lengths, more than the 1000 gas provided

		hostAfterFork := newInternalMockVMHostWithGas(true, 1000)
		result, totalLen, err := NewVMHooksImpl(hostAfterFork).getArgumentsFromMemory(
			hostAfterFork, manyArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.ErrorIs(t, err, vmhost.ErrNotEnoughGas)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), totalLen)

		// Same call, same gas, before the fork: all 4000 bytes get loaded and
		// parsed into 1000 (zero-length) arguments for free.
		hostBeforeFork := newInternalMockVMHostWithGas(false, 1000)
		result, totalLen, err = NewVMHooksImpl(hostBeforeFork).getArgumentsFromMemory(
			hostBeforeFork, manyArgs, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Len(t, result, int(manyArgs))
		assert.Equal(t, int32(0), totalLen)
		assert.Equal(t, uint64(1000), hostBeforeFork.Metering().GasLeft())
	})

	t.Run("zero arguments are charged nothing on either side of the fork", func(t *testing.T) {
		for _, fixAuditChangesV4 := range []bool{true, false} {
			host := newInternalMockVMHostWithGas(fixAuditChangesV4, 1000)
			context := NewVMHooksImpl(host)

			result, totalLen, err := context.getArgumentsFromMemory(host, 0, executor.MemPtr(0), executor.MemPtr(100))

			require.NoError(t, err)
			assert.Equal(t, [][]byte{}, result)
			assert.Equal(t, int32(0), totalLen)
			assert.Equal(t, uint64(1000), host.Metering().GasLeft())
		}
	})
}
