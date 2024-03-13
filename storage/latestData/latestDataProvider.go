package latestData

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("storage/latestData")
var _ storage.LatestStorageDataProviderHandler = (*latestDataProvider)(nil)

// ArgsLatestDataProvider holds the arguments needed for creating a latestDataProvider object
type ArgsLatestDataProvider struct {
	GeneralConfig         config.Config
	Marshalizer           marshal.Marshalizer
	Hasher                hashing.Hasher
	BootstrapDataProvider factory.BootstrapDataProviderHandler
	DirectoryReader       storage.DirectoryReaderHandler
	WorkingDir            string
	ChainID               string
	DefaultDBPath         string
	DefaultEpochString    string
}

type iteratedShardData struct {
	bootstrapData  *bootstrapStorage.BootstrapData
	epochStartSlot uint64
	successful     bool
}

type latestDataProvider struct {
	generalConfig         config.Config
	marshalizer           marshal.Marshalizer
	hasher                hashing.Hasher
	bootstrapDataProvider factory.BootstrapDataProviderHandler
	directoryReader       storage.DirectoryReaderHandler
	workingDir            string
	chainID               string
	defaultDBPath         string
	defaultEpochString    string
}

// NewLatestDataProvider returns a new instance of latestDataProvider
func NewLatestDataProvider(args ArgsLatestDataProvider) (*latestDataProvider, error) {
	return &latestDataProvider{
		generalConfig:         args.GeneralConfig,
		marshalizer:           args.Marshalizer,
		hasher:                args.Hasher,
		workingDir:            args.WorkingDir,
		chainID:               args.ChainID,
		directoryReader:       args.DirectoryReader,
		defaultEpochString:    args.DefaultEpochString,
		defaultDBPath:         args.DefaultDBPath,
		bootstrapDataProvider: args.BootstrapDataProvider,
	}, nil
}

// Get will return a struct containing the latest data in storage
func (ldp *latestDataProvider) Get() (storage.LatestDataFromStorage, error) {
	parentDir, lastEpoch, err := ldp.GetParentDirAndLastEpoch()
	if err != nil {
		return storage.LatestDataFromStorage{}, err
	}

	ldfs, err := ldp.getLastEpochAndSlotFromStorage(parentDir, lastEpoch)
	if err == storage.ErrBootstrapDataNotFoundInStorage && lastEpoch > 0 {
		// retry with epoch before
		return ldp.getLastEpochAndSlotFromStorage(parentDir, lastEpoch-1)
	}
	return ldfs, err
}

// GetParentDirAndLastEpoch returns the parent directory and last epoch
func (ldp *latestDataProvider) GetParentDirAndLastEpoch() (string, uint32, error) {
	parentDir := filepath.Join(
		ldp.workingDir,
		ldp.defaultDBPath,
		ldp.chainID)

	directoriesNames, err := ldp.directoryReader.ListDirectoriesAsString(parentDir)
	if err != nil {
		return "", 0, err
	}

	epochDirs := make([]string, 0, len(directoriesNames))
	for _, dirName := range directoriesNames {
		isEpochDir := strings.HasPrefix(dirName, ldp.defaultEpochString)
		if !isEpochDir {
			continue
		}

		epochDirs = append(epochDirs, dirName)
	}

	lastEpoch, err := ldp.GetLastEpochFromDirNames(epochDirs)
	if err != nil {
		return "", 0, err
	}

	return parentDir, lastEpoch, nil
}

func (ldp *latestDataProvider) getLastEpochAndSlotFromStorage(parentDir string, lastEpoch uint32) (storage.LatestDataFromStorage, error) {
	persisterFactory := factory.NewPersisterFactory(ldp.generalConfig.Storages.BootstrapStorage.DB)
	epochPath := filepath.Join(
		parentDir,
		fmt.Sprintf("%s_%d", ldp.defaultEpochString, lastEpoch),
	)

	var mostRecentBootstrapData *bootstrapStorage.BootstrapData
	highestSlot := int64(0)
	epochStartSlot := uint64(0)

	persisterPath := filepath.Join(
		epochPath,
		ldp.generalConfig.Storages.BootstrapStorage.DB.FilePath,
	)

	data := ldp.loadData(highestSlot, persisterFactory, persisterPath)
	if data.successful {
		epochStartSlot = data.epochStartSlot
		mostRecentBootstrapData = data.bootstrapData
	}

	if mostRecentBootstrapData == nil {
		return storage.LatestDataFromStorage{}, storage.ErrBootstrapDataNotFoundInStorage
	}

	lastestData := storage.LatestDataFromStorage{
		Epoch:          mostRecentBootstrapData.LastHeader.Epoch,
		LastSlot:       mostRecentBootstrapData.LastSlot,
		EpochStartSlot: epochStartSlot,
	}

	return lastestData, nil
}

func (ldp *latestDataProvider) loadData(currentHighestSlot int64, persisterFactory storage.PersisterFactory, persisterPath string) *iteratedShardData {
	bootstrapData, storer, errGet := ldp.bootstrapDataProvider.LoadForPath(persisterFactory, persisterPath)
	defer func() {
		if storer != nil {
			err := storer.Close()
			if err != nil {
				log.Debug("latestDataProvider: closing storer", "path", persisterPath, "error", err)
			}
		}
	}()
	if errGet != nil {
		return &iteratedShardData{}
	}

	if bootstrapData.LastSlot > currentHighestSlot {
		epochStartSlot, err := ldp.loadEpochStartSlot(bootstrapData.EpochStartTriggerConfigKey, storer)
		if err != nil {
			return &iteratedShardData{}
		}

		return &iteratedShardData{
			bootstrapData:  bootstrapData,
			epochStartSlot: epochStartSlot,
			successful:     true,
		}
	}

	return &iteratedShardData{}
}

// loadEpochStartSlot will return the epoch start slot from the bootstrap unit
func (ldp *latestDataProvider) loadEpochStartSlot(
	key []byte,
	storer storage.Storer,
) (uint64, error) {
	trigInternalKey := append([]byte(core.TriggerRegistryKeyPrefix), key...)
	data, err := storer.Get(trigInternalKey)
	if err != nil {
		return 0, err
	}

	state := &epochStart.TriggerRegistry{}
	err = json.Unmarshal(data, state)
	if err != nil {
		return 0, err
	}

	return state.CurrEpochStartSlot, nil

}

// GetLastEpochFromDirNames returns the last epoch found in storage directory
func (ldp *latestDataProvider) GetLastEpochFromDirNames(epochDirs []string) (uint32, error) {
	if len(epochDirs) == 0 {
		return 0, nil
	}

	re := regexp.MustCompile("[0-9]+")
	epochsInDirName := make([]uint32, 0, len(epochDirs))

	for _, dirname := range epochDirs {
		epochStr := re.FindString(dirname)
		epoch, err := strconv.ParseInt(epochStr, 10, 64)
		if err != nil {
			return 0, err
		}

		epochsInDirName = append(epochsInDirName, uint32(epoch))
	}

	sort.Slice(epochsInDirName, func(i, j int) bool {
		return epochsInDirName[i] > epochsInDirName[j]
	})

	return epochsInDirName[0], nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (ldp *latestDataProvider) IsInterfaceNil() bool {
	return ldp == nil
}
