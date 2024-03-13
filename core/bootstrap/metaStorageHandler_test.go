package bootstrap

import (
	"os"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewMetaStorageHandler_InvalidConfigErr(t *testing.T) {
	gCfg := config.Config{}
	pathManager := &mock.PathManagerStub{}
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	uit64Cvt := &mock.Uint64ByteSliceConverterMock{}

	mtStrHandler, err := NewMetaStorageHandler(gCfg, pathManager, marshalizer, hasher, 1, uit64Cvt)
	assert.True(t, check.IfNil(mtStrHandler))
	assert.NotNil(t, err)
}

func TestNewMetaStorageHandler_CreateForMetaErr(t *testing.T) {
	defer func() {
		_ = os.RemoveAll("./Epoch_0")
	}()

	gCfg := mock.GetGeneralConfig()
	pathManager := &mock.PathManagerStub{}
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	uit64Cvt := &mock.Uint64ByteSliceConverterMock{}

	mtStrHandler, err := NewMetaStorageHandler(gCfg, pathManager, marshalizer, hasher, 1, uit64Cvt)
	assert.False(t, check.IfNil(mtStrHandler))
	assert.Nil(t, err)
}

func TestMetaStorageHandler_saveLastHeader(t *testing.T) {
	defer func() {
		_ = os.RemoveAll("./Epoch_0")
	}()

	gCfg := mock.GetGeneralConfig()
	pathManager := &mock.PathManagerStub{}
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	uit64Cvt := &mock.Uint64ByteSliceConverterMock{}

	mtStrHandler, _ := NewMetaStorageHandler(gCfg, pathManager, marshalizer, hasher, 1, uit64Cvt)

	header := &block.Block{Header: &block.BlockHeader{Nonce: 0}}

	headerHash, _ := tools.CalculateHash(marshalizer, hasher, header.Header)
	expectedBootInfo := bootstrapStorage.BootstrapHeaderInfo{
		Nonce: 0,
		Epoch: 0,
		Hash:  headerHash,
	}

	bootHeaderInfo, err := mtStrHandler.saveLastHeader(header)
	assert.Nil(t, err)
	assert.Equal(t, expectedBootInfo.Clone(), bootHeaderInfo.Clone())
}

func TestMetaStorageHandler_saveTriggerRegistry(t *testing.T) {
	defer func() {
		_ = os.RemoveAll("./Epoch_0")
	}()

	gCfg := mock.GetGeneralConfig()
	pathManager := &mock.PathManagerStub{}
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	uit64Cvt := &mock.Uint64ByteSliceConverterMock{}

	mtStrHandler, _ := NewMetaStorageHandler(gCfg, pathManager, marshalizer, hasher, 1, uit64Cvt)

	components := &ComponentsNeededForBootstrap{
		EpochStartBlock:    &block.Block{Header: &block.BlockHeader{Nonce: 3}},
		PreviousEpochStart: &block.Block{Header: &block.BlockHeader{Nonce: 2}},
	}

	_, err := mtStrHandler.saveTriggerRegistry(components)
	assert.Nil(t, err)
}

func TestMetaStorageHandler_saveDataToStorage(t *testing.T) {
	defer func() {
		_ = os.RemoveAll("./Epoch_0")
	}()

	gCfg := mock.GetGeneralConfig()
	pathManager := &mock.PathManagerStub{}
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	uit64Cvt := &mock.Uint64ByteSliceConverterMock{}

	mtStrHandler, _ := NewMetaStorageHandler(gCfg, pathManager, marshalizer, hasher, 1, uit64Cvt)

	components := &ComponentsNeededForBootstrap{
		EpochStartBlock:    &block.Block{Header: &block.BlockHeader{Nonce: 3}},
		PreviousEpochStart: &block.Block{Header: &block.BlockHeader{Nonce: 2}},
	}

	err := mtStrHandler.SaveDataToStorage(components)
	assert.Nil(t, err)
}
