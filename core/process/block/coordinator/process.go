package coordinator

import (
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.TransactionCoordinator = (*transactionCoordinator)(nil)

var log = logger.GetOrCreate("process/coordinator")

type transactionCoordinator struct {
	accounts    state.AccountsAdapter
	kapps       state.AccountsAdapter
	hasher      hashing.Hasher
	marshalizer marshal.Marshalizer

	mutPreProcessor sync.RWMutex
	txPreProcessor  process.PreProcessor

	mutRequestedTxs sync.RWMutex
	requestedTxs    int

	feeHandler     process.TransactionFeeHandler
	economicsFee   process.EconomicsDataHandler
	forkController core.ForkController

	transactionsLogProcessor process.TransactionLogProcessor
}

// NewTransactionCoordinator creates a transaction coordinator to run and coordinate preprocessors and processors
func NewTransactionCoordinator(
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	accounts state.AccountsAdapter,
	kapps state.AccountsAdapter,
	requestHandler process.RequestHandler,
	txPreProcessor process.PreProcessor,
	feeHandler process.TransactionFeeHandler,
	economicsFee process.EconomicsDataHandler,
	transactionsLogProcessor process.TransactionLogProcessor,
	forkController core.ForkController,
) (*transactionCoordinator, error) {
	if check.IfNil(accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(kapps) {
		return nil, common.ErrNilKAppAccountsAdapter
	}
	if check.IfNil(requestHandler) {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(txPreProcessor) {
		return nil, process.ErrNilPreProcessorsContainer
	}
	if check.IfNil(feeHandler) {
		return nil, process.ErrNilTxFeeHandler
	}
	if check.IfNil(economicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(forkController) {
		return nil, common.ErrNilForkController
	}
	if check.IfNil(transactionsLogProcessor) {
		return nil, process.ErrNilTxLogsProcessor
	}

	tc := &transactionCoordinator{
		accounts:       accounts,
		kapps:          kapps,
		hasher:         hasher,
		marshalizer:    marshalizer,
		txPreProcessor: txPreProcessor,
		feeHandler:     feeHandler,
		economicsFee:   economicsFee,
		forkController: forkController,

		transactionsLogProcessor: transactionsLogProcessor,
	}

	return tc, nil
}

// RequestBlockTransactions verifies missing transaction and requests them
func (tc *transactionCoordinator) RequestBlockTransactions(block *block.Block) {
	if check.IfNil(block) {
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		requestedTxs := tc.txPreProcessor.RequestBlockTransactions(block)

		tc.mutRequestedTxs.Lock()
		tc.requestedTxs = requestedTxs
		tc.mutRequestedTxs.Unlock()

		wg.Done()
	}()

	wg.Wait()
}

// IsDataPreparedForProcessing verifies if all the needed data is prepared
func (tc *transactionCoordinator) IsDataPreparedForProcessing(haveTime func() time.Duration) error {
	var errFound error
	errMutex := sync.Mutex{}

	wg := sync.WaitGroup{}
	tc.mutRequestedTxs.RLock()
	wg.Add(1)

	go func() {
		err := tc.txPreProcessor.IsDataPrepared(tc.requestedTxs, haveTime)
		if err != nil {
			log.Trace("IsDataPrepared", "error", err.Error())

			errMutex.Lock()
			errFound = err
			errMutex.Unlock()
		}
		wg.Done()
	}()

	tc.mutRequestedTxs.RUnlock()
	wg.Wait()

	return errFound
}

func (tc *transactionCoordinator) SaveTxsToStorage(block *block.Block) error {
	if check.IfNil(block) {
		return nil
	}

	err := tc.txPreProcessor.SaveTxsToStorage(block)
	if err != nil {
		log.Trace("SaveTxsToStorage", "error", err.Error())

		return err
	}

	return nil
}

// RestoreBlockDataFromStorage restores block data from storage to pool
func (tc *transactionCoordinator) RestoreBlockDataFromStorage(block *block.Block) (int, error) {
	if check.IfNil(block) {
		return 0, nil
	}

	restoredTxs, err := tc.txPreProcessor.RestoreBlockDataIntoPools(block)
	if err != nil {
		log.Trace("RestoreBlockDataIntoPools", "error", err.Error())

	}

	return restoredTxs, err
}

// RemoveBlockDataFromPool deletes block data from pools
func (tc *transactionCoordinator) RemoveBlockDataFromPool(body *block.Block) error {
	if check.IfNil(body) {
		return nil
	}

	return tc.txPreProcessor.RemoveTxsFromPools(body)
}

func (tc *transactionCoordinator) RemoveTxsFromPool(block *block.Block) error {
	if check.IfNil(block) {
		return nil
	}

	err := tc.txPreProcessor.RemoveTxsFromPools(block)
	if err != nil {
		log.Trace("RemoveTxsFromPools", "error", err.Error())
		return err
	}

	return nil
}

// ProcessBlockTransaction processes transactions and updates state tries
func (tc *transactionCoordinator) ProcessBlockTransactions(
	block *block.Block,
	timeRemaining func() time.Duration,
) error {
	if check.IfNil(block) {
		return process.ErrNilBlockHeader
	}

	haveTime := func() bool {
		return timeRemaining() >= 0
	}

	AccsSnapshot := tc.accounts.JournalLen()
	KAppsSnapshot := tc.kapps.JournalLen()

	startTime := time.Now()
	txsToBeReverted, numTxsProcessed, err := tc.txPreProcessor.ProcessBlockTransactions(block, haveTime)
	elapsedTime := time.Since(startTime)
	log.Debug("elapsed time to processBlockTransactions", "time [s]", elapsedTime)
	if err != nil {
		log.Debug("ProcessBlockTransaction",
			"txs to be reverted", len(txsToBeReverted),
			"num txs processed", numTxsProcessed,
			"error", err.Error(),
		)
		startTime = time.Now()

		errAccountState := tc.accounts.RevertToSnapshot(AccsSnapshot)
		if errAccountState != nil {
			// TODO: evaluate if reloading the trie from disk will might solve the problem
			log.Debug("AccRevertToSnapshot", "error", errAccountState.Error())
		}

		errKAppState := tc.kapps.RevertToSnapshot(KAppsSnapshot)
		if errAccountState != nil {
			// TODO: evaluate if reloading the trie from disk will might solve the problem
			log.Debug("KAppsRevertToSnapshot", "error", errKAppState.Error())
		}

		if len(txsToBeReverted) > 0 {
			tc.feeHandler.RevertFees(txsToBeReverted)
		}

		elapsedTime = time.Since(startTime)
		log.Debug("ProcessBlockTransaction Revert",
			"elapsedTime [s]", elapsedTime,
		)
		return err
	}

	if block.GetTxFees() != tc.feeHandler.GetAccumulatedTxFees() {
		return process.ErrInvalidTXFees
	}

	if block.GetKAppFees() != tc.feeHandler.GetAccumulatedKAppFees() {
		return process.ErrInvalidKAppsFees
	}

	if tc.forkController.EnableSmartContracts() {
		if numTxsProcessed != len(block.TxHashes) {
			return process.ErrInvalidNumberOfBlockTxs
		}
	}

	return nil
}

func (tc *transactionCoordinator) CreateAndProcessBlockTransactions(
	blk *block.Block,
	haveTime func() bool,
) error {
	if check.IfNil(blk) {
		return process.ErrNilBlockHeader
	}

	AccsSnapshot := tc.accounts.JournalLen()
	KAppsSnapshot := tc.kapps.JournalLen()

	startTime := time.Now()
	txsToBeReverted, numTxsProcessed, err := tc.txPreProcessor.CreateAndProcessBlockTransactions(blk, haveTime)
	elapsedTime := time.Since(startTime)
	log.Debug("elapsed time to processBlockTransactions", "time [s]", elapsedTime)
	if err != nil {
		log.Debug("ProcessBlockTransaction",
			"txs to be reverted", len(txsToBeReverted),
			"num txs processed", numTxsProcessed,
			"error", err.Error(),
		)

		errAccountState := tc.accounts.RevertToSnapshot(AccsSnapshot)
		if errAccountState != nil {
			log.Debug("AccRevertToSnapshot", "error", errAccountState.Error())
		}

		errKAppState := tc.kapps.RevertToSnapshot(KAppsSnapshot)
		if errAccountState != nil {
			log.Debug("KAppRevertToSnapshot", "error", errKAppState.Error())
		}

		if len(txsToBeReverted) > 0 {
			tc.feeHandler.RevertFees(txsToBeReverted)
		}

		return err
	}

	return nil
}

// CreateBlockStarted initializes necessary data for preprocessors at block create or block process
func (tc *transactionCoordinator) CreateBlockStarted() {
	tc.mutPreProcessor.RLock()
	tc.txPreProcessor.CreateBlockStarted()
	tc.mutPreProcessor.RUnlock()

	tc.transactionsLogProcessor.Clean()
}

func (tc *transactionCoordinator) GetAllCurrentUsedTxs() map[string]data.TransactionHandler {
	return tc.txPreProcessor.GetAllCurrentUsedTxs()
}

// GetAllCurrentLogs return the cached logs data from current round
func (tc *transactionCoordinator) GetAllCurrentLogs() []*data.LogData {
	return tc.transactionsLogProcessor.GetAllCurrentLogs()
}

// IsInterfaceNil returns true if there is no value under the interface
func (tc *transactionCoordinator) IsInterfaceNil() bool {
	return tc == nil
}

// CreateMarshalizedData creates marshalized data for broadcasting
func (tc *transactionCoordinator) CreateMarshalizedData(blk *block.Block) ([][]byte, error) {
	if check.IfNil(blk) {
		return nil, common.ErrNilHeader
	}

	return tc.txPreProcessor.CreateMarshalizedData(blk.TxHashes)
}
