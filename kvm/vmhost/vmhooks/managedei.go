package vmhooks

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"slices"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/tools/check"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

const (
	managedSCAddressName                      = "managedSCAddress"
	managedOwnerAddressName                   = "managedOwnerAddress"
	managedCallerName                         = "managedCaller"
	managedSignalErrorName                    = "managedSignalError"
	managedWriteLogName                       = "managedWriteLog"
	managedMultiTransferKDANFTExecuteName     = "managedMultiTransferKDANFTExecute"
	managedExecuteOnDestContextName           = "managedExecuteOnDestContext"
	managedExecuteOnDestContextByCallerName   = "managedExecuteOnDestContextByCaller"
	managedExecuteOnSameContextName           = "managedExecuteOnSameContext"
	managedExecuteReadOnlyName                = "managedExecuteReadOnly"
	managedCreateContractName                 = "managedCreateContract"
	managedDeployFromSourceContractName       = "managedDeployFromSourceContract"
	managedUpgradeContractName                = "managedUpgradeContract"
	managedUpgradeFromSourceContractName      = "managedUpgradeFromSourceContract"
	managedGetKDACallValueName                = "managedGetKDACallValue"
	managedGetMultiKDACallValueName           = "managedGetMultiKDACallValue"
	managedGetMultiKDAWithoutKLVCallValueName = "managedGetMultiKDAWithoutKLVCallValue"
	managedGetBackTransferName                = "managedGetBackTransfer"
	managedGetKDABalanceName                  = "managedGetKDABalance"
	managedGetKDATokenDataName                = "managedGetKDATokenData"
	managedGetUserKDAName                     = "managedGetUserKDA"
	managedGetSftMetadataName                 = "managedGetSftMetadataName"
	managedAccHasPermName                     = "managedAccHasPerm"
	managedGetKDARolesName                    = "managedGetKDARoles"
	managedGetReturnDataName                  = "managedGetReturnData"
	managedGetPrevBlockRandomSeedName         = "managedGetPrevBlockRandomSeed"
	managedGetBlockRandomSeedName             = "managedGetBlockRandomSeed"
	managedGetStateRootHashName               = "managedGetStateRootHash"
	managedGetOriginalTxHashName              = "managedGetOriginalTxHash"
	managedBufferToHexName                    = "managedBufferToHex"
	managedGetCodeMetadataName                = "managedGetCodeMetadata"
	managedIsBuiltinFunction                  = "managedIsBuiltinFunction"
)

// ManagedSCAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedSCAddress(destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetSCAddress
	metering.UseGasAndAddTracedGas(managedSCAddressName, gasToUse)

	scAddress := runtime.GetContextAddress()

	managedType.SetBytes(destinationHandle, scAddress)
}

// ManagedOwnerAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedOwnerAddress(destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	blockchain := context.GetBlockchainContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetOwnerAddress
	metering.UseGasAndAddTracedGas(managedOwnerAddressName, gasToUse)

	owner, err := blockchain.GetOwnerAddress()
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	managedType.SetBytes(destinationHandle, owner)
}

// ManagedCaller VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedCaller(destinationHandle int32) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCaller
	metering.UseGasAndAddTracedGas(managedCallerName, gasToUse)

	caller := runtime.GetVMInput().CallerAddr
	managedType.SetBytes(destinationHandle, caller)
}

// ManagedSignalError VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedSignalError(errHandle int32) {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(managedSignalErrorName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.SignalError
	metering.UseAndTraceGas(gasToUse)

	errBytes, err := managedType.GetBytes(errHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}
	managedType.ConsumeGasForBytes(errBytes)

	gasToUse = metering.GasSchedule().BaseOperationCost.PersistPerByte * uint64(len(errBytes))
	err = metering.UseGasBounded(gasToUse)
	if err != nil {
		_ = context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	runtime.SignalUserError(string(errBytes))
}

