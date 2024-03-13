package factory

import (
	"github.com/klever-io/klever-go/statusHandler/presenter"
	"github.com/klever-io/klever-go/statusHandler/view"
)

type presenterFactory struct {
}

// NewPresenterFactory is responsible for creating a new presenter factory object
func NewPresenterFactory() *presenterFactory {
	presenterFactoryObject := presenterFactory{}

	return &presenterFactoryObject
}

// Create returns an presenter object that will hold presenter in the system
func (pf *presenterFactory) Create() view.Presenter {
	presenterStatusHandler := presenter.NewPresenterStatusHandler()

	return presenterStatusHandler
}
