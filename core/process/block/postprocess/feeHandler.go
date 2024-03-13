package postprocess

import (
	"fmt"
	"sync"

	"github.com/bugsnag/bugsnag-go/v2"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core/process"
)

var log = logger.GetOrCreate("process/postprocess")

var _ process.TransactionFeeHandler = (*feeHandler)(nil)

type fees struct {
	TxFee   int64
	KAppFee int64
}

type feeHandler struct {
	mut                 sync.RWMutex
	mapHashFee          map[string]fees
	accumulatedTxFees   int64
	accumulatedKAppFees int64
}

// NewFeeAccumulator constructor for the fee accumulator
func NewFeeAccumulator() (*feeHandler, error) {
	f := &feeHandler{}
	f.accumulatedTxFees = 0
	f.accumulatedKAppFees = 0
	f.mapHashFee = make(map[string]fees)
	return f, nil
}

// CreateBlockStarted does the cleanup before creating a new block
func (f *feeHandler) CreateBlockStarted() {
	f.mut.Lock()
	f.mapHashFee = make(map[string]fees)
	f.accumulatedTxFees = 0
	f.accumulatedKAppFees = 0
	f.mut.Unlock()
}

// GetAccumulatedTxFees returns the total accumulated fees for TX
func (f *feeHandler) GetAccumulatedTxFees() int64 {
	f.mut.RLock()
	accumulatedTxFees := f.accumulatedTxFees
	f.mut.RUnlock()

	return accumulatedTxFees
}

// GetAccumulatedKAppFees returns the total accumulated fees for KApp
func (f *feeHandler) GetAccumulatedKAppFees() int64 {
	f.mut.RLock()
	accumulatedKAppFees := f.accumulatedKAppFees
	f.mut.RUnlock()

	return accumulatedKAppFees
}

// ProcessTransactionFee adds the tx cost to the accumulated amount
func (f *feeHandler) ProcessTransactionFee(txCost int64, kAppCost int64, txHash []byte) {
	if txCost < 0 || kAppCost < 0 {
		_ = bugsnag.Notify(fmt.Errorf("process transaction fee: %w", process.ErrNegativeValue), bugsnag.MetaData{"data": {"txCost": txCost, "kAppCost": kAppCost}})
		log.Error("negative cost in ProcessTransactionFee", "error", process.ErrNegativeValue.Error())
		return
	}

	f.mut.Lock()
	fee, ok := f.mapHashFee[string(txHash)]
	if !ok {
		fee.TxFee = 0
		fee.KAppFee = 0
	}

	fee.TxFee += txCost
	fee.KAppFee += kAppCost

	f.mapHashFee[string(txHash)] = fee
	f.accumulatedTxFees += txCost
	f.accumulatedKAppFees += kAppCost
	f.mut.Unlock()
}

// RevertFees reverts the accumulated fees for txHashes
func (f *feeHandler) RevertFees(txHashes [][]byte) {
	f.mut.Lock()
	defer f.mut.Unlock()

	for _, txHash := range txHashes {
		fee, ok := f.mapHashFee[string(txHash)]
		if !ok {
			continue
		}
		f.accumulatedTxFees -= fee.TxFee
		f.accumulatedKAppFees -= fee.KAppFee
		delete(f.mapHashFee, string(txHash))
	}
}

// RevertTransactionFee reverts the accumulated fees for txHashes
func (f *feeHandler) RevertTransactionFee(txHash []byte, txCost int64, kAppCost int64) {
	f.mut.Lock()
	defer f.mut.Unlock()

	fee, ok := f.mapHashFee[string(txHash)]
	if !ok {
		return
	}

	// prevent revert > fee
	if fee.TxFee < txCost {
		txCost = fee.TxFee
	}

	if fee.KAppFee < kAppCost {
		kAppCost = fee.KAppFee
	}

	fee.TxFee -= txCost
	fee.KAppFee -= kAppCost

	f.accumulatedTxFees -= txCost
	f.accumulatedKAppFees -= kAppCost
	f.mapHashFee[string(txHash)] = fee
}

// IsInterfaceNil returns true if there is no value under the interface
func (f *feeHandler) IsInterfaceNil() bool {
	return f == nil
}
