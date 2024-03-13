package storage

import "errors"

// ErrKeyNotFound is raised when a key is not found
var ErrKeyNotFound = errors.New("key not found")

// ErrSerialDBIsClosed is raised when the serialDB is closed
var ErrSerialDBIsClosed = errors.New("serialDB is closed")

// ErrInvalidBatch is raised when the used batch is invalid
var ErrInvalidBatch = errors.New("batch is invalid")

// ErrInvalidNumOpenFiles is raised when the max num of open files is less than 1
var ErrInvalidNumOpenFiles = errors.New("maxOpenFiles is invalid")

// ErrNilPersister is raised when a nil persister is provided
var ErrNilPersister = errors.New("expected not nil persister")

// ErrNilCacher is raised when a nil cacher is provided
var ErrNilCacher = errors.New("expected not nil cacher")

// ErrNilBloomFilter is raised when a nil bloom filter is provided
var ErrNilBloomFilter = errors.New("expected not nil bloom filter")

// ErrCacheSizeIsLowerThanBatchSize signals that size of cache is lower than size of batch
var ErrCacheSizeIsLowerThanBatchSize = errors.New("cache size is lower than batch size")

// ErrNotSupportedCacheType is raised when an unsupported cache type is provided
var ErrNotSupportedCacheType = errors.New("not supported cache type")

// ErrNotSupportedDBType is raised when an unsupported database type is provided
var ErrNotSupportedDBType = errors.New("not supported db type")

// ErrNotSupportedHashType is raised when an unsupported hasher is provided
var ErrNotSupportedHashType = errors.New("hash type not supported")

// ErrCacheSizeInvalid signals that size of cache is less than 1
var ErrCacheSizeInvalid = errors.New("cache size is less than 1")

// ErrCacheCapacityInvalid signals that capacity of cache is less than 1
var ErrCacheCapacityInvalid = errors.New("cache capacity is less than 1")

// ErrLRUCacheWithProvidedSize signals that a simple LRU cache is wanted but the user provided a positive size in bytes value
var ErrLRUCacheWithProvidedSize = errors.New("LRU cache does not support size in bytes")

// ErrLRUCacheInvalidSize signals that the provided size in bytes value for LRU cache is invalid
var ErrLRUCacheInvalidSize = errors.New("wrong size in bytes value for LRU cache")

// ErrNegativeSizeInBytes signals that the provided size in bytes value is negative
var ErrNegativeSizeInBytes = errors.New("negative size in bytes")

// ErrNilTimeCache signals that a nil time cache was provided
var ErrNilTimeCache = errors.New("nil time cache")

// ErrEmptyKey is raised when a key is empty
var ErrEmptyKey = errors.New("key is empty")

// ErrEmptyPruningPathTemplate signals that an empty path template for pruning storers has been provided
var ErrEmptyPruningPathTemplate = errors.New("empty path template for pruning storers")

// ErrEmptyStaticPathTemplate signals that an empty path template for static storers has been provided
var ErrEmptyStaticPathTemplate = errors.New("empty path template for static storers")

// ErrInvalidPruningPathTemplate signals that an invalid path template for pruning storers has been provided
var ErrInvalidPruningPathTemplate = errors.New("invalid path template for pruning storers")

// ErrInvalidStaticPathTemplate signals that an invalid path template for static storers has been provided
var ErrInvalidStaticPathTemplate = errors.New("invalid path template for static storers")

// ErrDestroyingUnit signals that the destroy unit method did not manage to destroy all the persisters in a pruning storer
var ErrDestroyingUnit = errors.New("destroy unit didn't remove all the persisters")

// ErrClosingPersisters signals that not all persisters were closed
var ErrClosingPersisters = errors.New("cannot close all the persisters")

// ErrInvalidConfig signals an invalid config
var ErrInvalidConfig = errors.New("invalid config")

// ErrImmuneItemsCapacityReached signals that capacity for immune items is reached
var ErrImmuneItemsCapacityReached = errors.New("capacity reached for immune items")

// ErrFailedCacheEviction signals a failed eviction within a cache
var ErrFailedCacheEviction = errors.New("failed eviction within cache")

// ErrItemAlreadyInCache signals that an item is already in cache
var ErrItemAlreadyInCache = errors.New("item already in cache")

// ErrBootstrapDataNotFoundInStorage signals that no BootstrapData was find in the storage
var ErrBootstrapDataNotFoundInStorage = errors.New("didn't find any bootstrap data in storage")

// ErrNilOldDataCleanerProvider signals that a nil old data cleaner provider has been given
var ErrNilOldDataCleanerProvider = errors.New("nil old data cleaner provider")

// ErrNilStorageListProvider signals that a nil storage list provided has been provided
var ErrNilStorageListProvider = errors.New("nil storage list provider")

// ErrOldestEpochNotAvailable signals that fetching the oldest epoch is not available
var ErrOldestEpochNotAvailable = errors.New("oldest epoch not available")

// ErrCannotComputeStorageOldestEpoch signals an issue when computing the oldest epoch for storage
var ErrCannotComputeStorageOldestEpoch = errors.New("could not compute the oldest epoch for storage")

// ErrInvalidDatabasePath signals that an invalid database path has been provided
var ErrInvalidDatabasePath = errors.New("invalid database path")

// ErrDBIsClosed is raised when the DB is closed
var ErrDBIsClosed = errors.New("DB is closed")
