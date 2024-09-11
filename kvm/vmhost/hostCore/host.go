package hostCore

import (
	"context"
	"math"
	"runtime/debug"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto"
	"github.com/klever-io/klever-go/kvm/crypto/factory"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/contexts"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/kvm/wasmer2"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

var log = logger.GetOrCreate("vm/host")
var logGasTrace = logger.GetOrCreate("gasTrace")

// MaximumRuntimeInstanceStackSize specifies the maximum number of allowed Wasmer
// instances on the InstanceStack of the RuntimeContext
var MaximumRuntimeInstanceStackSize = uint64(10)

var _ vmhost.VMHost = (*vmHost)(nil)

const minExecutionTimeout = time.Millisecond * 400
const internalVMErrors = "internalVMErrors"

// vmHost implements HostContext interface.
type vmHost struct {
	cryptoHook       crypto.VMCrypto
	mutExecution     sync.RWMutex
	closingInstance  bool
	executionTimeout time.Duration

	blockchainContext   vmhost.BlockchainContext
	runtimeContext      vmhost.RuntimeContext
	outputContext       vmhost.OutputContext
	meteringContext     vmhost.MeteringContext
	storageContext      vmhost.StorageContext
	managedTypesContext vmhost.ManagedTypesContext

	gasSchedule          config.GasScheduleMap
	builtInFuncContainer vmcommon.BuiltInFunctionContainer
	kdaTransferParser    vmcommon.KDATransferParser
	callArgsParser       vmhost.CallArgsParser
	forkController       core.ForkController
	activationEpochMap   map[uint32]struct{}

	transferLogIdentifiers map[string]bool
}

// NewVMHost creates a new VM vmHost
func NewVMHost(
	blockChainHook vmcommon.BlockchainHook,
	hostParameters *vmhost.VMHostParameters,
) (vmhost.VMHost, error) {
	if check.IfNil(blockChainHook) {
		return nil, vmhost.ErrNilBlockChainHook
	}
	if hostParameters == nil {
		return nil, vmhost.ErrNilHostParameters
	}
	if check.IfNil(hostParameters.KDATransferParser) {
		return nil, vmhost.ErrNilKDATransferParser
	}
	if check.IfNil(hostParameters.BuiltInFuncContainer) {
		return nil, vmhost.ErrNilBuiltInFunctionsContainer
	}
	if check.IfNil(hostParameters.EpochNotifier) {
		return nil, vmhost.ErrNilEpochNotifier
	}
	if check.IfNil(hostParameters.ForkController) {
		return nil, vmhost.ErrNilEnableEpochsHandler
	}
	if check.IfNil(hostParameters.Hasher) {
		return nil, vmhost.ErrNilHasher
	}
	if hostParameters.VMType == nil {
		return nil, vmhost.ErrNilVMType
	}

	cryptoHook := factory.NewVMCrypto()
	host := &vmHost{
		cryptoHook:           cryptoHook,
		meteringContext:      nil,
		runtimeContext:       nil,
		blockchainContext:    nil,
		storageContext:       nil,
		managedTypesContext:  nil,
		gasSchedule:          hostParameters.GasSchedule,
		builtInFuncContainer: hostParameters.BuiltInFuncContainer,
		kdaTransferParser:    hostParameters.KDATransferParser,
		callArgsParser:       parsers.NewCallArgsParser(),
		executionTimeout:     minExecutionTimeout,
		forkController:       hostParameters.ForkController,
	}
	newExecutionTimeout := time.Duration(hostParameters.TimeOutForSCExecutionInMilliseconds) * time.Millisecond
	if newExecutionTimeout > minExecutionTimeout {
		host.executionTimeout = newExecutionTimeout
	}

	var err error
	host.blockchainContext, err = contexts.NewBlockchainContext(host, blockChainHook)
	if err != nil {
		return nil, err
	}

	vmExecutor, err := host.createExecutor(hostParameters)
	if err != nil {
		return nil, err
	}

	host.runtimeContext, err = contexts.NewRuntimeContext(
		host,
		hostParameters.VMType,
		host.builtInFuncContainer,
		vmExecutor,
		hostParameters.Hasher,
	)
	if err != nil {
		return nil, err
	}

	host.meteringContext, err = contexts.NewMeteringContext(host, hostParameters.GasSchedule, hostParameters.BlockGasLimit)
	if err != nil {
		return nil, err
	}

	host.outputContext, err = contexts.NewOutputContext(host)
	if err != nil {
		return nil, err
	}

	host.storageContext, err = contexts.NewStorageContext(
		host,
		blockChainHook,
		hostParameters.ProtectedKeyPrefix,
	)
	if err != nil {
		return nil, err
	}

	host.managedTypesContext, err = contexts.NewManagedTypesContext(host)
	if err != nil {
		return nil, err
	}

	host.runtimeContext.SetMaxInstanceStackSize(MaximumRuntimeInstanceStackSize)

	host.initContexts()
	hostParameters.EpochNotifier.RegisterNotifyHandler(host)

	host.transferLogIdentifiers = make(map[string]bool)
	host.transferLogIdentifiers["transferValueOnly"] = true
	host.transferLogIdentifiers["KleverTransfer"] = true

	return host, nil
}

// Creates a new executor instance. Should only be called once per VM host instantiation.
func (host *vmHost) createExecutor(hostParameters *vmhost.VMHostParameters) (executor.Executor, error) {
	vmHooks := vmhooks.NewVMHooksImpl(host)
	gasCostConfig, err := config.CreateGasConfig(host.gasSchedule)
	if err != nil {
		return nil, err
	}

	var vmExecutorFactory executor.ExecutorAbstractFactory

	if hostParameters.OverrideVMExecutor != nil {
		vmExecutorFactory = hostParameters.OverrideVMExecutor
	} else {
		vmExecutorFactory = wasmer2.ExecutorFactory()
	}
	vmExecutorFactoryArgs := executor.ExecutorFactoryArgs{
		VMHooks:                  vmHooks,
		OpcodeCosts:              gasCostConfig.WASMOpcodeCost,
		RkyvSerializationEnabled: true,
		WasmerSIGSEGVPassthrough: hostParameters.WasmerSIGSEGVPassthrough,
	}
	return vmExecutorFactory.CreateExecutor(vmExecutorFactoryArgs)
}

// GetVersion returns the VM version string
func (host *vmHost) GetVersion() string {
	return vmhost.VMVersion
}

// Crypto returns the VMCrypto instance of the host
func (host *vmHost) Crypto() crypto.VMCrypto {
	return host.cryptoHook
}

// Blockchain returns the BlockchainContext instance of the host
func (host *vmHost) Blockchain() vmhost.BlockchainContext {
	return host.blockchainContext
}

// Runtime returns the RuntimeContext instance of the host
func (host *vmHost) Runtime() vmhost.RuntimeContext {
	return host.runtimeContext
}

// Output returns the OutputContext instance of the host
func (host *vmHost) Output() vmhost.OutputContext {
	return host.outputContext
}

// Metering returns the MeteringContext instance of the host
func (host *vmHost) Metering() vmhost.MeteringContext {
	return host.meteringContext
}

// Storage returns the StorageContext instance of the host
func (host *vmHost) Storage() vmhost.StorageContext {
	return host.storageContext
}

// ForkController returns the forkController instance of the host
func (host *vmHost) ForkController() core.ForkController {
	return host.forkController
}

// ManagedTypes returns the ManagedTypeContext instance of the host
func (host *vmHost) ManagedTypes() vmhost.ManagedTypesContext {
	return host.managedTypesContext
}

// GetContexts returns the main contexts of the host
func (host *vmHost) GetContexts() (
	vmhost.ManagedTypesContext,
	vmhost.BlockchainContext,
	vmhost.MeteringContext,
	vmhost.OutputContext,
	vmhost.RuntimeContext,
	vmhost.StorageContext,
) {
	return host.managedTypesContext,
		host.blockchainContext,
		host.meteringContext,
		host.outputContext,
		host.runtimeContext,
		host.storageContext
}

// InitState resets the contexts of the host and reconfigures its flags
func (host *vmHost) InitState() {
	host.initContexts()
}

func (host *vmHost) close() {
	host.runtimeContext.ClearWarmInstanceCache()
}

// Close will close all underlying processes
func (host *vmHost) Close() error {
	host.mutExecution.Lock()
	host.close()
	host.closingInstance = true
	host.mutExecution.Unlock()

	return nil
}

// Reset is a function which closes the VM and resets the closingInstance variable
func (host *vmHost) Reset() {
	host.mutExecution.Lock()
	host.close()
	// keep closingInstance flag to false
	host.mutExecution.Unlock()
}

func (host *vmHost) initContexts() {
	host.ClearContextStateStack()
	host.managedTypesContext.InitState()
	host.outputContext.InitState()
	host.meteringContext.InitState()
	host.runtimeContext.InitState()
	host.storageContext.InitState()
	host.blockchainContext.InitState()
}

// ClearContextStateStack cleans the state stacks of all the contexts of the host
func (host *vmHost) ClearContextStateStack() {
	host.managedTypesContext.ClearStateStack()
	host.outputContext.ClearStateStack()
	host.meteringContext.ClearStateStack()
	host.runtimeContext.ClearStateStack()
	host.storageContext.ClearStateStack()
	host.blockchainContext.ClearStateStack()
}

// GasScheduleChange applies a new gas schedule to the host
func (host *vmHost) GasScheduleChange(newGasSchedule config.GasScheduleMap) {
	host.mutExecution.Lock()
	defer host.mutExecution.Unlock()

	host.gasSchedule = newGasSchedule
	gasCostConfig, err := config.CreateGasConfig(newGasSchedule)
	if err != nil {
		log.Error("cannot apply new gas config", "err", err)
		return
	}

	host.runtimeContext.GetVMExecutor().SetOpcodeCosts(gasCostConfig.WASMOpcodeCost)

	host.meteringContext.SetGasSchedule(newGasSchedule)
	host.runtimeContext.ClearWarmInstanceCache()
}

// GetGasScheduleMap returns the currently stored gas schedule
func (host *vmHost) GetGasScheduleMap() config.GasScheduleMap {
	return host.gasSchedule
}

// RunSmartContractCreate executes the deployment of a new contract
func (host *vmHost) RunSmartContractCreate(input *vmcommon.ContractCreateInput) (vmOutput *vmcommon.VMOutput, err error) {
	err = validateVMInput(&input.VMInput)
	if err != nil {
		return nil, err
	}

	host.mutExecution.RLock()
	defer host.mutExecution.RUnlock()

	if host.closingInstance {
		return nil, vmhost.ErrVMIsClosing
	}

	host.setGasTracerEnabledIfLogIsTrace()
	ctx, cancel := context.WithTimeout(context.Background(), host.executionTimeout)
	defer cancel()

	log.Trace("RunSmartContractCreate begin",
		"len(code)", len(input.ContractCode),
		"metadata", input.ContractCodeMetadata,
		"gasProvided", input.GasProvided,
		"executionTimeout", host.executionTimeout)

	done := make(chan struct{})
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				log.Error("VM execution panicked", "error", r, "stack", "\n"+string(debug.Stack()))
				err = vmhost.ErrExecutionPanicked
				host.Runtime().CleanInstance()
			} else {
				host.Runtime().EndExecution()
			}

			close(done)
		}()

		vmOutput = host.doRunSmartContractCreate(input)
		host.CompleteLogEntriesWithCallType(vmOutput, vmhost.DeploySmartContractString)

		logsFromErrors := host.createLogEntryFromErrors(input.CallerAddr, input.CallerAddr, "_init")
		if logsFromErrors != nil {
			vmOutput.Logs = append(vmOutput.Logs, logsFromErrors)
		}

		log.Trace("RunSmartContractCreate end",
			"returnCode", vmOutput.ReturnCode,
			"returnMessage", vmOutput.ReturnMessage,
			"gasRemaining", vmOutput.GasRemaining)
		host.logFromGasTracer("init")
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		host.Runtime().FailExecution(vmhost.ErrExecutionFailedWithTimeout)
		<-done
		err = vmhost.ErrExecutionFailedWithTimeout
	}

	return
}

