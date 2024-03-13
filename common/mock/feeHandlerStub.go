package mock

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// FeeHandlerStub -
type FeeHandlerStub struct {
	CheckValidityTxValuesCalled  func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error)
	ComputeTransactionCostCalled func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error)
	EpochConfirmedCalled         func(epoch uint32)
}

// ComputeTransactionCost
func (fhs *FeeHandlerStub) ComputeTransactionCost(tx process.TransactionWithFeeHandler, _ bool) (*transaction.CostResponse, error) {
	if fhs.ComputeTransactionCostCalled != nil {
		return fhs.ComputeTransactionCostCalled(tx)
	}
	return &transaction.CostResponse{}, nil
}

func (fhs *FeeHandlerStub) SetProposalController(controller kapps.ActiveProposalController) error {
	return nil
}

func (fhs *FeeHandlerStub) SetTXSimulatorProcessor(txSimulatorProcessor txsimulator.TransactionSimulatorProcessor) error {
	return nil
}

// CheckValidityTxValues -
func (fhs *FeeHandlerStub) CheckValidityTxValues(tx process.TransactionWithFeeHandler, _ bool) (*transaction.CostResponse, error) {
	if fhs.CheckValidityTxValuesCalled != nil {
		return fhs.CheckValidityTxValuesCalled(tx)
	}
	return &transaction.CostResponse{}, nil
}

// EpochConfirmed -
func (fhs *FeeHandlerStub) EpochConfirmed(epoch uint32) {
	if fhs.EpochConfirmedCalled != nil {
		fhs.EpochConfirmedCalled(epoch)
	}
}

func (fhs *FeeHandlerStub) ComputeGasLimit(tx data.TransactionHandler) uint64 {
	return 0
}

// LeaderPercentage returns the leader percentage
func (fhs *FeeHandlerStub) LeaderPercentage() float64 {
	return 0
}

// IsInterfaceNil returns true if there is no value under the interface
func (fhs *FeeHandlerStub) IsInterfaceNil() bool {
	return fhs == nil
}
