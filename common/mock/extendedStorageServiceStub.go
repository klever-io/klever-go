package mock

import (
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
)

// StorageListProviderStub -
type StorageListProviderStub struct {
	GetAllStorersCalled func() map[retriever.UnitType]storage.Storer
}

// GetAllStorers -
func (sis *StorageListProviderStub) GetAllStorers() map[retriever.UnitType]storage.Storer {
	if sis.GetAllStorersCalled != nil {
		return sis.GetAllStorersCalled()
	}

	return nil
}

// IsInterfaceNil -
func (slps *StorageListProviderStub) IsInterfaceNil() bool {
	return slps == nil
}
