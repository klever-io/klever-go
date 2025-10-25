package vmhost

import (
	"context"
	"crypto/elliptic"
	"io"
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/vmcommon"
)

// StateStack defines the functionality for working with a state stack
type StateStack interface {
	InitState()
	PushState()
	PopSetActiveState()
	PopDiscard()
	ClearStateStack()
}

// CallArgsParser defines the functionality to parse transaction data for a smart contract call
type CallArgsParser interface {
	ParseData(data string) (string, [][]byte, error)
	IsInterfaceNil() bool
}

// VMHost defines the functionality for working with the VM
type VMHost interface {
	vmcommon.VMExecutionHandler
	Crypto() crypto.VMCrypto
	Blockchain() BlockchainContext
	Runtime() RuntimeContext
	ManagedTypes() ManagedTypesContext
	Output() OutputContext
	Metering() MeteringContext
	Storage() StorageContext
	ForkController() core.ForkController

	ExecuteKDATransfer(transfersArgs *KDATransfersArgs, callType vm.CallType) (*vmcommon.VMOutput, uint64, error)
	CreateNewContract(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error)
	ExecuteOnSameContext(input *vmcommon.ContractCallInput) error
	ExecuteOnDestContext(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	IsBuiltinFunctionName(functionName string) bool
	IsBuiltinFunctionCall(data []byte) bool
	GetCallArgsParser() CallArgsParser

	GetGasScheduleMap() config.GasScheduleMap
	GetContexts() (ManagedTypesContext, BlockchainContext, MeteringContext, OutputContext, RuntimeContext, StorageContext)
	SetRuntimeContext(runtime RuntimeContext)

	SetBuiltInFunctionsContainer(builtInFuncs vmcommon.BuiltInFunctionContainer)
	SetExecutionMode(mode vmcommon.ExecutionMode)
	GetExecutionMode() vmcommon.ExecutionMode
	InitState()

	CompleteLogEntriesWithCallType(vmOutput *vmcommon.VMOutput, callType string)

	Reset()

	// GetExecutionContext returns the execution context for timeout protection
	GetExecutionContext() context.Context

	IsInterfaceNil() bool
}

// BlockchainContext defines the functionality needed for interacting with the blockchain context
type BlockchainContext interface {
	StateStack

	NewAddress(creatorAddress []byte) ([]byte, error)
	AccountExists(addr []byte) bool
	GetBalance(addr []byte) []byte
	GetBalanceBigInt(addr []byte) *big.Int
	GetNonce(addr []byte) (uint64, error)
	CurrentEpoch() uint32
	GetStateRootHash() []byte
	LastTimeStamp() int64
	LastNonce() uint64
	LastSlot() uint64
	LastEpoch() uint32
	CurrentSlot() uint64
	CurrentNonce() uint64
	CurrentTimeStamp() int64
	CurrentRandomSeed() []byte
	LastRandomSeed() []byte
	GetCodeHash(addr []byte) []byte
	GetCode(addr []byte) ([]byte, error)
	GetCodeSize(addr []byte) (int32, error)
	BlockHash(number uint64) []byte
	GetOwnerAddress() ([]byte, error)
	IsSmartContract(addr []byte) bool
	IsPayable(sndAddress, rcvAddress []byte) (bool, error)
	SaveCompiledCode(codeHash []byte, code []byte)
	GetCompiledCode(codeHash []byte) (bool, []byte)
	GetKDAToken(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error)
	GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error)
	GetUserAccount(address []byte) (state.UserAccountHandler, error)
	ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	GetSnapshot() int
	RevertToSnapshot(snapshot int)
	TransferValueOnly(destination []byte, sender []byte, value *big.Int) error
	IncreaseNonce(address []byte)
	KDATransfer(sender []byte, tc *transaction.TransferContract) error
	GetKAppController() kapp.KAppController
	ClearCompiledCodes()
	ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
}

