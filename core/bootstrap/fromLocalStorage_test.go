package bootstrap

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareEpochFromStorage(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, err := NewEpochStartBootstrap(args)
	require.Nil(t, err)
	epochStartProvider.initializeFromLocalStorage()

	epochStartProvider.baseData.lastEpoch = 10
	_, err = epochStartProvider.prepareEpochFromStorage()
	assert.Nil(t, err)
}

func TestGetEpochStartMetaFromStorage(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.initializeFromLocalStorage()

	meta := &block.Block{Header: &block.BlockHeader{Nonce: 1}}
	metaBytes, _ := json.Marshal(meta)
	storer := &mock.StorerStub{
		GetCalled: func(key []byte) (bytes []byte, err error) {
			return metaBytes, nil
		},
		SearchFirstCalled: func(key []byte) ([]byte, error) {
			return metaBytes, nil
		},
	}
	metaBlock, err := epochStartProvider.getEpochStartMetaFromStorage(storer)
	assert.Nil(t, err)
	assert.Equal(t, meta, metaBlock)
}

func TestGetLastBootstrapData(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.initializeFromLocalStorage()

	slot := int64(10)

	slotNum := bootstrapStorage.SlotNum{
		Num: slot,
	}
	slotBytes, _ := json.Marshal(&slotNum)
	nodesCoordinatorConfigKey := []byte("key")

	nodesConfigRegistry := sharding.NodesCoordinatorRegistry{
		CurrentEpoch: 10,
	}
	bootstrapData := bootstrapStorage.BootstrapData{
		NodesCoordinatorConfigKey: nodesCoordinatorConfigKey,
	}

	storer := &mock.StorerStub{
		GetCalled: func(key []byte) (b []byte, err error) {
			switch {
			case bytes.Equal([]byte(core.HighestSlotFromBootStorage), key):
				return slotBytes, nil
			case bytes.Equal([]byte(strconv.FormatInt(slot, 10)), key):

				bootstrapDataBytes, _ := json.Marshal(&bootstrapData)
				return bootstrapDataBytes, nil
			default:
				return nil, nil
			}
		},
		SearchFirstCalled: func(key []byte) ([]byte, error) {
			nodesConfigRegistryBytes, _ := json.Marshal(nodesConfigRegistry)
			return nodesConfigRegistryBytes, nil
		},
	}

	bootData, nodesRegistry, err := epochStartProvider.getLastBootstrapData(storer)
	assert.Nil(t, err)
	assert.Equal(t, bootstrapData.Clone(), bootData.Clone())
	assert.Equal(t, &nodesConfigRegistry, nodesRegistry)
}
