package storageBootstrap

import (
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

var log = logger.GetOrCreate("process/sync")

// ArgsBaseStorageBootstrapper is structure used to create a new storage bootstrapper
type ArgsBaseStorageBootstrapper struct {
	BootStorer         process.BootStorer
	ForkDetector       process.ForkDetector
	BlockProcessor     process.BlockProcessor
	ChainHandler       data.ChainHandler
	Marshalizer        marshal.Marshalizer
	Store              retriever.StorageService
	Uint64Converter    typeConverters.Uint64ByteSliceConverter
	BootstrapSlotIndex uint64
	NodesCoordinator   sharding.NodesCoordinator
	EpochStartTrigger  process.EpochStartTriggerHandler
	//BlockTracker       process.BlockTracker
	ChainID string
}

// ArgsShardStorageBootstrapper is structure used to create a new storage bootstrapper for shard
type ArgsShardStorageBootstrapper struct {
	ArgsBaseStorageBootstrapper
}

// ArgsMetaStorageBootstrapper is structure used to create a new storage bootstrapper for metachain
type ArgsMetaStorageBootstrapper struct {
	ArgsBaseStorageBootstrapper
}

type storageBootstrapper struct {
	bootStorer        process.BootStorer
	forkDetector      process.ForkDetector
	blkExecutor       process.BlockProcessor
	blkc              data.ChainHandler
	marshalizer       marshal.Marshalizer
	store             retriever.StorageService
	uint64Converter   typeConverters.Uint64ByteSliceConverter
	nodesCoordinator  sharding.NodesCoordinator
	epochStartTrigger process.EpochStartTriggerHandler
	// blockTracker      process.BlockTracker

	bootstrapSlotIndex   uint64
	bootstrapper         storageBootstrapperHandler
	headerNonceHashStore storage.Storer
	highestNonce         uint64
	chainID              string
}

func (st *storageBootstrapper) loadBlocks() error {
	var err error
	var headerInfo *bootstrapStorage.BootstrapData

	minSlot := uint64(0)
	if !check.IfNil(st.blkc.GetGenesisHeader()) {
		minSlot = st.blkc.GetGenesisHeader().GetSlot()
	}

	slot := st.bootStorer.GetHighestSlot()
	if slot <= int64(minSlot) {
		log.Debug("Load blocks does nothing as start from genesis")
		err = st.bootStorer.SaveLastSlot(0)
		log.LogIfError(
			err,
			"function", "storageBootstrapper.loadBlocks",
			"operation", "SaveLastSlot",
		)

		return process.ErrNotEnoughValidBlocksInStorage
	}
	storageHeadersInfo := make([]*bootstrapStorage.BootstrapData, 0)

	log.Debug("Load blocks started...")

	for {
		headerInfo, err = st.bootStorer.Get(slot)
		if err != nil {
			break
		}

		if slot == headerInfo.LastSlot {
			err = sync.ErrCorruptBootstrapFromStorageDb
			break
		}

		storageHeadersInfo = append(storageHeadersInfo, headerInfo.Clone())

		if uint64(slot) > st.bootstrapSlotIndex {
			slot = headerInfo.LastSlot
			continue
		}

		err = st.applyHeaderInfo(headerInfo)
		if err != nil {
			slot = headerInfo.LastSlot
			continue
		}

		bootInfos, err := st.getBootInfos(headerInfo)
		if err != nil {
			slot = headerInfo.LastSlot
			continue
		}

		err = st.applyBootInfos(bootInfos)
		if err != nil {
			slot = headerInfo.LastSlot
			continue
		}

		break
	}

	if err != nil {
		log.Warn("bootstrapper", "error", err)
		st.restoreBlockChainToGenesis()
		err = st.bootStorer.SaveLastSlot(0)
		log.LogIfError(
			err,
			"function", "storageBootstrapper.loadBlocks",
			"operation", "SaveLastSlot after restoreBlockChainToGenesis",
		)

		return process.ErrNotEnoughValidBlocksInStorage
	}

	log.Debug("storageBootstrapper.loadBlocks",
		"LastHeader", st.displayBoostrapHeaderInfo(headerInfo.LastHeader),
		"HighestFinalBlockNonce", headerInfo.HighestFinalBlockNonce,
		"NodesCoordinatorConfigKey", headerInfo.NodesCoordinatorConfigKey,
		"EpochStartTriggerConfigKey", headerInfo.EpochStartTriggerConfigKey,
	)

	st.cleanupStorageForHigherNonceIfExist()

	for i := 0; i < len(storageHeadersInfo)-1; i++ {
		st.cleanupStorage(storageHeadersInfo[i].LastHeader.Clone())
		st.bootstrapper.cleanupNotarizedStorage(storageHeadersInfo[i].LastHeader.Hash)
	}

	err = st.bootStorer.SaveLastSlot(slot)
	if err != nil {
		log.Debug("cannot save last slot in storage ", "error", err.Error())
	}

	st.highestNonce = headerInfo.LastHeader.Nonce

	return nil
}

func (st *storageBootstrapper) displayBoostrapHeaderInfo(hinfo *bootstrapStorage.BootstrapHeaderInfo) string {
	return fmt.Sprintf("nonce %d, epoch %d, hash %s",
		hinfo.Nonce, hinfo.Epoch, logger.DisplayByteSlice(hinfo.Hash))
}

func (st *storageBootstrapper) cleanupStorageForHigherNonceIfExist() {
	slot := st.bootStorer.GetHighestSlot()
	bootstrapData, err := st.bootStorer.Get(slot)
	if err != nil {
		log.Debug("cleanupStorageForHigherNonceIfExist.Get",
			"slot", slot,
			"error", err.Error())
		return
	}

	highestBlockNonce := bootstrapData.LastHeader.GetNonce()
	header, hash, err := st.bootstrapper.getHeaderWithNonce(highestBlockNonce + 1)
	if err != nil {
		log.Trace("cleanupStorageForHigherNonceIfExist.getHeaderWithNonce",
			"nonce", highestBlockNonce+1,
			"error", err)
		return
	}

	headerInfo := &bootstrapStorage.BootstrapHeaderInfo{
		Epoch: header.GetEpoch(),
		Nonce: header.GetNonce(),
		Hash:  hash,
	}

	st.cleanupStorage(headerInfo)
	st.bootstrapper.cleanupNotarizedStorage(headerInfo.Hash)
}

// GetHighestBlockNonce will return nonce of last block loaded from storage
func (st *storageBootstrapper) GetHighestBlockNonce() uint64 {
	return st.highestNonce
}

func (st *storageBootstrapper) applyHeaderInfo(hdrInfo *bootstrapStorage.BootstrapData) error {
	headerHash := hdrInfo.LastHeader.Hash
	log.Debug("storageBootstrapper.applyHeaderInfo", "headerHash", headerHash)
	headerFromStorage, err := st.bootstrapper.getHeader(headerHash)
	if err != nil {
		log.Debug("cannot get header ", "nonce", hdrInfo.LastHeader.Nonce, "error", err.Error())
		return err
	}

	if string(headerFromStorage.GetChainID()) != st.chainID {
		log.Debug("chain ID missmatch for header with nonce", "nonce", headerFromStorage.GetNonce(),
			"reference", []byte(st.chainID),
			"fromStorage", headerFromStorage.GetChainID())
		return process.ErrInvalidChainID
	}

	err = st.blkExecutor.RevertStateToBlock(headerFromStorage)
	if err != nil {
		log.Debug("cannot recreate trie for header with nonce", "nonce", headerFromStorage.GetNonce())
		return err
	}

	err = st.applyBlock(headerFromStorage, headerHash)
	if err != nil {
		log.Debug("cannot apply block for header ", "nonce", headerFromStorage.GetNonce(), "error", err.Error())
		return err
	}

	return nil
}

func (st *storageBootstrapper) getBootInfos(hdrInfo *bootstrapStorage.BootstrapData) ([]*bootstrapStorage.BootstrapData, error) {
	highestFinalBlockNonce := hdrInfo.HighestFinalBlockNonce
	highestBlockNonce := hdrInfo.LastHeader.Nonce

	lastSlot := hdrInfo.LastSlot
	bootInfos := []*bootstrapStorage.BootstrapData{hdrInfo}

	log.Debug("block info from storage",
		"highest block nonce", highestBlockNonce,
		"highest final block nonce", highestFinalBlockNonce,
		"last slot", lastSlot)

	if highestFinalBlockNonce == highestBlockNonce {
		return bootInfos, nil
	}

	lowestNonce := uint64(tools.MaxInt64(int64(highestFinalBlockNonce)-1, 1))
	for highestBlockNonce > lowestNonce {
		strHdrI, err := st.bootStorer.Get(lastSlot)
		if err != nil {
			log.Debug("cannot load header info from storage ", "error", err.Error())
			return nil, err
		}

		bootInfos = append(bootInfos, strHdrI)
		highestBlockNonce = strHdrI.LastHeader.Nonce

		lastSlot = strHdrI.LastSlot
		if lastSlot == 0 {
			break
		}
	}

	return bootInfos, nil
}

func (st *storageBootstrapper) applyBootInfos(bootInfos []*bootstrapStorage.BootstrapData) error {
	var err error

	defer func() {
		if err != nil {
			//st.blockTracker.RestoreToGenesis()
			st.forkDetector.RestoreToGenesis()
		}
	}()

	for i := len(bootInfos) - 1; i >= 0; i-- {
		log.Debug("apply header",
			"epoch", bootInfos[i].LastHeader.Epoch,
			"nonce", bootInfos[i].LastHeader.Nonce)

		var header data.HeaderHandler
		header, err = st.bootstrapper.getHeader(bootInfos[i].LastHeader.Hash)
		if err != nil {
			log.Debug("cannot get header", "hash", bootInfos[i].LastHeader.Hash, "error", err.Error())
			return err
		}

		log.Debug("add header to fork detector",
			"slot", header.GetSlot(),
			"nonce", header.GetNonce(),
			"hash", bootInfos[i].LastHeader.Hash)

		err = st.forkDetector.AddHeader(header, bootInfos[i].LastHeader.Hash, process.BHProcessed, nil, nil)
		if err != nil {
			log.Warn("cannot add header to fork detector", "error", err.Error())
		}

		if i > 0 {
			log.Debug("added self notarized header in block tracker",
				"slot", header.GetSlot(),
				"nonce", header.GetNonce(),
				"hash", bootInfos[i].LastHeader.Hash)

			//st.blockTracker.AddSelfNotarizedHeader(header, bootInfos[i].LastHeader.Hash)
		}

		//st.blockTracker.AddTrackedHeader(header, bootInfos[i].LastHeader.Hash)
	}

	if len(bootInfos) == 1 {
		st.forkDetector.SetFinalToLastCheckpoint()
	}

	err = st.nodesCoordinator.LoadState(bootInfos[0].NodesCoordinatorConfigKey)
	if err != nil {
		log.Debug("cannot load nodes coordinator state", "error", err.Error())
		return err
	}

	err = st.epochStartTrigger.LoadState(bootInfos[0].EpochStartTriggerConfigKey)
	if err != nil {
		log.Debug("cannot load epoch start trigger state", "error", err.Error())
		return err
	}

	return nil
}

func (st *storageBootstrapper) cleanupStorage(headerInfo *bootstrapStorage.BootstrapHeaderInfo) {
	log.Debug("cleanup storage")

	nonceToByteSlice := st.uint64Converter.ToByteSlice(headerInfo.Nonce)
	err := st.headerNonceHashStore.Remove(nonceToByteSlice)
	if err != nil {
		log.Debug("block was not removed from storage",
			"nonce", headerInfo.Nonce,
			"hash", headerInfo.Hash,
			"error", err.Error())
		return
	}

	log.Debug("block was removed from storage",
		"nonce", headerInfo.Nonce,
		"hash", headerInfo.Hash)
}

func (st *storageBootstrapper) applyBlock(header data.HeaderHandler, headerHash []byte) error {
	err := st.blkc.SetCurrentBlockHeader(header)
	if err != nil {
		return err
	}

	st.blkc.SetCurrentBlockHeaderHash(headerHash)

	return nil
}

func (st *storageBootstrapper) restoreBlockChainToGenesis() {
	genesisHeader := st.blkc.GetGenesisHeader()
	err := st.blkExecutor.RevertStateToBlock(genesisHeader)
	if err != nil {
		log.Debug("cannot recreate trie for genesis header with nonce", "nonce", genesisHeader.GetNonce())
	}

	err = st.blkc.SetCurrentBlockHeader(nil)
	if err != nil {
		log.Debug("cannot set current block header", "error", err.Error())
	}

	st.blkc.SetCurrentBlockHeaderHash(nil)
}

func checkBaseStorageBootstrapperArguments(args ArgsBaseStorageBootstrapper) error {
	if check.IfNil(args.BootStorer) {
		return common.ErrNilBootStorer
	}
	if check.IfNil(args.ForkDetector) {
		return common.ErrNilForkDetector
	}
	if check.IfNil(args.BlockProcessor) {
		return common.ErrNilBlockProcessor
	}
	if check.IfNil(args.ChainHandler) {
		return common.ErrNilBlockChain
	}
	if check.IfNil(args.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(args.Store) {
		return common.ErrNilStore
	}
	if check.IfNil(args.Uint64Converter) {
		return process.ErrNilUint64Converter
	}
	if check.IfNil(args.NodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(args.EpochStartTrigger) {
		return common.ErrNilEpochStartTrigger
	}

	return nil
}
