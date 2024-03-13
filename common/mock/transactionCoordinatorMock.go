package mock

import (
	"time"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// TransactionCoordinatorMock -
type TransactionCoordinatorMock struct {
	RequestBlockTransactionsCalled          func(blk *block.Block)
	IsDataPreparedForProcessingCalled       func(haveTime func() time.Duration) error
	SaveTxsToStorageCalled                  func(blk *block.Block) error
	RestoreBlockDataFromStorageCalled       func(blk *block.Block) (int, error)
	RemoveTxsFromPoolCalled                 func(blk *block.Block) error
	ProcessBlockTransactionsCalled          func(blk *block.Block, timeRemaining func() time.Duration) error
	CreateBlockStartedCalled                func()
	CreateMarshalizedDataCalled             func(blk *block.Block) ([][]byte, error)
	GetAllCurrentUsedTxsCalled              func() map[string]data.TransactionHandler
	VerifyCreatedBlockTransactionsCalled    func(blk *block.Block) error
	CreateAndProcessBlockTransactionsCalled func(blk *block.Block, haveTime func() bool) error
	RemoveBlockDataFromPoolCalled           func(body *block.Block) error
	GetAllCurrentLogsCalled                 func() []*data.LogData
}

// CreateReceiptsHash -
func (tcm *TransactionCoordinatorMock) CreateReceiptsHash() ([]byte, error) {
	return []byte("receiptHash"), nil
}

// RequestBlockTransactions -
func (tcm *TransactionCoordinatorMock) RequestBlockTransactions(blk *block.Block) {
	if tcm.RequestBlockTransactionsCalled == nil {
		return
	}

	tcm.RequestBlockTransactionsCalled(blk)
}

// IsDataPreparedForProcessing -
func (tcm *TransactionCoordinatorMock) IsDataPreparedForProcessing(haveTime func() time.Duration) error {
	if tcm.IsDataPreparedForProcessingCalled == nil {
		return nil
	}

	return tcm.IsDataPreparedForProcessingCalled(haveTime)
}

// SaveTxsToStorage -
func (tcm *TransactionCoordinatorMock) SaveTxsToStorage(blk *block.Block) error {
	if tcm.SaveTxsToStorageCalled == nil {
		return nil
	}

	return tcm.SaveTxsToStorageCalled(blk)
}

// RestoreBlockDataFromStorage -
func (tcm *TransactionCoordinatorMock) RestoreBlockDataFromStorage(blk *block.Block) (int, error) {
	if tcm.RestoreBlockDataFromStorageCalled == nil {
		return 0, nil
	}

	return tcm.RestoreBlockDataFromStorageCalled(blk)
}

// RemoveTxsFromPool -
func (tcm *TransactionCoordinatorMock) RemoveTxsFromPool(blk *block.Block) error {
	if tcm.RemoveTxsFromPoolCalled == nil {
		return nil
	}

	return tcm.RemoveTxsFromPoolCalled(blk)
}

// ProcessBlockTransaction -
func (tcm *TransactionCoordinatorMock) ProcessBlockTransactions(blk *block.Block, timeRemaining func() time.Duration) error {
	if tcm.ProcessBlockTransactionsCalled == nil {
		return nil
	}

	return tcm.ProcessBlockTransactionsCalled(blk, timeRemaining)
}

// CreateBlockStarted -
func (tcm *TransactionCoordinatorMock) CreateBlockStarted() {
	if tcm.CreateBlockStartedCalled == nil {
		return
	}

	tcm.CreateBlockStartedCalled()
}

// CreateMarshalizedData -
func (tcm *TransactionCoordinatorMock) CreateMarshalizedData(blk *block.Block) ([][]byte, error) {
	if tcm.CreateMarshalizedDataCalled == nil {
		return nil, nil
	}

	return tcm.CreateMarshalizedDataCalled(blk)
}

// GetAllCurrentUsedTxs -
func (tcm *TransactionCoordinatorMock) GetAllCurrentUsedTxs() map[string]data.TransactionHandler {
	if tcm.GetAllCurrentUsedTxsCalled == nil {
		return nil
	}

	return tcm.GetAllCurrentUsedTxsCalled()
}

// VerifyCreatedBlockTransactions -
func (tcm *TransactionCoordinatorMock) VerifyCreatedBlockTransactions(blk *block.Block) error {
	if tcm.VerifyCreatedBlockTransactionsCalled == nil {
		return nil
	}

	return tcm.VerifyCreatedBlockTransactionsCalled(blk)
}

// CreateAndProcessBlocks creates blocks from storage and processes the reward transactions added into the blocks
// as long as it has time
func (tcm *TransactionCoordinatorMock) CreateAndProcessBlockTransactions(blk *block.Block, haveTime func() bool) error {
	if tcm.CreateAndProcessBlockTransactionsCalled == nil {
		return nil
	}
	return tcm.CreateAndProcessBlockTransactionsCalled(blk, haveTime)
}

// RemoveBlockDataFromPool deletes block data from pools
func (tcm *TransactionCoordinatorMock) RemoveBlockDataFromPool(blk *block.Block) error {
	if tcm.RemoveBlockDataFromPoolCalled == nil {
		return nil
	}
	return tcm.RemoveBlockDataFromPoolCalled(blk)
}

// GetAllCurrentLogs returns all current logs
func (tcm *TransactionCoordinatorMock) GetAllCurrentLogs() []*data.LogData {
	if tcm.GetAllCurrentLogsCalled == nil {
		return make([]*data.LogData, 0)
	}
	return tcm.GetAllCurrentLogsCalled()
}

// IsInterfaceNil returns true if there is no value under the interface
func (tcm *TransactionCoordinatorMock) IsInterfaceNil() bool {
	return tcm == nil
}
