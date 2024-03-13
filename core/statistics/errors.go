package statistics

import (
	"errors"
)

// ErrInvalidSlotInterval signals that an invalid slot duration was provided
var ErrInvalidSlotInterval = errors.New("invalid slot duration")

// ErrInvalidShardCount signals that an invalid number of shard is set
var ErrInvalidShardCount = errors.New("invalid shard count")

// ErrNilFileToWriteStats signals that the file where statistics should be written is nil
var ErrNilFileToWriteStats = errors.New("nil file to write statistics")

// ErrNilStatusHandler signals that a nil status handler has been provided
var ErrNilStatusHandler = errors.New("nil status handler")

// ErrNilInitialTPSBenchmarks signals that nil TPS benchmarks have been provided
var ErrNilInitialTPSBenchmarks = errors.New("nil initial TPS benchmarks")
