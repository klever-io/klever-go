package mock

import (
	"math/big"
	"time"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/kapps"
)

// BlockProcessorMock mocks the implementation for a blockProcessor
type BlockProcessorMock struct {
	ProcessBlockCalled               func(header data.HeaderHandler, haveTime func() time.Duration) error
	CommitBlockCalled                func(header data.HeaderHandler) error
	RevertStateToSnapshotCalled      func(header data.HeaderHandler)
	PruneStateOnRollbackCalled       func(currHeader data.HeaderHandler, prevHeader data.HeaderHandler)
	CreateGenesisBlockCalled         func(balances map[string]*big.Int) (data.HeaderHandler, error)
	CreateBlockCalled                func(initialHdrData data.HeaderHandler, haveTime func() bool) (data.HeaderHandler, error)
	RestoreBlockIntoPoolsCalled      func(header data.HeaderHandler) error
	SetOnRequestTransactionCalled    func(f func(txHash []byte))
	MarshalizedDataToBroadcastCalled func(header data.HeaderHandler) ([]byte, [][]byte, error)
	DecodeBlockHeaderCalled          func(dta []byte) data.HeaderHandler
	CreateNewHeaderCalled            func(slot uint64, nonce uint64) data.HeaderHandler
	RevertStateToBlockCalled         func(header data.HeaderHandler) error
	RevertIndexedBlockCalled         func(header data.HeaderHandler)
}

func (bpm *BlockProcessorMock) SetProposalController(controller kapps.ActiveProposalController) error {

	return nil
}

// SetNumProcessedObj -
func (bpm *BlockProcessorMock) SetNumProcessedObj(_ uint64) {
}

// RestoreLastNotarizedHrdsToGenesis -
func (bpm *BlockProcessorMock) RestoreLastNotarizedHrdsToGenesis() {
}

// CreateNewHeader -
func (bpm *BlockProcessorMock) CreateNewHeader(slot uint64, nonce uint64) data.HeaderHandler {
	return bpm.CreateNewHeaderCalled(slot, nonce)
}

// ProcessBlock mocks pocessing a block
func (bpm *BlockProcessorMock) ProcessBlock(header data.HeaderHandler, haveTime func() time.Duration) error {
	return bpm.ProcessBlockCalled(header, haveTime)
}

// CommitBlock mocks the commit of a block
func (bpm *BlockProcessorMock) CommitBlock(header data.HeaderHandler) error {
	return bpm.CommitBlockCalled(header)
}

// PruneStateOnRollback recreates the state tries to the root hashes indicated by the provided header
func (bpm *BlockProcessorMock) PruneStateOnRollback(currHeader data.HeaderHandler, prevHeader data.HeaderHandler) {
	if bpm.PruneStateOnRollbackCalled != nil {
		bpm.PruneStateOnRollbackCalled(currHeader, prevHeader)
	}
}

// RevertStateToBlock recreates the state tries to the root hashes indicated by the provided header
func (bpm *BlockProcessorMock) RevertStateToBlock(header data.HeaderHandler) error {
	if bpm.RevertStateToBlockCalled != nil {
		return bpm.RevertStateToBlockCalled(header)
	}

	return nil
}

// RevertIndexedBlock -
func (bpm *BlockProcessorMock) RevertIndexedBlock(header data.HeaderHandler) {
	if bpm.RevertIndexedBlockCalled != nil {
		bpm.RevertIndexedBlockCalled(header)
	}
}

// RevertStateToSnapshot mocks revert of the accounts state
func (bpm *BlockProcessorMock) RevertStateToSnapshot(header data.HeaderHandler) {
	bpm.RevertStateToSnapshotCalled(header)
}

// CreateBlock mocks the creation of a new block with header and body
func (bpm *BlockProcessorMock) CreateBlock(initialHdrData data.HeaderHandler, haveTime func() bool) (data.HeaderHandler, error) {
	return bpm.CreateBlockCalled(initialHdrData, haveTime)
}

// RestoreBlockIntoPools -
func (bpm *BlockProcessorMock) RestoreBlockIntoPools(header data.HeaderHandler) error {
	return bpm.RestoreBlockIntoPoolsCalled(header)
}

// MarshalizedDataToBroadcast -
func (bpm *BlockProcessorMock) MarshalizedDataToBroadcast(header data.HeaderHandler) ([]byte, [][]byte, error) {
	return bpm.MarshalizedDataToBroadcastCalled(header)
}

// DecodeBlockHeader -
func (bpm *BlockProcessorMock) DecodeBlockHeader(dta []byte) data.HeaderHandler {
	return bpm.DecodeBlockHeaderCalled(dta)
}

// IsInterfaceNil returns true if there is no value under the interface
func (bpm *BlockProcessorMock) IsInterfaceNil() bool {
	return bpm == nil
}
