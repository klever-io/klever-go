package mock

// FeeAccumulatorStub is a stub which implements TransactionFeeHandler interface
type FeeAccumulatorStub struct {
	CreateBlockStartedCalled     func()
	GetAccumulatedTxFeesCalled   func() int64
	GetAccumulatedKAppFeesCalled func() int64
	ProcessTransactionFeeCalled  func(txCost int64, kAppCost int64, hash []byte)
	RevertFeesCalled             func(txHashes [][]byte)
	RevertTransactionFeeCalled   func(txHash []byte, txCost int64, kAppCost int64)
}

// RevertFees -
func (f *FeeAccumulatorStub) RevertFees(txHashes [][]byte) {
	if f.RevertFeesCalled != nil {
		f.RevertFeesCalled(txHashes)
	}
}

// RevertFees -
func (f *FeeAccumulatorStub) RevertTransactionFee(txHash []byte, txCost int64, kAppCost int64) {
	if f.RevertFeesCalled != nil {
		f.RevertTransactionFeeCalled(txHash, txCost, kAppCost)
	}
}

// CreateBlockStarted -
func (f *FeeAccumulatorStub) CreateBlockStarted() {
	if f.CreateBlockStartedCalled != nil {
		f.CreateBlockStartedCalled()
	}
}

// GetAccumulatedTxFees -
func (f *FeeAccumulatorStub) GetAccumulatedTxFees() int64 {
	if f.GetAccumulatedTxFeesCalled != nil {
		return f.GetAccumulatedTxFeesCalled()
	}
	return 0
}

// GetAccumulatedKAppFees -
func (f *FeeAccumulatorStub) GetAccumulatedKAppFees() int64 {
	if f.GetAccumulatedKAppFeesCalled != nil {
		return f.GetAccumulatedKAppFeesCalled()
	}
	return 0
}

// ProcessTransactionFee -
func (f *FeeAccumulatorStub) ProcessTransactionFee(txCost int64, kAppCost int64, txHash []byte) {
	if f.ProcessTransactionFeeCalled != nil {
		f.ProcessTransactionFeeCalled(txCost, kAppCost, txHash)
	}
}

// IsInterfaceNil -
func (f *FeeAccumulatorStub) IsInterfaceNil() bool {
	return f == nil
}
