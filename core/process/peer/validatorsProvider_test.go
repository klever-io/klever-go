package peer

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	arg.CacheRefreshInterval = 0
	vp, err := NewValidatorsProvider(arg)

	assert.Equal(t, common.ErrInvalidCacheRefreshIntervalInSec, err)
	assert.True(t, check.IfNil(vp))
}

func TestValidatorsProvider_Cancel_startRefreshProcess(t *testing.T) {
	arg := createDefaultValidatorsProviderArg()

	arg.CacheRefreshInterval = 1 * time.Millisecond
	vsp := validatorsProvider{
		nodesCoordinator:             arg.NodesCoordinator,
		validatorStatistics:          arg.ValidatorStatistics,
		cache:                        make(map[string]*state.ValidatorApiResponse),
		cacheRefreshIntervalDuration: arg.CacheRefreshInterval,
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

func TestValidatorsProvider_GetLatestValidators(t *testing.T) {
	t.Parallel()

	t.Run("should return cached validators if cache is fresh", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		vp := newTestValidatorsProvider(arg)

		// Manually set cache and last update time
		vp.cache = map[string]*state.ValidatorApiResponse{
			"validator1": {ValidatorStatus: "eligible"},
		}
		vp.lastCacheUpdate = time.Now()

		validators := vp.GetLatestValidators()
		assert.Len(t, validators, 1)
		assert.Equal(t, "eligible", validators["validator1"].ValidatorStatus)
	})

	t.Run("should update cache if it's stale", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.CacheRefreshInterval = time.Millisecond
		arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return []byte("rootHash")
			},
			GetValidatorInfoForRootHashCalled: func(rootHash []byte) ([]*state.ValidatorInfo, error) {
				return []*state.ValidatorInfo{
					{PublicKey: []byte("validator2"), List: string(core.EligibleList)},
				}, nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		// Set an old cache
		vp.cache = map[string]*state.ValidatorApiResponse{
			"validator1": {ValidatorStatus: "eligible"},
		}
		vp.lastCacheUpdate = time.Now().Add(-time.Hour)

		time.Sleep(time.Millisecond * 10) // Ensure cache refresh interval has passed

		validators := vp.GetLatestValidators()
		assert.Len(t, validators, 1)
		assert.Contains(t, validators, hex.EncodeToString([]byte("validator2")))
	})
}

func TestValidatorsProvider_GetLatestPeers(t *testing.T) {
	t.Parallel()

	t.Run("should return nil if no finalized root hash", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		peers := vp.GetLatestPeers()
		assert.Nil(t, peers)
	})

	t.Run("should return peers from validator statistics", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return []byte("rootHash")
			},
			ListPeerAccountsCalled: func(rootHash []byte) ([]state.PeerAccountHandler, error) {
				return []state.PeerAccountHandler{
					state.NewEmptyPeerAccount(),
					state.NewEmptyPeerAccount(),
				}, nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		peers := vp.GetLatestPeers()
		assert.Len(t, peers, 2)
	})
}

func TestValidatorsProvider_UpdateCache(t *testing.T) {
	t.Parallel()

	t.Run("should not update cache if no finalized root hash", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		vp.updateCache()
		assert.Empty(t, vp.cache)
	})

	t.Run("should update cache with new validator info", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return []byte("rootHash")
			},
			GetValidatorInfoForRootHashCalled: func(rootHash []byte) ([]*state.ValidatorInfo, error) {
				return []*state.ValidatorInfo{
					{PublicKey: []byte("validator1"), List: string(core.EligibleList)},
				}, nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		vp.updateCache()
		assert.Len(t, vp.cache, 1)
		assert.Contains(t, vp.cache, hex.EncodeToString([]byte("validator1")))
	})
}

func TestValidatorsProvider_CreateNewCache(t *testing.T) {
	t.Parallel()

	t.Run("should create cache with different validator types", func(t *testing.T) {
		arg := createDefaultValidatorsProviderArg()
		arg.NodesCoordinator = &mock.NodesCoordinatorMock{
			GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
				return [][]byte{[]byte("elected")}, nil
			},
			GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
				return [][]byte{[]byte("eligible")}, nil
			},
		}
		vp := newTestValidatorsProvider(arg)

		cache := vp.createNewCache(0, []*state.ValidatorInfo{
			{PublicKey: []byte("elected"), List: string(core.ElectedList)},
			{PublicKey: []byte("eligible"), List: string(core.EligibleList)},
		})

		assert.Len(t, cache, 2)
		assert.Equal(t, string(core.ElectedList), cache[hex.EncodeToString([]byte("elected"))].ValidatorStatus)
		assert.Equal(t, string(core.EligibleList), cache[hex.EncodeToString([]byte("eligible"))].ValidatorStatus)
	})
}

func TestValidatorsProvider_Close(t *testing.T) {
	t.Parallel()

	arg := createDefaultValidatorsProviderArg()
	vp, _ := NewValidatorsProvider(arg)

	err := vp.Close()
	assert.Nil(t, err)
}

func TestValidatorsProvider_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	arg := createDefaultValidatorsProviderArg()
	arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
		LastFinalizedRootHashCalled: func() []byte {
			return []byte("rootHash")
		},
		GetValidatorInfoForRootHashCalled: func(rootHash []byte) ([]*state.ValidatorInfo, error) {
			return []*state.ValidatorInfo{
				{PublicKey: []byte("validator1"), List: string(core.EligibleList)},
			}, nil
		},
	}
	vp, err := NewValidatorsProvider(arg)
	require.Nil(t, err)
	t.Cleanup(func() { _ = vp.Close() })

	// hammer the public read path while the background refresher is alive;
	// the 1ms refresh interval keeps forcing the stale->updateCache path
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = vp.GetLatestValidators()
			}
		}()
	}

	// force refreshes through the epoch channel, as epochStartEventHandler does
	for epoch := uint32(0); epoch < 10; epoch++ {
		vp.refreshCache <- epoch
	}

	wg.Wait()
	assert.Nil(t, vp.Close())

	validators := vp.GetLatestValidators()
	assert.Contains(t, validators, hex.EncodeToString([]byte("validator1")))
}

// newTestValidatorsProvider builds a validatorsProvider without launching the
// background refresh goroutine, keeping cache-focused unit tests deterministic
// and race-free. The constructor's goroutine/lifecycle is covered separately by
// the NewValidatorsProvider-based tests (Close, Cancel_startRefreshProcess).
func newTestValidatorsProvider(arg ArgValidatorsProvider) *validatorsProvider {
	return &validatorsProvider{
		nodesCoordinator:             arg.NodesCoordinator,
		validatorStatistics:          arg.ValidatorStatistics,
		cache:                        make(map[string]*state.ValidatorApiResponse),
		cacheRefreshIntervalDuration: arg.CacheRefreshInterval,
		refreshCache:                 make(chan uint32),
		cancelFunc:                   func() {},
		pubkeyConverter:              arg.PubKeyConverter,
		maxRating:                    arg.MaxRating,
		currentEpoch:                 arg.StartEpoch,
	}
}

func createDefaultValidatorsProviderArg() ArgValidatorsProvider {
	return ArgValidatorsProvider{
		NodesCoordinator:        &mock.NodesCoordinatorMock{},
		StartEpoch:              1,
		EpochStartEventNotifier: &mock.EpochStartNotifierStub{},
		CacheRefreshInterval:    1 * time.Millisecond,
		ValidatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte {
				return []byte("rootHash")
			},
		},
		MaxRating:       100,
		PubKeyConverter: cryptoMock.NewPubkeyConverterMock(32),
	}
}
