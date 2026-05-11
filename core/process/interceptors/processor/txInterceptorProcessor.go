package processor

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.InterceptorProcessor = (*TxInterceptorProcessor)(nil)

// TxInterceptorProcessor is the processor used when intercepting transactions
// (smart contract results, receipts, transaction) structs which satisfy TransactionHandler interface.
type TxInterceptorProcessor struct {
	dataPool              TXDataPool
	txValidator           process.TxValidator
	requestedItemsHandler retriever.RequestedItemsHandler
}

// NewTxInterceptorProcessor creates a new TxInterceptorProcessor instance
func NewTxInterceptorProcessor(argument *ArgTxInterceptorProcessor) (*TxInterceptorProcessor, error) {
	if argument == nil {
		return nil, process.ErrNilArgumentStruct
	}
	if check.IfNil(argument.TxDataCache) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(argument.TxValidator) {
		return nil, process.ErrNilTxValidator
	}

	return &TxInterceptorProcessor{
		dataPool:              argument.TxDataCache,
		txValidator:           argument.TxValidator,
		requestedItemsHandler: argument.RequestedItemsHandler,
	}, nil
}

// Validate checks if the intercepted data can be processed
func (txip *TxInterceptorProcessor) Validate(data process.InterceptedData, _ core.PeerID) error {
	interceptedTx, ok := data.(InterceptedTransactionHandler)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	err := txip.txValidator.CheckDup(data.Hash())
	// allow requested transactions to be validated
	if err != nil && !txip.checkRequested(data.Hash()) {
		return err
	}

	return txip.txValidator.CheckTxValidity(interceptedTx)
}

// checkRequested checks if the requested item is in the requested items handler
func (txip *TxInterceptorProcessor) checkRequested(hash []byte) bool {
	if txip.requestedItemsHandler == nil {
		return false
	}

	return txip.requestedItemsHandler.Has(string(hash))
}

// Save will save the received data into the cacher
func (txip *TxInterceptorProcessor) Save(data process.InterceptedData, _ core.PeerID, _ string) error {
	interceptedTx, ok := data.(InterceptedTransactionHandler)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	// Guarded type assertion: an interceptor reached here with a non-Transaction
	// payload would panic the goroutine and silently leak its throttler slot
	// (CWE-704).
	tx, ok := interceptedTx.Transaction().(*transaction.Transaction)
	if !ok {
		return process.ErrWrongTypeAssertion
	}
	size := tx.GetSize()

	txip.dataPool.AddData(
		data.Hash(),
		interceptedTx.Transaction(),
		size,
		"0",
	)

	return nil
}

// Save will save the received data into the cacher
func (txip *TxInterceptorProcessor) Notify(data process.InterceptedData, _ core.PeerID, _ string) error {
	interceptedTx, ok := data.(InterceptedTransactionHandler)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	// See Save above; same guard, same rationale.
	tx, ok := interceptedTx.Transaction().(*transaction.Transaction)
	if !ok {
		return process.ErrWrongTypeAssertion
	}
	size := tx.GetSize()

	txip.dataPool.Notify(data.Hash(),
		interceptedTx.Transaction(),
		size,
	)

	return nil
}

// RegisterHandler registers a callback function to be notified of incoming transactions
func (txip *TxInterceptorProcessor) RegisterHandler(_ func(topic string, hash []byte, data interface{})) {
	log.Error("txInterceptorProcessor.RegisterHandler", "error", "not implemented")
}

// IsInterfaceNil returns true if there is no value under the interface
func (txip *TxInterceptorProcessor) IsInterfaceNil() bool {
	return txip == nil
}
