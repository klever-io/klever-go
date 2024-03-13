package converters

import (
	"github.com/klever-io/klever-go/indexer/data"
)

// ConvertTxsSliceIntoMap will convert the slice of the provided transactions into a map where the key represents the hash of the transaction and the value is the transaction
func ConvertTxsSliceIntoMap(txs []*data.Transaction) map[string]*data.Transaction {
	mapTxs := make(map[string]*data.Transaction, len(txs))

	for _, tx := range txs {
		mapTxs[tx.Hash] = tx
	}

	return mapTxs
}

func ConvertMapTxsToSlice(txs map[string]*data.Transaction) []*data.Transaction {
	transactions := make([]*data.Transaction, len(txs))
	i := 0
	for _, tx := range txs {
		transactions[i] = tx
		i++
	}
	return transactions
}
