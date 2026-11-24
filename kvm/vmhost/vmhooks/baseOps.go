package vmhooks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"google.golang.org/protobuf/proto"
)

const (
	getSCAddressName                 = "getSCAddress"
	getOwnerAddressName              = "getOwnerAddress"
	isSmartContractName              = "isSmartContract"
	getExternalBalanceName           = "getExternalBalance"
	blockHashName                    = "blockHash"
	getArgumentLengthName            = "getArgumentLength"
	getArgumentName                  = "getArgument"
	getFunctionName                  = "getFunction"
	getNumArgumentsName              = "getNumArguments"
	storageStoreName                 = "storageStore"
	storageLoadLengthName            = "storageLoadLength"
	storageLoadName                  = "storageLoad"
	storageLoadFromAddressName       = "storageLoadFromAddress"
	getCallerName                    = "getCaller"
	checkNoPaymentName               = "checkNoPayment"
	callValueName                    = "callValue"
	getKDAValueName                  = "getKDAValue"
	getKDATokenNameName              = "getKDATokenName"
	getKDATokenNonceName             = "getKDATokenNonce"
	getKDATokenTypeName              = "getKDATokenType"
	getCallValueByTokenNameName      = "getCallValueByTokenName"
	getCallValueTokenNameName        = "getCallValueTokenName"
	getKDAValueByIndexName           = "getKDAValueByIndex"
	getKDATokenNameByIndexName       = "getKDATokenNameByIndex"
	getKDATokenNonceByIndexName      = "getKDATokenNonceByIndex"
	getKDATokenTypeByIndexName       = "getKDATokenTypeByIndex"
	getCallValueTokenNameByIndexName = "getCallValueTokenNameByIndex"
	getNumKDATransfersName           = "getNumKDATransfers"
	writeLogName                     = "writeLog"
	writeEventLogName                = "writeEventLog"
	returnDataName                   = "returnData"
	signalErrorName                  = "signalError"
	getGasLeftName                   = "getGasLeft"
	getKDABalanceName                = "getKDABalance"
	getKDANameLength                 = "GetKDANameLength"
	getKDAURILength                  = "GetKDAURILength"
	getKDATokenDataName              = "getKDATokenData"
	validateTokenIdentifierName      = "validateTokenIdentifier"
	executeOnDestContextName         = "executeOnDestContext"
	executeOnSameContextName         = "executeOnSameContext"
	executeReadOnlyName              = "executeReadOnly"
	createContractName               = "createContract"
	deployFromSourceContractName     = "deployFromSourceContract"
	upgradeContractName              = "upgradeContract"
	upgradeFromSourceContractName    = "upgradeFromSourceContract"
	deleteContractName               = "deleteContract"
	getNumReturnDataName             = "getNumReturnData"
	getReturnDataSizeName            = "getReturnDataSize"
	getReturnDataName                = "getReturnData"
	cleanReturnDataName              = "cleanReturnData"
	deleteFromReturnDataName         = "deleteFromReturnData"
	setStorageLockName               = "setStorageLock"
	getStorageLockName               = "getStorageLock"
	isStorageLockedName              = "isStorageLocked"
	clearStorageLockName             = "clearStorageLock"
	getBlockTimestampName            = "getBlockTimestamp"
	getBlockNonceName                = "getBlockNonce"
	getBlockRoundName                = "getBlockRound"
	getBlockEpochName                = "getBlockEpoch"
	getBlockRandomSeedName           = "getBlockRandomSeed"
	getStateRootHashName             = "getStateRootHash"
	getPrevBlockTimestampName        = "getPrevBlockTimestamp"
	getPrevBlockNonceName            = "getPrevBlockNonce"
	getPrevBlockRoundName            = "getPrevBlockRound"
	getPrevBlockEpochName            = "getPrevBlockEpoch"
	getPrevBlockRandomSeedName       = "getPrevBlockRandomSeed"
	getOriginalTxHashName            = "getOriginalTxHash"
	getCurrentTxHashName             = "getCurrentTxHash"
)

type CreateContractCallType int

const (
	CreateContract = iota
	DeployContract
)
const maxNumArgumentsFromMemory = 16000000
const maxTotalArgumentsBytes = vmhost.MaxTotalArgumentsBytes
const maxNumTopics = 16000000

var logEEI = logger.GetOrCreate("vm/eei")

func getKDATransferFromInputFailIfWrongIndex(host vmhost.VMHost, index int32) *vmcommon.KDATransfer {
	kdaTransfers := host.Runtime().GetVMInput().KDATransfers
	// #nosec G115
	if int32(len(kdaTransfers))-1 < index || index < 0 {
		WithFaultAndHost(host, vmhost.ErrInvalidTokenIndex, host.Runtime().BaseOpsErrorShouldFailExecution())
		return nil
	}
	return kdaTransfers[index]
}

func failIfMoreThanOneKDATransfer(context *VMHooksImpl) bool {
	runtime := context.GetRuntimeContext()
	if len(runtime.GetVMInput().KDATransfers) > 1 {
		return context.WithFault(vmhost.ErrTooManyKDATransfers, true)
	}
	return false
}

// GetGasLeft VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetGasLeft() int64 {
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetGasLeft
	metering.UseGasAndAddTracedGas(getGasLeftName, gasToUse)

	return int64(metering.GasLeft()) // #nosec G115 - return negative if exceeded MaxInt64
}

// GetSCAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetSCAddress(resultOffset executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetSCAddress
	metering.UseGasAndAddTracedGas(getSCAddressName, gasToUse)

	owner := runtime.GetContextAddress()
	err := context.MemStore(resultOffset, owner)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
}

// GetOwnerAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetOwnerAddress(resultOffset executor.MemPtr) {
	blockchain := context.GetBlockchainContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetOwnerAddress
	metering.UseGasAndAddTracedGas(getOwnerAddressName, gasToUse)

	err := blockchain.CountContractAccountRead()
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	owner, err := blockchain.GetOwnerAddress()
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	err = context.MemStore(resultOffset, owner)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
}

// IsSmartContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) IsSmartContract(addressOffset executor.MemPtr) int32 {
	blockchain := context.GetBlockchainContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.IsSmartContract
	metering.UseGasAndAddTracedGas(isSmartContractName, gasToUse)

	address, err := context.MemLoad(addressOffset, vmhost.AddressLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	// check recent deploys/used on OutputAccounts first
	if context.GetOutputContext() != nil && context.GetOutputContext().GetOutputAccounts() != nil {
		ocAccs := context.GetOutputContext().GetOutputAccounts()
		if acc, ok := ocAccs[string(address)]; ok && acc != nil && len(acc.Code) > 0 {
			return 1
		}
	}

	if core.IsSmartContractAddress(address) {
		err = blockchain.CountContractAccountRead()
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
	}

	isSmartContract := blockchain.IsSmartContract(address)

	return int32(vmhost.BooleanToInt(isSmartContract)) // #nosec G115
}

// SignalError VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) SignalError(messageOffset executor.MemPtr, messageLength executor.MemLength) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(signalErrorName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.SignalError
	// #nosec G115
	gasToUse += metering.GasSchedule().BaseOperationCost.PersistPerByte * uint64(messageLength)

	err := metering.UseGasBounded(gasToUse)
	if err != nil {
		_ = context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	message, err := context.MemLoad(messageOffset, messageLength)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
	runtime.SignalUserError(string(message))
}

// GetExternalBalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetExternalBalance(addressOffset executor.MemPtr, resultOffset executor.MemPtr) {
	blockchain := context.GetBlockchainContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetExternalBalance
	metering.UseGasAndAddTracedGas(getExternalBalanceName, gasToUse)

	address, err := context.MemLoad(addressOffset, vmhost.AddressLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	err = blockchain.CountContractAccountRead()
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	balance := blockchain.GetBalance(address)

	err = context.MemStore(resultOffset, balance)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
}

// GetBlockHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockHash(nonce int64, resultOffset executor.MemPtr) int32 {
	blockchain := context.GetBlockchainContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockHash
	metering.UseGasAndAddTracedGas(blockHashName, gasToUse)

	hash := blockchain.BlockHash(uint64(nonce)) // #nosec G115
	err := context.MemStore(resultOffset, hash)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// kdaTokenDecodedLen is the encoded size of the records GetKDAToken decoded, a nil one counting
// nothing. proto.Size sums every field and map entry on its own, so it does not depend on map
// iteration order, and it sizes the decoded structs without marshalling them again
func kdaTokenDecodedLen(kdaData *kapps.KDAData, userKDA *kapps.UserKDA) uint64 {
	decodedLen := uint64(0)
	if kdaData != nil {
		decodedLen = uint64(proto.Size(kdaData)) // #nosec G115 - a size is non-negative
	}
	if userKDA != nil {
		decodedLen = math.AddUint64(decodedLen, uint64(proto.Size(userKDA))) // #nosec G115 - a size is non-negative
	}
	return decodedLen
}

