package mock

import (
	"context"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

var _ vmhost.VMHost = (*VMHostStub)(nil)

// VMHostStub is used in tests to check the VMHost interface method calls
type VMHostStub struct {
	InitStateCalled       func()
	PushStateCalled       func()
	PopStateCalled        func()
	ClearStateStackCalled func()
	GetVersionCalled      func() string

	CryptoCalled         func() crypto.VMCrypto
	BlockchainCalled     func() vmhost.BlockchainContext
	RuntimeCalled        func() vmhost.RuntimeContext
	OutputCalled         func() vmhost.OutputContext
	MeteringCalled       func() vmhost.MeteringContext
	StorageCalled        func() vmhost.StorageContext
	ForkControllerCalled func() core.ForkController
	GetContextsCalled    func() (vmhost.ManagedTypesContext, vmhost.BlockchainContext, vmhost.MeteringContext, vmhost.OutputContext, vmhost.RuntimeContext, vmhost.StorageContext)
	ManagedTypesCalled   func() vmhost.ManagedTypesContext

	ExecuteKDATransferCalled    func(transfersArgs *vmhost.KDATransfersArgs, callType vm.CallType) (*vmcommon.VMOutput, uint64, error)
	CreateNewContractCalled     func(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error)
	ExecuteOnSameContextCalled  func(input *vmcommon.ContractCallInput) error
	ExecuteOnDestContextCalled  func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	IsBuiltinFunctionNameCalled func(functionName string) bool
	IsBuiltinFunctionCallCalled func(data []byte) bool

	RunSmartContractCallCalled           func(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, err error)
	RunSmartContractCreateCalled         func(input *vmcommon.ContractCreateInput) (vmOutput *vmcommon.VMOutput, err error)
	GetGasScheduleMapCalled              func() config.GasScheduleMap
	GasScheduleChangeCalled              func(newGasSchedule config.GasScheduleMap)
	IsInterfaceNilCalled                 func() bool
	CompleteLogEntriesWithCallTypeCalled func(vmOutput *vmcommon.VMOutput, callType string)

	SetRuntimeContextCalled            func(runtime vmhost.RuntimeContext)
	SetExecutionModeCalled             func(mode vmcommon.ExecutionMode)
	GetExecutionModeCalled             func() vmcommon.ExecutionMode
	SetBuiltInFunctionsContainerCalled func(builtInFuncs vmcommon.BuiltInFunctionContainer)
}

// GetVersion mocked method
func (vhs *VMHostStub) GetVersion() string {
	if vhs.GetVersionCalled != nil {
		return vhs.GetVersionCalled()
	}

	return "stub"
}

// SetExecutionMode mocked method
func (vhs *VMHostStub) SetExecutionMode(mode vmcommon.ExecutionMode) {
	if vhs.SetExecutionModeCalled != nil {
		vhs.SetExecutionModeCalled(mode)
	}
}

// GetExecutionMode mocked method
func (vhs *VMHostStub) GetExecutionMode() vmcommon.ExecutionMode {
	if vhs.GetExecutionModeCalled != nil {
		return vhs.GetExecutionModeCalled()
	}
	return vmcommon.ExecutionModeQuery
}

// InitState mocked method
func (vhs *VMHostStub) InitState() {
	if vhs.InitStateCalled != nil {
		vhs.InitStateCalled()
	}
}

func (vhs *VMHostStub) GetCallArgsParser() vmhost.CallArgsParser {
	return parsers.NewCallArgsParser()
}

// PushState mocked method
func (vhs *VMHostStub) PushState() {
	if vhs.PushStateCalled != nil {
		vhs.PushStateCalled()
	}
}

// PopState mocked method
func (vhs *VMHostStub) PopState() {
	if vhs.PopStateCalled != nil {
		vhs.PopStateCalled()
	}
}

// ClearStateStack mocked method
func (vhs *VMHostStub) ClearStateStack() {
	if vhs.ClearStateStackCalled != nil {
		vhs.ClearStateStackCalled()
	}
}

// Crypto mocked method
func (vhs *VMHostStub) Crypto() crypto.VMCrypto {
	if vhs.CryptoCalled != nil {
		return vhs.CryptoCalled()
	}
	return nil
}

// Blockchain mocked method
func (vhs *VMHostStub) Blockchain() vmhost.BlockchainContext {
	if vhs.BlockchainCalled != nil {
		return vhs.BlockchainCalled()
	}
	return nil
}

// Runtime mocked method
func (vhs *VMHostStub) Runtime() vmhost.RuntimeContext {
	if vhs.RuntimeCalled != nil {
		return vhs.RuntimeCalled()
	}
	return nil
}

// ManagedTypes mocked method
func (vhs *VMHostStub) ManagedTypes() vmhost.ManagedTypesContext {
	if vhs.ManagedTypesCalled != nil {
		return vhs.ManagedTypesCalled()
	}
	return nil
}

// IsAheadOfTimeCompileEnabled mocked method
func (vhs *VMHostStub) IsAheadOfTimeCompileEnabled() bool {
	return true
}

// IsDynamicGasLockingEnabled mocked method
func (vhs *VMHostStub) IsDynamicGasLockingEnabled() bool {
	return true
}

// IsKDAFunctionsEnabled mocked method
func (vhs *VMHostStub) IsKDAFunctionsEnabled() bool {
	return true
}

// Output mocked method
func (vhs *VMHostStub) Output() vmhost.OutputContext {
	if vhs.OutputCalled != nil {
		return vhs.OutputCalled()
	}
	return nil
}

// Metering mocked method
func (vhs *VMHostStub) Metering() vmhost.MeteringContext {
	if vhs.MeteringCalled != nil {
		return vhs.MeteringCalled()
	}
	return nil
}

// Storage mocked method
func (vhs *VMHostStub) Storage() vmhost.StorageContext {
	if vhs.StorageCalled != nil {
		return vhs.StorageCalled()
	}
	return nil
}

// ForkController mocked method
func (vhs *VMHostStub) ForkController() core.ForkController {
	if vhs.ForkControllerCalled != nil {
		return vhs.ForkControllerCalled()
	}
	return nil
}

// ExecuteKDATransfer mocked method
func (vhs *VMHostStub) ExecuteKDATransfer(transfersArgs *vmhost.KDATransfersArgs, callType vm.CallType) (*vmcommon.VMOutput, uint64, error) {
	if vhs.ExecuteKDATransferCalled != nil {
		return vhs.ExecuteKDATransferCalled(transfersArgs, callType)
	}
	return nil, 0, nil
}

// CreateNewContract mocked method
func (vhs *VMHostStub) CreateNewContract(input *vmcommon.ContractCreateInput, createContractCallType int) ([]byte, error) {
	if vhs.CreateNewContractCalled != nil {
		return vhs.CreateNewContractCalled(input, createContractCallType)
	}
	return nil, nil
}

// ExecuteOnSameContext mocked method
func (vhs *VMHostStub) ExecuteOnSameContext(input *vmcommon.ContractCallInput) error {
	if vhs.ExecuteOnSameContextCalled != nil {
		return vhs.ExecuteOnSameContextCalled(input)
	}
	return nil
}

// ExecuteOnDestContext mocked method
func (vhs *VMHostStub) ExecuteOnDestContext(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if vhs.ExecuteOnDestContextCalled != nil {
		return vhs.ExecuteOnDestContextCalled(input)
	}
	return nil, nil
}

// IsBuiltinFunctionName mocked method
func (vhs *VMHostStub) IsBuiltinFunctionName(functionName string) bool {
	if vhs.IsBuiltinFunctionNameCalled != nil {
		return vhs.IsBuiltinFunctionNameCalled(functionName)
	}
	return false
}

// IsBuiltinFunctionCall mocked method
func (vhs *VMHostStub) IsBuiltinFunctionCall(data []byte) bool {
	if vhs.IsBuiltinFunctionCallCalled != nil {
		return vhs.IsBuiltinFunctionCallCalled(data)
	}
	return false
}

// GetGasScheduleMap returns the currently stored gas schedule
func (vhs *VMHostStub) GetGasScheduleMap() config.GasScheduleMap {
	if vhs.GetGasScheduleMapCalled != nil {
		return vhs.GetGasScheduleMapCalled()
	}
	return nil
}

// RunSmartContractCall mocked method
func (vhs *VMHostStub) RunSmartContractCall(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, err error) {
	if vhs.RunSmartContractCallCalled != nil {
		return vhs.RunSmartContractCallCalled(input)
	}
	return nil, nil
}

// RunSmartContractCreate mocked method
func (vhs *VMHostStub) RunSmartContractCreate(input *vmcommon.ContractCreateInput) (vmOutput *vmcommon.VMOutput, err error) {
	if vhs.RunSmartContractCreateCalled != nil {
		return vhs.RunSmartContractCreateCalled(input)
	}
	return nil, nil
}

// GasScheduleChange mocked method
func (vhs *VMHostStub) GasScheduleChange(newGasSchedule config.GasScheduleMap) {
	if vhs.GasScheduleChangeCalled != nil {
		vhs.GasScheduleChangeCalled(newGasSchedule)
	}
}

// SetBuiltInFunctionsContainer mocked method
func (vhs *VMHostStub) SetBuiltInFunctionsContainer(builtInFuncs vmcommon.BuiltInFunctionContainer) {
	if vhs.SetBuiltInFunctionsContainerCalled != nil {
		vhs.SetBuiltInFunctionsContainerCalled(builtInFuncs)
	}
}

// IsInterfaceNil mocked method
func (vhs *VMHostStub) IsInterfaceNil() bool {
	if vhs.IsInterfaceNilCalled != nil {
		return vhs.IsInterfaceNilCalled()
	}
	return false
}

// GetContexts mocked method
func (vhs *VMHostStub) GetContexts() (
	vmhost.ManagedTypesContext,
	vmhost.BlockchainContext,
	vmhost.MeteringContext,
	vmhost.OutputContext,
	vmhost.RuntimeContext,
	vmhost.StorageContext,
) {
	if vhs.GetContextsCalled != nil {
		return vhs.GetContextsCalled()
	}
	return nil, nil, nil, nil, nil, nil
}

// SetRuntimeContext mocked method
func (vhs *VMHostStub) SetRuntimeContext(runtime vmhost.RuntimeContext) {
	if vhs.SetRuntimeContextCalled != nil {
		vhs.SetRuntimeContextCalled(runtime)
	}
}

// GetExecutionContext mocked method
func (vhs *VMHostStub) GetExecutionContext() context.Context {
	return nil
}

// CompleteLogEntriesWithCallType mocked method
func (vhs *VMHostStub) CompleteLogEntriesWithCallType(vmOutput *vmcommon.VMOutput, callType string) {
	if vhs.CompleteLogEntriesWithCallTypeCalled != nil {
		vhs.CompleteLogEntriesWithCallTypeCalled(vmOutput, callType)
	}
}

// Close -
func (vhs *VMHostStub) Close() error {
	return nil
}

// Reset -
func (vhs *VMHostStub) Reset() {
}
