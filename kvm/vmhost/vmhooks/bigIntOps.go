package vmhooks

import (
	"math/big"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/math"
	twos "github.com/klever-io/klever-go/kvm/math/twos-complement"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

const (
	bigIntNewName                    = "bigIntNew"
	bigIntUnsignedByteLengthName     = "bigIntUnsignedByteLength"
	bigIntSignedByteLengthName       = "bigIntSignedByteLength"
	bigIntGetUnsignedBytesName       = "bigIntGetUnsignedBytes"
	bigIntGetSignedBytesName         = "bigIntGetSignedBytes"
	bigIntSetUnsignedBytesName       = "bigIntSetUnsignedBytes"
	bigIntSetSignedBytesName         = "bigIntSetSignedBytes"
	bigIntIsInt64Name                = "bigIntIsInt64"
	bigIntGetInt64Name               = "bigIntGetInt64"
	bigIntSetInt64Name               = "bigIntSetInt64"
	bigIntAddName                    = "bigIntAdd"
	bigIntSubName                    = "bigIntSub"
	bigIntMulName                    = "bigIntMul"
	bigIntTDivName                   = "bigIntTDiv"
	bigIntTModName                   = "bigIntTMod"
	bigIntEDivName                   = "bigIntEDiv"
	bigIntEModName                   = "bigIntEMod"
	bigIntPowName                    = "bigIntPow"
	bigIntLog2Name                   = "bigIntLog2"
	bigIntSqrtName                   = "bigIntSqrt"
	bigIntAbsName                    = "bigIntAbs"
	bigIntNegName                    = "bigIntNeg"
	bigIntSignName                   = "bigIntSign"
	bigIntCmpName                    = "bigIntCmp"
	bigIntNotName                    = "bigIntNot"
	bigIntAndName                    = "bigIntAnd"
	bigIntOrName                     = "bigIntOr"
	bigIntXorName                    = "bigIntXor"
	bigIntShrName                    = "bigIntShr"
	bigIntShlName                    = "bigIntShl"
	bigIntFinishUnsignedName         = "bigIntFinishUnsigned"
	bigIntFinishSignedName           = "bigIntFinishSigned"
	bigIntStorageStoreUnsignedName   = "bigIntStorageStoreUnsigned"
	bigIntStorageLoadUnsignedName    = "bigIntStorageLoadUnsigned"
	bigIntGetUnsignedArgumentName    = "bigIntGetUnsignedArgument"
	bigIntGetSignedArgumentName      = "bigIntGetSignedArgument"
	bigIntGetCallValueName           = "bigIntGetCallValue"
	bigIntGetKDACallValueName        = "bigIntGetKDACallValue"
	bigIntGetKDACallValueByIndexName = "bigIntGetKDACallValueByIndex"
	bigIntGetKDAExternalBalanceName  = "bigIntGetKDAExternalBalance"
	bigIntGetExternalBalanceName     = "bigIntGetExternalBalance"
	bigIntToStringName               = "bigIntToString"
)

// BigIntGetUnsignedArgument VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetUnsignedArgument(id int32, destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetUnsignedArgument
	arg, ok := context.chargeBigIntArgument(bigIntGetUnsignedArgumentName, gasToUse, id)
	if !ok {
		return
	}

	value := managedType.GetBigIntOrCreate(destinationHandle)

	value.SetBytes(arg)
}

// chargeBigIntArgument charges a BigInt argument getter's flat gas and returns argument id for it to
// copy, or false when there is nothing to copy. From FixAuditChangesV6 the flat gas is bounded and an
// oversized argument is paid for before it is copied, so the destination is not created or changed
// for an argument left unpaid. An index out of range is not a fault, as it never was
func (context *VMHooksImpl) chargeBigIntArgument(hookName string, gasToUse uint64, id int32) ([]byte, bool) {
	runtime := context.GetRuntimeContext()
	err := startTracingAndUseGas(context.host, hookName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return nil, false
	}

	args := runtime.Arguments()
	// #nosec G115
	if int32(len(args)) <= id || id < 0 {
		return nil, false
	}

	if context.host.ForkController().FixAuditChangesV6() {
		err = context.GetManagedTypesContext().ConsumeGasForThisIntNumberOfBytesBounded(int64(len(args[id])))
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return nil, false
		}
	}

	return args[id], true
}