// ManagedWriteLog VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedWriteLog(
	topicsHandle int32,
	dataHandle int32,
) {
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()
	metering.StartGasTracing(managedWriteLogName)

	topics, sumOfTopicByteLengths, err := managedType.ReadManagedVecOfManagedBuffers(topicsHandle)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	dataBytes, err := managedType.GetBytes(dataHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}
	managedType.ConsumeGasForBytes(dataBytes)
	dataByteLen := uint64(len(dataBytes))

	gasToUse := metering.GasSchedule().BaseOpsAPICost.Log
	gasForData := math.MulUint64(
		metering.GasSchedule().BaseOperationCost.DataCopyPerByte,
		sumOfTopicByteLengths+dataByteLen)
	gasToUse = math.AddUint64(gasToUse, gasForData)
	metering.UseAndTraceGas(gasToUse)

	output.WriteLog(runtime.GetContextAddress(), topics, [][]byte{dataBytes})
}

// ManagedGetOriginalTxHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetOriginalTxHash(resultHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetOriginalTxHash
	metering.UseGasAndAddTracedGas(managedGetOriginalTxHashName, gasToUse)

	managedType.SetBytes(resultHandle, runtime.GetOriginalTxHash())
}

// ManagedGetStateRootHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetStateRootHash(resultHandle int32) {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetStateRootHash
	metering.UseGasAndAddTracedGas(managedGetStateRootHashName, gasToUse)

	managedType.SetBytes(resultHandle, blockchain.GetStateRootHash())
}

// ManagedGetBlockRandomSeed VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetBlockRandomSeed(resultHandle int32) {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRandomSeed
	metering.UseGasAndAddTracedGas(managedGetBlockRandomSeedName, gasToUse)

	managedType.SetBytes(resultHandle, blockchain.CurrentRandomSeed())
}

// ManagedGetPrevBlockRandomSeed VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetPrevBlockRandomSeed(resultHandle int32) {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRandomSeed
	metering.UseGasAndAddTracedGas(managedGetPrevBlockRandomSeedName, gasToUse)

	managedType.SetBytes(resultHandle, blockchain.LastRandomSeed())
}

// ManagedGetReturnData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetReturnData(resultID int32, resultHandle int32) {
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetReturnData
	metering.UseGasAndAddTracedGas(managedGetReturnDataName, gasToUse)

	returnData := output.ReturnData()
	// #nosec G115
	if resultID >= int32(len(returnData)) || resultID < 0 {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	managedType.SetBytes(resultHandle, returnData[resultID])
}

// ManagedGetKDACallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDACallValue(kdaCallValueHandle int32, kdaHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := context.ComputeGetCallValueGas()
	metering.UseGasAndAddTracedGas(managedGetKDACallValueName, gasToUse)

	tokenID, err := managedType.GetBytes(kdaHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	callValue := runtime.GetVMInput().GetKDACallValue(tokenID)

	value := managedType.GetBigIntOrCreate(kdaCallValueHandle)
	value.Set(big.NewInt(0).Set(callValue))
}

// ManagedGetMultiKDACallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetMultiKDACallValue(multiCallValueHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(managedGetMultiKDACallValueName, gasToUse)

	kdaTransfers := runtime.GetVMInput().KDATransfers
	multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
	managedType.ConsumeGasForBytes(multiCallBytes)

	managedType.SetBytes(multiCallValueHandle, multiCallBytes)
}

// ManagedGetMultiKDAWithoutKLVCallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetMultiKDAWithoutKLVCallValue(multiCallValueHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(managedGetMultiKDAWithoutKLVCallValueName, gasToUse)

	kdaTransfers := runtime.GetVMInput().KDATransfers
	if context.host.ForkController().FixAuditChangesV4() {
		// the loop below deletes in place, which would shuffle the runtime input's backing array
		// it would also leave duplicate elements in the backing array
		kdaTransfers = slices.Clone(kdaTransfers)
	}
	// remove klv transfers if any
	for i := 0; i < len(kdaTransfers); {
		kdaName := kdaTransfers[i].KDATokenName
		if kdaName == nil ||
			len(kdaName) == 0 && context.host.ForkController().FixAuditChangesV4() ||
			bytes.Equal(kdaName, kdautils.KLVIdentifier) {
			// Remove the element by creating a new slice without it
			kdaTransfers = append(kdaTransfers[:i], kdaTransfers[i+1:]...)
		} else {
			i++
		}
	}
	multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
	managedType.ConsumeGasForBytes(multiCallBytes)

	managedType.SetBytes(multiCallValueHandle, multiCallBytes)
}

