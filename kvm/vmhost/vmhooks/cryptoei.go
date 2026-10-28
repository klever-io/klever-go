package vmhooks

import (
	"crypto/elliptic"
	"math/big"
	"slices"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/klever-io/klever-go/kvm/crypto/signing/secp256k1"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

const blsPublicKeyLength = 96
const blsSignatureLength = 48
const ed25519PublicKeyLength = 32
const ed25519SignatureLength = 64
const secp256k1CompressedPublicKeyLength = 33
const secp256k1UncompressedPublicKeyLength = 65
const secp256k1SignatureComponentMaxLength = 32
const secp256k1PlainMessageLength = 32
const curveNameLength = 4

const (
	sha256Name                      = "sha256"
	keccak256Name                   = "keccak256"
	ripemd160Name                   = "ripemd160"
	verifyBLSName                   = "verifyBLS"
	verifyEd25519Name               = "verifyEd25519"
	verifyCustomSecp256k1Name       = "verifyCustomSecp256k1"
	encodeSecp256k1DerSignatureName = "encodeSecp256k1DerSignature"
	addECName                       = "addEC"
	doubleECName                    = "doubleEC"
	isOnCurveECName                 = "isOnCurveEC"
	scalarBaseMultECName            = "scalarBaseMultEC"
	scalarMultECName                = "scalarMultEC"
	marshalECName                   = "marshalEC"
	unmarshalECName                 = "unmarshalEC"
	marshalCompressedECName         = "marshalCompressedEC"
	unmarshalCompressedECName       = "unmarshalCompressedEC"
	generateKeyECName               = "generateKeyEC"
	createECName                    = "createEC"
	getCurveLengthECName            = "getCurveLengthEC"
	getPrivKeyByteLengthECName      = "getPrivKeyByteLengthEC"
	ellipticCurveGetValuesName      = "ellipticCurveGetValues"
)

// startTracingAndUseGas opens the gas trace of the hook and charges the hook's fixed cost into it, so
// every later charge the hook makes lands in the same trace entry. Before FixAuditChangesV6 the charge
// is unbounded, as hooks always did, and the error is always nil; from it the charge is bounded, so an
// underfunded call fails here instead of overdrawing. Overdrawing would leave GasLeft() at 0, and every
// later bounded charge would then pass its own 0 < 0 check and the crypto work would run unpaid. What
// the hooks charge bounded from the fork is this fixed cost and their user-sized byte inputs; the big
// int result copies keep going through ConsumeGasForBigIntCopy, which can overdraw, on both sides of
// the fork
func startTracingAndUseGas(host vmhost.VMHost, hookName string, gasToUse uint64) error {
	metering := host.Metering()
	if !host.ForkController().FixAuditChangesV6() {
		metering.StartGasTracing(hookName)
		metering.UseAndTraceGas(gasToUse)
		return nil
	}

	return metering.UseGasBoundedAndAddTracedGas(hookName, gasToUse)
}

// checkMBufferLength rejects with errInvalid, from FixAuditChangesV6, a buffer whose length is none of
// validLengths, before anything is copied. Keys and signatures are rejected rather than sliced: a hook
// must not call a buffer valid when only a prefix of it is, or a contract keying on the whole buffer
// (a used-signature set, a per-key allowlist) could be fed the same value padded differently. A missing
// buffer passes, left for loadMBufferBytes to report
func checkMBufferLength(host vmhost.VMHost, mBufferHandle int32, errInvalid error, validLengths ...int32) error {
	if !host.ForkController().FixAuditChangesV6() {
		return nil
	}
	length := host.ManagedTypes().GetLength(mBufferHandle)
	if length < 0 || slices.Contains(validLengths, length) {
		return nil
	}
	return errInvalid
}

// checkMemLength rejects with errInvalid, from FixAuditChangesV6, a contract-memory range whose length
// is not validLength, before anything is copied. It is the raw-memory twin of checkMBufferLength: the
// managed hooks reject a wrong-length buffer for free, and the non-managed ones have to do the same
// rather than pay DataCopyPerByte for a range they are about to refuse
func checkMemLength(host vmhost.VMHost, length executor.MemLength, errInvalid error, validLength int32) error {
	if !host.ForkController().FixAuditChangesV6() || length == validLength {
		return nil
	}
	return errInvalid
}

// checkMBufferMaxLength rejects with errInvalid, from FixAuditChangesV6, a buffer longer than maxLength,
// before anything is copied. A missing buffer passes, left for loadMBufferBytes to report
func checkMBufferMaxLength(host vmhost.VMHost, mBufferHandle int32, errInvalid error, maxLength int32) error {
	if !host.ForkController().FixAuditChangesV6() {
		return nil
	}
	if host.ManagedTypes().GetLength(mBufferHandle) > maxLength {
		return errInvalid
	}
	return nil
}

// loadMBufferBytes copies a user-sized managed buffer out for a hook and charges a per-byte cost for it.
// Before FixAuditChangesV6 it charges preForkCost per byte after the copy, unbounded, as hooks always
// did; from it, postForkCost per byte bounded, before the copy. A zero cost charges nothing on that side
// of the fork. Callers that need a size check make it before this, with checkMBufferLength
func loadMBufferBytes(
	host vmhost.VMHost,
	mBufferHandle int32,
	preForkCost uint64,
	postForkCost uint64,
) ([]byte, error) {
	managedType := host.ManagedTypes()
	metering := host.Metering()

	if !host.ForkController().FixAuditChangesV6() {
		data, err := managedType.GetBytes(mBufferHandle)
		if err != nil {
			return nil, err
		}
		if preForkCost != 0 {
			metering.UseAndTraceGas(math.MulUint64(preForkCost, uint64(len(data))))
		}
		return data, nil
	}

	length := managedType.GetLength(mBufferHandle)
	if length < 0 {
		return nil, vmhost.ErrNoManagedBufferUnderThisHandle
	}
	if postForkCost != 0 {
		err := metering.UseGasBounded(math.MulUint64(postForkCost, uint64(length))) // #nosec G115 - checked non-negative above
		if err != nil {
			return nil, err
		}
	}

	return managedType.GetBytes(mBufferHandle)
}

// loadMemBytes loads a user-sized range of length bytes of contract memory for a hook and charges a
// per-byte cost for it. Before FixAuditChangesV6 it charges preForkCost per byte as hooks always did, unbounded,
// before the load; from it, postForkCost per byte bounded, before the load. A zero cost charges nothing
// on that side of the fork
func loadMemBytes(
	context *VMHooksImpl,
	dataOffset executor.MemPtr,
	length executor.MemLength,
	preForkCost uint64,
	postForkCost uint64,
) ([]byte, error) {
	metering := context.GetMeteringContext()

	if !context.GetVMHost().ForkController().FixAuditChangesV6() {
		if preForkCost != 0 {
			// #nosec G115
			metering.UseAndTraceGas(math.MulUint64(preForkCost, uint64(length)))
		}
	} else if postForkCost != 0 && length >= 0 { // a negative length is left for MemLoad to reject
		err := metering.UseGasBounded(math.MulUint64(postForkCost, uint64(length)))
		if err != nil {
			return nil, err
		}
	}

	return context.MemLoad(dataOffset, length)
}

// memLoadDataCopy is loadMemBytes charging DataCopyPerByte per byte on both sides of the fork
func memLoadDataCopy(context *VMHooksImpl, dataOffset executor.MemPtr, length executor.MemLength) ([]byte, error) {
	cost := context.GetMeteringContext().GasSchedule().BaseOperationCost.DataCopyPerByte
	return loadMemBytes(context, dataOffset, length, cost, cost)
}

// mBufferLoadDataCopy is loadMBufferBytes charging DataCopyPerByte per byte on both sides of the fork
func mBufferLoadDataCopy(host vmhost.VMHost, mBufferHandle int32) ([]byte, error) {
	cost := host.Metering().GasSchedule().BaseOperationCost.DataCopyPerByte
	return loadMBufferBytes(host, mBufferHandle, cost, cost)
}

// checkVerifyKeyAndSigLength rejects a wrong-sized public key or signature before either is copied
func checkVerifyKeyAndSigLength(host vmhost.VMHost, keyHandle, keyLength, sigHandle, sigLength int32) error {
	if err := checkMBufferLength(host, keyHandle, vmhost.ErrInvalidPublicKeySize, keyLength); err != nil {
		return err
	}
	return checkMBufferLength(host, sigHandle, vmhost.ErrInvalidSignatureSize, sigLength)
}

// Sha256 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) Sha256(
	dataOffset executor.MemPtr,
	length executor.MemLength,
	resultOffset executor.MemPtr) int32 {

	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), sha256Name, metering.GasSchedule().CryptoAPICost.SHA256)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	data, err := memLoadDataCopy(context, dataOffset, length)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	result, err := crypto.Sha256(data)
	if err != nil {
		context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// ManagedSha256 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedSha256(inputHandle, outputHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), sha256Name, metering.GasSchedule().CryptoAPICost.SHA256)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	inputBytes, err := mBufferLoadDataCopy(context.GetVMHost(), inputHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	resultBytes, err := crypto.Sha256(inputBytes)
	if err != nil {
		context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	managedType.SetBytes(outputHandle, resultBytes)

	return 0
}

// Keccak256 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) Keccak256(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), keccak256Name, metering.GasSchedule().CryptoAPICost.Keccak256)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	data, err := memLoadDataCopy(context, dataOffset, length)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	result, err := crypto.Keccak256(data)
	if err != nil {
		context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// ManagedKeccak256 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedKeccak256(inputHandle, outputHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), keccak256Name, metering.GasSchedule().CryptoAPICost.Keccak256)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	inputBytes, err := mBufferLoadDataCopy(context.GetVMHost(), inputHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	resultBytes, err := crypto.Keccak256(inputBytes)
	if err != nil {
		context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	managedType.SetBytes(outputHandle, resultBytes)

	return 0
}

// Ripemd160 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) Ripemd160(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), ripemd160Name, metering.GasSchedule().CryptoAPICost.Ripemd160)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	data, err := memLoadDataCopy(context, dataOffset, length)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	result, err := crypto.Ripemd160(data)
	if err != nil {
		context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// ManagedRipemd160 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedRipemd160(inputHandle int32, outputHandle int32) int32 {
	host := context.GetVMHost()
	return ManagedRipemd160WithHost(host, inputHandle, outputHandle)
}

// ManagedRipemd160WithHost VMHooks implementation.
func ManagedRipemd160WithHost(host vmhost.VMHost, inputHandle int32, outputHandle int32) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	crypto := host.Crypto()

	err := startTracingAndUseGas(host, ripemd160Name, metering.GasSchedule().CryptoAPICost.Ripemd160)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	inputBytes, err := mBufferLoadDataCopy(host, inputHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	result, err := crypto.Ripemd160(inputBytes)
	if err != nil {
		WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	managedType.SetBytes(outputHandle, result)

	return 0
}

// VerifyBLS VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) VerifyBLS(
	keyOffset executor.MemPtr,
	messageOffset executor.MemPtr,
	messageLength executor.MemLength,
	sigOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), verifyBLSName, metering.GasSchedule().CryptoAPICost.VerifyBLS)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	key, err := loadMemBytes(context, keyOffset, blsPublicKeyLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	message, err := memLoadDataCopy(context, messageOffset, messageLength)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	sig, err := loadMemBytes(context, sigOffset, blsSignatureLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifyBLS(key, message, sig)
	if invalidSigErr != nil {
		context.WithFault(invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// ManagedVerifyBLS VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedVerifyBLS(
	keyHandle int32,
	messageHandle int32,
	sigHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedVerifyBLSWithHost(host, keyHandle, messageHandle, sigHandle)
}

// ManagedVerifyBLSWithHost VMHooks implementation.
func ManagedVerifyBLSWithHost(
	host vmhost.VMHost,
	keyHandle int32,
	messageHandle int32,
	sigHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	crypto := host.Crypto()

	err := startTracingAndUseGas(host, verifyBLSName, metering.GasSchedule().CryptoAPICost.VerifyBLS)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	err = checkVerifyKeyAndSigLength(host, keyHandle, blsPublicKeyLength, sigHandle, blsSignatureLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	keyBytes, err := mBufferLoadDataCopy(host, keyHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	msgBytes, err := mBufferLoadDataCopy(host, messageHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sigBytes, err := mBufferLoadDataCopy(host, sigHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifyBLS(keyBytes, msgBytes, sigBytes)
	if invalidSigErr != nil {
		WithFaultAndHost(host, invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// VerifyEd25519 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) VerifyEd25519(
	keyOffset executor.MemPtr,
	messageOffset executor.MemPtr,
	messageLength executor.MemLength,
	sigOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), verifyEd25519Name, metering.GasSchedule().CryptoAPICost.VerifyEd25519)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	key, err := loadMemBytes(context, keyOffset, ed25519PublicKeyLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	message, err := memLoadDataCopy(context, messageOffset, messageLength)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	sig, err := loadMemBytes(context, sigOffset, ed25519SignatureLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifyEd25519(key, message, sig)
	if invalidSigErr != nil {
		context.WithFault(invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// ManagedVerifyEd25519 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedVerifyEd25519(
	keyHandle, messageHandle, sigHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedVerifyEd25519WithHost(host, keyHandle, messageHandle, sigHandle)
}

// ManagedVerifyEd25519WithHost VMHooks implementation.
func ManagedVerifyEd25519WithHost(
	host vmhost.VMHost,
	keyHandle, messageHandle, sigHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	crypto := host.Crypto()

	err := startTracingAndUseGas(host, verifyEd25519Name, metering.GasSchedule().CryptoAPICost.VerifyEd25519)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	err = checkVerifyKeyAndSigLength(host, keyHandle, ed25519PublicKeyLength, sigHandle, ed25519SignatureLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	keyBytes, err := mBufferLoadDataCopy(host, keyHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	msgBytes, err := mBufferLoadDataCopy(host, messageHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	sigBytes, err := mBufferLoadDataCopy(host, sigHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifyEd25519(keyBytes, msgBytes, sigBytes)
	if invalidSigErr != nil {
		WithFaultAndHost(host, invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// VerifyCustomSecp256k1 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) VerifyCustomSecp256k1(
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	messageOffset executor.MemPtr,
	messageLength executor.MemLength,
	sigOffset executor.MemPtr,
	hashType int32,
) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), verifyCustomSecp256k1Name, metering.GasSchedule().CryptoAPICost.VerifySecp256k1)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	if keyLength != secp256k1CompressedPublicKeyLength && keyLength != secp256k1UncompressedPublicKeyLength {
		_ = context.WithFault(vmhost.ErrInvalidPublicKeySize, runtime.BaseOpsErrorShouldFailExecution())
		return 1
	}

	plainMessage := isSecp256k1PlainMessage(hashType)
	if plainMessage {
		err = checkMemLength(context.GetVMHost(), messageLength, vmhost.ErrInvalidSecp256k1PlainMessageLength, secp256k1PlainMessageLength)
		if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	key, err := loadMemBytes(context, keyOffset, keyLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	message, err := memLoadDataCopy(context, messageOffset, messageLength)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	if plainMessage {
		err = checkSecp256k1PlainMessage(context.GetVMHost(), message)
		if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	// read the 2 leading bytes first
	// byte1: 0x30, header
	// byte2: the remaining buffer length
	const sigHeaderLength = 2
	sigHeader, err := loadMemBytes(context, sigOffset, sigHeaderLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	// MemLoad truncates instead of failing when the range runs past the end of contract memory, so the
	// header can come back shorter than it asked for. Indexing it then panics, which the host recovers
	// as ErrExecutionPanicked; from FixAuditChangesV6 it is a plain out-of-bounds fault instead. The
	// pre-fork path keeps panicking, because replayed history recorded that result
	if context.GetVMHost().ForkController().FixAuditChangesV6() && len(sigHeader) < sigHeaderLength {
		_ = context.WithFault(vmhost.ErrBadBounds, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	sigLength := int32(sigHeader[1]) + sigHeaderLength
	sig, err := loadMemBytes(context, sigOffset, sigLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifySecp256k1(key, message, sig, uint8(hashType)) // #nosec G115
	if invalidSigErr != nil {
		context.WithFault(invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// ManagedVerifyCustomSecp256k1 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedVerifyCustomSecp256k1(
	keyHandle, messageHandle, sigHandle int32,
	hashType int32,
) int32 {
	host := context.GetVMHost()
	return ManagedVerifyCustomSecp256k1WithHost(
		host,
		keyHandle,
		messageHandle,
		sigHandle,
		hashType)
}

// ManagedVerifyCustomSecp256k1WithHost VMHooks implementation.
func ManagedVerifyCustomSecp256k1WithHost(
	host vmhost.VMHost,
	keyHandle, messageHandle, sigHandle int32,
	hashType int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	crypto := host.Crypto()

	err := startTracingAndUseGas(host, verifyCustomSecp256k1Name, metering.GasSchedule().CryptoAPICost.VerifySecp256k1)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	// the key sizes VerifyCustomSecp256k1 accepts
	err = checkMBufferLength(host, keyHandle, vmhost.ErrInvalidPublicKeySize,
		secp256k1CompressedPublicKeyLength, secp256k1UncompressedPublicKeyLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	if isSecp256k1PlainMessage(hashType) {
		err = checkMBufferLength(host, messageHandle, vmhost.ErrInvalidSecp256k1PlainMessageLength, secp256k1PlainMessageLength)
		if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	keyBytes, err := mBufferLoadDataCopy(host, keyHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	msgBytes, err := mBufferLoadDataCopy(host, messageHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	if isSecp256k1PlainMessage(hashType) {
		err = checkSecp256k1PlainMessage(host, msgBytes)
		if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
			return 1
		}
	}

	sigBytes, err := mBufferLoadDataCopy(host, sigHandle)
	if WithFaultAndHost(host, err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return 1
	}

	invalidSigErr := crypto.VerifySecp256k1(keyBytes, msgBytes, sigBytes, uint8(hashType)) // #nosec G115
	if invalidSigErr != nil {
		WithFaultAndHost(host, invalidSigErr, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	return 0
}

// isSecp256k1PlainMessage reports whether hashType selects plain-message mode, where the message goes to
// the curve unhashed and the curve reads it as a 32-byte scalar: bytes past 32 are dropped and a shorter
// message is zero-extended, so a signature over one digest would also pass for that digest with anything
// appended. The verify hooks then hold the message to exactly 32 bytes from FixAuditChangesV6, before
// anything is copied, with checkMemLength / checkMBufferLength. hashType is compared after the same uint8
// narrowing VerifySecp256k1 applies, so a value like 256 narrows to ECDSAPlainMsg and is held to the same
// 32-byte rule instead of slipping past it
func isSecp256k1PlainMessage(hashType int32) bool {
	// #nosec G115
	return secp256k1.MessageHashType(uint8(hashType)) == secp256k1.ECDSAPlainMsg
}

// checkSecp256k1PlainMessage rejects a plain message MemLoad truncated (FixAuditChangesV6) and, from
// FixAuditChangesV7, one not below N, since the curve silently reduces it mod N
func checkSecp256k1PlainMessage(host vmhost.VMHost, message []byte) error {
	if host.ForkController().FixAuditChangesV6() && len(message) != secp256k1PlainMessageLength {
		return vmhost.ErrBadBounds
	}
	if !host.ForkController().FixAuditChangesV7() {
		return nil
	}

	var scalar btcec.ModNScalar
	if scalar.SetByteSlice(message) {
		return vmhost.ErrNonCanonicalSecp256k1PlainMessage
	}

	return nil
}

// VerifySecp256k1 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) VerifySecp256k1(
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	messageOffset executor.MemPtr,
	messageLength executor.MemLength,
	sigOffset executor.MemPtr,
) int32 {
	return context.VerifyCustomSecp256k1(
		keyOffset,
		keyLength,
		messageOffset,
		messageLength,
		sigOffset,
		int32(secp256k1.ECDSADoubleSha256),
	)
}

// ManagedVerifySecp256k1 VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedVerifySecp256k1(
	keyHandle, messageHandle, sigHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedVerifySecp256k1WithHost(host, keyHandle, messageHandle, sigHandle)
}

// ManagedVerifySecp256k1WithHost VMHooks implementation.
func ManagedVerifySecp256k1WithHost(
	host vmhost.VMHost,
	keyHandle, messageHandle, sigHandle int32,
) int32 {
	return ManagedVerifyCustomSecp256k1WithHost(
		host,
		keyHandle,
		messageHandle,
		sigHandle,
		int32(secp256k1.ECDSADoubleSha256),
	)
}

// EncodeSecp256k1DerSignature VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) EncodeSecp256k1DerSignature(
	rOffset executor.MemPtr,
	rLength executor.MemLength,
	sOffset executor.MemPtr,
	sLength executor.MemLength,
	sigOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	crypto := context.GetCryptoContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), encodeSecp256k1DerSignatureName, metering.GasSchedule().CryptoAPICost.EncodeDERSig)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	// secp256k1 is designed to works with values till 32 bytes
	if rLength > secp256k1SignatureComponentMaxLength ||
		sLength > secp256k1SignatureComponentMaxLength {
		_ = context.WithFault(
			vmhost.ErrRAndSExceedsMax32BytesSecp256k1,
			runtime.CryptoAPIErrorShouldFailExecution(),
		)
		return 1
	}

	r, err := loadMemBytes(context, rOffset, rLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	s, err := loadMemBytes(context, sOffset, sLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	derSig, err := crypto.EncodeSecp256k1DERSignature(r, s)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	err = context.MemStore(sigOffset, derSig)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// ManagedEncodeSecp256k1DerSignature VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedEncodeSecp256k1DerSignature(
	rHandle, sHandle, sigHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedEncodeSecp256k1DerSignatureWithHost(host, rHandle, sHandle, sigHandle)
}

// ManagedEncodeSecp256k1DerSignatureWithHost VMHooks implementation.
func ManagedEncodeSecp256k1DerSignatureWithHost(
	host vmhost.VMHost,
	rHandle, sHandle, sigHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	crypto := host.Crypto()

	err := startTracingAndUseGas(host, encodeSecp256k1DerSignatureName, metering.GasSchedule().CryptoAPICost.EncodeDERSig)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	err = checkMBufferMaxLength(host, rHandle, vmhost.ErrRAndSExceedsMax32BytesSecp256k1, secp256k1SignatureComponentMaxLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	err = checkMBufferMaxLength(host, sHandle, vmhost.ErrRAndSExceedsMax32BytesSecp256k1, secp256k1SignatureComponentMaxLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	r, err := loadMBufferBytes(host, rHandle, 0, 0)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	s, err := loadMBufferBytes(host, sHandle, 0, 0)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	// secp256k1 is designed to works with values till 32 bytes
	if len(s) > secp256k1SignatureComponentMaxLength ||
		len(r) > secp256k1SignatureComponentMaxLength {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrRAndSExceedsMax32BytesSecp256k1,
			runtime.CryptoAPIErrorShouldFailExecution(),
		)
		return 1
	}

	derSig, err := crypto.EncodeSecp256k1DERSignature(r, s)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	managedType.SetBytes(sigHandle, derSig)

	return 0
}

// AddEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) AddEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	fstPointXHandle int32,
	fstPointYHandle int32,
	sndPointXHandle int32,
	sndPointYHandle int32,
) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return
	}
	err := startTracingAndUseGas(context.GetVMHost(), addECName, metering.GasSchedule().CryptoAPICost.AddECC*uint64(curveMultiplier)/100)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return
	}

	ec, err1 := managedType.GetEllipticCurve(ecHandle)
	if context.WithFault(err1, runtime.CryptoAPIErrorShouldFailExecution()) {
		return
	}

	xResult, yResult, err := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrNoBigIntUnderThisHandle, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	x1, y1, err := managedType.GetTwoBigInt(fstPointXHandle, fstPointYHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrNoBigIntUnderThisHandle, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}
	x2, y2, err := managedType.GetTwoBigInt(sndPointXHandle, sndPointYHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrNoBigIntUnderThisHandle, runtime.BigIntAPIErrorShouldFailExecution())
		return
	}

	if !ec.IsOnCurve(x1, y1) || !ec.IsOnCurve(x2, y2) {
		_ = context.WithFault(vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return
	}

	managedType.ConsumeGasForBigIntCopy(xResult, yResult, ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x1, y1, x2, y2)
	xResultAdd, yResultAdd := ec.Add(x1, y1, x2, y2)
	xResult.Set(xResultAdd)
	yResult.Set(yResultAdd)
}

// DoubleEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) DoubleEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
) {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return
	}
	err := startTracingAndUseGas(context.GetVMHost(), doubleECName, metering.GasSchedule().CryptoAPICost.DoubleECC*uint64(curveMultiplier)/100)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return
	}

	ec, err1 := managedType.GetEllipticCurve(ecHandle)
	if context.WithFault(err1, runtime.CryptoAPIErrorShouldFailExecution()) {
		return
	}

	xResult, yResult, err1 := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	x, y, err2 := managedType.GetTwoBigInt(pointXHandle, pointYHandle)
	if err1 != nil || err2 != nil {
		_ = context.WithFault(vmhost.ErrNoBigIntUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return
	}
	if !ec.IsOnCurve(x, y) {
		_ = context.WithFault(vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return
	}

	managedType.ConsumeGasForBigIntCopy(xResult, yResult, ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x, y)

	xResultDouble, yResultDouble := ec.Double(x, y)
	xResult.Set(xResultDouble)
	yResult.Set(yResultDouble)
}

// IsOnCurveEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) IsOnCurveEC(
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}
	err := startTracingAndUseGas(context.GetVMHost(), isOnCurveECName, metering.GasSchedule().CryptoAPICost.IsOnCurveECC*uint64(curveMultiplier)/100)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	x, y, err := managedType.GetTwoBigInt(pointXHandle, pointYHandle)
	if err != nil || x == nil || y == nil {
		_ = context.WithFault(vmhost.ErrNoBigIntUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x, y)
	if ec.IsOnCurve(x, y) {
		return 1
	}

	return 0
}

// ScalarBaseMultEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ScalarBaseMultEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataOffset executor.MemPtr,
	length executor.MemLength,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	if length < 0 {
		_ = context.WithFault(vmhost.ErrNegativeLength, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	curveMultiplier := managedType.GetScalarMult100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	oneByteScalarGasCost := metering.GasSchedule().CryptoAPICost.ScalarMultECC * uint64(curveMultiplier) / 100
	err := startTracingAndUseGas(context.GetVMHost(), scalarBaseMultECName, oneByteScalarGasCost)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMemBytes(context, dataOffset, length, oneByteScalarGasCost, oneByteScalarGasCost)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	host := context.GetVMHost()
	return commonScalarBaseMultEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

// ManagedScalarBaseMultEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedScalarBaseMultEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedScalarBaseMultECWithHost(
		host,
		xResultHandle,
		yResultHandle,
		ecHandle,
		dataHandle,
	)
}

// ManagedScalarBaseMultECWithHost VMHooks implementation.
func ManagedScalarBaseMultECWithHost(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.GetScalarMult100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = WithFaultAndHost(host, vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	oneByteScalarGasCost := metering.GasSchedule().CryptoAPICost.ScalarMultECC * uint64(curveMultiplier) / 100
	err := startTracingAndUseGas(host, scalarBaseMultECName, oneByteScalarGasCost)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMBufferBytes(host, dataHandle, oneByteScalarGasCost, oneByteScalarGasCost)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonScalarBaseMultEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

// reduceScalarModOrder reduces, from FixAuditChangesV6, a scalar longer than the curve order's byte size
// to the same scalar modulo the order, as crypto/elliptic's own constant-time curves do. The curves here
// have cofactor 1, so k·P = (k mod N)·P: the point is exactly the one the generic multiplication always
// returned, it just no longer walks bits that cannot change it. A scalar that fits is passed on as is
func reduceScalarModOrder(host vmhost.VMHost, ec *elliptic.CurveParams, scalar []byte) []byte {
	if !host.ForkController().FixAuditChangesV6() || len(scalar) <= (ec.N.BitLen()+7)/8 {
		return scalar
	}
	return new(big.Int).Mod(new(big.Int).SetBytes(scalar), ec.N).Bytes()
}

func commonScalarBaseMultEC(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	data []byte,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	xResult, yResult, err := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, xResult, yResult)

	xResultSBM, yResultSBM := ec.ScalarBaseMult(reduceScalarModOrder(host, ec, data))
	if !ec.IsOnCurve(xResultSBM, yResultSBM) {
		_ = WithFaultAndHost(host, vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	xResult.Set(xResultSBM)
	yResult.Set(yResultSBM)

	return 0
}

// ScalarMultEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ScalarMultEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
	dataOffset executor.MemPtr,
	length executor.MemLength,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	if length < 0 {
		_ = context.WithFault(vmhost.ErrNegativeLength, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	curveMultiplier := managedType.GetScalarMult100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	oneByteScalarGasCost := metering.GasSchedule().CryptoAPICost.ScalarMultECC * uint64(curveMultiplier) / 100
	err := startTracingAndUseGas(context.GetVMHost(), scalarMultECName, oneByteScalarGasCost)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMemBytes(context, dataOffset, length, oneByteScalarGasCost, oneByteScalarGasCost)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	host := context.GetVMHost()
	return commonScalarMultEC(host, xResultHandle, yResultHandle, ecHandle, pointXHandle, pointYHandle, data)
}

// ManagedScalarMultEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedScalarMultEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
	dataHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedScalarMultECWithHost(
		host,
		xResultHandle,
		yResultHandle,
		ecHandle,
		pointXHandle,
		pointYHandle,
		dataHandle,
	)
}

// ManagedScalarMultECWithHost VMHooks implementation.
func ManagedScalarMultECWithHost(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
	dataHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.GetScalarMult100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = WithFaultAndHost(host, vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	oneByteScalarGasCost := metering.GasSchedule().CryptoAPICost.ScalarMultECC * uint64(curveMultiplier) / 100
	err := startTracingAndUseGas(host, scalarMultECName, oneByteScalarGasCost)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMBufferBytes(host, dataHandle, oneByteScalarGasCost, oneByteScalarGasCost)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonScalarMultEC(host, xResultHandle, yResultHandle, ecHandle, pointXHandle, pointYHandle, data)
}

func commonScalarMultEC(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	pointXHandle int32,
	pointYHandle int32,
	data []byte,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()

	ec, err1 := managedType.GetEllipticCurve(ecHandle)
	if WithFaultAndHost(host, err1, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	xResult, yResult, err1 := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	x, y, err2 := managedType.GetTwoBigInt(pointXHandle, pointYHandle)
	if err1 != nil || err2 != nil {
		_ = WithFaultAndHost(host, vmhost.ErrNoBigIntUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	if !ec.IsOnCurve(x, y) {
		_ = WithFaultAndHost(host, vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	managedType.ConsumeGasForBigIntCopy(xResult, yResult, ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x, y)
	xResultSM, yResultSM := ec.ScalarMult(x, y, reduceScalarModOrder(host, ec, data))
	if !ec.IsOnCurve(xResultSM, yResultSM) {
		_ = WithFaultAndHost(host, vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	xResult.Set(xResultSM)
	yResult.Set(yResultSM)

	return 0
}

// MarshalEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MarshalEC(
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	host := context.GetVMHost()
	result, err := commonMarshalEC(host, xPairHandle, yPairHandle, ecHandle)
	if err != nil {
		_ = context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	return int32(len(result)) // #nosec G115
}

// ManagedMarshalEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedMarshalEC(
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedMarshalECWithHost(
		host,
		xPairHandle,
		yPairHandle,
		ecHandle,
		resultHandle,
	)
}

// ManagedMarshalECWithHost VMHooks implementation.
func ManagedMarshalECWithHost(
	host vmhost.VMHost,
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	result, err := commonMarshalEC(host, xPairHandle, yPairHandle, ecHandle)
	if err != nil {
		_ = WithFaultAndHost(host, err, true)
		return -1
	}

	managedType := host.ManagedTypes()
	managedType.SetBytes(resultHandle, result)
	return int32(len(result)) // #nosec G115
}

func commonMarshalEC(
	host vmhost.VMHost,
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
) ([]byte, error) {
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		return nil, vmhost.ErrNoEllipticCurveUnderThisHandle
	}
	err := startTracingAndUseGas(host, marshalECName, metering.GasSchedule().CryptoAPICost.MarshalECC*uint64(curveMultiplier)/100)
	if err != nil {
		return nil, err
	}

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if err != nil {
		return nil, err
	}

	x, y, err := managedType.GetTwoBigInt(xPairHandle, yPairHandle)
	if err != nil {
		return nil, err
	}
	if !ec.IsOnCurve(x, y) {
		return nil, vmhost.ErrPointNotOnCurve
	}
	if x.BitLen() > ec.BitSize || y.BitLen() > ec.BitSize {
		return nil, vmhost.ErrLengthOfBufferNotCorrect
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x, y)

	result := elliptic.Marshal(ec, x, y)
	return result, nil
}

// MarshalCompressedEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) MarshalCompressedEC(
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	host := context.GetVMHost()
	result, err := commonMarshalCompressedEC(host, xPairHandle, yPairHandle, ecHandle)
	if err != nil {
		_ = context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	return int32(len(result)) // #nosec G115
}

// ManagedMarshalCompressedEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedMarshalCompressedEC(
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedMarshalCompressedECWithHost(
		host,
		xPairHandle,
		yPairHandle,
		ecHandle,
		resultHandle,
	)
}

// ManagedMarshalCompressedECWithHost VMHooks implementation.
func ManagedMarshalCompressedECWithHost(
	host vmhost.VMHost,
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()
	result, err := commonMarshalCompressedEC(host, xPairHandle, yPairHandle, ecHandle)
	if err != nil {
		_ = WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}

	managedType.SetBytes(resultHandle, result)
	return int32(len(result)) // #nosec G115
}

func commonMarshalCompressedEC(host vmhost.VMHost,
	xPairHandle int32,
	yPairHandle int32,
	ecHandle int32,
) ([]byte, error) {
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		return nil, vmhost.ErrNoEllipticCurveUnderThisHandle
	}
	err := startTracingAndUseGas(host, marshalCompressedECName, metering.GasSchedule().CryptoAPICost.MarshalCompressedECC*uint64(curveMultiplier)/100)
	if err != nil {
		return nil, err
	}

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if err != nil {
		return nil, err
	}

	x, y, err := managedType.GetTwoBigInt(xPairHandle, yPairHandle)
	if err != nil || x == nil || y == nil {
		return nil, vmhost.ErrNoBigIntUnderThisHandle
	}
	if !ec.IsOnCurve(x, y) {
		return nil, vmhost.ErrPointNotOnCurve
	}
	if x.BitLen() > ec.BitSize || y.BitLen() > ec.BitSize {
		return nil, vmhost.ErrLengthOfBufferNotCorrect
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, x, y)

	result := elliptic.MarshalCompressed(ec, x, y)
	return result, nil
}

// UnmarshalEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) UnmarshalEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataOffset executor.MemPtr,
	length executor.MemLength,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	host := context.GetVMHost()
	err := startTracingAndUseGas(host, unmarshalECName, metering.GasSchedule().CryptoAPICost.UnmarshalECC*uint64(curveMultiplier)/100)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	byteLen := (managedType.GetEllipticCurveSizeOfField(ecHandle) + 7) / 8
	err = checkMemLength(host, length, vmhost.ErrLengthOfBufferNotCorrect, 1+2*byteLen)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMemBytes(context, dataOffset, length, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonUnmarshalEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

// ManagedUnmarshalEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedUnmarshalEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedUnmarshalECWithHost(
		host,
		xResultHandle,
		yResultHandle,
		ecHandle,
		dataHandle,
	)
}

// ManagedUnmarshalECWithHost VMHooks implementation.
func ManagedUnmarshalECWithHost(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = WithFaultAndHost(host, vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	err := startTracingAndUseGas(host, unmarshalECName, metering.GasSchedule().CryptoAPICost.UnmarshalECC*uint64(curveMultiplier)/100)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	byteLen := (managedType.GetEllipticCurveSizeOfField(ecHandle) + 7) / 8
	err = checkMBufferLength(host, dataHandle, vmhost.ErrLengthOfBufferNotCorrect, 1+2*byteLen)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMBufferBytes(host, dataHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	// before FixAuditChangesV6 an error here returned 1 without failing the execution; from it an unpaid
	// load has to stop the contract
	if WithFaultAndHost(host, err, host.ForkController().FixAuditChangesV6() && runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonUnmarshalEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

func commonUnmarshalEC(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	data []byte,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	byteLen := (ec.BitSize + 7) / 8
	if len(data) != 1+2*byteLen {
		_ = WithFaultAndHost(host, vmhost.ErrLengthOfBufferNotCorrect, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	xResult, yResult, err := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, xResult, yResult)

	xResultU, yResultU := elliptic.Unmarshal(ec, data)
	if xResultU == nil || yResultU == nil || !ec.IsOnCurve(xResultU, yResultU) {
		_ = WithFaultAndHost(host, vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	xResult.Set(xResultU)
	yResult.Set(yResultU)

	return 0
}

// UnmarshalCompressedEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) UnmarshalCompressedEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataOffset executor.MemPtr,
	length executor.MemLength,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	curveMultiplier := managedType.GetUCompressed100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	host := context.GetVMHost()
	err := startTracingAndUseGas(host, unmarshalCompressedECName, metering.GasSchedule().CryptoAPICost.UnmarshalCompressedECC*uint64(curveMultiplier)/100)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	byteLen := (managedType.GetEllipticCurveSizeOfField(ecHandle) + 7) / 8
	err = checkMemLength(host, length, vmhost.ErrLengthOfBufferNotCorrect, byteLen+1)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMemBytes(context, dataOffset, length, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	// a failed load returns no data, so report failure like the sibling hooks rather than
	// int32(len(data)), which is 0 here and is the success code
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonUnmarshalCompressedEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

// ManagedUnmarshalCompressedEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedUnmarshalCompressedEC(
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedUnmarshalCompressedECWithHost(
		host,
		xResultHandle,
		yResultHandle,
		ecHandle,
		dataHandle,
	)
}

// ManagedUnmarshalCompressedECWithHost VMHooks implementation.
func ManagedUnmarshalCompressedECWithHost(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	dataHandle int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.GetUCompressed100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		_ = WithFaultAndHost(host, vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	err := startTracingAndUseGas(host, unmarshalCompressedECName, metering.GasSchedule().CryptoAPICost.UnmarshalCompressedECC*uint64(curveMultiplier)/100)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	byteLen := (managedType.GetEllipticCurveSizeOfField(ecHandle) + 7) / 8
	err = checkMBufferLength(host, dataHandle, vmhost.ErrLengthOfBufferNotCorrect, byteLen+1)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	data, err := loadMBufferBytes(host, dataHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	// a failed load returns no data, so report failure like the sibling hooks rather than
	// int32(len(data)), which is 0 here and is the success code
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	return commonUnmarshalCompressedEC(host, xResultHandle, yResultHandle, ecHandle, data)
}

func commonUnmarshalCompressedEC(
	host vmhost.VMHost,
	xResultHandle int32,
	yResultHandle int32,
	ecHandle int32,
	data []byte,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}
	byteLen := (ec.BitSize+7)/8 + 1
	if len(data) != byteLen {
		_ = WithFaultAndHost(host, vmhost.ErrLengthOfBufferNotCorrect, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}

	xResult, yResult, err := managedType.GetTwoBigInt(xResultHandle, yResultHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, xResult, yResult)

	xResultUC, yResultUC := elliptic.UnmarshalCompressed(ec, data)
	if xResultUC == nil || yResultUC == nil || !ec.IsOnCurve(xResultUC, yResultUC) {
		_ = WithFaultAndHost(host, vmhost.ErrPointNotOnCurve, runtime.CryptoAPIErrorShouldFailExecution())
		return 1
	}
	xResult.Set(xResultUC)
	yResult.Set(yResultUC)
	return 0
}

// GenerateKeyEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GenerateKeyEC(
	xPubKeyHandle int32,
	yPubKeyHandle int32,
	ecHandle int32,
	resultOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	host := context.GetVMHost()
	result, err := commonGenerateEC(host, xPubKeyHandle, yPubKeyHandle, ecHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	err = context.MemStore(resultOffset, result)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return int32(len(result)) // #nosec G115
	}

	return 0
}

// ManagedGenerateKeyEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGenerateKeyEC(
	xPubKeyHandle int32,
	yPubKeyHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	return ManagedGenerateKeyECWithHost(
		host,
		xPubKeyHandle,
		yPubKeyHandle,
		ecHandle,
		resultHandle,
	)
}

// ManagedGenerateKeyECWithHost VMHooks implementation.
func ManagedGenerateKeyECWithHost(
	host vmhost.VMHost,
	xPubKeyHandle int32,
	yPubKeyHandle int32,
	ecHandle int32,
	resultHandle int32,
) int32 {
	runtime := host.Runtime()
	managedType := host.ManagedTypes()
	result, err := commonGenerateEC(host, xPubKeyHandle, yPubKeyHandle, ecHandle)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return 1
	}

	managedType.SetBytes(resultHandle, result)
	return 0
}

func commonGenerateEC(
	host vmhost.VMHost,
	xPubKeyHandle int32,
	yPubKeyHandle int32,
	ecHandle int32,
) ([]byte, error) {
	metering := host.Metering()
	managedType := host.ManagedTypes()

	curveMultiplier := managedType.Get100xCurveGasCostMultiplier(ecHandle)
	if curveMultiplier < 0 {
		return nil, vmhost.ErrNoEllipticCurveUnderThisHandle
	}
	if curveMultiplier == 250 {
		curveMultiplier = 500
	}
	// #nosec G115
	err := startTracingAndUseGas(host, generateKeyECName, metering.GasSchedule().CryptoAPICost.GenerateKeyECC*uint64(curveMultiplier)/100)
	if err != nil {
		return nil, err
	}

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if err != nil {
		return nil, err
	}

	xPubKey, yPubKey, err := managedType.GetTwoBigInt(xPubKeyHandle, yPubKeyHandle)
	if err != nil {
		return nil, err
	}
	managedType.ConsumeGasForBigIntCopy(ec.P, ec.N, ec.B, ec.Gx, ec.Gy, xPubKey, yPubKey)

	ioReader := managedType.GetRandReader()
	result, xPubKeyGK, yPubKeyGK, err := elliptic.GenerateKey(ec, ioReader)
	if err != nil {
		return nil, err
	}

	xPubKey.Set(xPubKeyGK)
	yPubKey.Set(yPubKeyGK)

	return result, nil
}

// CreateEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) CreateEC(dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	err := startTracingAndUseGas(context.GetVMHost(), createECName, metering.GasSchedule().CryptoAPICost.EllipticCurveNew)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	if dataLength != curveNameLength {
		_ = context.WithFault(vmhost.ErrBadBounds, runtime.CryptoAPIErrorShouldFailExecution())
		return -1
	}
	data, err := loadMemBytes(context, dataOffset, dataLength, 0, 0)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	curveChoice := string(data[:])
	switch curveChoice {
	case "p224":
		curveParams := elliptic.P224().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p256":
		curveParams := elliptic.P256().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p384":
		curveParams := elliptic.P384().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p521":
		curveParams := elliptic.P521().Params()
		return managedType.PutEllipticCurve(curveParams)
	}
	return -1
}

// ManagedCreateEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedCreateEC(dataHandle int32) int32 {
	host := context.GetVMHost()
	return ManagedCreateECWithHost(host, dataHandle)
}

// ManagedCreateECWithHost VMHooks implementation.
func ManagedCreateECWithHost(host vmhost.VMHost, dataHandle int32) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	err := startTracingAndUseGas(host, createECName, metering.GasSchedule().CryptoAPICost.EllipticCurveNew)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	err = checkMBufferLength(host, dataHandle, vmhost.ErrBadBounds, curveNameLength)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	data, err := loadMBufferBytes(host, dataHandle, 0, 0)
	if WithFaultAndHost(host, err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	curveChoice := string(data[:])
	switch curveChoice {
	case "p224":
		curveParams := elliptic.P224().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p256":
		curveParams := elliptic.P256().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p384":
		curveParams := elliptic.P384().Params()
		return managedType.PutEllipticCurve(curveParams)
	case "p521":
		curveParams := elliptic.P521().Params()
		return managedType.PutEllipticCurve(curveParams)
	}

	_ = WithFaultAndHost(host, vmhost.ErrBadBounds, runtime.CryptoAPIErrorShouldFailExecution())
	return -1
}

// GetCurveLengthEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCurveLengthEC(ecHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	err := startTracingAndUseGas(context.GetVMHost(), getCurveLengthECName, metering.GasSchedule().BigIntAPICost.BigIntGetInt64)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	ecLength := managedType.GetEllipticCurveSizeOfField(ecHandle)
	if ecLength == -1 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.BigIntAPIErrorShouldFailExecution())
	}

	return ecLength
}

// GetPrivKeyByteLengthEC VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrivKeyByteLengthEC(ecHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	err := startTracingAndUseGas(context.GetVMHost(), getPrivKeyByteLengthECName, metering.GasSchedule().BigIntAPICost.BigIntGetInt64)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	byteLength := managedType.GetPrivateKeyByteLengthEC(ecHandle)
	if byteLength == -1 {
		_ = context.WithFault(vmhost.ErrNoEllipticCurveUnderThisHandle, runtime.BigIntAPIErrorShouldFailExecution())
	}

	return byteLength
}

// EllipticCurveGetValues VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) EllipticCurveGetValues(ecHandle int32, fieldOrderHandle int32, basePointOrderHandle int32, eqConstantHandle int32, xBasePointHandle int32, yBasePointHandle int32) int32 {
	managedType := context.GetManagedTypesContext()
	metering := context.GetMeteringContext()
	runtime := context.GetRuntimeContext()

	err := startTracingAndUseGas(context.GetVMHost(), ellipticCurveGetValuesName, metering.GasSchedule().BigIntAPICost.BigIntGetInt64*5)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}

	ec, err := managedType.GetEllipticCurve(ecHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	fieldOrder, basePointOrder, err := managedType.GetTwoBigInt(fieldOrderHandle, basePointOrderHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	eqConstant, err := managedType.GetBigInt(eqConstantHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	xBasePoint, yBasePoint, err := managedType.GetTwoBigInt(xBasePointHandle, yBasePointHandle)
	if context.WithFault(err, runtime.CryptoAPIErrorShouldFailExecution()) {
		return -1
	}
	fieldOrder.Set(ec.P)
	basePointOrder.Set(ec.N)
	eqConstant.Set(ec.B)
	xBasePoint.Set(ec.Gx)
	yBasePoint.Set(ec.Gy)
	return ecHandle
}
