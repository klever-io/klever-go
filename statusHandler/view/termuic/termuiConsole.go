package termuic

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	ui "github.com/gizak/termui/v3"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/statusHandler/view"
	"github.com/klever-io/klever-go/statusHandler/view/termuic/termuiRenders"
	"github.com/klever-io/klever-go/tools/check"
)

// numOfTicksBeforeRedrawing represents the number of ticks which have to pass until a fake resize will be made
// in order to clean the unwanted appeared characters
const numOfTicksBeforeRedrawing = 10

var log = logger.GetOrCreate("statushandler/view/termuic")

// ConnectorConsole data where is store data from handler
type ConnectorConsole struct {
	presenter                 view.Presenter
	consoleRender             ConnectorRender
	grid                      *termuiRenders.DrawableContainer
	mutRefresh                *sync.RWMutex
	chanNodeIsStarting        chan struct{}
	refreshTimeInMilliseconds int
}

// NewConnectorConsole method is used to return a new ConnectorConsole structure
func NewConnectorConsole(presenter view.Presenter, refreshTimeInMilliseconds int, chanNodeIsStarting chan struct{}) (*ConnectorConsole, error) {
	if check.IfNil(presenter) {
		return nil, statusHandler.ErrNilPresenterInterface
	}
	if refreshTimeInMilliseconds < 1 {
		return nil, statusHandler.ErrInvalidRefreshTimeInMilliseconds
	}
	if chanNodeIsStarting == nil {
		return nil, statusHandler.ErrNilChanNodeIsStarting
	}

	tc := ConnectorConsole{
		presenter:                 presenter,
		mutRefresh:                &sync.RWMutex{},
		refreshTimeInMilliseconds: refreshTimeInMilliseconds,
		chanNodeIsStarting:        chanNodeIsStarting,
	}

	return &tc, nil
}

var closeOnce sync.Once

func safeClose() {
	closeOnce.Do(func() {
		ui.Close()
	})
}

// Start method - will start connector console
func (tc *ConnectorConsole) Start(chanStart chan struct{}) error {
	go func() {
		defer func() {
			log.Debug("closing connector ui")
			safeClose()
		}()

		<-chanStart
		if err := ui.Init(); err != nil {
			log.Debug("cannot initialize ui", "error", err.Error())
			stopApplication()
			return
		}

		tc.eventLoop()
	}()

	return nil
}

func (tc *ConnectorConsole) eventLoop() {
	tc.grid = termuiRenders.NewDrawableContainer()
	if tc.grid == nil {
		log.Debug("cannot render connector console", "error", statusHandler.ErrNilGrid.Error())
		return
	}

	var err error
	tc.consoleRender, err = termuiRenders.NewWidgetsRender(tc.presenter, tc.grid)
	if err != nil {
		log.Debug("nil console render", "error", err.Error())
		return
	}

	termWidth, termHeight := ui.TerminalDimensions()
	tc.grid.SetRectangle(0, 0, termWidth, termHeight)

	uiEvents := ui.PollEvents()
	// handles kill signal sent to gotop
	sigTerm := make(chan os.Signal, 2)
	signal.Notify(sigTerm, os.Interrupt, syscall.SIGTERM)

	tc.consoleRender.RefreshData(tc.refreshTimeInMilliseconds)
	ticksCounter := uint32(0)

	for {
		select {
		case <-time.After(time.Millisecond * time.Duration(tc.refreshTimeInMilliseconds)):
			tc.doChanges(&ticksCounter, tc.refreshTimeInMilliseconds)
		case s := <-sigTerm:
			log.Info("received signal, terminating application", "signal", s.String())
			ui.Clear()
			return
		case e := <-uiEvents:
			tc.processUiEvents(e, tc.refreshTimeInMilliseconds)
		case <-tc.chanNodeIsStarting:
			tc.presenter.InvalidateCache()
		}
	}
}

func (tc *ConnectorConsole) processUiEvents(e ui.Event, numMillisecondsRefreshTime int) {
	switch e.ID {
	case "<Resize>":
		tc.doResizeEvent(e, numMillisecondsRefreshTime)
	case "<C-c>":
		safeClose()
		stopApplication()
		return
	}
}

func (tc *ConnectorConsole) doChanges(counter *uint32, numMillisecondsRefreshTime int) {
	atomic.AddUint32(counter, 1)
	if atomic.LoadUint32(counter) > numOfTicksBeforeRedrawing {
		width, height := ui.TerminalDimensions()
		tc.doResize(width, height, numMillisecondsRefreshTime)
		atomic.StoreUint32(counter, 0)
	} else {
		tc.refreshWindow(numMillisecondsRefreshTime)
	}
}

func (tc *ConnectorConsole) doResizeEvent(e ui.Event, numMillisecondsRefreshTime int) {
	payload := e.Payload.(ui.Resize)
	tc.doResize(payload.Width, payload.Height, numMillisecondsRefreshTime)
}

func (tc *ConnectorConsole) doResize(width int, height int, numMillisecondsRefreshTime int) {
	tc.grid.SetRectangle(0, 0, width, height)
	tc.refreshWindow(numMillisecondsRefreshTime)
}

func (tc *ConnectorConsole) refreshWindow(numMillisecondsRefreshTime int) {
	tc.mutRefresh.Lock()
	defer tc.mutRefresh.Unlock()

	tc.consoleRender.RefreshData(numMillisecondsRefreshTime)
	ui.Clear()
	ui.Render(tc.grid.TopLeft(), tc.grid.TopRight(), tc.grid.Bottom())
}
