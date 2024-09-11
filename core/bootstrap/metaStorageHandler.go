package bootstrap

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

type metaStorageHandler struct {
	*baseStorageHandler
}

// NewMetaStorageHandler will return a new instance of metaStorageHandler
func NewMetaStorageHandler(
	generalConfig config.Config,
	pathManagerHandler storage.PathManagerHandler,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	currentEpoch uint32,
	uint64Converter typeConverters.Uint64ByteSliceConverter,
) (*metaStorageHandler, error) {
	epochStartNotifier := &disabled.EpochStartNotifier{}
	storageFactory, err := factory.NewStorageServiceFactory(
		&generalConfig,
		pathManagerHandler,
		epochStartNotifier,
		currentEpoch,
	)
	if err != nil {
		return nil, err
	}

	storageService, err := storageFactory.CreateForMeta()
	if err != nil {
		return nil, err
	}

	base := &baseStorageHandler{
		storageService:  storageService,
		marshalizer:     marshalizer,
		hasher:          hasher,
		currentEpoch:    currentEpoch,
		uint64Converter: uint64Converter,
	}

	return &metaStorageHandler{baseStorageHandler: base}, nil
}

// CloseStorageService closes the containing storage service
func (msh *metaStorageHandler) CloseStorageService() {
	err := msh.storageService.CloseAll()
	if err != nil {
		log.Warn("error while closing storers", "error", err)
	}
}

// SaveDataToStorage will save the fetched data to storage so it will be used by the storage bootstrap component
func (msh *metaStorageHandler) SaveDataToStorage(components *ComponentsNeededForBootstrap) error {
	defer func() {
		err := msh.storageService.CloseAll()
		if err != nil {
			log.Debug("error while closing storers", "error", err)
		}
	}()

	bootStorer := msh.storageService.GetStorer(retriever.BootstrapUnit)

	lastHeader, err := msh.saveLastHeader(components.EpochStartBlock)
	if err != nil {
		return err
	}

	err = msh.saveHdrForEpochTrigger(components.EpochStartBlock)
	if err != nil {
		return err
	}

	err = msh.saveHdrForEpochTrigger(components.PreviousEpochStart)
	if err != nil {
		return err
	}

	_, err = msh.saveHdrToStorage(components.PreviousEpochStart)
	if err != nil {
		return err
	}

	triggerConfigKey, err := msh.saveTriggerRegistry(components)
	if err != nil {
		return err
	}

	nodesCoordinatorConfigKey, err := msh.saveNodesCoordinatorRegistry(components.EpochStartBlock, components.NodesConfig)
	if err != nil {
		return err
	}

	bootStrapData := bootstrapStorage.BootstrapData{
		LastHeader:                 lastHeader,
		NodesCoordinatorConfigKey:  nodesCoordinatorConfigKey,
		EpochStartTriggerConfigKey: triggerConfigKey,
		HighestFinalBlockNonce:     lastHeader.Nonce,
		LastSlot:                   0,
	}
	bootStrapDataBytes, err := msh.marshalizer.Marshal(&bootStrapData)
	if err != nil {
		return err
	}

	slotToUseAsKey := tools.SafeU64ToI64(components.EpochStartBlock.Header.Slot)
	slotNum := bootstrapStorage.SlotNum{Num: slotToUseAsKey}
	slotNumBytes, err := msh.marshalizer.Marshal(&slotNum)
	if err != nil {
		return err
	}

	err = bootStorer.Put([]byte(core.HighestSlotFromBootStorage), slotNumBytes)
	if err != nil {
		return err
	}
	key := []byte(strconv.FormatInt(slotToUseAsKey, 10))
	err = bootStorer.Put(key, bootStrapDataBytes)
	if err != nil {
		return err
	}

	//TODO: Check if it's needed to commit: https://github.com/ElrondNetwork/elrond-go/commit/2230d7e20f9a76a0a0c5fe2aa5b719fb246a7ce5
	err = msh.commitTries(components)
	if err != nil {
		return err
	}

	log.Debug("saved bootstrap data to storage", "slot", slotToUseAsKey)
	return nil
}

func (msh *metaStorageHandler) saveLastHeader(metaBlock *block.Block) (*bootstrapStorage.BootstrapHeaderInfo, error) {
	lastHeaderHash, err := msh.saveHdrToStorage(metaBlock)
	if err != nil {
		return nil, err
	}

	bootstrapHdrInfo := &bootstrapStorage.BootstrapHeaderInfo{
		Epoch: metaBlock.Header.Epoch,
		Nonce: metaBlock.Header.Nonce,
		Hash:  lastHeaderHash,
	}

	return bootstrapHdrInfo, nil
}

func (msh *metaStorageHandler) saveTriggerRegistry(components *ComponentsNeededForBootstrap) ([]byte, error) {
	metaBlock := components.EpochStartBlock
	hash, err := tools.CalculateHash(msh.marshalizer, msh.hasher, metaBlock.Header)
	if err != nil {
		return nil, err
	}

	triggerReg := epochStart.TriggerRegistry{
		Epoch:                      metaBlock.Header.Epoch,
		CurrentSlot:                metaBlock.Header.Slot,
		EpochFinalityAttestingSlot: metaBlock.Header.Slot,
		CurrEpochStartSlot:         metaBlock.Header.Slot,
		PrevEpochStartSlot:         components.PreviousEpochStart.GetSlot(),
		EpochStartHash:             hash,
		EpochStartBlock:            metaBlock,
	}

	bootstrapKey := []byte(fmt.Sprint(metaBlock.Header.Slot))
	trigInternalKey := append([]byte(core.TriggerRegistryKeyPrefix), bootstrapKey...)

	triggerRegBytes, err := json.Marshal(&triggerReg)
	if err != nil {
		return nil, err
	}

	bootstrapStorageUnit := msh.storageService.GetStorer(retriever.BootstrapUnit)
	errPut := bootstrapStorageUnit.Put(trigInternalKey, triggerRegBytes)
	if errPut != nil {
		return nil, errPut
	}

	return bootstrapKey, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (msh *metaStorageHandler) IsInterfaceNil() bool {
	return msh == nil
}
