package txpool

import (
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core/counting"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/txcache"
)

var _ retriever.ShardedDataCacherNotifier = (*shardedTxPool)(nil)

var log = logger.GetOrCreate("txpool")

const txPoolTTL = 1 * time.Minute

// shardedTxPool holds transaction caches organised by source & destination shard
type shardedTxPool struct {
	mutexBackingMap   sync.RWMutex
	backingMap        map[string]*txPoolShard
	mutexAddCallbacks sync.RWMutex
	onAddCallbacks    []func(key []byte, value interface{})
	configPrototype   txcache.Config
}

type txPoolShard struct {
	CacheID string
	Cache   txCache
}

// NewShardedTxPool creates a new sharded tx pool
// Implements "retriever.TxPool"
func NewShardedTxPool(args ArgShardedTxPool) (*shardedTxPool, error) {
	log.Debug("NewShardedTxPool", "args", args.String())

	err := args.verify()
	if err != nil {
		return nil, err
	}

	halfOfSizeInBytes := args.Config.SizeInBytes / 2
	halfOfCapacity := args.Config.Capacity / 2

	configPrototype := txcache.Config{
		NumChunks:                     args.Config.Shards,
		EvictionEnabled:               true,
		NumBytesThreshold:             uint32(halfOfSizeInBytes),
		CountThreshold:                halfOfCapacity,
		NumBytesPerSenderThreshold:    args.Config.SizeInBytesPerSender,
		CountPerSenderThreshold:       args.Config.SizePerSender,
		NumSendersToPreemptivelyEvict: retriever.TxPoolNumSendersToPreemptivelyEvict,
	}

	shardedTxPoolObject := &shardedTxPool{
		mutexBackingMap:   sync.RWMutex{},
		backingMap:        make(map[string]*txPoolShard),
		mutexAddCallbacks: sync.RWMutex{},
		onAddCallbacks:    make([]func(key []byte, value interface{}), 0),
		configPrototype:   configPrototype,
	}

	return shardedTxPoolObject, nil
}

// ShardDataStore returns the requested cache, as the generic Cacher interface
func (txPool *shardedTxPool) ShardDataStore(cacheID string) storage.Cacher {
	cache := txPool.getTxCache(cacheID)
	return cache
}

// getTxCache returns the requested cache
func (txPool *shardedTxPool) getTxCache(cacheID string) txCache {
	shard := txPool.getOrCreateShard(cacheID)
	return shard.Cache
}

func (txPool *shardedTxPool) getOrCreateShard(cacheID string) *txPoolShard {
	txPool.mutexBackingMap.RLock()
	shard, ok := txPool.backingMap[cacheID]
	txPool.mutexBackingMap.RUnlock()

	if ok {
		return shard
	}

	shard = txPool.createShard(cacheID)
	return shard
}

func (txPool *shardedTxPool) createShard(cacheID string) *txPoolShard {
	txPool.mutexBackingMap.Lock()
	defer txPool.mutexBackingMap.Unlock()

	shard, ok := txPool.backingMap[cacheID]
	if !ok {
		cache := txPool.createTxCache(cacheID)
		shard = &txPoolShard{
			CacheID: cacheID,
			Cache:   cache,
		}

		txPool.backingMap[cacheID] = shard
	}

	return shard
}

func (txPool *shardedTxPool) createTxCache(cacheID string) txCache {
	config := txPool.configPrototype
	config.Name = cacheID
	cache, err := txcache.NewTxCache(config)
	if err != nil {
		log.Error("shardedTxPool.createTxCache()", "err", err)
		return txcache.NewDisabledCache()
	}

	return cache
}

// ImmunizeSetOfDataAgainstEviction marks the items as non-evictable
func (txPool *shardedTxPool) ImmunizeSetOfDataAgainstEviction(keys [][]byte, cacheID string) {
	shard := txPool.getOrCreateShard(cacheID)
	shard.Cache.ImmunizeTxsAgainstEviction(keys)
}

