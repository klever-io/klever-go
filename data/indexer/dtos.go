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
