package statistics

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/statistics/machine"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

// ResourceMonitor outputs statistics about resources used by the binary
type ResourceMonitor struct {
	startTime time.Time
}

// NewResourceMonitor creates a new ResourceMonitor instance
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		startTime: time.Now(),
	}
}

// GenerateStatistics creates a new statistic string
func (rm *ResourceMonitor) GenerateStatistics(generalConfig *config.Config, pathManager storage.PathManagerHandler, chainName string) []interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	fileDescriptors := int32(0)
	numOpenFiles := 0
	numConns := 0
	proc, err := machine.GetCurrentProcess()
	if err == nil {
		fileDescriptors, _ = proc.NumFDs()
		var openFiles []process.OpenFilesStat
		openFiles, err = proc.OpenFiles()
		if err == nil {
			numOpenFiles = len(openFiles)
		}

		var conns []net.ConnectionStat
		conns, err = proc.Connections()
		if err == nil {
			numConns = len(conns)
		}
	}

	trieStoragePath, mainDb := path.Split(pathManager.PathForStatic(generalConfig.AccountsTrieStorage.DB.FilePath))

	trieDbFilePath := filepath.Join(trieStoragePath, mainDb)
	evictionWaitingListDbFilePath := filepath.Join(trieStoragePath, generalConfig.EvictionWaitingList.DB.FilePath)
	snapshotsDbFilePath := filepath.Join(trieStoragePath, generalConfig.TrieSnapshotDB.FilePath)

	peerTrieStoragePath, mainDb := path.Split(pathManager.PathForStatic(generalConfig.PeerAccountsTrieStorage.DB.FilePath))

	peerTrieDbFilePath := filepath.Join(peerTrieStoragePath, mainDb)
	peerTrieEvictionWaitingListDbFilePath := filepath.Join(peerTrieStoragePath, generalConfig.EvictionWaitingList.DB.FilePath)
	peerTrieSnapshotsDbFilePath := filepath.Join(peerTrieStoragePath, generalConfig.TrieSnapshotDB.FilePath)

	kappTrieStoragePath, mainDb := path.Split(pathManager.PathForStatic(generalConfig.KAppAccountsTrieStorage.DB.FilePath))

	kappTrieDbFilePath := filepath.Join(kappTrieStoragePath, mainDb)
	kappTrieSnapshotsDbFilePath := filepath.Join(kappTrieStoragePath, generalConfig.TrieSnapshotDB.FilePath)

	return []interface{}{
		"timestamp", time.Now().Unix(),
		"uptime", time.Duration(time.Now().UnixNano() - rm.startTime.UnixNano()).Round(time.Second),
		"num go", runtime.NumGoroutine(),
		"alloc", tools.ConvertBytes(memStats.Alloc),
		"heap alloc", tools.ConvertBytes(memStats.HeapAlloc),
		"heap idle", tools.ConvertBytes(memStats.HeapIdle),
		"heap inuse", tools.ConvertBytes(memStats.HeapInuse),
		"heap sys", tools.ConvertBytes(memStats.HeapSys),
		"heap num objs", memStats.HeapObjects,
		"sys mem", tools.ConvertBytes(memStats.Sys),
		"num GC", memStats.NumGC,
		"FDs", fileDescriptors,
		"num opened files", numOpenFiles,
		"num conns", numConns,
		"accountsTrieDbMem", getDirMemSize(trieDbFilePath),
		"evictionDbMem", getDirMemSize(evictionWaitingListDbFilePath),
		"snapshotsDbMem", getDirMemSize(snapshotsDbFilePath),
		"peerTrieDbMem", getDirMemSize(peerTrieDbFilePath),
		"peerTrieEvictionDbMem", getDirMemSize(peerTrieEvictionWaitingListDbFilePath),
		"peerTrieSnapshotsDbMem", getDirMemSize(peerTrieSnapshotsDbFilePath),
		"kappTrieDbMem", getDirMemSize(kappTrieDbFilePath),
		"kappTrieSnapshotsDbMem", getDirMemSize(kappTrieSnapshotsDbFilePath),
	}
}

func getDirMemSize(dir string) string {
	files, _ := os.ReadDir(dir)

	size := int64(0)
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		size += info.Size()
	}

	return tools.ConvertBytes(uint64(size)) // #nosec G115
}

// SaveStatistics generates and saves statistic data on the disk
func (rm *ResourceMonitor) SaveStatistics(generalConfig *config.Config, pathManager storage.PathManagerHandler, chainName string) {

	stats := rm.GenerateStatistics(generalConfig, pathManager, chainName)
	log.Debug("node statistics", stats...)
}
