package mock

import (
	"github.com/klever-io/klever-go/data"
)

// BlockChainMock is a mock implementation of the blockchain interface
type BlockChainMock struct {
	GetGenesisHeaderCalled             func() data.HeaderHandler
	SetGenesisHeaderCalled             func(handler data.HeaderHandler) error
	GetGenesisHeaderHashCalled         func() []byte
	SetGenesisHeaderHashCalled         func([]byte)
	GetCurrentBlockHeaderCalled        func() data.HeaderHandler
	SetCurrentBlockHeaderCalled        func(data.HeaderHandler) error
	GetCurrentBlockHeaderHashCalled    func() []byte
	SetCurrentBlockHeaderHashCalled    func([]byte)
	SetCurrentBlockHeaderAndHashCalled func(data.HeaderHandler, []byte) error
	GetCurrentBlockHeaderAndHashCalled func() (data.HeaderHandler, []byte)
	GetCurrentBlockRootHashCalled      func() []byte
	GetLocalHeightCalled               func() int64
	SetLocalHeightCalled               func(int64)
	GetNetworkHeightCalled             func() int64
	SetNetworkHeightCalled             func(int64)
	HasBadBlockCalled                  func([]byte) bool
	PutBadBlockCalled                  func([]byte)
	CreateNewHeaderCalled              func() data.HeaderHandler
}

// GetGenesisHeader returns the genesis block header pointer
func (bc *BlockChainMock) GetGenesisHeader() data.HeaderHandler {
	if bc.GetGenesisHeaderCalled != nil {
		return bc.GetGenesisHeaderCalled()
	}
	return nil
}

// SetGenesisHeader sets the genesis block header pointer
func (bc *BlockChainMock) SetGenesisHeader(genesisBlock data.HeaderHandler) error {
	if bc.SetGenesisHeaderCalled != nil {
		return bc.SetGenesisHeaderCalled(genesisBlock)
	}
	return nil
}

// GetGenesisHeaderHash returns the genesis block header hash
func (bc *BlockChainMock) GetGenesisHeaderHash() []byte {
	if bc.GetGenesisHeaderHashCalled != nil {
		return bc.GetGenesisHeaderHashCalled()
	}
	return nil
}

// SetGenesisHeaderHash sets the genesis block header hash
func (bc *BlockChainMock) SetGenesisHeaderHash(hash []byte) {
	if bc.SetGenesisHeaderHashCalled != nil {
		bc.SetGenesisHeaderHashCalled(hash)
	}
}

// GetCurrentBlockHeader returns current block header pointer
func (bc *BlockChainMock) GetCurrentBlockHeader() data.HeaderHandler {
	if bc.GetCurrentBlockHeaderCalled != nil {
		return bc.GetCurrentBlockHeaderCalled()
	}
	return nil
}

// SetCurrentBlockHeader sets current block header pointer
func (bc *BlockChainMock) SetCurrentBlockHeader(header data.HeaderHandler) error {
	if bc.SetCurrentBlockHeaderCalled != nil {
		return bc.SetCurrentBlockHeaderCalled(header)
	}
	return nil
}

// GetCurrentBlockHeaderHash returns the current block header hash
func (bc *BlockChainMock) GetCurrentBlockHeaderHash() []byte {
	if bc.GetCurrentBlockHeaderHashCalled != nil {
		return bc.GetCurrentBlockHeaderHashCalled()
	}
	return nil
}

// SetCurrentBlockHeaderHash returns the current block header hash
func (bc *BlockChainMock) SetCurrentBlockHeaderHash(hash []byte) {
	if bc.SetCurrentBlockHeaderHashCalled != nil {
		bc.SetCurrentBlockHeaderHashCalled(hash)
	}
}

// SetCurrentBlockHeaderAndHash atomically sets the current block header and its hash
func (bc *BlockChainMock) SetCurrentBlockHeaderAndHash(header data.HeaderHandler, hash []byte) error {
	if bc.SetCurrentBlockHeaderAndHashCalled != nil {
		return bc.SetCurrentBlockHeaderAndHashCalled(header, hash)
	}
	return nil
}

// GetCurrentBlockHeaderAndHash atomically returns the current block header and its hash
func (bc *BlockChainMock) GetCurrentBlockHeaderAndHash() (data.HeaderHandler, []byte) {
	if bc.GetCurrentBlockHeaderAndHashCalled != nil {
		return bc.GetCurrentBlockHeaderAndHashCalled()
	}
	return nil, nil
}

// GetCurrentBlockRootHash returns the current block root hash
func (bc *BlockChainMock) GetCurrentBlockRootHash() []byte {
	if bc.GetCurrentBlockRootHashCalled != nil {
		return bc.GetCurrentBlockRootHashCalled()
	}
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (bc *BlockChainMock) IsInterfaceNil() bool {
	return bc == nil
}

// CreateNewHeader -
func (bc *BlockChainMock) CreateNewHeader() data.HeaderHandler {
	if bc.CreateNewHeaderCalled != nil {
		return bc.CreateNewHeaderCalled()
	}

	return nil
}
