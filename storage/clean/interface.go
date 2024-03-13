package clean

import (
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/storage"
)

// StorageListProviderHandler defines the actions needed for returning all storers
type StorageListProviderHandler interface {
	GetAllStorers() map[retriever.UnitType]storage.Storer
	IsInterfaceNil() bool
}

// EpochStartNotifier defines what a component which will handle registration to epoch start event should do
type EpochStartNotifier interface {
	RegisterHandler(handler eventNotifier.ActionHandler)
	IsInterfaceNil() bool
}

// OldDataCleanerProvider defines what a component that handles the deletion or keeping of old data should do
type OldDataCleanerProvider interface {
	ShouldClean() bool
	IsInterfaceNil() bool
}
