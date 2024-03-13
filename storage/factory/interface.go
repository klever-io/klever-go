package factory

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/storage"
)

// BootstrapDataProviderHandler defines which actions should be done for loading bootstrap data from the boot storer
type BootstrapDataProviderHandler interface {
	LoadForPath(persisterFactory storage.PersisterFactory, path string) (*bootstrapStorage.BootstrapData, storage.Storer, error)
	GetStorer(storer storage.Storer) (process.BootStorer, error)
	IsInterfaceNil() bool
}
