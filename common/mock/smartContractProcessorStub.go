package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/vmcommon"
)

// SmartContractProcessorStub is a stub implementation of the SmartContractProcessor interface
type SmartContractProcessorStub struct {
	ExecuteSmartContractTransactionCalled func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error)
	DeploySmartContractCalled             func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error)
	ProcessIfErrorCalled                  func(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error
	IsPayableCalled                       func(sndAddress []byte, recvAddress []byte) (bool, error)
	LastBlockCalled                       func() data.HeaderHandler
	IsInterfaceNilCalled                  func() bool
}

// ExecuteSmartContractTransaction is the stub implementation for ExecuteSmartContractTransaction
func (stub *SmartContractProcessorStub) ExecuteSmartContractTransaction(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
	if stub.ExecuteSmartContractTransactionCalled != nil {
		return stub.ExecuteSmartContractTransactionCalled(ctx, tc, acntSrc, acntDst)
	}
	return vmcommon.Ok, nil
}

// DeploySmartContract is the stub implementation for DeploySmartContract
func (stub *SmartContractProcessorStub) DeploySmartContract(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
	if stub.DeploySmartContractCalled != nil {
		return stub.DeploySmartContractCalled(ctx, tc)
	}
	return vmcommon.Ok, nil
}

// ProcessIfError is the stub implementation for ProcessIfError
func (stub *SmartContractProcessorStub) ProcessIfError(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error {
	if stub.ProcessIfErrorCalled != nil {
		return stub.ProcessIfErrorCalled(ctx, tc, returnCode, returnMessage)
	}
	return nil
}

// IsPayable is the stub implementation for IsPayable
func (stub *SmartContractProcessorStub) IsPayable(sndAddress []byte, recvAddress []byte) (bool, error) {
	if stub.IsPayableCalled != nil {
		return stub.IsPayableCalled(sndAddress, recvAddress)
	}
	return true, nil
}

// LastBlock is the stub implementation for LastBlock
func (stub *SmartContractProcessorStub) LastBlock() data.HeaderHandler {
	if stub.LastBlockCalled != nil {
		return stub.LastBlockCalled()
	}
	return nil
}

// IsInterfaceNil is the stub implementation for IsInterfaceNil
func (stub *SmartContractProcessorStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}
	return false
}

// SetVMExecutionMode is the stub implementation for SetVMExecutionMode
func (stub *SmartContractProcessorStub) SetVMExecutionMode(mode vmcommon.ExecutionMode) {
	// No-op for stub
}

// GetVMExecutionMode is the stub implementation for GetVMExecutionMode
func (stub *SmartContractProcessorStub) GetVMExecutionMode() vmcommon.ExecutionMode {
	return vmcommon.ExecutionModeQuery
}

// NewSmartContractProcessorStub creates a new instance of SmartContractProcessorStub
func NewSmartContractProcessorStub() *SmartContractProcessorStub {
	return &SmartContractProcessorStub{}
}