// chargeKDATokenDecode charges, from FixAuditChangesV6, DataCopyPerByte for every byte of the
// records GetKDAToken just decoded, bounded, before anything uses them; before the fork it charges
// nothing. GetKDAToken reads and unmarshals the KDAData and, given an address, the UserKDA, whose
// metadata alone can reach MaxLeafSize, for the flat cost of the read. BlockchainHook has no raw-size
// accessor: the UserKDA blob could be read a second time through GetUserAccount, but the KDAData
// lives in the KDA KApp account, which that does not reach. So this charges after the decode rather
// than before it: what is left unpaid is at most one decode per failing transaction, up to
// MaxLeafSize plus the KDAData, while a contract looping over a read pays for every decode after the
// first. A raw-size accessor that lets this charge before the decode is tracked in KLC-2795
func chargeKDATokenDecode(host vmhost.VMHost, kdaData *kapps.KDAData, userKDA *kapps.UserKDA) error {
	if !host.ForkController().FixAuditChangesV6() {
		return nil
	}
	return host.ManagedTypes().ConsumeGasForByteLenBounded(kdaTokenDecodedLen(kdaData, userKDA))
}

func getKDADataFromBlockchainHook(
	context *VMHooksImpl,
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
) (*kapps.KDAData, *kapps.UserKDA, error) {
	host := context.GetVMHost()
	metering := context.GetMeteringContext()
	blockchain := context.GetBlockchainContext()

	// charged into the trace the calling hook already opened, so this is UseGasBounded rather than
	// startTracingAndUseGas: opening a second trace here would split the hook into two entries, and
	// BigIntGetKDAExternalBalance charges its own fixed cost before calling in
	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetExternalBalance
	if host.ForkController().FixAuditChangesV6() {
		if err := metering.UseGasBounded(gasToUse); err != nil {
			return nil, nil, err
		}
	} else {
		metering.UseAndTraceGas(gasToUse)
	}

	// the address is a fixed AddressLen, not contract-sized, so it stays a plain load
	address, err := context.MemLoad(addressOffset, vmhost.AddressLen)
	if err != nil {
		return nil, nil, err
	}

	// the token id was never charged before FixAuditChangesV6, so what is left uncharged is
	// dropped; a length MemLoad would truncate rather than reject must not force the copy for free
	tokenID, _, err := context.memLoadCharged(host, tokenIDOffset, tokenIDLen)
	if err != nil {
		return nil, nil, err
	}

	kdaToken, userKDA, err := blockchain.GetKDAToken(address, tokenID, uint64(nonce)) // #nosec G115
	if err != nil {
		return nil, nil, err
	}

	err = chargeKDATokenDecode(host, kdaToken, userKDA)
	if err != nil {
		return nil, nil, err
	}

	return kdaToken, userKDA, nil
}

// GetKDABalance VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDABalance(
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
	resultOffset executor.MemPtr,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(getKDABalanceName)

	_, userKDA, err := getKDADataFromBlockchainHook(context, addressOffset, tokenIDOffset, tokenIDLen, nonce)

	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}
	err = context.MemStore(resultOffset, big.NewInt(userKDA.Balance).Bytes())
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(big.NewInt(userKDA.Balance).Bytes())) // #nosec G115
}

// GetKDANameLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDANameLength(
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(getKDANameLength)

	kdaData, _, err := getKDADataFromBlockchainHook(context, addressOffset, tokenIDOffset, tokenIDLen, nonce)

	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(kdaData.Name)) // #nosec G115
}

// GetKDAURILength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDAURILength(
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(getKDAURILength)

	kdaData, _, err := getKDADataFromBlockchainHook(context, addressOffset, tokenIDOffset, tokenIDLen, nonce)

	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	if kdaData == nil {
		context.WithFault(vmhost.ErrNilKDAData, runtime.BaseOpsErrorShouldFailExecution())
		return 0
	}

	if len(kdaData.URIs) == 0 {
		return 0
	}

	dMap := types.NewDeterministicMap(kdaData.URIs)

	return int32(len(dMap.GetAt(0))) // #nosec G115
}

// GetKDATokenData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenData(
	addressOffset executor.MemPtr,
	tokenIDOffset executor.MemPtr,
	tokenIDLen executor.MemLength,
	nonce int64,
	precisionHandle int32,
	idOffset executor.MemPtr,
	nameOffset executor.MemPtr,
	creatorOffset executor.MemPtr,
	logoOffset executor.MemPtr,
	initialSupplyOffset executor.MemPtr,
	circulatingSupplyOffset executor.MemPtr,
	maxSupplyOffset executor.MemPtr,
	mintedOffset executor.MemPtr,
	burnedOffset executor.MemPtr,
	royaltiesOffset executor.MemPtr,
	propertiesOffset executor.MemPtr,
	attributesOffset executor.MemPtr,
	rolesOffset executor.MemPtr,
) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(getKDATokenDataName)

	kdaData, userKDA, err := getKDADataFromBlockchainHook(context, addressOffset, tokenIDOffset, tokenIDLen, nonce)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	var properties []byte
	if context.host.ForkController().FixAuditChangesV6() {
		// from FixAuditChangesV6 the properties JSON is paid for, bounded, before anything is
		// written, the precision handle included: it was written for free before the fork (KLC-2755)
		properties, err = json.Marshal(kdaData.Properties)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
		err = managedType.ConsumeGasForByteLenBounded(uint64(len(properties)))
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}

		value := managedType.GetBigIntOrCreate(precisionHandle)
		value.Set(big.NewInt(int64(kdaData.Precision)))
	} else {
		value := managedType.GetBigIntOrCreate(precisionHandle)
		value.Set(big.NewInt(int64(kdaData.Precision)))

		properties, err = json.Marshal(kdaData.Properties)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
	}
	err = context.MemStore(propertiesOffset, properties)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	// BUG: implement all fields
	return int32(len(big.NewInt(userKDA.Balance).Bytes())) // #nosec G115
}

