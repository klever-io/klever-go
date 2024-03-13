package mock

import (
	"github.com/klever-io/klever-go/data/block"
)

// HeaderSyncHandlerStub -
type HeaderSyncHandlerStub struct {
	SyncUnFinishedMetaHeadersCalled func(epoch uint32) error
	GetEpochStartMetaBlockCalled    func() (*block.Block, error)
	GetUnFinishedMetaBlocksCalled   func() (map[string]*block.Block, error)
}

// SyncUnFinishedMetaHeaders -
func (hsh *HeaderSyncHandlerStub) SyncUnFinishedMetaHeaders(epoch uint32) error {
	if hsh.SyncUnFinishedMetaHeadersCalled != nil {
		return hsh.SyncUnFinishedMetaHeadersCalled(epoch)
	}
	return nil
}

// GetEpochStartMetaBlock -
func (hsh *HeaderSyncHandlerStub) GetEpochStartMetaBlock() (*block.Block, error) {
	if hsh.GetEpochStartMetaBlockCalled != nil {
		return hsh.GetEpochStartMetaBlockCalled()
	}
	return nil, nil
}

// GetUnFinishedMetaBlocks -
func (hsh *HeaderSyncHandlerStub) GetUnFinishedMetaBlocks() (map[string]*block.Block, error) {
	if hsh.GetUnFinishedMetaBlocksCalled != nil {
		return hsh.GetUnFinishedMetaBlocksCalled()
	}
	return nil, nil
}

// IsInterfaceNil -
func (hsh *HeaderSyncHandlerStub) IsInterfaceNil() bool {
	return hsh == nil
}
