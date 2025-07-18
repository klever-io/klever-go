package preprocess

import (
	"bytes"
	"errors"
	"sort"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.DataMarshalizer = (*transactions)(nil)
var _ process.PreProcessor = (*transactions)(nil)

var log = logger.GetOrCreate("process/block/preprocess")

type transactions struct {
	*basePreProcess
	chRcvAllTxs          chan bool
	forkProcessor        core.ForkController
	onRequestTransaction func(txHashes [][]byte)
	txsForCurrBlock      txsForBlock
	txPool               retriever.ShardedDataCacherNotifier
	storage              retriever.StorageService
	txProcessor          process.TransactionProcessor
	orderedTxs           map[string][]data.TransactionHandler
	orderedTxHashes      map[string][][]byte
	mutOrderedTxs        sync.RWMutex
	accountsInfo         map[string]bool
	mutAccountsInfo      sync.RWMutex
	emptyAddress         []byte
}

// NewTransactionPreprocessor creates a new transaction preprocessor object
func NewTransactionPreprocessor(
	txDataPool retriever.ShardedDataCacherNotifier,
	store retriever.StorageService,
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	txProcessor process.TransactionProcessor,
	accounts state.AccountsAdapter,
	kapps state.AccountsAdapter,
	peers state.AccountsAdapter,
	onRequestTransaction func(txHashes [][]byte),
	economicsFee process.EconomicsDataHandler,
	pubkeyConverter core.PubkeyConverter,
	forkController core.ForkController,
) (*transactions, error) {

	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(txDataPool) {
		return nil, process.ErrNilTransactionPool
	}
	if check.IfNil(store) {
		return nil, common.ErrNilTxStorage
	}
	if check.IfNil(txProcessor) {
		return nil, process.ErrNilTxProcessor
	}
	if check.IfNil(accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(kapps) {
		return nil, common.ErrNilKAppAccountsAdapter
	}
	if check.IfNil(peers) {
		return nil, common.ErrNilPeerAccountsAdapter
	}
	if onRequestTransaction == nil {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(economicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(pubkeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(forkController) {
		return nil, common.ErrNilForkController
	}

	bpp := basePreProcess{
		hasher:          hasher,
		marshalizer:     marshalizer,
		economicsFee:    economicsFee,
		accounts:        accounts,
		peers:           peers,
		kapps:           kapps,
		pubkeyConverter: pubkeyConverter,
		forkController:  forkController,
	}

	txs := transactions{
		basePreProcess:       &bpp,
		storage:              store,
		txPool:               txDataPool,
		onRequestTransaction: onRequestTransaction,
		forkProcessor:        forkController,
		txProcessor:          txProcessor,
	}

	txs.chRcvAllTxs = make(chan bool)
	txs.txPool.RegisterOnAdded(txs.receivedTransaction)
	txs.txsForCurrBlock.txHashAndInfo = make(map[string]*txInfo)
	txs.orderedTxs = make(map[string][]data.TransactionHandler)
	txs.orderedTxHashes = make(map[string][][]byte)
	txs.accountsInfo = make(map[string]bool)

	txs.emptyAddress = make([]byte, txs.pubkeyConverter.Len())

	return &txs, nil
}

// waitForTxHashes waits for a call whether all the requested transactions appeared
func (txs *transactions) waitForTxHashes(waitTime time.Duration) error {
	select {
	case <-txs.chRcvAllTxs:
		return nil
	case <-time.After(waitTime):
		return process.ErrTimeIsOut
	}
}

// IsDataPrepared returns non error if all the requested transactions arrived and were saved into the pool
func (txs *transactions) IsDataPrepared(requestedTxs int, haveTime func() time.Duration) error {
	if requestedTxs > 0 {
		log.Debug("requested missing txs", "num txs", requestedTxs)
		err := txs.waitForTxHashes(haveTime())
		txs.txsForCurrBlock.mutTxsForBlock.Lock()
		missingTxs := txs.txsForCurrBlock.missingTxs
		txs.txsForCurrBlock.missingTxs = 0
		txs.txsForCurrBlock.mutTxsForBlock.Unlock()
		log.Debug("received missing txs", "num txs", requestedTxs-missingTxs, "requested", requestedTxs, "missing", missingTxs)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveTxsFromPools removes transactions from associated pools
func (txs *transactions) RemoveTxsFromPools(blk *block.Block) error {
	return txs.removeTxsFromPools(blk, txs.txPool)
}

// receivedTransaction is a call back function which is called when a new transaction
// is added in the transaction pool
func (txs *transactions) receivedTransaction(key []byte, value interface{}) {
	wrappedTx, ok := value.(*txcache.WrappedTransaction)
	if !ok {
		log.Warn("transactions.receivedTransaction", "error", process.ErrWrongTypeAssertion)
		return
	}

	receivedAllMissing := txs.baseReceivedTransaction(key, wrappedTx.Tx, &txs.txsForCurrBlock)

	if receivedAllMissing {
		txs.chRcvAllTxs <- true
	}
}

// RestoreBlockDataIntoPools restores the transactions to associated pools
func (txs *transactions) RestoreBlockDataIntoPools(blk *block.Block) (int, error) {
	if check.IfNil(blk) {
		return 0, process.ErrNilBlockHeader
	}

	txsBuff, err := txs.storage.GetAll(retriever.TransactionUnit, blk.TxHashes)
	if err != nil {
		log.Debug("tx from block was not found in TransactionUnit",
			"num txs", len(blk.TxHashes),
		)

		return 0, err
	}

	storer := txs.storage.GetStorer(retriever.TransactionUnit)

	for txHash, txBuff := range txsBuff {
		tx := transaction.Transaction{}
		err = txs.marshalizer.Unmarshal(&tx, txBuff)
		if err != nil {
			return 0, err
		}

		// reset TX processed fields
		tx.PrepareForProcessing()

		txs.txPool.AddData([]byte(txHash), &tx, tx.GetSize(), "0")
		// remove from storage
		err = storer.Remove([]byte(txHash))
		if err != nil {
			log.Debug("store.Remove",
				"error", err.Error(),
				"dataUnit", retriever.TransactionUnit,
				"txHash", txHash,
			)
		}
	}

	txsRestored := len(blk.TxHashes)

	return txsRestored, nil
}

// SaveTxsToStorage saves transactions from body into storage
func (txs *transactions) SaveTxsToStorage(blk *block.Block) error {
	for i := 0; i < len(blk.TxHashes); i++ {
		txHash := blk.TxHashes[i]

		txs.txsForCurrBlock.mutTxsForBlock.RLock()
		txInfoFromMap := txs.txsForCurrBlock.txHashAndInfo[string(txHash)]
		txs.txsForCurrBlock.mutTxsForBlock.RUnlock()

		if txInfoFromMap == nil || txInfoFromMap.tx == nil {
			log.Debug("missing transaction in saveTxsToStorage ", "type", retriever.TransactionUnit, "txHash", txHash)
			return process.ErrMissingTransaction
		}

		buff, err := txs.marshalizer.Marshal(txInfoFromMap.tx)
		if err != nil {
			return err
		}

		errNotCritical := txs.storage.Put(retriever.TransactionUnit, txHash, buff)
		if errNotCritical != nil {
			log.Debug("store.Put",
				"error", errNotCritical.Error(),
				"dataUnit", retriever.TransactionUnit,
			)
		}
	}

	return nil
}

// CreateBlockStarted cleans the local cache map for processed/created transactions at this slot
func (txs *transactions) CreateBlockStarted() {
	_ = tools.EmptyChannel(txs.chRcvAllTxs)

	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	txs.txsForCurrBlock.missingTxs = 0
	txs.txsForCurrBlock.txHashAndInfo = make(map[string]*txInfo)
	txs.txsForCurrBlock.mutTxsForBlock.Unlock()

	txs.mutOrderedTxs.Lock()
	txs.orderedTxs = make(map[string][]data.TransactionHandler)
	txs.orderedTxHashes = make(map[string][][]byte)
	txs.mutOrderedTxs.Unlock()

	txs.mutAccountsInfo.Lock()
	txs.accountsInfo = make(map[string]bool)
	txs.mutAccountsInfo.Unlock()
}

// CreateMarshalizedData marshalizes transactions and creates and saves them into a new structure
func (txs *transactions) CreateMarshalizedData(txHashes [][]byte) ([][]byte, error) {
	mrsScrs, err := txs.createMarshalizedData(txHashes, &txs.txsForCurrBlock)
	if err != nil {
		return nil, err
	}

	return mrsScrs, nil
}

// GetAllCurrentUsedTxs returns all the transactions used at current creation / processing
func (txs *transactions) GetAllCurrentUsedTxs() map[string]data.TransactionHandler {
	txPool := make(map[string]data.TransactionHandler, len(txs.txsForCurrBlock.txHashAndInfo))

	txs.txsForCurrBlock.mutTxsForBlock.RLock()
	for txHash, txInfoFromMap := range txs.txsForCurrBlock.txHashAndInfo {
		txPool[txHash] = txInfoFromMap.tx
	}
	txs.txsForCurrBlock.mutTxsForBlock.RUnlock()

	return txPool
}

// IsInterfaceNil returns true if there is no value under the interface
func (txs *transactions) IsInterfaceNil() bool {
	return txs == nil
}

// this function is used to compute the transactions to be included in the block
// it sorts the transactions by sender and nonce with randomness and returns the selected transactions
// transactions are selected based on the gas bandwidth available
func (txs *transactions) ComputeSortedTxs(
	gasBandwidth uint64,
	randomness []byte,
) ([]*txcache.WrappedTransaction, error) {
	txCache, err := txs.getTXPool()
	if err != nil {
		return nil, err
	}

	// select transactions from the pool based on max number of transactions to select and max gas bandwidth per sender
	selectedTXs := txCache.SelectTransactions(process.MaxNumOfTxsToSelect, process.NumTxPerBatchForFillingBlock, core.MaxGasBandwidthPerBatchPerSender)
	// pre-filter transactions prioritizing the move balance operations
	// reaming transactions are not used in this step
	selectedTxs, _ := txs.preFilterTransactionsWithPriority(selectedTXs, gasBandwidth)
	txs.sortTransactionsBySenderAndNonce(selectedTxs, randomness)

	return selectedTxs, nil
}

func (txs *transactions) getTXPool() (*txcache.TxCache, error) {
	txPool := txs.txPool.ShardDataStore("0")
	if check.IfNil(txPool) {
		log.Debug("CreateAndProcessBlock", "error", common.ErrNilTxDataPool.Error())
		return nil, common.ErrNilTxDataPool
	}

	txCache, isTxCache := txPool.(*txcache.TxCache)
	if !isTxCache {
		log.Debug("CreateAndProcessBlock", "error", common.ErrWrongTypeAssertion.Error())
		return nil, common.ErrWrongTypeAssertion
	}

	return txCache, nil
}

// sortTransactionsBySenderAndNonce sorts the provided transactions and hashes simultaneously
func (txs *transactions) sortTransactionsBySenderAndNonce(transactions []*txcache.WrappedTransaction, randomness []byte) {
	// make sure randomness is 32bytes and uniform
	randSeed := txs.hasher.Compute(string(randomness))
	xoredAddresses := make(map[string][]byte)

	for _, tx := range transactions {
		xoredBytes := xorBytes(tx.Tx.GetSender(), randSeed)
		xoredAddresses[string(tx.Tx.GetSender())] = txs.hasher.Compute(string(xoredBytes))
	}

	sorter := func(i, j int) bool {
		txI := transactions[i].Tx
		txJ := transactions[j].Tx

		delta := bytes.Compare(xoredAddresses[string(txI.GetSender())], xoredAddresses[string(txJ.GetSender())])
		if delta == 0 {
			delta = int(txI.GetNonce()) - int(txJ.GetNonce()) // #nosec G115
		}

		return delta < 0
	}

	sort.Slice(transactions, sorter)
}

// parameters need to be of the same len, otherwise it will panic (if second slice shorter)
func xorBytes(a, b []byte) []byte {
	res := make([]byte, len(a))
	for i := range a {
		res[i] = a[i] ^ b[i]
	}
	return res
}

// preFilterTransactions filters the transactions prioritizing the move balance operations
func (txs *transactions) preFilterTransactionsWithPriority(
	transactions []*txcache.WrappedTransaction,
	gasBandwidth uint64,
) ([]*txcache.WrappedTransaction, []*txcache.WrappedTransaction) {
	selectedTxs, skippedTxs, gasEstimation := txs.filterNoDataTXs(transactions)

	return txs.prefilterTransactions(selectedTxs, skippedTxs, gasEstimation, gasBandwidth)
}

func (txs *transactions) filterNoDataTXs(transactions []*txcache.WrappedTransaction) ([]*txcache.WrappedTransaction, []*txcache.WrappedTransaction, uint64) {
	selectedTxs := make([]*txcache.WrappedTransaction, 0, len(transactions))
	skippedTxs := make([]*txcache.WrappedTransaction, 0, len(transactions))
	skippedAddresses := make(map[string]struct{})

	skipped := 0
	gasEstimation := uint64(0)
	for i, tx := range transactions {
		// prioritize no data TX (mostly move balance operations)
		// and don't care about gas cost for them
		if len(tx.Tx.GetData()) > 0 {
			skippedTxs = append(skippedTxs, transactions[i])
			skippedAddresses[string(tx.Tx.GetSender())] = struct{}{}
			skipped++
			continue
		}

		// once address is marked to be skipped, all its transactions next will be skipped
		if shouldSkipTransactionIfMarkedAddress(tx, skippedAddresses) {
			skippedTxs = append(skippedTxs, transactions[i])
			continue
		}

		selectedTxs = append(selectedTxs, transactions[i])
		// base protocol TX have a hardcoded minimum gas limit
		// to compute the gas estimation for the block filling only
		gasEstimation += core.MinGasLimit
	}
	return selectedTxs, skippedTxs, gasEstimation
}

func shouldSkipTransactionIfMarkedAddress(tx *txcache.WrappedTransaction, addressesToSkip map[string]struct{}) bool {
	_, ok := addressesToSkip[string(tx.Tx.GetSender())]
	return ok
}

func (txs *transactions) prefilterTransactions(
	initialTxs []*txcache.WrappedTransaction,
	additionalTxs []*txcache.WrappedTransaction,
	initialTxsGasEstimation uint64,
	gasBandwidth uint64,
) ([]*txcache.WrappedTransaction, []*txcache.WrappedTransaction) {

	selectedTxs, remainingTxs, gasEstimation := txs.addTxsWithinBandwidth(initialTxs, additionalTxs, initialTxsGasEstimation, gasBandwidth)

	log.Debug("preFilterTransactions estimation",
		"initialTxs", len(initialTxs),
		"gasCost initialTxs", initialTxsGasEstimation,
		"additionalTxs", len(additionalTxs),
		"gasCostEstimation", gasEstimation,
		"selected", len(selectedTxs),
		"skipped", len(remainingTxs))

	return selectedTxs, remainingTxs
}

func (txs *transactions) addTxsWithinBandwidth(
	initialTxs []*txcache.WrappedTransaction,
	additionalTxs []*txcache.WrappedTransaction,
	initialTxsGasEstimation uint64,
	totalGasBandwidth uint64,
) ([]*txcache.WrappedTransaction, []*txcache.WrappedTransaction, uint64) {
	remainingTxs := make([]*txcache.WrappedTransaction, 0, len(additionalTxs))
	resultedTxs := make([]*txcache.WrappedTransaction, 0, len(initialTxs))
	resultedTxs = append(resultedTxs, initialTxs...)

	gasEstimation := initialTxsGasEstimation
	for i, tx := range additionalTxs {
		// use gasProvided by TX
		gasProvided := tx.Tx.GetGasLimit()
		if gasEstimation+gasProvided > totalGasBandwidth {
			remainingTxs = append(remainingTxs, additionalTxs[i:]...)
			break
		}

		resultedTxs = append(resultedTxs, additionalTxs[i])
		gasEstimation += gasProvided
	}
	return resultedTxs, remainingTxs, gasEstimation
}

// RequestBlockTransactions request for transactions if missing from a block.Body
func (txs *transactions) RequestBlockTransactions(blk *block.Block) int {
	if check.IfNil(blk) {
		return 0
	}

	return txs.computeExistingAndRequestMissingTxs(blk)
}

// computeExistingAndRequestMissingTxsForShards calculates what transactions are available and requests
// what are missing from block.Body
func (txs *transactions) computeExistingAndRequestMissingTxs(blk *block.Block) int {
	numMissingTxsForShard := txs.computeExistingAndRequestMissing(
		blk,
		&txs.txsForCurrBlock,
		txs.chRcvAllTxs,
		txs.txPool,
		txs.onRequestTransaction,
	)

	return numMissingTxsForShard
}

// getAllTxsFromMiniBlock gets all the transactions from a miniblock into a new structure
func (txs *transactions) getAllTxsFromBlock(
	blk *block.Block,
) ([]*transaction.Transaction, [][]byte, error) {
	txCache := txs.txPool.ShardDataStore("0")
	if txCache == nil {
		return nil, nil, process.ErrNilTransactionPool
	}

	// verify if all transaction exists
	txsSlice := make([]*transaction.Transaction, 0, len(blk.TxHashes))
	txHashes := make([][]byte, 0, len(blk.TxHashes))
	for _, txHash := range blk.TxHashes {
		tmp, _ := txCache.Peek(txHash)
		if tmp == nil {
			return nil, nil, process.ErrMissingTransaction
		}

		tx, ok := tmp.(*transaction.Transaction)
		if !ok {
			return nil, nil, process.ErrWrongTypeAssertion
		}
		txHashes = append(txHashes, txHash)
		txsSlice = append(txsSlice, tx)
	}

	return txsSlice, txHashes, nil
}

// ProcessBlockTransactions processes all the transactions from a block and saves the processed transactions in local cache
func (txs *transactions) ProcessBlockTransactions(
	blk *block.Block,
	haveTime func() bool,
) (data.ProcessResults, error) {
	log.Debug("ProcessBlockTransactions has been started", "nonce", blk.GetNonce())

	result := &processResults{
		txHashes: make([][]byte, 0),
		size:     0,
	}

	totalTimeUsedForProcess := time.Duration(0)
	numTxsBad := 0

	if !haveTime() {
		return result, process.ErrTimeIsOut
	}

	blockTxs, blockTxHashes, err := txs.getAllTxsFromBlock(blk)
	if err != nil {
		return result, err
	}

	for index := range blockTxs {
		// TODO: check if have been processed (its in storer??)
		// execute transaction to change the trie root hash
		startTime := time.Now()
		err := txs.processAndRemoveBadTransaction(
			blk,
			blockTxHashes[index],
			blockTxs[index],
		)
		elapsedTime := time.Since(startTime)
		totalTimeUsedForProcess += elapsedTime

		// if transaction have not been pre-processed successfully,
		// or BW fee is not enough, Result is not set to FAILED
		// then remove from pool as nothing can be done
		if err != nil && blockTxs[index].Result != transaction.Transaction_FAILED {
			numTxsBad++
			log.Trace("bad tx",
				"error", err.Error(),
				"hash", blockTxHashes[index],
			)
			// skip tx have been removed from pool
			continue
		}

		result.size += int64(blockTxs[index].Size())

		result.txHashes = append(result.txHashes, blockTxHashes[index])
	}

	log.Debug("createAndProcessBlock has been finished",
		"total txs", len(blockTxs),
		"num txs processed", result.size,
		"num txs bad", numTxsBad,
		"used time for processAndRemoveBadTransaction", totalTimeUsedForProcess)

	return result, nil
}

// CreateAndProcessBlock -
func (txs *transactions) CreateAndProcessBlockTransactions(blk *block.Block, haveTime func() bool) (data.ProcessResults, error) {
	startTime := time.Now()

	// get block gas limit from network economics
	gasBandwidth := txs.economicsFee.MaxGasLimitPerBlock()
	result := &processResults{
		txHashes: make([][]byte, 0),
		size:     0,
	}

	// get transactions from pool and sort them by sender and nonce with randomness
	// limiting the number of transactions to be processed
	selectedTXs, err := txs.ComputeSortedTxs(gasBandwidth, blk.GetRandSeed())
	if err != nil {
		log.Debug("computeSortedTxs", "error", err.Error())
		return result, nil
	}
	elapsedTime := time.Since(startTime)

	if len(selectedTXs) == 0 {
		log.Trace("no transaction found after computeSortedTxs",
			"time [s]", elapsedTime,
		)
		return result, nil
	}

	if !haveTime() {
		log.Debug("time is up after computeSortedTxs",
			"num txs", len(selectedTXs),
			"time [s]", elapsedTime,
		)
		return result, nil
	}

	log.Debug("elapsed time to computeSortedTxs",
		"num txs", len(selectedTXs),
		"time [s]", elapsedTime,
	)

	startTime = time.Now()
	processResults, err := txs.createAndProcessBlock(
		blk,
		haveTime,
		selectedTXs,
	)
	elapsedTime = time.Since(startTime)
	log.Debug("elapsed time to createAndProcessBlocks",
		"time [s]", elapsedTime,
	)

	if err != nil {
		log.Debug("createAndProcessBlocks", "error", err.Error())
		return processResults, err
	}

	return processResults, nil
}

func (txs *transactions) createAndProcessBlock(
	blk *block.Block,
	haveTime func() bool,
	sortedTxs []*txcache.WrappedTransaction,
) (data.ProcessResults, error) {
	log.Debug("createAndProcessBlock has been started")

	totalTimeUsedForProcess := time.Duration(0)
	txHashes := make([][]byte, 0)
	numTxsBad := 0
	numTxsSkipped := 0

	txsSize := int64(0)

	senderAddressToSkip := []byte("")

	defer func() {
		go txs.notifyTransactionProviderIfNeeded()
	}()

	for index := range sortedTxs {
		if !haveTime() {
			log.Debug("time is out in createAndProcessBlock")
			break
		}

		tx, ok := sortedTxs[index].Tx.(*transaction.Transaction)
		if !ok {
			log.Debug("wrong type assertion", "hash", sortedTxs[index].TxHash)
			continue
		}

		// clear any TX previous processing status
		tx.PrepareForProcessing()

		txHash := sortedTxs[index].TxHash
		if len(senderAddressToSkip) > 0 {
			if bytes.Equal(senderAddressToSkip, tx.GetSender()) {
				numTxsSkipped++
				continue
			}
		}

		// execute transaction to change the trie root hash
		startTime := time.Now()
		err := txs.processAndRemoveBadTransaction(
			blk,
			txHash,
			tx,
		)
		elapsedTime := time.Since(startTime)
		totalTimeUsedForProcess += elapsedTime

		txs.mutAccountsInfo.Lock()
		txs.accountsInfo[string(tx.GetSender())] = true
		txs.mutAccountsInfo.Unlock()

		// if transaction have not been pre-processed successfully,
		// or BW fee is not enough, Result is not set to FAILED
		// then remove from pool as nothing can be done
		if err != nil && tx.Result != transaction.Transaction_FAILED {
			if errors.Is(err, process.ErrHigherNonceInTransaction) {
				senderAddressToSkip = tx.GetSender()
			}

			numTxsBad++
			log.Trace("bad tx",
				"error", err.Error(),
				"hash", txHash,
			)
			// skip tx have been removed from pool
			continue
		}

		senderAddressToSkip = []byte("")

		// compute TX Size
		txsSize += int64(tx.Size())

		txHashes = append(txHashes, txHash)
	}

	if len(txHashes) > 0 {
		// Update block TxRootHash
		blk.TxHashes = txHashes
		err := blk.UpdateTxRootHash(txs.hasher)
		if err != nil {
			return nil, err
		}
	}

	log.Debug("createAndProcessBlock has been finished",
		"total txs", len(sortedTxs),
		"num txs added", len(txHashes),
		"num txs bad", numTxsBad,
		"num txs skipped", numTxsSkipped,
		"used time for processAndRemoveBadTransaction", totalTimeUsedForProcess)

	return &processResults{
		txHashes: txHashes,
		size:     txsSize,
	}, nil
}

// processAndRemoveBadTransaction processed transactions, if txs are with error it removes them from pool
func (txs *transactions) processAndRemoveBadTransaction(
	block *block.Block,
	txHash []byte,
	tx *transaction.Transaction,
) error {
	if check.IfNil(tx) {
		return process.ErrNilTransaction
	}

	ownerAcc, _, err := txs.txProcessor.PreProcessTransaction(tx)
	if err != nil {
		log.Trace("processAndRemoveBadTransaction.PreProcessTransaction", "tx", txHash, "error", err.Error())
		txs.txPool.RemoveData(txHash, "0")

		return err
	}

	_, err = txs.txProcessor.ProcessBandwidthFee(txHash, tx, ownerAcc)
	if err != nil {
		// remove any transactions that cannot pay for BW fee
		log.Trace("processAndRemoveBadTransaction bandwidth fee process", "tx", txHash, "error", err.Error())
		txs.txPool.RemoveData(txHash, "0")
		return err
	}

	AccsSnapshot := txs.accounts.JournalLen()
	KappsSnapshot := txs.kapps.JournalLen()
	PeerSnapshot := txs.peers.JournalLen()

	err = txs.txProcessor.ProcessTransaction(block, txHash, tx)
	if err != nil {
		tx.Result = transaction.Transaction_FAILED

		log.Trace("bad tx",
			"error", err.Error(),
			"hash", txHash,
		)

		errAccountState := txs.accounts.RevertToSnapshot(AccsSnapshot)
		if errAccountState != nil {
			log.Debug("processAndRemoveBadTransaction AccRevertToSnapshot", "error", errAccountState.Error())
		}

		errKAppState := txs.kapps.RevertToSnapshot(KappsSnapshot)
		if errAccountState != nil {
			log.Debug("processAndRemoveBadTransaction KAppRevertToSnapshot", "error", errKAppState.Error())
		}

		if txs.forkProcessor.EnableSmartContracts() {
			errPeerState := txs.peers.RevertToSnapshot(PeerSnapshot)
			if errAccountState != nil {
				log.Debug("processAndRemoveBadTransaction PeerRevertToSnapshot", "error", errPeerState.Error())
			}
		}
	}

	txs.txsForCurrBlock.mutTxsForBlock.Lock()
	txs.txsForCurrBlock.txHashAndInfo[string(txHash)] = &txInfo{tx: tx}
	txs.txsForCurrBlock.mutTxsForBlock.Unlock()

	return err
}

func (txs *transactions) notifyTransactionProviderIfNeeded() {
	txs.mutAccountsInfo.RLock()
	defer txs.mutAccountsInfo.RUnlock()

	txShardPool := txs.txPool.ShardDataStore("0")
	if check.IfNil(txShardPool) {
		log.Error("notifyTransactionProviderIfNeeded txShardPool", "error", common.ErrNilTxDataPool)
		return
	}
	sortedTransactionsProvider, ok := txShardPool.(TxCache)
	if !ok {
		log.Error("notifyTransactionProviderIfNeeded sortedTransactionsProvider", "error", common.ErrWrongTypeAssertion)
	}

	for senderAddress := range txs.accountsInfo {

		account, err := txs.getAccountForAddress([]byte(senderAddress))
		if err != nil {
			log.Debug("notifyTransactionProviderIfNeeded.getAccountForAddress", "error", err)
			continue
		}

		sortedTransactionsProvider.NotifyAccountNonce([]byte(senderAddress), account.GetNonce())
	}
}

func (txs *transactions) getAccountForAddress(address []byte) (state.AccountHandler, error) {
	account, err := txs.accounts.GetExistingAccount(address)
	if err != nil {
		return nil, err
	}

	return account, nil
}
