package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type bootstrapDataProvider struct {
	marshalizer marshal.Marshalizer
}

// NewBootstrapDataProvider returns a new instance of bootstrapDataProvider
func NewBootstrapDataProvider(marshalizer marshal.Marshalizer) (*bootstrapDataProvider, error) {
	if check.IfNil(marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	return &bootstrapDataProvider{
		marshalizer: marshalizer,
	}, nil
}

// LoadForPath returns the bootstrap data and the storer for the given persister factory and path
func (bdp *bootstrapDataProvider) LoadForPath(
	persisterFactory storage.PersisterFactory,
	path string,
) (*bootstrapStorage.BootstrapData, storage.Storer, error) {
	persister, err := persisterFactory.Create(path)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			errClose := persister.Close()
			if errClose != nil {
				log.Debug("encountered a non-critical error closing bootstrap persister",
					"error", errClose,
				)
			}
		}
	}()

	cacher, err := lrucache.NewCache(10)
	if err != nil {
		return nil, nil, err
	}

	storer, err := storageUnit.NewStorageUnit(cacher, persister)
	if err != nil {
		return nil, nil, err
	}

	bootStorer, err := bdp.GetStorer(storer)
	if err != nil {
		return nil, nil, err
	}

	highestSlot := bootStorer.GetHighestSlot()
	bootstrapData, err := bootStorer.Get(highestSlot)
	if err != nil {
		return nil, nil, err
	}

	return bootstrapData, storer, nil
}

// GetStorer returns the bootstorer
func (bdp *bootstrapDataProvider) GetStorer(storer storage.Storer) (process.BootStorer, error) {
	bootStorer, err := bootstrapStorage.NewBootstrapStorer(bdp.marshalizer, storer)
	if err != nil {
		return nil, err
	}

	return bootStorer, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (bdp *bootstrapDataProvider) IsInterfaceNil() bool {
	return bdp == nil
}
