package txcache

import (
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/atomic"
	"github.com/klever-io/klever-go/tools/check"
)

// cleanupInterval defines how often expired transactions are cleaned up
// Set to half of the TTL to ensure timely cleanup while avoiding excessive overhead
const cleanupInterval = 30 * time.Second

var _ storage.Cacher = (*TxCache)(nil)

// TxCache represents a cache-like structure (it has a fixed capacity and implements an eviction mechanism) for holding transactions
type TxCache struct {
	name                      string
	txListBySender            *txListBySenderMap
	txByHash                  *txByHashMap
	config                    Config
	evictionMutex             sync.Mutex
	evictionJournal           evictionJournal
	evictionSnapshotOfSenders []*txListForSender
	isEvictionInProgress      atomic.Flag
	numSendersSelected        atomic.Counter
	numSendersWithInitialGap  atomic.Counter
	numSendersWithMiddleGap   atomic.Counter
	numSendersInGracePeriod   atomic.Counter
	sweepingMutex             sync.Mutex
	sweepingListOfSenders     []*txListForSender
	mutTxOperation            sync.Mutex
	cleanupMutex              sync.Mutex
	cleanupTicker             *time.Ticker
	cleanupStop               chan struct{}
	// Cleanup metrics
	totalExpiredRemoved atomic.Counter
	totalCleanupRuns    atomic.Counter
	lastCleanupDuration atomic.Counter // in microseconds
	lastCleanupTime     atomic.Counter // unix timestamp
}

// NewTxCache creates a new transaction cache
func NewTxCache(config Config) (*TxCache, error) {
	// Set default cleanup interval if not specified
	// 0 means use default, negative means disable
	if config.CleanupInterval == 0 {
		config.CleanupInterval = cleanupInterval
	}

	log.Debug("NewTxCache", "config", config.String())
	storage.MonitorNewCache(config.Name, uint64(config.NumBytesThreshold))

	err := config.verify()
	if err != nil {
		return nil, err
	}

	// Note: for simplicity, we use the same "numChunks" for both internal concurrent maps
	numChunks := config.NumChunks
	senderConstraintsObj := config.getSenderConstraints()
	scoreComputerObj := newDefaultScoreComputer()

	txCache := &TxCache{
		name:            config.Name,
		txListBySender:  newTxListBySenderMap(numChunks, senderConstraintsObj, scoreComputerObj),
		txByHash:        newTxByHashMap(numChunks),
		config:          config,
		evictionJournal: evictionJournal{},
	}

	txCache.initSweepable()
	txCache.startPeriodicCleanup()
	return txCache, nil
}

// AddTx adds a transaction in the cache
// Eviction happens if maximum capacity is reached
func (cache *TxCache) AddTx(tx *WrappedTransaction) (ok bool, added bool) {
	if tx == nil || check.IfNil(tx.Tx) {
		return false, false
	}

	if cache.config.EvictionEnabled {
		cache.doEviction()
	}

	cache.mutTxOperation.Lock()
	addedInByHash := cache.txByHash.addTx(tx)
	addedInBySender, evicted := cache.txListBySender.addTx(tx)
	cache.mutTxOperation.Unlock()
	if addedInByHash != addedInBySender {
		// This can happen  when two go-routines concur to add the same transaction:
		// - A adds to "txByHash"
		// - B won't add to "txByHash" (duplicate)
		// - B adds to "txListBySender"
		// - A won't add to "txListBySender" (duplicate)
		log.Trace("TxCache.AddTx(): slight inconsistency detected:", "name", cache.name, "tx", tx.TxHash, "sender", tx.Tx.GetSender(), "addedInByHash", addedInByHash, "addedInBySender", addedInBySender)
	}

	if len(evicted) > 0 {
		cache.monitorEvictionWrtSenderLimit(tx.Tx.GetSender(), evicted)
		cache.txByHash.RemoveTxsBulk(evicted)
	}

	// The return value "added" is true even if transaction added, but then removed due to limits be sender.
	// This it to ensure that onAdded() notification is triggered.
	return true, addedInByHash || addedInBySender
}

