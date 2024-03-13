package mock

import (
	"math/big"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// EconomicsHandlerStub -
type EconomicsHandlerStub struct {
	ComputeTransactionCostCalled           func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error)
	GetTransactionFeeCalled                func(tx process.TransactionWithFeeHandler) int64
	CheckValidityTxValuesCalled            func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error)
	EpochConfirmedCalled                   func(epoch uint32)
	LeaderPercentageCalled                 func() float64
	ProtocolSustainabilityPercentageCalled func() float64
	ProtocolSustainabilityAddressCalled    func() string
	MinInflationRateCalled                 func() float64
	MaxInflationRateCalled                 func(year uint32) float64
	RewardsTopUpGradientPointCalled        func() int64
	RewardsTopUpFactorCalled               func() float64
	GenesisTotalSupplyCalled               func() *big.Int
	TxSimulatorProcessor                   txsimulator.TransactionSimulatorProcessor
}

// GetTransactionFee computes the provided transaction's fee using enable from epoch approach
func (e *EconomicsHandlerStub) GetTransactionFee(tx process.TransactionWithFeeHandler) int64 {
	if e.GetTransactionFeeCalled != nil {
		return e.GetTransactionFee(tx)
	}
	return 0
}

// SetProposalController -
func (e *EconomicsHandlerStub) SetProposalController(controller kapps.ActiveProposalController) error {

	return nil
}

func (e *EconomicsHandlerStub) ComputeTransactionCost(tx process.TransactionWithFeeHandler, _ bool) (*transaction.CostResponse, error) {
	if e.ComputeTransactionCostCalled != nil {
		return e.ComputeTransactionCostCalled(tx)
	}
	return &transaction.CostResponse{}, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (e *EconomicsHandlerStub) EpochConfirmed(epoch uint32) {
}

// CheckValidityTxValues checks if the provided transaction is economically correct
func (e *EconomicsHandlerStub) CheckValidityTxValues(tx process.TransactionWithFeeHandler, _ bool) (*transaction.CostResponse, error) {
	if e.CheckValidityTxValuesCalled != nil {
		return e.CheckValidityTxValuesCalled(tx)
	}
	return &transaction.CostResponse{}, nil
}

// LeaderPercentage -
func (e *EconomicsHandlerStub) LeaderPercentage() float64 {
	if e.LeaderPercentageCalled != nil {
		return e.LeaderPercentageCalled()
	}
	return 0.0
}

// ProtocolSustainabilityPercentage -
func (e *EconomicsHandlerStub) ProtocolSustainabilityPercentage() float64 {
	if e.ProtocolSustainabilityAddressCalled != nil {
		return e.ProtocolSustainabilityPercentageCalled()
	}
	return 0.0
}

// ProtocolSustainabilityAddress -
func (e *EconomicsHandlerStub) ProtocolSustainabilityAddress() string {
	if e.ProtocolSustainabilityAddressCalled != nil {
		return e.ProtocolSustainabilityAddressCalled()
	}
	return ""
}

// MinInflationRate -
func (e *EconomicsHandlerStub) MinInflationRate() float64 {
	if e.MinInflationRateCalled != nil {
		return e.MinInflationRateCalled()
	}
	return 0.0
}

// MaxInflationRate -
func (e *EconomicsHandlerStub) MaxInflationRate(year uint32) float64 {
	if e.MaxInflationRateCalled != nil {
		return e.MaxInflationRateCalled(year)
	}
	return 0.0
}

// RewardsTopUpGradientPoint -
func (e *EconomicsHandlerStub) RewardsTopUpGradientPoint() int64 {
	if e.RewardsTopUpGradientPointCalled != nil {
		return e.RewardsTopUpGradientPointCalled()
	}

	return 0
}

// RewardsTopUpFactor -
func (e *EconomicsHandlerStub) RewardsTopUpFactor() float64 {
	if e.RewardsTopUpFactorCalled != nil {
		return e.RewardsTopUpFactorCalled()
	}

	return 0
}

func (e *EconomicsHandlerStub) GenesisTotalSupply() *big.Int {
	if e.GenesisTotalSupplyCalled != nil {
		return e.GenesisTotalSupplyCalled()
	}

	return big.NewInt(0)
}

// IsInterfaceNil returns true if there is no value under the interface
func (e *EconomicsHandlerStub) IsInterfaceNil() bool {
	return e == nil
}

func (e *EconomicsHandlerStub) ComputeGasLimit(tx data.TransactionHandler) uint64 {
	return 0
}

func (e *EconomicsHandlerStub) SetTXSimulatorProcessor(txSimulatorProcessor txsimulator.TransactionSimulatorProcessor) error {
	e.TxSimulatorProcessor = txSimulatorProcessor

	return nil
}
