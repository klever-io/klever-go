package ntp

import (
	"time"
)

// SyncTimer defines an interface for time synchronization
type SyncTimer interface {
	Close() error
	StartSyncingTime()
	ClockOffset() time.Duration
	LastSyncTimestamp() time.Time
	ClockSnapshot() (offset time.Duration, lastSync time.Time)
	FormattedCurrentTime() string
	CurrentTime() time.Time
	IsInterfaceNil() bool
}