// BigIntGetSignedArgument VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetSignedArgument(id int32, destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetSignedArgument
	arg, ok := context.chargeBigIntArgument(bigIntGetSignedArgumentName, gasToUse, id)
	if !ok {
		return
	}

	value := managedType.GetBigIntOrCreate(destinationHandle)

	twos.SetBytes(value, arg)
}

// BigIntStorageStoreUnsigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntStorageStoreUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength, sourceHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	storage := context.GetStorageContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntStorageStoreUnsigned
	key, err := context.chargeAndLoadKey(context.GetVMHost(), bigIntStorageStoreUnsignedName, gasToUse, keyOffset, keyLength)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	value := managedType.GetBigIntOrCreate(sourceHandle)
	bytes := value.Bytes()

	storageStatus, err := storage.SetStorage(key, bytes)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	return int32(storageStatus) // #nosec G115
}

// BigIntStorageLoadUnsigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntStorageLoadUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength, destinationHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	storage := context.GetStorageContext()
	metering := context.GetMeteringContext()

	key, _, err := context.memLoadCharged(context.GetVMHost(), keyOffset, keyLength)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	bytes, trieDepth, usedCache, err := storage.GetStorage(key)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	err = storage.UseGasForStorageLoad(bigIntStorageLoadUnsignedName,
		int64(trieDepth),
		metering.GasSchedule().BigIntAPICost.BigIntStorageLoadUnsigned,
		usedCache)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = useGasForCachedStorageValue(context.GetVMHost(), bytes, usedCache)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	value := managedType.GetBigIntOrCreate(destinationHandle)
	value.SetBytes(bytes)

	return int32(len(bytes)) // #nosec G115
}

// BigIntGetCallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetCallValue(destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := context.ComputeGetCallValueGas()
	metering.UseGasAndAddTracedGas(bigIntGetCallValueName, gasToUse)

	value := managedType.GetBigIntOrCreate(destinationHandle)
	// get KLV from KDA Transfers if any
	value.Set(runtime.GetVMInput().GetKDACallValue(nil))
}

// BigIntGetKDACallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetKDACallValue(destination int32) {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return
	}
	context.BigIntGetKDACallValueByIndex(destination, 0)
}

// BigIntGetKDACallValueByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetKDACallValueByIndex(destinationHandle int32, index int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetCallValue
	metering.UseGasAndAddTracedGas(bigIntGetKDACallValueByIndexName, gasToUse)

	value := managedType.GetBigIntOrCreate(destinationHandle)
	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)
	if kdaTransfer != nil {
		value.Set(kdaTransfer.KDAValue)
	} else {
		value.Set(big.NewInt(0))
	}
}

// BigIntGetExternalBalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetExternalBalance(addressOffset executor.MemPtr, result int32) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetExternalBalance
	metering.UseGasAndAddTracedGas(bigIntGetExternalBalanceName, gasToUse)

	address, err := context.MemLoad(addressOffset, vmhost.AddressLen)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	err = blockchain.CountContractAccountRead()
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	balance := blockchain.GetBalance(address)
	value := managedType.GetBigIntOrCreate(result)

	value.SetBytes(balance)
}

// BigIntGetKDAExternalBalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetKDAExternalBalance(
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
	resultHandle int32) {

	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	// opens this hook's trace, which the shared KDA read below then charges into
	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetExternalBalance
	err := startTracingAndUseGas(context.GetVMHost(), bigIntGetKDAExternalBalanceName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	kdaData, userKDA, err := getKDADataFromBlockchainHook(context, addressOffset, tokenIDOffset, tokenIDLen, nonce)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if kdaData == nil {
		return
	}

	value := managedType.GetBigIntOrCreate(resultHandle)
	value.Set(big.NewInt(userKDA.Balance))
}

// BigIntNew VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntNew(smallValue int64) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntNew
	metering.UseGasAndAddTracedGas(bigIntNewName, gasToUse)

	return managedType.NewBigIntFromInt64(smallValue)
}

// BigIntUnsignedByteLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntUnsignedByteLength(referenceHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntUnsignedByteLength
	// from FixAuditChangesV6 the flat gas is bounded and the length is computed rather than measured
	// on an encoding of the value, which is as long as the caller made it
	err := startTracingAndUseGas(context.host, bigIntUnsignedByteLengthName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	value, err := managedType.GetBigInt(referenceHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	if context.host.ForkController().FixAuditChangesV6() {
		return int32(unsignedByteLength(value)) // #nosec G115
	}

	bytes := value.Bytes()
	return int32(len(bytes)) // #nosec G115
}

// BigIntSignedByteLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSignedByteLength(referenceHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSignedByteLength
	// from FixAuditChangesV6 the flat gas is bounded and the length is computed rather than measured
	// on an encoding of the value, which is as long as the caller made it
	err := startTracingAndUseGas(context.host, bigIntSignedByteLengthName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	value, err := managedType.GetBigInt(referenceHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	if context.host.ForkController().FixAuditChangesV6() {
		return int32(twosComplementByteLength(value)) // #nosec G115
	}

	bytes := twos.ToBytes(value)
	return int32(len(bytes)) // #nosec G115
}

// unsignedByteLength is len(value.Bytes()), without the bytes
func unsignedByteLength(value *big.Int) int {
	return (value.BitLen() + 7) / 8
}

// twosComplementByteLength is len(twos.ToBytes(value)), without the bytes. Zero encodes to none.
// A positive value takes its magnitude's bytes plus a 0x00 when the top byte has no room for the
// sign bit: bitLen/8 + 1 either way. A negative value encodes as the inverted bytes of |value|-1,
// with a 0xFF in front under the same condition, so it is the same formula on the bit length of
// |value|-1, which is that of |value| less one exactly when |value| is a power of two
func twosComplementByteLength(value *big.Int) int {
	bitLen := value.BitLen()
	switch value.Sign() {
	case 0:
		return 0
	case -1:
		if value.TrailingZeroBits() == uint(bitLen-1) { // #nosec G115 -- bitLen >= 1 for a non-zero value
			bitLen--
		}
	}
	return bitLen/8 + 1
}

// decimalStringMaxLength is an upper bound on len(value.String()), without the string. A value of
// bitLen bits has at most ceil(bitLen*log10(2)) digits, and 30103/100000 is above log10(2); one more
// for the sign. A zero, whose bit length is 0, gets 1, which "0" fits in. Integer arithmetic only, so
// every node computes the same bound
func decimalStringMaxLength(value *big.Int) uint64 {
	bitLen := uint64(value.BitLen()) // #nosec G115 -- a bit length is never negative
	return (bitLen*30103+99999)/100000 + 1
}

// BigIntGetUnsignedBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetUnsignedBytes(referenceHandle int32, byteOffset executor.MemPtr) int32 {
	gasToUse := context.GetMeteringContext().GasSchedule().BigIntAPICost.BigIntGetUnsignedBytes
	return context.storeBigIntBytes(bigIntGetUnsignedBytesName, gasToUse, referenceHandle, byteOffset, unsignedByteLength, (*big.Int).Bytes)
}

// BigIntGetSignedBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetSignedBytes(referenceHandle int32, byteOffset executor.MemPtr) int32 {
	gasToUse := context.GetMeteringContext().GasSchedule().BigIntAPICost.BigIntGetSignedBytes
	return context.storeBigIntBytes(bigIntGetSignedBytesName, gasToUse, referenceHandle, byteOffset, twosComplementByteLength, twos.ToBytes)
}