// RuntimeContext defines the functionality needed for interacting with the runtime context
type RuntimeContext interface {
	StateStack

	GetVMExecutor() executor.Executor
	ReplaceVMExecutor(vmExecutor executor.Executor)
	InitStateFromContractCallInput(input *vmcommon.ContractCallInput)
	SetCustomCallFunction(callFunction string)
	GetVMInput() *vmcommon.ContractCallInput
	SetVMInput(vmInput *vmcommon.ContractCallInput)
	GetContextAddress() []byte
	GetOriginalCallerAddress() []byte
	SetCodeAddress(scAddress []byte)
	GetSCCode() ([]byte, error)
	GetSCCodeSize() uint64
	GetVMType() []byte
	FunctionName() string
	Arguments() [][]byte
	GetCurrentTxHash() []byte
	GetOriginalTxHash() []byte
	ExtractCodeUpgradeFromArgs() ([]byte, []byte, error)
	SignalUserError(message string)
	FailExecution(err error)
	MustVerifyNextContractCode()
	SetRuntimeBreakpointValue(value BreakpointValue)
	GetRuntimeBreakpointValue() BreakpointValue
	GetInstanceStackSize() uint64
	CountSameContractInstancesOnStack(address []byte) uint64
	IsFunctionImported(name string) bool
	ReadOnly() bool
	SetReadOnly(readOnly bool)
	StartWasmerInstance(contract []byte, gasLimit uint64, newCode bool) error
	ClearWarmInstanceCache()
	SetMaxInstanceStackSize(uint64)
	VerifyContractCode() error
	GetInstance() executor.Instance
	GetInstanceTracker() InstanceTracker
	FunctionNameChecked() (string, error)
	CallSCFunction(functionName string) error
	GetPointsUsed() uint64
	SetPointsUsed(gasPoints uint64)
	BaseOpsErrorShouldFailExecution() bool
	SyncExecAPIErrorShouldFailExecution() bool
	CryptoAPIErrorShouldFailExecution() bool
	BigIntAPIErrorShouldFailExecution() bool
	BigFloatAPIErrorShouldFailExecution() bool
	ManagedBufferAPIErrorShouldFailExecution() bool
	ManagedMapAPIErrorShouldFailExecution() bool
	CleanInstance()

	AddError(err error, otherInfo ...string)
	GetAllErrors() error

	HasFunction(functionName string) bool
	EndExecution()
	ValidateInstances() error
}

// InstanceTracker defines the functionality needed for interacting with the instance tracker
type InstanceTracker interface {
	StateStack

	TrackedInstances() map[string]executor.Instance
}

// ManagedTypesContext defines the functionality needed for interacting with the big int context
type ManagedTypesContext interface {
	StateStack

	GetRandReader() io.Reader
	ConsumeGasForThisBigIntNumberOfBytes(byteLen *big.Int) error
	ConsumeGasForThisIntNumberOfBytes(byteLen int)
	ConsumeGasForBytes(bytes []byte)
	ConsumeGasForBigIntCopy(values ...*big.Int)
	ConsumeGasForBigFloatCopy(values ...*big.Float)
	NewBigInt(value *big.Int) int32
	NewBigIntFromInt64(int64Value int64) int32
	GetBigIntOrCreate(handle int32) *big.Int
	GetBigInt(id int32) (*big.Int, error)
	GetTwoBigInt(handle1 int32, handle2 int32) (*big.Int, *big.Int, error)
	PutBigFloat(value *big.Float) (int32, error)
	BigFloatPrecIsNotValid(precision uint) bool
	BigFloatExpIsNotValid(exponent int) bool
	EncodedBigFloatIsNotValid(encodedBigFloat []byte) bool
	GetBigFloatOrCreate(handle int32) (*big.Float, error)
	GetBigFloat(handle int32) (*big.Float, error)
	GetTwoBigFloats(handle1 int32, handle2 int32) (*big.Float, *big.Float, error)
	PutEllipticCurve(ec *elliptic.CurveParams) int32
	GetEllipticCurve(handle int32) (*elliptic.CurveParams, error)
	GetEllipticCurveSizeOfField(ecHandle int32) int32
	Get100xCurveGasCostMultiplier(ecHandle int32) int32
	GetScalarMult100xCurveGasCostMultiplier(ecHandle int32) int32
	GetUCompressed100xCurveGasCostMultiplier(ecHandle int32) int32
	GetPrivateKeyByteLengthEC(ecHandle int32) int32
	NewManagedBuffer() int32
	NewManagedBufferFromBytes(bytes []byte) int32
	SetBytes(mBufferHandle int32, bytes []byte)
	GetBytes(mBufferHandle int32) ([]byte, error)
	AppendBytes(mBufferHandle int32, bytes []byte) bool
	GetLength(mBufferHandle int32) int32
	GetSlice(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error)
	DeleteSlice(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error)
	InsertSlice(mBufferHandle int32, startPosition int32, slice []byte) ([]byte, error)
	ReadManagedVecOfManagedBuffers(managedVecHandle int32) ([][]byte, uint64, error)
	WriteManagedVecOfManagedBuffers(data [][]byte, destinationHandle int32)
	NewManagedMap() int32
	ManagedMapPut(mMapHandle int32, keyHandle int32, valueHandle int32) error
	ManagedMapGet(mMapHandle int32, keyHandle int32, outValueHandle int32) error
	ManagedMapRemove(mMapHandle int32, keyHandle int32, outValueHandle int32) error
	ManagedMapContains(mMapHandle int32, keyHandle int32) (bool, error)
	GetBackTransfers() ([]*vmcommon.KDATransfer, *big.Int)
	CleanBackTransfers()
	AddValueOnlyBackTransfer(value *big.Int)
	AddBackTransfers(transfers []*vmcommon.KDATransfer)
}

