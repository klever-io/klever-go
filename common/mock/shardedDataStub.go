package mock

import (
	"github.com/klever-io/klever-go/core/counting"
	"github.com/klever-io/klever-go/storage"
)

// ShardedDataStub -
type ShardedDataStub struct {
	RegisterOnAddedCalled                  func(func(key []byte, value interface{}))
	ShardDataStoreCalled                   func(cacheID string) storage.Cacher
	AddDataCalled                          func(key []byte, data interface{}, sizeInBytes int, cacheID string)
	NotifyCalled                           func(key []byte, data interface{}, sizeInBytes int)
	SearchFirstDataCalled                  func(key []byte) (value interface{}, ok bool)
	RemoveDataCalled                       func(key []byte, cacheID string)
	RemoveDataFromAllShardsCalled          func(key []byte)
	MoveDataCalled                         func(sourceCacheID, destCacheID string, key [][]byte)
	ClearCalled                            func()
	ClearShardStoreCalled                  func(cacheID string)
	RemoveSetOfDataFromPoolCalled          func(keys [][]byte, destCacheID string)
	ImmunizeSetOfDataAgainstEvictionCalled func(keys [][]byte, cacheID string)
	CreateShardStoreCalled                 func(destCacheID string)
	GetCountsCalled                        func() counting.CountsWithSize
	GetPaginatedCalled                     func(cacheID string, page int, pageSize int) ([]interface{}, int)
	GetSenderPaginatedCalled               func(cacheID string, sender []byte, page int, pageSize int) ([]interface{}, int)
}

// NewShardedDataStub -
func NewShardedDataStub() *ShardedDataStub {
	return &ShardedDataStub{}
}

// RegisterOnAdded -
func (shardedData *ShardedDataStub) RegisterOnAdded(handler func(key []byte, value interface{})) {
	if shardedData.RegisterOnAddedCalled != nil {
		shardedData.RegisterOnAddedCalled(handler)
	}
}

// ShardDataStore -
func (shardedData *ShardedDataStub) ShardDataStore(cacheID string) storage.Cacher {
	return shardedData.ShardDataStoreCalled(cacheID)
}

// AddData -
func (shardedData *ShardedDataStub) AddData(key []byte, data interface{}, sizeInBytes int, cacheID string) {
	if shardedData.AddDataCalled != nil {
		shardedData.AddDataCalled(key, data, sizeInBytes, cacheID)
	}
}

// Notify -
func (shardedData *ShardedDataStub) Notify(key []byte, data interface{}, sizeInBytes int) {
	if shardedData.NotifyCalled != nil {
		shardedData.NotifyCalled(key, data, sizeInBytes)
	}
}

// SearchFirstData -
func (shardedData *ShardedDataStub) SearchFirstData(key []byte) (value interface{}, ok bool) {
	if shardedData.SearchFirstDataCalled != nil {
		return shardedData.SearchFirstDataCalled(key)
	}
	return nil, false
}

// RemoveData -
func (shardedData *ShardedDataStub) RemoveData(key []byte, cacheID string) {
	shardedData.RemoveDataCalled(key, cacheID)
}

// RemoveDataFromAllShards -
func (shardedData *ShardedDataStub) RemoveDataFromAllShards(key []byte) {
	shardedData.RemoveDataFromAllShardsCalled(key)
}

// Clear -
func (shardedData *ShardedDataStub) Clear() {
	shardedData.ClearCalled()
}

// ClearShardStore -
func (shardedData *ShardedDataStub) ClearShardStore(cacheID string) {
	shardedData.ClearShardStoreCalled(cacheID)
}

// RemoveSetOfDataFromPool -
func (shardedData *ShardedDataStub) RemoveSetOfDataFromPool(keys [][]byte, cacheID string) {
	shardedData.RemoveSetOfDataFromPoolCalled(keys, cacheID)
}

// ImmunizeSetOfDataAgainstEviction -
func (shardedData *ShardedDataStub) ImmunizeSetOfDataAgainstEviction(keys [][]byte, cacheID string) {
	if shardedData.ImmunizeSetOfDataAgainstEvictionCalled != nil {
		shardedData.ImmunizeSetOfDataAgainstEvictionCalled(keys, cacheID)
	}
}

// GetCounts -
func (sd *ShardedDataStub) GetCounts() counting.CountsWithSize {
	if sd.GetCountsCalled != nil {
		return sd.GetCountsCalled()
	}

	return &counting.NullCounts{}
}

// GetPaginated -
func (sd *ShardedDataStub) GetPaginated(cacheID string, page int, pageSize int) ([]interface{}, int) {
	return sd.GetPaginatedCalled(cacheID, page, pageSize)
}

// GetSenderPaginated -
func (sd *ShardedDataStub) GetSenderPaginated(cacheID string, sender []byte, page int, pageSize int) ([]interface{}, int) {
	return sd.GetSenderPaginatedCalled(cacheID, sender, page, pageSize)
}

// IsInterfaceNil returns true if there is no value under the interface
func (shardedData *ShardedDataStub) IsInterfaceNil() bool {
	return shardedData == nil
}
