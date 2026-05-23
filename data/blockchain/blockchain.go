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

// prepareCurrentBlockHeader validates the header, updates status metrics, and returns
// a clone ready to be stored. Returns (nil, nil) when header is the nil interface.
// Callers must assign the returned value under the appropriate mutex.
func (bc *blockChain) prepareCurrentBlockHeader(header data.HeaderHandler) (data.HeaderHandler, error) {
	if check.IfNil(header) {
		return nil, nil
	}

	h, ok := header.(*block.Block)
	if !ok {
		return nil, common.ErrInvalidHeaderType
	}

	log.Trace("setCurrentBlockHeader", "nonce", h.Header.Nonce)

	bc.appStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, h.Header.Slot)
	bc.appStatusHandler.SetUInt64Value(core.MetricNonce, h.Header.Nonce)

	return h.Clone(), nil
}

// SetCurrentBlockHeader sets current block header pointer
func (bc *blockChain) SetCurrentBlockHeader(header data.HeaderHandler) error {
	clone, err := bc.prepareCurrentBlockHeader(header)
	if err != nil {
		return err
	}

	bc.mut.Lock()
	bc.currentBlockHeader = clone
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

// GetCurrentBlockHeaderAndHash atomically returns the current block header and
// its hash under a single read lock acquisition. Use this in preference to
// calling GetCurrentBlockHeader and GetCurrentBlockHeaderHash separately when
// the (header, hash) pair must be consistent — those two calls take separate
// RLocks and can observe an update interleaved between them.
//
// The returned hash is a defensive copy so callers cannot mutate the
// backing array and corrupt the atomic snapshot.
func (bc *blockChain) GetCurrentBlockHeaderAndHash() (data.HeaderHandler, []byte) {
	bc.mut.RLock()
	defer bc.mut.RUnlock()

	hashCopy := append([]byte(nil), bc.currentBlockHeaderHash...)
	if check.IfNil(bc.currentBlockHeader) {
		return nil, hashCopy
	}

	return bc.currentBlockHeader.Clone(), hashCopy
}

// SetCurrentBlockHeaderHash returns the current block header hash
func (bc *blockChain) SetCurrentBlockHeaderHash(hash []byte) {
	bc.mut.Lock()
	bc.currentBlockHeaderHash = hash
	bc.mut.Unlock()
}

// SetCurrentBlockHeaderAndHash atomically sets the current block header and its hash
// under a single mutex acquisition, preventing concurrent readers from observing a
// mismatched (header, hash) pair between the two updates.
//
// The hash bytes are defensively copied so subsequent caller-side mutation of the
// input slice cannot corrupt the stored snapshot.
func (bc *blockChain) SetCurrentBlockHeaderAndHash(header data.HeaderHandler, hash []byte) error {
	clone, err := bc.prepareCurrentBlockHeader(header)
	if err != nil {
		return err
	}

	hashCopy := append([]byte(nil), hash...)

	bc.mut.Lock()
	bc.currentBlockHeader = clone
	bc.currentBlockHeaderHash = hashCopy
	bc.mut.Unlock()

	return nil
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
