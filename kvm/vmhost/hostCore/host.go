package hostCore

import (
	"context"
	"errors"
	"math"
	"runtime/debug"
	"sync"
	"time"

	executorwrapper "github.com/klever-io/klever-go/kvm/executor/wrapper"

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

const (
	minExecutionTimeout = time.Millisecond * 400
	internalVMErrors    = "internalVMErrors"

	// importDBExecutionTimeout is used in ExecutionModeReplay (import-db replay). It is
	// excessively permissive for any real call so that machine-speed differences never turn a
	// network-accepted tx into a spurious timeout failure. Replay execution is bounded by gas,
	// so a genuine infinite loop fails on out-of-gas rather than hanging for this long.
	importDBExecutionTimeout = time.Second * 5
)

// vmHost implements HostContext interface.
type vmHost struct {
	cryptoHook       crypto.VMCrypto
	mutExecution     sync.RWMutex
	closingInstance  bool
	executionTimeout time.Duration
	executionContext context.Context        // Per-execution timeout context
	executionMode    vmcommon.ExecutionMode // Current execution mode of the host
	toleranceTimeout time.Duration          // Timeout with tolerance for cosigner validation

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
	if err := validateHostParameters(blockChainHook, hostParameters); err != nil {
		return nil, err
	}

	host := createBaseHost(hostParameters)
	configureTimeouts(host, hostParameters)

	if err := initializeHostContexts(host, blockChainHook, hostParameters); err != nil {
		return nil, err
	}

	finalizeHostSetup(host, hostParameters)

	return host, nil
}

// validateHostParameters validates all required parameters for VM host creation
func validateHostParameters(blockChainHook vmcommon.BlockchainHook, hostParameters *vmhost.VMHostParameters) error {
	if check.IfNil(blockChainHook) {
		return vmhost.ErrNilBlockChainHook
	}
	if hostParameters == nil {
		return vmhost.ErrNilHostParameters
	}
	if check.IfNil(hostParameters.KDATransferParser) {
		return vmhost.ErrNilKDATransferParser
	}
	if check.IfNil(hostParameters.BuiltInFuncContainer) {
		return vmhost.ErrNilBuiltInFunctionsContainer
	}
	if check.IfNil(hostParameters.EpochNotifier) {
		return vmhost.ErrNilEpochNotifier
	}
	if check.IfNil(hostParameters.ForkController) {
		return vmhost.ErrNilEnableEpochsHandler
	}
	if check.IfNil(hostParameters.Hasher) {
		return vmhost.ErrNilHasher
	}
	if hostParameters.VMType == nil {
		return vmhost.ErrNilVMType
	}
	return nil
}

// createBaseHost creates the base vmHost struct with basic configuration
func createBaseHost(hostParameters *vmhost.VMHostParameters) *vmHost {
	cryptoHook := factory.NewVMCrypto()
	return &vmHost{
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
		executionMode:        hostParameters.ExecutionMode,
	}
}

// configureTimeouts sets up execution and tolerance timeouts for the VM host
func configureTimeouts(host *vmHost, hostParameters *vmhost.VMHostParameters) {
	newExecutionTimeout := time.Duration(hostParameters.TimeOutForSCExecutionInMilliseconds) * time.Millisecond
	if newExecutionTimeout > minExecutionTimeout {
		host.executionTimeout = newExecutionTimeout
	}

	// Calculate tolerance timeout for cosigner validation
	// Leader uses executionTimeout, cosigners use toleranceTimeout
	tolerancePercentage := hostParameters.TimeOutTolerancePercentage
	if tolerancePercentage == 0 {
		tolerancePercentage = core.DefaultTolerancePercentage
	}
	if tolerancePercentage > 100 {
		log.Warn("TimeOutTolerancePercentage exceeds 100%, using 100%",
			"configured", tolerancePercentage)
		tolerancePercentage = 100
	}
	additionalTime := (host.executionTimeout * time.Duration(tolerancePercentage)) / 100
	host.toleranceTimeout = host.executionTimeout + additionalTime
}

// initializeHostContexts creates and initializes all VM host contexts
func initializeHostContexts(host *vmHost, blockChainHook vmcommon.BlockchainHook, hostParameters *vmhost.VMHostParameters) error {
	var err error

	host.blockchainContext, err = contexts.NewBlockchainContext(host, blockChainHook)
	if err != nil {
		return err
	}

	vmExecutor, err := host.createExecutor(hostParameters)
	if err != nil {
		return err
	}

	host.runtimeContext, err = contexts.NewRuntimeContext(
		host,
		hostParameters.VMType,
		host.builtInFuncContainer,
		vmExecutor,
		hostParameters.Hasher,
	)
	if err != nil {
		return err
	}

	host.meteringContext, err = contexts.NewMeteringContext(host, hostParameters.GasSchedule)
	if err != nil {
		return err
	}

	host.outputContext, err = contexts.NewOutputContext(host)
	if err != nil {
		return err
	}

	host.storageContext, err = contexts.NewStorageContext(
		host,
		blockChainHook,
		hostParameters.ProtectedKeyPrefix,
	)
	if err != nil {
		return err
	}

	host.managedTypesContext, err = contexts.NewManagedTypesContext(host)
	if err != nil {
		return err
	}

	return nil
}

// finalizeHostSetup completes the VM host setup with final configurations
func finalizeHostSetup(host *vmHost, hostParameters *vmhost.VMHostParameters) {
	host.runtimeContext.SetMaxInstanceStackSize(MaximumRuntimeInstanceStackSize)
	host.initContexts()
	hostParameters.EpochNotifier.RegisterNotifyHandler(host)
	initializeTransferLogIdentifiers(host)
}

// initializeTransferLogIdentifiers sets up transfer log identifier mappings
func initializeTransferLogIdentifiers(host *vmHost) {
	host.transferLogIdentifiers = make(map[string]bool)
	host.transferLogIdentifiers["transferValueOnly"] = true
	host.transferLogIdentifiers["KleverTransfer"] = true
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
		ExecutionTimeout:         host.executionTimeout,
	}

	wrappedExecutorFactory := executorwrapper.SimpleWrappedExecutorFactory(vmExecutorFactory)
	return wrappedExecutorFactory.CreateExecutor(vmExecutorFactoryArgs)
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

// SetExecutionMode sets the execution mode for the VM host
// This determines which timeout will be used during smart contract execution
func (host *vmHost) SetExecutionMode(mode vmcommon.ExecutionMode) {
	host.mutExecution.Lock()
	defer host.mutExecution.Unlock()
	log.Debug("vmHost SetExecutionMode", "mode", mode)
	host.executionMode = mode
}

// GetExecutionMode retrieves the current execution mode
func (host *vmHost) GetExecutionMode() vmcommon.ExecutionMode {
	host.mutExecution.RLock()
	defer host.mutExecution.RUnlock()
	return host.executionMode
}

// getEffectiveTimeout returns the timeout to use based on the current execution mode
func (host *vmHost) getEffectiveTimeout() time.Duration {
	switch host.executionMode {
	case vmcommon.ExecutionModeLeader:
		return host.executionTimeout // Base timeout (e.g., 500ms)
	case vmcommon.ExecutionModeValidator:
		return host.toleranceTimeout // Base + tolerance (e.g., 575ms with 15% tolerance)
	case vmcommon.ExecutionModeQuery:
		return host.executionTimeout // Use base timeout for queries
	case vmcommon.ExecutionModeReplay:
		return importDBExecutionTimeout // import-db replay: excessively permissive timeout (gas-bounded)
	default:
		// Default to base timeout for unknown modes
		return host.executionTimeout
	}
}

// setupExecutionContext initializes the timeout context and sets up cleanup.
// Returns the context and a cleanup function that should be deferred.
func (host *vmHost) setupExecutionContext() (context.Context, func()) {
	host.setGasTracerEnabledIfLogIsTrace()
	effectiveTimeout := host.getEffectiveTimeout()

	log.Trace("setupExecutionContext",
		"executionTimeout", effectiveTimeout,
		"executionMode", host.executionMode,
	)

	// Create main execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), effectiveTimeout)

	// Hook context has no timeout - manually cancelled after FailExecution()
	// This ensures SetBreakpointValue() completes before hooks can panic and call CleanInstance()
	ctxHook, cancelHook := context.WithCancel(context.Background())

	// Set execution context on vmHost instance (not global) for timeout protection
	// This ensures each RunSmartContractCreate invocation has its own isolated context
	// preventing race conditions from parallel queries and context overwrite from nested calls
	host.executionContext = ctxHook

	cleanup := func() {
		cancelHook()
		cancel()
		host.executionContext = nil
	}

	return ctx, cleanup
}

// handleTimeout processes timeout during contract execution.
// It ensures SetBreakpointValue() completes before hooks can panic and call CleanInstance().
// This sequential execution prevents race conditions.
func (host *vmHost) handleTimeout(cancelHook context.CancelFunc, done <-chan struct{}) error {
	// Timeout detected. Set breakpoint first, then cancel hooks.
	// Sequential execution ensures SetBreakpointValue() completes before
	// any hook can panic and call CleanInstance(), preventing race condition.
	host.Runtime().FailExecution(vmhost.ErrExecutionFailedWithTimeout)

	// Now safe to cancel hook context - FailExecution() has completed
	cancelHook()

	// Wait for execution to complete cleanup
	<-done
	return vmhost.ErrExecutionFailedWithTimeout
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

	ctx, cancel := host.setupExecutionContext()
	defer cancel()

	log.Trace("RunSmartContractCreate begin",
		"len(code)", len(input.ContractCode),
		"metadata", input.ContractCodeMetadata,
		"gasProvided", input.GasProvided,
	)

	// Track execution time
	startTime := time.Now()
	defer func() {
		elapsedTime := time.Since(startTime)
		log.Trace("RunSmartContractCreate execTime",
			"time [ms]", elapsedTime.Milliseconds(),
			"timedOut", errors.Is(err, vmhost.ErrExecutionFailedWithTimeout))
	}()

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
		// Normal termination
		return
	case <-ctx.Done():
		err = host.handleTimeout(cancel, done)
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

	ctx, cancel := host.setupExecutionContext()
	defer cancel()

	log.Trace("RunSmartContractCall begin",
		"function", input.Function,
		"gasProvided", input.GasProvided,
	)

	// Track execution time
	startTime := time.Now()
	defer func() {
		elapsedTime := time.Since(startTime)
		log.Trace("RunSmartContractCall execTime",
			"function", input.Function,
			"time [ms]", elapsedTime.Milliseconds(),
			"timedOut", errors.Is(err, vmhost.ErrExecutionFailedWithTimeout))
	}()

	done := make(chan struct{})
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				if r != vmhost.ErrExecutionFailedWithTimeout {
					log.Error("VM execution panicked", "error", r, "stack", "\n"+string(debug.Stack()))
					err = vmhost.ErrExecutionPanicked
				} else {
					err = vmhost.ErrExecutionFailedWithTimeout
				}
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
	case <-ctx.Done():
		err = host.handleTimeout(cancel, done)
	}

	return vmOutput, err
}

func (host *vmHost) createLogEntryFromErrors(sndAddress, rcvAddress []byte, function string) *vmcommon.LogEntry {
	formattedErrors := host.runtimeContext.GetAllErrors()
	if formattedErrors == nil {
		return nil
	}

	logFromError := &vmcommon.LogEntry{
		Identifier:  []byte(internalVMErrors),
		Address:     sndAddress,
		Topics:      [][]byte{rcvAddress, []byte(function)},
		Data:        [][]byte{[]byte(formattedErrors.Error())},
		IsSystemLog: true,
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

// GetExecutionContext returns the execution context for timeout protection
func (host *vmHost) GetExecutionContext() context.Context {
	return host.executionContext
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
