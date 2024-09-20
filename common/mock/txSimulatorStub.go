package mock

import (
	txSimData "github.com/klever-io/klever-go/core/process/txsimulator/data"
	"github.com/klever-io/klever-go/data/transaction"
)

// TransactionSimulatorProcessor defines the actions which a transaction simulator processor has to implement
type TransactionSimulatorProcessorStub struct {
	ProcessTXCalled      func(tx *transaction.Transaction) (*txSimData.SimulationResults, error)
	IsInterfaceNilCalled func() bool
}

func (t *TransactionSimulatorProcessorStub) ProcessTx(tx *transaction.Transaction) (*txSimData.SimulationResults, error) {
	if t.ProcessTXCalled != nil {
		return t.ProcessTXCalled(tx)
	}
	return nil, nil
}

func (t *TransactionSimulatorProcessorStub) IsInterfaceNil() bool {
	if t.IsInterfaceNilCalled != nil {
		return t.IsInterfaceNilCalled()
	}
	return t == nil
}
