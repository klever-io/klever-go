package factory

import (
	"path/filepath"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/clean"
	"github.com/klever-io/klever-go/storage/pruning"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("storage/factory")

const (
	minimumNumberOfActivePersisters = 1
	minimumNumberOfEpochsToKeep     = 2
)

// StorageServiceFactory handles the creation of storage services for both meta and shards
type StorageServiceFactory struct {
	generalConfig          *config.Config
	pathManager            storage.PathManagerHandler
	epochStartNotifier     storage.EpochStartNotifier
	oldDataCleanerProvider clean.OldDataCleanerProvider
	currentEpoch           uint32
}

// NewStorageServiceFactory will return a new instance of StorageServiceFactory
func NewStorageServiceFactory(
	config *config.Config,
	pathManager storage.PathManagerHandler,
	epochStartNotifier storage.EpochStartNotifier,
	currentEpoch uint32,
) (*StorageServiceFactory, error) {
	if config == nil {
		return nil, common.ErrNilConfig
	}
	if config.StoragePruning.NumEpochsToKeep < minimumNumberOfEpochsToKeep && config.StoragePruning.CleanOldEpochsData {
		return nil, common.ErrInvalidNumberOfEpochsToSave
	}
	if config.StoragePruning.NumActivePersisters < minimumNumberOfActivePersisters {
		return nil, common.ErrInvalidNumberOfActivePersisters
	}
	if check.IfNil(pathManager) {
		return nil, common.ErrNilPathManager
	}
	if check.IfNil(epochStartNotifier) {
		return nil, common.ErrNilEpochStartNotifier
	}

	oldDataCleanProvider, err := clean.NewOldDataCleanerProvider(
		config.StoragePruning,
	)
	if err != nil {
		return nil, err
	}

	return &StorageServiceFactory{
		generalConfig:          config,
		pathManager:            pathManager,
		epochStartNotifier:     epochStartNotifier,
		currentEpoch:           currentEpoch,
		oldDataCleanerProvider: oldDataCleanProvider,
	}, nil
}

// CreateForMeta will return the storage service which contains all storers needed for metachain
func (psf *StorageServiceFactory) CreateForMeta() (retriever.StorageService, error) {
	var txUnit *pruning.PruningStorer
	var blockUnit *pruning.PruningStorer
	var bootstrapUnit *pruning.PruningStorer
	var txLogsUnit *pruning.PruningStorer
	var err error

	successfullyCreatedStorers := make([]storage.Storer, 0)

	defer func() {
		// cleanup
		if err != nil {
			log.Error("create meta store", "error", err.Error())
			for _, storer := range successfullyCreatedStorers {
				_ = storer.DestroyUnit()
			}
		}
	}()

	// metaHdrHashNonce is static
	metaHdrHashNonceUnitConfig := GetDBFromConfig(psf.generalConfig.Storages.HdrNonceHashStorage.DB)
	dbPath := psf.pathManager.PathForStatic(psf.generalConfig.Storages.HdrNonceHashStorage.DB.FilePath)
	metaHdrHashNonceUnitConfig.FilePath = dbPath
	metaHdrHashNonceUnit, err := storageUnit.NewStorageUnitFromConf(
		GetCacherFromConfig(psf.generalConfig.Storages.HdrNonceHashStorage.Cache),
		metaHdrHashNonceUnitConfig,
		GetBloomFromConfig(psf.generalConfig.Storages.HdrNonceHashStorage.Bloom))
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, metaHdrHashNonceUnit)

	heartbeatDbConfig := GetDBFromConfig(psf.generalConfig.Storages.HeartbeatStorage.DB)
	dbPath = psf.pathManager.PathForStatic(psf.generalConfig.Storages.HeartbeatStorage.DB.FilePath)
	heartbeatDbConfig.FilePath = dbPath
	heartbeatStorageUnit, err := storageUnit.NewStorageUnitFromConf(
		GetCacherFromConfig(psf.generalConfig.Storages.HeartbeatStorage.Cache),
		heartbeatDbConfig,
		GetBloomFromConfig(psf.generalConfig.Storages.HeartbeatStorage.Bloom))
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, heartbeatStorageUnit)

	statusMetricsDbConfig := GetDBFromConfig(psf.generalConfig.Storages.StatusMetricsStorage.DB)
	dbPath = psf.pathManager.PathForStatic(psf.generalConfig.Storages.StatusMetricsStorage.DB.FilePath)
	statusMetricsDbConfig.FilePath = dbPath
	statusMetricsStorageUnit, err := storageUnit.NewStorageUnitFromConf(
		GetCacherFromConfig(psf.generalConfig.Storages.StatusMetricsStorage.Cache),
		statusMetricsDbConfig,
		GetBloomFromConfig(psf.generalConfig.Storages.StatusMetricsStorage.Bloom))
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, statusMetricsStorageUnit)

	txUnitArgs := psf.createPruningStorerArgs(psf.generalConfig.Storages.TxStorage)
	txUnit, err = pruning.NewPruningStorer(txUnitArgs)
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, txUnit)

	blockUnitArgs := psf.createPruningStorerArgs(psf.generalConfig.Storages.BlocksStorage)
	blockUnit, err = pruning.NewPruningStorer(blockUnitArgs)
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, blockUnit)

	bootstrapUnitArgs := psf.createPruningStorerArgs(psf.generalConfig.Storages.BootstrapStorage)
	bootstrapUnit, err = pruning.NewPruningStorer(bootstrapUnitArgs)
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, bootstrapUnit)

	txLogsUnitArgs := psf.createPruningStorerArgs(psf.generalConfig.LogsAndEvents.TxLogsStorage)
	txLogsUnit, err = pruning.NewPruningStorer(txLogsUnitArgs)
	if err != nil {
		return nil, err
	}
	successfullyCreatedStorers = append(successfullyCreatedStorers, txLogsUnit)

	store := retriever.NewChainStorer()
	store.AddStorer(retriever.BlockUnit, blockUnit)
	store.AddStorer(retriever.HdrNonceHashDataUnit, metaHdrHashNonceUnit)
	store.AddStorer(retriever.TransactionUnit, txUnit)
	store.AddStorer(retriever.HeartbeatUnit, heartbeatStorageUnit)
	store.AddStorer(retriever.BootstrapUnit, bootstrapUnit)
	store.AddStorer(retriever.StatusMetricsUnit, statusMetricsStorageUnit)
	store.AddStorer(retriever.TxLogsUnit, txLogsUnit)

	err = psf.setupDbLookupExtensions(store, &successfullyCreatedStorers)
	if err != nil {
		return nil, err
	}

	err = psf.initOldDatabasesCleaningIfNeeded(store)
	if err != nil {
		return nil, err
	}

	return store, err
}

