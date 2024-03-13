package mock

import (
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/data"
)

// SlotHandlerMock -
type SlotHandlerMock struct {
	IsMinerCalled        func(blsPubKey []byte) bool
	ProduceBlockCalled   func(blkc data.ChainHandler, sm consensus.SlotManager) (data.HeaderHandler, []byte, error)
	ProducerPubKeyCalled func() ([]byte, error)
}

// IsMiner -
func (shm *SlotHandlerMock) IsMiner(blsPubKey []byte) bool {
	return shm.IsMinerCalled(blsPubKey)
}

// ProducerPubKey -
func (shm *SlotHandlerMock) ProducerPubKey() ([]byte, error) {
	if shm.ProducerPubKeyCalled != nil {
		return shm.ProducerPubKeyCalled()
	}
	return make([]byte, 96), nil
}

// ProduceBlock -
func (shm *SlotHandlerMock) ProduceBlock(blkc data.ChainHandler, sm consensus.SlotManager) (data.HeaderHandler, []byte, error) {
	return shm.ProduceBlockCalled(blkc, sm)
}

// IsInterfaceNil returns true if there is no value under the interface
func (shm *SlotHandlerMock) IsInterfaceNil() bool {
	return shm == nil
}
