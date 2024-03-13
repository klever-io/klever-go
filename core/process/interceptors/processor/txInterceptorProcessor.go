package processor

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.InterceptorProcessor = (*TxInterceptorProcessor)(nil)

// TxInterceptorProcessor is the processor used when intercepting transactions
// (smart contract results, receipts, transaction) structs which satisfy TransactionHandler interface.
type TxInterceptorProcessor struct {
	dataPool    TXDataPool
	txValidator process.TxValidator
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
		dataPool:    argument.TxDataCache,
		txValidator: argument.TxValidator,
	}, nil
}

// Validate checks if the intercepted data can be processed
func (txip *TxInterceptorProcessor) Validate(data process.InterceptedData, _ core.PeerID) error {
	interceptedTx, ok := data.(InterceptedTransactionHandler)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	err := txip.txValidator.CheckDup(data.Hash())
	if err != nil {
		return err
	}

	return txip.txValidator.CheckTxValidity(interceptedTx)
}

// Save will save the received data into the cacher
func (txip *TxInterceptorProcessor) Save(data process.InterceptedData, _ core.PeerID, _ string) error {
	interceptedTx, ok := data.(InterceptedTransactionHandler)
	if !ok {
		return process.ErrWrongTypeAssertion
	}

	tx := interceptedTx.Transaction().(*transaction.Transaction)
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

	tx := interceptedTx.Transaction().(*transaction.Transaction)
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
