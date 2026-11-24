package vmhooks

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	kvmmath "github.com/klever-io/klever-go/kvm/math"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/vmcommon"
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
	// the stub enables every fork; V6 moves the argument data charge into getArgumentsFromMemory
	// and is switched on only by the tests covering it
	forkController.FixAuditChangesV6Value = false

	return &contextmock.VMHostMock{
		RuntimeContext:        mockRuntime,
		MeteringContext:       mockMetering,
		ForkControllerContext: forkController,
		// prices the copy as managedTypesContext does, so the charge lands on the metering mock
		ManagedTypesContext: &hostmock.ManagedTypesContextMock{
			ConsumeGasForByteLenBoundedCalled: func(byteLen uint64) error {
				dataCopyPerByte := mockMetering.GasSchedule().BaseOperationCost.DataCopyPerByte
				return mockMetering.UseGasBounded(kvmmath.MulUint64(byteLen, dataCopyPerByte))
			},
		},
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

func newInternalMockVMHostWithV6(gas uint64) *contextmock.VMHostMock {
	host := newInternalMockVMHostWithGas(true, gas)
	host.ForkControllerContext.(*commonMock.ForkControllerStub).FixAuditChangesV6Value = true
	return host
}

// Before FixAuditChangesV6 the argument data was loaded first and charged by the caller afterwards,
// so a contract could force up to maxTotalArgumentsBytes of copying before running out of gas.
// After it the data is charged here, bounded, before MemLoadMultiple, and nothing is left for the caller.
func TestVMHooksImpl_getArgumentsFromMemory_argumentDataGasCharge(t *testing.T) {
	t.Parallel()

	args := [][]byte{[]byte("arg1"), []byte("argument2")}
	lengths := []int32{int32(len(args[0])), int32(len(args[1]))}
	// DataCopyPerByte is 1 in the test gas schedule
	const lengthsCharge = uint64(2 * 4)
	dataCharge := uint64(len(args[0]) + len(args[1]))

	t.Run("the data is charged and nothing is left uncharged", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(1000)
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(100), append(append([]byte{}, args[0]...), args[1]...)))

		result, uncharged, err := NewVMHooksImpl(host).getArgumentsFromMemory(host, 2, executor.MemPtr(0), executor.MemPtr(100))

		require.NoError(t, err)
		assert.Equal(t, args, result)
		assert.Equal(t, int32(0), uncharged)
		assert.Equal(t, 1000-lengthsCharge-dataCharge, host.Metering().GasLeft())
	})

	// the data offset is out of bounds, so reaching MemLoadMultiple would fail with a memory error instead
	t.Run("one gas short fails before the data is loaded", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(lengthsCharge + dataCharge - 1)
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), encodeArgLengths(lengths)))

		result, uncharged, err := NewVMHooksImpl(host).getArgumentsFromMemory(host, 2, executor.MemPtr(0), executor.MemPtr(1<<20))

		require.ErrorIs(t, err, vmhost.ErrNotEnoughGas)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), uncharged)
		assert.Equal(t, dataCharge-1, host.Metering().GasLeft())
	})
}

func TestVMHooksImpl_memLoadCharged(t *testing.T) {
	t.Parallel()

	payload := []byte("hello")
	length := int32(len(payload))

	t.Run("after V6 the bytes are charged before the load", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(1000)
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), payload))

		data, uncharged, err := NewVMHooksImpl(host).memLoadCharged(host, executor.MemPtr(0), length)

		require.NoError(t, err)
		assert.Equal(t, payload, data)
		assert.Equal(t, int32(0), uncharged)
		assert.Equal(t, uint64(1000-len(payload)), host.Metering().GasLeft())
	})

	t.Run("after V6 one gas short fails and charges nothing", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(uint64(length - 1))

		data, uncharged, err := NewVMHooksImpl(host).memLoadCharged(host, executor.MemPtr(1<<20), length)

		require.ErrorIs(t, err, vmhost.ErrNotEnoughGas)
		assert.Nil(t, data)
		assert.Equal(t, int32(0), uncharged)
		assert.Equal(t, uint64(length-1), host.Metering().GasLeft())
	})

	t.Run("before V6 nothing is charged and the caller gets the full length", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(true, 1000)
		require.NoError(t, host.Runtime().GetInstance().MemStore(executor.MemPtr(0), payload))

		data, uncharged, err := NewVMHooksImpl(host).memLoadCharged(host, executor.MemPtr(0), length)

		require.NoError(t, err)
		assert.Equal(t, payload, data)
		assert.Equal(t, length, uncharged)
		assert.Equal(t, uint64(1000), host.Metering().GasLeft())
	})

	t.Run("after V6 a negative length is rejected by MemLoad, not by gas", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(1000)

		data, uncharged, err := NewVMHooksImpl(host).memLoadCharged(host, executor.MemPtr(0), -1)

		require.ErrorIs(t, err, executor.ErrMemoryNegativeLength)
		assert.Nil(t, data)
		assert.Equal(t, int32(0), uncharged)
		assert.Equal(t, uint64(1000), host.Metering().GasLeft())
	})
}