// GetByTxHash gets the transaction by hash
func (cache *TxCache) GetByTxHash(txHash []byte) (*WrappedTransaction, bool) {
	tx, ok := cache.txByHash.getTx(string(txHash))
	return tx, ok
}

// SelectTransactions selects a reasonably fair list of transactions to be included in the next block
// It returns at most "numRequested" transactions
// Each sender gets the chance to give at least "batchSizePerSender" transactions, unless "numRequested" limit is reached before iterating over all senders
func (cache *TxCache) SelectTransactions(numRequested int, batchSizePerSender int, bandwidthPerSender uint64) []*WrappedTransaction {
	result := cache.doSelectTransactions(numRequested, batchSizePerSender, bandwidthPerSender)
	go cache.doAfterSelection()
	return result
}

func (cache *TxCache) doSelectTransactions(numRequested int, batchSizePerSender int, bandwidthPerSender uint64) []*WrappedTransaction {
	stopWatch := cache.monitorSelectionStart()

	// Log selection start with more details
	log.Debug("TxCache.doSelectTransactions: Starting transaction selection",
		"cache", cache.name,
		"numRequested", numRequested,
		"batchSizePerSender", batchSizePerSender,
		"bandwidthPerSender", bandwidthPerSender,
		"totalTxs", cache.CountTx(),
		"totalSenders", cache.CountSenders(),
		"totalBytes", cache.NumBytes(),
		"sendersWithInitialGap", cache.numSendersWithInitialGap.GetUint64(),
		"sendersWithMiddleGap", cache.numSendersWithMiddleGap.GetUint64(),
		"sendersInGracePeriod", cache.numSendersInGracePeriod.GetUint64(),
	)

	result := make([]*WrappedTransaction, numRequested)
	resultFillIndex := 0
	resultIsFull := false

	snapshotOfSenders := cache.getSendersEligibleForSelection()

	// Log details about each sender's status
	if len(snapshotOfSenders) == 0 {
		log.Debug("TxCache.doSelectTransactions: No eligible senders found",
			"cache", cache.name,
			"totalSenders", cache.CountSenders(),
			"totalTxs", cache.CountTx(),
		)
	}

	for pass := 0; !resultIsFull; pass++ {
		copiedInThisPass := 0

		for _, txList := range snapshotOfSenders {
			batchSizeWithScoreCoefficient := batchSizePerSender * int(txList.getLastComputedScore()+1)
			// Reset happens on first pass only
			isFirstBatch := pass == 0
			journal := txList.selectBatchTo(isFirstBatch, result[resultFillIndex:], batchSizeWithScoreCoefficient, bandwidthPerSender)
			cache.monitorBatchSelectionEnd(journal)

			if isFirstBatch {
				cache.collectSweepable(txList)
			}

			resultFillIndex += journal.copied
			copiedInThisPass += journal.copied
			resultIsFull = resultFillIndex == numRequested
			if resultIsFull {
				break
			}
		}

		nothingCopiedThisPass := copiedInThisPass == 0

		// No more passes needed
		if nothingCopiedThisPass {
			break
		}
	}

	result = result[:resultFillIndex]
	cache.monitorSelectionEnd(result, stopWatch)
	return result
}

func (cache *TxCache) getSendersEligibleForSelection() []*txListForSender {
	return cache.txListBySender.getSnapshotDescending()
}

func (cache *TxCache) doAfterSelection() {
	cache.sweepSweepable()
	cache.Diagnose(false)
}

