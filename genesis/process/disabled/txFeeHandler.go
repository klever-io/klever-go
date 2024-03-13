package disabled

import (
	"github.com/klever-io/klever-go/core/process"
)

var _ process.TransactionFeeHandler = (*TXFeeHandler)(nil)

type TXFeeHandler struct {
}

// CreateBlockStarted does the cleanup before creating a new block
func (f *TXFeeHandler) CreateBlockStarted() {
}

// GetAccumulatedTxFees returns the total accumulated fees for TX
func (f *TXFeeHandler) GetAccumulatedTxFees() int64 {
	return 0
}

// GetAccumulatedKAppFees returns the total accumulated fees for KApp
func (f *TXFeeHandler) GetAccumulatedKAppFees() int64 {
	return 0
}

// ProcessTransactionFee adds the tx cost to the accumulated amount
func (f *TXFeeHandler) ProcessTransactionFee(txCost int64, kAppCost int64, txHash []byte) {
}

// RevertFees reverts the accumulated fees for txHashes
func (f *TXFeeHandler) RevertFees(txHashes [][]byte) {
}

// RevertTransactionFee reverts the accumulated fees for txHashes
func (f *TXFeeHandler) RevertTransactionFee(txHash []byte, txCost int64, kAppCost int64) {
}

// IsInterfaceNil returns true if there is no value under the interface
func (f *TXFeeHandler) IsInterfaceNil() bool {
	return f == nil
}