func (psf *StorageServiceFactory) setupDbLookupExtensions(chainStorer *retriever.ChainStorer, createdStorers *[]storage.Storer) error {
	if !psf.generalConfig.DbLookupExtensions.Enabled {
		return nil
	}

	// Create the blocksMetadata (PRUNING) storer
	blocksMetadataConfig := psf.generalConfig.DbLookupExtensions.BlocksMetadataStorage
	blocksMetadataPruningStorerArgs := psf.createPruningStorerArgs(blocksMetadataConfig)
	blocksMetadataPruningStorer, err := pruning.NewPruningStorer(blocksMetadataPruningStorerArgs)
	if err != nil {
		return err
	}

	*createdStorers = append(*createdStorers, blocksMetadataPruningStorer)
	chainStorer.AddStorer(retriever.BlockUnit, blocksMetadataPruningStorer)

	// Create the blocksHashByTxHash (STATIC) storer
	blockHashByTxHashConfig := psf.generalConfig.DbLookupExtensions.BlockHashByTxHashStorage
	blockHashByTxHashDbConfig := GetDBFromConfig(blockHashByTxHashConfig.DB)
	blockHashByTxHashDbConfig.FilePath = psf.pathManager.PathForStatic(blockHashByTxHashConfig.DB.FilePath)
	blockHashByTxHashCacherConfig := GetCacherFromConfig(blockHashByTxHashConfig.Cache)
	blockHashByTxHashBloomFilter := GetBloomFromConfig(blockHashByTxHashConfig.Bloom)
	blockHashByTxHashUnit, err := storageUnit.NewStorageUnitFromConf(blockHashByTxHashCacherConfig, blockHashByTxHashDbConfig, blockHashByTxHashBloomFilter)
	if err != nil {
		return err
	}

	*createdStorers = append(*createdStorers, blockHashByTxHashUnit)
	chainStorer.AddStorer(retriever.BlockHashByTxHashUnit, blockHashByTxHashUnit)

	// Create the epochByHash (STATIC) storer
	epochByHashConfig := psf.generalConfig.DbLookupExtensions.EpochByHashStorage
	epochByHashDbConfig := GetDBFromConfig(epochByHashConfig.DB)
	epochByHashDbConfig.FilePath = psf.pathManager.PathForStatic(epochByHashConfig.DB.FilePath)
	epochByHashCacherConfig := GetCacherFromConfig(epochByHashConfig.Cache)
	epochByHashBloomFilter := GetBloomFromConfig(epochByHashConfig.Bloom)
	epochByHashUnit, err := storageUnit.NewStorageUnitFromConf(epochByHashCacherConfig, epochByHashDbConfig, epochByHashBloomFilter)
	if err != nil {
		return err
	}

	*createdStorers = append(*createdStorers, epochByHashUnit)
	chainStorer.AddStorer(retriever.EpochByHashUnit, epochByHashUnit)

	return nil
}