// AddData adds the transaction to the cache
func (txPool *shardedTxPool) AddData(key []byte, value interface{}, sizeInBytes int, cacheID string) {
	valueAsTransaction, ok := value.(data.TransactionHandler)
	if !ok {
		return
	}

	wrapper := &txcache.WrappedTransaction{
		Tx:     valueAsTransaction,
		TxHash: key,
		Size:   int64(sizeInBytes),
		// TODO: move to pool config
		ExpireOn: time.Now().Add(txPoolTTL).Unix(),
	}

	txPool.addTx(wrapper, cacheID)
}

// addTx adds the transaction to the cache
func (txPool *shardedTxPool) addTx(tx *txcache.WrappedTransaction, cacheID string) {
	shard := txPool.getOrCreateShard(cacheID)
	cache := shard.Cache
	_, added := cache.AddTx(tx)
	if added {
		txPool.onAdded(tx.TxHash, tx)
	}
}

func (txPool *shardedTxPool) Notify(txHash []byte, value interface{}, sizeInBytes int) {
	valueAsTransaction, ok := value.(data.TransactionHandler)
	if !ok {
		return
	}

	wrapper := &txcache.WrappedTransaction{
		Tx:     valueAsTransaction,
		TxHash: txHash,
		Size:   int64(sizeInBytes),
	}

	txPool.onAdded(txHash, wrapper)
}

func (txPool *shardedTxPool) onAdded(key []byte, value interface{}) {
	txPool.mutexAddCallbacks.RLock()
	defer txPool.mutexAddCallbacks.RUnlock()

	for _, handler := range txPool.onAddCallbacks {
		handler(key, value)
	}
}

// SearchFirstData searches the transaction against all shard data store, retrieving the first found
func (txPool *shardedTxPool) SearchFirstData(key []byte) (interface{}, bool) {
	tx, ok := txPool.searchFirstTx(key)
	return tx, ok
}

// searchFirstTx searches the transaction against all shard data store, retrieving the first found
func (txPool *shardedTxPool) searchFirstTx(txHash []byte) (tx data.TransactionHandler, ok bool) {
	txPool.mutexBackingMap.RLock()
	defer txPool.mutexBackingMap.RUnlock()

	var txFromCache *txcache.WrappedTransaction
	var hashExists bool
	for _, shard := range txPool.backingMap {
		txFromCache, hashExists = shard.Cache.GetByTxHash(txHash)
		if hashExists {
			return txFromCache.Tx, true
		}
	}

	return nil, false
}

// RemoveData removes the transaction from the pool
func (txPool *shardedTxPool) RemoveData(key []byte, cacheID string) {
	shard := txPool.getOrCreateShard(cacheID)
	shard.Cache.RemoveTxByHash(key)
}

// GetPaginated list MemPool paginated
func (txPool *shardedTxPool) GetPaginated(cacheID string, page int, pageSize int) ([]interface{}, int) {
	shard := txPool.getOrCreateShard(cacheID)
	keys := shard.Cache.Keys()
	idx := page * pageSize
	if idx >= len(keys) {
		return make([]interface{}, 0), len(keys)
	}
	idxEnd := idx + pageSize
	if idxEnd > len(keys) {
		idxEnd = len(keys)
	}

	result := make([]interface{}, idxEnd-idx)
	count := 0
	for i := idx; i < idxEnd; i++ {
		txObj, found := shard.Cache.Get(keys[i])
		if !found {
			continue
		}
		result[count] = txObj
		count++
	}

	return result, len(keys)
}

// GetKeysPaginated list MemPool paginated
func (txPool *shardedTxPool) GetSenderPaginated(cacheID string, sender []byte, page int, pageSize int) ([]interface{}, int) {
	shard := txPool.getOrCreateShard(cacheID)

	return shard.Cache.GetBySenderPaginated(sender, page, pageSize)
}

