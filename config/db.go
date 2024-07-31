package config

// CacheConfig will map the cache configuration
type CacheConfig struct {
	Name                 string `yaml:"name"`
	Type                 string `yaml:"type"`
	Capacity             uint32 `yaml:"capacity"`
	SizePerSender        uint32 `yaml:"sizePerSender"`
	SizeInBytes          uint64 `yaml:"sizeInBytes"`
	SizeInBytesPerSender uint32 `yaml:"sizeInBytesPerSender"`
	Shards               uint32 `yaml:"shards"`
}

// BloomFilterConfig will map the bloom filter configuration
type BloomFilterConfig struct {
	Size     uint     `yaml:"size"`
	HashFunc []string `yaml:"hashFunc"`
}

// StorageConfig will map the storage unit configuration
type StorageConfig struct {
	Cache CacheConfig       `yaml:"cache"`
	DB    DBConfig          `yaml:"db"`
	Bloom BloomFilterConfig `yaml:"bloom"`
}

// DBConfig will map the database configuration
type DBConfig struct {
	FilePath          string `yaml:"filePath"`
	Type              string `yaml:"type"`
	BatchDelaySeconds int    `yaml:"batchDelaySeconds"`
	MaxBatchSize      int    `yaml:"maxBatchSize"`
	MaxOpenFiles      int    `yaml:"maxOpenFiles"`
}

// ResourceStatsConfig will hold all resource stats settings
type ResourceStatsConfig struct {
	Enabled              bool `yaml:"enabled"`
	RefreshIntervalInSec int  `yaml:"refreshIntervalInSec"`
}

// ValidatorStatisticsConfig will hold all validator statistics settings
type ValidatorStatisticsConfig struct {
	CacheRefreshIntervalInSec int `yaml:"cacheRefreshIntervalInSec"`
}

// StoragePruningConfig will hold settings related to storage pruning
type StoragePruningConfig struct {
	Enabled             bool   `yaml:"enabled"`
	CleanOldEpochsData  bool   `yaml:"cleanOldEpochsData"`
	NumEpochsToKeep     uint64 `yaml:"numEpochsToKeep"`
	NumActivePersisters uint64 `yaml:"numActivePersisters"`
}

// StoragesConfig -
type StoragesConfig struct {
	BlocksStorage                   StorageConfig `yaml:"blocksStorage"`
	TxStorage                       StorageConfig `yaml:"txStorage"`
	HdrNonceHashStorage             StorageConfig `yaml:"hdrNonceHashStorage"`
	StatusMetricsStorage            StorageConfig `yaml:"statusMetricsStorage"`
	HeartbeatStorage                StorageConfig `yaml:"heartbeatStorage"`
	BootstrapStorage                StorageConfig `yaml:"bootstrapStorage"`
	SmartContractsStorage           StorageConfig `yaml:"smartContractsStorage"`
	SmartContractsStorageSimulate   StorageConfig `yaml:"smartContractsStorageSimulate"`
	SmartContractsStorageForSCQuery StorageConfig `yaml:"smartContractsStorageForSCQuery"`
}

// DbLookupExtensionsConfig holds the configuration for the db lookup extensions
type DbLookupExtensionsConfig struct {
	Enabled                  bool          `yaml:"enabled"`
	BlocksMetadataStorage    StorageConfig `yaml:"blocksMetadataStorage"`
	BlockHashByTxHashStorage StorageConfig `yaml:"blockHashByTxHashStorage"`
	EpochByHashStorage       StorageConfig `yaml:"epochByHashStorage"`
}

// TrieStorageManagerConfig will hold config information about trie storage manager
type TrieStorageManagerConfig struct {
	PruningBufferLen   uint32 `yaml:"pruningBufferLen"`
	SnapshotsBufferLen uint32 `yaml:"snapshotsBufferLen"`
	MaxSnapshots       uint32 `yaml:"maxSnapshots"`
}

// HeadersPoolConfig will map the headers cache configuration
type HeadersPoolConfig struct {
	MaxHeadersPerShard            int `yaml:"maxHeadersPerShard"`
	NumElementsToRemoveOnEviction int `yaml:"numElementsToRemoveOnEviction"`
}

// StateTriesConfig will hold information about state tries
type StateTriesConfig struct {
	CheckpointSlotsModulus      uint `yaml:"checkpointSlotsModulus"`
	AccountsStatePruningEnabled bool `yaml:"accountsStatePruningEnabled"`
	PeerStatePruningEnabled     bool `yaml:"peerStatePruningEnabled"`
	KAppStatePruningEnabled     bool `yaml:"kappStatePruningEnabled"`
	MaxStateTrieLevelInMemory   uint `yaml:"maxStateTrieLevelInMemory"`
	MaxPeerTrieLevelInMemory    uint `yaml:"maxPeerTrieLevelInMemory"`
	MaxKAppTrieLevelInMemory    uint `yaml:"maxKAppTrieLevelInMemory"`
}
