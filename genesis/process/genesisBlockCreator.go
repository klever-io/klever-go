package process

import (
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("genesis/process")

type genesisBlockCreator struct {
	arg ArgsGenesisBlockCreator
}

// NewGenesisBlockCreator creates a new genesis block creator instance able to create genesis blocks on all initial shards
func NewGenesisBlockCreator(arg ArgsGenesisBlockCreator) (*genesisBlockCreator, error) {
	err := checkArgumentsForBlockCreator(arg)
	if err != nil {
		return nil, fmt.Errorf("%w while creating NewGenesisBlockCreator", err)
	}

	gbc := &genesisBlockCreator{
		arg: arg,
	}

	return gbc, nil
}

func getGenesisBlocksSlotNonceEpoch(arg ArgsGenesisBlockCreator) (uint64, uint64, uint32) {
	return 0, 0, 0
}

func checkArgumentsForBlockCreator(arg ArgsGenesisBlockCreator) error {
	if check.IfNil(arg.Accounts) {
		return common.ErrNilAccountsAdapter
	}
	if check.IfNil(arg.PubkeyConv) {
		return common.ErrNilPubkeyConverter
	}
	if check.IfNil(arg.InitialNodesSetup) {
		return common.ErrNilNodesSetup
	}
	if check.IfNil(arg.Store) {
		return common.ErrNilStore
	}
	if check.IfNil(arg.Blkc) {
		return common.ErrNilBlockChain
	}
	if check.IfNil(arg.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arg.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arg.Uint64ByteSliceConverter) {
		return process.ErrNilUint64Converter
	}
	if check.IfNil(arg.DataPool) {
		return common.ErrNilPoolsHolder
	}
	if check.IfNil(arg.AccountsParser) {
		return genesis.ErrNilAccountsParser
	}
	if check.IfNil(arg.TxLogsProcessor) {
		return process.ErrNilTxLogsProcessor
	}
	if arg.TrieStorageManagers == nil {
		return genesis.ErrNilTrieStorageManager
	}
	if check.IfNil(arg.SignMarshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arg.KAppController) {
		return common.ErrKAppController
	}

	return nil
}

func mustDoGenesisProcess(arg ArgsGenesisBlockCreator) bool {
	genesisEpoch := uint32(0)

	return arg.StartEpochNum == genesisEpoch
}

func (gbc *genesisBlockCreator) createEmptyGenesisBlock() (data.HeaderHandler, error) {
	slot, nonce, epoch := getGenesisBlocksSlotNonceEpoch(gbc.arg)

	emptyGenesisBlocks := &block.Block{
		Header: &block.BlockHeader{
			Slot:      slot,
			Nonce:     nonce,
			Epoch:     epoch,
			Timestamp: gbc.arg.GenesisTime,
		},
	}

	return emptyGenesisBlocks, nil
}

// CreateGenesisBlock will try to create the genesis blocks for all shards
func (gbc *genesisBlockCreator) CreateGenesisBlock() (data.HeaderHandler, error) {
	var err error

	if !mustDoGenesisProcess(gbc.arg) {
		return gbc.createEmptyGenesisBlock()
	}

	log.Debug("createArgsGenesisBlockCreator")

	genesisBlock, err := gbc.createHeader(gbc.arg)
	if err != nil {
		return nil, err
	}

	return genesisBlock, nil
}

func (gbc *genesisBlockCreator) createHeader(
	metaArgsGenesisBlockCreator ArgsGenesisBlockCreator,
) (data.HeaderHandler, error) {
	log.Debug("genesisBlockCreator.createHeaders")
	var err error

	metaArgsGenesisBlockCreator.Blkc = blockchain.NewBlockChain()
	genesisBlock, err := createGenesisBlock(
		metaArgsGenesisBlockCreator,
	)
	if err != nil {
		return nil, fmt.Errorf("'%w' while generating genesis block", err)
	}

	err = gbc.saveGenesisBlock(genesisBlock)
	if err != nil {
		return nil, fmt.Errorf("'%w' while saving genesis block", err)
	}

	log.Info("genesisBlockCreator.createHeaders",
		"nonce", genesisBlock.GetNonce(),
		"slot", genesisBlock.GetSlot(),
		"trie root hash", genesisBlock.GetTrieRoot(),
	)

	return genesisBlock, nil
}

func (gbc *genesisBlockCreator) saveGenesisBlock(blck data.HeaderHandler) error {
	blockBuff, err := gbc.arg.Marshalizer.Marshal(blck)
	if err != nil {
		return err
	}

	blockObj, ok := blck.(*block.Block)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	blockHeaderBuff, err := gbc.arg.Marshalizer.Marshal(blockObj.Header)
	if err != nil {
		return err
	}

	hash := gbc.arg.Hasher.Compute(string(blockHeaderBuff))
	unitType := retriever.BlockUnit

	return gbc.arg.Store.Put(unitType, hash, blockBuff)
}
