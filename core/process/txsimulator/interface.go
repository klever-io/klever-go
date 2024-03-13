package txsimulator

import (
	txSimData "github.com/klever-io/klever-go/core/process/txsimulator/data"
	"github.com/klever-io/klever-go/data/transaction"
)

// TransactionProcessor defines the operations needed to be done by a transaction processor
type TransactionProcessor interface {
	ProcessTransaction(transaction *transaction.Transaction) error
	IsInterfaceNil() bool
}

// TransactionSimulatorProcessor defines the actions which a transaction simulator processor has to implement
type TransactionSimulatorProcessor interface {
	ProcessTx(tx *transaction.Transaction) (*txSimData.SimulationResults, error)
	IsInterfaceNil() bool
}
