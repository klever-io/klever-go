package mock

import (
	"context"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
)

// BlockInterceptorProcessorStub -
type BlockInterceptorProcessorStub struct {
	GetEpochStartBlockCalled func() (*block.Block, error)
}

// Validate -
func (m *BlockInterceptorProcessorStub) Validate(_ process.InterceptedData, _ core.PeerID) error {
	return nil
}

func (m *BlockInterceptorProcessorStub) GetPrevEpochStartBlock(ctx context.Context, epoch uint32) (*block.Block, error) {
	return &block.Block{}, nil
}

// Save -
func (m *BlockInterceptorProcessorStub) Save(_ process.InterceptedData, _ core.PeerID, _ string) error {
	return nil
}

// RegisterHandler -
func (m *BlockInterceptorProcessorStub) RegisterHandler(_ func(topic string, hash []byte, data interface{})) {
}

// SignalEndOfProcessing -
func (m *BlockInterceptorProcessorStub) SignalEndOfProcessing(_ []process.InterceptedData) {
}

// IsInterfaceNil -
func (m *BlockInterceptorProcessorStub) IsInterfaceNil() bool {
	return m == nil
}

// GetEpochStartBlock -
func (m *BlockInterceptorProcessorStub) GetEpochStartBlock(_ context.Context) (*block.Block, error) {
	if m.GetEpochStartBlockCalled != nil {
		return m.GetEpochStartBlockCalled()
	}

	return &block.Block{}, nil
}

// SignalEndOfProcessing -
func (m *BlockInterceptorProcessorStub) Notify(data process.InterceptedData, fromConnectedPeer core.PeerID, topic string) error {
	return nil
}