// ValidateTokenIdentifier VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ValidateTokenIdentifier(
	tokenIdHandle int32,
) int32 {
	managedType := context.GetManagedTypesContext()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetArgument
	// from FixAuditChangesV6 the flat gas is bounded, and a buffer longer than any token identifier
	// is answered 0, as ValidateToken answers it, without being cloned first
	err := startTracingAndUseGas(context.host, validateTokenIdentifierName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	if context.host.ForkController().FixAuditChangesV6() &&
		managedType.GetLength(tokenIdHandle) > vmcommon.TokenIdentifierMaxLength {
		return 0
	}

	tokenID, err := managedType.GetBytes(tokenIdHandle)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	if vmcommon.ValidateToken(tokenID) {
		return 1
	} else {
		return 0
	}

}

type indirectContractCallArguments struct {
	dest     []byte
	value    *big.Int
	function []byte
	args     [][]byte
}

func (context *VMHooksImpl) extractIndirectContractCallArgumentsWithValue(
	host vmhost.VMHost,
	destOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) (*indirectContractCallArguments, error) {
	return context.extractIndirectContractCallArguments(
		host,
		destOffset,
		valueOffset,
		true,
		functionOffset,
		functionLength,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

func (context *VMHooksImpl) extractIndirectContractCallArgumentsWithoutValue(
	host vmhost.VMHost,
	destOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) (*indirectContractCallArguments, error) {
	return context.extractIndirectContractCallArguments(
		host,
		destOffset,
		0,
		false,
		functionOffset,
		functionLength,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

func (context *VMHooksImpl) extractIndirectContractCallArguments(
	host vmhost.VMHost,
	destOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	hasValueOffset bool,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) (*indirectContractCallArguments, error) {
	metering := host.Metering()

	dest, err := context.MemLoad(destOffset, vmhost.AddressLen)
	if err != nil {
		return nil, err
	}

	var value *big.Int

	if hasValueOffset {
		valueBytes, err := context.MemLoad(valueOffset, vmhost.BalanceLen)
		if err != nil {
			return nil, err
		}
		value = big.NewInt(0).SetBytes(valueBytes)
	}

	// verifyValidFunctionName rejects contract function names this long and builtin names are far shorter,
	// so a name this long could never be called: reject it before the copy
	if host.ForkController().FixAuditChangesV6() && functionLength >= vmhost.MaxFunctionNameLength {
		return nil, vmhost.ErrInvalidFunctionName
	}

	// the function name was never charged before FixAuditChangesV6, so what is left uncharged is dropped
	function, _, err := context.memLoadCharged(host, functionOffset, functionLength)
	if err != nil {
		return nil, err
	}

	args, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
	if err != nil {
		return nil, err
	}

	if host.ForkController().FixAuditChangesV4() {
		// #nosec G115
		err = host.ManagedTypes().ConsumeGasForByteLenBounded(uint64(argsUncharged))
		if err != nil {
			return nil, err
		}
	} else {
		useDataCopyGas(metering, argsUncharged)
	}

	return &indirectContractCallArguments{
		dest:     dest,
		value:    value,
		function: function,
		args:     args,
	}, nil
}

// TransferKDANFTExecuteWithTypedArgs defines the actual transfer KDA execute logic.
//
// The transfers are settled before the follow-up execution runs, and nothing below the
// transaction undoes them: when that execution fails, failing the whole execution is what rolls
// them back (KLC-2686).
func TransferKDANFTExecuteWithTypedArgs(
	host vmhost.VMHost,
	dest []byte,
	transfers []*vmcommon.KDATransfer,
	gasLimit int64,
	function []byte,
	data [][]byte,
) int32 {
	var executeErr error

	runtime := host.Runtime()
	metering := host.Metering()

	output := host.Output()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.TransferValue * uint64(len(transfers))
	metering.UseAndTraceGas(gasToUse)

	sender := runtime.GetContextAddress()

	var contractCallInput *vmcommon.ContractCallInput
	if len(function) > 0 {
		contractCallInput, executeErr = prepareIndirectContractCallInput(
			host,
			sender,
			big.NewInt(0),
			gasLimit,
			dest,
			function,
			data,
			gasToUse,
		)
		if WithFaultAndHost(host, executeErr, runtime.SyncExecAPIErrorShouldFailExecution()) {
			return 1
		}

		// From FixAuditChangesV6 a zero-value transfer is still settled below, but it moved nothing and
		// is not the callee's call value (KLC-2830). The kept transfers are the ones settled below, so
		// the callee still sees them executed.
		callValueTransfers := transfers
		if host.ForkController().FixAuditChangesV6() {
			callValueTransfers = vmhost.WithoutZeroValueTransfers(transfers)
		}
		contractCallInput.KDATransfers = append(contractCallInput.KDATransfers, callValueTransfers...)

	}

	originalCaller := host.Runtime().GetOriginalCallerAddress()
	transfersArgs := &vmhost.KDATransfersArgs{
		Destination:    dest,
		OriginalCaller: originalCaller,
		Sender:         sender,
		Transfers:      transfers,
	}
	// The transfer records itself in the output state. If it or the follow-up call fails, restore
	// the output as if neither happened, so no contract can observe a transfer that was not kept
	// (KLC-2687). The balances are not restored here: the transfer is already settled and nothing
	// below the transaction undoes it, so failing the whole execution is what rolls it back
	// (KLC-2686). Restore before faulting, because the fault writes the return code into the
	// output state.
	output.PushState()

	gasLimitForExec, executeErr := output.TransferKDA(transfersArgs, contractCallInput)
	if executeErr != nil {
		output.PopSetActiveState()
		WithFaultAndHost(host, executeErr, runtime.BaseOpsErrorShouldFailExecution())
		return 1
	}

	if contractCallInput != nil && host.Blockchain().IsSmartContract(dest) {
		contractCallInput.GasProvided = gasLimitForExec
		logEEI.Trace("KDA post-transfer execution begin")
		_, executeErr := executeOnDestContextFromAPI(host, contractCallInput)
		if executeErr != nil {
			logEEI.Trace("KDA post-transfer execution failed", "error", executeErr)
			output.PopSetActiveState()
			WithFaultAndHost(host, executeErr, runtime.BaseOpsErrorShouldFailExecution())
			return 1
		}
	}

	output.PopDiscard()
	return 0
}

// UpgradeContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) UpgradeContract(
	destOffset executor.MemPtr,
	gasLimit int64,
	valueOffset executor.MemPtr,
	codeOffset executor.MemPtr,
	codeMetadataOffset executor.MemPtr,
	length executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(upgradeContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	value, err := context.MemLoad(valueOffset, vmhost.BalanceLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	code, codeUncharged, err := context.memLoadCharged(host, codeOffset, length)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	codeMetadata, err := context.MemLoad(codeMetadataOffset, vmhost.CodeMetadataLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	data, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)

	useDataCopyGas(metering, argsUncharged)

	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	calledSCAddress, err := context.MemLoad(destOffset, vmhost.AddressLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	useDataCopyGas(metering, codeUncharged)

	upgradeContract(host, calledSCAddress, code, codeMetadata, value, data, gasLimit)

}

// UpgradeFromSourceContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) UpgradeFromSourceContract(
	destOffset executor.MemPtr,
	gasLimit int64,
	valueOffset executor.MemPtr,
	sourceContractAddressOffset executor.MemPtr,
	codeMetadataOffset executor.MemPtr,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(upgradeFromSourceContractName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	metering.UseAndTraceGas(gasToUse)

	value, err := context.MemLoad(valueOffset, vmhost.BalanceLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	sourceContractAddress, err := context.MemLoad(sourceContractAddressOffset, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	codeMetadata, err := context.MemLoad(codeMetadataOffset, vmhost.CodeMetadataLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	data, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)

	useDataCopyGas(metering, argsUncharged)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	calledSCAddress, err := context.MemLoad(destOffset, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	UpgradeFromSourceContractWithTypedArgs(
		host,
		sourceContractAddress,
		calledSCAddress,
		value,
		data,
		gasLimit,
		codeMetadata,
	)
}

// UpgradeFromSourceContractWithTypedArgs - upgradeFromSourceContract with args already read from memory
func UpgradeFromSourceContractWithTypedArgs(
	host vmhost.VMHost,
	sourceContractAddress []byte,
	destContractAddress []byte,
	value []byte,
	data [][]byte,
	gasLimit int64,
	codeMetadata []byte,
) {
	runtime := host.Runtime()
	blockchain := host.Blockchain()

	err := blockchain.CountContractAccountRead()
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	code, err := blockchain.GetCode(sourceContractAddress)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	upgradeContract(host, destContractAddress, code, codeMetadata, value, data, gasLimit)
}

func upgradeContract(
	host vmhost.VMHost,
	destContractAddress []byte,
	code []byte,
	codeMetadata []byte,
	value []byte,
	data [][]byte,
	gasLimit int64,
) {
	runtime := host.Runtime()
	metering := host.Metering()
	gasSchedule := metering.GasSchedule()
	minCallCost := math.MulUint64(2, gasSchedule.BaseOpsAPICost.ExecuteOnDestContext)
	if gasLimit < 0 || uint64(gasLimit) < minCallCost {
		runtime.SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
		return
	}

	args := make([][]byte, 2)
	args[0] = code
	args[1] = codeMetadata
	args = append(args, data...)

	bValue := big.NewInt(0).SetBytes(value)

	contractCallInput, err := prepareIndirectContractCallInput(
		host,
		runtime.GetContextAddress(),
		bValue,
		gasLimit,
		destContractAddress,
		[]byte(vmhost.UpgradeFunctionName),
		args,
		0,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	vmOutput, err := host.ExecuteOnDestContext(contractCallInput)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	host.CompleteLogEntriesWithCallType(vmOutput, vmhost.UpgradeFromSourceString)
}

// DeleteContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) DeleteContract(
	destOffset executor.MemPtr,
	gasLimit int64,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()
	metering.StartGasTracing(deleteContractName)

	data, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)

	useDataCopyGas(metering, argsUncharged)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	calledSCAddress, err := context.MemLoad(destOffset, vmhost.AddressLen)
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

func deleteContract(
	host vmhost.VMHost,
	dest []byte,
	data [][]byte,
	gasLimit int64,
) {
	runtime := host.Runtime()
	metering := host.Metering()
	gasSchedule := metering.GasSchedule()
	minCallCost := math.MulUint64(2, gasSchedule.BaseOpsAPICost.ExecuteOnDestContext)
	if gasLimit < 0 || uint64(gasLimit) < minCallCost {
		runtime.SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
		return
	}

	contractCallInput, err := prepareIndirectContractCallInput(
		host,
		runtime.GetContextAddress(),
		big.NewInt(0),
		gasLimit,
		dest,
		[]byte(vmhost.DeleteFunctionName),
		data,
		0,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	vmOutput, err := host.ExecuteOnDestContext(contractCallInput)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	host.CompleteLogEntriesWithCallType(vmOutput, vmhost.DeleteContractString)
}

// GetArgumentLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetArgumentLength(id int32) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetArgument
	metering.UseGasAndAddTracedGas(getArgumentLengthName, gasToUse)

	args := runtime.Arguments()
	// #nosec G115
	if id < 0 || int32(len(args)) <= id {
		context.WithFault(vmhost.ErrInvalidArgument, runtime.BaseOpsErrorShouldFailExecution())
		return -1
	}

	return int32(len(args[id])) // #nosec G115
}

// GetArgument VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetArgument(id int32, argOffset executor.MemPtr) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetArgument
	// from FixAuditChangesV6 the flat gas is bounded and the argument is paid for before it is
	// written to contract memory, as MBufferGetArgument pays for it
	err := startTracingAndUseGas(context.host, getArgumentName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	args := runtime.Arguments()
	// #nosec G115
	if id < 0 || int32(len(args)) <= id {
		context.WithFault(vmhost.ErrInvalidArgument, runtime.BaseOpsErrorShouldFailExecution())
		return -1
	}

	if context.host.ForkController().FixAuditChangesV6() {
		err = context.GetManagedTypesContext().ConsumeGasForByteLenBounded(uint64(len(args[id])))
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
	}

	err = context.MemStore(argOffset, args[id])
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(args[id])) // #nosec G115
}

// GetFunction VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetFunction(functionOffset executor.MemPtr) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetFunction
	metering.UseGasAndAddTracedGas(getFunctionName, gasToUse)

	function := runtime.FunctionName()
	err := context.MemStore(functionOffset, []byte(function))
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(function)) // #nosec G115
}

// GetNumArguments VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetNumArguments() int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetNumArguments
	metering.UseGasAndAddTracedGas(getNumArgumentsName, gasToUse)

	args := runtime.Arguments()
	return int32(len(args)) // #nosec G115
}

// StorageStore VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) StorageStore(
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	dataOffset executor.MemPtr,
	dataLength executor.MemLength) int32 {

	host := context.GetVMHost()
	return context.StorageStoreWithHost(
		host,
		keyOffset,
		keyLength,
		dataOffset,
		dataLength,
	)
}

// StorageStoreWithHost - storageStore with host instead of pointer context
func (context *VMHooksImpl) StorageStoreWithHost(
	host vmhost.VMHost,
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	dataOffset executor.MemPtr,
	dataLength executor.MemLength) int32 {

	runtime := host.Runtime()

	// the fixed cost first, and bounded, so neither copy can be forced with gas the contract does
	// not have
	gasToUse := host.Metering().GasSchedule().BaseOpsAPICost.StorageStore
	key, err := context.chargeAndLoadKey(host, storageStoreName, gasToUse, keyOffset, keyLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	data, _, err := context.memLoadCharged(host, dataOffset, dataLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return setStorage(host, key, data)
}

// StorageStoreWithTypedArgs - storageStore with args already read from memory
func StorageStoreWithTypedArgs(host vmhost.VMHost, key []byte, data []byte) int32 {
	runtime := host.Runtime()

	gasToUse := host.Metering().GasSchedule().BaseOpsAPICost.StorageStore
	err := startTracingAndUseGas(host, storageStoreName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return setStorage(host, key, data)
}

// setStorage is StorageStoreWithTypedArgs with the fixed cost already charged, which the hook path
// has to do before it copies the key and the value out of contract memory
func setStorage(host vmhost.VMHost, key []byte, data []byte) int32 {
	storageStatus, err := host.Storage().SetStorage(key, data)
	if WithFaultAndHost(host, err, host.Runtime().BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(storageStatus) // #nosec G115
}

// StorageLoadLength VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) StorageLoadLength(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	runtime := context.GetRuntimeContext()
	storage := context.GetStorageContext()
	metering := context.GetMeteringContext()

	key, _, err := context.memLoadCharged(context.GetVMHost(), keyOffset, keyLength)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	getStorage := storage.GetStorageUnmetered
	if context.host.ForkController().FixAuditChangesV6() {
		getStorage = storage.GetStorage
	}

	data, trieDepth, usedCache, err := getStorage(key)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = storage.UseGasForStorageLoad(
		storageLoadLengthName,
		int64(trieDepth),
		metering.GasSchedule().BaseOpsAPICost.StorageLoad,
		usedCache)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(data)) // #nosec G115
}

// StorageLoadFromAddress VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) StorageLoadFromAddress(
	addressOffset executor.MemPtr,
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	dataOffset executor.MemPtr) int32 {

	host := context.GetVMHost()
	return context.StorageLoadFromAddressWithHost(
		host,
		addressOffset,
		keyOffset,
		keyLength,
		dataOffset,
	)
}

// StorageLoadFromAddressWithHost - storageLoadFromAddress with host instead of pointer context
func (context *VMHooksImpl) StorageLoadFromAddressWithHost(
	host vmhost.VMHost,
	addressOffset executor.MemPtr,
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
	dataOffset executor.MemPtr) int32 {

	runtime := host.Runtime()

	key, _, err := context.memLoadCharged(host, keyOffset, keyLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	address, err := context.MemLoad(addressOffset, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	data, usedCache, err := storageLoadFromAddress(host, address, key)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = useGasForCachedStorageValue(host, data, usedCache)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = context.MemStore(dataOffset, data)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(data)) // #nosec G115
}

// StorageLoadFromAddressWithTypedArgs - storageLoadFromAddress with args already read from memory
func StorageLoadFromAddressWithTypedArgs(host vmhost.VMHost, address []byte, key []byte) ([]byte, error) {
	data, _, err := storageLoadFromAddress(host, address, key)
	return data, err
}

func storageLoadFromAddress(host vmhost.VMHost, address []byte, key []byte) ([]byte, bool, error) {
	storage := host.Storage()
	metering := host.Metering()
	data, trieDepth, usedCache, err := storage.GetStorageFromAddress(address, key)
	if err != nil {
		return nil, false, err
	}
	err = storage.UseGasForStorageLoad(
		storageLoadFromAddressName,
		int64(trieDepth),
		metering.GasSchedule().BaseOpsAPICost.StorageLoad,
		usedCache)
	if err != nil {
		return nil, false, err
	}

	return data, usedCache, nil
}

// StorageLoad VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) StorageLoad(keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32 {
	host := context.GetVMHost()
	return context.StorageLoadWithHost(
		host,
		keyOffset,
		keyLength,
		dataOffset,
	)
}

// StorageLoadWithHost - storageLoad with host instead of pointer context
func (context *VMHooksImpl) StorageLoadWithHost(host vmhost.VMHost, keyOffset executor.MemPtr, keyLength executor.MemLength, dataOffset executor.MemPtr) int32 {
	runtime := host.Runtime()

	key, _, err := context.memLoadCharged(host, keyOffset, keyLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	data, usedCache, err := storageLoad(host, key)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = useGasForCachedStorageValue(host, data, usedCache)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = context.MemStore(dataOffset, data)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(data)) // #nosec G115
}

// StorageLoadWithWithTypedArgs - storageLoad with args already read from memory
func StorageLoadWithWithTypedArgs(host vmhost.VMHost, key []byte) ([]byte, error) {
	data, _, err := storageLoad(host, key)
	return data, err
}

func storageLoad(host vmhost.VMHost, key []byte) ([]byte, bool, error) {
	storage := host.Storage()
	metering := host.Metering()
	data, trieDepth, usedCache, err := storage.GetStorage(key)
	if err != nil {
		return nil, false, err
	}

	err = storage.UseGasForStorageLoad(
		storageLoadName,
		int64(trieDepth),
		metering.GasSchedule().BaseOpsAPICost.StorageLoad,
		usedCache)
	if err != nil {
		return nil, false, err
	}

	return data, usedCache, nil
}

func useGasForCachedStorageValue(host vmhost.VMHost, value []byte, usedCache bool) error {
	if !usedCache || !host.ForkController().FixAuditChangesV6() {
		return nil
	}

	return host.ManagedTypes().ConsumeGasForByteLenBounded(uint64(len(value)))
}

// SetStorageLock VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) SetStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength, lockTimestamp int64) int32 {
	host := context.GetVMHost()
	return context.SetStorageLockWithHost(
		host,
		keyOffset,
		keyLength,
		lockTimestamp,
	)
}

// SetStorageLockWithHost - setStorageLock with host instead of pointer context
func (context *VMHooksImpl) SetStorageLockWithHost(host vmhost.VMHost, keyOffset executor.MemPtr, keyLength executor.MemLength, lockTimestamp int64) int32 {
	runtime := host.Runtime()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.Int64StorageStore
	key, err := context.chargeAndLoadKey(host, setStorageLockName, gasToUse, keyOffset, keyLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return SetStorageLockWithTypedArgs(host, key, lockTimestamp)
}

// SetStorageLockWithTypedArgs - setStorageLock with args already read from memory
func SetStorageLockWithTypedArgs(host vmhost.VMHost, key []byte, lockTimestamp int64) int32 {
	runtime := host.Runtime()
	storage := host.Storage()
	timeLockKeyPrefix := string(storage.GetVmProtectedPrefix(vmhost.TimeLockKeyPrefix))
	timeLockKey := vmhost.CustomStorageKey(timeLockKeyPrefix, key)
	bigTimestamp := big.NewInt(0).SetInt64(lockTimestamp)
	storageStatus, err := storage.SetProtectedStorage(timeLockKey, bigTimestamp.Bytes())
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}
	return int32(storageStatus) // #nosec G115
}

// GetStorageLock VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength) int64 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	storage := context.GetStorageContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.StorageLoad
	key, err := context.chargeAndLoadKey(context.GetVMHost(), getStorageLockName, gasToUse, keyOffset, keyLength)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	timeLockKeyPrefix := string(storage.GetVmProtectedPrefix(vmhost.TimeLockKeyPrefix))
	timeLockKey := vmhost.CustomStorageKey(timeLockKeyPrefix, key)

	data, trieDepth, usedCache, err := storage.GetStorage(timeLockKey)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = storage.UseGasForStorageLoad(
		getStorageLockName,
		int64(trieDepth),
		metering.GasSchedule().BaseOpsAPICost.StorageLoad,
		usedCache)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	timeLock := big.NewInt(0).SetBytes(data).Int64()

	// TODO if timelock <= currentTimeStamp { fail somehow }

	return timeLock
}

// IsStorageLocked VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) IsStorageLocked(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	timeLock := context.GetStorageLock(keyOffset, keyLength)
	if timeLock < 0 {
		return -1
	}

	currentTimestamp := context.GetBlockTimestamp()
	if timeLock <= currentTimestamp {
		return 0
	}

	return 1
}

// ClearStorageLock VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ClearStorageLock(keyOffset executor.MemPtr, keyLength executor.MemLength) int32 {
	return context.SetStorageLock(keyOffset, keyLength, 0)
}

// GetCaller VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCaller(resultOffset executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCaller
	metering.UseGasAndAddTracedGas(getCallerName, gasToUse)

	caller := runtime.GetVMInput().CallerAddr

	err := context.MemStore(resultOffset, caller)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}
}