// storeBigIntBytes is BigIntGetUnsignedBytes and BigIntGetSignedBytes: it charges the flat gas, encodes
// the big int at referenceHandle and stores it at byteOffset, returning the length stored or -1. From
// FixAuditChangesV6 the bytes are paid for, bounded, before the value is serialized: the same amount,
// from encodedLen, the length computed without the bytes. Before it they are charged after the store,
// unbounded, as they always were
func (context *VMHooksImpl) storeBigIntBytes(
	hookName string,
	gasToUse uint64,
	referenceHandle int32,
	byteOffset executor.MemPtr,
	encodedLen func(*big.Int) int,
	encode func(*big.Int) []byte,
) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.host, hookName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	value, err := managedType.GetBigInt(referenceHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	fixAuditChangesV6 := context.host.ForkController().FixAuditChangesV6()
	if fixAuditChangesV6 {
		err = managedType.ConsumeGasForByteLenBounded(uint64(encodedLen(value))) // #nosec G115 - a byte length is never negative
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return -1
		}
	}

	bytes := encode(value)
	err = context.MemStore(byteOffset, bytes)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	if !fixAuditChangesV6 {
		gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, uint64(len(bytes)))
		metering.UseAndTraceGas(gasToUse)
	}

	return int32(len(bytes)) // #nosec G115
}

// BigIntSetUnsignedBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSetUnsignedBytes(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSetUnsignedBytes
	bytes, err := context.chargeAndLoadBytes(context.GetVMHost(), bigIntSetUnsignedBytesName, gasToUse, byteOffset, byteLength)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	value := managedType.GetBigIntOrCreate(destinationHandle)
	value.SetBytes(bytes)
}

// BigIntSetSignedBytes VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSetSignedBytes(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSetSignedBytes
	bytes, err := context.chargeAndLoadBytes(context.GetVMHost(), bigIntSetSignedBytesName, gasToUse, byteOffset, byteLength)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	value := managedType.GetBigIntOrCreate(destinationHandle)
	twos.SetBytes(value, bytes)
}

// BigIntIsInt64 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntIsInt64(destinationHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntIsInt64
	metering.UseGasAndAddTracedGas(bigIntIsInt64Name, gasToUse)

	value, err := managedType.GetBigInt(destinationHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}
	if value.IsInt64() {
		return 1
	}
	return 0
}

// BigIntGetInt64 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntGetInt64(destinationHandle int32) int64 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntGetInt64
	metering.UseGasAndAddTracedGas(bigIntGetInt64Name, gasToUse)

	value := managedType.GetBigIntOrCreate(destinationHandle)
	if !value.IsInt64() {
		if context.WithFault(vmhost.ErrBigIntCannotBeRepresentedAsInt64, runtime.BigIntAPIErrorShouldFailExecution()) {
			return -1
		}
	}
	return value.Int64()
}

// BigIntSetInt64 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSetInt64(destinationHandle int32, value int64) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSetInt64
	metering.UseGasAndAddTracedGas(bigIntSetInt64Name, gasToUse)

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	dest.SetInt64(value)
}

// consumeGasForBigIntCopy charges the copy of values as ConsumeGasForBigIntCopy does: for each value
// in turn, CopyPerByteForTooBig per byte when it is longer than 32 bytes. Before FixAuditChangesV6 the
// charges are unbounded, as they always were, and can overdraw. From it each is bounded: the first the
// gas left cannot cover faults the hook with ErrNotEnoughGas, through the BigInt API predicate, and it
// returns false so the hook stops before the work the charge pays for. The amounts and their order
// are the same on both sides of the fork
func (context *VMHooksImpl) consumeGasForBigIntCopy(values ...*big.Int) bool {
	managedType := context.GetManagedTypesContext()
	if !context.host.ForkController().FixAuditChangesV6() {
		managedType.ConsumeGasForBigIntCopy(values...)
		return true
	}

	runtime := context.GetRuntimeContext()
	for _, value := range values {
		byteLen := (value.BitLen() + 7) / 8
		err := managedType.ConsumeGasForThisIntNumberOfBytesBounded(int64(byteLen))
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return false
		}
	}
	return true
}

// BigIntAdd VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntAdd(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntAdd
	err := startTracingAndUseGas(context.host, bigIntAddName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	dest.Add(a, b)
}

// BigIntSub VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSub(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSub
	err := startTracingAndUseGas(context.host, bigIntSubName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	dest.Sub(a, b)
}

// BigIntMul VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntMul(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntMul
	err := startTracingAndUseGas(context.host, bigIntMulName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}

	dest.Mul(a, b)
}

