package machine

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/shirou/gopsutil/v4/disk"
)

var diskLog = logger.GetOrCreate("statistics/machine/disk")

const dbSizeCacheIntervalSec = 300 // 5 minutes

// DiskStatistics can compute disk usage and database directory size
type DiskStatistics struct {
	diskTotal     uint64
	diskAvailable uint64
	diskPercent   uint64
	dbSize        uint64
	lastDbCheck   int64 // unix timestamp of last DB size computation
	workingDir    string
	dbPath        string
}

// NewDiskStatistics creates a DiskStatistics object
func NewDiskStatistics(workingDir string, dbPath string) *DiskStatistics {
	return &DiskStatistics{
		workingDir: workingDir,
		dbPath:     dbPath,
	}
}

// ComputeStatistics computes disk usage and DB size statistics.
// Disk stats are refreshed every call. DB size is cached for 5 minutes.
func (ds *DiskStatistics) ComputeStatistics() {
	ds.computeDiskUsage()
	ds.computeDbSizeIfNeeded()
}

func (ds *DiskStatistics) computeDiskUsage() {
	usage, err := disk.Usage(ds.workingDir)
	if err != nil {
		diskLog.Trace("failed to get disk usage", "error", err)
		return
	}

	atomic.StoreUint64(&ds.diskTotal, usage.Total)
	atomic.StoreUint64(&ds.diskAvailable, usage.Free)
	atomic.StoreUint64(&ds.diskPercent, uint64(usage.UsedPercent))
}

func (ds *DiskStatistics) computeDbSizeIfNeeded() {
	now := time.Now().Unix()
	lastCheck := atomic.LoadInt64(&ds.lastDbCheck)
	if now-lastCheck < dbSizeCacheIntervalSec {
		return
	}

	size := getDirSizeBytes(ds.dbPath)
	atomic.StoreUint64(&ds.dbSize, size)
	atomic.StoreInt64(&ds.lastDbCheck, now)
}

func getDirSizeBytes(dir string) uint64 {
	var size uint64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += uint64(info.Size()) // #nosec G115
		}
		return nil
	})
	return size
}

// DiskTotal returns total disk space in bytes
func (ds *DiskStatistics) DiskTotal() uint64 {
	return atomic.LoadUint64(&ds.diskTotal)
}

// DiskAvailable returns available disk space in bytes
func (ds *DiskStatistics) DiskAvailable() uint64 {
	return atomic.LoadUint64(&ds.diskAvailable)
}

// DiskPercent returns disk usage percent
func (ds *DiskStatistics) DiskPercent() uint64 {
	return atomic.LoadUint64(&ds.diskPercent)
}

// DbSize returns total database directory size in bytes
func (ds *DiskStatistics) DbSize() uint64 {
	return atomic.LoadUint64(&ds.dbSize)
}
