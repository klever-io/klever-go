package data

// PreparedBlockData carries the result of prepareTransactionsForDatabase
// shared between the websocket dispatcher and the elastic worker.
type PreparedBlockData struct {
	Txs     []*Transaction
	TxsMap  map[string]*Transaction
	Altered *AlteredData
}
