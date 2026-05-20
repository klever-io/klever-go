package indexer

import (
	"github.com/klever-io/klever-go/data"
)

// ArgsSaveBlockData will contains all information that are needed to save block data
type ArgsSaveBlockData struct {
	HeaderHash       []byte
	Header           data.HeaderHandler
	Signer           []byte
	TransactionsPool *Pool
	Validators       []string
	// Prepared, when non-nil, is a *indexer/data.PreparedBlockData consumers
	// reuse instead of re-prepping. Typed as any to avoid importing the
	// internal indexer/data package.
	Prepared any
}

// Pool will holds all types of transaction
type Pool struct {
	Txs  map[string]data.TransactionHandler
	Logs []*data.LogData
}

// ValidatorRatingInfo is a structure containing validator rating information
type ValidatorRatingInfo struct {
	PublicKey string
	Rating    float32
}