// BigIntTDiv VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntTDiv(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntTDiv
	err := startTracingAndUseGas(context.host, bigIntTDivName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	if b.Sign() == 0 {
		_ = context.WithFault(vmhost.ErrDivZero, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Quo(a, b) // Quo implements truncated division (like Go)
}

// BigIntTMod VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntTMod(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntTMod
	err := startTracingAndUseGas(context.host, bigIntTModName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	if b.Sign() == 0 {
		_ = context.WithFault(vmhost.ErrDivZero, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Rem(a, b) // Rem implements truncated modulus (like Go)
}

// BigIntEDiv VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntEDiv(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntEDiv
	err := startTracingAndUseGas(context.host, bigIntEDivName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	if b.Sign() == 0 {
		_ = context.WithFault(vmhost.ErrDivZero, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Div(a, b) // Div implements Euclidean division (unlike Go)
}

// BigIntEMod VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntEMod(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntEMod
	err := startTracingAndUseGas(context.host, bigIntEModName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a, b) {
		return
	}
	if b.Sign() == 0 {
		_ = context.WithFault(vmhost.ErrDivZero, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Mod(a, b) // Mod implements Euclidean division (unlike Go)
}

// BigIntSqrt VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSqrt(destinationHandle, opHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSqrt
	err := startTracingAndUseGas(context.host, bigIntSqrtName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a) {
		return
	}
	if a.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBadLowerBounds, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Sqrt(a)
}

// BigIntPow VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntPow(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntPow
	err := startTracingAndUseGas(context.host, bigIntPowName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	// The fault error becomes the on-chain returnMessage (receipts), so the
	// order of the negative-exponent check relative to the result-size charge
	// is replay-visible: pre-fork a large negative exponent faults inside
	// ConsumeGasForThisBigIntNumberOfBytes with ErrNotEnoughGas, post-fork it
	// always faults with ErrBadLowerBounds before any gas math.
	fixAuditV3 := context.host.ForkController().FixAuditChangesV3()
	if fixAuditV3 && b.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBadLowerBounds, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}

	opBitLength := int64(a.BitLen())
	//this calculates the length of the result in bytes
	lengthOfResult := big.NewInt(0).Div(big.NewInt(0).Mul(b, big.NewInt(opBitLength)), big.NewInt(8))

	err = managedType.ConsumeGasForThisBigIntNumberOfBytes(lengthOfResult)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	if !context.consumeGasForBigIntCopy(a, b) {
		return
	}

	if !fixAuditV3 && b.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBadLowerBounds, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}

	// A zero base resolves to a trivial result (0^0 == 1, 0^n == 0 for n > 0)
	// without invoking big.Int.Exp. The gas above is charged identically to the
	// general path (for a zero base the result-size charge is 0 and the copy
	// charge still covers the exponent), so this stays byte-identical to the
	// pre-existing behavior across historical replay and needs no fork gate.
	if a.Sign() == 0 {
		if b.Sign() == 0 {
			dest.SetInt64(1)
		} else {
			dest.SetInt64(0)
		}
		return
	}

	dest.Exp(a, b, nil)
}

// BigIntLog2 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntLog2(op1Handle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntLog
	err := startTracingAndUseGas(context.host, bigIntLog2Name, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}

	a, err := managedType.GetBigInt(op1Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -1
	}
	if !context.consumeGasForBigIntCopy(a) {
		return -1
	}
	if a.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBadLowerBounds, runtime.BigIntAPIErrorShouldFailExecution())
		return -1
	}

	return int32(a.BitLen() - 1) // #nosec G115
}

// BigIntAbs VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntAbs(destinationHandle, opHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntAbs
	err := startTracingAndUseGas(context.host, bigIntAbsName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a) {
		return
	}
	dest.Abs(a)
}

// BigIntNeg VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntNeg(destinationHandle, opHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntNeg
	err := startTracingAndUseGas(context.host, bigIntNegName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a) {
		return
	}
	dest.Neg(a)
}

