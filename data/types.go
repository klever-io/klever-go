package data

import "github.com/klever-io/klever-go/data/transaction"

type TransactionWithResults struct {
	*transaction.Transaction
	Log *LogData
}

// New TransactionWithResults
func NewTransactionWithResults(tx *transaction.Transaction) *TransactionWithResults {
	return &TransactionWithResults{Transaction: tx, Log: nil}
}

// LogData holds the data needed for indexing logs and events
type LogData struct {
	LogHandler
	TxHash string
}