// RemoveTxByHash removes tx by hash
func (cache *TxCache) RemoveTxByHash(txHash []byte) (*WrappedTransaction, bool) {
	cache.mutTxOperation.Lock()
	defer cache.mutTxOperation.Unlock()

	tx, foundInByHash := cache.txByHash.removeTx(string(txHash))
	if !foundInByHash {
		return nil, false
	}

	foundInBySender := cache.txListBySender.removeTx(tx)
	if !foundInBySender {
		// This condition can arise often at high load & eviction, when two go-routines concur to remove the same transaction:
		// - A = remove transactions upon commit / final
		// - B = remove transactions due to high load (eviction)
		//
		// - A reaches "RemoveTxByHash()", then "cache.txByHash.removeTx()".
		// - B reaches "cache.txByHash.RemoveTxsBulk()"
		// - B reaches "cache.txListBySender.RemoveSendersBulk()"
		// - A reaches "cache.txListBySender.removeTx()", but sender does not exist anymore
		log.Trace("TxCache.RemoveTxByHash(): slight inconsistency detected: !foundInBySender", "name", cache.name, "tx", txHash)
	}

	return tx, true
}

func (cache *TxCache) RemoveTxBySenderNonce(sender []byte, nonce uint64) int {
	cache.mutTxOperation.Lock()
	defer cache.mutTxOperation.Unlock()

	removed := cache.txListBySender.removeTxLowerNonce(sender, nonce)
	if len(removed) > 0 {
		cache.txByHash.RemoveTxsBulk(removed)
		log.Debug("TxCache.RemoveTxBySenderNonce: Removed transactions",
			"cache", cache.name,
			"sender", sender,
			"removedCount", len(removed),
			"maxNonce", nonce,
		)
	}

	return len(removed)
}

func (cache *TxCache) GetBySenderPaginated(sender []byte, page int, pageSize int) ([]interface{}, int) {
	cache.mutTxOperation.Lock()
	defer cache.mutTxOperation.Unlock()

	listForSender, ok := cache.txListBySender.getListForSender(string(sender))
	if !ok {
		return make([]interface{}, 0), 0
	}

	l := listForSender.items.Len()
	idx := page * pageSize
	if idx >= l {
		return make([]interface{}, 0), l
	}
	idxEnd := idx + pageSize
	if idxEnd > l {
		idxEnd = l
	}

	result := make([]interface{}, idxEnd-idx)
	count := 0
	for element := listForSender.items.Front(); element != nil && count < idxEnd; element = element.Next() {
		// listForSender.items is a linked list, so we need to skip elements until we reach the desired index
		if count < idx {
			count++
			continue
		}
		tx, ok := element.Value.(*WrappedTransaction)
		if !ok {
			continue
		}
		result[count-idx] = tx.Tx

		count++
	}

	return result, listForSender.items.Len()
}

// NumBytes gets the approximate number of bytes stored in the cache
func (cache *TxCache) NumBytes() int {
	return int(cache.txByHash.numBytes.GetUint64()) // #nosec G115
}

// CountTx gets the number of transactions in the cache
func (cache *TxCache) CountTx() uint64 {
	return cache.txByHash.counter.GetUint64()
}

// Len is an alias for CountTx
func (cache *TxCache) Len() int {
	return int(cache.CountTx()) // #nosec G115
}

// CountSenders gets the number of senders in the cache
func (cache *TxCache) CountSenders() uint64 {
	return cache.txListBySender.counter.GetUint64()
}

// ForEachTransaction iterates over the transactions in the cache
func (cache *TxCache) ForEachTransaction(function ForEachTransaction) {
	cache.txByHash.forEach(function)
}

// Clear clears the cache
func (cache *TxCache) Clear() {
	// Log cache statistics before clearing
	txCount := cache.CountTx()
	senderCount := cache.CountSenders()
	bytesCount := cache.NumBytes()

	log.Debug("TxCache.Clear: Clearing transaction cache",
		"cache", cache.name,
		"txCount", txCount,
		"senderCount", senderCount,
		"bytesCount", bytesCount,
	)

	cache.mutTxOperation.Lock()
	cache.txListBySender.clear()
	cache.txByHash.clear()
	cache.mutTxOperation.Unlock()

	log.Debug("TxCache.Clear: Cache cleared successfully",
		"cache", cache.name,
	)
}

