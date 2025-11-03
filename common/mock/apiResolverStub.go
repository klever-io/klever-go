package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

// APIResolverStub -
type APIResolverStub struct {
	EstimateTransactionGasCalled func(tx *transaction.Transaction) (*transaction.CostResponse, error)
	ExecuteSCQueryCalled         func(query *process.SCQuery) (*vmcommon.VMOutput, error)
	StatusMetricsCalled          func() core.StatusMetricsHandler
	GetTotalStakedValueCalled    func() (int64, error)
}

// EstimateTransactionGas -
func (a *APIResolverStub) EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error) {
	if a.EstimateTransactionGasCalled != nil {
		return a.EstimateTransactionGasCalled(tx)
	}
	return nil, nil
}

// ExecuteSCQuery -
func (a *APIResolverStub) ExecuteSCQuery(query *process.SCQuery) (*vmcommon.VMOutput, error) {
	if a.ExecuteSCQueryCalled != nil {
		return a.ExecuteSCQueryCalled(query)
	}
	return nil, nil
}

// StatusMetrics -
func (a *APIResolverStub) StatusMetrics() core.StatusMetricsHandler {
	if a.StatusMetricsCalled != nil {
		return a.StatusMetricsCalled()
	}
	return nil
}

// GetTotalStakedValue -
func (a *APIResolverStub) GetTotalStakedValue() (int64, error) {
	if a.GetTotalStakedValueCalled != nil {
		return a.GetTotalStakedValueCalled()
	}
	return 0, nil
}

// IsInterfaceNil -
func (a *APIResolverStub) IsInterfaceNil() bool {
	return a == nil
}