func TestVMHooksImpl_extractIndirectContractCallArguments_functionNameLength(t *testing.T) {
	t.Parallel()

	extract := func(host *contextmock.VMHostMock, functionLength int32) error {
		_, err := NewVMHooksImpl(host).extractIndirectContractCallArgumentsWithoutValue(
			host, executor.MemPtr(0), executor.MemPtr(100), functionLength, 0, executor.MemPtr(0), executor.MemPtr(0))
		return err
	}

	t.Run("after V6 a name at the limit is rejected before it is charged", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(1000)

		require.ErrorIs(t, extract(host, vmhost.MaxFunctionNameLength), vmhost.ErrInvalidFunctionName)
		assert.Equal(t, uint64(1000), host.Metering().GasLeft())
	})

	t.Run("after V6 the longest callable name is charged and loaded", func(t *testing.T) {
		host := newInternalMockVMHostWithV6(1000)

		require.NoError(t, extract(host, vmhost.MaxFunctionNameLength-1))
		assert.Equal(t, uint64(1000-(vmhost.MaxFunctionNameLength-1)), host.Metering().GasLeft())
	})

	t.Run("before V6 a name at the limit is still loaded, for free", func(t *testing.T) {
		host := newInternalMockVMHostWithGas(true, 1000)

		require.NoError(t, extract(host, vmhost.MaxFunctionNameLength))
		assert.Equal(t, uint64(1000), host.Metering().GasLeft())
	})
}

func TestPrepareIndirectContractCallInput_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	t.Run("recipient address is independent of the destination", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		destination := []byte("destination-address")

		input, err := prepareIndirectContractCallInput(host, []byte("sender"), big.NewInt(0), 100, destination, []byte("fn"), nil, 0)
		require.NoError(t, err)

		destination[0] = 'X'

		require.Equal(t, []byte("destination-address"), input.RecipientAddr)
	})

	t.Run("call value is independent of the source big int", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		value := big.NewInt(25)

		input, err := prepareIndirectContractCallInput(host, []byte("sender"), value, 100, []byte("destination"), []byte("fn"), nil, 0)
		require.NoError(t, err)
		require.Len(t, input.KDATransfers, 1)

		value.SetInt64(999)

		require.Equal(t, big.NewInt(25), input.KDATransfers[0].KDAValue)
		require.NotSame(t, value, input.KDATransfers[0].KDAValue)
	})

	t.Run("token name is independent of the KLV identifier", func(t *testing.T) {
		t.Parallel()

		input, err := prepareIndirectContractCallInput(newInternalMockVMHost(), []byte("sender"), big.NewInt(25), 100, []byte("destination"), []byte("fn"), nil, 0)
		require.NoError(t, err)
		require.Len(t, input.KDATransfers, 1)

		tokenName := input.KDATransfers[0].KDATokenName
		require.Equal(t, kdautils.KLVIdentifier, tokenName)
		require.NotSame(t, &kdautils.KLVIdentifier[0], &tokenName[0])
	})

	t.Run("negative call value stays negative", func(t *testing.T) {
		t.Parallel()

		input, err := prepareIndirectContractCallInput(newInternalMockVMHost(), []byte("sender"), big.NewInt(-25), 100, []byte("destination"), []byte("fn"), nil, 0)
		require.NoError(t, err)
		require.Len(t, input.KDATransfers, 1)

		require.Negative(t, input.KDATransfers[0].KDAValue.Sign())
		require.Equal(t, big.NewInt(-25), input.KDATransfers[0].KDAValue)
	})

	t.Run("nil destination stays nil", func(t *testing.T) {
		t.Parallel()

		input, err := prepareIndirectContractCallInput(newInternalMockVMHost(), []byte("sender"), big.NewInt(0), 100, nil, []byte("fn"), nil, 0)
		require.NoError(t, err)

		require.Nil(t, input.RecipientAddr)
	})

	t.Run("empty destination stays empty and non-nil", func(t *testing.T) {
		t.Parallel()

		input, err := prepareIndirectContractCallInput(newInternalMockVMHost(), []byte("sender"), big.NewInt(0), 100, []byte{}, []byte("fn"), nil, 0)
		require.NoError(t, err)

		require.NotNil(t, input.RecipientAddr)
		require.Empty(t, input.RecipientAddr)
	})

	t.Run("zero value appends no transfer", func(t *testing.T) {
		t.Parallel()

		input, err := prepareIndirectContractCallInput(newInternalMockVMHost(), []byte("sender"), big.NewInt(0), 100, []byte("destination"), []byte("fn"), nil, 0)
		require.NoError(t, err)

		require.Empty(t, input.KDATransfers)
	})
}