// ManagedGetBackTransfers VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetBackTransfers(kdaTransfersValueHandle int32, callValueHandle int32) {
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(managedGetBackTransferName, gasToUse)

	kdaTransfers, transferValue := managedType.GetBackTransfers()
	multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
	managedType.ConsumeGasForBytes(multiCallBytes)
	managedType.ConsumeGasForBigIntCopy(transferValue)

	managedType.SetBytes(kdaTransfersValueHandle, multiCallBytes)
	managedType.GetBigIntOrCreate(callValueHandle).Set(transferValue)
}

// ManagedGetKDABalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDABalance(addressHandle int32, tokenIDHandle int32, nonce int64, valueHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	blockchain := context.GetBlockchainContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetExternalBalance
	metering.UseGasAndAddTracedGas(managedGetKDABalanceName, gasToUse)

	address, err := managedType.GetBytes(addressHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}
	tokenID, err := managedType.GetBytes(tokenIDHandle)
	if err != nil {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	_, kdaToken, err := blockchain.GetKDAToken(address, tokenID, uint64(nonce)) // #nosec G115
	if err != nil {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	value := managedType.GetBigIntOrCreate(valueHandle)
	value.Set(big.NewInt(kdaToken.Balance))
}

// ManagedGetUserKDA VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetUserKDA(
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	balanceHandle, frozenHandle, lastClaimHandle, bucketsHandle, mimeHandle, metadataHandle int32) {
	host := context.GetVMHost()
	ManagedGetUserKDAWithHost(
		host,
		addressHandle,
		tickerHandle,
		nonce,
		balanceHandle, frozenHandle, lastClaimHandle, bucketsHandle, mimeHandle, metadataHandle)

}

// ManagedGetKDATokenData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDATokenData(
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	precisionHandle, idHandle, nameHandle, creatorHandle, adminHandle, logoHandle, urisHandle, initialSupplyHandle, circulatingSupplyHandle, maxSupplyHandle, mintedHandle, burnedHandle, royaltiesHandle, propertiesHandle, attributesHandle, rolesHandle, issueDateHandle int32) {
	host := context.GetVMHost()
	ManagedGetKDATokenDataWithHost(
		host,
		addressHandle,
		tickerHandle,
		nonce,
		precisionHandle, idHandle, nameHandle, creatorHandle, adminHandle, logoHandle, urisHandle, initialSupplyHandle, circulatingSupplyHandle, maxSupplyHandle, mintedHandle, burnedHandle, royaltiesHandle, propertiesHandle, attributesHandle, rolesHandle, issueDateHandle)

}

// ManagedGetKDARoles VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDARoles(
	tickerHandle int32,
	rolesHandle int32) {
	host := context.GetVMHost()
	ManagedGetKDARolesWithHost(
		host,
		tickerHandle,
		rolesHandle)

}

