package mock

import "github.com/klever-io/klever-go/core/process/block/bootstrapStorage"

// BoostrapStorerMock -
type BoostrapStorerMock struct {
	PutCalled                         func(slot int64, bootData *bootstrapStorage.BootstrapData) error
	GetCalled                         func(slot int64) (*bootstrapStorage.BootstrapData, error)
	GetHighestSlotCalled              func() int64
	SaveLastSlotCalled                func(slot int64) error
	RaiseHighestFinalBlockNonceCalled func(slot int64, nonce uint64) error
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
func (bsm *BoostrapStorerMock) SaveLastSlot(slot int64) error {
	if bsm.SaveLastSlotCalled != nil {
		return bsm.SaveLastSlotCalled(slot)
	}
	return nil
}

func (bsm *BoostrapStorerMock) RaiseHighestFinalBlockNonce(slot int64, nonce uint64) error {
	if bsm.RaiseHighestFinalBlockNonceCalled != nil {
		return bsm.RaiseHighestFinalBlockNonceCalled(slot, nonce)
	}
	return nil
}

// IsInterfaceNil -
func (bsm *BoostrapStorerMock) IsInterfaceNil() bool {
	return bsm == nil
}