// Put is not implemented
func (cache *TxCache) Put(_ []byte, _ interface{}, _ int) (evicted bool) {
	log.Error("TxCache.Put is not implemented")
	return false
}

// Get gets a transaction (unwrapped) by hash
// Implemented for compatibiltiy reasons (see txPoolsCleaner.go).
func (cache *TxCache) Get(key []byte) (value interface{}, ok bool) {
	tx, ok := cache.GetByTxHash(key)
	if ok {
		return tx.Tx, true
	}
	return nil, false
}

// Has checks if a transaction exists
func (cache *TxCache) Has(key []byte) bool {
	_, ok := cache.GetByTxHash(key)
	return ok
}

// Peek gets a transaction (unwrapped) by hash
// Implemented for compatibility reasons (see transactions.go, common.go).
func (cache *TxCache) Peek(key []byte) (value interface{}, ok bool) {
	tx, ok := cache.GetByTxHash(key)
	if ok {
		return tx.Tx, true
	}
	return nil, false
}

// HasOrAdd is not implemented
func (cache *TxCache) HasOrAdd(_ []byte, _ interface{}, _ int) (has, added bool) {
	log.Error("TxCache.HasOrAdd is not implemented")
	return false, false
}

// Remove removes tx by hash
func (cache *TxCache) Remove(key []byte) {
	_, _ = cache.RemoveTxByHash(key)
}

// Keys returns the tx hashes in the cache
func (cache *TxCache) Keys() [][]byte {
	return cache.txByHash.keys()
}

// MaxSize is not implemented
func (cache *TxCache) MaxSize() int {
	return int(cache.config.CountThreshold)
}

// RegisterHandler is not implemented
func (cache *TxCache) RegisterHandler(func(key []byte, value interface{}), string) {
	log.Error("TxCache.RegisterHandler is not implemented")
}

// UnRegisterHandler is not implemented
func (cache *TxCache) UnRegisterHandler(string) {
	log.Error("TxCache.UnRegisterHandler is not implemented")
}

// NotifyAccountNonce should be called by external components (such as interceptors and transactions processor)
// in order to inform the cache about initial nonce gap phenomena
func (cache *TxCache) NotifyAccountNonce(accountKey []byte, nonce uint64) {
	cache.txListBySender.notifyAccountNonce(accountKey, nonce)
}

// PendingFor returns number of pending TX for an account
func (cache *TxCache) PendingFor(accountKey []byte) (uint64, uint64) {
	return cache.txListBySender.PendingFor(accountKey)
}

// ImmunizeTxsAgainstEviction does nothing for this type of cache
func (cache *TxCache) ImmunizeTxsAgainstEviction(_ [][]byte) {
}

// GetCleanupStats returns statistics about the cleanup process
func (cache *TxCache) GetCleanupStats() map[string]interface{} {
	lastCleanupTime := cache.lastCleanupTime.GetUint64()
	var timeSinceLastCleanup int64
	if lastCleanupTime > 0 && lastCleanupTime <= math.MaxInt64 {
		timeSinceLastCleanup = time.Now().Unix() - int64(lastCleanupTime)
	}

	return map[string]interface{}{
		"totalExpiredRemoved":   cache.totalExpiredRemoved.GetUint64(),
		"totalCleanupRuns":      cache.totalCleanupRuns.GetUint64(),
		"lastCleanupDurationUs": cache.lastCleanupDuration.GetUint64(),
		"lastCleanupTime":       lastCleanupTime,
		"timeSinceLastCleanup":  timeSinceLastCleanup,
		"currentTxCount":        cache.CountTx(),
		"currentSenderCount":    cache.CountSenders(),
		"currentBytes":          cache.NumBytes(),
	}
}

