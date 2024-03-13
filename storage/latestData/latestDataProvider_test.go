package latestData

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLatestDataProvider_ShouldWork(t *testing.T) {
	t.Parallel()

	ldp, err := NewLatestDataProvider(getLatestDataProviderArgs())
	require.False(t, check.IfNil(ldp))
	require.NoError(t, err)
}

func TestGetParentDirAndLastEpoch_ShouldWork(t *testing.T) {
	t.Parallel()

	workingDir := "testDir"
	defaultDbPath := "default"
	chainID := "chainID"
	lastEpoch := uint32(2)
	lastDirectories := []string{"WrongEpoch_10", "Epoch_1", fmt.Sprintf("Epoch_%d", lastEpoch)}

	args := getLatestDataProviderArgs()
	args.ChainID = chainID
	args.DefaultDBPath = defaultDbPath
	args.WorkingDir = workingDir
	args.DirectoryReader = &mock.DirectoryReaderStub{
		ListDirectoriesAsStringCalled: func(directoryPath string) ([]string, error) {
			return lastDirectories, nil
		},
	}

	ldp, _ := NewLatestDataProvider(args)

	parentDir, epoch, err := ldp.GetParentDirAndLastEpoch()
	assert.NoError(t, err)
	assert.Equal(t, workingDir+"/"+defaultDbPath+"/"+chainID, parentDir)
	assert.Equal(t, lastEpoch, epoch)
}

func TestLatestDataProvider_GetCannotGetListDirectoriesShouldErr(t *testing.T) {
	t.Parallel()

	localErr := errors.New("localErr")
	args := getLatestDataProviderArgs()
	args.DirectoryReader = &mock.DirectoryReaderStub{
		ListDirectoriesAsStringCalled: func(directoryPath string) ([]string, error) {
			return nil, localErr
		},
	}
	ldp, _ := NewLatestDataProvider(args)

	_, err := ldp.Get()
	assert.Equal(t, localErr, err)
}

func TestLatestDataProvider_Get(t *testing.T) {
	t.Parallel()

	defaultPath := "db"
	args := getLatestDataProviderArgs()
	args.DefaultDBPath = defaultPath
	lastEpoch := uint32(1)
	args.DirectoryReader = &mock.DirectoryReaderStub{
		ListDirectoriesAsStringCalled: func(directoryPath string) ([]string, error) {
			if directoryPath == defaultPath {
				return []string{fmt.Sprintf("Epoch_%d", lastEpoch)}, nil
			}
			return []string{}, nil
		},
	}

	startSlot, lastSlot := uint64(5), int64(10)
	state := &epochStart.TriggerRegistry{
		CurrEpochStartSlot: startSlot,
	}
	storer := &mock.StorerStub{
		GetCalled: func(key []byte) ([]byte, error) {
			stateBytes, _ := json.Marshal(state)
			return stateBytes, nil
		},
	}

	args.BootstrapDataProvider = &mock.BootStrapDataProviderStub{
		LoadForPathCalled: func(persisterFactory storage.PersisterFactory, path string) (*bootstrapStorage.BootstrapData, storage.Storer, error) {
			bootstrapData := &bootstrapStorage.BootstrapData{
				LastSlot:   lastSlot,
				LastHeader: &bootstrapStorage.BootstrapHeaderInfo{Epoch: lastEpoch},
			}

			return bootstrapData, storer, nil
		},
	}

	ldp, _ := NewLatestDataProvider(args)

	expectedRes := storage.LatestDataFromStorage{
		Epoch:          lastEpoch,
		LastSlot:       lastSlot,
		EpochStartSlot: startSlot,
	}
	result, err := ldp.Get()
	assert.NoError(t, err)
	assert.Equal(t, expectedRes, result)
}

func getLatestDataProviderArgs() ArgsLatestDataProvider {
	return ArgsLatestDataProvider{
		GeneralConfig:         config.Config{},
		Marshalizer:           &mock.MarshalizerMock{},
		Hasher:                &mock.HasherMock{},
		BootstrapDataProvider: &mock.BootStrapDataProviderStub{},
		DirectoryReader:       &mock.DirectoryReaderStub{},
		WorkingDir:            "",
		ChainID:               "",
		DefaultDBPath:         "db",
		DefaultEpochString:    "Epoch",
	}
}

func TestLoadEpochStartSlotMetachain(t *testing.T) {
	t.Parallel()

	key := []byte("123")
	startSlot := uint64(1000)
	state := &epochStart.TriggerRegistry{
		CurrEpochStartSlot: startSlot,
	}
	storer := &mock.StorerStub{
		GetCalled: func(key []byte) ([]byte, error) {
			stateBytes, _ := json.Marshal(state)
			return stateBytes, nil
		},
	}

	args := getLatestDataProviderArgs()
	ldp, _ := NewLatestDataProvider(args)

	slot, err := ldp.loadEpochStartSlot(key, storer)
	assert.NoError(t, err)
	assert.Equal(t, startSlot, slot)
}
