package tools

import "time"

// FileLoggingHandler will handle log file rotation
type FileLoggingHandler interface {
	SetFileRotation(lifeSpanDuration time.Duration, checkSizeSpanDuration time.Duration, maxBackups int, maxFileSize int64) error
	ChangeFileLifeSpan(newDuration time.Duration) error
	Close() error
	IsInterfaceNil() bool
}