// CheckNoPayment VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) CheckNoPayment() {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(checkNoPaymentName, gasToUse)

	vmInput := runtime.GetVMInput()

	if len(vmInput.KDATransfers) > 0 {
		_ = context.WithFault(vmhost.ErrNonPayableFunctionKda, runtime.BaseOpsErrorShouldFailExecution())
		return
	}
}

// GetCallValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCallValue(resultOffset executor.MemPtr) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := context.ComputeGetCallValueGas()
	metering.UseGasAndAddTracedGas(callValueName, gasToUse)

	value := runtime.GetVMInput().GetKDACallValue(nil).Bytes()
	value = vmhost.PadBytesLeft(value, vmhost.BalanceLen)

	err := context.MemStore(resultOffset, value)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(value)) // #nosec G115
}

// GetKDAValue VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDAValue(resultOffset executor.MemPtr) int32 {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return -1
	}
	return context.GetKDAValueByIndex(resultOffset, 0)
}

// GetKDAValueByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDAValueByIndex(resultOffset executor.MemPtr, index int32) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(getKDAValueByIndexName, gasToUse)

	var value []byte

	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)
	if kdaTransfer != nil && kdaTransfer.KDAValue.Cmp(vmhost.Zero) > 0 {
		value = kdaTransfer.KDAValue.Bytes()
		value = vmhost.PadBytesLeft(value, vmhost.BalanceLen)
	}

	err := context.MemStore(resultOffset, value)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(value)) // #nosec G115
}

