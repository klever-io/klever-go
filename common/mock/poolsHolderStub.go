package mock

import (
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
)

// PoolsHolderStub -
type PoolsHolderStub struct {
	HeadersCalled              func() retriever.HeadersPool
	TransactionsCalled         func() retriever.ShardedDataCacherNotifier
	SmartContractCalled        func() storage.Cacher
	UnsignedTransactionsCalled func() retriever.ShardedDataCacherNotifier
	RewardTransactionsCalled   func() retriever.ShardedDataCacherNotifier
	BlocksCalled               func() storage.Cacher
	MetaBlocksCalled           func() storage.Cacher
	CurrBlockTxsCalled         func() retriever.TransactionCacher
	TrieNodesCalled            func() storage.Cacher
	SmartContractsCalled       func() storage.Cacher
}

// NewPoolsHolderStub -
func NewPoolsHolderStub() *PoolsHolderStub {
	return &PoolsHolderStub{}
}

// Headers -
func (holder *PoolsHolderStub) Headers() retriever.HeadersPool {
	if holder.HeadersCalled != nil {
		return holder.HeadersCalled()
	}

	return nil
}

// Transactions -
func (holder *PoolsHolderStub) Transactions() retriever.ShardedDataCacherNotifier {
	if holder.TransactionsCalled != nil {
		return holder.TransactionsCalled()
	}
	var a retriever.ShardedDataCacherNotifier
	return a
}

func (holder *PoolsHolderStub) SmartContracts() storage.Cacher {
	return holder.SmartContractCalled()
}

// Blocks -
func (holder *PoolsHolderStub) Blocks() storage.Cacher {
	if holder.BlocksCalled != nil {
		return holder.BlocksCalled()
	}

	return NewCacherStub()
}

// MetaBlocks -
func (holder *PoolsHolderStub) MetaBlocks() storage.Cacher {
	if holder.MetaBlocksCalled != nil {
		return holder.MetaBlocksCalled()
	}

	return NewCacherStub()
}

// CurrentBlockTxs -
func (holder *PoolsHolderStub) CurrentBlockTxs() retriever.TransactionCacher {
	if holder.CurrBlockTxsCalled != nil {
		return holder.CurrBlockTxsCalled()
	}

	return nil
}

// TrieNodes -
func (holder *PoolsHolderStub) TrieNodes() storage.Cacher {
	if holder.TrieNodesCalled != nil {
		return holder.TrieNodesCalled()
	}

	return NewCacherStub()
}

// IsInterfaceNil returns true if there is no value under the interface
func (holder *PoolsHolderStub) IsInterfaceNil() bool {
	return holder == nil
}