func TestCreateContract_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	t.Run("call value is independent of the source big int", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		var captured *vmcommon.ContractCreateInput
		host.CreateNewContractCalled = func(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error) {
			captured = input
			return []byte("new-address"), nil
		}
		value := big.NewInt(25)

		_, err := createContract([]byte("sender"), nil, value, 100, []byte("code"), []byte("metadata"), host, CreateContract)
		require.NoError(t, err)
		require.NotNil(t, captured)
		require.Len(t, captured.KDATransfers, 1)

		value.SetInt64(999)

		require.Equal(t, big.NewInt(25), captured.KDATransfers[0].KDAValue)
		require.NotSame(t, value, captured.KDATransfers[0].KDAValue)
	})

	t.Run("token name is independent of the KLV identifier", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		var captured *vmcommon.ContractCreateInput
		host.CreateNewContractCalled = func(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error) {
			captured = input
			return []byte("new-address"), nil
		}

		_, err := createContract([]byte("sender"), nil, big.NewInt(25), 100, []byte("code"), []byte("metadata"), host, CreateContract)
		require.NoError(t, err)
		require.NotNil(t, captured)
		require.Len(t, captured.KDATransfers, 1)

		tokenName := captured.KDATransfers[0].KDATokenName
		require.Equal(t, kdautils.KLVIdentifier, tokenName)
		require.NotSame(t, &kdautils.KLVIdentifier[0], &tokenName[0])
	})

	t.Run("negative call value stays negative", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		var captured *vmcommon.ContractCreateInput
		host.CreateNewContractCalled = func(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error) {
			captured = input
			return []byte("new-address"), nil
		}

		_, err := createContract([]byte("sender"), nil, big.NewInt(-25), 100, []byte("code"), []byte("metadata"), host, CreateContract)
		require.NoError(t, err)
		require.NotNil(t, captured)
		require.Len(t, captured.KDATransfers, 1)

		require.Negative(t, captured.KDATransfers[0].KDAValue.Sign())
		require.Equal(t, big.NewInt(-25), captured.KDATransfers[0].KDAValue)
	})

	t.Run("zero value appends no transfer", func(t *testing.T) {
		t.Parallel()

		host := newInternalMockVMHost()
		var captured *vmcommon.ContractCreateInput
		host.CreateNewContractCalled = func(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error) {
			captured = input
			return []byte("new-address"), nil
		}

		_, err := createContract([]byte("sender"), nil, big.NewInt(0), 100, []byte("code"), []byte("metadata"), host, CreateContract)
		require.NoError(t, err)
		require.NotNil(t, captured)
		require.Empty(t, captured.KDATransfers)
	})
}
