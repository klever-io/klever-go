package mock

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// GetGeneralConfig returns the common configuration used for testing
func GetGeneralConfig() config.Config {
	return config.Config{
		WhiteListPool:          getLRUCacheConfig(),
		WhiteListerVerifiedTxs: getLRUCacheConfig(),
		StoragePruning: config.StoragePruningConfig{
			Enabled:             false,
			CleanOldEpochsData:  false,
			NumEpochsToKeep:     3,
			NumActivePersisters: 3,
		},
		TrieSync: config.TrieSyncConfig{
			NumConcurrentTrieSyncers:  200,
			MaxHardCapForMissingNodes: 5000,
		},
		EvictionWaitingList: config.EvictionWaitingListConfig{
			Size: 100,
			DB: config.DBConfig{
				FilePath:          AddTimestampSuffix("EvictionWaitingList"),
				Type:              string(storageUnit.MemoryDB),
				BatchDelaySeconds: 30,
				MaxBatchSize:      6,
				MaxOpenFiles:      10,
			},
		},
		SmartContractDataPool: getLRUCacheConfig(),
		TrieSnapshotDB: config.DBConfig{
			FilePath:          AddTimestampSuffix("TrieSnapshot"),
			Type:              string(storageUnit.MemoryDB),
			BatchDelaySeconds: 30,
			MaxBatchSize:      6,
			MaxOpenFiles:      10,
		},
		StateTriesConfig: config.StateTriesConfig{
			CheckpointSlotsModulus:      100,
			AccountsStatePruningEnabled: false,
			PeerStatePruningEnabled:     false,
			KAppStatePruningEnabled:     false,
			MaxStateTrieLevelInMemory:   5,
			MaxPeerTrieLevelInMemory:    5,
			MaxKAppTrieLevelInMemory:    5,
		},
		AccountsTrieStorage: config.StorageConfig{
			Cache: getLRUCacheConfig(),
			DB: config.DBConfig{
				FilePath:          AddTimestampSuffix("AccountsTrie/MainDB"),
				Type:              string(storageUnit.MemoryDB),
				BatchDelaySeconds: 30,
				MaxBatchSize:      6,
				MaxOpenFiles:      10,
			},
		},
		PeerAccountsTrieStorage: config.StorageConfig{
			Cache: getLRUCacheConfig(),
			DB: config.DBConfig{
				FilePath:          AddTimestampSuffix("PeerAccountsTrie/MainDB"),
				Type:              string(storageUnit.MemoryDB),
				BatchDelaySeconds: 30,
				MaxBatchSize:      6,
				MaxOpenFiles:      10,
			},
		},
		KAppAccountsTrieStorage: config.StorageConfig{
			Cache: getLRUCacheConfig(),
			DB: config.DBConfig{
				FilePath:          AddTimestampSuffix("KAppAccountsTrie/MainDB"),
				Type:              string(storageUnit.MemoryDB),
				BatchDelaySeconds: 30,
				MaxBatchSize:      6,
				MaxOpenFiles:      10,
			},
		},
		LogsAndEvents: config.LogsAndEventsConfig{
			SaveInStorageEnabled: true,
			TxLogsStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("Logs"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 2,
					MaxBatchSize:      100,
					MaxOpenFiles:      10,
				},
			},
		},

		Storages: config.StoragesConfig{
			TxStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("Transactions"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			BlocksStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("MiniBlocks"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			HdrNonceHashStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("MetaHdrHashNonce"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			HeartbeatStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("HeartbeatStorage"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			StatusMetricsStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("StatusMetricsStorageDB"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			BootstrapStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("BootstrapData"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 1,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			SmartContractsStorage: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("SmartContractsStorage"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			SmartContractsStorageSimulate: config.StorageConfig{
				Cache: getLRUCacheConfig(),
				DB: config.DBConfig{
					FilePath:          AddTimestampSuffix("SmartContractsStorageSimulate"),
					Type:              string(storageUnit.MemoryDB),
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
		},

		TxDataPool: config.CacheConfig{
			Capacity:             10000,
			SizePerSender:        1000,
			SizeInBytes:          1000000000,
			SizeInBytesPerSender: 10000000,
			Shards:               1,
		},
		HeadersPoolConfig: config.HeadersPoolConfig{
			MaxHeadersPerShard:            100,
			NumElementsToRemoveOnEviction: 1,
		},
		TrieNodesDataPool: getLRUCacheConfig(),
	}
}

func getLRUCacheConfig() config.CacheConfig {
	return config.CacheConfig{
		Type:     "LRU",
		Capacity: 1000,
		Shards:   1,
	}
}
