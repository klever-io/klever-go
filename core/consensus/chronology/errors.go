package chronology

import (
	"errors"
)

// ErrNilSlotManager is raised when a valid slotManager is expected but nil used
var ErrNilSlotManager = errors.New("slotManager is nil")

// ErrNilSlotHandler is raised when a valid slotHandler is expected but nil used
var ErrNilSlotHandler = errors.New("slotHandler is nil")

// ErrNilSyncTimer is raised when a valid sync timer is expected but nil used
var ErrNilSyncTimer = errors.New("sync timer is nil")

// ErrNilAppStatusHandler is raised when the AppStatusHandler is nil when setting it
var ErrNilAppStatusHandler = errors.New("nil AppStatusHandler")

// ErrNilWatchdog signals that a nil watchdog has been provided
var ErrNilWatchdog = errors.New("nil watchdog")

// ErrNilBlockchain signals that a nil blockchain structure has been provided
var ErrNilBlockchain = errors.New("nil blockchain")
