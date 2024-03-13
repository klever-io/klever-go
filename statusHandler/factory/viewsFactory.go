package factory

import (
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/statusHandler/view"
	"github.com/klever-io/klever-go/statusHandler/view/termuic"
	"github.com/klever-io/klever-go/tools/check"
)

type viewsFactory struct {
	presenter                 view.Presenter
	refreshTimeInMilliseconds int
}

// NewViewsFactory is responsible for creating a new viewers factory object
func NewViewsFactory(presenter view.Presenter, refreshTimeInMilliseconds int) (*viewsFactory, error) {
	if check.IfNil(presenter) {
		return nil, statusHandler.ErrNilPresenterInterface
	}

	return &viewsFactory{
		presenter:                 presenter,
		refreshTimeInMilliseconds: refreshTimeInMilliseconds,
	}, nil
}

// Create returns an view slice that will hold all views in the system
func (vf *viewsFactory) Create() ([]Viewer, error) {
	views := make([]Viewer, 0)

	connectorConsole, err := vf.createConnectorConsole()
	if err != nil {
		return nil, err
	}
	views = append(views, connectorConsole)

	return views, nil
}

func (vf *viewsFactory) createConnectorConsole() (*termuic.ConnectorConsole, error) {
	chanNodeIsStarting := make(chan struct{})
	connectorConsole, err := termuic.NewConnectorConsole(vf.presenter, vf.refreshTimeInMilliseconds, chanNodeIsStarting)
	if err != nil {
		return nil, err
	}

	return connectorConsole, nil
}
