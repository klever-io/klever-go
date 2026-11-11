package vmhooks

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"
	"slices"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/tools/check"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
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

	err := blockchain.CountContractAccountRead()
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

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

	gasToUse := metering.GasSchedule().BaseOpsAPICost.SignalError
	err := startTracingAndUseGas(context.host, managedSignalErrorName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	// from FixAuditChangesV6 the message is paid for, DataCopyPerByte and PersistPerByte a byte and
	// bounded, before it is cloned: the amounts charged after the clone before the fork. A missing
	// buffer is not charged, so that GetBytes reports it as before. An underfunded call fails the
	// execution with ErrNotEnoughGas rather than with the message, as it did pre-fork once the
	// persist charge found the gas overdrawn
	if context.host.ForkController().FixAuditChangesV6() {
		errLen := managedType.GetLength(errHandle)
		if errLen >= 0 {
			gasSchedule := metering.GasSchedule()
			gasPerByte := math.AddUint64(gasSchedule.BaseOperationCost.DataCopyPerByte, gasSchedule.BaseOperationCost.PersistPerByte)
			err = metering.UseGasBounded(math.MulUint64(gasPerByte, uint64(errLen)))
			if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
				return
			}
		}

		errBytes, err := managedType.GetBytes(errHandle)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return
		}
		runtime.SignalUserError(string(errBytes))
		return
	}

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
	fixAuditV6 := context.host.ForkController().FixAuditChangesV6()

	topics, sumOfTopicByteLengths, err := managedType.ReadManagedVecOfManagedBuffers(topicsHandle)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	// post-fork the data is paid, bounded, before it is cloned
	if fixAuditV6 {
		err = consumeGasForBufferLenBounded(managedType, dataHandle)
		if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
			return
		}
	}
	dataBytes, err := managedType.GetBytes(dataHandle)
	if context.WithFault(err, runtime.ManagedBufferAPIErrorShouldFailExecution()) {
		return
	}
	if !fixAuditV6 {
		managedType.ConsumeGasForBytes(dataBytes)
	}
	dataByteLen := uint64(len(dataBytes))

	gasToUse := metering.GasSchedule().BaseOpsAPICost.Log
	// pre-fork the topics and the data were charged per byte a second time here, on top of the
	// copies already charged above; post-fork each byte is paid once, as in WriteEventLog
	if !fixAuditV6 {
		gasForData := math.MulUint64(
			metering.GasSchedule().BaseOperationCost.DataCopyPerByte,
			sumOfTopicByteLengths+dataByteLen)
		gasToUse = math.AddUint64(gasToUse, gasForData)
	}
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
	fixAuditV6 := context.host.ForkController().FixAuditChangesV6()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetReturnData
	if !fixAuditV6 {
		metering.UseGasAndAddTracedGas(managedGetReturnDataName, gasToUse)
	} else {
		err := metering.UseGasBoundedAndAddTracedGas(managedGetReturnDataName, gasToUse)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	returnData := output.ReturnData()
	// #nosec G115
	if resultID >= int32(len(returnData)) || resultID < 0 {
		_ = context.WithFault(vmhost.ErrArgOutOfRange, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	// pre-fork the item was copied for the flat cost alone, into a new handle when resultHandle had
	// none; post-fork a new handle is priced as MBufferNewFromBytes and the item's bytes are paid,
	// bounded, before anything is copied
	if fixAuditV6 {
		if managedType.GetLength(resultHandle) < 0 {
			err := metering.UseGasBounded(metering.GasSchedule().ManagedBufferAPICost.MBufferNewFromBytes)
			if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
				return
			}
		}
		err := managedType.ConsumeGasForByteLenBounded(uint64(len(returnData[resultID])))
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	managedType.SetBytes(resultHandle, returnData[resultID])
}

// ManagedGetKDACallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDACallValue(kdaCallValueHandle int32, kdaHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	host := context.GetVMHost()
	gasToUse := context.ComputeGetCallValueGas()
	err := startTracingAndUseGas(host, managedGetKDACallValueName, gasToUse)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	tokenID, err := loadMBufferBytes(host, kdaHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	if err != nil {
		faultKDAArg(host, err)
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
	if !context.host.ForkController().FixAuditChangesV6() {
		metering.UseGasAndAddTracedGas(managedGetMultiKDACallValueName, gasToUse)

		kdaTransfers := runtime.GetVMInput().KDATransfers
		multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
		managedType.ConsumeGasForBytes(multiCallBytes)

		managedType.SetBytes(multiCallValueHandle, multiCallBytes)
		return
	}

	// from FixAuditChangesV6 the fixed cost and the encoded transfers are paid for, bounded, before the
	// transfers are encoded: the amounts charged after the encoding before the fork
	err := startTracingAndUseGas(context.host, managedGetMultiKDACallValueName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	// from FixAuditChangesV6 the transfer values are paid for too, bounded and each on its own, before
	// they are copied into new big int handles: above 32 bytes they pay the big int copy, like the
	// family's other copies
	kdaTransfers := runtime.GetVMInput().KDATransfers
	err = consumeGasForKDATransfersBounded(managedType, kdaTransfers, nil)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	managedType.SetBytes(multiCallValueHandle, writeKDATransfersToBytes(managedType, kdaTransfers))
}

// isKLVTransferName tells whether ManagedGetMultiKDAWithoutKLVCallValue leaves a transfer of this
// token out: no name, KLV's, and from FixAuditChangesV4 an empty one
func isKLVTransferName(kdaName []byte, fixAuditChangesV4 bool) bool {
	return kdaName == nil ||
		len(kdaName) == 0 && fixAuditChangesV4 ||
		bytes.Equal(kdaName, kdautils.KLVIdentifier)
}

// ManagedGetMultiKDAWithoutKLVCallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetMultiKDAWithoutKLVCallValue(multiCallValueHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()
	fixAuditChangesV4 := context.host.ForkController().FixAuditChangesV4()
	fixAuditChangesV6 := context.host.ForkController().FixAuditChangesV6()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	if !fixAuditChangesV6 {
		metering.UseGasAndAddTracedGas(managedGetMultiKDAWithoutKLVCallValueName, gasToUse)
	}

	kdaTransfers := runtime.GetVMInput().KDATransfers
	if fixAuditChangesV6 {
		// from FixAuditChangesV6 the fixed cost and the encoded transfers that are not KLV are paid
		// for, bounded, before anything is cloned, filtered or encoded: the amounts charged after the
		// encoding before the fork. Counting them reads the transfers and copies nothing
		err := startTracingAndUseGas(context.host, managedGetMultiKDAWithoutKLVCallValueName, gasToUse)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}

		// from FixAuditChangesV6 the values of the transfers that are not KLV are paid for too,
		// bounded and each on its own, before they are copied into new big int handles: above 32
		// bytes they pay the big int copy, like the family's other copies
		isKLVTransfer := func(kdaTransfer *vmcommon.KDATransfer) bool {
			return isKLVTransferName(kdaTransfer.KDATokenName, fixAuditChangesV4)
		}
		err = consumeGasForKDATransfersBounded(managedType, kdaTransfers, isKLVTransfer)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	if fixAuditChangesV4 {
		// the loop below deletes in place, which would shuffle the runtime input's backing array
		// it would also leave duplicate elements in the backing array
		kdaTransfers = slices.Clone(kdaTransfers)
	}
	// remove klv transfers if any
	for i := 0; i < len(kdaTransfers); {
		if isKLVTransferName(kdaTransfers[i].KDATokenName, fixAuditChangesV4) {
			// Remove the element by creating a new slice without it
			kdaTransfers = append(kdaTransfers[:i], kdaTransfers[i+1:]...)
		} else {
			i++
		}
	}
	multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
	if !fixAuditChangesV6 {
		managedType.ConsumeGasForBytes(multiCallBytes)
	}

	managedType.SetBytes(multiCallValueHandle, multiCallBytes)
}

// ManagedGetBackTransfers VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetBackTransfers(kdaTransfersValueHandle int32, callValueHandle int32) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	if !context.host.ForkController().FixAuditChangesV6() {
		metering.UseGasAndAddTracedGas(managedGetBackTransferName, gasToUse)

		kdaTransfers, transferValue := managedType.GetBackTransfers()
		multiCallBytes := writeKDATransfersToBytes(managedType, kdaTransfers)
		managedType.ConsumeGasForBytes(multiCallBytes)
		managedType.ConsumeGasForBigIntCopy(transferValue)

		managedType.SetBytes(kdaTransfersValueHandle, multiCallBytes)
		managedType.GetBigIntOrCreate(callValueHandle).Set(transferValue)
		return
	}

	// from FixAuditChangesV6 the fixed cost, the encoded transfers, the transfer values and the call
	// value copy are paid for, bounded and in that order, before the transfers are encoded or the value
	// is copied: the amounts charged after the encoding before the fork, and the transfer values, which
	// are copied into new big int handles and above 32 bytes pay the big int copy, each on its own,
	// like the family's other copies. GetBackTransfers hands the transfers over and clears
	// them, so a call that cannot pay for them still consumes them, as it did before the fork
	err := startTracingAndUseGas(context.host, managedGetBackTransferName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	kdaTransfers, transferValue := managedType.GetBackTransfers()
	err = consumeGasForKDATransfersBounded(managedType, kdaTransfers, nil)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
	err = managedType.ConsumeGasForThisIntNumberOfBytesBounded(int64((transferValue.BitLen() + 7) / 8))
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	managedType.SetBytes(kdaTransfersValueHandle, writeKDATransfersToBytes(managedType, kdaTransfers))
	managedType.GetBigIntOrCreate(callValueHandle).Set(transferValue)
}

// faultKDAArg keeps the KDA getters' historical ErrArgOutOfRange for a bad handle or a missing
// token, while reporting a shortfall on the pre-charged buffer copy as itself: a contract that
// cannot pay for the copy must not be told its argument is out of range. From FixAuditChangesV7
// a usage-counter limit (ErrMaxCallsReached; for these getters, the trie-read limit) is reported
// as itself too
func faultKDAArg(host vmhost.VMHost, err error) {
	keepErr := errors.Is(err, vmhost.ErrNotEnoughGas) ||
		(host.ForkController().FixAuditChangesV7() && errors.Is(err, process.ErrMaxCallsReached))
	if !keepErr {
		err = vmhost.ErrArgOutOfRange
	}
	_ = WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution())
}

// loadKDAToken is everything the two-handle KDA getters do before they start writing handles: the
// fixed cost first and bounded from FixAuditChangesV6, then the address and ticker buffers paid for
// before they are cloned, then the token read, then from FixAuditChangesV6 the decoded records (see
// chargeKDATokenDecode). The error is returned for the caller to pass to
// faultKDAArg, which keeps the historical ErrArgOutOfRange for everything but a shortfall
// and, from FixAuditChangesV7, the trie-read limit
func loadKDAToken(
	host vmhost.VMHost,
	hookName string,
	gasToUse uint64,
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
) (*kapps.KDAData, *kapps.UserKDA, error) {
	dataCopyPerByte := host.Metering().GasSchedule().BaseOperationCost.DataCopyPerByte

	err := startTracingAndUseGas(host, hookName, gasToUse)
	if err != nil {
		return nil, nil, err
	}

	address, err := loadMBufferBytes(host, addressHandle, 0, dataCopyPerByte)
	if err != nil {
		return nil, nil, err
	}

	ticker, err := loadMBufferBytes(host, tickerHandle, 0, dataCopyPerByte)
	if err != nil {
		return nil, nil, err
	}

	kdaData, userKDA, err := host.Blockchain().GetKDAToken(address, ticker, uint64(nonce)) // #nosec G115
	if err != nil {
		return kdaData, userKDA, err
	}

	err = chargeKDATokenDecode(host, kdaData, userKDA)
	if err != nil {
		return nil, nil, err
	}

	return kdaData, userKDA, nil
}

// ManagedGetKDABalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ManagedGetKDABalance(addressHandle int32, tokenIDHandle int32, nonce int64, valueHandle int32) {
	metering := context.GetMeteringContext()
	managedType := context.GetManagedTypesContext()
	host := context.GetVMHost()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetExternalBalance
	_, kdaToken, err := loadKDAToken(host, managedGetKDABalanceName, gasToUse, addressHandle, tokenIDHandle, nonce)
	if err != nil {
		faultKDAArg(host, err)
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
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()
	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetKDARoles
	err := startTracingAndUseGas(host, managedGetKDARolesName, gasToUse)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	ticker, err := loadMBufferBytes(host, tickerHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	kda, _, err := blockchain.GetKDAToken(nil, ticker, 0)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	// read without an address, so only the KDAData was decoded and only it is paid for
	err = chargeKDATokenDecode(host, kda, nil)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	// from FixAuditChangesV6 the roles are paid for, bounded, before they are written: the amount
	// charged while and after writing them before the fork, which is then not charged again
	chargeAfter := managedType
	if host.ForkController().FixAuditChangesV6() {
		err = managedType.ConsumeGasForByteLenBounded(rolesChargedLen(kda.Roles))
		if err != nil {
			faultKDAArg(host, err)
			return
		}
		chargeAfter = unchargedManagedTypes{managedType}
	}

	roles := writeRolesToBytes(chargeAfter, kda.Roles)
	managedType.SetBytes(rolesHandle, roles)
	chargeAfter.ConsumeGasForBytes(roles)
}

func ManagedGetUserKDAWithHost(
	host vmhost.VMHost,
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	balanceHandle, frozenHandle, lastClaimHandle, bucketsHandle, mimeHandle, metadataHandle int32) {

	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetUserKDA
	_, userKDA, err := loadKDAToken(host, managedGetUserKDAName, gasToUse, addressHandle, tickerHandle, nonce)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	// from FixAuditChangesV6 the buckets, MIME and metadata are paid for, bounded, before any output is
	// written: the amounts charged while and after writing them before the fork, which are then not
	// charged again
	chargeAfter := managedType
	if host.ForkController().FixAuditChangesV6() {
		outputLen := math.AddUint64(userBucketsChargedLen(userKDA.Buckets), uint64(len(userKDA.MIME)+len(userKDA.Metadata)))
		err = managedType.ConsumeGasForByteLenBounded(outputLen)
		if err != nil {
			faultKDAArg(host, err)
			return
		}
		chargeAfter = unchargedManagedTypes{managedType}
	}

	managedType.GetBigIntOrCreate(balanceHandle).Set(big.NewInt(int64(userKDA.Balance)))
	managedType.GetBigIntOrCreate(frozenHandle).Set(big.NewInt(int64(userKDA.FrozenBalance)))

	lastClaim := writeLastClaim(managedType, userKDA.LastClaim)
	managedType.SetBytes(lastClaimHandle, lastClaim)

	buckets := writeUserBuckets(chargeAfter, userKDA.Buckets)
	managedType.SetBytes(bucketsHandle, buckets)
	chargeAfter.ConsumeGasForBytes(buckets)

	managedType.SetBytes(mimeHandle, userKDA.MIME)
	chargeAfter.ConsumeGasForBytes(userKDA.MIME)
	managedType.SetBytes(metadataHandle, userKDA.Metadata)
	chargeAfter.ConsumeGasForBytes(userKDA.Metadata)
}

func ManagedGetKDATokenDataWithHost(
	host vmhost.VMHost,
	addressHandle int32,
	tickerHandle int32,
	nonce int64,
	precisionHandle, idHandle, nameHandle, creatorHandle, adminHandle, logoHandle, urisHandle, initialSupplyHandle, circulatingSupplyHandle, maxSupplyHandle, mintedHandle, burnedHandle, royaltiesHandle, propertiesHandle, attributesHandle, rolesHandle, issueDateHandle int32) {
	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetKDATokenData
	kda, _, err := loadKDAToken(host, managedGetKDATokenDataName, gasToUse, addressHandle, tickerHandle, nonce)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	// from FixAuditChangesV6 every byte output is paid for, bounded, before any output is written: the
	// logo, royalties, roles and URIs for the amounts charged while and after writing them before the
	// fork, which are then not charged again, and the id, name, creator and admin, which were written
	// for free before it (KLC-2755)
	chargeAfter := managedType
	if host.ForkController().FixAuditChangesV6() {
		outputLen := uint64(len(kda.ID) + len(kda.Name) + len(kda.OwnerAddress) + len(kda.AdminAddress) + len(kda.Logo))
		outputLen = math.AddUint64(outputLen, royaltiesChargedLen(kda.Royalties))
		outputLen = math.AddUint64(outputLen, rolesChargedLen(kda.Roles))
		outputLen = math.AddUint64(outputLen, urisChargedLen(kda.URIs))
		err = managedType.ConsumeGasForByteLenBounded(outputLen)
		if err != nil {
			faultKDAArg(host, err)
			return
		}
		chargeAfter = unchargedManagedTypes{managedType}
	}

	managedType.GetBigIntOrCreate(precisionHandle).Set(big.NewInt(int64(kda.Precision)))
	managedType.SetBytes(idHandle, kda.ID)
	managedType.SetBytes(nameHandle, kda.Name)
	managedType.SetBytes(creatorHandle, kda.OwnerAddress)
	managedType.SetBytes(adminHandle, kda.AdminAddress)
	managedType.SetBytes(logoHandle, []byte(kda.Logo))
	chargeAfter.ConsumeGasForBytes([]byte(kda.Logo))

	managedType.GetBigIntOrCreate(initialSupplyHandle).Set(big.NewInt(int64(kda.InitialSupply)))
	managedType.GetBigIntOrCreate(circulatingSupplyHandle).Set(big.NewInt(int64(kda.CirculatingSupply)))
	managedType.GetBigIntOrCreate(maxSupplyHandle).Set(big.NewInt(int64(kda.MaxSupply)))
	managedType.GetBigIntOrCreate(mintedHandle).Set(big.NewInt(int64(kda.MintedValue)))
	managedType.GetBigIntOrCreate(burnedHandle).Set(big.NewInt(int64(kda.BurnedValue)))
	managedType.GetBigIntOrCreate(issueDateHandle).Set(big.NewInt(int64(kda.IssueDate)))

	royalties := writeRoyaltiesToBytes(chargeAfter, kda.Royalties)
	managedType.SetBytes(royaltiesHandle, royalties)
	chargeAfter.ConsumeGasForBytes(royalties)

	managedType.GetBigIntOrCreate(propertiesHandle).Set(big.NewInt(int64(getPropertiesValue(kda.Properties, int32(kda.AssetType)))))
	managedType.GetBigIntOrCreate(attributesHandle).Set(big.NewInt(int64(getAttributesValue(kda.Attributes))))

	roles := writeRolesToBytes(chargeAfter, kda.Roles)
	managedType.SetBytes(rolesHandle, roles)
	chargeAfter.ConsumeGasForBytes(roles)

	uris := writeURIsToBytes(chargeAfter, kda.URIs)
	managedType.SetBytes(urisHandle, uris)
	chargeAfter.ConsumeGasForBytes(uris)
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

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, managedUpgradeFromSourceContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	vmInput, err := readDestinationValueArguments(host, destHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if host.ForkController().FixAuditChangesV4() && vmInput.value.Sign() < 0 {
		WithFaultAndHost(host, vmhost.ErrTransferNegativeValue, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	// the source is an address, which ManagedDeployFromSourceContract and the memory path both read
	// as at most AddressLen bytes, so from FixAuditChangesV6 a longer one is rejected before it is
	// cloned, as they reject theirs
	err = checkMBufferMaxLength(host, addressHandle, vmhost.ErrInvalidArgument, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
	sourceContractAddress, err := host.ManagedTypes().GetBytes(addressHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	// the code metadata is paid for before it is cloned from FixAuditChangesV6, as the managed
	// deploy hooks pay for theirs: it is whatever buffer the contract passes, and was cloned for
	// free before
	codeMetadata, err := loadMBufferBytes(host, codeMetadataHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
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
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
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

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, managedUpgradeContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	vmInput, err := readDestinationValueArguments(host, destHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if host.ForkController().FixAuditChangesV4() && vmInput.value.Sign() < 0 {
		WithFaultAndHost(host, vmhost.ErrTransferNegativeValue, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	// the code metadata and the code are paid for before they are cloned from FixAuditChangesV6, as
	// the managed deploy hooks pay for theirs: both are whatever buffer the contract passes, and were
	// cloned for free before. The code is priced as a copy, not compiled: the nested upgrade call
	// charges CompilePerByte for it itself, in DeductInitialGasForIndirectDeployment
	dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte
	codeMetadata, err := loadMBufferBytes(host, codeMetadataHandle, 0, dataCopyPerByte)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	code, err := loadMBufferBytes(host, codeHandle, 0, dataCopyPerByte)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	lenReturnData := len(host.Output().ReturnData())

	upgradeContract(host, vmInput.destination, code, codeMetadata, vmInput.value.Bytes(), vmInput.arguments, gas)
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
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

	// the destination is an address; post-fork a longer one is rejected before it is cloned
	err := checkMBufferMaxLength(host, destHandle, vmhost.ErrInvalidArgument, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

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

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, managedDeployFromSourceContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	vmInput, err := readDestinationValueArguments(host, addressHandle, valueHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	// paid for before it is cloned from FixAuditChangesV6, like the code buffer: the managed hooks
	// take whatever buffer the contract passes, where the legacy ones read a fixed CodeMetadataLen
	codeMetadata, err := loadMBufferBytes(host, codeMetadataHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
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
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

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

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, managedCreateContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	sender := runtime.GetContextAddress()
	value, err := managedType.GetBigInt(valueHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	data, actualLen, err := managedType.ReadManagedVecOfManagedBuffers(argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	// pre-fork the arguments were charged per byte a second time here, on top of the decode;
	// post-fork the decode's charge is the only one
	if !host.ForkController().FixAuditChangesV6() {
		gasToUse = math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, actualLen)
		metering.UseAndTraceGas(gasToUse)
	}

	// paid for before it is cloned from FixAuditChangesV6, like the code buffer: the managed hooks
	// take whatever buffer the contract passes, where the legacy ones read a fixed CodeMetadataLen
	codeMetadata, err := loadMBufferBytes(host, codeMetadataHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	// from FixAuditChangesV6 the deployment's per-byte cost is charged here, before the buffer is
	// cloned and before the BoundGasLimit inside createContract, instead of inside CreateNewContract
	code, err := loadMBufferBytes(host, codeHandle, 0, metering.GasSchedule().BaseOperationCost.CompilePerByte)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	lenReturnData := len(host.Output().ReturnData())
	newAddress, err := createContract(sender, data, value, gas, code, codeMetadata, host, CreateContract)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	managedType.SetBytes(resultAddressHandle, newAddress)
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// setReturnDataIfExists writes the return data the call added after oldLen into resultHandle as a
// managed vector. From FixAuditChangesV6 the conversion is charged per item, bounded, and fails
// with ErrNotEnoughGas instead of making handles it cannot pay for.
func setReturnDataIfExists(
	host vmhost.VMHost,
	oldLen int,
	resultHandle int32,
) error {
	returnData := host.Output().ReturnData()
	if len(returnData) > oldLen {
		return host.ManagedTypes().WriteManagedVecOfManagedBuffers(returnData[oldLen:], resultHandle)
	}
	host.ManagedTypes().SetBytes(resultHandle, make([]byte, 0))
	return nil
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
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}
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
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}
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
	err = setReturnDataIfExists(host, lenReturnData, resultHandle)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}
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
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(managedMultiTransferKDANFTExecuteName)

	vmInput, err := readDestinationFunctionArguments(host, dstHandle, functionHandle, argumentsHandle)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	transfers, err := readKDATransfers(host, tokenTransfersHandle)
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

// maxManagedBufferToHexLength bounds the source buffer ManagedBufferToHex will encode,
// so the hex-encoded output (always exactly 2x the source length) is bounded to 32MB.
// Reuses maxTotalArgumentsBytes (baseOps.go), the established ceiling in this package for
// a single VM-hook-processed blob, rather than duplicating the literal.
const maxManagedBufferToHexLength = maxTotalArgumentsBytes

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

	if host.ForkController().FixAuditChangesV5() {
		if len(mBuff) > maxManagedBufferToHexLength {
			WithFaultAndHost(host, vmhost.ErrManagedBufferToHexLengthExceedsMaximum, runtime.BaseOpsErrorShouldFailExecution())
			return
		}
		// Hex-encoding doubles the byte count: charge once for the source read (N)
		// and once more for the encoded output, at its real size (2N) - both before
		// any encoding happens (CertiK KLR-43 recommendation - charge source and
		// output, fail before allocation). Uses UseGasBounded directly rather than
		// ConsumeGasForBytes, which never fails on insufficient gas - the whole
		// point here is to actually stop execution before the allocation, not just
		// account for it after the fact.
		dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte
		readGas := math.MulUint64(dataCopyPerByte, uint64(len(mBuff)))
		err = metering.UseGasBounded(readGas)
		if err != nil {
			WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
			return
		}

		writeGas := math.MulUint64(dataCopyPerByte, 2*uint64(len(mBuff)))
		err = metering.UseGasBounded(writeGas)
		if err != nil {
			WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
			return
		}
	}

	// Encodes directly into a preallocated destination buffer and hands it to SetBytes as
	// []byte, instead of hex.EncodeToString (make(2N) + an unavoidable string(dst) copy,
	// since dst escapes into the return value) followed by a []byte(encoded) conversion back.
	// This cuts total churn from ~9x source to ~5x (GetBytes' N-byte clone + this 2N encode
	// buffer + SetBytes' own 2N defensive clone) - not all the way to 2x, since SetBytes'
	// clone-on-write is unavoidable from this call site and must stay (it's what closes the
	// managed-buffer aliasing class of bug fixed by KLC-2551 / KLR-28). Peak memory matters
	// here regardless, since this hook runs uninterruptible and the timeout cannot abort it
	// mid-call.
	encoded := make([]byte, hex.EncodedLen(len(mBuff)))
	hex.Encode(encoded, mBuff)
	managedType.SetBytes(destHandle, encoded)
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

	// from FixAuditChangesV6 the flat gas is bounded, and the address is charged per byte before
	// it is cloned. It is still looked up whatever its length: an account can live under an
	// address that is not vmhost.AddressLen long. A missing handle is not charged, so the clone
	// reports it as before the fork
	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCodeMetadata +
		metering.GasSchedule().ManagedBufferAPICost.MBufferSetBytes
	err := startTracingAndUseGas(host, managedGetCodeMetadataName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	if host.ForkController().FixAuditChangesV6() {
		err = consumeGasForBufferLenBounded(managedType, addressHandle)
		if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	mBuffAddress, err := managedType.GetBytes(addressHandle)
	if err != nil {
		WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	err = host.Blockchain().CountContractAccountRead()
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
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

// maxBuiltinFunctionNameLength is vmhost.MaxFunctionNameLength, the protocol limit that
// verifyValidFunctionName (contexts/validator.go) puts on contract-exported function names - a
// built-in function name is the same kind of identifier and can never legitimately exceed it.
// Uses >=, not >, to actually mirror it: validator.go rejects at exactly 256, so a real maximum is 255.
//
// The >= vs > distinction is cosmetic rather than consensus-observable. Since this hook now
// returns 0 instead of reverting, both spellings produce the same result and the same gas at
// exactly 256: the charge is clamped to the cap either way. At 256, > would still charge that
// capped gas but would GetBytes (clone), convert to string, and map-lookup; >= skips all three.
// CapsGas cannot pin the spelling without asserting on that implementation detail - see
// TestManagedIsBuiltinFunctionWithHost_CapsGasAndReturnsFalseAboveMax.
const maxBuiltinFunctionNameLength = vmhost.MaxFunctionNameLength

func ManagedIsBuiltinFunctionWithHost(host vmhost.VMHost, functionNameHandle int32) int32 {
	runtime := host.Runtime()
	metering := host.Metering()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.IsBuiltinFunction
	metering.UseGasAndAddTracedGas(managedIsBuiltinFunction, gasToUse)

	if host.ForkController().FixAuditChangesV5() {
		// Decide and charge from the length alone, before any copy. GetBytes always clones
		// (managedTypesContext.GetBytes -> bytes.Clone), so reading first and only then
		// charging a price of at most maxBuiltinFunctionNameLength bytes at DataCopyPerByte
		// let a contract force an unbounded copy for a fixed, tiny amount of gas - repeatedly,
		// on a hook that runs uninterruptible and cannot be aborted by the execution timeout.
		// ConsumeGasForBytes could not have stopped it either: it routes to UseAndTraceGas,
		// which only accumulates and never fails on insufficient gas. UseGasBounded is the
		// fail-closed half already applied to ManagedBufferToHex, which still clones first
		// and then charges so it can stop before hex allocation, not to skip GetBytes
		// (CertiK KLR-43).
		nameLength := managedType.GetLength(functionNameHandle)
		if nameLength < 0 {
			WithFaultAndHost(host, vmhost.ErrNoManagedBufferUnderThisHandle, runtime.BaseOpsErrorShouldFailExecution())
			return -1
		}

		chargeLength := uint64(nameLength) // #nosec G115 - guarded non-negative above
		if chargeLength > maxBuiltinFunctionNameLength {
			chargeLength = maxBuiltinFunctionNameLength
		}

		dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte
		err := metering.UseGasBounded(math.MulUint64(dataCopyPerByte, chargeLength))
		if err != nil {
			WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution())
			return -1
		}

		if nameLength >= maxBuiltinFunctionNameLength {
			// This is a pure predicate ("is this a builtin function name?"), and a
			// name at or over the cap can never be a real builtin (the same length
			// is already rejected for exported names by verifyValidFunctionName),
			// so a truthful "not a builtin" is the correct answer here - reverting
			// the whole tx would be a new failure mode this call never had before
			// the fork, for input that used to just return false (CertiK KLR-43).
			// Returning here also means an at-or-over-cap buffer is never cloned.
			return 0
		}
	}

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
	metering := host.Metering()
	blockchain := host.Blockchain()
	managedType := host.ManagedTypes()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetSFTMetadata
	err := startTracingAndUseGas(host, managedGetSftMetadataName, gasToUse)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	ticker, err := loadMBufferBytes(host, tickerHandle, 0, metering.GasSchedule().BaseOperationCost.DataCopyPerByte)
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	meta, err := blockchain.GetSFTMeta(ticker, uint64(nonce)) // #nosec G115
	if err != nil {
		faultKDAArg(host, err)
		return
	}

	// from FixAuditChangesV6 the name, hash and attributes are paid for, bounded, before they are
	// written: the amount writeSFTMeta charged while writing them before the fork, which it then does
	// not charge again
	chargeAfter := managedType
	if host.ForkController().FixAuditChangesV6() {
		err = managedType.ConsumeGasForByteLenBounded(sftMetaChargedLen(meta))
		if err != nil {
			faultKDAArg(host, err)
			return
		}
		chargeAfter = unchargedManagedTypes{managedType}
	}

	metadata := writeSFTMeta(chargeAfter, meta)
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

	// from FixAuditChangesV6 the flat gas is bounded, and each address is charged per byte right
	// before it is cloned, the source first, so a missing source handle is never charged the target;
	// an unknown source account still is, as it is looked up after both clones, where it always was.
	// Both are still answered whatever their length: an account can live under an address that is
	// not vmhost.AddressLen long. A missing handle is not charged, so the clone reports it as
	// before the fork
	gasToUse := metering.GasSchedule().BaseOpsAPICost.Int64Finish +
		metering.GasSchedule().BaseOpsAPICost.GetOwnerAddress
	err := startTracingAndUseGas(host, managedAccHasPermName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 0
	}
	fixAuditChangesV6 := host.ForkController().FixAuditChangesV6()

	// check if valid ops
	if ops < 0 {
		_ = WithFaultAndHost(
			host,
			vmhost.ErrArgOutOfRange,
			runtime.BaseOpsErrorShouldFailExecution(),
		)
		return 0
	}

	if fixAuditChangesV6 {
		err = consumeGasForBufferLenBounded(managedType, sourceAccAddr)
		if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
			return 0
		}
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

	if fixAuditChangesV6 {
		err = consumeGasForBufferLenBounded(managedType, targetAccAddr)
		if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
			return 0
		}
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

	err = blockchain.CountContractAccountRead()
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
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