// OutputContext defines the functionality needed for interacting with the output context
type OutputContext interface {
	StateStack
	PopMergeActiveState()
	CensorVMOutput()
	AddToActiveState(rightOutput *vmcommon.VMOutput)

	GetOutputAccount(address []byte) (*vmcommon.OutputAccount, bool)
	SetOutputAccount(address []byte, data *vmcommon.OutputAccount)
	GetOutputAccounts() map[string]*vmcommon.OutputAccount
	DeleteOutputAccount(address []byte)
	WriteLog(address []byte, topics [][]byte, data [][]byte)
	WriteLogWithIdentifier(address []byte, topics [][]byte, data [][]byte, identifier []byte)
	TransferValueOnly(destination []byte, sender []byte, value *big.Int, checkPayable bool) error
	TransferKDA(transfersArgs *KDATransfersArgs, callInput *vmcommon.ContractCallInput) (uint64, error)
	ReturnCode() vmcommon.ReturnCode
	SetReturnCode(returnCode vmcommon.ReturnCode)
	ReturnMessage() string
	SetReturnMessage(message string)
	ReturnData() [][]byte
	ClearReturnData()
	RemoveReturnData(index uint32)
	Finish(data []byte)
	PrependFinish(data []byte)
	DeleteFirstReturnData()
	GetVMOutput() *vmcommon.VMOutput
	RemoveNonUpdatedStorage()
	DeployCode(input CodeDeployInput)
	CreateVMOutputInCaseOfError(err error) *vmcommon.VMOutput
	NextOutputTransferIndex() uint32
	GetCrtTransferIndex() uint32
	SetCrtTransferIndex(index uint32)
	IsInterfaceNil() bool
}

// MeteringContext defines the functionality needed for interacting with the metering context
type MeteringContext interface {
	StateStack
	PopMergeActiveState()

	InitStateFromContractCallInput(input *vmcommon.VMInput)
	SetGasSchedule(gasMap config.GasScheduleMap)
	GasSchedule() *config.GasCost
	UseGas(gas uint64)
	UseAndTraceGas(gas uint64)
	UseGasAndAddTracedGas(functionName string, gas uint64)
	UseGasBoundedAndAddTracedGas(functionName string, gas uint64) error
	RestoreGas(gas uint64)
	GasLeft() uint64
	GasUsedForExecution() uint64
	GasSpentByContract() uint64
	GetGasForExecution() uint64
	GetGasProvided() uint64
	GetSCPrepareInitialCost() uint64
	BoundGasLimit(value int64) uint64
	DeductInitialGasForExecution(contract []byte) error
	DeductInitialGasForDirectDeployment(input CodeDeployInput) error
	DeductInitialGasForIndirectDeployment(input CodeDeployInput) error
	DeductInitialGasForDirectDelete() error
	UseGasForIndirectDeployment(input CodeDeployInput) error
	UseGasBounded(gasToUse uint64) error
	UpdateGasStateOnSuccess(vmOutput *vmcommon.VMOutput) error
	UpdateGasStateOnFailure(vmOutput *vmcommon.VMOutput)
	TrackGasUsedByOutOfVMFunction(builtinInput *vmcommon.ContractCallInput, builtinOutput *vmcommon.VMOutput)
	StartGasTracing(functionName string)
	SetGasTracing(enableGasTracing bool)
	GetGasTrace() map[string]map[string][]uint64
}

// StorageStatus defines the states the storage can be in
type StorageStatus int

const (
	// StorageUnchanged signals that the storage was not changed
	StorageUnchanged StorageStatus = iota

	// StorageModified signals that the storage has been modified
	StorageModified

	// StorageAdded signals that something was added to storage
	StorageAdded

	// StorageDeleted signals that something was removed from storage
	StorageDeleted
)

// StorageContext defines the functionality needed for interacting with the storage context
type StorageContext interface {
	StateStack

	SetAddress(address []byte)
	GetStorageUpdates(address []byte) map[string]*vmcommon.StorageUpdate
	GetStorageFromAddress(address []byte, key []byte) ([]byte, uint32, bool, error)
	GetStorageFromAddressNoChecks(address []byte, key []byte) ([]byte, uint32, bool, error)
	GetStorage(key []byte) ([]byte, uint32, bool, error)
	GetStorageUnmetered(key []byte) ([]byte, uint32, bool, error)
	SetStorage(key []byte, value []byte) (StorageStatus, error)
	SetProtectedStorage(key []byte, value []byte) (StorageStatus, error)
	UseGasForStorageLoad(tracedFunctionName string, trieDepth int64, blockchainLoadCost uint64, usedCache bool) error
	GetVmProtectedPrefix(prefix string) []byte
}

// GasTracing defines the functionality needed for a gas tracing
type GasTracing interface {
	BeginTrace(scAddress string, functionName string)
	AddToCurrentTrace(usedGas uint64)
	AddTracedGas(scAddress string, functionName string, usedGas uint64)
	GetGasTrace() map[string]map[string][]uint64
	IsInterfaceNil() bool
}

// HashComputer provides hash computation
type HashComputer interface {
	Compute(string) []byte
	Size() int
	IsInterfaceNil() bool
}