// RunSmartContractCall executes the call of an existing contract
func (host *vmHost) RunSmartContractCall(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, err error) {
	err = validateVMInput(&input.VMInput)
	if err != nil {
		return nil, err
	}

	host.mutExecution.RLock()
	defer host.mutExecution.RUnlock()

	if host.closingInstance {
		return nil, vmhost.ErrVMIsClosing
	}

	host.setGasTracerEnabledIfLogIsTrace()
	ctx, cancel := context.WithTimeout(context.Background(), host.executionTimeout)
	defer cancel()

	log.Trace("RunSmartContractCall begin",
		"function", input.Function,
		"gasProvided", input.GasProvided)

	done := make(chan struct{})
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				log.Error("VM execution panicked", "error", r, "stack", "\n"+string(debug.Stack()))
				err = vmhost.ErrExecutionPanicked
				host.Runtime().CleanInstance()
			} else {
				host.Runtime().EndExecution()
			}

			close(done)
		}()

		switch input.Function {
		case vmhost.UpgradeFunctionName:
			vmOutput = host.doRunSmartContractUpgrade(input)
		case vmhost.DeleteFunctionName:
			vmOutput = host.doRunSmartContractDelete(input)
		default:
			vmOutput = host.doRunSmartContractCall(input)
		}

		logsFromErrors := host.createLogEntryFromErrors(input.CallerAddr, input.RecipientAddr, input.Function)
		if logsFromErrors != nil {
			vmOutput.Logs = append(vmOutput.Logs, logsFromErrors)
		}

		log.Trace("RunSmartContractCall end",
			"function", input.Function,
			"returnCode", vmOutput.ReturnCode,
			"returnMessage", vmOutput.ReturnMessage,
			"gasRemaining", vmOutput.GasRemaining)
		host.logFromGasTracer(input.Function)
	}()

	select {
	case <-done:
		// Normal termination.
		return
	case <-ctx.Done():
		// Terminated due to timeout. The VM sets the `ExecutionFailed` breakpoint
		// in Wasmer. Also, the VM must wait for Wasmer to reach the end of a WASM
		// basic block in order to close the WASM instance cleanly. This is done by
		// reading the `done` channel once more, awaiting the call to `close(done)`
		// from above.
		host.Runtime().FailExecution(vmhost.ErrExecutionFailedWithTimeout)
		<-done
		err = vmhost.ErrExecutionFailedWithTimeout
	}

	return
}