func (psf *StorageServiceFactory) createPruningStorerArgs(storageConfig config.StorageConfig) *pruning.StorerArgs {
	cleanOldEpochsData := psf.generalConfig.StoragePruning.CleanOldEpochsData
	// #nosec G115 - we are not using the value from the config directly
	numOfEpochsToKeep := uint32(psf.generalConfig.StoragePruning.NumEpochsToKeep)
	// #nosec G115 - we are not using the value from the config directly
	numOfActivePersisters := uint32(psf.generalConfig.StoragePruning.NumActivePersisters)
	pruningEnabled := psf.generalConfig.StoragePruning.Enabled
	dbPath := filepath.Join(psf.pathManager.PathForEpoch(psf.currentEpoch, storageConfig.DB.FilePath))
	args := &pruning.StorerArgs{
		Identifier:                storageConfig.DB.FilePath,
		PruningEnabled:            pruningEnabled,
		StartingEpoch:             psf.currentEpoch,
		CleanOldEpochsData:        cleanOldEpochsData,
		CacheConf:                 GetCacherFromConfig(storageConfig.Cache),
		PathManager:               psf.pathManager,
		DbPath:                    dbPath,
		PersisterFactory:          NewPersisterFactory(storageConfig.DB),
		BloomFilterConf:           GetBloomFromConfig(storageConfig.Bloom),
		NumOfEpochsToKeep:         numOfEpochsToKeep,
		NumOfActivePersisters:     numOfActivePersisters,
		Notifier:                  psf.epochStartNotifier,
		MaxBatchSize:              storageConfig.DB.MaxBatchSize,
		EnabledDbLookupExtensions: psf.generalConfig.DbLookupExtensions.Enabled,
	}

	return args
}

func (psf *StorageServiceFactory) initOldDatabasesCleaningIfNeeded(store retriever.StorageService) error {
	// TODO: check if full Archive
	if !psf.generalConfig.StoragePruning.CleanOldEpochsData ||
		!psf.generalConfig.StoragePruning.Enabled {
		return nil
	}

	_, err := clean.NewOldDatabaseCleaner(clean.ArgsOldDatabaseCleaner{
		DatabasePath:           psf.pathManager.DatabasePath(),
		StorageListProvider:    store,
		EpochStartNotifier:     psf.epochStartNotifier,
		OldDataCleanerProvider: psf.oldDataCleanerProvider,
	})

	return err
}