// GetKDATokenName VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenName(resultOffset executor.MemPtr) int32 {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return -1
	}
	return context.GetKDATokenNameByIndex(resultOffset, 0)
}

// GetKDATokenNameByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenNameByIndex(resultOffset executor.MemPtr, index int32) int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	fixAuditV6 := context.host.ForkController().FixAuditChangesV6()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	if !fixAuditV6 {
		metering.UseGasAndAddTracedGas(getKDATokenNameByIndexName, gasToUse)
	} else {
		// from FixAuditChangesV6 the fixed cost is bounded, so an underfunded call copies nothing
		err := startTracingAndUseGas(context.host, getKDATokenNameByIndexName, gasToUse)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
	}

	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)
	var tokenName []byte
	if kdaTransfer != nil {
		tokenName = kdaTransfer.KDATokenName
		// from FixAuditChangesV6 the name, which the calling contract sets, is paid for, bounded,
		// before it is copied out
		if fixAuditV6 {
			err := context.GetManagedTypesContext().ConsumeGasForByteLenBounded(uint64(len(tokenName)))
			if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
				return -1
			}
		}
	}

	err := context.MemStore(resultOffset, tokenName)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(tokenName)) // #nosec G115
}

// GetKDATokenNonce VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenNonce() int64 {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return -1
	}
	return context.GetKDATokenNonceByIndex(0)
}

// GetKDATokenNonceByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenNonceByIndex(index int32) int64 {
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(getKDATokenNonceByIndexName, gasToUse)

	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)
	nonce := int64(0)
	if kdaTransfer != nil {
		nonce = int64(kdaTransfer.KDATokenNonce) // #nosec G115
	}
	return nonce
}

// GetKDATokenType VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenType() int32 {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return -1
	}
	return context.GetKDATokenTypeByIndex(0)
}

// GetKDATokenTypeByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetKDATokenTypeByIndex(index int32) int32 {
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(getKDATokenTypeByIndexName, gasToUse)

	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)
	if kdaTransfer != nil {
		return int32(kdaTransfer.KDATokenType) // #nosec G115
	}
	return 0
}

// GetNumKDATransfers VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetNumKDATransfers() int32 {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	metering.UseGasAndAddTracedGas(getNumKDATransfersName, gasToUse)

	return int32(len(runtime.GetVMInput().KDATransfers)) // #nosec G115
}

// GetCallValueByTokenName VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCallValueByTokenName(
	callValueOffset executor.MemPtr,
	tokenNameOffset executor.MemPtr,
	tokenNameLength executor.MemLength,
) int32 {
	host := context.GetVMHost()
	runtime := context.GetRuntimeContext()

	gasToUse := context.ComputeGetCallValueGas()
	err := startTracingAndUseGas(host, getCallValueByTokenNameName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	// the token name was never charged before FixAuditChangesV6, so what is left uncharged is dropped
	tokenName, _, err := context.memLoadCharged(host, tokenNameOffset, tokenNameLength)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	value := runtime.GetVMInput().GetKDACallValue(tokenName).Bytes()
	value = vmhost.PadBytesLeft(value, vmhost.BalanceLen)

	err = context.MemStore(callValueOffset, value)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(value)) // #nosec G115
}

// GetCallValueTokenName VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCallValueTokenName(callValueOffset executor.MemPtr, tokenNameOffset executor.MemPtr) int32 {
	isFail := failIfMoreThanOneKDATransfer(context)
	if isFail {
		return -1
	}
	return context.GetCallValueTokenNameByIndex(callValueOffset, tokenNameOffset, 0)
}

// GetCallValueTokenNameByIndex VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCallValueTokenNameByIndex(
	callValueOffset executor.MemPtr,
	tokenNameOffset executor.MemPtr,
	index int32,
) int32 {

	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()
	fixAuditV6 := context.host.ForkController().FixAuditChangesV6()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCallValue
	if !fixAuditV6 {
		metering.UseGasAndAddTracedGas(getCallValueTokenNameByIndexName, gasToUse)
	} else {
		// from FixAuditChangesV6 the fixed cost is bounded, so an underfunded call copies nothing
		err := startTracingAndUseGas(context.host, getCallValueTokenNameByIndexName, gasToUse)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return -1
		}
	}

	callValue := big.NewInt(0).Bytes()
	tokenName := make([]byte, 0)
	kdaTransfer := getKDATransferFromInputFailIfWrongIndex(context.GetVMHost(), index)

	if kdaTransfer != nil {
		// from FixAuditChangesV6 the name, which the calling contract sets, is paid for, bounded,
		// before it is copied out
		if fixAuditV6 {
			err := context.GetManagedTypesContext().ConsumeGasForByteLenBounded(uint64(len(kdaTransfer.KDATokenName)))
			if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
				return -1
			}
		}
		tokenName = make([]byte, len(kdaTransfer.KDATokenName))
		copy(tokenName, kdaTransfer.KDATokenName)
		callValue = kdaTransfer.KDAValue.Bytes()
	}
	callValue = vmhost.PadBytesLeft(callValue, vmhost.BalanceLen)

	err := context.MemStore(tokenNameOffset, tokenName)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	err = context.MemStore(callValueOffset, callValue)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return int32(len(tokenName)) // #nosec G115
}

// WriteLog VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) WriteLog(
	dataPointer executor.MemPtr,
	dataLength executor.MemLength,
	topicPtr executor.MemPtr,
	numTopics int32) {

	// note: deprecated
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()

	if context.host.ForkController().FixAuditChangesV4() {
		if numTopics < 0 || dataLength < 0 {
			context.WithFault(vmhost.ErrNegativeLength, runtime.BaseOpsErrorShouldFailExecution())
			return
		}
		if numTopics > maxNumTopics {
			context.WithFault(fmt.Errorf("invalid numTopics (%d)", numTopics), runtime.BaseOpsErrorShouldFailExecution())
			return
		}

		gasToUse := metering.GasSchedule().BaseOpsAPICost.Log
		topicsBytes := math.AddUint64(math.MulUint64(uint64(numTopics), uint64(vmhost.HashLen)), uint64(dataLength))
		gas := math.MulUint64(metering.GasSchedule().BaseOperationCost.PersistPerByte, topicsBytes)
		gasToUse = math.AddUint64(gasToUse, gas)
		err := metering.UseGasBoundedAndAddTracedGas(writeLogName, gasToUse)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	} else {
		gasToUse := metering.GasSchedule().BaseOpsAPICost.Log
		// #nosec G115
		gas := math.MulUint64(metering.GasSchedule().BaseOperationCost.PersistPerByte, uint64(numTopics*vmhost.HashLen+dataLength))
		gasToUse = math.AddUint64(gasToUse, gas)
		metering.UseGasAndAddTracedGas(writeLogName, gasToUse)

		if hasNegativeLength(numTopics, dataLength) {
			err := vmhost.ErrNegativeLength
			context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
			return
		}
	}

	log, err := context.MemLoad(dataPointer, dataLength)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	topics := make([][]byte, numTopics)
	for i := int32(0); i < numTopics; i++ {
		topics[i], err = context.MemLoad(topicPtr.Offset(i*vmhost.HashLen), vmhost.HashLen)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	output.WriteLog(runtime.GetContextAddress(), topics, [][]byte{log})
}

func hasNegativeLength(numTopics int32, dataLength executor.MemLength) bool {
	return numTopics < 0 || dataLength < 0
}

// WriteEventLog VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) WriteEventLog(
	numTopics int32,
	topicLengthsOffset executor.MemPtr,
	topicOffset executor.MemPtr,
	dataOffset executor.MemPtr,
	dataLength executor.MemLength,
) {

	host := context.GetVMHost()
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(writeEventLogName)

	// from FixAuditChangesV6 the fixed cost is charged before any copy, and bounded, so a contract
	// cannot force the topic and data copies with gas it does not have. UseGasBounded records into
	// the trace opened above, so the hook still shows up as one entry
	if host.ForkController().FixAuditChangesV6() {
		err := metering.UseGasBounded(metering.GasSchedule().BaseOpsAPICost.Log)
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return
		}
	}

	topics, topicsUncharged, err := context.getArgumentsFromMemory(
		host,
		numTopics,
		topicLengthsOffset,
		topicOffset,
	)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	data, dataUncharged, err := context.memLoadCharged(host, dataOffset, dataLength)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	// pre-fork the fixed cost and both copies were charged here, after the loads, on an int32 sum
	// that could wrap: the topics reach maxTotalArgumentsBytes and the data length reaches int32
	// max, because MemLoad truncates instead of rejecting an oversized length. A wrapped sum
	// saturates the charge and runs the transaction out of gas instead of failing cleanly. From the
	// fork the copies are paid for inside the two loaders above and the fixed cost before them, so
	// there is no sum left to wrap
	if !host.ForkController().FixAuditChangesV6() {
		gasToUse := metering.GasSchedule().BaseOpsAPICost.Log
		gasForData := math.MulUint64(
			metering.GasSchedule().BaseOperationCost.DataCopyPerByte,
			uint64(topicsUncharged+dataUncharged)) // #nosec G115
		gasToUse = math.AddUint64(gasToUse, gasForData)
		metering.UseAndTraceGas(gasToUse)
	}

	output.WriteLog(runtime.GetContextAddress(), topics, [][]byte{data})
}

