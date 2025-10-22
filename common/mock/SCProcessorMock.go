package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/vmcommon"
)

type SCProcessorMock struct {
	ExecuteSmartContractTransactionCalled func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error)
	DeploySmartContractCalled             func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error)
	ProcessIfErrorCalled                  func(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error
	IsPayableCalled                       func(sndAddress []byte, recvAddress []byte) (bool, error)
}

func (s *SCProcessorMock) ExecuteSmartContractTransaction(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
	if s.ExecuteSmartContractTransactionCalled != nil {
		return s.ExecuteSmartContractTransactionCalled(ctx, tc, acntSrc, acntDst)
	}

	return vmcommon.Ok, nil
}

func (s *SCProcessorMock) DeploySmartContract(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
	if s.DeploySmartContractCalled != nil {
		return s.DeploySmartContractCalled(ctx, tc)
	}

	return vmcommon.Ok, nil
}

func (s *SCProcessorMock) ProcessIfError(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error {
	if s.ProcessIfErrorCalled != nil {
		return s.ProcessIfErrorCalled(ctx, tc, returnCode, returnMessage)
	}

	return nil
}

func (s *SCProcessorMock) IsPayable(sndAddress []byte, recvAddress []byte) (bool, error) {
	if s.IsPayableCalled != nil {
		return s.IsPayableCalled(sndAddress, recvAddress)
	}

	return true, nil
}

// LastBlock returns the last committed block
func (s *SCProcessorMock) LastBlock() data.HeaderHandler {
	return &block.Block{}
}

func (s *SCProcessorMock) IsInterfaceNil() bool {
	return s == nil
}

// SetVMExecutionMode sets the execution mode
func (s *SCProcessorMock) SetVMExecutionMode(mode vmcommon.ExecutionMode) {
	// No-op for mock
}

// GetVMExecutionMode gets the execution mode
func (s *SCProcessorMock) GetVMExecutionMode() vmcommon.ExecutionMode {
	return vmcommon.ExecutionModeQuery
}
