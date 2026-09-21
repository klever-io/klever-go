package vmhooks

import (
	"bytes"
	"math/big"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/math"
	twos "github.com/klever-io/klever-go/kvm/math/twos-complement"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

const (
	mBufferNewName                = "mBufferNew"
	mBufferNewFromBytesName       = "mBufferNewFromBytes"
	mBufferGetLengthName          = "mBufferGetLength"
	mBufferGetBytesName           = "mBufferGetBytes"
	mBufferGetByteSliceName       = "mBufferGetByteSlice"
	mBufferCopyByteSliceName      = "mBufferCopyByteSlice"
	mBufferEqName                 = "mBufferEq"
	mBufferSetBytesName           = "mBufferSetBytes"
	mBufferAppendName             = "mBufferAppend"
	mBufferAppendBytesName        = "mBufferAppendBytes"
	mBufferToBigIntUnsignedName   = "mBufferToBigIntUnsigned"
	mBufferToBigIntSignedName     = "mBufferToBigIntSigned"
	mBufferFromBigIntUnsignedName = "mBufferFromBigIntUnsigned"
	mBufferFromBigIntSignedName   = "mBufferFromBigIntSigned"
	mBufferStorageStoreName       = "mBufferStorageStore"
	mBufferStorageLoadName        = "mBufferStorageLoad"
	mBufferGetArgumentName        = "mBufferGetArgument"
	mBufferFinishName             = "mBufferFinish"
	mBufferSetRandomName          = "mBufferSetRandom"
	mBufferToBigFloatName         = "mBufferToBigFloat"
	mBufferFromBigFloatName       = "mBufferFromBigFloat"
)

// maxManagedBufferLength bounds the bytes a single managed-buffer hook will accept. It is the
// same ceiling as maxTotalArgumentsBytes, the established limit in this package for a single
// VM-hook-processed blob, so the two cannot drift apart.
const maxManagedBufferLength = maxTotalArgumentsBytes

// consumeGasForBuffersBounded charges bounded per-byte gas for the current length of each
// handle, before the buffers are cloned. Pre-fork it charges nothing, so the legacy call sites
// keep their own unbounded charging untouched.
func consumeGasForBuffersBounded(host vmhost.VMHost, mBufferHandles ...int32) error {
	if !host.ForkController().FixAuditChangesV5() {
		return nil
	}

	managedType := host.ManagedTypes()
	for _, mBufferHandle := range mBufferHandles {
		length := managedType.GetLength(mBufferHandle)
		if length <= 0 {
			continue
		}

		// #nosec G115
		err := managedType.ConsumeGasForByteLenBounded(uint64(length))
		if err != nil {
			return err
		}
	}

	return nil
}

// validateCappedLength rejects a caller-supplied length that is negative or above the
// managed-buffer cap, before it is used to size any host work
func validateCappedLength(dataLength executor.MemLength) error {
	if dataLength < 0 || dataLength > maxManagedBufferLength {
		return vmhost.ErrManagedBufferLengthExceedsMaximum
	}

	return nil
}

// consumeGasForCappedLength validates a caller-supplied length and charges bounded per-byte gas
// for it, before the bytes are loaded
func consumeGasForCappedLength(managedType vmhost.ManagedTypesContext, dataLength executor.MemLength) error {
	err := validateCappedLength(dataLength)
	if err != nil {
		return err
	}

	// #nosec G115
	return managedType.ConsumeGasForByteLenBounded(uint64(dataLength))
}

// post-fork the flat and the per-byte gas are bounded and paid before the buffer is read
func consumeBufferReadGas(host vmhost.VMHost, gasToUse uint64, mBufferHandle int32) error {
	metering := host.Metering()
	if !host.ForkController().FixAuditChangesV5() {
		metering.UseAndTraceGas(gasToUse)
		return nil
	}

	err := metering.UseGasBounded(gasToUse)
	if err != nil {
		return err
	}

	return consumeGasForBuffersBounded(host, mBufferHandle)
}

// useBoundedGas charges gasToUse under functionName's own gas trace. Post-fork the charge is
// bounded so it cannot be overdrawn before the host work. The amount is whatever the caller
// computed and may already include a per-byte component (MBufferSetRandom's does), so this is
// not necessarily a flat charge.
func useBoundedGas(host vmhost.VMHost, functionName string, gasToUse uint64) error {
	metering := host.Metering()
	if !host.ForkController().FixAuditChangesV5() {
		metering.UseGasAndAddTracedGas(functionName, gasToUse)
		return nil
	}

	return metering.UseGasBoundedAndAddTracedGas(functionName, gasToUse)
}

// MBufferNew VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferNew() int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferNew
	metering.UseGasAndAddTracedGas(mBufferNewName, gasToUse)

	return managedType.NewManagedBuffer()
}

// MBufferNewFromBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferNewFromBytes(dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferNewFromBytes
	// post-fork the flat and the per-byte gas are bounded and paid before the memory load
	err := useBoundedGas(context.host, mBufferNewFromBytesName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}

	if context.host.ForkController().FixAuditChangesV5() {
		err = consumeGasForCappedLength(managedType, dataLength)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return -1
		}
	}

	data, err := context.MemLoad(dataOffset, dataLength)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}

	return managedType.NewManagedBufferFromBytes(data)
}

// MBufferGetLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferGetLength(mBufferHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferGetLength
	metering.UseGasAndAddTracedGas(mBufferGetLengthName, gasToUse)

	length := managedType.GetLength(mBufferHandle)
	if length == -1 {
		_ = context.WithFault(vmhost.ErrNoManagedBufferUnderThisHandle, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return -1
	}

	return length
}

// MBufferGetBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferGetBytes(mBufferHandle int32, resultOffset executor.MemPtr) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferGetBytesName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferGetBytes
	// post-fork the flat and the per-byte gas are bounded and paid before the buffer clone
	err := consumeBufferReadGas(context.host, gasToUse, mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	mBufferBytes, err := managedType.GetBytes(mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(mBufferBytes)
	}

	err = context.MemStore(resultOffset, mBufferBytes)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// MBufferGetByteSlice VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferGetByteSlice(
	sourceHandle int32,
	startingPosition int32,
	sliceLength int32,
	resultOffset executor.MemPtr) int32 {

	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferGetByteSliceName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferGetByteSlice
	// post-fork the flat and the per-byte gas are bounded and paid before the buffer clone
	err := consumeBufferReadGas(context.host, gasToUse, sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sourceBytes, err := managedType.GetBytes(sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(sourceBytes)
	}

	if vmhost.SliceIsOutOfBounds(fixAuditV5, startingPosition, sliceLength, len(sourceBytes)) {
		// does not fail execution if slice exceeds bounds
		return 1
	}

	slice := sourceBytes[startingPosition : startingPosition+sliceLength]
	err = context.MemStore(resultOffset, slice)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// MBufferCopyByteSlice VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferCopyByteSlice(sourceHandle int32, startingPosition int32, sliceLength int32, destinationHandle int32) int32 {
	host := context.GetVMHost()
	return ManagedBufferCopyByteSliceWithHost(host, sourceHandle, startingPosition, sliceLength, destinationHandle)
}

// ManagedBufferCopyByteSliceWithHost VMHooks implementation.
func ManagedBufferCopyByteSliceWithHost(host vmhost.VMHost, sourceHandle int32, startingPosition int32, sliceLength int32, destinationHandle int32) int32 {
	managedType := host.ManagedTypes()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(mBufferCopyByteSliceName)

	fixAuditV5 := host.ForkController().FixAuditChangesV5()
	// post-fork the destination write is capped before any gas is charged, so the ceiling holds
	// whatever the gas schedule prices a byte at
	if fixAuditV5 && sliceLength > maxManagedBufferLength {
		_ = WithFaultAndHost(host, vmhost.ErrManagedBufferLengthExceedsMaximum, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return 1
	}

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferCopyByteSlice
	// post-fork the flat and the per-byte gas are bounded and paid before the buffer clone
	err := consumeBufferReadGas(host, gasToUse, sourceHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sourceBytes, err := managedType.GetBytes(sourceHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(sourceBytes)
	}

	if vmhost.SliceIsOutOfBounds(fixAuditV5, startingPosition, sliceLength, len(sourceBytes)) {
		// does not fail execution if slice exceeds bounds
		return 1
	}

	slice := sourceBytes[startingPosition : startingPosition+sliceLength]
	gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, uint64(len(slice)))
	// post-fork the destination write is paid before it happens
	if fixAuditV5 {
		err = metering.UseGasBounded(gasToUse)
		if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		managedType.SetBytes(destinationHandle, slice)
		return 0
	}

	managedType.SetBytes(destinationHandle, slice)
	metering.UseAndTraceGas(gasToUse)

	return 0
}

// MBufferEq VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferEq(mBufferHandle1 int32, mBufferHandle2 int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferEqName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferCopyByteSlice
	// post-fork the flat gas is bounded and each buffer is paid for before it is cloned
	err := consumeBufferReadGas(context.host, gasToUse, mBufferHandle1)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}

	bytes1, err := managedType.GetBytes(mBufferHandle1)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}
	if fixAuditV5 {
		err = consumeGasForBuffersBounded(context.host, mBufferHandle2)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return -1
		}
	} else {
		managedType.ConsumeGasForBytes(bytes1)
	}

	bytes2, err := managedType.GetBytes(mBufferHandle2)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(bytes2)
	}

	if bytes.Equal(bytes1, bytes2) {
		return 1
	}

	return 0
}

// MBufferSetBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferSetBytes(mBufferHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferSetBytesName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferSetBytes
	// post-fork the write is capped and the flat and per-byte gas are bounded and paid before the memory load
	if fixAuditV5 {
		err := metering.UseGasBounded(gasToUse)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		err = consumeGasForCappedLength(managedType, dataLength)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	data, err := context.MemLoad(dataOffset, dataLength)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(data)
	}
	managedType.SetBytes(mBufferHandle, data)

	return 0
}

// MBufferSetByteSlice VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferSetByteSlice(
	mBufferHandle int32,
	startingPosition int32,
	dataLength executor.MemLength,
	dataOffset executor.MemPtr) int32 {

	host := context.GetVMHost()
	return context.ManagedBufferSetByteSliceWithHost(host, mBufferHandle, startingPosition, dataLength, dataOffset)
}

// ManagedBufferSetByteSliceWithHost VMHooks implementation.
func (context *VMHooksImpl) ManagedBufferSetByteSliceWithHost(
	host vmhost.VMHost,
	mBufferHandle int32,
	startingPosition int32,
	dataLength executor.MemLength,
	dataOffset executor.MemPtr) int32 {

	managedType := host.ManagedTypes()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(mBufferGetByteSliceName)

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferSetBytes
	// post-fork the write is capped and the flat and per-byte gas for the loaded bytes are
	// bounded and paid before the memory load; the destination clone is paid for where it
	// happens, in ManagedBufferSetByteSliceWithTypedArgs
	if host.ForkController().FixAuditChangesV5() {
		err := metering.UseGasBounded(gasToUse)
		if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		err = consumeGasForCappedLength(managedType, dataLength)
		if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	data, err := context.MemLoad(dataOffset, dataLength)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	return ManagedBufferSetByteSliceWithTypedArgs(host, mBufferHandle, startingPosition, dataLength, data)
}

// ManagedBufferSetByteSliceWithTypedArgs VMHooks implementation.
// It pays for its own work: post-fork the destination buffer that SetByteSlice clones is charged
// with bounded gas before the clone, so a caller only has to pay for producing data itself.
func ManagedBufferSetByteSliceWithTypedArgs(host vmhost.VMHost, mBufferHandle int32, startingPosition int32, dataLength int32, data []byte) int32 {
	managedType := host.ManagedTypes()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(mBufferGetByteSliceName)

	if host.ForkController().FixAuditChangesV5() {
		err := consumeGasForBuffersBounded(host, mBufferHandle)
		if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		managedType.ConsumeGasForBytes(data)
	}

	ok, err := managedType.SetByteSlice(mBufferHandle, startingPosition, dataLength, data)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !ok {
		// does not fail execution if slice exceeds bounds
		return 1
	}

	return 0
}

// MBufferAppend VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferAppend(accumulatorHandle int32, dataHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferAppendName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferAppend
	// post-fork the resulting length is capped and the flat and per-byte gas are bounded and paid before the clone
	if fixAuditV5 {
		// the resulting length is capped before any gas is charged, so the ceiling holds
		// whatever the gas schedule prices a byte at
		if int64(managedType.GetLength(accumulatorHandle))+int64(managedType.GetLength(dataHandle)) > maxManagedBufferLength {
			_ = context.WithFault(vmhost.ErrManagedBufferLengthExceedsMaximum, runtime.ManagedBufferAPIErrorShouldFailExecution())
			return 1
		}

		err := metering.UseGasBounded(gasToUse)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		// the append always copies the incoming bytes and copies the accumulator only when it
		// has to reallocate, which is exactly what ConsumeGasForAppend charges. Charging the
		// accumulator unconditionally would bill the sum of every intermediate length, making a
		// buffer built in a loop cost gas quadratic in the number of appends.
		err = managedType.ConsumeGasForAppend(accumulatorHandle, managedType.GetLength(dataHandle))
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	dataBufferBytes, err := managedType.GetBytes(dataHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(dataBufferBytes)
	}

	isSuccess := managedType.AppendBytes(accumulatorHandle, dataBufferBytes)
	if !isSuccess {
		_ = context.WithFault(vmhost.ErrNoManagedBufferUnderThisHandle, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return 1
	}

	return 0
}

// MBufferAppendBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferAppendBytes(accumulatorHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferAppendBytesName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferAppendBytes
	// post-fork the resulting length is capped and the flat and per-byte gas are bounded and paid before the memory load
	if fixAuditV5 {
		err := metering.UseGasBounded(gasToUse)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		if int64(managedType.GetLength(accumulatorHandle))+int64(dataLength) > maxManagedBufferLength {
			_ = context.WithFault(vmhost.ErrManagedBufferLengthExceedsMaximum, runtime.ManagedBufferAPIErrorShouldFailExecution())
			return 1
		}

		err = validateCappedLength(dataLength)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}

		// the append always copies the incoming bytes and copies the accumulator only when it
		// has to reallocate, which is exactly what ConsumeGasForAppend charges. Charging the
		// accumulator unconditionally would bill the sum of every intermediate length, making a
		// buffer built in a loop cost gas quadratic in the number of appends.
		err = managedType.ConsumeGasForAppend(accumulatorHandle, dataLength)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	data, err := context.MemLoad(dataOffset, dataLength)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	isSuccess := managedType.AppendBytes(accumulatorHandle, data)
	if !isSuccess {
		_ = context.WithFault(vmhost.ErrNoManagedBufferUnderThisHandle, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return 1
	}

	if !fixAuditV5 {
		gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, uint64(len(data)))
		metering.UseAndTraceGas(gasToUse)
	}

	return 0
}

// MBufferToBigIntUnsigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferToBigIntUnsigned(mBufferHandle int32, bigIntHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferToBigIntUnsigned
	// post-fork the flat gas is bounded and the buffer is paid for before it is cloned and parsed
	err := useBoundedGas(context.host, mBufferToBigIntUnsignedName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	err = consumeGasForBuffersBounded(context.host, mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	managedBuffer, err := managedType.GetBytes(mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	bigInt := managedType.GetBigIntOrCreate(bigIntHandle)
	bigInt.SetBytes(managedBuffer)

	return 0
}

// MBufferToBigIntSigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferToBigIntSigned(mBufferHandle int32, bigIntHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferToBigIntSigned
	// post-fork the flat gas is bounded and the buffer is paid for before it is cloned and parsed
	err := useBoundedGas(context.host, mBufferToBigIntSignedName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	err = consumeGasForBuffersBounded(context.host, mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	managedBuffer, err := managedType.GetBytes(mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	bigInt := managedType.GetBigIntOrCreate(bigIntHandle)
	twos.SetBytes(bigInt, managedBuffer)

	return 0
}

// MBufferFromBigIntUnsigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferFromBigIntUnsigned(mBufferHandle int32, bigIntHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferFromBigIntUnsigned
	// post-fork the flat gas is bounded and the value is paid for before it is serialized and copied
	err := useBoundedGas(context.host, mBufferFromBigIntUnsignedName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return 1
	}

	value, err := managedType.GetBigInt(bigIntHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return 1
	}

	if fixAuditV5 {
		// #nosec G115
		err = managedType.ConsumeGasForByteLenBounded(uint64(value.BitLen())/8 + 1)
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	managedType.SetBytes(mBufferHandle, value.Bytes())

	return 0
}

// MBufferFromBigIntSigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferFromBigIntSigned(mBufferHandle int32, bigIntHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferFromBigIntSigned
	// post-fork the flat gas is bounded and the value is paid for before it is serialized and copied
	err := useBoundedGas(context.host, mBufferFromBigIntSignedName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return 1
	}

	value, err := managedType.GetBigInt(bigIntHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return 1
	}

	if fixAuditV5 {
		// #nosec G115
		err = managedType.ConsumeGasForByteLenBounded(uint64(value.BitLen())/8 + 1)
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	managedType.SetBytes(mBufferHandle, twos.ToBytes(value))
	return 0
}

// MBufferToBigFloat VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferToBigFloat(mBufferHandle, bigFloatHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferToBigFloatName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferToBigFloat
	err := consumeBufferReadGas(context.host, gasToUse, mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	managedBuffer, err := managedType.GetBytes(mBufferHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	if !fixAuditV5 {
		managedType.ConsumeGasForBytes(managedBuffer)
	}
	if managedType.EncodedBigFloatIsNotValid(managedBuffer) {
		_ = context.WithFault(vmhost.ErrBigFloatWrongPrecision, runtime.BigFloatAPIErrorShouldFailExecution())
		return 1
	}

	value, err := managedType.GetBigFloatOrCreate(bigFloatHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	bigFloat := new(big.Float)
	err = bigFloat.GobDecode(managedBuffer)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}
	if bigFloat.IsInf() {
		_ = context.WithFault(vmhost.ErrInfinityFloatOperation, runtime.BigFloatAPIErrorShouldFailExecution())
		return 1
	}
	fixAuditV4 := context.host.ForkController().FixAuditChangesV4()
	if fixAuditV4 && managedType.BigFloatExpIsNotValid(bigFloat.MantExp(nil)) {
		_ = context.WithFault(vmhost.ErrExponentTooBigOrTooSmall, runtime.BigFloatAPIErrorShouldFailExecution())
		return 1
	}
	if fixAuditV4 && managedType.BigFloatIsNotCanonical(bigFloat) {
		_ = context.WithFault(vmhost.ErrBigFloatNonCanonicalEncoding, runtime.BigFloatAPIErrorShouldFailExecution())
		return 1
	}
	value.Set(bigFloat)
	return 0
}

// MBufferFromBigFloat VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferFromBigFloat(mBufferHandle, bigFloatHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(mBufferFromBigFloatName)

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferFromBigFloat
	// post-fork the flat and the per-byte gas are bounded and the encoding is paid for before it is copied
	if fixAuditV5 {
		err := metering.UseGasBounded(gasToUse)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	value, err := managedType.GetBigFloat(bigFloatHandle)
	if context.WithFault(err, runtime.BigFloatAPIErrorShouldFailExecution()) {
		return 1
	}

	encodedFloat, err := value.GobEncode()
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	if fixAuditV5 {
		err = managedType.ConsumeGasForByteLenBounded(uint64(len(encodedFloat)))
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	} else {
		managedType.ConsumeGasForBytes(encodedFloat)
	}

	managedType.SetBytes(mBufferHandle, encodedFloat)

	return 0
}

// MBufferStorageStore VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferStorageStore(keyHandle int32, sourceHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	storage := context.GetStorageContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferStorageStore
	// post-fork the flat gas is bounded and both buffers are paid for before they are cloned
	err := useBoundedGas(context.host, mBufferStorageStoreName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	err = consumeGasForBuffersBounded(context.host, keyHandle, sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	key, err := managedType.GetBytes(keyHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sourceBytes, err := managedType.GetBytes(sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	_, err = storage.SetStorage(key, sourceBytes)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// MBufferStorageLoad VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferStorageLoad(keyHandle int32, destinationHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	storage := context.GetStorageContext()
	metering := context.GetMeteringContext()

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	// post-fork the key is paid for with bounded gas before it is cloned
	err := consumeGasForBuffersBounded(context.host, keyHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	key, err := managedType.GetBytes(keyHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	storageBytes, trieDepth, usedCache, err := storage.GetStorage(key)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 0
	}
	err = storage.UseGasForStorageLoad(
		mBufferStorageLoadName,
		int64(trieDepth),
		metering.GasSchedule().ManagedBufferAPICost.MBufferStorageLoad,
		usedCache)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	// post-fork the loaded value is paid for with bounded gas before it is copied into the destination
	if fixAuditV5 {
		err = managedType.ConsumeGasForByteLenBounded(uint64(len(storageBytes)))
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	managedType.SetBytes(destinationHandle, storageBytes)

	return 0
}

// MBufferStorageLoadFromAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferStorageLoadFromAddress(addressHandle, keyHandle, destinationHandle int32) {
	host := context.GetVMHost()
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()

	fixAuditV5 := host.ForkController().FixAuditChangesV5()
	// post-fork the key and the address are paid for with bounded gas before they are cloned
	err := consumeGasForBuffersBounded(host, keyHandle, addressHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}

	key, err := managedType.GetBytes(keyHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}

	address, err := managedType.GetBytes(addressHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	storageBytes, err := StorageLoadFromAddressWithTypedArgs(host, address, key)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}

	// post-fork the loaded value is paid for with bounded gas before it is copied into the destination
	if fixAuditV5 {
		err = managedType.ConsumeGasForByteLenBounded(uint64(len(storageBytes)))
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return
		}
	}

	managedType.SetBytes(destinationHandle, storageBytes)
}

// MBufferGetArgument VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferGetArgument(id int32, destinationHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	fixAuditV5 := context.host.ForkController().FixAuditChangesV5()
	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferGetArgument
	// post-fork the flat gas is bounded and the argument is paid for before it is copied
	err := useBoundedGas(context.host, mBufferGetArgumentName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	args := runtime.Arguments()
	// #nosec G115
	if int32(len(args)) <= id || id < 0 {
		context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return 1
	}

	if fixAuditV5 {
		err = managedType.ConsumeGasForByteLenBounded(uint64(len(args[id])))
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	managedType.SetBytes(destinationHandle, args[id])
	return 0
}

// MBufferFinish VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferFinish(sourceHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()
	metering.StartGasTracing(mBufferFinishName)

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferFinish
	// post-fork the flat and the per-byte gas are bounded and paid before the buffer clone
	err := consumeBufferReadGas(context.host, gasToUse, sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sourceBytes, err := managedType.GetBytes(sourceHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.PersistPerByte, uint64(len(sourceBytes)))
	err = metering.UseGasBounded(gasToUse)
	if err != nil {
		_ = context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return 1
	}

	output.Finish(sourceBytes)
	return 0
}

// MBufferSetRandom VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MBufferSetRandom(destinationHandle int32, length int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	if length < 1 {
		_ = context.WithFault(vmhost.ErrLengthOfBufferNotCorrect, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return -1
	}

	baseGasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferSetRandom
	lengthDependentGasToUse := math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, uint64(length))
	gasToUse := math.AddUint64(baseGasToUse, lengthDependentGasToUse)
	// post-fork the length is capped and the length-dependent gas is bounded and paid before the allocation
	if context.host.ForkController().FixAuditChangesV5() && length > maxManagedBufferLength {
		_ = context.WithFault(vmhost.ErrManagedBufferLengthExceedsMaximum, runtime.ManagedBufferAPIErrorShouldFailExecution())
		return -1
	}

	err := useBoundedGas(context.host, mBufferSetRandomName, gasToUse)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}

	randomizer := managedType.GetRandReader()
	buffer := make([]byte, length)
	_, err = randomizer.Read(buffer)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return -1
	}

	managedType.SetBytes(destinationHandle, buffer)
	return 0
}
