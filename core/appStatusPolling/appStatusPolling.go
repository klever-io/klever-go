package appStatusPolling

import (
	"sync"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"
)

const minPollingDuration = time.Second

// defaultCloseJoinTimeout bounds Close's wait for an in-flight handler batch, so
// that one stuck handler cannot consume the node's whole shutdown budget.
const defaultCloseJoinTimeout = time.Second

// AppStatusPolling will update an AppStatusHandler by polling components at a predefined interval
type AppStatusPolling struct {
	pollingDuration     time.Duration
	mutRegisteredFunc   sync.RWMutex
	registeredFunctions []func(appStatusHandler core.AppStatusHandler)
	appStatusHandler    core.AppStatusHandler
	done                chan struct{}
	mutClose            sync.Mutex
	closed              bool
	stopped             chan struct{}
	closeJoinTimeout    time.Duration
}

// NewAppStatusPolling will return an instance of AppStatusPolling
func NewAppStatusPolling(appStatusHandler core.AppStatusHandler, pollingDuration time.Duration) (*AppStatusPolling, error) {
	if check.IfNil(appStatusHandler) {
		return nil, ErrNilAppStatusHandler
	}
	if pollingDuration < minPollingDuration {
		return nil, ErrPollingDurationToSmall
	}
	return &AppStatusPolling{
		pollingDuration:  pollingDuration,
		appStatusHandler: appStatusHandler,
		done:             make(chan struct{}),
		closeJoinTimeout: defaultCloseJoinTimeout,
	}, nil
}

// RegisterPollingFunc will register a new handler function
func (asp *AppStatusPolling) RegisterPollingFunc(handler func(appStatusHandler core.AppStatusHandler)) error {
	if handler == nil {
		return ErrNilHandlerFunc
	}
	asp.mutRegisteredFunc.Lock()
	asp.registeredFunctions = append(asp.registeredFunctions, handler)
	asp.mutRegisteredFunc.Unlock()
	return nil
}

// Poll starts the polling goroutine, which runs until Close. It is a no-op
// after Close, and a no-op if the goroutine is already running.
func (asp *AppStatusPolling) Poll() {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	if asp.closed || asp.stopped != nil {
		return
	}

	asp.stopped = make(chan struct{})
	go func(stopped chan struct{}) {
		defer close(stopped)

		ticker := time.NewTicker(asp.pollingDuration)
		defer ticker.Stop()

		for {
			select {
			case <-asp.done:
				return
			case <-ticker.C:
				// A batch slower than pollingDuration leaves a tick queued, so the
				// select above sees two ready cases and picks at random. Re-check
				// done here, or Close can be followed by one more batch.
				select {
				case <-asp.done:
					return
				default:
				}

				asp.mutRegisteredFunc.RLock()
				for _, handler := range asp.registeredFunctions {
					handler(asp.appStatusHandler)
				}
				asp.mutRegisteredFunc.RUnlock()
			}
		}
	}(asp.stopped)
}

// Close stops the polling goroutine and waits for the in-flight handler batch,
// so a handler cannot still be reading a component that closeAllComponents
// (cmd/node/startup.go) tears down after this returns. The wait is bounded by
// closeJoinTimeout because that drain shares one watchdog with the storage
// close; on timeout Close returns ErrCloseTimeout and gives up rather than let
// the watchdog skip storage. Idempotent, and safe to call from a handler.
func (asp *AppStatusPolling) Close() error {
	asp.mutClose.Lock()
	if !asp.closed {
		asp.closed = true
		close(asp.done)
	}
	stopped := asp.stopped
	joinTimeout := asp.closeJoinTimeout
	asp.mutClose.Unlock()

	if stopped == nil {
		return nil
	}

	timer := time.NewTimer(joinTimeout)
	defer timer.Stop()

	select {
	case <-stopped:
		return nil
	case <-timer.C:
		return ErrCloseTimeout
	}
}