// GetBlockTimestamp VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockTimestamp() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockTimeStamp
	metering.UseGasAndAddTracedGas(getBlockTimestampName, gasToUse)

	return int64(blockchain.CurrentTimeStamp()) // #nosec G115
}

// GetBlockNonce VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockNonce() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockNonce
	metering.UseGasAndAddTracedGas(getBlockNonceName, gasToUse)

	return int64(blockchain.CurrentNonce()) // #nosec G115
}

// GetBlockRound VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockRound() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRound
	metering.UseGasAndAddTracedGas(getBlockRoundName, gasToUse)

	return int64(blockchain.CurrentSlot()) // #nosec G115
}

// GetBlockEpoch VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockEpoch() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockEpoch
	metering.UseGasAndAddTracedGas(getBlockEpochName, gasToUse)

	return int64(blockchain.CurrentEpoch())
}

// GetBlockRandomSeed VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetBlockRandomSeed(pointer executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRandomSeed
	metering.UseGasAndAddTracedGas(getBlockRandomSeedName, gasToUse)

	randomSeed := blockchain.CurrentRandomSeed()
	err := context.MemStore(pointer, randomSeed)
	context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
}

// GetStateRootHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetStateRootHash(pointer executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetStateRootHash
	metering.UseGasAndAddTracedGas(getStateRootHashName, gasToUse)

	stateRootHash := blockchain.GetStateRootHash()
	err := context.MemStore(pointer, stateRootHash)
	context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
}

// GetPrevBlockTimestamp VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrevBlockTimestamp() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockTimeStamp
	metering.UseGasAndAddTracedGas(getPrevBlockTimestampName, gasToUse)

	return int64(blockchain.LastTimeStamp())
}

// GetPrevBlockNonce VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrevBlockNonce() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockNonce
	metering.UseGasAndAddTracedGas(getPrevBlockNonceName, gasToUse)

	return int64(blockchain.LastNonce()) // #nosec G115
}

// GetPrevBlockRound VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrevBlockRound() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRound
	metering.UseGasAndAddTracedGas(getPrevBlockRoundName, gasToUse)

	return int64(blockchain.LastSlot()) // #nosec G115
}

// GetPrevBlockEpoch VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrevBlockEpoch() int64 {
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockEpoch
	metering.UseGasAndAddTracedGas(getPrevBlockEpochName, gasToUse)

	return int64(blockchain.LastEpoch())
}

// GetPrevBlockRandomSeed VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetPrevBlockRandomSeed(pointer executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	blockchain := context.GetBlockchainContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetBlockRandomSeed
	metering.UseGasAndAddTracedGas(getPrevBlockRandomSeedName, gasToUse)

	randomSeed := blockchain.LastRandomSeed()
	err := context.MemStore(pointer, randomSeed)
	context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
}

// Finish VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) Finish(pointer executor.MemPtr, length executor.MemLength) {
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()
	metering.StartGasTracing(returnDataName)

	gasToUse := metering.GasSchedule().BaseOpsAPICost.Finish
	// #nosec G115
	gas := math.MulUint64(metering.GasSchedule().BaseOperationCost.PersistPerByte, uint64(length))
	gasToUse = math.AddUint64(gasToUse, gas)
	err := metering.UseGasBounded(gasToUse)

	if err != nil {
		_ = context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
		return
	}

	data, err := context.MemLoad(pointer, length)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return
	}

	output.Finish(data)
}

