package mock

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/dataPool"
	"github.com/klever-io/klever-go/data/retriever/dataPool/headersCache"
	"github.com/klever-io/klever-go/data/retriever/txpool"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// PoolsHolderMock -
type PoolsHolderMock struct {
	transactions   retriever.ShardedDataCacherNotifier
	headers        retriever.HeadersPool
	blocks         storage.Cacher
	trieNodes      storage.Cacher
	smartContracts storage.Cacher
	currBlockTxs   retriever.TransactionCacher
}

// NewPoolsHolderMock -
func NewPoolsHolderMock() *PoolsHolderMock {
	var err error
	holder := &PoolsHolderMock{}

	holder.transactions, err = txpool.NewShardedTxPool(
		txpool.ArgShardedTxPool{
			Config: storageUnit.CacheConfig{
				Capacity:             100000,
				SizePerSender:        1000,
				SizeInBytes:          1000000000,
				SizeInBytesPerSender: 10000000,
				Shards:               16,
			},
		},
	)
	panicIfError("NewPoolsHolderMock", err)

	holder.headers, err = headersCache.NewHeadersPool(config.HeadersPoolConfig{MaxHeadersPerShard: 1000, NumElementsToRemoveOnEviction: 100})
	panicIfError("NewPoolsHolderMock", err)

	holder.blocks, err = storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10000, Shards: 1, SizeInBytes: 0})
	panicIfError("NewPoolsHolderMock", err)

	holder.currBlockTxs, err = dataPool.NewCurrentBlockPool()
	panicIfError("NewPoolsHolderMock", err)

	holder.trieNodes, err = storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10000, Shards: 1, SizeInBytes: 0})
	panicIfError("NewPoolsHolderMock", err)

	return holder
}

// CurrentBlockTxs -
func (holder *PoolsHolderMock) CurrentBlockTxs() retriever.TransactionCacher {
	return holder.currBlockTxs
}

// Transactions -
func (holder *PoolsHolderMock) Transactions() retriever.ShardedDataCacherNotifier {
	return holder.transactions
}

// Headers -
func (holder *PoolsHolderMock) Headers() retriever.HeadersPool {
	return holder.headers
}

// Blocks -
func (holder *PoolsHolderMock) Blocks() storage.Cacher {
	return holder.blocks
}

// SetTransactions -
func (holder *PoolsHolderMock) SetTransactions(pool retriever.ShardedDataCacherNotifier) {
	holder.transactions = pool
}

// TrieNodes -
func (holder *PoolsHolderMock) TrieNodes() storage.Cacher {
	return holder.trieNodes
}

// SmartContracts -
func (holder *PoolsHolderMock) SmartContracts() storage.Cacher {
	return holder.smartContracts
}

// IsInterfaceNil returns true if there is no value under the interface
func (holder *PoolsHolderMock) IsInterfaceNil() bool {
	return holder == nil
}
