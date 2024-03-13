package bootstrap

import (
	"encoding/json"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// baseStorageHandler handles the storage functions for saving bootstrap data
type baseStorageHandler struct {
	storageService  retriever.StorageService
	marshalizer     marshal.Marshalizer
	hasher          hashing.Hasher
	currentEpoch    uint32
	uint64Converter typeConverters.Uint64ByteSliceConverter
}

func (bsh *baseStorageHandler) saveNodesCoordinatorRegistry(
	metaBlock *block.Block,
	nodesConfig *sharding.NodesCoordinatorRegistry,
) ([]byte, error) {
	key := append([]byte(core.NodesCoordinatorRegistryKeyPrefix), metaBlock.GetPrevRandSeed()...)

	registryBytes, err := json.Marshal(nodesConfig)
	if err != nil {
		return nil, err
	}

	bootstrapUnit := bsh.storageService.GetStorer(retriever.BootstrapUnit)
	err = bootstrapUnit.Put(key, registryBytes)
	if err != nil {
		return nil, err
	}

	log.Debug("saving nodes coordinator config", "key", key)

	return metaBlock.GetPrevRandSeed(), nil
}

func (bsh *baseStorageHandler) commitTries(components *ComponentsNeededForBootstrap) error {
	for _, trie := range components.UserAccountTries {
		err := trie.Commit()
		if err != nil {
			return err
		}
	}

	for _, trie := range components.PeerAccountTries {
		err := trie.Commit()
		if err != nil {
			return err
		}
	}

	for _, trie := range components.KAppAccountTries {
		err := trie.Commit()
		if err != nil {
			return err
		}
	}

	return nil
}

func (bsh *baseStorageHandler) saveHdrToStorage(metaBlock *block.Block) ([]byte, error) {
	dataBytes, err := bsh.marshalizer.Marshal(metaBlock)
	if err != nil {
		return nil, err
	}

	headerBytes, err := bsh.marshalizer.Marshal(metaBlock.Header)
	if err != nil {
		return nil, err
	}

	headerHash := bsh.hasher.Compute(string(headerBytes))

	metaHdrStorage := bsh.storageService.GetStorer(retriever.BlockUnit)
	err = metaHdrStorage.Put(headerHash, dataBytes)
	if err != nil {
		return nil, err
	}

	nonceToByteSlice := bsh.uint64Converter.ToByteSlice(metaBlock.GetNonce())
	metaHdrNonceStorage := bsh.storageService.GetStorer(retriever.HdrNonceHashDataUnit)
	err = metaHdrNonceStorage.Put(nonceToByteSlice, headerHash)
	if err != nil {
		return nil, err
	}

	return headerHash, nil
}

func (bsh *baseStorageHandler) saveHdrForEpochTrigger(metaBlock *block.Block) error {
	lastHeaderBytes, err := bsh.marshalizer.Marshal(metaBlock)
	if err != nil {
		return err
	}

	epochStartIdentifier := core.EpochStartIdentifier(metaBlock.Header.Epoch)
	metaHdrStorage := bsh.storageService.GetStorer(retriever.BlockUnit)
	err = metaHdrStorage.Put([]byte(epochStartIdentifier), lastHeaderBytes)
	if err != nil {
		return err
	}

	triggerStorage := bsh.storageService.GetStorer(retriever.BootstrapUnit)
	err = triggerStorage.Put([]byte(epochStartIdentifier), lastHeaderBytes)
	if err != nil {
		return err
	}

	return nil
}
