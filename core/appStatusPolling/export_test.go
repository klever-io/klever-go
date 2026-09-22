package appStatusPolling

import "time"

// SetCloseJoinTimeout overrides the join bound on this instance. Call it before
// Poll or Close so no concurrent reader is running.
func (asp *AppStatusPolling) SetCloseJoinTimeout(timeout time.Duration) {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	asp.closeJoinTimeout = timeout
}

// PollStarted reports whether Poll started the polling goroutine.
func (asp *AppStatusPolling) PollStarted() bool {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	return asp.stopped != nil
}

// CloseRequested reports whether Close already signalled the polling goroutine.
func (asp *AppStatusPolling) CloseRequested() bool {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	return asp.closed
}

// StoppedChan returns the channel the polling goroutine closes on exit, or nil
// if Poll never started one.
func (asp *AppStatusPolling) StoppedChan() <-chan struct{} {
	asp.mutClose.Lock()
	defer asp.mutClose.Unlock()

	return asp.stopped
}