// startPeriodicCleanup starts a goroutine that periodically cleans expired transactions
func (cache *TxCache) startPeriodicCleanup() {
	// Get cleanup interval from config
	interval := cache.config.CleanupInterval

	// Negative interval means cleanup is disabled
	if interval < 0 {
		log.Debug("TxCache.startPeriodicCleanup: Periodic cleanup disabled",
			"cache", cache.name,
		)
		return
	}

	ticker := time.NewTicker(interval)
	stop := make(chan struct{})

	cache.cleanupMutex.Lock()
	cache.cleanupTicker = ticker
	cache.cleanupStop = stop
	cache.cleanupMutex.Unlock()

	go func() {
		// Ensure ticker is always stopped when goroutine exits
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Wrap each cleanup tick in its own recover so the goroutine continues
				func() {
					defer func() {
						if r := recover(); r != nil {
							stack := debug.Stack()
							log.Error("TxCache.startPeriodicCleanup: panic during cleanup tick",
								"cache", cache.name,
								"panic", r,
								"stack", string(stack),
							)
						}
					}()
					cache.cleanExpiredTransactions()
				}()
			case <-stop:
				return
			}
		}
	}()

	log.Debug("TxCache.startPeriodicCleanup: Started periodic cleanup",
		"cache", cache.name,
		"interval", interval,
	)
}

// cleanExpiredTransactions removes expired transactions from the cache
func (cache *TxCache) cleanExpiredTransactions() {
	startTime := time.Now()
	currentTime := startTime.Unix()

	// Increment cleanup run counter
	cache.totalCleanupRuns.Increment()

	// Get all senders
	senders := cache.txListBySender.getSnapshotAscending()

	// Process expired transactions for all senders
	result := cache.processExpiredTransactions(senders, currentTime)

	// Perform bulk removal if needed
	cache.performBulkRemoval(result)

	// Update metrics and log
	cache.updateCleanupMetrics(result, startTime, currentTime, len(senders))
}

// expiredTransactionResult holds the result of processing expired transactions
type expiredTransactionResult struct {
	txsToRemove         [][]byte
	sendersToRemove     []string
	totalExpiredTxs     int
	totalExpiredSenders int
}

// processExpiredTransactions processes all senders and identifies expired transactions
func (cache *TxCache) processExpiredTransactions(senders []*txListForSender, currentTime int64) expiredTransactionResult {
	result := expiredTransactionResult{
		txsToRemove:     make([][]byte, 0),
		sendersToRemove: make([]string, 0),
	}

	for _, listForSender := range senders {
		expiredInfo := cache.processExpiredForSender(listForSender, currentTime)

		result.txsToRemove = append(result.txsToRemove, expiredInfo.expiredTxHashes...)
		result.totalExpiredTxs += expiredInfo.expiredCount

		if expiredInfo.shouldRemoveSender {
			result.sendersToRemove = append(result.sendersToRemove, listForSender.sender)
			result.totalExpiredSenders++
		}
	}

	return result
}

// senderExpiredInfo holds expired transaction info for a sender
type senderExpiredInfo struct {
	expiredTxHashes    [][]byte
	expiredCount       int
	shouldRemoveSender bool
}

// processExpiredForSender processes expired transactions for a single sender
func (cache *TxCache) processExpiredForSender(listForSender *txListForSender, currentTime int64) senderExpiredInfo {
	listForSender.mutex.Lock()
	defer listForSender.mutex.Unlock()

	info := senderExpiredInfo{
		expiredTxHashes: make([][]byte, 0),
	}

	hasRemainingTxs := false

	// Check each transaction for expiration
	for element := listForSender.items.Front(); element != nil; {
		nextElement := element.Next()

		if tx, ok := element.Value.(*WrappedTransaction); ok {
			if tx.ExpireOn < currentTime {
				listForSender.items.Remove(element)
				listForSender.onRemovedListElement(element)
				info.expiredTxHashes = append(info.expiredTxHashes, tx.TxHash)
				info.expiredCount++
			} else {
				hasRemainingTxs = true
			}
		}

		element = nextElement
	}

	// If sender has no remaining transactions, mark for removal
	info.shouldRemoveSender = !hasRemainingTxs && len(info.expiredTxHashes) > 0

	return info
}

