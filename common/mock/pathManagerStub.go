package mock

import (
	"fmt"
)

// PathManagerStub -
type PathManagerStub struct {
	PathForEpochCalled  func(epoch uint32, identifier string) string
	PathForStaticCalled func(identifier string) string
	DatabasePathCalled  func() string
}

// PathForEpoch -
func (p *PathManagerStub) PathForEpoch(epoch uint32, identifier string) string {
	if p.PathForEpochCalled != nil {
		return p.PathForEpochCalled(epoch, identifier)
	}

	return fmt.Sprintf("Epoch_%d/%s", epoch, identifier)
}

// PathForStatic -
func (p *PathManagerStub) PathForStatic(identifier string) string {
	if p.PathForEpochCalled != nil {
		return p.PathForStaticCalled(identifier)
	}

	return fmt.Sprintf("Static/%s", identifier)
}

// DatabasePath -
func (p *PathManagerStub) DatabasePath() string {
	if p.DatabasePathCalled != nil {
		return p.DatabasePathCalled()
	}

	return "db"
}

// IsInterfaceNil -
func (p *PathManagerStub) IsInterfaceNil() bool {
	return p == nil
}
