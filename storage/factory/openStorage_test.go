package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func createMockArgsOpenStorageUnits() ArgsNewOpenStorageUnits {
	return ArgsNewOpenStorageUnits{
		GeneralConfig:             config.Config{},
		Marshalizer:               &mock.MarshalizerMock{},
		BootstrapDataProvider:     &mock.BootStrapDataProviderStub{},
		LatestStorageDataProvider: &mock.LatestStorageDataProviderStub{},
		WorkingDir:                "",
		ChainID:                   "",
		DefaultDBPath:             "",
		DefaultEpochString:        "Epoch",
	}
}

func TestNewStorageUnitOpenHandler(t *testing.T) {
	t.Parallel()

	suoh, err := NewStorageUnitOpenHandler(createMockArgsOpenStorageUnits())

	assert.NoError(t, err)
	assert.False(t, check.IfNil(suoh))
}

func TestGetMostRecentBootstrapStorageUnit_CannotCreatePersister(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}

	t.Parallel()

	args := createMockArgsOpenStorageUnits()
	args.BootstrapDataProvider = &mock.BootStrapDataProviderStub{
		LoadForPathCalled: func(persisterFactory storage.PersisterFactory, path string) (*bootstrapStorage.BootstrapData, storage.Storer, error) {
			return &bootstrapStorage.BootstrapData{
				LastSlot: 100,
			}, nil, nil
		},
	}
	suoh, _ := NewStorageUnitOpenHandler(args)

	storer, err := suoh.GetMostRecentBootstrapStorageUnit()
	assert.Nil(t, storer)
	assert.Equal(t, storage.ErrNotSupportedDBType, err)
}

func TestGetMostRecentBootstrapStorageUnit(t *testing.T) {
	t.Parallel()

	args := createMockArgsOpenStorageUnits()
	args.GeneralConfig = config.Config{
		Storages: config.StoragesConfig{
			BootstrapStorage: config.StorageConfig{
				DB: config.DBConfig{Type: "MemoryDB"},
			},
		},
	}

	args.BootstrapDataProvider = &mock.BootStrapDataProviderStub{
		LoadForPathCalled: func(persisterFactory storage.PersisterFactory, path string) (*bootstrapStorage.BootstrapData, storage.Storer, error) {
			return &bootstrapStorage.BootstrapData{
				LastSlot: 100,
			}, nil, nil
		},
	}
	suoh, _ := NewStorageUnitOpenHandler(args)

	storer, err := suoh.GetMostRecentBootstrapStorageUnit()
	assert.NoError(t, err)
	assert.NotNil(t, storer)

}
