package mock

import (
	"github.com/klever-io/klever-go/data/block"
)

// HeaderSyncHandlerStub -
type HeaderSyncHandlerStub struct {
	GetEpochStartMetaBlockCalled func(uint32) (*block.Block, error)
}

// GetEpochStartMetaBlock -
func (hsh *HeaderSyncHandlerStub) GetEpochStartMetaBlock(epoch uint32) (*block.Block, error) {
	if hsh.GetEpochStartMetaBlockCalled != nil {
		return hsh.GetEpochStartMetaBlockCalled(epoch)
	}
	return nil, nil
}

// IsInterfaceNil -
func (hsh *HeaderSyncHandlerStub) IsInterfaceNil() bool {
	return hsh == nil
}
