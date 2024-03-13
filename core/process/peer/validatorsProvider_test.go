package peer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewValidatorsProvider_WithNilValidatorStatisticsShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.ValidatorStatistics = nil
	vp, err := NewValidatorsProvider(arg)
	assert.Equal(t, common.ErrNilValidatorStatistics, err)
	assert.Nil(t, vp)
}

func TestNewValidatorsProvider_WithMaxRatingZeroShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.MaxRating = uint32(0)
	vp, err := NewValidatorsProvider(arg)
	assert.Equal(t, common.ErrMaxRatingZero, err)
	assert.Nil(t, vp)
}

func TestNewValidatorsProvider_WithNilValidatorPubkeyConverterShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.PubKeyConverter = nil
	vp, err := NewValidatorsProvider(arg)

	assert.Equal(t, common.ErrNilPubkeyConverter, err)
	assert.True(t, check.IfNil(vp))
}

func TestNewValidatorsProvider_WithNilNodesCoordinatorrShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.NodesCoordinator = nil
	vp, err := NewValidatorsProvider(arg)

	assert.Equal(t, common.ErrNilNodesCoordinator, err)
	assert.True(t, check.IfNil(vp))
}

func TestNewValidatorsProvider_WithNilStartOfEpochTriggerShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.EpochStartEventNotifier = nil
	vp, err := NewValidatorsProvider(arg)

	assert.Equal(t, common.ErrNilEpochStartNotifier, err)
	assert.True(t, check.IfNil(vp))
}

func TestNewValidatorsProvider_WithNilRefresCacheIntervalInSecShouldErr(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()
	arg.CacheRefreshIntervalDurationInSec = 0
	vp, err := NewValidatorsProvider(arg)

	assert.Equal(t, common.ErrInvalidCacheRefreshIntervalInSec, err)
	assert.True(t, check.IfNil(vp))
}

func TestValidatorsProvider_Cancel_startRefreshProcess(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()

	arg.CacheRefreshIntervalDurationInSec = 1 * time.Millisecond
	vsp := validatorsProvider{
		nodesCoordinator:             arg.NodesCoordinator,
		validatorStatistics:          arg.ValidatorStatistics,
		cache:                        make(map[string]*state.ValidatorApiResponse),
		cacheRefreshIntervalDuration: arg.CacheRefreshIntervalDurationInSec,
		refreshCache:                 make(chan uint32),
		lock:                         sync.RWMutex{},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	mutFinished := sync.Mutex{}
	finished := false
	go func() {
		vsp.startRefreshProcess(ctx)
		mutFinished.Lock()
		finished = true
		mutFinished.Unlock()
	}()

	time.Sleep(5 * time.Millisecond)
	mutFinished.Lock()
	currentFinished := finished
	mutFinished.Unlock()
	assert.False(t, currentFinished)

	cancelFunc()

	time.Sleep(5 * time.Millisecond)
	mutFinished.Lock()
	currentFinished = finished
	mutFinished.Unlock()
	assert.True(t, currentFinished)
}

func createDefaultValidatorsProviderArg() ArgValidatorsProvider {
	return ArgValidatorsProvider{
		NodesCoordinator:                  &mock.NodesCoordinatorMock{},
		StartEpoch:                        1,
		EpochStartEventNotifier:           &mock.EpochStartNotifierStub{},
		CacheRefreshIntervalDurationInSec: 1 * time.Millisecond,
		ValidatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return []byte("rootHash")
			},
		},
		MaxRating:       100,
		PubKeyConverter: cryptoMock.NewPubkeyConverterMock(32),
	}
}
