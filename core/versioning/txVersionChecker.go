package versioning

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
)

// TxVersionChecker represents transaction option decoder
type txVersionChecker struct {
	minTxVersion uint32
}

// NewTxVersionChecker will create a new instance of TxOptionsChecker
func NewTxVersionChecker(minTxVersion uint32) *txVersionChecker {
	return &txVersionChecker{
		minTxVersion: minTxVersion,
	}
}

// CheckTxVersion will check transaction version
func (tvc *txVersionChecker) CheckTxVersion(tx *transaction.Transaction) error {
	if tx.RawData.Version < tvc.minTxVersion {
		return process.ErrInvalidTransactionVersion
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (tvc *txVersionChecker) IsInterfaceNil() bool {
	return tvc == nil
}
