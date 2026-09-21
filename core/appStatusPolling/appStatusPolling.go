package appStatusPolling

import (
	"sync"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"
)

const minPollingDuration = time.Second

// closeJoinTimeout bounds how long Close waits for an in-flight handler batch.
// Handlers normally complete in well under a millisecond, so this is ~1000x
// headroom; it exists only so that a handler stuck on a syscall cannot consume
// the node's whole shutdown budget. See the note on Close.
const closeJoinTimeout = time.Second

// AppStatusPolling will update an AppStatusHandler by polling components at a predefined interval
type AppStatusPolling struct {
	pollingDuration     time.Duration
	mutRegisteredFunc   sync.RWMutex
	registeredFunctions []func(appStatusHandler core.AppStatusHandler)
	appStatusHandler    core.AppStatusHandler
	done                chan struct{}
	wg                  sync.WaitGroup
	mutClose            sync.Mutex
	closed              bool
	joined              chan struct{}
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
//
// Poll after Close is a no-op. Registering the goroutine under mutClose is what
// makes that safe: it keeps wg.Go's Add from landing while Close's waiter is
// inside wg.Wait, which sync detects and answers with a process-killing
// "WaitGroup misuse: Add called concurrently with Wait" panic.
func (asp *AppStatusPolling) Poll() {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	if asp.closed {
		return
	}

	asp.wg.Go(func() {
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
	})
}

// Close stops the polling goroutine and waits for it to exit, so that a handler
// caught mid-tick cannot still be reading from components that the caller tears
// down after Close returns. cmd/node/startup.go:closeAllComponents relies on
// this: it drains the background closers before closing NetMessenger (still
// read by in-flight handlers) and then the Store and trie persisters.
//
// The join is bounded by closeJoinTimeout rather than unbounded, because
// closeAllComponents drains these closers *before* Store.CloseAll and the trie
// persisters, under a single maxTimeToClose watchdog (cmd/node/main.go). An
// unbounded wait on a stuck handler would let that watchdog fire and skip the
// storage close entirely — a worse outcome than the shutdown race this join
// exists to prevent. Not every handler is a cheap in-memory read:
// registerMemStatistics calls runtime.ReadMemStats plus several gopsutil /proc
// lookups, any of which can stall on an IO-loaded node.
//
// On timeout Close returns ErrCloseTimeout and gives up rather than blocking;
// closeAllComponents logs it via LogIfError and proceeds to close storage. The
// first Close starts one waiter; later calls share it. The waiter exits once
// the handler returns.
//
// Idempotent. A call from inside a registered polling handler returns
// ErrCloseTimeout after closeJoinTimeout; the handler then finishes and the
// waiter completes.
func (asp *AppStatusPolling) Close() error {
	asp.mutClose.Lock()
	if !asp.closed {
		asp.closed = true
		close(asp.done)
		asp.joined = make(chan struct{})
		go func(joined chan struct{}) {
			asp.wg.Wait()
			close(joined)
		}(asp.joined)
	}
	joined := asp.joined
	asp.mutClose.Unlock()

	timer := time.NewTimer(closeJoinTimeout)
	defer timer.Stop()

	select {
	case <-joined:
		return nil
	case <-timer.C:
		return ErrCloseTimeout
	}
}
