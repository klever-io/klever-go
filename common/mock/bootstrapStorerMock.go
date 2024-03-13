package mock

import "github.com/klever-io/klever-go/core/process/block/bootstrapStorage"

// BoostrapStorerMock -
type BoostrapStorerMock struct {
	PutCalled            func(slot int64, bootData *bootstrapStorage.BootstrapData) error
	GetCalled            func(slot int64) (*bootstrapStorage.BootstrapData, error)
	GetHighestSlotCalled func() int64
}

// Put -
func (bsm *BoostrapStorerMock) Put(slot int64, bootData *bootstrapStorage.BootstrapData) error {
	if bsm.PutCalled != nil {
		return bsm.PutCalled(slot, bootData)
	}
	return nil
}

func (bsm *BoostrapStorerMock) Get(slot int64) (*bootstrapStorage.BootstrapData, error) {
	return bsm.GetCalled(slot)
}

// GetHighestSlot -
func (bsm *BoostrapStorerMock) GetHighestSlot() int64 {
	return bsm.GetHighestSlotCalled()
}

// SaveLastSlot -
func (bsm *BoostrapStorerMock) SaveLastSlot(_ int64) error {
	return nil
}

// IsInterfaceNil -
func (bsm *BoostrapStorerMock) IsInterfaceNil() bool {
	return bsm == nil
}
