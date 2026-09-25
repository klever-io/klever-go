package data

// PreparedBlockData carries the result of prepareTransactionsForDatabase
// shared between the websocket dispatcher and the elastic worker.
type PreparedBlockData struct {
	Txs     []*Transaction
	TxsMap  map[string]*Transaction
	Altered *AlteredData
	// LogsResults and LogsDB, when set, are the websocket dispatcher's own
	// ExtractDataFromLogs/PrepareLogsForDB results for this block, computed
	// synchronously on the commit goroutine and reused by the elastic worker instead of
	// recomputing them on its own goroutine — see eventsProcessor.SaveBlock.
	LogsResults *PreparedLogsResults
	LogsDB      []*Logs
}