// ExecuteOnSameContext VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ExecuteOnSameContext(
	gasLimit int64,
	addressOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(executeOnSameContextName)

	return context.ExecuteOnSameContextWithHost(
		host,
		gasLimit,
		addressOffset,
		valueOffset,
		functionOffset,
		functionLength,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

// ExecuteOnSameContextWithHost - executeOnSameContext with host instead of pointer context
func (context *VMHooksImpl) ExecuteOnSameContextWithHost(
	host vmhost.VMHost,
	gasLimit int64,
	addressOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	runtime := host.Runtime()

	callArgs, err := context.extractIndirectContractCallArgumentsWithValue(
		host, addressOffset, valueOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return ExecuteOnSameContextWithTypedArgs(
		host,
		gasLimit,
		callArgs.value,
		callArgs.function,
		callArgs.dest,
		callArgs.args,
	)
}

// ExecuteOnSameContextWithTypedArgs - executeOnSameContext with args already read from memory
func ExecuteOnSameContextWithTypedArgs(
	host vmhost.VMHost,
	gasLimit int64,
	value *big.Int,
	function []byte,
	dest []byte,
	args [][]byte,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.ExecuteOnSameContext
	metering.UseAndTraceGas(gasToUse)

	sender := runtime.GetContextAddress()

	contractCallInput, err := prepareIndirectContractCallInput(
		host,
		sender,
		value,
		gasLimit,
		dest,
		function,
		args,
		gasToUse,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	if host.IsBuiltinFunctionName(contractCallInput.Function) {
		WithFaultAndHost(host, vmhost.ErrInvalidBuiltInFunctionCall, runtime.BaseOpsErrorShouldFailExecution())
		return 1
	}

	err = host.ExecuteOnSameContext(contractCallInput)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return 0
}

// ExecuteOnDestContext VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ExecuteOnDestContext(
	gasLimit int64,
	addressOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(executeOnDestContextName)

	return context.ExecuteOnDestContextWithHost(
		host,
		gasLimit,
		addressOffset,
		valueOffset,
		functionOffset,
		functionLength,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

// ExecuteOnDestContextWithHost - executeOnDestContext with host instead of pointer context
func (context *VMHooksImpl) ExecuteOnDestContextWithHost(
	host vmhost.VMHost,
	gasLimit int64,
	addressOffset executor.MemPtr,
	valueOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	runtime := host.Runtime()

	callArgs, err := context.extractIndirectContractCallArgumentsWithValue(
		host, addressOffset, valueOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return ExecuteOnDestContextWithTypedArgs(
		host,
		gasLimit,
		callArgs.value,
		callArgs.function,
		callArgs.dest,
		callArgs.args,
	)
}

// ExecuteOnDestContextWithTypedArgs - executeOnDestContext with args already read from memory
func ExecuteOnDestContextWithTypedArgs(
	host vmhost.VMHost,
	gasLimit int64,
	value *big.Int,
	function []byte,
	dest []byte,
	args [][]byte,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.ExecuteOnDestContext
	metering.UseAndTraceGas(gasToUse)

	sender := runtime.GetContextAddress()

	contractCallInput, err := prepareIndirectContractCallInput(
		host,
		sender,
		value,
		gasLimit,
		dest,
		function,
		args,
		gasToUse,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	vmOutput, err := executeOnDestContextFromAPI(host, contractCallInput)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	host.CompleteLogEntriesWithCallType(vmOutput, vmhost.ExecuteOnDestContextString)

	return 0
}

// ExecuteReadOnly VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) ExecuteReadOnly(
	gasLimit int64,
	addressOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	host := context.GetVMHost()
	metering := host.Metering()
	metering.StartGasTracing(executeReadOnlyName)

	return context.ExecuteReadOnlyWithHost(
		host,
		gasLimit,
		addressOffset,
		functionOffset,
		functionLength,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

// ExecuteReadOnlyWithHost - executeReadOnly with host instead of pointer context
func (context *VMHooksImpl) ExecuteReadOnlyWithHost(
	host vmhost.VMHost,
	gasLimit int64,
	addressOffset executor.MemPtr,
	functionOffset executor.MemPtr,
	functionLength executor.MemLength,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	runtime := host.Runtime()

	callArgs, err := context.extractIndirectContractCallArgumentsWithoutValue(
		host, addressOffset, functionOffset, functionLength, numArguments, argumentsLengthOffset, dataOffset)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return ExecuteReadOnlyWithTypedArguments(
		host,
		gasLimit,
		callArgs.function,
		callArgs.dest,
		callArgs.args,
	)
}

// ExecuteReadOnlyWithTypedArguments - executeReadOnly with args already read from memory
func ExecuteReadOnlyWithTypedArguments(
	host vmhost.VMHost,
	gasLimit int64,
	function []byte,
	dest []byte,
	args [][]byte,
) int32 {
	runtime := host.Runtime()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.ExecuteReadOnly
	metering.UseAndTraceGas(gasToUse)

	sender := runtime.GetContextAddress()

	contractCallInput, err := prepareIndirectContractCallInput(
		host,
		sender,
		big.NewInt(0),
		gasLimit,
		dest,
		function,
		args,
		gasToUse,
	)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	if host.IsBuiltinFunctionName(contractCallInput.Function) {
		WithFaultAndHost(host, vmhost.ErrInvalidBuiltInFunctionCall, runtime.BaseOpsErrorShouldFailExecution())
		return 1
	}

	wasReadOnly := runtime.ReadOnly()
	runtime.SetReadOnly(true)
	_, err = executeOnDestContextFromAPI(host, contractCallInput)
	runtime.SetReadOnly(wasReadOnly)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return -1
	}

	return 0
}

// CreateContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) CreateContract(
	gasLimit int64,
	valueOffset executor.MemPtr,
	codeOffset executor.MemPtr,
	codeMetadataOffset executor.MemPtr,
	length executor.MemLength,
	resultOffset executor.MemPtr,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	host := context.GetVMHost()
	return context.createContractWithHost(
		host,
		gasLimit,
		valueOffset,
		codeOffset,
		codeMetadataOffset,
		length,
		resultOffset,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)
}

func (context *VMHooksImpl) createContractWithHost(
	host vmhost.VMHost,
	gasLimit int64,
	valueOffset executor.MemPtr,
	codeOffset executor.MemPtr,
	codeMetadataOffset executor.MemPtr,
	length executor.MemLength,
	resultOffset executor.MemPtr,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	runtime := host.Runtime()

	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, createContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	sender := runtime.GetContextAddress()
	value, err := context.MemLoad(valueOffset, vmhost.BalanceLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	// From FixAuditChangesV6 the deployment's per-byte cost is charged here instead of inside
	// CreateNewContract: before the copy, on the requested length, and bounded, so a length the
	// contract cannot pay for no longer forces the memcpy. Charging it here also precedes the
	// BoundGasLimit that fixes the new contract's budget, which is what stops the caller
	// promising its whole remainder to the child and then being charged this on top
	code, err := loadMemBytes(context, codeOffset, length, 0, metering.GasSchedule().BaseOperationCost.CompilePerByte)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	codeMetadata, err := context.MemLoad(codeMetadataOffset, vmhost.CodeMetadataLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	data, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)

	useDataCopyGas(metering, argsUncharged)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	valueAsInt := big.NewInt(0).SetBytes(value)
	newAddress, err := createContract(sender, data, valueAsInt, gasLimit, code, codeMetadata, host, CreateContract)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	err = context.MemStore(resultOffset, newAddress)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// DeployFromSourceContract VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) DeployFromSourceContract(
	gasLimit int64,
	valueOffset executor.MemPtr,
	sourceContractAddressOffset executor.MemPtr,
	codeMetadataOffset executor.MemPtr,
	resultAddressOffset executor.MemPtr,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) int32 {
	host := context.GetVMHost()
	runtime := host.Runtime()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CreateContract
	err := startTracingAndUseGas(host, deployFromSourceContractName, gasToUse)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	value, err := context.MemLoad(valueOffset, vmhost.BalanceLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	sourceContractAddress, err := context.MemLoad(sourceContractAddressOffset, vmhost.AddressLen)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	codeMetadata, err := context.MemLoad(codeMetadataOffset, vmhost.CodeMetadataLen)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	data, argsUncharged, err := context.getArgumentsFromMemory(
		host,
		numArguments,
		argumentsLengthOffset,
		dataOffset,
	)

	useDataCopyGas(metering, argsUncharged)

	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	newAddress, err := DeployFromSourceContractWithTypedArgs(
		host,
		sourceContractAddress,
		codeMetadata,
		big.NewInt(0).SetBytes(value),
		data,
		gasLimit,
	)

	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	err = context.MemStore(resultAddressOffset, newAddress)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 1
	}

	return 0
}

// DeployFromSourceContractWithTypedArgs - deployFromSourceContract with args already read from memory
func DeployFromSourceContractWithTypedArgs(
	host vmhost.VMHost,
	sourceContractAddress []byte,
	codeMetadata []byte,
	value *big.Int,
	data [][]byte,
	gasLimit int64,
) ([]byte, error) {
	runtime := host.Runtime()
	sender := runtime.GetContextAddress()

	blockchain := host.Blockchain()
	err := blockchain.CountContractAccountRead()
	if err != nil {
		return nil, err
	}

	code, err := blockchain.GetCode(sourceContractAddress)
	if WithFaultAndHost(host, err, runtime.BaseOpsErrorShouldFailExecution()) {
		return nil, err
	}

	// there is no copy out of contract memory to charge before on this path, but the charge still
	// has to precede the BoundGasLimit inside createContract, as it does in the other deploy hooks.
	// Returned unfaulted: every caller faults it, and faulting here too would log it twice
	if host.ForkController().FixAuditChangesV6() {
		err = host.Metering().UseGasForIndirectDeployment(vmhost.CodeDeployInput{ContractCode: code})
		if err != nil {
			return nil, err
		}
	}

	return createContract(sender, data, value, gasLimit, code, codeMetadata, host, DeployContract)
}

// createContract builds the deploy input and hands it to the host. From FixAuditChangesV6 the
// caller must already have charged UseGasForIndirectDeployment for code: BoundGasLimit below fixes
// the new contract's budget, so a charge made after it lets the caller commit its whole remainder
// to the child and then be charged the code on top. TestVMHooksImpl_CopyGas_SameBeforeAndFromV6
// holds every hook that reaches here to that charge
func createContract(
	sender []byte,
	data [][]byte,
	value *big.Int,
	gasLimit int64,
	code []byte,
	codeMetadata []byte,
	host vmhost.VMHost,
	createContractCallType CreateContractCallType,
) ([]byte, error) {
	originalCaller := host.Runtime().GetOriginalCallerAddress()
	metering := host.Metering()
	contractCreate := &vmcommon.ContractCreateInput{
		VMInput: vmcommon.VMInput{
			OriginalCallerAddr: originalCaller,
			CallerAddr:         sender,
			Arguments:          data,
			GasProvided:        metering.BoundGasLimit(gasLimit),
			KDATransfers:       make([]*vmcommon.KDATransfer, 0),
		},
		ContractCode:         code,
		ContractCodeMetadata: codeMetadata,
	}

	if value.Cmp(big.NewInt(0)) != 0 {
		contractCreate.KDATransfers = append(contractCreate.KDATransfers, &vmcommon.KDATransfer{
			KDAValue:     big.NewInt(0).Set(value),
			KDATokenName: bytes.Clone(kdautils.KLVIdentifier),
		})
	}

	return host.CreateNewContract(contractCreate, int(createContractCallType))
}

// GetNumReturnData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetNumReturnData() int32 {
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetNumReturnData
	metering.UseGasAndAddTracedGas(getNumReturnDataName, gasToUse)

	returnData := output.ReturnData()
	return int32(len(returnData)) // #nosec G115
}

// GetReturnDataSize VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetReturnDataSize(resultID int32) int32 {
	runtime := context.GetRuntimeContext()
	output := context.GetOutputContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetReturnDataSize
	metering.UseGasAndAddTracedGas(getReturnDataSizeName, gasToUse)

	returnData := output.ReturnData()
	// #nosec G115
	if resultID >= int32(len(returnData)) || resultID < 0 {
		context.WithFault(vmhost.ErrInvalidArgument, runtime.BaseOpsErrorShouldFailExecution())
		return 0
	}

	return int32(len(returnData[resultID])) // #nosec G115
}

// GetReturnData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetReturnData(resultID int32, dataOffset executor.MemPtr) int32 {
	host := context.GetVMHost()
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	// from FixAuditChangesV6 the flat gas is bounded and the item is paid for before it is written
	// to contract memory
	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetReturnData
	err := startTracingAndUseGas(host, getReturnDataName, gasToUse)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 0
	}

	result := GetReturnDataWithHostAndTypedArgs(host, resultID)
	if result == nil {
		return 0
	}

	if host.ForkController().FixAuditChangesV6() {
		err = context.GetManagedTypesContext().ConsumeGasForByteLenBounded(uint64(len(result)))
		if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
			return 0
		}
	}

	err = context.MemStore(dataOffset, result)
	if context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution()) {
		return 0
	}

	return int32(len(result)) // #nosec G115
}

// GetReturnDataWithHostAndTypedArgs returns the return data item at resultID, faulting on an index
// out of range. It charges nothing: GetReturnData charges for the item before looking it up
func GetReturnDataWithHostAndTypedArgs(host vmhost.VMHost, resultID int32) []byte {
	returnData := host.Output().ReturnData()
	// #nosec G115
	if resultID >= int32(len(returnData)) || resultID < 0 {
		WithFaultAndHost(host, vmhost.ErrInvalidArgument, host.Runtime().BaseOpsErrorShouldFailExecution())
		return nil
	}

	return returnData[resultID]
}

// CleanReturnData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) CleanReturnData() {
	host := context.GetVMHost()
	CleanReturnDataWithHost(host)
}

// CleanReturnDataWithHost - exposed version of v1_5_deleteFromReturnData for tests
func CleanReturnDataWithHost(host vmhost.VMHost) {
	output := host.Output()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.CleanReturnData
	metering.UseGasAndAddTracedGas(cleanReturnDataName, gasToUse)

	output.ClearReturnData()
}