// BigIntSign VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntSign(opHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntSign
	err := startTracingAndUseGas(context.host, bigIntSignName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -2
	}

	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -2
	}
	if !context.consumeGasForBigIntCopy(a) {
		return -2
	}
	return int32(a.Sign()) // #nosec G115
}

// BigIntCmp VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntCmp(op1Handle, op2Handle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntCmp
	err := startTracingAndUseGas(context.host, bigIntCmpName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -2
	}

	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return -2
	}
	if !context.consumeGasForBigIntCopy(a, b) {
		return -2
	}
	return int32(a.Cmp(b)) // #nosec G115
}

// BigIntNot VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntNot(destinationHandle, opHandle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntNot
	err := startTracingAndUseGas(context.host, bigIntNotName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(dest, a) {
		return
	}
	if a.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBitwiseNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Not(a)
}

// BigIntAnd VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntAnd(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntAnd
	err := startTracingAndUseGas(context.host, bigIntAndName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(a, b) {
		return
	}
	if a.Sign() < 0 || b.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBitwiseNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.And(a, b)
}

// BigIntOr VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntOr(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntOr
	err := startTracingAndUseGas(context.host, bigIntOrName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(a, b) {
		return
	}
	if a.Sign() < 0 || b.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBitwiseNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Or(a, b)
}

