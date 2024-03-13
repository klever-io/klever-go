package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/endProcess"
)

// BootstrapperMock mocks the implementation for a Bootstrapper
type BootstrapperMock struct {
	CreateAndCommitEmptyBlockCalled func() (data.HeaderHandler, error)
	AddSyncStateListenerCalled      func(func(bool))
	GetNodeStateCalled              func() core.NodeState
	StartSyncingBlocksCalled        func()
	SetStatusHandlerCalled          func(handler core.AppStatusHandler) error
}

// CreateAndCommitEmptyBlock -
func (boot *BootstrapperMock) CreateAndCommitEmptyBlock() (data.HeaderHandler, error) {
	if boot.CreateAndCommitEmptyBlockCalled != nil {
		return boot.CreateAndCommitEmptyBlockCalled()
	}

	return &block.Block{}, nil
}

func (boot *BootstrapperMock) LoadStorage() {

}

func (boot *BootstrapperMock) LoadSyncUntil(syncUntil uint32) {

}

func (boot *BootstrapperMock) LoadStopNodeChannel(chanStopNodeProcess chan endProcess.ArgEndProcess) {

}

// AddSyncStateListener -
func (boot *BootstrapperMock) AddSyncStateListener(syncStateNotifier func(isSyncing bool)) {
	if boot.AddSyncStateListenerCalled != nil {
		boot.AddSyncStateListenerCalled(syncStateNotifier)
	}
}

// GetNodeState -
func (boot *BootstrapperMock) GetNodeState() core.NodeState {
	if boot.GetNodeStateCalled != nil {
		return boot.GetNodeStateCalled()
	}

	return core.NsSynchronized
}

// StartSyncingBlocks -
func (boot *BootstrapperMock) StartSyncingBlocks() {
	boot.StartSyncingBlocksCalled()
}

// SetStatusHandler -
func (boot *BootstrapperMock) SetStatusHandler(handler core.AppStatusHandler) error {
	return boot.SetStatusHandlerCalled(handler)
}

// Close -
func (boot *BootstrapperMock) Close() error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (boot *BootstrapperMock) IsInterfaceNil() bool {
	return boot == nil
}
