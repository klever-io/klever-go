package mock

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/dataPool"
	"github.com/klever-io/klever-go/data/retriever/dataPool/headersCache"
	"github.com/klever-io/klever-go/data/retriever/txpool"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// CreateTxPool -
func CreateTxPool(numShards uint32, selfShard uint32) (retriever.ShardedDataCacherNotifier, error) {
	return txpool.NewShardedTxPool(
		txpool.ArgShardedTxPool{
			Config: storageUnit.CacheConfig{
				Capacity:             100_000,
				SizePerSender:        1_000_000_000,
				SizeInBytes:          1_000_000_000,
				SizeInBytesPerSender: 33_554_432,
				Shards:               16,
			},
		},
	)
}

// CreatePoolsHolder -
func CreatePoolsHolder(numShards uint32, selfShard uint32) retriever.PoolsHolder {
	var err error

	txPool, err := CreateTxPool(numShards, selfShard)
	panicIfError("CreatePoolsHolder", err)

	headersPool, err := headersCache.NewHeadersPool(config.HeadersPoolConfig{
		MaxHeadersPerShard:            1000,
		NumElementsToRemoveOnEviction: 100,
	})
	panicIfError("CreatePoolsHolder", err)

	cacherConfig := storageUnit.CacheConfig{Capacity: 50000, Type: storageUnit.LRUCache}
	trieNodes, err := storageUnit.NewCache(cacherConfig)
	panicIfError("CreatePoolsHolder", err)

	scCacher, err := storageUnit.NewCache(cacherConfig)
	panicIfError("SCCacher", err)

	currentTx, err := dataPool.NewCurrentBlockPool()
	panicIfError("CreatePoolsHolder", err)

	holder, err := dataPool.NewDataPool(
		txPool,
		headersPool,
		trieNodes,
		scCacher,
		currentTx,
	)
	panicIfError("CreatePoolsHolder", err)

	return holder
}

// CreatePoolsHolderWithTxPool -
func CreatePoolsHolderWithTxPool(txPool retriever.ShardedDataCacherNotifier) retriever.PoolsHolder {
	var err error

	headersPool, err := headersCache.NewHeadersPool(config.HeadersPoolConfig{
		MaxHeadersPerShard:            1000,
		NumElementsToRemoveOnEviction: 100,
	})
	panicIfError("CreatePoolsHolderWithTxPool", err)

	cacherConfig := storageUnit.CacheConfig{Capacity: 50000, Type: storageUnit.LRUCache}
	trieNodes, err := storageUnit.NewCache(cacherConfig)
	panicIfError("CreatePoolsHolderWithTxPool", err)

	scCacher, err := storageUnit.NewCache(cacherConfig)
	panicIfError("SCCacher", err)

	currentTx, err := dataPool.NewCurrentBlockPool()
	panicIfError("CreatePoolsHolderWithTxPool", err)

	holder, err := dataPool.NewDataPool(
		txPool,
		headersPool,
		trieNodes,
		scCacher,
		currentTx,
	)
	panicIfError("CreatePoolsHolderWithTxPool", err)

	return holder
}
