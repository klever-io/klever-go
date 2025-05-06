package mock

import (
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/data"
)

// BroadcastMessengerMock -
type BroadcastMessengerMock struct {
	BroadcastBlockCalled                     func(data.HeaderHandler) error
	BroadcastTransactionsCalled              func([][]byte) error
	BroadcastConsensusMessageCalled          func(*consensus.Message) error
	BroadcastBlockDataLeaderCalled           func(h data.HeaderHandler, blockBuff []byte, txs [][]byte) error
	PrepareBroadcastHeaderValidatorCalled    func(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte)
	PrepareBroadcastBlockDataValidatorCalled func(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte)
}

// BroadcastBlock -
func (bmm *BroadcastMessengerMock) BroadcastBlock(headerhandler data.HeaderHandler) error {
	if bmm.BroadcastBlockCalled != nil {
		return bmm.BroadcastBlockCalled(headerhandler)
	}
	return nil
}

// BroadcastBlockAndTransactions -
func (bmm *BroadcastMessengerMock) BroadcastBlockAndTransactions(blockBuff []byte, txsBuff [][]byte) error {
	return nil
}

// BroadcastTransactions -
func (bmm *BroadcastMessengerMock) BroadcastTransactions(transactions [][]byte) error {
	if bmm.BroadcastTransactionsCalled != nil {
		return bmm.BroadcastTransactionsCalled(transactions)
	}
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (bmm *BroadcastMessengerMock) IsInterfaceNil() bool {
	return bmm == nil
}

// BroadcastConsensusMessage -
func (bmm *BroadcastMessengerMock) BroadcastConsensusMessage(message *consensus.Message) error {
	if bmm.BroadcastConsensusMessageCalled != nil {
		return bmm.BroadcastConsensusMessageCalled(message)
	}
	return nil
}

// BroadcastBlockDataLeader -
func (bmm *BroadcastMessengerMock) BroadcastBlockDataLeader(
	header data.HeaderHandler,
	blockBuff []byte,
	transactions [][]byte,
) error {
	if bmm.BroadcastBlockDataLeaderCalled != nil {
		return bmm.BroadcastBlockDataLeaderCalled(
			header,
			blockBuff,
			transactions,
		)
	}
	return nil
}

func (bbm *BroadcastMessengerMock) PrepareBroadcastHeaderValidator(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte) {
	if bbm.PrepareBroadcastHeaderValidatorCalled != nil {
		bbm.PrepareBroadcastHeaderValidatorCalled(
			header,
			transactions,
			idx,
			pkBytes,
		)
	}
}

func (bbm *BroadcastMessengerMock) PrepareBroadcastBlockDataValidator(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte) {
	if bbm.PrepareBroadcastBlockDataValidatorCalled != nil {
		bbm.PrepareBroadcastBlockDataValidatorCalled(
			header,
			transactions,
			idx,
			pkBytes,
		)
	}
}
