package mock

import "github.com/klever-io/klever-go/storage"

// LatestStorageDataProviderStub -
type LatestStorageDataProviderStub struct {
	GetParentDirAndLastEpochCalled func() (string, uint32, error)
	GetCalled                      func() (storage.LatestDataFromStorage, error)
	GetFromDirectoryCalled         func(path string) ([]string, error)
}

// GetParentDirAndLastEpoch -
func (lsdps *LatestStorageDataProviderStub) GetParentDirAndLastEpoch() (string, uint32, error) {
	if lsdps.GetParentDirAndLastEpochCalled != nil {
		return lsdps.GetParentDirAndLastEpochCalled()
	}

	return "", 0, nil
}

// Get -
func (lsdps *LatestStorageDataProviderStub) Get() (storage.LatestDataFromStorage, error) {
	if lsdps.GetCalled != nil {
		return lsdps.GetCalled()
	}

	return storage.LatestDataFromStorage{}, nil
}

// GetShardsFromDirectory -
func (lsdps *LatestStorageDataProviderStub) GetFromDirectory(path string) ([]string, error) {
	if lsdps.GetFromDirectoryCalled != nil {
		return lsdps.GetFromDirectoryCalled(path)
	}

	return nil, nil
}

// IsInterfaceNil --
func (lsdps *LatestStorageDataProviderStub) IsInterfaceNil() bool {
	return lsdps == nil
}
