package storage

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/atomic"
)

var log = logger.GetOrCreate("storage")

var cumulatedSizeInBytes atomic.Counter

// MonitorNewCache adds the size in the global cumulated size variable
func MonitorNewCache(tag string, sizeInBytes uint64) {
	cumulatedSizeInBytes.Add(int64(sizeInBytes))
	log.Debug("MonitorNewCache", "name", tag, "capacity", tools.ConvertBytes(sizeInBytes), "cumulated", tools.ConvertBytes(cumulatedSizeInBytes.GetUint64()))
}
