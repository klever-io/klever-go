package mock

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// SyncStateStub -
type SyncStateStub struct {
	GetEpochStartMetaBlockCalled func(epoch uint32) (*block.Block, error)
	SyncAllStateCalled           func(epoch uint32) error
	GetAllTriesCalled            func() (map[string]data.Trie, error)
	GetAllTransactionsCalled     func() (map[string]data.TransactionHandler, error)
}

// GetEpochStartMetaBlock -
func (sss *SyncStateStub) GetEpochStartMetaBlock(epoch uint32) (*block.Block, error) {
	if sss.GetEpochStartMetaBlockCalled != nil {
		return sss.GetEpochStartMetaBlockCalled(epoch)
	}
	return nil, nil
}

// SyncAllState -
func (sss *SyncStateStub) SyncAllState(epoch uint32) error {
	if sss.SyncAllStateCalled != nil {
		return sss.SyncAllStateCalled(epoch)
	}
	return nil
}

// GetAllTries -
func (sss *SyncStateStub) GetAllTries() (map[string]data.Trie, error) {
	if sss.GetAllTriesCalled != nil {
		return sss.GetAllTriesCalled()
	}
	return nil, nil
}

// GetAllTransactions -
func (sss *SyncStateStub) GetAllTransactions() (map[string]data.TransactionHandler, error) {
	if sss.GetAllTransactionsCalled != nil {
		return sss.GetAllTransactionsCalled()
	}
	return nil, nil
}

// IsInterfaceNil -
func (sss *SyncStateStub) IsInterfaceNil() bool {
	return sss == nil
}
