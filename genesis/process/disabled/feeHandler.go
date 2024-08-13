package disabled

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// FeeHandler represents a disabled fee handler implementation
type FeeHandler struct {
}

// HandleProposal returns nil
func (fh *FeeHandler) SetProposalController(controller kapps.ActiveProposalController) error {
	return nil
}

// SetTXSimulatorProcessor will load the tx simulator processor into ed instance
func (fh *FeeHandler) SetTXSimulatorProcessor(txSimulatorProcessor txsimulator.TransactionSimulatorProcessor) error {
	return nil
}

// EstimateTransactionGas will calculate how many gas units a transaction will consume
func (fh *FeeHandler) ComputeTransactionCost(_ process.TransactionWithFeeHandler, _ bool) (*transaction.CostResponse, error) {
	return &transaction.CostResponse{}, nil
}

func (fh *FeeHandler) CheckValidityTxValues(_ process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
	return &transaction.CostResponse{}, nil
}

func (fh *FeeHandler) ComputeGasLimit(tx data.TransactionHandler) uint64 {
	return 0
}

// LeaderPercentage returns the leader percentage
func (fh *FeeHandler) LeaderPercentage() float64 {
	return 0
}

// IsInterfaceNil returns true if there is no value under the interface
func (fh *FeeHandler) IsInterfaceNil() bool {
	return fh == nil
}