// revalidateEmptySenders checks if senders are still empty before removal
// This prevents the TOCTOU race where new transactions could be added after
// the initial emptiness check but before the actual removal
func (cache *TxCache) revalidateEmptySenders(sendersToRemove []string) []string {
	validatedEmpty := make([]string, 0, len(sendersToRemove))

	for _, sender := range sendersToRemove {
		// Check if sender still has no transactions
		listForSender, exists := cache.txListBySender.getListForSender(sender)
		if !exists {
			// Sender already removed or doesn't exist
			continue
		}

		// Lock the sender to check if it's truly empty
		listForSender.mutex.Lock()
		isEmpty := listForSender.items.Len() == 0
		listForSender.mutex.Unlock()

		if isEmpty {
			validatedEmpty = append(validatedEmpty, sender)
		} else {
			log.Debug("TxCache.revalidateEmptySenders: Sender no longer empty, skipping removal",
				"cache", cache.name,
				"sender", sender,
				"txCount", listForSender.items.Len(),
			)
		}
	}

	return validatedEmpty
}

// performBulkRemoval removes expired transactions and senders in bulk
func (cache *TxCache) performBulkRemoval(result expiredTransactionResult) {
	if len(result.txsToRemove) == 0 {
		return
	}

	cache.mutTxOperation.Lock()
	defer cache.mutTxOperation.Unlock()

	cache.txByHash.RemoveTxsBulk(result.txsToRemove)

	// Re-validate sender emptiness before removal to prevent TOCTOU race condition
	// New transactions may have been added between the emptiness check and this removal
	if len(result.sendersToRemove) > 0 {
		validatedSendersToRemove := cache.revalidateEmptySenders(result.sendersToRemove)
		if len(validatedSendersToRemove) > 0 {
			cache.txListBySender.RemoveSendersBulk(validatedSendersToRemove)
		}
	}
}

// updateCleanupMetrics updates cleanup metrics and logs the results
func (cache *TxCache) updateCleanupMetrics(result expiredTransactionResult, startTime time.Time, currentTime int64, totalSenders int) {
	elapsed := time.Since(startTime)

	// Update metrics
	cache.totalExpiredRemoved.Add(int64(result.totalExpiredTxs))
	cache.lastCleanupDuration.Set(int64(elapsed.Microseconds()))
	cache.lastCleanupTime.Set(int64(currentTime))

	if result.totalExpiredTxs > 0 {
		log.Info("TxCache.cleanExpiredTransactions: Cleaned expired transactions",
			"cache", cache.name,
			"expiredTxs", result.totalExpiredTxs,
			"expiredSenders", result.totalExpiredSenders,
			"totalSenders", totalSenders,
			"duration", elapsed,
		)
	}
}

// Close stops the periodic cleanup and cleans up resources
func (cache *TxCache) Close() error {
	cache.cleanupMutex.Lock()
	defer cache.cleanupMutex.Unlock()

	if cache.cleanupStop != nil {
		select {
		case <-cache.cleanupStop:
			// Already closed
		default:
			close(cache.cleanupStop)
		}
		cache.cleanupStop = nil
	}

	// Stop the ticker as a safety net in case the goroutine panicked
	// before it could stop the ticker itself
	if cache.cleanupTicker != nil {
		cache.cleanupTicker.Stop()
		cache.cleanupTicker = nil
	}

	log.Debug("TxCache.Close: Stopped periodic cleanup",
		"cache", cache.name,
	)

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (cache *TxCache) IsInterfaceNil() bool {
	return cache == nil
}
