package mock

import (
	"github.com/klever-io/klever-go/core/process"
)

// FeeHandlerStub -
type FeeHandlerStub struct {
	SetMaxGasLimitPerBlockCalled func(maxGasLimitPerBlock uint64)
	MaxGasLimitPerBlockCalled    func() uint64
	ComputeGasLimitCalled        func(tx process.TransactionCoordinator) uint64
	GetTransactionFeeCalled      func(tx process.TransactionCoordinator) int64
	CheckValidityTxValuesCalled  func(tx process.TransactionCoordinator) error
	DeveloperPercentageCalled    func() float64
	GenesisTotalSupplyCalled     func() int64
}

// DeveloperPercentage -
func (fhs *FeeHandlerStub) DeveloperPercentage() float64 {
	if fhs.DeveloperPercentageCalled != nil {
		return fhs.DeveloperPercentageCalled()
	}

	return 0.0
}

// SetMaxGasLimitPerBlock -
func (fhs *FeeHandlerStub) SetMaxGasLimitPerBlock(maxGasLimitPerBlock uint64) {
	fhs.SetMaxGasLimitPerBlockCalled(maxGasLimitPerBlock)
}

// MaxGasLimitPerBlock -
func (fhs *FeeHandlerStub) MaxGasLimitPerBlock(uint32) uint64 {
	return fhs.MaxGasLimitPerBlockCalled()
}

// ComputeGasLimit -
func (fhs *FeeHandlerStub) ComputeGasLimit(tx process.TransactionCoordinator) uint64 {
	if fhs.ComputeGasLimitCalled != nil {
		return fhs.ComputeGasLimitCalled(tx)
	}
	return 0
}

// GetTransactionFee -
func (fhs *FeeHandlerStub) GetTransactionFee(tx process.TransactionCoordinator) int64 {
	if fhs.GetTransactionFeeCalled != nil {
		return fhs.GetTransactionFeeCalled(tx)
	}
	return 0
}

// GenesisTotalSupply -
func (fhs *FeeHandlerStub) GenesisTotalSupply() int64 {
	if fhs.GenesisTotalSupplyCalled != nil {
		return fhs.GenesisTotalSupplyCalled()
	}

	return 0
}

// CheckValidityTxValues -
func (fhs *FeeHandlerStub) CheckValidityTxValues(tx process.TransactionCoordinator) error {
	if fhs.CheckValidityTxValuesCalled != nil {
		return fhs.CheckValidityTxValuesCalled(tx)
	}
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (fhs *FeeHandlerStub) IsInterfaceNil() bool {
	return fhs == nil
}
