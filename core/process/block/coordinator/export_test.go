package coordinator

func NewTestTransactionCoordinator(txc *transactionCoordinator) TransactionCoordinator {
	return TransactionCoordinator{txc}
}

type TransactionCoordinator struct {
	*transactionCoordinator
}

func (t *TransactionCoordinator) RequestedTxs() int {
	t.mutRequestedTxs.Lock()
	defer t.mutRequestedTxs.Unlock()
	return t.requestedTxs
}
