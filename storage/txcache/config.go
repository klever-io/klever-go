package txcache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/storage"
)

const numChunksLowerBound = 1
const numChunksUpperBound = 128
const maxNumItemsLowerBound = 4
const maxNumBytesLowerBound = maxNumItemsLowerBound * 1
const maxNumBytesUpperBound = 1_073_741_824 // one GB
const maxNumItemsPerSenderLowerBound = 1
const maxNumBytesPerSenderLowerBound = maxNumItemsPerSenderLowerBound * 1
const maxNumBytesPerSenderUpperBound = 33_554_432 // 32 MB
const numSendersToPreemptivelyEvictLowerBound = 1

// Config holds cache configuration
type Config struct {
	Name                          string
	NumChunks                     uint32
	EvictionEnabled               bool
	NumBytesThreshold             uint32
	NumBytesPerSenderThreshold    uint32
	CountThreshold                uint32
	CountPerSenderThreshold       uint32
	NumSendersToPreemptivelyEvict uint32
	CleanupInterval               time.Duration // Interval for periodic cleanup. 0 uses default (30s), negative disables cleanup.
}

type senderConstraints struct {
	maxNumTxs   uint32
	maxNumBytes uint32
}

func (config *Config) verify() error {
	if len(config.Name) == 0 {
		return fmt.Errorf("%w: config.Name is invalid", storage.ErrInvalidConfig)
	}
	if config.NumChunks < numChunksLowerBound || config.NumChunks > numChunksUpperBound {
		return fmt.Errorf("%w: config.NumChunks is invalid", storage.ErrInvalidConfig)
	}
	if config.NumBytesPerSenderThreshold < maxNumBytesPerSenderLowerBound || config.NumBytesPerSenderThreshold > maxNumBytesPerSenderUpperBound {
		return fmt.Errorf("%w: config.NumBytesPerSenderThreshold is invalid", storage.ErrInvalidConfig)
	}
	if config.CountPerSenderThreshold < maxNumItemsPerSenderLowerBound {
		return fmt.Errorf("%w: config.CountPerSenderThreshold is invalid", storage.ErrInvalidConfig)
	}
	if config.EvictionEnabled {
		if config.NumBytesThreshold < maxNumBytesLowerBound || config.NumBytesThreshold > maxNumBytesUpperBound {
			return fmt.Errorf("%w: config.NumBytesThreshold is invalid", storage.ErrInvalidConfig)
		}
		if config.CountThreshold < maxNumItemsLowerBound {
			return fmt.Errorf("%w: config.CountThreshold is invalid", storage.ErrInvalidConfig)
		}
		if config.NumSendersToPreemptivelyEvict < numSendersToPreemptivelyEvictLowerBound {
			return fmt.Errorf("%w: config.NumSendersToPreemptivelyEvict is invalid", storage.ErrInvalidConfig)
		}
	}

	return nil
}

func (config *Config) getSenderConstraints() senderConstraints {
	return senderConstraints{
		maxNumBytes: config.NumBytesPerSenderThreshold,
		maxNumTxs:   config.CountPerSenderThreshold,
	}
}

// String returns a readable representation of the object
func (config *Config) String() string {
	bytes, err := json.Marshal(config)
	if err != nil {
		log.Error("Config.String()", "err", err)
	}

	return string(bytes)
}
