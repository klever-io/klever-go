package txcache

import (
	"bytes"

	"github.com/klever-io/klever-go/data"
)

// WrappedTransaction contains a transaction, its hash and extra information
type WrappedTransaction struct {
	Tx                   data.TransactionHandler
	TxHash               []byte
	Size                 int64
	TxFeeScoreNormalized uint64
	ExpireOn             int64
}

func (wrappedTx *WrappedTransaction) sameAs(another *WrappedTransaction) bool {
	return bytes.Equal(wrappedTx.TxHash, another.TxHash)
}

// estimateTxGas returns an approximation for the necessary computation units (gas units)
func estimateTxGas(tx *WrappedTransaction) uint64 {
	return uint64(tx.Tx.GetTotalFees())
}
