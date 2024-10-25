package blockchain

import (
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
)

var _ data.ChainHandler = (*blockChain)(nil)

var log = logger.GetOrCreate("data/blockchain")

// blockChain holds the block information for the current shard.
//
// The BlockChain also holds pointers to the Genesis block header and the current block
type blockChain struct {
	mut                    sync.RWMutex
	appStatusHandler       core.AppStatusHandler
	genesisHeader          data.HeaderHandler
	genesisHeaderHash      []byte
	currentBlockHeader     data.HeaderHandler
	currentBlockHeaderHash []byte
}

// NewBlockChain returns an initialized blockchain
func NewBlockChain() *blockChain {
	return &blockChain{
		appStatusHandler: statusHandler.NewNilStatusHandler(),
	}
}

// SetGenesisHeader sets the genesis block header pointer
func (bc *blockChain) SetGenesisHeader(genesisBlock data.HeaderHandler) error {
	if check.IfNil(genesisBlock) {
		bc.mut.Lock()
		bc.genesisHeader = nil
		bc.mut.Unlock()

		return nil
	}

	gb, ok := genesisBlock.(*block.Block)
	if !ok {
		return common.ErrInvalidHeaderType
	}
	bc.mut.Lock()
	bc.genesisHeader = gb.Clone()
	bc.mut.Unlock()

	return nil
}

// SetCurrentBlockHeader sets current block header pointer
func (bc *blockChain) SetCurrentBlockHeader(header data.HeaderHandler) error {
	if check.IfNil(header) {
		bc.mut.Lock()
		bc.currentBlockHeader = nil
		bc.mut.Unlock()

		return nil
	}

	h, ok := header.(*block.Block)
	if !ok {
		return common.ErrInvalidHeaderType
	}

	log.Trace("SetCurrentBlockHeader", "nonce", h.Header.Nonce)

	bc.appStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, h.Header.Slot)
	bc.appStatusHandler.SetUInt64Value(core.MetricNonce, h.Header.Nonce)

	bc.mut.Lock()
	bc.currentBlockHeader = h.Clone()
	bc.mut.Unlock()

	return nil
}

// CreateNewHeader creates a new header
func (bc *blockChain) CreateNewHeader() data.HeaderHandler {
	return &block.Block{}
}

// SetAppStatusHandler will set the AppStatusHandler which will be used for monitoring
func (bc *blockChain) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return ErrNilAppStatusHandler
	}

	bc.mut.Lock()
	bc.appStatusHandler = ash
	bc.mut.Unlock()
	return nil
}

// GetGenesisHeader returns the genesis block header pointer
func (bc *blockChain) GetGenesisHeader() data.HeaderHandler {
	bc.mut.RLock()
	defer bc.mut.RUnlock()

	if check.IfNil(bc.genesisHeader) {
		return nil
	}

	return bc.genesisHeader.Clone()
}

// GetGenesisHeaderHash returns the genesis block header hash
func (bc *blockChain) GetGenesisHeaderHash() []byte {
	bc.mut.RLock()
	defer bc.mut.RUnlock()

	return bc.genesisHeaderHash
}

// SetGenesisHeaderHash sets the genesis block header hash
func (bc *blockChain) SetGenesisHeaderHash(hash []byte) {
	bc.mut.Lock()
	bc.genesisHeaderHash = hash
	bc.mut.Unlock()
}

// GetCurrentBlockHeader returns current block header pointer
func (bc *blockChain) GetCurrentBlockHeader() data.HeaderHandler {
	bc.mut.RLock()
	defer bc.mut.RUnlock()

	if check.IfNil(bc.currentBlockHeader) {
		return nil
	}

	return bc.currentBlockHeader.Clone()
}

// GetCurrentBlockHeaderHash returns the current block header hash
func (bc *blockChain) GetCurrentBlockHeaderHash() []byte {
	bc.mut.RLock()
	defer bc.mut.RUnlock()

	return bc.currentBlockHeaderHash
}

// SetCurrentBlockHeaderHash returns the current block header hash
func (bc *blockChain) SetCurrentBlockHeaderHash(hash []byte) {
	bc.mut.Lock()
	bc.currentBlockHeaderHash = hash
	bc.mut.Unlock()
}

// GetCurrentBlockRootHash returns the current committed block root hash. The returned byte slice is a new copy
// of the contained root hash.
func (bc *blockChain) GetCurrentBlockRootHash() []byte {
	header := bc.GetCurrentBlockHeader()
	if header == nil || header.IsInterfaceNil() {
		return nil
	}

	return header.GetTrieRoot()
}

// IsInterfaceNil returns true if there is no value under the interface
func (bc *blockChain) IsInterfaceNil() bool {
	return bc == nil
}
