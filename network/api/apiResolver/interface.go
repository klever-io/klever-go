package apiResolver

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

// TotalStakedValueHandler defines the behavior of a component able to return total staked value
type TotalStakedValueHandler interface {
	GetTotalStakedValue() (int64, error)
	IsInterfaceNil() bool
}

// SCQueryService defines how data should be get from a SC account
type SCQueryService interface {
	ExecuteQuery(query *process.SCQuery) (*vmcommon.VMOutput, error)
	// ComputeScCallGasLimit(tx *transaction.Transaction, tc data.SmartContractHandler) (uint64, error)
	Close() error
	IsInterfaceNil() bool
}

// TransactionCostHandler defines the actions which should be handler by a transaction cost estimator
type TransactionCostHandler interface {
	EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error)
	IsInterfaceNil() bool
}
