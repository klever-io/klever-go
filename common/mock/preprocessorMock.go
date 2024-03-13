package mock

import (
	"time"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/storage"
)

// PreProcessorMock -
type PreProcessorMock struct {
	CreateBlockStartedCalled                func()
	IsDataPreparedCalled                    func(requestedTxs int, haveTime func() time.Duration) error
	RemoveBlockDataFromPoolsCalled          func(body *block.Block, blkPool storage.Cacher) error
	RemoveTxsFromPoolsCalled                func(body *block.Block) error
	RestoreBlockDataIntoPoolsCalled         func(body *block.Block) (int, error)
	SaveTxsToStorageCalled                  func(body *block.Block) error
	ProcessBlockTransactionsCalled          func(body *block.Block, haveTime func() bool) ([][]byte, int, error)
	RequestBlockTransactionsCalled          func(body *block.Block) int
	CreateMarshalizedDataCalled             func(txHashes [][]byte) ([][]byte, error)
	RequestTransactionsForBlockCalled       func(blk *block.Block) int
	ProcessBlockCalled                      func(blk *block.Block, haveTime func() bool, getNumOfCrossInterMbsAndTxs func() (int, int)) ([][]byte, int, error)
	CreateAndProcessBlockTransactionsCalled func(blk *block.Block, haveTime func() bool) ([][]byte, int, error)
	GetAllCurrentUsedTxsCalled              func() map[string]data.TransactionHandler
	IsDataPreparedForProcessingCalled       func(haveTime func() time.Duration) error
	VerifyCreatedBlockTransactionsCalled    func(blk *block.Block) error
}

// CreateBlockStarted -
func (ppm *PreProcessorMock) CreateBlockStarted() {
	if ppm.CreateBlockStartedCalled == nil {
		return
	}
	ppm.CreateBlockStartedCalled()
}

// IsDataPrepared -
func (ppm *PreProcessorMock) IsDataPrepared(requestedTxs int, haveTime func() time.Duration) error {
	if ppm.IsDataPreparedCalled == nil {
		return nil
	}
	return ppm.IsDataPreparedCalled(requestedTxs, haveTime)
}

// RemoveBlockDataFromPools -
func (ppm *PreProcessorMock) RemoveBlockDataFromPools(body *block.Block, blkPool storage.Cacher) error {
	if ppm.RemoveBlockDataFromPoolsCalled == nil {
		return nil
	}
	return ppm.RemoveBlockDataFromPoolsCalled(body, blkPool)
}

// RemoveTxsFromPools -
func (ppm *PreProcessorMock) RemoveTxsFromPools(body *block.Block) error {
	if ppm.RemoveTxsFromPoolsCalled == nil {
		return nil
	}
	return ppm.RemoveTxsFromPoolsCalled(body)
}

// RestoreBlockDataIntoPools -
func (ppm *PreProcessorMock) RestoreBlockDataIntoPools(body *block.Block) (int, error) {
	if ppm.RestoreBlockDataIntoPoolsCalled == nil {
		return 0, nil
	}
	return ppm.RestoreBlockDataIntoPoolsCalled(body)
}

// SaveTxsToStorage -
func (ppm *PreProcessorMock) SaveTxsToStorage(body *block.Block) error {
	if ppm.SaveTxsToStorageCalled == nil {
		return nil
	}
	return ppm.SaveTxsToStorageCalled(body)
}

// ProcessBlockTransactions -
func (ppm *PreProcessorMock) ProcessBlockTransactions(body *block.Block, haveTime func() bool) ([][]byte, int, error) {
	if ppm.ProcessBlockTransactionsCalled == nil {
		return nil, 0, nil
	}
	return ppm.ProcessBlockTransactionsCalled(body, haveTime)
}

// RequestBlockTransactions -
func (ppm *PreProcessorMock) RequestBlockTransactions(body *block.Block) int {
	if ppm.RequestBlockTransactionsCalled == nil {
		return 0
	}
	return ppm.RequestBlockTransactionsCalled(body)
}

// CreateMarshalizedData -
func (ppm *PreProcessorMock) CreateMarshalizedData(txHashes [][]byte) ([][]byte, error) {
	if ppm.CreateMarshalizedDataCalled == nil {
		return nil, nil
	}
	return ppm.CreateMarshalizedDataCalled(txHashes)
}

// RequestTransactionsForBlock -
func (ppm *PreProcessorMock) RequestTransactionsForBlock(blk *block.Block) int {
	if ppm.RequestTransactionsForBlockCalled == nil {
		return 0
	}
	return ppm.RequestTransactionsForBlockCalled(blk)
}

// ProcessBlock -
func (ppm *PreProcessorMock) ProcessBlock(blk *block.Block, haveTime func() bool, getNumOfCrossInterMbsAndTxs func() (int, int)) ([][]byte, int, error) {
	if ppm.ProcessBlockCalled == nil {
		return nil, 0, nil
	}
	return ppm.ProcessBlockCalled(blk, haveTime, getNumOfCrossInterMbsAndTxs)
}

// CreateAndProcessBlocks creates blocks from storage and processes the reward transactions added into the blocks
// as long as it has time
func (ppm *PreProcessorMock) CreateAndProcessBlockTransactions(blk *block.Block, haveTime func() bool) ([][]byte, int, error) {
	if ppm.CreateAndProcessBlockTransactionsCalled == nil {
		return nil, 0, nil
	}
	return ppm.CreateAndProcessBlockTransactionsCalled(blk, haveTime)
}

// GetAllCurrentUsedTxs -
func (ppm *PreProcessorMock) GetAllCurrentUsedTxs() map[string]data.TransactionHandler {
	if ppm.GetAllCurrentUsedTxsCalled == nil {
		return nil
	}
	return ppm.GetAllCurrentUsedTxsCalled()
}

// IsDataPreparedForProcessing -
func (ppm *PreProcessorMock) IsDataPreparedForProcessing(haveTime func() time.Duration) error {
	if ppm.IsDataPreparedForProcessingCalled != nil {
		return ppm.IsDataPreparedForProcessingCalled(haveTime)
	}
	return nil
}

// VerifyCreatedBlockTransactions -
func (ppm *PreProcessorMock) VerifyCreatedBlockTransactions(blk *block.Block) error {
	if ppm.VerifyCreatedBlockTransactionsCalled != nil {
		return ppm.VerifyCreatedBlockTransactionsCalled(blk)
	}
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (ppm *PreProcessorMock) IsInterfaceNil() bool {
	return ppm == nil
}