func ManagedGetKDARolesWithHost(
	host vmhost.VMHost,
	tickerHandle int32,
	rolesHandle int32) {
	runtime := host.Runtime()
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedGetKDARolesName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetKDARoles
	metering.UseAndTraceGas(gasToUse)

	ticker, err := managedType.GetBytes(tickerHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	kda, _, err := blockchain.GetKDAToken(nil, ticker, 0)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	roles := writeRolesToBytes(managedType, kda.Roles)
	managedType.SetBytes(rolesHandle, roles)
	managedType.ConsumeGasForBytes(roles)
}

func ManagedGetUserKDAWithHost(
	host vmhost.VMHost,
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	balanceHandle, frozenHandle, lastClaimHandle, bucketsHandle, mimeHandle, metadataHandle int32) {

	runtime := host.Runtime()
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedGetUserKDAName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetUserKDA
	metering.UseAndTraceGas(gasToUse)

	address, err := managedType.GetBytes(addressHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}
	ticker, err := managedType.GetBytes(tickerHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	_, userKDA, err := blockchain.GetKDAToken(address, ticker, uint64(nonce)) // #nosec G115
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	managedType.GetBigIntOrCreate(balanceHandle).Set(big.NewInt(int64(userKDA.Balance)))
	managedType.GetBigIntOrCreate(frozenHandle).Set(big.NewInt(int64(userKDA.FrozenBalance)))

	lastClaim := writeLastClaim(managedType, userKDA.LastClaim)
	managedType.SetBytes(lastClaimHandle, lastClaim)

	buckets := writeUserBuckets(managedType, userKDA.Buckets)
	managedType.SetBytes(bucketsHandle, buckets)
	managedType.ConsumeGasForBytes(buckets)

	managedType.SetBytes(mimeHandle, userKDA.MIME)
	managedType.ConsumeGasForBytes(userKDA.MIME)
	managedType.SetBytes(metadataHandle, userKDA.Metadata)
	managedType.ConsumeGasForBytes(userKDA.Metadata)
}

func ManagedGetKDATokenDataWithHost(
	host vmhost.VMHost,
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	precisionHandle, idHandle, nameHandle, creatorHandle, adminHandle, logoHandle, urisHandle, initialSupplyHandle, circulatingSupplyHandle, maxSupplyHandle, mintedHandle, burnedHandle, royaltiesHandle, propertiesHandle, attributesHandle, rolesHandle, issueDateHandle int32) {
	runtime := host.Runtime()
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedGetKDATokenDataName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetKDATokenData
	metering.UseAndTraceGas(gasToUse)

	address, err := managedType.GetBytes(addressHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}
	ticker, err := managedType.GetBytes(tickerHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	kda, _, err := blockchain.GetKDAToken(address, ticker, uint64(nonce)) // #nosec G115
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	managedType.GetBigIntOrCreate(precisionHandle).Set(big.NewInt(int64(kda.Precision)))
	managedType.SetBytes(idHandle, kda.ID)
	managedType.SetBytes(nameHandle, kda.Name)
	managedType.SetBytes(creatorHandle, kda.OwnerAddress)
	managedType.SetBytes(adminHandle, kda.AdminAddress)
	managedType.SetBytes(logoHandle, []byte(kda.Logo))
	managedType.ConsumeGasForBytes([]byte(kda.Logo))

	managedType.GetBigIntOrCreate(initialSupplyHandle).Set(big.NewInt(int64(kda.InitialSupply)))
	managedType.GetBigIntOrCreate(circulatingSupplyHandle).Set(big.NewInt(int64(kda.CirculatingSupply)))
	managedType.GetBigIntOrCreate(maxSupplyHandle).Set(big.NewInt(int64(kda.MaxSupply)))
	managedType.GetBigIntOrCreate(mintedHandle).Set(big.NewInt(int64(kda.MintedValue)))
	managedType.GetBigIntOrCreate(burnedHandle).Set(big.NewInt(int64(kda.BurnedValue)))
	managedType.GetBigIntOrCreate(issueDateHandle).Set(big.NewInt(int64(kda.IssueDate)))

	royalties := writeRoyaltiesToBytes(managedType, kda.Royalties)
	managedType.SetBytes(royaltiesHandle, royalties)
	managedType.ConsumeGasForBytes(royalties)

	managedType.GetBigIntOrCreate(propertiesHandle).Set(big.NewInt(int64(getPropertiesValue(kda.Properties, int32(kda.AssetType)))))
	managedType.GetBigIntOrCreate(attributesHandle).Set(big.NewInt(int64(getAttributesValue(kda.Attributes))))

	roles := writeRolesToBytes(managedType, kda.Roles)
	managedType.SetBytes(rolesHandle, roles)
	managedType.ConsumeGasForBytes(roles)

	uris := writeURIsToBytes(managedType, kda.URIs)
	managedType.SetBytes(urisHandle, uris)
	managedType.ConsumeGasForBytes(uris)
}

// ManagedUpgradeFromSourceContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedUpgradeFromSourceContract(
	destHandle int32,
	gas int64,
	valueHandle int32,
	addressHandle int32,
	codeMetadataHandle int32,
	argumentsHandle int32,
	resultHandle int32,
) {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedUpgradeFromSourceContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	vmInput, err := readDestinationValueArguments(host, destHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if host.ForkController().FixAuditChangesV4() && vmInput.value.Sign() < 0 {
		WithFaultAndHost(host, vmhost.ErrTransferNegativeValue, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	sourceContractAddress, err := managedType.GetBytes(addressHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	codeMetadata, err := managedType.GetBytes(codeMetadataHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	lenReturnData := len(host.Output().ReturnData())

	UpgradeFromSourceContractWithTypedArgs(
		host,
		sourceContractAddress,
		vmInput.destination,
		vmInput.value.Bytes(),
		vmInput.arguments,
		gas,
		codeMetadata,
	)
	setReturnDataIfExists(host, lenReturnData, resultHandle)
}

// ManagedUpgradeContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedUpgradeContract(
	destHandle int32,
	gas int64,
	valueHandle int32,
	codeHandle int32,
	codeMetadataHandle int32,
	argumentsHandle int32,
	resultHandle int32,
) {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedUpgradeContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	vmInput, err := readDestinationValueArguments(host, destHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if host.ForkController().FixAuditChangesV4() && vmInput.value.Sign() < 0 {
		WithFaultAndHost(host, vmhost.ErrTransferNegativeValue, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	codeMetadata, err := managedType.GetBytes(codeMetadataHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	code, err := managedType.GetBytes(codeHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	lenReturnData := len(host.Output().ReturnData())

	upgradeContract(host, vmInput.destination, code, codeMetadata, vmInput.value.Bytes(), vmInput.arguments, gas)
	setReturnDataIfExists(host, lenReturnData, resultHandle)
}

// ManagedDeleteContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedDeleteContract(
	destHandle int32,
	gasLimit int64,
	argumentsHandle int32,
) {
	host := context.GetVMHost()
	ManagedDeleteContractWithHost(
		host,
		destHandle,
		gasLimit,
		argumentsHandle,
	)
}

func ManagedDeleteContractWithHost(
	host vmhost.VMHost,
	destHandle int32,
	gasLimit int64,
	argumentsHandle int32,
) {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(deleteContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	calledSCAddress, err := managedType.GetBytes(destHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	data, _, err := managedType.ReadManagedVecOfManagedBuffers(argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	deleteContract(
		host,
		calledSCAddress,
		data,
		gasLimit,
	)
}

// ManagedDeployFromSourceContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedDeployFromSourceContract(
	gas int64,
	valueHandle int32,
	addressHandle int32,
	codeMetadataHandle int32,
	argumentsHandle int32,
	resultAddressHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedDeployFromSourceContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	vmInput, err := readDestinationValueArguments(host, addressHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	codeMetadata, err := managedType.GetBytes(codeMetadataHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	lenReturnData := len(host.Output().ReturnData())

	newAddress, err := DeployFromSourceContractWithTypedArgs(
		host,
		vmInput.destination,
		codeMetadata,
		vmInput.value,
		vmInput.arguments,
		gas,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	managedType.SetBytes(resultAddressHandle, newAddress)
	setReturnDataIfExists(host, lenReturnData, resultHandle)

	return 0
}

// ManagedCreateContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedCreateContract(
	gas int64,
	valueHandle int32,
	codeHandle int32,
	codeMetadataHandle int32,
	argumentsHandle int32,
	resultAddressHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedCreateContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	sender := runtime.GetContextAddress()
	value, err := managedType.GetBigInt(valueHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	data, actualLen, err := managedType.ReadManagedVecOfManagedBuffers(argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, actualLen)
	metering.UseAndTraceGas(gasToUse)

	codeMetadata, err := managedType.GetBytes(codeMetadataHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	code, err := managedType.GetBytes(codeHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	lenReturnData := len(host.Output().ReturnData())
	newAddress, err := createContract(sender, data, value, gas, code, codeMetadata, host, CreateContract)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	managedType.SetBytes(resultAddressHandle, newAddress)
	setReturnDataIfExists(host, lenReturnData, resultHandle)

	return 0
}

func setReturnDataIfExists(
	host vmhost.VMHost,
	oldLen int,
	resultHandle int32,
) {
	returnData := host.Output().ReturnData()
	if len(returnData) > oldLen {
		host.ManagedTypes().WriteManagedVecOfManagedBuffers(returnData[oldLen:], resultHandle)
	} else {
		host.ManagedTypes().SetBytes(resultHandle, make([]byte, 0))
	}
}

// ManagedExecuteReadOnly VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedExecuteReadOnly(
	gas int64,
	addressHandle int32,
	functionHandle int32,
	argumentsHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(managedExecuteReadOnlyName)

	vmInput, err := readDestinationFunctionArguments(host, addressHandle, functionHandle, argumentsHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	lenReturnData := len(host.Output().ReturnData())
	returnVal := ExecuteReadOnlyWithTypedArguments(
		host,
		gas,
		[]byte(vmInput.function),
		vmInput.destination,
		vmInput.arguments,
	)
	setReturnDataIfExists(host, lenReturnData, resultHandle)
	return returnVal
}

// ManagedExecuteOnSameContext VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedExecuteOnSameContext(
	gas int64,
	addressHandle int32,
	valueHandle int32,
	functionHandle int32,
	argumentsHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(managedExecuteOnSameContextName)

	vmInput, err := readDestinationValueFunctionArguments(host, addressHandle, valueHandle, functionHandle, argumentsHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	lenReturnData := len(host.Output().ReturnData())
	returnVal := ExecuteOnSameContextWithTypedArgs(
		host,
		gas,
		vmInput.value,
		[]byte(vmInput.function),
		vmInput.destination,
		vmInput.arguments,
	)
	setReturnDataIfExists(host, lenReturnData, resultHandle)
	return returnVal
}

// ManagedExecuteOnDestContext VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedExecuteOnDestContext(
	gas int64,
	addressHandle int32,
	valueHandle int32,
	functionHandle int32,
	argumentsHandle int32,
	resultHandle int32,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(managedExecuteOnDestContextName)

	vmInput, err := readDestinationValueFunctionArguments(host, addressHandle, valueHandle, functionHandle, argumentsHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	lenReturnData := len(host.Output().ReturnData())
	returnVal := ExecuteOnDestContextWithTypedArgs(
		host,
		gas,
		vmInput.value,
		[]byte(vmInput.function),
		vmInput.destination,
		vmInput.arguments,
	)
	setReturnDataIfExists(host, lenReturnData, resultHandle)
	return returnVal
}

// ManagedMultiTransferKDANFTExecute VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedMultiTransferKDANFTExecute(
	dstHandle int32,
	tokenTransfersHandle int32,
	gasLimit int64,
	functionHandle int32,
	argumentsHandle int32,
) int32 {
	host := context.GetVMHost()
	managedType := host.ManagedTypes()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(managedMultiTransferKDANFTExecuteName)

	vmInput, err := readDestinationFunctionArguments(host, dstHandle, functionHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	transfers, err := readKDATransfers(managedType, tokenTransfersHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return TransferKDANFTExecuteWithTypedArgs(
		host,
		vmInput.destination,
		transfers,
		gasLimit,
		[]byte(vmInput.function),
		vmInput.arguments,
	)
}

// ManagedBufferToHex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedBufferToHex(sourceHandle int32, destHandle int32) {
	host := context.GetVMHost()
	ManagedBufferToHexWithHost(host, sourceHandle, destHandle)
}

func ManagedBufferToHexWithHost(host vmhost.VMHost, sourceHandle int32, destHandle int32) {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().ManagedBufferAPICost.MBufferSetBytes
	metering.UseGasAndAddTracedGas(managedBufferToHexName, gasToUse)

	mBuff, err := managedType.GetBytes(sourceHandle)
	if err != nil {
		WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	encoded := hex.EncodeToString(mBuff)
	managedType.SetBytes(destHandle, []byte(encoded))
}

// ManagedGetCodeMetadata VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetCodeMetadata(addressHandle int32, responseHandle int32) {
	host := context.GetVMHost()
	ManagedGetCodeMetadataWithHost(host, addressHandle, responseHandle)
}

func ManagedGetCodeMetadataWithHost(host vmhost.VMHost, addressHandle int32, responseHandle int32) {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCodeMetadata
	metering.UseGasAndAddTracedGas(managedGetCodeMetadataName, gasToUse)

	gasToUse = metering.GasSchedule().ManagedBufferAPICost.MBufferSetBytes
	metering.UseGasAndAddTracedGas(managedGetCodeMetadataName, gasToUse)

	mBuffAddress, err := managedType.GetBytes(addressHandle)
	if err != nil {
		WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	contract, err := host.Blockchain().GetUserAccount(mBuffAddress)
	if err != nil || check.IfNil(contract) {
		WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	codeMetadata := contract.GetCodeMetadata()

	managedType.SetBytes(responseHandle, codeMetadata)
}

// ManagedIsBuiltinFunction VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedIsBuiltinFunction(functionNameHandle int32) int32 {
	host := context.GetVMHost()
	return ManagedIsBuiltinFunctionWithHost(host, functionNameHandle)
}

func ManagedIsBuiltinFunctionWithHost(host vmhost.VMHost, functionNameHandle int32) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.IsBuiltinFunction
	metering.UseGasAndAddTracedGas(managedIsBuiltinFunction, gasToUse)

	mBuffFunctionName, err := managedType.GetBytes(functionNameHandle)
	if err != nil {
		WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
		return -1
	}

	isBuiltinFunction := host.IsBuiltinFunctionName(string(mBuffFunctionName))
	if isBuiltinFunction {
		return 1
	}

	return 0
}

// ManagedGetSftMetadata VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetSftMetadata(
	tickerHandle int32,
	nonce int64,
	dataHandle int32,
) {
	host := context.GetVMHost()
	ManagedGetSftMetadataWithHost(
		host,
		tickerHandle,
		nonce,
		dataHandle)
}

func ManagedGetSftMetadataWithHost(
	host vmhost.VMHost,
	tickerHandle int32,
	nonce int64,
	dataHandle int32,
) {
	runtime := host.Runtime()
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedGetSftMetadataName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetSFTMetadata
	metering.UseAndTraceGas(gasToUse)

	ticker, err := managedType.GetBytes(tickerHandle)
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	meta, err := blockchain.GetSFTMeta(ticker, uint64(nonce)) // #nosec G115
	if err != nil {
		_ = WithFaultAndHost(host, vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	metadata := writeSFTMeta(managedType, meta)
	managedType.SetBytes(dataHandle, metadata)
}

// ManagedAccHasPerm VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedAccHasPerm(
	ops int64,
	sourceAccAddr, targetAccAddr int32,
) int32 {
	return ManagedAccHasPermWithHost(
		context.GetVMHost(),
		ops,
		sourceAccAddr,
		targetAccAddr,
	)
}

func ManagedAccHasPermWithHost(
	host vmhost.VMHost,
	ops int64,
	sourceAccAddr, targetAccAddr int32,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	metering.StartGasTracing(managedAccHasPermName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.Int64Finish +
		metering.GasSchedule().BaseOpsAPICost.GetOwnerAddress
	metering.UseAndTraceGas(gasToUse)

	// check if valid ops
	if ops < 0 {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrArgOutOfRange,
			runtime.BaseOpsErrorShouldFailExecution(),
		)
		return 0
	}

	srcAddBytes, err := managedType.GetBytes(sourceAccAddr)
	if err != nil {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrArgOutOfRange,
			runtime.BaseOpsErrorShouldFailExecution(),
		)
		return 0
	}

	tgtAddBytes, err := managedType.GetBytes(targetAccAddr)
	if err != nil {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrArgOutOfRange,
			runtime.BaseOpsErrorShouldFailExecution(),
		)
		return 0
	}

	acc, err := blockchain.GetUserAccount(srcAddBytes)
	if err != nil {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrArgOutOfRange,
			runtime.BaseOpsErrorShouldFailExecution(),
		)
		return 0
	}

	return ValidatePerm(acc, tgtAddBytes, uint64(ops))
}

func ValidatePerm(acc state.UserAccountHandler, tgtAddBytes []byte, ops uint64) int32 {
	perms := acc.GetPermissions()

	for _, perm := range perms {
		for _, signer := range perm.Signers {
			if !bytes.Equal(signer.GetAddress(), tgtAddBytes) ||
				signer.GetWeight() < perm.GetThreshold() {
				continue
			}

			if perm.CheckPermissionGrantedForUint64(ops) {
				return 1
			}
		}
	}

	return 0
}
