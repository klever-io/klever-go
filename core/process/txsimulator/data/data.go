package data

import (
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

// SimulationResults is the data transfer object which will hold results for simulation a transaction's execution
type SimulationResults struct {
	Result     transaction.Transaction_TXResult `json:"result,omitempty"`
	FailReason string                           `json:"failReason,omitempty"`
	Hash       string                           `json:"hash,omitempty"`
	VMOutput   *vmcommon.VMOutput               `json:"-"`
}
