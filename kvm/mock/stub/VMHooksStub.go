package stub

import "github.com/klever-io/klever-go/kvm/executor"

var _ executor.VMHooks = (*VMHooksStub)(nil)

type VMHooksStub struct {
	GetGasLeftCalled                         func() int64
	GetSCAddressCalled                       func(resultOffset executor.MemPtr)
	GetOwnerAddressCalled                    func(resultOffset executor.MemPtr)
	IsSmartContractCalled                    func(addressOffset executor.MemPtr) int32
	SignalErrorCalled                        func(messageOffset executor.MemPtr, messageLength executor.MemLength)
	GetExternalBalanceCalled                 func(addressOffset executor.MemPtr, resultOffset executor.MemPtr)
	GetBlockHashCalled                       func(nonce int64, resultOffset executor.MemPtr) int32
	GetKDABalanceCalled                      func(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, resultOffset executor.MemPtr) int32
	GetKDANFTNameLengthCalled                func(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64) int32
	GetKDANFTURILengthCalled                 func(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64) int32
	GetKDATokenDataCalled                    func(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, precisionHandle int32, idOffset executor.MemPtr, nameOffset executor.MemPtr, creatorOffset executor.MemPtr, logoOffset executor.MemPtr, initialSupplyOffset executor.MemPtr, circulatingSupplyOffset executor.MemPtr, maxSupplyOffset executor.MemPtr, mintedOffset executor.MemPtr, burnedOffset executor.MemPtr, royaltiesOffset executor.MemPtr, propertiesOffset executor.MemPtr, attributesOffset executor.MemPtr, rolesOffset executor.MemPtr) int32
	ValidateTokenIdentifierCalled            func(tokenIdHandle int32) int32
	UpgradeContractCalled                    func(destOffset executor.MemPtr, gasLimit int64, valueOffset executor.MemPtr, codeOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, length executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr)
	UpgradeFromSourceContractCalled          func(destOffset executor.MemPtr, gasLimit int64, valueOffset executor.MemPtr, sourceContractAddressOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr)
	DeleteContractCalled                     func(destOffset executor.MemPtr, gasLimit int64, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr)
	GetArgumentLengthCalled                  func(id int32) int32
	GetArgumentCalled                        func(id int32, argOffset executor.MemPtr) int32
	GetFunctionCalled                        func(functionOffset executor.MemPtr) int32
	GetNumArgumentsCalled                    func() int32
	StorageStoreCalled                       func(keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr, dataLength executor.MemLength) int32
	StorageLoadLengthCalled                  func(keyOffset executor.MemPtr, keyLength executor.MemLength) int32
	StorageLoadFromAddressCalled             func(addressOffset executor.MemPtr, keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32
	StorageLoadCalled                        func(keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32
	SetStorageLockCalled                     func(keyOffset executor.MemPtr, keyLength executor.MemLength, lockTimestamp int64) int32
	GetStorageLockCalled                     func(keyOffset executor.MemPtr, keyLength executor.MemLength) int64
	IsStorageLockedCalled                    func(keyOffset executor.MemPtr, keyLength executor.MemLength) int32
	ClearStorageLockCalled                   func(keyOffset executor.MemPtr, keyLength executor.MemLength) int32
	GetCallerCalled                          func(resultOffset executor.MemPtr)
	CheckNoPaymentCalled                     func()
	GetCallValueCalled                       func(resultOffset executor.MemPtr) int32
	GetKDAValueCalled                        func(resultOffset executor.MemPtr) int32
	GetKDAValueByIndexCalled                 func(resultOffset executor.MemPtr, index int32) int32
	GetKDATokenNameCalled                    func(resultOffset executor.MemPtr) int32
	GetKDATokenNameByIndexCalled             func(resultOffset executor.MemPtr, index int32) int32
	GetKDATokenNonceCalled                   func() int64
	GetKDATokenNonceByIndexCalled            func(index int32) int64
	GetKDATokenTypeCalled                    func() int32
	GetKDATokenTypeByIndexCalled             func(index int32) int32
	GetNumKDATransfersCalled                 func() int32
	GetCallValueByTokenNameCalled            func(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr, tokenNameLength executor.MemLength) int32
	GetCallValueTokenNameCalled              func(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr) int32
	GetCallValueTokenNameByIndexCalled       func(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr, index int32) int32
	WriteLogCalled                           func(dataPointer executor.MemPtr, dataLength executor.MemLength, topicPtr executor.MemPtr, numTopics int32)
	WriteEventLogCalled                      func(numTopics int32, topicLengthsOffset executor.MemPtr, topicOffset executor.MemPtr, dataOffset executor.MemPtr, dataLength executor.MemLength)
	GetBlockTimestampCalled                  func() int64
	GetBlockNonceCalled                      func() int64
	GetBlockRoundCalled                      func() int64
	GetBlockEpochCalled                      func() int64
	GetBlockRandomSeedCalled                 func(pointer executor.MemPtr)
	GetStateRootHashCalled                   func(pointer executor.MemPtr)
	GetPrevBlockTimestampCalled              func() int64
	GetPrevBlockNonceCalled                  func() int64
	GetPrevBlockRoundCalled                  func() int64
	GetPrevBlockEpochCalled                  func() int64
	GetPrevBlockRandomSeedCalled             func(pointer executor.MemPtr)
	FinishCalled                             func(pointer executor.MemPtr, length executor.MemLength)
	ExecuteOnSameContextCalled               func(gasLimit int64, addressOffset executor.MemPtr, valueOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32
	ExecuteOnDestContextCalled               func(gasLimit int64, addressOffset executor.MemPtr, valueOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32
	ExecuteReadOnlyCalled                    func(gasLimit int64, addressOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32
	CreateContractCalled                     func(gasLimit int64, valueOffset executor.MemPtr, codeOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32
	DeployFromSourceContractCalled           func(gasLimit int64, valueOffset executor.MemPtr, sourceContractAddressOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, resultAddressOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32
	GetNumReturnDataCalled                   func() int32
	GetReturnDataSizeCalled                  func(resultID int32) int32
	GetReturnDataCalled                      func(resultID int32, dataOffset executor.MemPtr) int32
	CleanReturnDataCalled                    func()
	DeleteFromReturnDataCalled               func(resultID int32)
	GetOriginalTxHashCalled                  func(dataOffset executor.MemPtr)
	GetCurrentTxHashCalled                   func(dataOffset executor.MemPtr)
	ManagedSCAddressCalled                   func(destinationHandle int32)
	ManagedOwnerAddressCalled                func(destinationHandle int32)
	ManagedCallerCalled                      func(destinationHandle int32)
	ManagedSignalErrorCalled                 func(errHandle int32)
	ManagedWriteLogCalled                    func(topicsHandle int32, dataHandle int32)
	ManagedGetOriginalTxHashCalled           func(resultHandle int32)
	ManagedGetStateRootHashCalled            func(resultHandle int32)
	ManagedGetBlockRandomSeedCalled          func(resultHandle int32)
	ManagedGetPrevBlockRandomSeedCalled      func(resultHandle int32)
	ManagedGetReturnDataCalled               func(resultID int32, resultHandle int32)
	ManagedGetKDACallValueCalled             func(kdaCallValueHandle int32, kdaHandle int32)
	ManagedGetMultiKDACallValueCalled        func(multiCallValueHandle int32)
	ManagedGetBackTransfersCalled            func(kdaTransfersValueHandle int32, callValueHandle int32)
	ManagedGetKDABalanceCalled               func(addressHandle int32, tokenIDHandle int32, nonce int64, valueHandle int32)
	ManagedGetUserKDACalled                  func(addressHandle int32, tickerHandle int32, nonce int64, balanceHandle int32, frozenHandle int32, lastClaimHandle int32, bucketsHandle int32, mimeHandle int32, metadataHandle int32)
	ManagedGetKDATokenDataCalled             func(addressHandle int32, tickerHandle int32, nonce int64, precisionHandle int32, idHandle int32, nameHandle int32, creatorHandle int32, adminHandle int32, logoHandle int32, urisHandle int32, initialSupplyHandle int32, circulatingSupplyHandle int32, maxSupplyHandle int32, mintedHandle int32, burnedHandle int32, royaltiesHandle int32, propertiesHandle int32, attributesHandle int32, rolesHandle int32, issueDateHandle int32)
	ManagedGetKDARolesCalled                 func(tickerHandle int32, rolesHandle int32)
	ManagedUpgradeFromSourceContractCalled   func(destHandle int32, gas int64, valueHandle int32, addressHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultHandle int32)
	ManagedUpgradeContractCalled             func(destHandle int32, gas int64, valueHandle int32, codeHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultHandle int32)
	ManagedDeleteContractCalled              func(destHandle int32, gasLimit int64, argumentsHandle int32)
	ManagedDeployFromSourceContractCalled    func(gas int64, valueHandle int32, addressHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultAddressHandle int32, resultHandle int32) int32
	ManagedCreateContractCalled              func(gas int64, valueHandle int32, codeHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultAddressHandle int32, resultHandle int32) int32
	ManagedExecuteReadOnlyCalled             func(gas int64, addressHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32
	ManagedExecuteOnSameContextCalled        func(gas int64, addressHandle int32, valueHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32
	ManagedExecuteOnDestContextCalled        func(gas int64, addressHandle int32, valueHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32
	ManagedMultiTransferKDANFTExecuteCalled  func(dstHandle int32, tokenTransfersHandle int32, gasLimit int64, functionHandle int32, argumentsHandle int32) int32
	ManagedBufferToHexCalled                 func(sourceHandle int32, destHandle int32)
	ManagedGetCodeMetadataCalled             func(addressHandle int32, responseHandle int32)
	ManagedIsBuiltinFunctionCalled           func(functionNameHandle int32) int32
	ManagedGetSftMetadataCalled              func(tickerHandle int32, nonce int64, dataHandle int32)
	ManagedAccHasPermCalled                  func(ops int64, sourceAccAddr int32, targetAccAddr int32) int32
	BigFloatNewFromPartsCalled               func(integralPart int32, fractionalPart int32, exponent int32) int32
	BigFloatNewFromFracCalled                func(numerator int64, denominator int64) int32
	BigFloatNewFromSciCalled                 func(significand int64, exponent int64) int32
	BigFloatAddCalled                        func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigFloatSubCalled                        func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigFloatMulCalled                        func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigFloatDivCalled                        func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigFloatNegCalled                        func(destinationHandle int32, opHandle int32)
	BigFloatCloneCalled                      func(destinationHandle int32, opHandle int32)
	BigFloatCmpCalled                        func(op1Handle int32, op2Handle int32) int32
	BigFloatAbsCalled                        func(destinationHandle int32, opHandle int32)
	BigFloatSignCalled                       func(opHandle int32) int32
	BigFloatSqrtCalled                       func(destinationHandle int32, opHandle int32)
	BigFloatPowCalled                        func(destinationHandle int32, opHandle int32, exponent int32)
	BigFloatFloorCalled                      func(destBigIntHandle int32, opHandle int32)
	BigFloatCeilCalled                       func(destBigIntHandle int32, opHandle int32)
	BigFloatTruncateCalled                   func(destBigIntHandle int32, opHandle int32)
	BigFloatSetInt64Called                   func(destinationHandle int32, value int64)
	BigFloatIsIntCalled                      func(opHandle int32) int32
	BigFloatSetBigIntCalled                  func(destinationHandle int32, bigIntHandle int32)
	BigFloatGetConstPiCalled                 func(destinationHandle int32)
	BigFloatGetConstECalled                  func(destinationHandle int32)
	BigIntGetUnsignedArgumentCalled          func(id int32, destinationHandle int32)
	BigIntGetSignedArgumentCalled            func(id int32, destinationHandle int32)
	BigIntStorageStoreUnsignedCalled         func(keyOffset executor.MemPtr, keyLength executor.MemLength, sourceHandle int32) int32
	BigIntStorageLoadUnsignedCalled          func(keyOffset executor.MemPtr, keyLength executor.MemLength, destinationHandle int32) int32
	BigIntGetCallValueCalled                 func(destinationHandle int32)
	BigIntGetKDACallValueCalled              func(destination int32)
	BigIntGetKDACallValueByIndexCalled       func(destinationHandle int32, index int32)
	BigIntGetExternalBalanceCalled           func(addressOffset executor.MemPtr, result int32)
	BigIntGetKDAExternalBalanceCalled        func(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, resultHandle int32)
	BigIntNewCalled                          func(smallValue int64) int32
	BigIntUnsignedByteLengthCalled           func(referenceHandle int32) int32
	BigIntSignedByteLengthCalled             func(referenceHandle int32) int32
	BigIntGetUnsignedBytesCalled             func(referenceHandle int32, byteOffset executor.MemPtr) int32
	BigIntGetSignedBytesCalled               func(referenceHandle int32, byteOffset executor.MemPtr) int32
	BigIntSetUnsignedBytesCalled             func(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength)
	BigIntSetSignedBytesCalled               func(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength)
	BigIntIsInt64Called                      func(destinationHandle int32) int32
	BigIntGetInt64Called                     func(destinationHandle int32) int64
	BigIntSetInt64Called                     func(destinationHandle int32, value int64)
	BigIntAddCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntSubCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntMulCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntTDivCalled                         func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntTModCalled                         func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntEDivCalled                         func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntEModCalled                         func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntSqrtCalled                         func(destinationHandle int32, opHandle int32)
	BigIntPowCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntLog2Called                         func(op1Handle int32) int32
	BigIntAbsCalled                          func(destinationHandle int32, opHandle int32)
	BigIntNegCalled                          func(destinationHandle int32, opHandle int32)
	BigIntSignCalled                         func(opHandle int32) int32
	BigIntCmpCalled                          func(op1Handle int32, op2Handle int32) int32
	BigIntNotCalled                          func(destinationHandle int32, opHandle int32)
	BigIntAndCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntOrCalled                           func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntXorCalled                          func(destinationHandle int32, op1Handle int32, op2Handle int32)
	BigIntShrCalled                          func(destinationHandle int32, opHandle int32, bits int32)
	BigIntShlCalled                          func(destinationHandle int32, opHandle int32, bits int32)
	BigIntFinishUnsignedCalled               func(referenceHandle int32)
	BigIntFinishSignedCalled                 func(referenceHandle int32)
	BigIntToStringCalled                     func(bigIntHandle int32, destinationHandle int32)
	MBufferNewCalled                         func() int32
	MBufferNewFromBytesCalled                func(dataOffset executor.MemPtr, dataLength executor.MemLength) int32
	MBufferGetLengthCalled                   func(mBufferHandle int32) int32
	MBufferGetBytesCalled                    func(mBufferHandle int32, resultOffset executor.MemPtr) int32
	MBufferGetByteSliceCalled                func(sourceHandle int32, startingPosition int32, sliceLength int32, resultOffset executor.MemPtr) int32
	MBufferCopyByteSliceCalled               func(sourceHandle int32, startingPosition int32, sliceLength int32, destinationHandle int32) int32
	MBufferEqCalled                          func(mBufferHandle1 int32, mBufferHandle2 int32) int32
	MBufferSetBytesCalled                    func(mBufferHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32
	MBufferSetByteSliceCalled                func(mBufferHandle int32, startingPosition int32, dataLength executor.MemLength, dataOffset executor.MemPtr) int32
	MBufferAppendCalled                      func(accumulatorHandle int32, dataHandle int32) int32
	MBufferAppendBytesCalled                 func(accumulatorHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32
	MBufferToBigIntUnsignedCalled            func(mBufferHandle int32, bigIntHandle int32) int32
	MBufferToBigIntSignedCalled              func(mBufferHandle int32, bigIntHandle int32) int32
	MBufferFromBigIntUnsignedCalled          func(mBufferHandle int32, bigIntHandle int32) int32
	MBufferFromBigIntSignedCalled            func(mBufferHandle int32, bigIntHandle int32) int32
	MBufferToBigFloatCalled                  func(mBufferHandle int32, bigFloatHandle int32) int32
	MBufferFromBigFloatCalled                func(mBufferHandle int32, bigFloatHandle int32) int32
	MBufferStorageStoreCalled                func(keyHandle int32, sourceHandle int32) int32
	MBufferStorageLoadCalled                 func(keyHandle int32, destinationHandle int32) int32
	MBufferStorageLoadFromAddressCalled      func(addressHandle int32, keyHandle int32, destinationHandle int32)
	MBufferGetArgumentCalled                 func(id int32, destinationHandle int32) int32
	MBufferFinishCalled                      func(sourceHandle int32) int32
	MBufferSetRandomCalled                   func(destinationHandle int32, length int32) int32
	ManagedMapNewCalled                      func() int32
	ManagedMapPutCalled                      func(mMapHandle int32, keyHandle int32, valueHandle int32) int32
	ManagedMapGetCalled                      func(mMapHandle int32, keyHandle int32, outValueHandle int32) int32
	ManagedMapRemoveCalled                   func(mMapHandle int32, keyHandle int32, outValueHandle int32) int32
	ManagedMapContainsCalled                 func(mMapHandle int32, keyHandle int32) int32
	SmallIntGetUnsignedArgumentCalled        func(id int32) int64
	SmallIntGetSignedArgumentCalled          func(id int32) int64
	SmallIntFinishUnsignedCalled             func(value int64)
	SmallIntFinishSignedCalled               func(value int64)
	SmallIntStorageStoreUnsignedCalled       func(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32
	SmallIntStorageStoreSignedCalled         func(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32
	SmallIntStorageLoadUnsignedCalled        func(keyOffset executor.MemPtr, keyLength executor.MemLength) int64
	SmallIntStorageLoadSignedCalled          func(keyOffset executor.MemPtr, keyLength executor.MemLength) int64
	Int64getArgumentCalled                   func(id int32) int64
	Int64finishCalled                        func(value int64)
	Int64storageStoreCalled                  func(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32
	Int64storageLoadCalled                   func(keyOffset executor.MemPtr, keyLength executor.MemLength) int64
	Sha256Called                             func(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32
	ManagedSha256Called                      func(inputHandle int32, outputHandle int32) int32
	Keccak256Called                          func(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32
	ManagedKeccak256Called                   func(inputHandle int32, outputHandle int32) int32
	Ripemd160Called                          func(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32
	ManagedRipemd160Called                   func(inputHandle int32, outputHandle int32) int32
	VerifyBLSCalled                          func(keyOffset executor.MemPtr, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32
	ManagedVerifyBLSCalled                   func(keyHandle int32, messageHandle int32, sigHandle int32) int32
	VerifyEd25519Called                      func(keyOffset executor.MemPtr, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32
	ManagedVerifyEd25519Called               func(keyHandle int32, messageHandle int32, sigHandle int32) int32
	VerifyCustomSecp256k1Called              func(keyOffset executor.MemPtr, keyLength executor.MemLength, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr, hashType int32) int32
	ManagedVerifyCustomSecp256k1Called       func(keyHandle int32, messageHandle int32, sigHandle int32, hashType int32) int32
	VerifySecp256k1Called                    func(keyOffset executor.MemPtr, keyLength executor.MemLength, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32
	ManagedVerifySecp256k1Called             func(keyHandle int32, messageHandle int32, sigHandle int32) int32
	EncodeSecp256k1DerSignatureCalled        func(rOffset executor.MemPtr, rLength executor.MemLength, sOffset executor.MemPtr, sLength executor.MemLength, sigOffset executor.MemPtr) int32
	ManagedEncodeSecp256k1DerSignatureCalled func(rHandle int32, sHandle int32, sigHandle int32) int32
	AddECCalled                              func(xResultHandle int32, yResultHandle int32, ecHandle int32, fstPointXHandle int32, fstPointYHandle int32, sndPointXHandle int32, sndPointYHandle int32)
	DoubleECCalled                           func(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32)
	IsOnCurveECCalled                        func(ecHandle int32, pointXHandle int32, pointYHandle int32) int32
	ScalarBaseMultECCalled                   func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32
	ManagedScalarBaseMultECCalled            func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32
	ScalarMultECCalled                       func(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32
	ManagedScalarMultECCalled                func(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32, dataHandle int32) int32
	MarshalECCalled                          func(xPairHandle int32, yPairHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32
	ManagedMarshalECCalled                   func(xPairHandle int32, yPairHandle int32, ecHandle int32, resultHandle int32) int32
	MarshalCompressedECCalled                func(xPairHandle int32, yPairHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32
	ManagedMarshalCompressedECCalled         func(xPairHandle int32, yPairHandle int32, ecHandle int32, resultHandle int32) int32
	UnmarshalECCalled                        func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32
	ManagedUnmarshalECCalled                 func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32
	UnmarshalCompressedECCalled              func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32
	ManagedUnmarshalCompressedECCalled       func(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32
	GenerateKeyECCalled                      func(xPubKeyHandle int32, yPubKeyHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32
	ManagedGenerateKeyECCalled               func(xPubKeyHandle int32, yPubKeyHandle int32, ecHandle int32, resultHandle int32) int32
	CreateECCalled                           func(dataOffset executor.MemPtr, dataLength executor.MemLength) int32
	ManagedCreateECCalled                    func(dataHandle int32) int32
	GetCurveLengthECCalled                   func(ecHandle int32) int32
	GetPrivKeyByteLengthECCalled             func(ecHandle int32) int32
	EllipticCurveGetValuesCalled             func(ecHandle int32, fieldOrderHandle int32, basePointOrderHandle int32, eqConstantHandle int32, xBasePointHandle int32, yBasePointHandle int32) int32
}

func (V VMHooksStub) GetGasLeft() int64 {
	if V.GetGasLeftCalled != nil {
		return V.GetGasLeftCalled()
	}
	return 0
}

func (V VMHooksStub) GetSCAddress(resultOffset executor.MemPtr) {
	if V.GetSCAddressCalled != nil {
		V.GetSCAddressCalled(resultOffset)
	}
}

func (V VMHooksStub) GetOwnerAddress(resultOffset executor.MemPtr) {
	if V.GetOwnerAddressCalled != nil {
		V.GetOwnerAddressCalled(resultOffset)
	}
}

func (V VMHooksStub) IsSmartContract(addressOffset executor.MemPtr) int32 {
	if V.IsSmartContractCalled != nil {
		return V.IsSmartContractCalled(addressOffset)
	}
	return 0
}

func (V VMHooksStub) SignalError(messageOffset executor.MemPtr, messageLength executor.MemLength) {
	if V.SignalErrorCalled != nil {
		V.SignalErrorCalled(messageOffset, messageLength)
	}
}

func (V VMHooksStub) GetExternalBalance(addressOffset executor.MemPtr, resultOffset executor.MemPtr) {
	if V.GetExternalBalanceCalled != nil {
		V.GetExternalBalanceCalled(addressOffset, resultOffset)
	}
}

func (V VMHooksStub) GetBlockHash(nonce int64, resultOffset executor.MemPtr) int32 {
	if V.GetBlockHashCalled != nil {
		return V.GetBlockHashCalled(nonce, resultOffset)
	}
	return 0
}

func (V VMHooksStub) GetKDABalance(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, resultOffset executor.MemPtr) int32 {
	if V.GetKDABalanceCalled != nil {
		return V.GetKDABalanceCalled(addressOffset, tokenIDOffset, tokenIDLen, nonce, resultOffset)
	}
	return 0
}

func (V VMHooksStub) GetKDANFTNameLength(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64) int32 {
	if V.GetKDANFTNameLengthCalled != nil {
		return V.GetKDANFTNameLengthCalled(addressOffset, tokenIDOffset, tokenIDLen, nonce)
	}
	return 0
}

func (V VMHooksStub) GetKDANFTURILength(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64) int32 {
	if V.GetKDANFTURILengthCalled != nil {
		return V.GetKDANFTURILengthCalled(addressOffset, tokenIDOffset, tokenIDLen, nonce)
	}
	return 0
}

func (V VMHooksStub) GetKDATokenData(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, precisionHandle int32, idOffset executor.MemPtr, nameOffset executor.MemPtr, creatorOffset executor.MemPtr, logoOffset executor.MemPtr, initialSupplyOffset executor.MemPtr, circulatingSupplyOffset executor.MemPtr, maxSupplyOffset executor.MemPtr, mintedOffset executor.MemPtr, burnedOffset executor.MemPtr, royaltiesOffset executor.MemPtr, propertiesOffset executor.MemPtr, attributesOffset executor.MemPtr, rolesOffset executor.MemPtr) int32 {
	if V.GetKDATokenDataCalled != nil {
		return V.GetKDATokenDataCalled(addressOffset, tokenIDOffset, tokenIDLen, nonce, precisionHandle, idOffset, nameOffset, creatorOffset, logoOffset, initialSupplyOffset, circulatingSupplyOffset, maxSupplyOffset, mintedOffset, burnedOffset, royaltiesOffset, propertiesOffset, attributesOffset, rolesOffset)
	}
	return 0
}

func (V VMHooksStub) ValidateTokenIdentifier(tokenIdHandle int32) int32 {
	if V.ValidateTokenIdentifierCalled != nil {
		return V.ValidateTokenIdentifierCalled(tokenIdHandle)
	}
	return 0
}

func (V VMHooksStub) UpgradeContract(destOffset executor.MemPtr, gasLimit int64, valueOffset executor.MemPtr, codeOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, length executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) {
	if V.UpgradeContractCalled != nil {
		V.UpgradeContractCalled(destOffset, gasLimit, valueOffset, codeOffset, codeMetadataOffset, length, numArguments, argumentsLengthOffset, dataOffset)
	}
}

func (V VMHooksStub) UpgradeFromSourceContract(destOffset executor.MemPtr, gasLimit int64, valueOffset executor.MemPtr, sourceContractAddressOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) {
	if V.UpgradeFromSourceContractCalled != nil {
		V.UpgradeFromSourceContractCalled(destOffset, gasLimit, valueOffset, sourceContractAddressOffset, codeMetadataOffset, numArguments, argumentsLengthOffset, dataOffset)
	}
}

func (V VMHooksStub) DeleteContract(destOffset executor.MemPtr, gasLimit int64, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) {
	if V.DeleteContractCalled != nil {
		V.DeleteContractCalled(destOffset, gasLimit, numArguments, argumentsLengthOffset, dataOffset)
	}
}

func (V VMHooksStub) GetArgumentLength(id int32) int32 {
	if V.GetArgumentLengthCalled != nil {
		return V.GetArgumentLengthCalled(id)
	}
	return 0
}

func (V VMHooksStub) GetArgument(id int32, argOffset executor.MemPtr) int32 {
	if V.GetArgumentCalled != nil {
		return V.GetArgumentCalled(id, argOffset)
	}
	return 0
}

func (V VMHooksStub) GetFunction(functionOffset executor.MemPtr) int32 {
	if V.GetFunctionCalled != nil {
		return V.GetFunctionCalled(functionOffset)
	}
	return 0
}

func (V VMHooksStub) GetNumArguments() int32 {
	if V.GetNumArgumentsCalled != nil {
		return V.GetNumArgumentsCalled()
	}
	return 0
}

func (V VMHooksStub) StorageStore(keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	if V.StorageStoreCalled != nil {
		return V.StorageStoreCalled(keyOffset, keyLength, dataOffset, dataLength)
	}
	return 0
}

func (V VMHooksStub) StorageLoadLength(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	if V.StorageLoadLengthCalled != nil {
		return V.StorageLoadLengthCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) StorageLoadFromAddress(addressOffset executor.MemPtr, keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32 {
	if V.StorageLoadFromAddressCalled != nil {
		return V.StorageLoadFromAddressCalled(addressOffset, keyOffset, keyLength, dataOffset)
	}
	return 0
}

func (V VMHooksStub) StorageLoad(keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32 {
	if V.StorageLoadCalled != nil {
		return V.StorageLoadCalled(keyOffset, keyLength, dataOffset)
	}
	return 0
}

func (V VMHooksStub) SetStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength, lockTimestamp int64) int32 {
	if V.SetStorageLockCalled != nil {
		return V.SetStorageLockCalled(keyOffset, keyLength, lockTimestamp)
	}
	return 0
}

func (V VMHooksStub) GetStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength) int64 {
	if V.GetStorageLockCalled != nil {
		return V.GetStorageLockCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) IsStorageLocked(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	if V.IsStorageLockedCalled != nil {
		return V.IsStorageLockedCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) ClearStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	if V.ClearStorageLockCalled != nil {
		return V.ClearStorageLockCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) GetCaller(resultOffset executor.MemPtr) {
	if V.GetCallerCalled != nil {
		V.GetCallerCalled(resultOffset)
	}
}

func (V VMHooksStub) CheckNoPayment() {
	if V.CheckNoPaymentCalled != nil {
		V.CheckNoPaymentCalled()
	}
}

func (V VMHooksStub) GetCallValue(resultOffset executor.MemPtr) int32 {
	if V.GetCallValueCalled != nil {
		return V.GetCallValueCalled(resultOffset)
	}
	return 0
}

func (V VMHooksStub) GetKDAValue(resultOffset executor.MemPtr) int32 {
	if V.GetKDAValueCalled != nil {
		return V.GetKDAValueCalled(resultOffset)
	}
	return 0
}

func (V VMHooksStub) GetKDAValueByIndex(resultOffset executor.MemPtr, index int32) int32 {
	if V.GetKDAValueByIndexCalled != nil {
		return V.GetKDAValueByIndexCalled(resultOffset, index)
	}
	return 0
}

func (V VMHooksStub) GetKDATokenName(resultOffset executor.MemPtr) int32 {
	if V.GetKDATokenNameCalled != nil {
		return V.GetKDATokenNameCalled(resultOffset)
	}
	return 0
}

func (V VMHooksStub) GetKDATokenNameByIndex(resultOffset executor.MemPtr, index int32) int32 {
	if V.GetKDATokenNameByIndexCalled != nil {
		return V.GetKDATokenNameByIndexCalled(resultOffset, index)
	}
	return 0
}

func (V VMHooksStub) GetKDATokenNonce() int64 {
	if V.GetKDATokenNonceCalled != nil {
		return V.GetKDATokenNonceCalled()
	}
	return 0
}

func (V VMHooksStub) GetKDATokenNonceByIndex(index int32) int64 {
	if V.GetKDATokenNonceByIndexCalled != nil {
		return V.GetKDATokenNonceByIndexCalled(index)
	}
	return 0
}

func (V VMHooksStub) GetKDATokenType() int32 {
	if V.GetKDATokenTypeCalled != nil {
		return V.GetKDATokenTypeCalled()
	}
	return 0
}

func (V VMHooksStub) GetKDATokenTypeByIndex(index int32) int32 {
	if V.GetKDATokenTypeByIndexCalled != nil {
		return V.GetKDATokenTypeByIndexCalled(index)
	}
	return 0
}

func (V VMHooksStub) GetNumKDATransfers() int32 {
	if V.GetNumKDATransfersCalled != nil {
		return V.GetNumKDATransfersCalled()
	}
	return 0
}

func (V VMHooksStub) GetCallValueByTokenName(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr, tokenNameLength executor.MemLength) int32 {
	if V.GetCallValueByTokenNameCalled != nil {
		return V.GetCallValueByTokenNameCalled(callValueOffset, tokenNameOffset, tokenNameLength)
	}
	return 0
}

func (V VMHooksStub) GetCallValueTokenName(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr) int32 {
	if V.GetCallValueTokenNameCalled != nil {
		return V.GetCallValueTokenNameCalled(callValueOffset, tokenNameOffset)
	}
	return 0
}

func (V VMHooksStub) GetCallValueTokenNameByIndex(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr, index int32) int32 {
	if V.GetCallValueTokenNameByIndexCalled != nil {
		return V.GetCallValueTokenNameByIndexCalled(callValueOffset, tokenNameOffset, index)
	}
	return 0
}

func (V VMHooksStub) WriteLog(dataPointer executor.MemPtr, dataLength executor.MemLength, topicPtr executor.MemPtr, numTopics int32) {
	if V.WriteLogCalled != nil {
		V.WriteLogCalled(dataPointer, dataLength, topicPtr, numTopics)
	}
}

func (V VMHooksStub) WriteEventLog(numTopics int32, topicLengthsOffset executor.MemPtr, topicOffset executor.MemPtr, dataOffset executor.MemPtr, dataLength executor.MemLength) {
	if V.WriteEventLogCalled != nil {
		V.WriteEventLogCalled(numTopics, topicLengthsOffset, topicOffset, dataOffset, dataLength)
	}
}

func (V VMHooksStub) GetBlockTimestamp() int64 {
	if V.GetBlockTimestampCalled != nil {
		return V.GetBlockTimestampCalled()
	}
	return 0
}

func (V VMHooksStub) GetBlockNonce() int64 {
	if V.GetBlockNonceCalled != nil {
		return V.GetBlockNonceCalled()
	}
	return 0
}

func (V VMHooksStub) GetBlockRound() int64 {
	if V.GetBlockRoundCalled != nil {
		return V.GetBlockRoundCalled()
	}
	return 0
}

func (V VMHooksStub) GetBlockEpoch() int64 {
	if V.GetBlockEpochCalled != nil {
		return V.GetBlockEpochCalled()
	}
	return 0
}

func (V VMHooksStub) GetBlockRandomSeed(pointer executor.MemPtr) {
	if V.GetBlockRandomSeedCalled != nil {
		V.GetBlockRandomSeedCalled(pointer)
	}
}

func (V VMHooksStub) GetStateRootHash(pointer executor.MemPtr) {
	if V.GetStateRootHashCalled != nil {
		V.GetStateRootHashCalled(pointer)
	}
}

func (V VMHooksStub) GetPrevBlockTimestamp() int64 {
	if V.GetPrevBlockTimestampCalled != nil {
		return V.GetPrevBlockTimestampCalled()
	}
	return 0
}

func (V VMHooksStub) GetPrevBlockNonce() int64 {
	if V.GetPrevBlockNonceCalled != nil {
		return V.GetPrevBlockNonceCalled()
	}
	return 0
}

func (V VMHooksStub) GetPrevBlockRound() int64 {
	if V.GetPrevBlockRoundCalled != nil {
		return V.GetPrevBlockRoundCalled()
	}
	return 0
}

func (V VMHooksStub) GetPrevBlockEpoch() int64 {
	if V.GetPrevBlockEpochCalled != nil {
		return V.GetPrevBlockEpochCalled()
	}
	return 0
}

func (V VMHooksStub) GetPrevBlockRandomSeed(pointer executor.MemPtr) {
	if V.GetPrevBlockRandomSeedCalled != nil {
		V.GetPrevBlockRandomSeedCalled(pointer)
	}
}

func (V VMHooksStub) Finish(pointer executor.MemPtr, length executor.MemLength) {
	if V.FinishCalled != nil {
		V.FinishCalled(pointer, length)
	}
}

func (V VMHooksStub) ExecuteOnSameContext(gasLimit int64, addressOffset executor.MemPtr, valueOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32 {
	if V.ExecuteOnSameContextCalled != nil {
		return V.ExecuteOnSameContextCalled(gasLimit, addressOffset, valueOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	}
	return 0
}

func (V VMHooksStub) ExecuteOnDestContext(gasLimit int64, addressOffset executor.MemPtr, valueOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32 {
	if V.ExecuteOnDestContextCalled != nil {
		return V.ExecuteOnDestContextCalled(gasLimit, addressOffset, valueOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	}
	return 0
}

func (V VMHooksStub) ExecuteReadOnly(gasLimit int64, addressOffset executor.MemPtr, functionOffset executor.MemPtr, functionLength executor.MemLength, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32 {
	if V.ExecuteReadOnlyCalled != nil {
		return V.ExecuteReadOnlyCalled(gasLimit, addressOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	}
	return 0
}

func (V VMHooksStub) CreateContract(gasLimit int64, valueOffset executor.MemPtr, codeOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32 {
	if V.CreateContractCalled != nil {
		return V.CreateContractCalled(gasLimit, valueOffset, codeOffset, codeMetadataOffset, length, resultOffset, numArguments, argumentsLengthOffset, dataOffset)
	}
	return 0
}

func (V VMHooksStub) DeployFromSourceContract(gasLimit int64, valueOffset executor.MemPtr, sourceContractAddressOffset executor.MemPtr, codeMetadataOffset executor.MemPtr, resultAddressOffset executor.MemPtr, numArguments int32, argumentsLengthOffset executor.MemPtr, dataOffset executor.MemPtr) int32 {
	if V.DeployFromSourceContractCalled != nil {
		return V.DeployFromSourceContractCalled(gasLimit, valueOffset, sourceContractAddressOffset, codeMetadataOffset, resultAddressOffset, numArguments, argumentsLengthOffset, dataOffset)
	}
	return 0
}

func (V VMHooksStub) GetNumReturnData() int32 {
	if V.GetNumReturnDataCalled != nil {
		return V.GetNumReturnDataCalled()
	}
	return 0
}

func (V VMHooksStub) GetReturnDataSize(resultID int32) int32 {
	if V.GetReturnDataSizeCalled != nil {
		return V.GetReturnDataSizeCalled(resultID)
	}
	return 0
}

func (V VMHooksStub) GetReturnData(resultID int32, dataOffset executor.MemPtr) int32 {
	if V.GetReturnDataCalled != nil {
		return V.GetReturnDataCalled(resultID, dataOffset)
	}
	return 0
}

func (V VMHooksStub) CleanReturnData() {
	if V.CleanReturnDataCalled != nil {
		V.CleanReturnDataCalled()
	}
}

func (V VMHooksStub) DeleteFromReturnData(resultID int32) {
	if V.DeleteFromReturnDataCalled != nil {
		V.DeleteFromReturnDataCalled(resultID)
	}
}

func (V VMHooksStub) GetOriginalTxHash(dataOffset executor.MemPtr) {
	if V.GetOriginalTxHashCalled != nil {
		V.GetOriginalTxHashCalled(dataOffset)
	}
}

func (V VMHooksStub) GetCurrentTxHash(dataOffset executor.MemPtr) {
	if V.GetCurrentTxHashCalled != nil {
		V.GetCurrentTxHashCalled(dataOffset)
	}
}

func (V VMHooksStub) ManagedSCAddress(destinationHandle int32) {
	if V.ManagedSCAddressCalled != nil {
		V.ManagedSCAddressCalled(destinationHandle)
	}
}

func (V VMHooksStub) ManagedOwnerAddress(destinationHandle int32) {
	if V.ManagedOwnerAddressCalled != nil {
		V.ManagedOwnerAddressCalled(destinationHandle)
	}
}

func (V VMHooksStub) ManagedCaller(destinationHandle int32) {
	if V.ManagedCallerCalled != nil {
		V.ManagedCallerCalled(destinationHandle)
	}
}

func (V VMHooksStub) ManagedSignalError(errHandle int32) {
	if V.ManagedSignalErrorCalled != nil {
		V.ManagedSignalErrorCalled(errHandle)
	}
}

func (V VMHooksStub) ManagedWriteLog(topicsHandle int32, dataHandle int32) {
	if V.ManagedWriteLogCalled != nil {
		V.ManagedWriteLogCalled(topicsHandle, dataHandle)
	}
}

func (V VMHooksStub) ManagedGetOriginalTxHash(resultHandle int32) {
	if V.ManagedGetOriginalTxHashCalled != nil {
		V.ManagedGetOriginalTxHashCalled(resultHandle)
	}
}

func (V VMHooksStub) ManagedGetStateRootHash(resultHandle int32) {
	if V.ManagedGetStateRootHashCalled != nil {
		V.ManagedGetStateRootHashCalled(resultHandle)
	}
}

func (V VMHooksStub) ManagedGetBlockRandomSeed(resultHandle int32) {
	if V.ManagedGetBlockRandomSeedCalled != nil {
		V.ManagedGetBlockRandomSeedCalled(resultHandle)
	}
}

func (V VMHooksStub) ManagedGetPrevBlockRandomSeed(resultHandle int32) {
	if V.ManagedGetPrevBlockRandomSeedCalled != nil {
		V.ManagedGetPrevBlockRandomSeedCalled(resultHandle)
	}
}

func (V VMHooksStub) ManagedGetReturnData(resultID int32, resultHandle int32) {
	if V.ManagedGetReturnDataCalled != nil {
		V.ManagedGetReturnDataCalled(resultID, resultHandle)
	}
}

func (V VMHooksStub) ManagedGetKDACallValue(kdaCallValueHandle int32, kdaHandle int32) {
	if V.ManagedGetKDACallValueCalled != nil {
		V.ManagedGetKDACallValueCalled(kdaCallValueHandle, kdaHandle)
	}
}

func (V VMHooksStub) ManagedGetMultiKDACallValue(multiCallValueHandle int32) {
	if V.ManagedGetMultiKDACallValueCalled != nil {
		V.ManagedGetMultiKDACallValueCalled(multiCallValueHandle)
	}
}

func (V VMHooksStub) ManagedGetBackTransfers(kdaTransfersValueHandle int32, callValueHandle int32) {
	if V.ManagedGetBackTransfersCalled != nil {
		V.ManagedGetBackTransfersCalled(kdaTransfersValueHandle, callValueHandle)
	}
}

func (V VMHooksStub) ManagedGetKDABalance(addressHandle int32, tokenIDHandle int32, nonce int64, valueHandle int32) {
	if V.ManagedGetKDABalanceCalled != nil {
		V.ManagedGetKDABalanceCalled(addressHandle, tokenIDHandle, nonce, valueHandle)
	}
}

func (V VMHooksStub) ManagedGetUserKDA(addressHandle int32, tickerHandle int32, nonce int64, balanceHandle int32, frozenHandle int32, lastClaimHandle int32, bucketsHandle int32, mimeHandle int32, metadataHandle int32) {
	if V.ManagedGetUserKDACalled != nil {
		V.ManagedGetUserKDACalled(addressHandle, tickerHandle, nonce, balanceHandle, frozenHandle, lastClaimHandle, bucketsHandle, mimeHandle, metadataHandle)
	}
}

func (V VMHooksStub) ManagedGetKDATokenData(addressHandle int32, tickerHandle int32, nonce int64, precisionHandle int32, idHandle int32, nameHandle int32, creatorHandle int32, adminHandle int32, logoHandle int32, urisHandle int32, initialSupplyHandle int32, circulatingSupplyHandle int32, maxSupplyHandle int32, mintedHandle int32, burnedHandle int32, royaltiesHandle int32, propertiesHandle int32, attributesHandle int32, rolesHandle int32, issueDateHandle int32) {
	if V.ManagedGetKDATokenDataCalled != nil {
		V.ManagedGetKDATokenDataCalled(addressHandle, tickerHandle, nonce, precisionHandle, idHandle, nameHandle, creatorHandle, adminHandle, logoHandle, urisHandle, initialSupplyHandle, circulatingSupplyHandle, maxSupplyHandle, mintedHandle, burnedHandle, royaltiesHandle, propertiesHandle, attributesHandle, rolesHandle, issueDateHandle)
	}
}

func (V VMHooksStub) ManagedGetKDARoles(tickerHandle int32, rolesHandle int32) {
	if V.ManagedGetKDARolesCalled != nil {
		V.ManagedGetKDARolesCalled(tickerHandle, rolesHandle)
	}
}

func (V VMHooksStub) ManagedUpgradeFromSourceContract(destHandle int32, gas int64, valueHandle int32, addressHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultHandle int32) {
	if V.ManagedUpgradeFromSourceContractCalled != nil {
		V.ManagedUpgradeFromSourceContractCalled(destHandle, gas, valueHandle, addressHandle, codeMetadataHandle, argumentsHandle, resultHandle)
	}
}

func (V VMHooksStub) ManagedUpgradeContract(destHandle int32, gas int64, valueHandle int32, codeHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultHandle int32) {
	if V.ManagedUpgradeContractCalled != nil {
		V.ManagedUpgradeContractCalled(destHandle, gas, valueHandle, codeHandle, codeMetadataHandle, argumentsHandle, resultHandle)
	}
}

func (V VMHooksStub) ManagedDeleteContract(destHandle int32, gasLimit int64, argumentsHandle int32) {
	if V.ManagedDeleteContractCalled != nil {
		V.ManagedDeleteContractCalled(destHandle, gasLimit, argumentsHandle)
	}
}

func (V VMHooksStub) ManagedDeployFromSourceContract(gas int64, valueHandle int32, addressHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultAddressHandle int32, resultHandle int32) int32 {
	if V.ManagedDeployFromSourceContractCalled != nil {
		return V.ManagedDeployFromSourceContractCalled(gas, valueHandle, addressHandle, codeMetadataHandle, argumentsHandle, resultAddressHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedCreateContract(gas int64, valueHandle int32, codeHandle int32, codeMetadataHandle int32, argumentsHandle int32, resultAddressHandle int32, resultHandle int32) int32 {
	if V.ManagedCreateContractCalled != nil {
		return V.ManagedCreateContractCalled(gas, valueHandle, codeHandle, codeMetadataHandle, argumentsHandle, resultAddressHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedExecuteReadOnly(gas int64, addressHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32 {
	if V.ManagedExecuteReadOnlyCalled != nil {
		return V.ManagedExecuteReadOnlyCalled(gas, addressHandle, functionHandle, argumentsHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedExecuteOnSameContext(gas int64, addressHandle int32, valueHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32 {
	if V.ManagedExecuteOnSameContextCalled != nil {
		return V.ManagedExecuteOnSameContextCalled(gas, addressHandle, valueHandle, functionHandle, argumentsHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedExecuteOnDestContext(gas int64, addressHandle int32, valueHandle int32, functionHandle int32, argumentsHandle int32, resultHandle int32) int32 {
	if V.ManagedExecuteOnDestContextCalled != nil {
		return V.ManagedExecuteOnDestContextCalled(gas, addressHandle, valueHandle, functionHandle, argumentsHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedMultiTransferKDANFTExecute(dstHandle int32, tokenTransfersHandle int32, gasLimit int64, functionHandle int32, argumentsHandle int32) int32 {
	if V.ManagedMultiTransferKDANFTExecuteCalled != nil {
		return V.ManagedMultiTransferKDANFTExecuteCalled(dstHandle, tokenTransfersHandle, gasLimit, functionHandle, argumentsHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedBufferToHex(sourceHandle int32, destHandle int32) {
	if V.ManagedBufferToHexCalled != nil {
		V.ManagedBufferToHexCalled(sourceHandle, destHandle)
	}
}

func (V VMHooksStub) ManagedGetCodeMetadata(addressHandle int32, responseHandle int32) {
	if V.ManagedGetCodeMetadataCalled != nil {
		V.ManagedGetCodeMetadataCalled(addressHandle, responseHandle)
	}
}

func (V VMHooksStub) ManagedIsBuiltinFunction(functionNameHandle int32) int32 {
	if V.ManagedIsBuiltinFunctionCalled != nil {
		return V.ManagedIsBuiltinFunctionCalled(functionNameHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedGetSftMetadata(tickerHandle int32, nonce int64, dataHandle int32) {
	if V.ManagedGetSftMetadataCalled != nil {
		V.ManagedGetSftMetadataCalled(tickerHandle, nonce, dataHandle)
	}
}

func (V VMHooksStub) ManagedAccHasPerm(ops int64, sourceAccAddr int32, targetAccAddr int32) int32 {
	if V.ManagedAccHasPermCalled != nil {
		return V.ManagedAccHasPermCalled(ops, sourceAccAddr, targetAccAddr)
	}
	return 0
}

func (V VMHooksStub) BigFloatNewFromParts(integralPart int32, fractionalPart int32, exponent int32) int32 {
	if V.BigFloatNewFromPartsCalled != nil {
		return V.BigFloatNewFromPartsCalled(integralPart, fractionalPart, exponent)
	}
	return 0
}

func (V VMHooksStub) BigFloatNewFromFrac(numerator int64, denominator int64) int32 {
	if V.BigFloatNewFromFracCalled != nil {
		return V.BigFloatNewFromFracCalled(numerator, denominator)
	}
	return 0
}

func (V VMHooksStub) BigFloatNewFromSci(significand int64, exponent int64) int32 {
	if V.BigFloatNewFromSciCalled != nil {
		return V.BigFloatNewFromSciCalled(significand, exponent)
	}
	return 0
}

func (V VMHooksStub) BigFloatAdd(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigFloatAddCalled != nil {
		V.BigFloatAddCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigFloatSub(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigFloatSubCalled != nil {
		V.BigFloatSubCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigFloatMul(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigFloatMulCalled != nil {
		V.BigFloatMulCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigFloatDiv(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigFloatDivCalled != nil {
		V.BigFloatDivCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigFloatNeg(destinationHandle int32, opHandle int32) {
	if V.BigFloatNegCalled != nil {
		V.BigFloatNegCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatClone(destinationHandle int32, opHandle int32) {
	if V.BigFloatCloneCalled != nil {
		V.BigFloatCloneCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatCmp(op1Handle int32, op2Handle int32) int32 {
	if V.BigFloatCmpCalled != nil {
		return V.BigFloatCmpCalled(op1Handle, op2Handle)
	}
	return 0
}

func (V VMHooksStub) BigFloatAbs(destinationHandle int32, opHandle int32) {
	if V.BigFloatAbsCalled != nil {
		V.BigFloatAbsCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatSign(opHandle int32) int32 {
	if V.BigFloatSignCalled != nil {
		return V.BigFloatSignCalled(opHandle)
	}
	return 0
}

func (V VMHooksStub) BigFloatSqrt(destinationHandle int32, opHandle int32) {
	if V.BigFloatSqrtCalled != nil {
		V.BigFloatSqrtCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatPow(destinationHandle int32, opHandle int32, exponent int32) {
	if V.BigFloatPowCalled != nil {
		V.BigFloatPowCalled(destinationHandle, opHandle, exponent)
	}
}

func (V VMHooksStub) BigFloatFloor(destBigIntHandle int32, opHandle int32) {
	if V.BigFloatFloorCalled != nil {
		V.BigFloatFloorCalled(destBigIntHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatCeil(destBigIntHandle int32, opHandle int32) {
	if V.BigFloatCeilCalled != nil {
		V.BigFloatCeilCalled(destBigIntHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatTruncate(destBigIntHandle int32, opHandle int32) {
	if V.BigFloatTruncateCalled != nil {
		V.BigFloatTruncateCalled(destBigIntHandle, opHandle)
	}
}

func (V VMHooksStub) BigFloatSetInt64(destinationHandle int32, value int64) {
	if V.BigFloatSetInt64Called != nil {
		V.BigFloatSetInt64Called(destinationHandle, value)
	}
}

func (V VMHooksStub) BigFloatIsInt(opHandle int32) int32 {
	if V.BigFloatIsIntCalled != nil {
		return V.BigFloatIsIntCalled(opHandle)
	}
	return 0
}

func (V VMHooksStub) BigFloatSetBigInt(destinationHandle int32, bigIntHandle int32) {
	if V.BigFloatSetBigIntCalled != nil {
		V.BigFloatSetBigIntCalled(destinationHandle, bigIntHandle)
	}
}

func (V VMHooksStub) BigFloatGetConstPi(destinationHandle int32) {
	if V.BigFloatGetConstPiCalled != nil {
		V.BigFloatGetConstPiCalled(destinationHandle)
	}
}

func (V VMHooksStub) BigFloatGetConstE(destinationHandle int32) {
	if V.BigFloatGetConstECalled != nil {
		V.BigFloatGetConstECalled(destinationHandle)
	}
}

func (V VMHooksStub) BigIntGetUnsignedArgument(id int32, destinationHandle int32) {
	if V.BigIntGetUnsignedArgumentCalled != nil {
		V.BigIntGetUnsignedArgumentCalled(id, destinationHandle)
	}
}

func (V VMHooksStub) BigIntGetSignedArgument(id int32, destinationHandle int32) {
	if V.BigIntGetSignedArgumentCalled != nil {
		V.BigIntGetSignedArgumentCalled(id, destinationHandle)
	}
}

func (V VMHooksStub) BigIntStorageStoreUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength, sourceHandle int32) int32 {
	if V.BigIntStorageStoreUnsignedCalled != nil {
		return V.BigIntStorageStoreUnsignedCalled(keyOffset, keyLength, sourceHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntStorageLoadUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength, destinationHandle int32) int32 {
	if V.BigIntStorageLoadUnsignedCalled != nil {
		return V.BigIntStorageLoadUnsignedCalled(keyOffset, keyLength, destinationHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntGetCallValue(destinationHandle int32) {
	if V.BigIntGetCallValueCalled != nil {
		V.BigIntGetCallValueCalled(destinationHandle)
	}
}

func (V VMHooksStub) BigIntGetKDACallValue(destination int32) {
	if V.BigIntGetKDACallValueCalled != nil {
		V.BigIntGetKDACallValueCalled(destination)
	}
}

func (V VMHooksStub) BigIntGetKDACallValueByIndex(destinationHandle int32, index int32) {
	if V.BigIntGetKDACallValueByIndexCalled != nil {
		V.BigIntGetKDACallValueByIndexCalled(destinationHandle, index)
	}
}

func (V VMHooksStub) BigIntGetExternalBalance(addressOffset executor.MemPtr, result int32) {
	if V.BigIntGetExternalBalanceCalled != nil {
		V.BigIntGetExternalBalanceCalled(addressOffset, result)
	}
}

func (V VMHooksStub) BigIntGetKDAExternalBalance(addressOffset executor.MemPtr, tokenIDOffset executor.MemPtr, tokenIDLen executor.MemLength, nonce int64, resultHandle int32) {
	if V.BigIntGetKDAExternalBalanceCalled != nil {
		V.BigIntGetKDAExternalBalanceCalled(addressOffset, tokenIDOffset, tokenIDLen, nonce, resultHandle)
	}
}

func (V VMHooksStub) BigIntNew(smallValue int64) int32 {
	if V.BigIntNewCalled != nil {
		return V.BigIntNewCalled(smallValue)
	}
	return 0
}

func (V VMHooksStub) BigIntUnsignedByteLength(referenceHandle int32) int32 {
	if V.BigIntUnsignedByteLengthCalled != nil {
		return V.BigIntUnsignedByteLengthCalled(referenceHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntSignedByteLength(referenceHandle int32) int32 {
	if V.BigIntSignedByteLengthCalled != nil {
		return V.BigIntSignedByteLengthCalled(referenceHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntGetUnsignedBytes(referenceHandle int32, byteOffset executor.MemPtr) int32 {
	if V.BigIntGetUnsignedBytesCalled != nil {
		return V.BigIntGetUnsignedBytesCalled(referenceHandle, byteOffset)
	}
	return 0
}

func (V VMHooksStub) BigIntGetSignedBytes(referenceHandle int32, byteOffset executor.MemPtr) int32 {
	if V.BigIntGetSignedBytesCalled != nil {
		return V.BigIntGetSignedBytesCalled(referenceHandle, byteOffset)
	}
	return 0
}

func (V VMHooksStub) BigIntSetUnsignedBytes(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength) {
	if V.BigIntSetUnsignedBytesCalled != nil {
		V.BigIntSetUnsignedBytesCalled(destinationHandle, byteOffset, byteLength)
	}
}

func (V VMHooksStub) BigIntSetSignedBytes(destinationHandle int32, byteOffset executor.MemPtr, byteLength executor.MemLength) {
	if V.BigIntSetSignedBytesCalled != nil {
		V.BigIntSetSignedBytesCalled(destinationHandle, byteOffset, byteLength)
	}
}

func (V VMHooksStub) BigIntIsInt64(destinationHandle int32) int32 {
	if V.BigIntIsInt64Called != nil {
		return V.BigIntIsInt64Called(destinationHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntGetInt64(destinationHandle int32) int64 {
	if V.BigIntGetInt64Called != nil {
		return V.BigIntGetInt64Called(destinationHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntSetInt64(destinationHandle int32, value int64) {
	if V.BigIntSetInt64Called != nil {
		V.BigIntSetInt64Called(destinationHandle, value)
	}
}

func (V VMHooksStub) BigIntAdd(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntAddCalled != nil {
		V.BigIntAddCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntSub(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntSubCalled != nil {
		V.BigIntSubCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntMul(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntMulCalled != nil {
		V.BigIntMulCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntTDiv(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntTDivCalled != nil {
		V.BigIntTDivCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntTMod(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntTModCalled != nil {
		V.BigIntTModCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntEDiv(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntEDivCalled != nil {
		V.BigIntEDivCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntEMod(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntEModCalled != nil {
		V.BigIntEModCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntSqrt(destinationHandle int32, opHandle int32) {
	if V.BigIntSqrtCalled != nil {
		V.BigIntSqrtCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigIntPow(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntPowCalled != nil {
		V.BigIntPowCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntLog2(op1Handle int32) int32 {
	if V.BigIntLog2Called != nil {
		return V.BigIntLog2Called(op1Handle)
	}
	return 0
}

func (V VMHooksStub) BigIntAbs(destinationHandle int32, opHandle int32) {
	if V.BigIntAbsCalled != nil {
		V.BigIntAbsCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigIntNeg(destinationHandle int32, opHandle int32) {
	if V.BigIntNegCalled != nil {
		V.BigIntNegCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigIntSign(opHandle int32) int32 {
	if V.BigIntSignCalled != nil {
		return V.BigIntSignCalled(opHandle)
	}
	return 0
}

func (V VMHooksStub) BigIntCmp(op1Handle int32, op2Handle int32) int32 {
	if V.BigIntCmpCalled != nil {
		return V.BigIntCmpCalled(op1Handle, op2Handle)
	}
	return 0
}

func (V VMHooksStub) BigIntNot(destinationHandle int32, opHandle int32) {
	if V.BigIntNotCalled != nil {
		V.BigIntNotCalled(destinationHandle, opHandle)
	}
}

func (V VMHooksStub) BigIntAnd(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntAndCalled != nil {
		V.BigIntAndCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntOr(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntOrCalled != nil {
		V.BigIntOrCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntXor(destinationHandle int32, op1Handle int32, op2Handle int32) {
	if V.BigIntXorCalled != nil {
		V.BigIntXorCalled(destinationHandle, op1Handle, op2Handle)
	}
}

func (V VMHooksStub) BigIntShr(destinationHandle int32, opHandle int32, bits int32) {
	if V.BigIntShrCalled != nil {
		V.BigIntShrCalled(destinationHandle, opHandle, bits)
	}
}

func (V VMHooksStub) BigIntShl(destinationHandle int32, opHandle int32, bits int32) {
	if V.BigIntShlCalled != nil {
		V.BigIntShlCalled(destinationHandle, opHandle, bits)
	}
}

func (V VMHooksStub) BigIntFinishUnsigned(referenceHandle int32) {
	if V.BigIntFinishUnsignedCalled != nil {
		V.BigIntFinishUnsignedCalled(referenceHandle)
	}
}

func (V VMHooksStub) BigIntFinishSigned(referenceHandle int32) {
	if V.BigIntFinishSignedCalled != nil {
		V.BigIntFinishSignedCalled(referenceHandle)
	}
}

func (V VMHooksStub) BigIntToString(bigIntHandle int32, destinationHandle int32) {
	if V.BigIntToStringCalled != nil {
		V.BigIntToStringCalled(bigIntHandle, destinationHandle)
	}
}

func (V VMHooksStub) MBufferNew() int32 {
	if V.MBufferNewCalled != nil {
		return V.MBufferNewCalled()
	}
	return 0
}

func (V VMHooksStub) MBufferNewFromBytes(dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	if V.MBufferNewFromBytesCalled != nil {
		return V.MBufferNewFromBytesCalled(dataOffset, dataLength)
	}
	return 0
}

func (V VMHooksStub) MBufferGetLength(mBufferHandle int32) int32 {
	if V.MBufferGetLengthCalled != nil {
		return V.MBufferGetLengthCalled(mBufferHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferGetBytes(mBufferHandle int32, resultOffset executor.MemPtr) int32 {
	if V.MBufferGetBytesCalled != nil {
		return V.MBufferGetBytesCalled(mBufferHandle, resultOffset)
	}
	return 0
}

func (V VMHooksStub) MBufferGetByteSlice(sourceHandle int32, startingPosition int32, sliceLength int32, resultOffset executor.MemPtr) int32 {
	if V.MBufferGetByteSliceCalled != nil {
		return V.MBufferGetByteSliceCalled(sourceHandle, startingPosition, sliceLength, resultOffset)
	}
	return 0
}

func (V VMHooksStub) MBufferCopyByteSlice(sourceHandle int32, startingPosition int32, sliceLength int32, destinationHandle int32) int32 {
	if V.MBufferCopyByteSliceCalled != nil {
		return V.MBufferCopyByteSliceCalled(sourceHandle, startingPosition, sliceLength, destinationHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferEq(mBufferHandle1 int32, mBufferHandle2 int32) int32 {
	if V.MBufferEqCalled != nil {
		return V.MBufferEqCalled(mBufferHandle1, mBufferHandle2)
	}
	return 0
}

func (V VMHooksStub) MBufferSetBytes(mBufferHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	if V.MBufferSetBytesCalled != nil {
		return V.MBufferSetBytesCalled(mBufferHandle, dataOffset, dataLength)
	}
	return 0
}

func (V VMHooksStub) MBufferSetByteSlice(mBufferHandle int32, startingPosition int32, dataLength executor.MemLength, dataOffset executor.MemPtr) int32 {
	if V.MBufferSetByteSliceCalled != nil {
		return V.MBufferSetByteSliceCalled(mBufferHandle, startingPosition, dataLength, dataOffset)
	}
	return 0
}

func (V VMHooksStub) MBufferAppend(accumulatorHandle int32, dataHandle int32) int32 {
	if V.MBufferAppendCalled != nil {
		return V.MBufferAppendCalled(accumulatorHandle, dataHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferAppendBytes(accumulatorHandle int32, dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	if V.MBufferAppendBytesCalled != nil {
		return V.MBufferAppendBytesCalled(accumulatorHandle, dataOffset, dataLength)
	}
	return 0
}

func (V VMHooksStub) MBufferToBigIntUnsigned(mBufferHandle int32, bigIntHandle int32) int32 {
	if V.MBufferToBigIntUnsignedCalled != nil {
		return V.MBufferToBigIntUnsignedCalled(mBufferHandle, bigIntHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferToBigIntSigned(mBufferHandle int32, bigIntHandle int32) int32 {
	if V.MBufferToBigIntSignedCalled != nil {
		return V.MBufferToBigIntSignedCalled(mBufferHandle, bigIntHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferFromBigIntUnsigned(mBufferHandle int32, bigIntHandle int32) int32 {
	if V.MBufferFromBigIntUnsignedCalled != nil {
		return V.MBufferFromBigIntUnsignedCalled(mBufferHandle, bigIntHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferFromBigIntSigned(mBufferHandle int32, bigIntHandle int32) int32 {
	if V.MBufferFromBigIntSignedCalled != nil {
		return V.MBufferFromBigIntSignedCalled(mBufferHandle, bigIntHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferToBigFloat(mBufferHandle int32, bigFloatHandle int32) int32 {
	if V.MBufferToBigFloatCalled != nil {
		return V.MBufferToBigFloatCalled(mBufferHandle, bigFloatHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferFromBigFloat(mBufferHandle int32, bigFloatHandle int32) int32 {
	if V.MBufferFromBigFloatCalled != nil {
		return V.MBufferFromBigFloatCalled(mBufferHandle, bigFloatHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferStorageStore(keyHandle int32, sourceHandle int32) int32 {
	if V.MBufferStorageStoreCalled != nil {
		return V.MBufferStorageStoreCalled(keyHandle, sourceHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferStorageLoad(keyHandle int32, destinationHandle int32) int32 {
	if V.MBufferStorageLoadCalled != nil {
		return V.MBufferStorageLoadCalled(keyHandle, destinationHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferStorageLoadFromAddress(addressHandle int32, keyHandle int32, destinationHandle int32) {
	if V.MBufferStorageLoadFromAddressCalled != nil {
		V.MBufferStorageLoadFromAddressCalled(addressHandle, keyHandle, destinationHandle)
	}
}

func (V VMHooksStub) MBufferGetArgument(id int32, destinationHandle int32) int32 {
	if V.MBufferGetArgumentCalled != nil {
		return V.MBufferGetArgumentCalled(id, destinationHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferFinish(sourceHandle int32) int32 {
	if V.MBufferFinishCalled != nil {
		return V.MBufferFinishCalled(sourceHandle)
	}
	return 0
}

func (V VMHooksStub) MBufferSetRandom(destinationHandle int32, length int32) int32 {
	if V.MBufferSetRandomCalled != nil {
		return V.MBufferSetRandomCalled(destinationHandle, length)
	}
	return 0
}

func (V VMHooksStub) ManagedMapNew() int32 {
	if V.ManagedMapNewCalled != nil {
		return V.ManagedMapNewCalled()
	}
	return 0
}

func (V VMHooksStub) ManagedMapPut(mMapHandle int32, keyHandle int32, valueHandle int32) int32 {
	if V.ManagedMapPutCalled != nil {
		return V.ManagedMapPutCalled(mMapHandle, keyHandle, valueHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedMapGet(mMapHandle int32, keyHandle int32, outValueHandle int32) int32 {
	if V.ManagedMapGetCalled != nil {
		return V.ManagedMapGetCalled(mMapHandle, keyHandle, outValueHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedMapRemove(mMapHandle int32, keyHandle int32, outValueHandle int32) int32 {
	if V.ManagedMapRemoveCalled != nil {
		return V.ManagedMapRemoveCalled(mMapHandle, keyHandle, outValueHandle)
	}
	return 0
}

func (V VMHooksStub) ManagedMapContains(mMapHandle int32, keyHandle int32) int32 {
	if V.ManagedMapContainsCalled != nil {
		return V.ManagedMapContainsCalled(mMapHandle, keyHandle)
	}
	return 0
}

func (V VMHooksStub) SmallIntGetUnsignedArgument(id int32) int64 {
	if V.SmallIntGetUnsignedArgumentCalled != nil {
		return V.SmallIntGetUnsignedArgumentCalled(id)
	}
	return 0
}

func (V VMHooksStub) SmallIntGetSignedArgument(id int32) int64 {
	if V.SmallIntGetSignedArgumentCalled != nil {
		return V.SmallIntGetSignedArgumentCalled(id)
	}
	return 0
}

func (V VMHooksStub) SmallIntFinishUnsigned(value int64) {
	if V.SmallIntFinishUnsignedCalled != nil {
		V.SmallIntFinishUnsignedCalled(value)
	}
}

func (V VMHooksStub) SmallIntFinishSigned(value int64) {
	if V.SmallIntFinishSignedCalled != nil {
		V.SmallIntFinishSignedCalled(value)
	}
}

func (V VMHooksStub) SmallIntStorageStoreUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32 {
	if V.SmallIntStorageStoreUnsignedCalled != nil {
		return V.SmallIntStorageStoreUnsignedCalled(keyOffset, keyLength, value)
	}
	return 0
}

func (V VMHooksStub) SmallIntStorageStoreSigned(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32 {
	if V.SmallIntStorageStoreSignedCalled != nil {
		return V.SmallIntStorageStoreSignedCalled(keyOffset, keyLength, value)
	}
	return 0
}

func (V VMHooksStub) SmallIntStorageLoadUnsigned(keyOffset executor.MemPtr, keyLength executor.MemLength) int64 {
	if V.SmallIntStorageLoadUnsignedCalled != nil {
		return V.SmallIntStorageLoadUnsignedCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) SmallIntStorageLoadSigned(keyOffset executor.MemPtr, keyLength executor.MemLength) int64 {
	if V.SmallIntStorageLoadSignedCalled != nil {
		return V.SmallIntStorageLoadSignedCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) Int64getArgument(id int32) int64 {
	if V.Int64getArgumentCalled != nil {
		return V.Int64getArgumentCalled(id)
	}
	return 0
}

func (V VMHooksStub) Int64finish(value int64) {
	if V.Int64finishCalled != nil {
		V.Int64finishCalled(value)
	}
}

func (V VMHooksStub) Int64storageStore(keyOffset executor.MemPtr, keyLength executor.MemLength, value int64) int32 {
	if V.Int64storageStoreCalled != nil {
		return V.Int64storageStoreCalled(keyOffset, keyLength, value)
	}
	return 0
}

func (V VMHooksStub) Int64storageLoad(keyOffset executor.MemPtr, keyLength executor.MemLength) int64 {
	if V.Int64storageLoadCalled != nil {
		return V.Int64storageLoadCalled(keyOffset, keyLength)
	}
	return 0
}

func (V VMHooksStub) Sha256(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32 {
	if V.Sha256Called != nil {
		return V.Sha256Called(dataOffset, length, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedSha256(inputHandle int32, outputHandle int32) int32 {
	if V.ManagedSha256Called != nil {
		return V.ManagedSha256Called(inputHandle, outputHandle)
	}
	return 0
}

func (V VMHooksStub) Keccak256(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32 {
	if V.Keccak256Called != nil {
		return V.Keccak256Called(dataOffset, length, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedKeccak256(inputHandle int32, outputHandle int32) int32 {
	if V.ManagedKeccak256Called != nil {
		return V.ManagedKeccak256Called(inputHandle, outputHandle)
	}
	return 0
}

func (V VMHooksStub) Ripemd160(dataOffset executor.MemPtr, length executor.MemLength, resultOffset executor.MemPtr) int32 {
	if V.Ripemd160Called != nil {
		return V.Ripemd160Called(dataOffset, length, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedRipemd160(inputHandle int32, outputHandle int32) int32 {
	if V.ManagedRipemd160Called != nil {
		return V.ManagedRipemd160Called(inputHandle, outputHandle)
	}
	return 0
}

func (V VMHooksStub) VerifyBLS(keyOffset executor.MemPtr, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32 {
	if V.VerifyBLSCalled != nil {
		return V.VerifyBLSCalled(keyOffset, messageOffset, messageLength, sigOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedVerifyBLS(keyHandle int32, messageHandle int32, sigHandle int32) int32 {
	if V.ManagedVerifyBLSCalled != nil {
		return V.ManagedVerifyBLSCalled(keyHandle, messageHandle, sigHandle)
	}
	return 0
}

func (V VMHooksStub) VerifyEd25519(keyOffset executor.MemPtr, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32 {
	if V.VerifyEd25519Called != nil {
		return V.VerifyEd25519Called(keyOffset, messageOffset, messageLength, sigOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedVerifyEd25519(keyHandle int32, messageHandle int32, sigHandle int32) int32 {
	if V.ManagedVerifyEd25519Called != nil {
		return V.ManagedVerifyEd25519Called(keyHandle, messageHandle, sigHandle)
	}
	return 0
}

func (V VMHooksStub) VerifyCustomSecp256k1(keyOffset executor.MemPtr, keyLength executor.MemLength, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr, hashType int32) int32 {
	if V.VerifyCustomSecp256k1Called != nil {
		return V.VerifyCustomSecp256k1Called(keyOffset, keyLength, messageOffset, messageLength, sigOffset, hashType)
	}
	return 0
}

func (V VMHooksStub) ManagedVerifyCustomSecp256k1(keyHandle int32, messageHandle int32, sigHandle int32, hashType int32) int32 {
	if V.ManagedVerifyCustomSecp256k1Called != nil {
		return V.ManagedVerifyCustomSecp256k1Called(keyHandle, messageHandle, sigHandle, hashType)
	}
	return 0
}

func (V VMHooksStub) VerifySecp256k1(keyOffset executor.MemPtr, keyLength executor.MemLength, messageOffset executor.MemPtr, messageLength executor.MemLength, sigOffset executor.MemPtr) int32 {
	if V.VerifySecp256k1Called != nil {
		return V.VerifySecp256k1Called(keyOffset, keyLength, messageOffset, messageLength, sigOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedVerifySecp256k1(keyHandle int32, messageHandle int32, sigHandle int32) int32 {
	if V.ManagedVerifySecp256k1Called != nil {
		return V.ManagedVerifySecp256k1Called(keyHandle, messageHandle, sigHandle)
	}
	return 0
}

func (V VMHooksStub) EncodeSecp256k1DerSignature(rOffset executor.MemPtr, rLength executor.MemLength, sOffset executor.MemPtr, sLength executor.MemLength, sigOffset executor.MemPtr) int32 {
	if V.EncodeSecp256k1DerSignatureCalled != nil {
		return V.EncodeSecp256k1DerSignatureCalled(rOffset, rLength, sOffset, sLength, sigOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedEncodeSecp256k1DerSignature(rHandle int32, sHandle int32, sigHandle int32) int32 {
	if V.ManagedEncodeSecp256k1DerSignatureCalled != nil {
		return V.ManagedEncodeSecp256k1DerSignatureCalled(rHandle, sHandle, sigHandle)
	}
	return 0
}

func (V VMHooksStub) AddEC(xResultHandle int32, yResultHandle int32, ecHandle int32, fstPointXHandle int32, fstPointYHandle int32, sndPointXHandle int32, sndPointYHandle int32) {
	if V.AddECCalled != nil {
		V.AddECCalled(xResultHandle, yResultHandle, ecHandle, fstPointXHandle, fstPointYHandle, sndPointXHandle, sndPointYHandle)
	}
}

func (V VMHooksStub) DoubleEC(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32) {
	if V.DoubleECCalled != nil {
		V.DoubleECCalled(xResultHandle, yResultHandle, ecHandle, pointXHandle, pointYHandle)
	}
}

func (V VMHooksStub) IsOnCurveEC(ecHandle int32, pointXHandle int32, pointYHandle int32) int32 {
	if V.IsOnCurveECCalled != nil {
		return V.IsOnCurveECCalled(ecHandle, pointXHandle, pointYHandle)
	}
	return 0
}

func (V VMHooksStub) ScalarBaseMultEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32 {
	if V.ScalarBaseMultECCalled != nil {
		return V.ScalarBaseMultECCalled(xResultHandle, yResultHandle, ecHandle, dataOffset, length)
	}
	return 0
}

func (V VMHooksStub) ManagedScalarBaseMultEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32 {
	if V.ManagedScalarBaseMultECCalled != nil {
		return V.ManagedScalarBaseMultECCalled(xResultHandle, yResultHandle, ecHandle, dataHandle)
	}
	return 0
}

func (V VMHooksStub) ScalarMultEC(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32 {
	if V.ScalarMultECCalled != nil {
		return V.ScalarMultECCalled(xResultHandle, yResultHandle, ecHandle, pointXHandle, pointYHandle, dataOffset, length)
	}
	return 0
}

func (V VMHooksStub) ManagedScalarMultEC(xResultHandle int32, yResultHandle int32, ecHandle int32, pointXHandle int32, pointYHandle int32, dataHandle int32) int32 {
	if V.ManagedScalarMultECCalled != nil {
		return V.ManagedScalarMultECCalled(xResultHandle, yResultHandle, ecHandle, pointXHandle, pointYHandle, dataHandle)
	}
	return 0
}

func (V VMHooksStub) MarshalEC(xPairHandle int32, yPairHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32 {
	if V.MarshalECCalled != nil {
		return V.MarshalECCalled(xPairHandle, yPairHandle, ecHandle, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedMarshalEC(xPairHandle int32, yPairHandle int32, ecHandle int32, resultHandle int32) int32 {
	if V.ManagedMarshalECCalled != nil {
		return V.ManagedMarshalECCalled(xPairHandle, yPairHandle, ecHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) MarshalCompressedEC(xPairHandle int32, yPairHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32 {
	if V.MarshalCompressedECCalled != nil {
		return V.MarshalCompressedECCalled(xPairHandle, yPairHandle, ecHandle, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedMarshalCompressedEC(xPairHandle int32, yPairHandle int32, ecHandle int32, resultHandle int32) int32 {
	if V.ManagedMarshalCompressedECCalled != nil {
		return V.ManagedMarshalCompressedECCalled(xPairHandle, yPairHandle, ecHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) UnmarshalEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32 {
	if V.UnmarshalECCalled != nil {
		return V.UnmarshalECCalled(xResultHandle, yResultHandle, ecHandle, dataOffset, length)
	}
	return 0
}

func (V VMHooksStub) ManagedUnmarshalEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32 {
	if V.ManagedUnmarshalECCalled != nil {
		return V.ManagedUnmarshalECCalled(xResultHandle, yResultHandle, ecHandle, dataHandle)
	}
	return 0
}

func (V VMHooksStub) UnmarshalCompressedEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataOffset executor.MemPtr, length executor.MemLength) int32 {
	if V.UnmarshalCompressedECCalled != nil {
		return V.UnmarshalCompressedECCalled(xResultHandle, yResultHandle, ecHandle, dataOffset, length)
	}
	return 0
}

func (V VMHooksStub) ManagedUnmarshalCompressedEC(xResultHandle int32, yResultHandle int32, ecHandle int32, dataHandle int32) int32 {
	if V.ManagedUnmarshalCompressedECCalled != nil {
		return V.ManagedUnmarshalCompressedECCalled(xResultHandle, yResultHandle, ecHandle, dataHandle)
	}
	return 0
}

func (V VMHooksStub) GenerateKeyEC(xPubKeyHandle int32, yPubKeyHandle int32, ecHandle int32, resultOffset executor.MemPtr) int32 {
	if V.GenerateKeyECCalled != nil {
		return V.GenerateKeyECCalled(xPubKeyHandle, yPubKeyHandle, ecHandle, resultOffset)
	}
	return 0
}

func (V VMHooksStub) ManagedGenerateKeyEC(xPubKeyHandle int32, yPubKeyHandle int32, ecHandle int32, resultHandle int32) int32 {
	if V.ManagedGenerateKeyECCalled != nil {
		return V.ManagedGenerateKeyECCalled(xPubKeyHandle, yPubKeyHandle, ecHandle, resultHandle)
	}
	return 0
}

func (V VMHooksStub) CreateEC(dataOffset executor.MemPtr, dataLength executor.MemLength) int32 {
	if V.CreateECCalled != nil {
		return V.CreateECCalled(dataOffset, dataLength)
	}
	return 0
}

func (V VMHooksStub) ManagedCreateEC(dataHandle int32) int32 {
	if V.ManagedCreateECCalled != nil {
		return V.ManagedCreateECCalled(dataHandle)
	}
	return 0
}

func (V VMHooksStub) GetCurveLengthEC(ecHandle int32) int32 {
	if V.GetCurveLengthECCalled != nil {
		return V.GetCurveLengthECCalled(ecHandle)
	}
	return 0
}

func (V VMHooksStub) GetPrivKeyByteLengthEC(ecHandle int32) int32 {
	if V.GetPrivKeyByteLengthECCalled != nil {
		return V.GetPrivKeyByteLengthECCalled(ecHandle)
	}
	return 0
}

func (V VMHooksStub) EllipticCurveGetValues(ecHandle int32, fieldOrderHandle int32, basePointOrderHandle int32, eqConstantHandle int32, xBasePointHandle int32, yBasePointHandle int32) int32 {
	if V.EllipticCurveGetValuesCalled != nil {
		return V.EllipticCurveGetValuesCalled(ecHandle, fieldOrderHandle, basePointOrderHandle, eqConstantHandle, xBasePointHandle, yBasePointHandle)
	}
	return 0
}
