// Package vmhost contains the top-level components and definitions of the VM
package vmhost

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/vmcommon"
)

// VMVersion returns the current vm version
const VMVersion = "v1.5"

// WASMPageSize is the size in bytes of a WASM linear memory page
const WASMPageSize = uint32(65536)

// BreakpointValue encodes Wasmer runtime breakpoint types
type BreakpointValue uint64

const (
	// BreakpointNone means the lack of a breakpoint
	BreakpointNone BreakpointValue = iota

	// BreakpointExecutionFailed means that Wasmer must stop immediately
	// due to failure indicated by VM
	BreakpointExecutionFailed

	// BreakpointSignalError means that Wasmer must stop immediately
	// due to a contract-signalled error
	BreakpointSignalError

	// BreakpointOutOfGas means that Wasmer must stop immediately
	// due to gas being exhausted
	BreakpointOutOfGas

	// BreakpointMemoryLimit means that Wasmer must stop immediately
	// due to over-allocation of WASM memory
	BreakpointMemoryLimit
)

const (
	// BreakpointNoneString is the human-readable name of BreakpointNone
	BreakpointNoneString = "BreakpointNone"

	// BreakpointExecutionFailedString is the human-readable name of BreakpointExecutionFailed
	BreakpointExecutionFailedString = "BreakpointExecutionFailed"

	// BreakpointSignalErrorString is the human-readable name of BreakpointSignalError
	BreakpointSignalErrorString = "BreakpointSignalError"

	// BreakpointOutOfGasString is the human-readable name of BreakpointOutOfGas
	BreakpointOutOfGasString = "BreakpointOutOfGas"

	// UnknownBreakpointString is the human-readable label for an unknown breakpoint value
	UnknownBreakpointString = "unknown breakpoint"

	// BackTransferString is the human-readable label for execution type
	BackTransferString = "BackTransfer"

	// DirectCallString is the human-readable label for execution type
	DirectCallString = "DirectCall"

	// ExecuteOnDestContextString is the human-readable label for execution type
	ExecuteOnDestContextString = "ExecuteOnDestContext"

	// ExecuteOnSameContextString is the human-readable label for execution type
	ExecuteOnSameContextString = "ExecuteOnSameContext"

	// TransferAndExecuteString is the human-readable label for execution type
	TransferAndExecuteString = "TransferAndExecute"

	// UpgradeFromSourceString is the human-readable label for execution type
	UpgradeFromSourceString = "UpgradeFromSource"

	// DeleteContractString is the human-readable label for execution type
	DeleteContractString = "DeleteContract"

	// TransferValueOnlyString is the human-readable label for transfer type
	TransferValueOnlyString = "transferValueOnly"

	// DeploySmartContractString is the human-readable label for execution type
	DeploySmartContractString = "DeploySmartContract"

	// DeployFromSourceString is the human-readable label for execution type
	DeployFromSourceString = "DeployFromSource"
)

// String returns the human-readable name of a BreakpointValue
func (b BreakpointValue) String() string {
	switch b {
	case BreakpointNone:
		return BreakpointNoneString
	case BreakpointExecutionFailed:
		return BreakpointExecutionFailedString
	case BreakpointSignalError:
		return BreakpointSignalErrorString
	case BreakpointOutOfGas:
		return BreakpointOutOfGasString
	default:
		return UnknownBreakpointString
	}
}

// TimeLockKeyPrefix is the storage key prefix used for timelock-related storage.
const TimeLockKeyPrefix = "TIMELOCK"

const (
	// AddressLen specifies the length of the address
	AddressLen = 32

	// HashLen specifies the lenghth of a hash
	HashLen = 32

	// BalanceLen specifies the number of bytes on which the balance is stored
	BalanceLen = 32

	// CodeMetadataLen specifies the length of the code metadata
	CodeMetadataLen = 2

	// InitFunctionName specifies the name for the init function
	InitFunctionName = "init"

	// UpgradeFunctionName specifies if the call is an upgradeContract call
	UpgradeFunctionName = "upgradeContract"

	// ContractsUpgradeFunctionName specifies the contract's function called at upgrade
	ContractsUpgradeFunctionName = "upgrade"

	// DeleteFunctionName specifies if the call is an deleteContract call
	DeleteFunctionName = "deleteContract"
)

// CodeDeployInput contains code deploy state, whether it comes from a ContractCreateInput or a ContractCallInput
type CodeDeployInput struct {
	ContractCode         []byte
	ContractCodeMetadata []byte
	ContractAddress      []byte
	CodeDeployerAddress  []byte
}

// VMHostParameters represents the parameters to be passed to VMHost
type VMHostParameters struct {
	VMType                              []byte
	OverrideVMExecutor                  executor.ExecutorAbstractFactory
	BlockGasLimit                       uint64
	GasSchedule                         config.GasScheduleMap
	BuiltInFuncContainer                vmcommon.BuiltInFunctionContainer
	KDATransferParser                   vmcommon.KDATransferParser
	ProtectedKeyPrefix                  [][]byte
	WasmerSIGSEGVPassthrough            bool
	EpochNotifier                       vmcommon.EpochNotifier
	ForkController                      core.ForkController
	Hasher                              HashComputer
	TimeOutForSCExecutionInMilliseconds uint32
}

// KDATransfersArgs defines the structure for KDATransferArgs
type KDATransfersArgs struct {
	Destination    []byte
	OriginalCaller []byte
	Sender         []byte
	Transfers      []*vmcommon.KDATransfer
	Function       string
	Arguments      [][]byte
}
