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
	// Prepared optionally carries pre-computed block data produced by the
	// events orchestrator (concretely a *indexer/data.PreparedBlockData). When
	// non-nil, downstream consumers (e.g., elasticProcessor.SaveTransactions)
	// must reuse it instead of recomputing. Typed as any so this public DTO
	// does not depend on the internal indexer/data package.
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
