package dataPool

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
)

var _ retriever.PoolsHolder = (*dataPool)(nil)

type dataPool struct {
	transactions   retriever.ShardedDataCacherNotifier
	headers        retriever.HeadersPool
	blocks         storage.Cacher
	trieNodes      storage.Cacher
	smartContracts storage.Cacher
	currBlockTxs   retriever.TransactionCacher
}

// NewDataPool creates a data pools holder object
func NewDataPool(
	transactions retriever.ShardedDataCacherNotifier,
	headers retriever.HeadersPool,
	trieNodes storage.Cacher,
	smartContracts storage.Cacher,
	currBlockTxs retriever.TransactionCacher,
) (*dataPool, error) {
	if check.IfNil(transactions) {
		return nil, common.ErrNilTxDataPool
	}
	if check.IfNil(headers) {
		return nil, common.ErrNilHeadersDataPool
	}
	if check.IfNil(currBlockTxs) {
		return nil, common.ErrNilCurrBlockTxs
	}
	if check.IfNil(trieNodes) {
		return nil, common.ErrNilTrieNodesPool
	}
	if check.IfNil(smartContracts) {
		return nil, common.ErrNilSmartContractsPool
	}

	return &dataPool{
		transactions:   transactions,
		headers:        headers,
		trieNodes:      trieNodes,
		smartContracts: smartContracts,
		currBlockTxs:   currBlockTxs,
	}, nil
}

// CurrentBlockTxs returns the holder for current block transactions
func (dp *dataPool) CurrentBlockTxs() retriever.TransactionCacher {
	return dp.currBlockTxs
}

// Transactions returns the holder for transactions
func (dp *dataPool) Transactions() retriever.ShardedDataCacherNotifier {
	return dp.transactions
}

// Headers returns the holder for headers
func (dp *dataPool) Headers() retriever.HeadersPool {
	return dp.headers
}

// Blocks returns the holder for blocks
func (dp *dataPool) Blocks() storage.Cacher {
	return dp.blocks
}

// SmartContracts returns the holder for smart contracts
func (dp *dataPool) SmartContracts() storage.Cacher {
	return dp.smartContracts
}

// TrieNodes returns the holder for trie nodes
func (dp *dataPool) TrieNodes() storage.Cacher {
	return dp.trieNodes
}

// IsInterfaceNil returns true if there is no value under the interface
func (dp *dataPool) IsInterfaceNil() bool {
	return dp == nil
}
