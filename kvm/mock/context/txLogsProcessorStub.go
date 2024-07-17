package mock

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/vmcommon"
)

// TxLogsProcessorStub -
type TxLogsProcessorStub struct {
	GetLogCalled  func(txHash []byte) (data.LogHandler, error)
	SaveLogCalled func(txHash []byte, sender []byte, tc data.SmartContractHandler, contractID int, vmLogs []*vmcommon.LogEntry) error
}

// GetLog -
func (txls *TxLogsProcessorStub) GetLog(txHash []byte) (data.LogHandler, error) {
	if txls.GetLogCalled != nil {
		return txls.GetLogCalled(txHash)
	}

	return nil, nil
}

// Clean -
func (txls *TxLogsProcessorStub) Clean() {
}

// SaveLog -
func (txls *TxLogsProcessorStub) SaveLog(txHash []byte, sender []byte, tc data.SmartContractHandler, contractID int, vmLogs []*vmcommon.LogEntry) error {
	if txls.SaveLogCalled != nil {
		return txls.SaveLogCalled(txHash, sender, tc, contractID, vmLogs)
	}

	return nil
}

// IsInterfaceNil -
func (txls *TxLogsProcessorStub) IsInterfaceNil() bool {
	return txls == nil
}

// GetAllCurrentLogs -
func (txls *TxLogsProcessorStub) GetAllCurrentLogs() []*data.LogData {
	return nil
}
