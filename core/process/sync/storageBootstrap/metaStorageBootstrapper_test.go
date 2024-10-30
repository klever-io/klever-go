package storageBootstrap

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/stretchr/testify/assert"
)

func validArguments() ArgsMetaStorageBootstrapper {
	_, bootStorer := bootStorerMock()

	return ArgsMetaStorageBootstrapper{
		ArgsBaseStorageBootstrapper: ArgsBaseStorageBootstrapper{
			BootStorer:         bootStorer,
			ForkDetector:       &mock.ForkDetectorMock{},
			BlockProcessor:     &consensusMock.BlockProcessorMock{},
			ChainHandler:       &mock.BlockChainMock{},
			Marshalizer:        &mock.MarshalizerMock{},
			Store:              &mock.ChainStorerMock{},
			Uint64Converter:    &mock.Uint64ByteSliceConverterMock{},
			BootstrapSlotIndex: 0,
			NodesCoordinator:   &mock.NodesCoordinatorMock{},
			EpochStartTrigger:  &mock.EpochStartTriggerStub{},
			ChainID:            "test-chain",
		},
	}
}

func TestNewMetaStorageBootstrapper_NewBootstrapper(t *testing.T) {
	t.Parallel()

	args := validArguments()

	// Create a new MetaStorageBootstrapper
	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Nil(t, err)
	assert.NotNil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilBootStorer(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.BootStorer = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilBootStorer, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilForkDetector(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.ForkDetector = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilForkDetector, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilBlockProcessor(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.BlockProcessor = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilBlockProcessor, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilChainHandler(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.ChainHandler = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilBlockChain, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilMarshalizer(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.Marshalizer = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilMarshalizer, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilStore(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.Store = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilStore, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilUint64Converter(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.Uint64Converter = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, process.ErrNilUint64Converter, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilNodesCoordinator(t *testing.T) {

	args := validArguments()
	args.NodesCoordinator = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
	assert.Nil(t, bootstrapper)
}

func TestNewMetaStorageBootstrapper_NilEpochStartTrigger(t *testing.T) {
	t.Parallel()

	args := validArguments()
	args.EpochStartTrigger = nil

	bootstrapper, err := NewMetaStorageBootstrapper(args)
	assert.Equal(t, common.ErrNilEpochStartTrigger, err)
	assert.Nil(t, bootstrapper)
}
