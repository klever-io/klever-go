package mock

import "github.com/klever-io/klever-go/storage"

// UnitOpenerStub -
type UnitOpenerStub struct {
}

// GetMostRecentBootstrapStorageUnit -
func (u *UnitOpenerStub) GetMostRecentBootstrapStorageUnit() (storage.Storer, error) {
	return &StorerMock{}, nil
}

// IsInterfaceNil -
func (u *UnitOpenerStub) IsInterfaceNil() bool {
	return u == nil
}
