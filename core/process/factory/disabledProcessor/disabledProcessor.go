package disabledProcessor

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
)

var _ process.InterceptorProcessor = (*processor)(nil)

type processor struct {
}

func (p *processor) Validate(data process.InterceptedData, fromConnectedPeer core.PeerID) error {
	return nil
}

func (p *processor) Save(data process.InterceptedData, fromConnectedPeer core.PeerID, topic string) error {
	return nil
}

func (p *processor) Notify(data process.InterceptedData, fromConnectedPeer core.PeerID, topic string) error {
	//
	return nil
}

func (p *processor) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {

}

// CommitBlock -
func (p *processor) CommitBlock(header data.HeaderHandler) error {
	return nil
}

// CreateBlock -
func (p *processor) CreateBlock(initialHdrData data.HeaderHandler, haveTime func() bool) (data.HeaderHandler, error) {
	return nil, nil
}

// CreateNewHeader -
func (p *processor) CreateNewHeader(slot uint64, nonce uint64) data.HeaderHandler {
	return nil
}

// DecodeBlockHeader -
func (p *processor) DecodeBlockHeader(dta []byte) data.HeaderHandler {
	return nil
}

// MarshalizedDataToBroadcast -
func (p *processor) MarshalizedDataToBroadcast(header data.HeaderHandler) ([]byte, [][]byte, error) {
	return nil, nil, nil
}

// SetNumProcessedObj will set the num of processed transactions
func (p *processor) SetNumProcessedObj(_ uint64) {

}

// ProcessBlock -
func (p *processor) ProcessBlock(header data.HeaderHandler, haveTime func() time.Duration) error {
	return nil
}

// PruneStateOnRollback recreates thee state tries to the root hashes indicated by the provided header
func (p *processor) PruneStateOnRollback(currHeader data.HeaderHandler, prevHeader data.HeaderHandler) {

}

// RestoreBlockIntoPools -
func (p *processor) RestoreBlockIntoPools(header data.HeaderHandler) error {
	return nil
}

// RevertStateToSnapshot -
func (p *processor) RevertStateToSnapshot(header data.HeaderHandler) {

}

// RevertStateToBlock recreates thee state tries to the root hashes indicated by the provided header
func (p *processor) RevertStateToBlock(header data.HeaderHandler) error {

	return nil
}

func (p *processor) IsInterfaceNil() bool {
	return false
}

// NewDisabledProcessor -
func NewDisabledProcessor() (*processor, error) {
	return &processor{}, nil
}