// DeleteFromReturnData VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) DeleteFromReturnData(resultID int32) {
	host := context.GetVMHost()
	DeleteFromReturnDataWithHost(host, resultID)
}

// DeleteFromReturnDataWithHost - exposed version of v1_5_deleteFromReturnData for tests
func DeleteFromReturnDataWithHost(host vmhost.VMHost, resultID int32) {
	output := host.Output()
	metering := host.Metering()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.DeleteFromReturnData
	metering.UseGasAndAddTracedGas(deleteFromReturnDataName, gasToUse)

	returnData := output.ReturnData()
	// #nosec G115
	if resultID < int32(len(returnData)) {
		output.RemoveReturnData(uint32(resultID)) // #nosec G115
	}
}

// GetOriginalTxHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetOriginalTxHash(dataOffset executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetOriginalTxHash
	metering.UseGasAndAddTracedGas(getOriginalTxHashName, gasToUse)

	err := context.MemStore(dataOffset, runtime.GetOriginalTxHash())
	_ = context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
}

// GetCurrentTxHash VMHooks implementation.
// @autogenerate(VMHooks)
func (context *VMHooksImpl) GetCurrentTxHash(dataOffset executor.MemPtr) {
	runtime := context.GetRuntimeContext()
	metering := context.GetMeteringContext()

	gasToUse := metering.GasSchedule().BaseOpsAPICost.GetCurrentTxHash
	metering.UseGasAndAddTracedGas(getCurrentTxHashName, gasToUse)

	err := context.MemStore(dataOffset, runtime.GetCurrentTxHash())
	_ = context.WithFault(err, runtime.BaseOpsErrorShouldFailExecution())
}

func prepareIndirectContractCallInput(
	host vmhost.VMHost,
	sender []byte,
	value *big.Int,
	gasLimit int64,
	destination []byte,
	function []byte,
	data [][]byte,
	_ uint64,
) (*vmcommon.ContractCallInput, error) {
	metering := host.Metering()

	contractCallInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			OriginalCallerAddr: host.Runtime().GetOriginalCallerAddress(),
			CallerAddr:         sender,
			Arguments:          data,
			GasProvided:        metering.BoundGasLimit(gasLimit),
			CallType:           vm.DirectCall,
			KDATransfers:       make([]*vmcommon.KDATransfer, 0),
		},
		RecipientAddr: bytes.Clone(destination),
		Function:      string(function),
	}

	if value.Cmp(big.NewInt(0)) != 0 {
		contractCallInput.KDATransfers = append(contractCallInput.KDATransfers, &vmcommon.KDATransfer{
			KDAValue:     big.NewInt(0).Set(value),
			KDATokenName: bytes.Clone(kdautils.KLVIdentifier),
		})
	}

	return contractCallInput, nil
}

func (context *VMHooksImpl) getArgumentsFromMemory(
	host vmhost.VMHost,
	numArguments int32,
	argumentsLengthOffset executor.MemPtr,
	dataOffset executor.MemPtr,
) ([][]byte, int32, error) {
	if numArguments < 0 || numArguments > maxNumArgumentsFromMemory {
		return nil, 0, fmt.Errorf("invalid numArguments (%d)", numArguments)
	}
	argumentsLengthsByteLength, err := argumentsLengthByteCount(numArguments)
	if err != nil {
		return nil, 0, err
	}

	if host.ForkController().FixAuditChangesV4() {
		// #nosec G115
		err = host.ManagedTypes().ConsumeGasForByteLenBounded(uint64(argumentsLengthsByteLength))
		if err != nil {
			return nil, 0, err
		}
	}

	argumentsLengthData, err := context.MemLoad(argumentsLengthOffset, argumentsLengthsByteLength)
	if err != nil {
		return nil, 0, err
	}
	if len(argumentsLengthData) != int(argumentsLengthsByteLength) {
		return nil, 0, fmt.Errorf("MemLoad returned truncated data")
	}

	argumentLengths := createInt32Array(argumentsLengthData, numArguments)

	totalArgumentBytes := int32(0)
	for _, length := range argumentLengths {
		if length < 0 {
			return nil, 0, fmt.Errorf("invalid argument length (%d)", length)
		}
		totalArgumentBytes, err = math.AddInt32WithErr(totalArgumentBytes, length)
		if err != nil {
			return nil, 0, fmt.Errorf("total argument length overflow: %w", err)
		}
	}
	if totalArgumentBytes > maxTotalArgumentsBytes {
		return nil, 0, fmt.Errorf("total argument length exceeds maximum")
	}

	// the second return value is what the caller still has to charge: nothing once the data is paid for here
	uncharged := totalArgumentBytes
	if host.ForkController().FixAuditChangesV6() {
		// #nosec G115
		err = host.ManagedTypes().ConsumeGasForByteLenBounded(uint64(totalArgumentBytes))
		if err != nil {
			return nil, 0, err
		}
		uncharged = 0
	}

	data, err := context.MemLoadMultiple(dataOffset, argumentLengths)
	if err != nil {
		return nil, 0, err
	}

	return data, uncharged, nil
}

// memLoadCharged loads length bytes and returns how many of them the caller still has to charge:
// from FixAuditChangesV6 they are paid here, before the load, so that is 0
func (context *VMHooksImpl) memLoadCharged(
	host vmhost.VMHost,
	memPtr executor.MemPtr,
	length executor.MemLength,
) ([]byte, int32, error) {
	uncharged := length
	// a negative length is left for MemLoad to reject, as before the fork
	if host.ForkController().FixAuditChangesV6() && length >= 0 {
		// #nosec G115
		err := host.ManagedTypes().ConsumeGasForByteLenBounded(uint64(length))
		if err != nil {
			return nil, 0, err
		}
		uncharged = 0
	}

	data, err := context.MemLoad(memPtr, length)
	if err != nil {
		return nil, 0, err
	}

	return data, uncharged, nil
}

// chargeAndLoadKey charges the hook's fixed cost first, bounded from FixAuditChangesV6, and then
// the key copy before it happens. The key was never charged before the fork, so what is left
// uncharged is dropped: storing it is priced by SetStorage, not copying it out of contract memory,
// and a length MemLoad would truncate rather than reject must not force the copy for free. The
// error is returned rather than faulted, because the storage hooks fault through different
// predicates
func (context *VMHooksImpl) chargeAndLoadKey(
	host vmhost.VMHost,
	hookName string,
	gasToUse uint64,
	keyOffset executor.MemPtr,
	keyLength executor.MemLength,
) ([]byte, error) {
	err := startTracingAndUseGas(host, hookName, gasToUse)
	if err != nil {
		return nil, err
	}

	key, _, err := context.memLoadCharged(host, keyOffset, keyLength)
	return key, err
}

// chargeAndLoadBytes charges the hook's fixed cost first, bounded from FixAuditChangesV6, and then
// the copy on the requested length before it happens. Pre-fork the copy is charged afterwards, on
// whatever MemLoad returned after truncating and through unbounded UseGas, exactly as it was.
//
// Unlike chargeAndLoadKey this one assumes host == context.GetVMHost(): the copy goes through
// loadMemBytes, which reads the fork flag and the metering off the context's host. A *WithHost
// caller would have to thread host through loadMemBytes first, or it would be charged against two
// hosts
func (context *VMHooksImpl) chargeAndLoadBytes(
	host vmhost.VMHost,
	hookName string,
	gasToUse uint64,
	byteOffset executor.MemPtr,
	byteLength executor.MemLength,
) ([]byte, error) {
	metering := host.Metering()
	dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte

	err := startTracingAndUseGas(host, hookName, gasToUse)
	if err != nil {
		return nil, err
	}

	bytes, err := loadMemBytes(context, byteOffset, byteLength, 0, dataCopyPerByte)
	if err != nil {
		return nil, err
	}

	if !host.ForkController().FixAuditChangesV6() {
		metering.UseGas(math.MulUint64(dataCopyPerByte, uint64(len(bytes))))
	}

	return bytes, nil
}

// useDataCopyGas charges DataCopyPerByte for length bytes
func useDataCopyGas(metering vmhost.MeteringContext, length int32) {
	// #nosec G115
	gasToUse := math.MulUint64(metering.GasSchedule().BaseOperationCost.DataCopyPerByte, uint64(length))
	metering.UseAndTraceGas(gasToUse)
}

func argumentsLengthByteCount(numArguments int32) (int32, error) {
	byteCount := numArguments * 4
	if int64(byteCount) != int64(numArguments)*4 {
		return 0, fmt.Errorf("invalid numArguments (%d)", numArguments)
	}
	return byteCount, nil
}

func createInt32Array(rawData []byte, numIntegers int32) []int32 {
	integers := make([]int32, numIntegers)
	index := 0
	for cursor := 0; cursor < len(rawData); cursor += 4 {
		rawInt := rawData[cursor : cursor+4]
		actualInt := binary.LittleEndian.Uint32(rawInt)
		integers[index] = int32(actualInt) // #nosec G115
		index++
	}
	return integers
}

func executeOnDestContextFromAPI(host vmhost.VMHost, input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	vmOutput, err := host.ExecuteOnDestContext(input)
	if err != nil {
		return nil, err
	}

	return vmOutput, err
}
