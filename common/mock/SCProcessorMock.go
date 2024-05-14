package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/vmcommon"
)

type SCProcessorMock struct {
	ExecuteSmartContractTransactionCalled func(ctx kapp.KappContext, tx data.TransactionHandler, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error)
	DeploySmartContractCalled             func(ctx kapp.KappContext, tx data.TransactionHandler, tc data.SmartContractHandler, acntSrc state.UserAccountHandler) (vmcommon.ReturnCode, error)
	ProcessIfErrorCalled                  func(acntSnd state.UserAccountHandler, txHash []byte, tx data.TransactionHandler, tc data.SmartContractHandler, contractID int, returnCode string, returnMessage []byte) error
	IsPayableCalled                       func(sndAddress []byte, recvAddress []byte) (bool, error)
}

func (s *SCProcessorMock) ExecuteSmartContractTransaction(ctx kapp.KappContext, tx data.TransactionHandler, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
	if s.ExecuteSmartContractTransactionCalled != nil {
		return s.ExecuteSmartContractTransactionCalled(ctx, tx, tc, acntSrc, acntDst)
	}

	return vmcommon.Ok, nil
}

func (s *SCProcessorMock) DeploySmartContract(ctx kapp.KappContext, tx data.TransactionHandler, tc data.SmartContractHandler, acntSrc state.UserAccountHandler) (vmcommon.ReturnCode, error) {
	if s.DeploySmartContractCalled != nil {
		return s.DeploySmartContractCalled(ctx, tx, tc, acntSrc)
	}

	return vmcommon.Ok, nil
}

func (s *SCProcessorMock) ProcessIfError(acntSnd state.UserAccountHandler, txHash []byte, tx data.TransactionHandler, tc data.SmartContractHandler, contractID int, returnCode string, returnMessage []byte) error {
	if s.ProcessIfErrorCalled != nil {
		return s.ProcessIfErrorCalled(acntSnd, txHash, tx, tc, contractID, returnCode, returnMessage)
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
