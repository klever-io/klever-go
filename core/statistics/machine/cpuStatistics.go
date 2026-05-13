package machine

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

var durationSecond = time.Second
var errCpuCount = errors.New("cpu count is zero")

// CpuStatistics can compute the cpu usage percent
type CpuStatistics struct {
	numCpu                int
	cpuUsagePercent       uint64
	systemCpuUsagePercent uint64
}

// NewCpuStatistics will create a CpuStatistics object
func NewCpuStatistics() (*CpuStatistics, error) {
	numCpu, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}
	if numCpu == 0 {
		return nil, errCpuCount
	}

	return &CpuStatistics{
		cpuUsagePercent: 0,
		numCpu:          numCpu,
	}, nil
}

// ComputeStatistics computes the current cpu usage. It should be called on a go routine as it is a blocking
// call for a bounded time (1 second)
func (cs *CpuStatistics) ComputeStatistics() {
	currentProcess, err := GetCurrentProcess()
	if err != nil {
		return
	}

	percent, err := currentProcess.Percent(durationSecond)
	if err != nil {
		return
	}

	atomic.StoreUint64(&cs.cpuUsagePercent, uint64(percent/float64(cs.numCpu)))

	percents, err := cpu.Percent(0, false)
	if err != nil || len(percents) == 0 {
		return
	}
	atomic.StoreUint64(&cs.systemCpuUsagePercent, uint64(percents[0]))
}

// CpuPercentUsage will return the cpu percent usage. Concurrent safe.
func (cs *CpuStatistics) CpuPercentUsage() uint64 {
	return atomic.LoadUint64(&cs.cpuUsagePercent)
}

// SystemCpuPercentUsage will return the total system cpu percent usage
func (cs *CpuStatistics) SystemCpuPercentUsage() uint64 {
	return atomic.LoadUint64(&cs.systemCpuUsagePercent)
}
