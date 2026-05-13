package machine

import (
	"os"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/shirou/gopsutil/v4/process"
)

var log = logger.GetOrCreate("statistics/machine")

// GetCurrentProcess returns details about the current process
func GetCurrentProcess() (*process.Process, error) {
	checkPid := os.Getpid()
	ret, err := process.NewProcess(int32(checkPid)) // #nosec G115
	if err != nil {
		return nil, err
	}

	return ret, nil
}