func (host *vmHost) createLogEntryFromErrors(sndAddress, rcvAddress []byte, function string) *vmcommon.LogEntry {
	formattedErrors := host.runtimeContext.GetAllErrors()
	if formattedErrors == nil {
		return nil
	}

	logFromError := &vmcommon.LogEntry{
		Identifier: []byte(internalVMErrors),
		Address:    sndAddress,
		Topics:     [][]byte{rcvAddress, []byte(function)},
		Data:       [][]byte{[]byte(formattedErrors.Error())},
	}

	return logFromError
}

// IsInterfaceNil returns true if there is no value under the interface
func (host *vmHost) IsInterfaceNil() bool {
	return host == nil
}

// SetRuntimeContext sets the runtimeContext for this host, used in tests
func (host *vmHost) SetRuntimeContext(runtime vmhost.RuntimeContext) {
	host.runtimeContext = runtime
}

// GetRuntimeErrors obtains the cumultated error object after running the SC
func (host *vmHost) GetRuntimeErrors() error {
	if host.runtimeContext != nil {
		return host.runtimeContext.GetAllErrors()
	}
	return nil
}

// SetBuiltInFunctionsContainer sets the built in function container - only for testing
func (host *vmHost) SetBuiltInFunctionsContainer(builtInFuncs vmcommon.BuiltInFunctionContainer) {
	if check.IfNil(builtInFuncs) {
		return
	}
	host.builtInFuncContainer = builtInFuncs
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (host *vmHost) EpochConfirmed(epoch uint32) {
	_, ok := host.activationEpochMap[epoch]
	if ok {
		host.Runtime().ClearWarmInstanceCache()
		host.Blockchain().ClearCompiledCodes()
	}
}

func validateVMInput(vmInput *vmcommon.VMInput) error {
	if vmInput.GasProvided > math.MaxInt64 {
		return vmhost.ErrInvalidGasProvided
	}

	return nil
}

func (host *vmHost) setGasTracerEnabledIfLogIsTrace() {
	host.Metering().SetGasTracing(false)
	if logGasTrace.GetLevel() == logger.LogTrace {
		host.Metering().SetGasTracing(true)
	}
}

func (host *vmHost) logFromGasTracer(functionName string) {
	if logGasTrace.GetLevel() == logger.LogTrace {
		scGasTrace := host.meteringContext.GetGasTrace()
		totalGasUsedByAPIs := 0
		for scAddress, gasTrace := range scGasTrace {
			logGasTrace.Trace("Gas Trace for", "SC Address", scAddress, "function", functionName)
			for apiName, value := range gasTrace {
				totalGasUsed := uint64(0)
				for _, usedGas := range value {
					totalGasUsed += usedGas
				}
				logGasTrace.Trace("Gas Trace for", "apiName", apiName, "totalGasUsed", totalGasUsed, "numberOfCalls", len(value))
				totalGasUsedByAPIs += int(totalGasUsed) // #nosec G115
			}
			logGasTrace.Trace("Gas Trace for", "TotalGasUsedByAPIs", totalGasUsedByAPIs)
		}
	}
}

// CompleteLogEntriesWithCallType sets the call type on a log entry if it's not already filled
func (host *vmHost) CompleteLogEntriesWithCallType(vmOutput *vmcommon.VMOutput, callType string) {
	for _, logEntry := range vmOutput.Logs {
		_, containsId := host.transferLogIdentifiers[string(logEntry.Identifier)]
		if containsId && len(logEntry.Data) > 1 && len(logEntry.Data[0]) == 0 {
			logEntry.Data[0] = []byte(callType)
		}
	}
}
