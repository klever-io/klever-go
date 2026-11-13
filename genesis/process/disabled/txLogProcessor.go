package disabled

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ process.TransactionLogProcessor = (*TxLogProcessor)(nil)

// TxLogProcessor discards every log, for executions that must never reach the block's logs
type TxLogProcessor struct {
}

// GetAllCurrentLogs returns nothing
func (tlp *TxLogProcessor) GetAllCurrentLogs() []*data.LogData {
	return nil
}

// GetLog returns no log
func (tlp *TxLogProcessor) GetLog(_ []byte) (data.LogHandler, error) {
	return nil, process.ErrLogNotFound
}

// SaveLog discards the log
func (tlp *TxLogProcessor) SaveLog(_ []byte, _ []byte, _ data.SmartContractHandler, _ int, _ []*vmcommon.LogEntry) error {
	return nil
}

// SaveLogsToStorage does nothing
func (tlp *TxLogProcessor) SaveLogsToStorage() error {
	return nil
}

// Clean does nothing
func (tlp *TxLogProcessor) Clean() {
	// no-op: SaveLog discards every log, so there is nothing to clean
}

// RemoveLog does nothing
func (tlp *TxLogProcessor) RemoveLog(_ []byte) {
	// no-op: SaveLog discards every log, so there is nothing to remove
}

// IsInterfaceNil returns true if there is no value under the interface
func (tlp *TxLogProcessor) IsInterfaceNil() bool {
	return tlp == nil
}