// BigIntXor VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntXor(destinationHandle, op1Handle, op2Handle int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntXor
	err := startTracingAndUseGas(context.host, bigIntXorName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, b, err := managedType.GetTwoBigInt(op1Handle, op2Handle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(a, b) {
		return
	}
	if a.Sign() < 0 || b.Sign() < 0 {
		_ = context.WithFault(vmhost.ErrBitwiseNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	dest.Xor(a, b)
}

// BigIntShr VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntShr(destinationHandle, opHandle, bits int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntShr
	err := startTracingAndUseGas(context.host, bigIntShrName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(a) {
		return
	}
	if a.Sign() < 0 || bits < 0 {
		_ = context.WithFault(vmhost.ErrShiftNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	// from FixAuditChangesV6 the result is paid for, bounded, before it is written: the same amount,
	// from its length computed without shifting (a is not negative here, so the shift drops bits
	// bits off its length)
	if context.host.ForkController().FixAuditChangesV6() {
		resultByteLen := int64(0)
		if resultBitLen := int64(a.BitLen()) - int64(bits); resultBitLen > 0 {
			resultByteLen = (resultBitLen + 7) / 8
		}
		err = managedType.ConsumeGasForThisIntNumberOfBytesBounded(resultByteLen)
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return
		}
		dest.Rsh(a, uint(bits))
		return
	}
	dest.Rsh(a, uint(bits))
	managedType.ConsumeGasForBigIntCopy(dest)
}

// BigIntShl VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntShl(destinationHandle, opHandle, bits int32) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntShl
	err := startTracingAndUseGas(context.host, bigIntShlName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dest := managedType.GetBigIntOrCreate(destinationHandle)
	a, err := managedType.GetBigInt(opHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}
	if !context.consumeGasForBigIntCopy(a) {
		return
	}
	if a.Sign() < 0 || bits < 0 {
		_ = context.WithFault(vmhost.ErrShiftNegative, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	if context.host.ForkController().FixAuditChangesV6() {
		resultByteLen := int64(0)
		if a.Sign() != 0 {
			resultByteLen = (int64(a.BitLen()) + int64(bits) + 7) / 8
		}
		err = managedType.ConsumeGasForThisIntNumberOfBytesBounded(resultByteLen)
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return
		}
		dest.Lsh(a, uint(bits))
		return
	}
	dest.Lsh(a, uint(bits))
	managedType.ConsumeGasForBigIntCopy(dest)

}

// BigIntFinishUnsigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntFinishUnsigned(referenceHandle int32) {
	gasToUse := context.GetMeteringContext().GasSchedule().BigIntAPICost.BigIntFinishUnsigned
	context.finishBigInt(bigIntFinishUnsignedName, gasToUse, referenceHandle, unsignedByteLength, (*big.Int).Bytes)
}

// BigIntFinishSigned VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntFinishSigned(referenceHandle int32) {
	gasToUse := context.GetMeteringContext().GasSchedule().BigIntAPICost.BigIntFinishSigned
	context.finishBigInt(bigIntFinishSignedName, gasToUse, referenceHandle, twosComplementByteLength, twos.ToBytes)
}

// finishBigInt is BigIntFinishUnsigned and BigIntFinishSigned: it charges the flat gas and finishes
// the big int at referenceHandle, encoded. From FixAuditChangesV6 the bytes are paid for, bounded,
// before the value is serialized: the same amount, from encodedLen, the length computed without the
// bytes. Before it they are charged after the finish, unbounded, as they always were
func (context *VMHooksImpl) finishBigInt(
	hookName string,
	gasToUse uint64,
	referenceHandle int32,
	encodedLen func(*big.Int) int,
	encode func(*big.Int) []byte,
) {
	managedType := context.GetManagedTypesContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	err := startTracingAndUseGas(context.host, hookName, gasToUse)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	value, err := managedType.GetBigInt(referenceHandle)
	if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	persistPerByte := metering.GasSchedule().BaseOperationCost.PersistPerByte
	if context.host.ForkController().FixAuditChangesV6() {
		err = metering.UseGasBounded(math.MulUint64(persistPerByte, uint64(encodedLen(value)))) // #nosec G115 - a byte length is never negative
		if context.WithFault(err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return
		}
		output.Finish(encode(value))
		return
	}

	bytes := encode(value)
	output.Finish(bytes)

	metering.UseAndTraceGas(math.MulUint64(persistPerByte, uint64(len(bytes))))
}

// BigIntToString VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) BigIntToString(bigIntHandle int32, destinationHandle int32) {
	host := context.GetVMHost()
	BigIntToStringWithHost(host, bigIntHandle, destinationHandle)
}

// BigIntToStringWithHost writes the decimal string of the big int into the destination buffer. From
// FixAuditChangesV6 the operand is paid for, bounded, as every other big int hook pays for its
// operands (KLC-2728). The length of the string is only known once the value is converted, so the
// call is then refused, before converting, when the gas left cannot cover DataCopyPerByte for an
// upper bound of that length; the exact length is then charged, the amount charged before the fork,
// before the string is written
func BigIntToStringWithHost(host vmhost.VMHost, bigIntHandle int32, destinationHandle int32) {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	fixAuditV6 := host.ForkController().FixAuditChangesV6()

	gasToUse := metering.GasSchedule().BigIntAPICost.BigIntFinishSigned
	err := startTracingAndUseGas(host, bigIntToStringName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	value, err := managedType.GetBigInt(bigIntHandle)
	if WithFaultAndHost(host, err, runtime.BigIntAPIErrorShouldFailExecution()) {
		return
	}

	dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte
	if fixAuditV6 {
		// the operand term, CopyPerByteForTooBig a byte above 32 bytes, prices the base-10
		// conversion, superlinear in the operand, the way the family prices its operands; the
		// execution timeout remains the hard time bound, since MaxGasPerTX is set by governance
		err = managedType.ConsumeGasForThisIntNumberOfBytesBounded(int64(unsignedByteLength(value)))
		if WithFaultAndHost(host, err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return
		}
		if metering.GasLeft() < math.MulUint64(dataCopyPerByte, decimalStringMaxLength(value)) {
			_ = WithFaultAndHost(host, vmhost.ErrNotEnoughGas, runtime.BigIntAPIErrorShouldFailExecution())
			return
		}
		resultStr := value.String()
		// within the bound just checked, so it never refuses
		err = metering.UseGasBounded(math.MulUint64(dataCopyPerByte, uint64(len(resultStr))))
		if WithFaultAndHost(host, err, runtime.BigIntAPIErrorShouldFailExecution()) {
			return
		}
		managedType.SetBytes(destinationHandle, []byte(resultStr))
		return
	}

	resultStr := value.String()
	managedType.SetBytes(destinationHandle, []byte(resultStr))
	gasToUse = math.MulUint64(dataCopyPerByte, uint64(len(resultStr)))
	metering.UseAndTraceGas(gasToUse)
}
