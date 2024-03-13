package bootstrap

import (
	"encoding/json"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
)

func (e *epochStartBootstrap) initializeFromLocalStorage() {
	latestData, errNotCritical := e.latestStorageDataProvider.Get()
	if errNotCritical != nil {
		e.baseData.storageExists = false
		log.Debug("no epoch db found in storage", "error", errNotCritical.Error())
		return
	}

	if latestData.Epoch < e.startEpoch {
		e.baseData.storageExists = false
		log.Warn("data is older then start epoch", "latestEpoch", latestData.Epoch, "startEpoch", e.startEpoch)
		return
	}

	e.baseData.storageExists = true
	e.baseData.lastEpoch = latestData.Epoch
	e.baseData.lastSlot = latestData.LastSlot
	e.baseData.epochStartSlot = latestData.EpochStartSlot
	log.Debug("got last data from storage",
		"epoch", e.baseData.lastEpoch,
		"last slot", e.baseData.lastSlot,
		"epoch start Slot", e.baseData.epochStartSlot)
}

func (e *epochStartBootstrap) prepareEpochFromStorage() (Parameters, error) {
	err := e.createTriesComponents()
	if err != nil {
		return Parameters{}, err
	}

	parameters := Parameters{
		Epoch:       e.baseData.lastEpoch,
		NodesConfig: e.nodesConfig,
	}
	return parameters, nil
}

func (e *epochStartBootstrap) getLastBootstrapData(storer storage.Storer) (*bootstrapStorage.BootstrapData, *sharding.NodesCoordinatorRegistry, error) {
	bootStorer, err := bootstrapStorage.NewBootstrapStorer(e.marshalizer, storer)
	if err != nil {
		return nil, nil, err
	}

	highestSlot := bootStorer.GetHighestSlot()
	bootstrapData, err := bootStorer.Get(highestSlot)
	if err != nil {
		return nil, nil, err
	}

	ncInternalkey := append([]byte(core.NodesCoordinatorRegistryKeyPrefix), bootstrapData.NodesCoordinatorConfigKey...)
	data, err := storer.SearchFirst(ncInternalkey)
	if err != nil {
		log.Debug("getLastBootstrapData", "key", ncInternalkey, "error", err)
		return nil, nil, err
	}

	config := &sharding.NodesCoordinatorRegistry{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, nil, err
	}

	return bootstrapData, config, nil
}

func (e *epochStartBootstrap) getEpochStartMetaFromStorage(storer storage.Storer) (*block.Block, error) {
	epochIdentifier := core.EpochStartIdentifier(e.baseData.lastEpoch)
	data, err := storer.SearchFirst([]byte(epochIdentifier))
	if err != nil {
		log.Debug("getEpochStartMetaFromStorage", "key", epochIdentifier, "error", err)
		return nil, err
	}

	metaBlock := &block.Block{}
	err = e.marshalizer.Unmarshal(metaBlock, data)
	if err != nil {
		return nil, err
	}

	return metaBlock, nil
}
