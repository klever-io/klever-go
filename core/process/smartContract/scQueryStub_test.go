package smartContract_test

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/vmcommon"
)

// ScQueryStub -
type ScQueryStub struct {
	ExecuteQueryCalled func(query *process.SCQuery) (*vmcommon.VMOutput, error)
	CloseCalled        func() error
}

// ExecuteQuery -
func (s *ScQueryStub) ExecuteQuery(query *process.SCQuery) (*vmcommon.VMOutput, error) {
	if s.ExecuteQueryCalled != nil {
		return s.ExecuteQueryCalled(query)
	}
	return &vmcommon.VMOutput{}, nil
}

// Close -
func (s *ScQueryStub) Close() error {
	if s.CloseCalled != nil {
		return s.CloseCalled()
	}

	return nil
}

// IsInterfaceNil -
func (s *ScQueryStub) IsInterfaceNil() bool {
	return s == nil
}
