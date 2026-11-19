package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

func newStorageServiceFactoryForPruning(t *testing.T, cleanOldEpochsData bool) *StorageServiceFactory {
	t.Helper()

	cfg := &config.Config{
		StoragePruning: config.StoragePruningConfig{
			Enabled:             true,
			CleanOldEpochsData:  cleanOldEpochsData,
			NumEpochsToKeep:     minimumNumberOfEpochsToKeep,
			NumActivePersisters: minimumNumberOfActivePersisters,
		},
	}

	psf, err := NewStorageServiceFactory(cfg, &mock.PathManagerStub{}, &mock.EpochStartNotifierStub{}, 0)
	require.NoError(t, err)

	return psf
}

// The pruning storers and the old-database cleaner read CleanOldEpochsData
// when they are built, so the config handed to the factory is what decides
// whether old epoch databases are ever deleted.
func TestStorageServiceFactory_CleanOldEpochsDataOffBuildsStorersThatKeepOldEpochs(t *testing.T) {
	t.Parallel()

	psf := newStorageServiceFactoryForPruning(t, false)

	args := psf.createPruningStorerArgs(config.StorageConfig{})
	require.False(t, args.CleanOldEpochsData)
	require.False(t, psf.oldDataCleanerProvider.ShouldClean())

	// With cleaning off no old-database cleaner is created, so a nil store is never touched.
	require.NoError(t, psf.initOldDatabasesCleaningIfNeeded(nil))
}

func TestStorageServiceFactory_CleanOldEpochsDataOnBuildsStorersThatClean(t *testing.T) {
	t.Parallel()

	psf := newStorageServiceFactoryForPruning(t, true)

	args := psf.createPruningStorerArgs(config.StorageConfig{})
	require.True(t, args.CleanOldEpochsData)
	require.True(t, psf.oldDataCleanerProvider.ShouldClean())
}
