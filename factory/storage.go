package factory

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/storage"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/latestData"
	"github.com/klever-io/klever-go/tools/marshal"
)

// CreateLatestStorageDataProvider will create a latest storage data provider handler
func CreateLatestStorageDataProvider(
	bootstrapDataProvider storageFactory.BootstrapDataProviderHandler,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	generalConfig config.Config,
	chainID string,
	workingDir string,
	defaultDBPath string,
	defaultEpochString string,
) (storage.LatestStorageDataProviderHandler, error) {
	directoryReader := storageFactory.NewDirectoryReader()

	latestStorageDataArgs := latestData.ArgsLatestDataProvider{
		GeneralConfig:         generalConfig,
		Marshalizer:           marshalizer,
		Hasher:                hasher,
		BootstrapDataProvider: bootstrapDataProvider,
		DirectoryReader:       directoryReader,
		WorkingDir:            workingDir,
		ChainID:               chainID,
		DefaultDBPath:         defaultDBPath,
		DefaultEpochString:    defaultEpochString,
	}
	return latestData.NewLatestDataProvider(latestStorageDataArgs)
}

// CreateUnitOpener will create a new unit opener handler
func CreateUnitOpener(
	bootstrapDataProvider storageFactory.BootstrapDataProviderHandler,
	latestDataFromStorageProvider storage.LatestStorageDataProviderHandler,
	internalMarshalizer marshal.Marshalizer,
	generalConfig config.Config,
	chainID string,
	workingDir string,
	defaultDBPath string,
	defaultEpochString string,
) (storage.UnitOpenerHandler, error) {
	argsStorageUnitOpener := storageFactory.ArgsNewOpenStorageUnits{
		GeneralConfig:             generalConfig,
		Marshalizer:               internalMarshalizer,
		BootstrapDataProvider:     bootstrapDataProvider,
		LatestStorageDataProvider: latestDataFromStorageProvider,
		WorkingDir:                workingDir,
		ChainID:                   chainID,
		DefaultDBPath:             defaultDBPath,
		DefaultEpochString:        defaultEpochString,
	}

	return storageFactory.NewStorageUnitOpenHandler(argsStorageUnitOpener)
}
