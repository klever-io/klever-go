package data

// PreparedBlockData carries the result of prepareTransactionsForDatabase so
// the events orchestrator can pre-compute it once on the commit goroutine and
// hand it to the indexer worker (avoiding duplicate prep and decoupling
// websocket dispatch from the elastic worker's lifecycle).
type PreparedBlockData struct {
	Txs     []*Transaction
	TxsMap  map[string]*Transaction
	Altered *AlteredData
}
