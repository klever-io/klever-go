package mock

import (
	"github.com/klever-io/klever-go/tools"
)

// BlockSizeThrottlerStub -
type BlockSizeThrottlerStub struct {
	GetCurrentMaxSizeCalled                           func() uint32
	AddCalled                                         func(slot uint64, size uint32)
	SucceedCalled                                     func(slot uint64)
	ComputeCurrentMaxSizeCalled                       func()
	getMaxSizeWhenNotSucceedCalled                    func(lastActionMaxSize uint32) uint32
	getCloserBelowCurrentMaxSizeUsedWithSucceedCalled func(currentMaxSize uint32) uint32
}

// GetCurrentMaxSize -
func (bsts *BlockSizeThrottlerStub) GetCurrentMaxSize() uint32 {
	if bsts.GetCurrentMaxSizeCalled != nil {
		return bsts.GetCurrentMaxSizeCalled()
	}

	return uint32(tools.MegabyteSize * 90 / 100)
}

// Add -
func (bsts *BlockSizeThrottlerStub) Add(slot uint64, size uint32) {
	if bsts.AddCalled != nil {
		bsts.AddCalled(slot, size)
		return
	}
}

// Succeed -
func (bsts *BlockSizeThrottlerStub) Succeed(slot uint64) {
	if bsts.SucceedCalled != nil {
		bsts.SucceedCalled(slot)
		return
	}
}

// ComputeCurrentMaxSize -
func (bsts *BlockSizeThrottlerStub) ComputeCurrentMaxSize() {
	if bsts.ComputeCurrentMaxSizeCalled != nil {
		bsts.ComputeCurrentMaxSizeCalled()
		return
	}
}

// getMaxSizeWhenNotSucceed -
func (bsts *BlockSizeThrottlerStub) getMaxSizeWhenNotSucceed(lastActionMaxSize uint32) uint32 {
	if bsts.getMaxSizeWhenNotSucceedCalled != nil {
		return bsts.getMaxSizeWhenNotSucceedCalled(lastActionMaxSize)
	}

	return uint32(tools.MegabyteSize * 90 / 100)
}

// getMaxSizeWhenNotSucceed -
func (bsts *BlockSizeThrottlerStub) getCloserBelowCurrentMaxSizeUsedWithSucceed(lastActionMaxSize uint32) uint32 {
	if bsts.getCloserBelowCurrentMaxSizeUsedWithSucceedCalled != nil {
		return bsts.getCloserBelowCurrentMaxSizeUsedWithSucceedCalled(lastActionMaxSize)
	}

	return uint32(tools.MegabyteSize * 90 / 100)
}

// IsInterfaceNil returns true if there is no value under the interface
func (bsts *BlockSizeThrottlerStub) IsInterfaceNil() bool {
	return bsts == nil
}
