package apiResolver

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

// TODO:

// NodeAPIResolver can resolve API requests
type NodeAPIResolver struct {
	statusMetricsHandler    core.StatusMetricsHandler
	totalStakedValueHandler TotalStakedValueHandler
	scQueryService          SCQueryService
	txCostHandler           TransactionCostHandler
}

// NewNodeAPIResolver creates a new NodeAPIResolver instance
func NewNodeAPIResolver(
	statusMetricsHandler core.StatusMetricsHandler,
	totalStakedValueHandler TotalStakedValueHandler,
	scQueryService SCQueryService,
	txCostHandler TransactionCostHandler,
) (*NodeAPIResolver, error) {
	if check.IfNil(statusMetricsHandler) {
		return nil, ErrNilStatusMetrics
	}
	if check.IfNil(totalStakedValueHandler) {
		return nil, ErrNilTotalStakedValueHandler
	}
	if check.IfNil(scQueryService) {
		return nil, ErrNilSCQueryService
	}
	if check.IfNil(txCostHandler) {
		return nil, ErrNilTransactionCostHandler
	}

	return &NodeAPIResolver{
		statusMetricsHandler:    statusMetricsHandler,
		totalStakedValueHandler: totalStakedValueHandler,
		scQueryService:          scQueryService,
		txCostHandler:           txCostHandler,
	}, nil
}

func (nar *NodeAPIResolver) ExecuteSCQuery(query *process.SCQuery) (*vmcommon.VMOutput, error) {
	return nar.scQueryService.ExecuteQuery(query)
}

// EstimateTransactionGas will calculate how many gas a transaction will consume
func (nar *NodeAPIResolver) EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error) {
	return nar.txCostHandler.EstimateTransactionGas(tx)
}

// StatusMetrics returns an implementation of the StatusMetricsHandler interface
func (nar *NodeAPIResolver) StatusMetrics() core.StatusMetricsHandler {
	return nar.statusMetricsHandler
}

// GetTotalStakedValue will return total staked value
func (nar *NodeAPIResolver) GetTotalStakedValue() (int64, error) {
	return nar.totalStakedValueHandler.GetTotalStakedValue()
}

// IsInterfaceNil returns true if there is no value under the interface
func (nar *NodeAPIResolver) IsInterfaceNil() bool {
	return nar == nil
}
