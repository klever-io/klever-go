package bootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func createMockStorageEpochStartBootstrapArgs() ArgsStorageEpochStartBootstrap {
	return ArgsStorageEpochStartBootstrap{
		ArgsEpochStartBootstrap:    createMockEpochStartBootstrapArgs(),
		ImportDbConfig:             config.ImportDbConfig{},
		ChanGracefullyClose:        make(chan endProcess.ArgEndProcess, 1),
		TimeToWaitForRequestedData: time.Second,
	}
}

func TestNewStorageEpochStartBootstrap_InvalidArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockStorageEpochStartBootstrapArgs()
	args.Hasher = nil
	sesb, err := NewStorageEpochStartBootstrap(args)
	assert.True(t, check.IfNil(sesb))
	assert.True(t, errors.Is(err, common.ErrNilHasher))

	args = createMockStorageEpochStartBootstrapArgs()
	args.ChanGracefullyClose = nil
	sesb, err = NewStorageEpochStartBootstrap(args)
	assert.True(t, check.IfNil(sesb))
	assert.True(t, errors.Is(err, common.ErrNilGracefullyCloseChannel))
}

func TestNewStorageEpochStartBootstrap_ShouldWork(t *testing.T) {
	t.Parallel()

	args := createMockStorageEpochStartBootstrapArgs()
	sesb, err := NewStorageEpochStartBootstrap(args)
	assert.False(t, check.IfNil(sesb))
	assert.Nil(t, err)
}

func TestStorageEpochStartBootstrap_BootstrapStartInEpochNotEnabled(t *testing.T) {
	args := createMockStorageEpochStartBootstrapArgs()

	err := errors.New("localErr")
	args.LatestStorageDataProvider = &mock.LatestStorageDataProviderStub{
		GetCalled: func() (storage.LatestDataFromStorage, error) {
			return storage.LatestDataFromStorage{}, err
		},
	}
	sesb, _ := NewStorageEpochStartBootstrap(args)

	params, err := sesb.Bootstrap()
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), params.Epoch)
}

func TestStorageEpochStartBootstrap_BootstrapFromGenesis(t *testing.T) {
	slotsPerEpoch := uint64(100)
	slotInterval := uint64(60000)
	args := createMockStorageEpochStartBootstrapArgs()
	args.GenesisNodesConfig = &mock.NodesSetupStub{
		GetSlotIntervalCalled: func() uint64 {
			return slotInterval
		},
		GetSlotsPerEpochCalled: func() uint64 {
			return slotsPerEpoch
		},
	}
	args.GeneralConfig = mock.GetGeneralConfig()
	sesb, _ := NewStorageEpochStartBootstrap(args)

	params, err := sesb.Bootstrap()
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), params.Epoch)
}
