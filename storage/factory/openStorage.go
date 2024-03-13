package factory

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgsNewOpenStorageUnits defines the arguments in order to open a set of storage units from disk
type ArgsNewOpenStorageUnits struct {
	GeneralConfig             config.Config
	Marshalizer               marshal.Marshalizer
	BootstrapDataProvider     BootstrapDataProviderHandler
	LatestStorageDataProvider storage.LatestStorageDataProviderHandler
	WorkingDir                string
	ChainID                   string
	DefaultDBPath             string
	DefaultEpochString        string
}

type openStorageUnits struct {
	generalConfig             config.Config
	marshalizer               marshal.Marshalizer
	bootstrapDataProvider     BootstrapDataProviderHandler
	latestStorageDataProvider storage.LatestStorageDataProviderHandler
	workingDir                string
	chainID                   string
	defaultDBPath             string
	defaultEpochString        string
}

// NewStorageUnitOpenHandler creates an openStorageUnits component
func NewStorageUnitOpenHandler(args ArgsNewOpenStorageUnits) (*openStorageUnits, error) {
	o := &openStorageUnits{
		generalConfig:             args.GeneralConfig,
		marshalizer:               args.Marshalizer,
		workingDir:                args.WorkingDir,
		chainID:                   args.ChainID,
		defaultDBPath:             args.DefaultDBPath,
		defaultEpochString:        args.DefaultEpochString,
		bootstrapDataProvider:     args.BootstrapDataProvider,
		latestStorageDataProvider: args.LatestStorageDataProvider,
	}

	return o, nil
}

// GetMostRecentBootstrapStorageUnit will open bootstrap storage unit
func (o *openStorageUnits) GetMostRecentBootstrapStorageUnit() (storage.Storer, error) {
	parentDir, lastEpoch, err := o.latestStorageDataProvider.GetParentDirAndLastEpoch()
	if err != nil {
		return nil, err
	}

	// TODO: refactor this - as it works with bootstrap storage unit only
	persisterFactory := NewPersisterFactory(o.generalConfig.Storages.BootstrapStorage.DB)
	pathWithoutShard := filepath.Join(
		parentDir,
		fmt.Sprintf("%s_%d", o.defaultEpochString, lastEpoch),
	)

	persisterPath := filepath.Join(
		pathWithoutShard,
		o.generalConfig.Storages.BootstrapStorage.DB.FilePath,
	)

	persister, err := createDB(persisterFactory, persisterPath)
	if err != nil {
		return nil, err
	}

	cacher, err := lrucache.NewCache(10)
	if err != nil {
		return nil, err
	}

	storer, err := storageUnit.NewStorageUnit(cacher, persister)
	if err != nil {
		return nil, err
	}

	return storer, nil
}

func createDB(persisterFactory *PersisterFactory, persisterPath string) (storage.Persister, error) {
	if persisterFactory.dbType == "" {
		return nil, storage.ErrNotSupportedDBType
	}
	var persister storage.Persister
	var err error
	for i := 0; i < core.MaxRetriesToCreateDB; i++ {
		persister, err = persisterFactory.Create(persisterPath)
		if err == nil {
			return persister, nil
		}
		log.Warn("Create Persister failed", "path", persisterPath, "error", err)
		//TODO: extract this in a parameter and inject it
		time.Sleep(core.SleepTimeBetweenCreateDBRetries)
	}
	return nil, err
}

// IsInterfaceNil returns true if there is no value under the interface
func (o *openStorageUnits) IsInterfaceNil() bool {
	return o == nil
}
