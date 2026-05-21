package metrics

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	"github.com/klever-io/klever-go/core/statistics/machine"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/tools/check"
)

// StartMachineStatisticsPolling will start read information about current running machine.
// The returned io.Closer stops the AppStatusPolling goroutine; the inner goroutines in
// registerCPUStatistics / registerNetStatistics / registerDiskStatistics still leak
func StartMachineStatisticsPolling(ash core.AppStatusHandler, notifier eventNotifier.EpochStartEventNotifier, pollingInterval time.Duration, workingDir string) (io.Closer, error) {
	if check.IfNil(ash) {
		return nil, errors.New("nil AppStatusHandler")
	}
	if check.IfNil(notifier) {
		return nil, errors.New("nil EpochStartEventNotifier")
	}

	appStatusPollingHandler, err := appStatusPolling.NewAppStatusPolling(ash, pollingInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot init AppStatusPolling: %w", err)
	}

	err = registerCPUStatistics(appStatusPollingHandler)
	if err != nil {
		return nil, err
	}

	err = registerMemStatistics(appStatusPollingHandler)
	if err != nil {
		return nil, err
	}

	err = registerNetStatistics(appStatusPollingHandler, notifier)
	if err != nil {
		return nil, err
	}

	err = registerDiskStatistics(appStatusPollingHandler, workingDir)
	if err != nil {
		return nil, err
	}

	appStatusPollingHandler.Poll()

	return appStatusPollingHandler, nil
}

func registerMemStatistics(appStatusPollingHandler *appStatusPolling.AppStatusPolling) error {
	return appStatusPollingHandler.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		mem := machine.AcquireMemStatistics()

		appStatusHandler.SetUInt64Value(core.MetricMemLoadPercent, mem.PercentUsed)
		appStatusHandler.SetUInt64Value(core.MetricMemTotal, mem.Total)
		appStatusHandler.SetUInt64Value(core.MetricMemUsedGolang, mem.UsedByGolang)
		appStatusHandler.SetUInt64Value(core.MetricMemUsedSystem, mem.UsedBySystem)
		appStatusHandler.SetUInt64Value(core.MetricMemHeapInUse, mem.HeapInUse)
		appStatusHandler.SetUInt64Value(core.MetricMemStackInUse, mem.StackInUse)
	})
}

func registerNetStatistics(appStatusPollingHandler *appStatusPolling.AppStatusPolling, notifier eventNotifier.EpochStartEventNotifier) error {
	netStats := machine.NewNetStatistics()
	notifier.RegisterHandler(netStats.EpochStartEventHandler())
	go func() {
		for {
			netStats.ComputeStatistics()
		}
	}()

	return appStatusPollingHandler.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetUInt64Value(core.MetricNetworkRecvBps, netStats.BpsRecv())
		appStatusHandler.SetUInt64Value(core.MetricNetworkRecvBpsPeak, netStats.BpsRecvPeak())
		appStatusHandler.SetUInt64Value(core.MetricNetworkRecvPercent, netStats.PercentRecv())

		appStatusHandler.SetUInt64Value(core.MetricNetworkSentBps, netStats.BpsSent())
		appStatusHandler.SetUInt64Value(core.MetricNetworkSentBpsPeak, netStats.BpsSentPeak())
		appStatusHandler.SetUInt64Value(core.MetricNetworkSentPercent, netStats.PercentSent())

		appStatusHandler.SetUInt64Value(core.MetricNetworkRecvBytesInCurrentEpochPerHost, netStats.TotalBytesReceivedInCurrentEpoch())
		appStatusHandler.SetUInt64Value(core.MetricNetworkSendBytesInCurrentEpochPerHost, netStats.TotalBytesSentInCurrentEpoch())
	})
}

func registerDiskStatistics(appStatusPollingHandler *appStatusPolling.AppStatusPolling, workingDir string) error {
	dbPath := filepath.Join(workingDir, factory.DefaultDBPath)
	diskStats := machine.NewDiskStatistics(workingDir, dbPath)
	go func() {
		for {
			diskStats.ComputeStatistics()
			time.Sleep(60 * time.Second)
		}
	}()

	return appStatusPollingHandler.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetUInt64Value(core.MetricDiskTotalBytes, diskStats.DiskTotal())
		appStatusHandler.SetUInt64Value(core.MetricDiskAvailableBytes, diskStats.DiskAvailable())
		appStatusHandler.SetUInt64Value(core.MetricDiskUsagePercent, diskStats.DiskPercent())
		appStatusHandler.SetUInt64Value(core.MetricDbSizeBytes, diskStats.DbSize())
	})
}

func registerCPUStatistics(appStatusPollingHandler *appStatusPolling.AppStatusPolling) error {
	cpuStats, err := machine.NewCpuStatistics()
	if err != nil {
		return err
	}

	go func() {
		for {
			cpuStats.ComputeStatistics()
		}
	}()

	return appStatusPollingHandler.RegisterPollingFunc(func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetUInt64Value(core.MetricCPULoadPercent, cpuStats.CpuPercentUsage())
		appStatusHandler.SetUInt64Value(core.MetricSystemCPUPercent, cpuStats.SystemCpuPercentUsage())
	})
}