// RemoveSetOfDataFromPool removes a bunch of transactions from the pool
func (txPool *shardedTxPool) RemoveSetOfDataFromPool(keys [][]byte, cacheID string) {
	txPool.removeTxBulk(keys, cacheID)
}

// removeTxBulk removes a bunch of transactions from the pool
func (txPool *shardedTxPool) removeTxBulk(txHashes [][]byte, cacheID string) {
	shard := txPool.getOrCreateShard(cacheID)
	numRemoved := 0
	sendersNonce := make(map[string]uint64)
	for _, key := range txHashes {
		//
		if tx, ok := shard.Cache.RemoveTxByHash(key); ok {
			if sendersNonce[string(tx.Tx.GetSender())] < tx.Tx.GetNonce() {
				sendersNonce[string(tx.Tx.GetSender())] = tx.Tx.GetNonce()
			}
			numRemoved++
		}
	}
	log.Debug("shardedTxPool.removeTxBulk()", "name", cacheID, "numToRemove", len(txHashes), "numRemoved", numRemoved)

	// cleanup lower nonce
	accRemoved := 0
	for sender, nonce := range sendersNonce {
		numRemoved = shard.Cache.RemoveTxBySenderNonce([]byte(sender), nonce)
		accRemoved += numRemoved
	}
	log.Debug("shardedTxPool.removeTxBulk()", "cleanup", accRemoved)

}

// RemoveDataFromAllShards removes the transaction from the pool (it searches in all shards)
func (txPool *shardedTxPool) RemoveDataFromAllShards(key []byte) {
	txPool.removeTxFromAllShards(key)
}

// removeTxFromAllShards removes the transaction from the pool (it searches in all shards)
func (txPool *shardedTxPool) removeTxFromAllShards(txHash []byte) {
	txPool.mutexBackingMap.RLock()
	defer txPool.mutexBackingMap.RUnlock()

	for _, shard := range txPool.backingMap {
		cache := shard.Cache
		_, _ = cache.RemoveTxByHash(txHash)
	}
}

// Clear clears everything in the pool
func (txPool *shardedTxPool) Clear() {
	txPool.mutexBackingMap.Lock()
	txPool.backingMap = make(map[string]*txPoolShard)
	txPool.mutexBackingMap.Unlock()
}

// ClearShardStore clears a specific cache
func (txPool *shardedTxPool) ClearShardStore(cacheID string) {
	shard := txPool.getOrCreateShard(cacheID)
	shard.Cache.Clear()
}

// RegisterOnAdded registers a new handler to be called when a new transaction is added
func (txPool *shardedTxPool) RegisterOnAdded(handler func(key []byte, value interface{})) {
	if handler == nil {
		log.Error("attempt to register a nil handler")
		return
	}

	txPool.mutexAddCallbacks.Lock()
	txPool.onAddCallbacks = append(txPool.onAddCallbacks, handler)
	txPool.mutexAddCallbacks.Unlock()
}

// GetCounts returns the total number of transactions in the pool
func (txPool *shardedTxPool) GetCounts() counting.CountsWithSize {
	txPool.mutexBackingMap.RLock()
	defer txPool.mutexBackingMap.RUnlock()

	counts := counting.NewConcurrentShardedCountsWithSize()

	for cacheID, shard := range txPool.backingMap {
		cache := shard.Cache
		counts.PutCounts(cacheID, int64(cache.Len()), int64(cache.NumBytes()))
	}

	return counts
}

// Diagnose diagnoses the internal caches
func (txPool *shardedTxPool) Diagnose(deep bool) {
	log.Debug("shardedTxPool.Diagnose()", "counts", txPool.GetCounts().String())

	txPool.mutexBackingMap.RLock()
	defer txPool.mutexBackingMap.RUnlock()

	for _, shard := range txPool.backingMap {
		shard.Cache.Diagnose(deep)
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (txPool *shardedTxPool) IsInterfaceNil() bool {
	return txPool == nil
}
