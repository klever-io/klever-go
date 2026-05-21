package appStatusPolling

import (
	"sync"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"
)

const minPollingDuration = time.Second

// AppStatusPolling will update an AppStatusHandler by polling components at a predefined interval
type AppStatusPolling struct {
	pollingDuration     time.Duration
	mutRegisteredFunc   sync.RWMutex
	registeredFunctions []func(appStatusHandler core.AppStatusHandler)
	appStatusHandler    core.AppStatusHandler
	done                chan struct{}
	closeOnce           sync.Once
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

// Poll will notify the AppStatusHandler at a given time. The goroutine runs
// until Close is called.
func (asp *AppStatusPolling) Poll() {
	go func() {
		ticker := time.NewTicker(asp.pollingDuration)
		defer ticker.Stop()

		for {
			select {
			case <-asp.done:
				return
			case <-ticker.C:
				asp.mutRegisteredFunc.RLock()
				for _, handler := range asp.registeredFunctions {
					handler(asp.appStatusHandler)
				}
				asp.mutRegisteredFunc.RUnlock()
			}
		}
	}()
}

// Close stops the polling goroutine. Idempotent; always returns nil.
func (asp *AppStatusPolling) Close() error {
	asp.closeOnce.Do(func() {
		close(asp.done)
	})
	return nil
}
