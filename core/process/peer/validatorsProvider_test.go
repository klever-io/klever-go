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
	"github.com/klever-io/klever-go/data/block"
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
		vp, _ := NewValidatorsProvider(arg)

		// Manually set cache and last update time; the constructor starts a
		// background refresh goroutine, so the fields must be set under the lock
		vp.lock.Lock()
		vp.cache = map[string]*state.ValidatorApiResponse{
			"validator1": {ValidatorStatus: "eligible"},
		}
		vp.lastCacheUpdate = time.Now()
		vp.lock.Unlock()

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
		vp, _ := NewValidatorsProvider(arg)

		// Set an old cache under the lock (the background refresh goroutine
		// started by the constructor also writes these fields)
		vp.lock.Lock()
		vp.cache = map[string]*state.ValidatorApiResponse{
			"validator1": {ValidatorStatus: "eligible"},
		}
		vp.lastCacheUpdate = time.Now().Add(-time.Hour)
		vp.lock.Unlock()

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
		vp, _ := NewValidatorsProvider(arg)

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
		vp, _ := NewValidatorsProvider(arg)

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
		vp, _ := NewValidatorsProvider(arg)

		vp.updateCache()
		vp.lock.RLock()
		cache := vp.cache
		vp.lock.RUnlock()
		assert.Empty(t, cache)
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
		vp, _ := NewValidatorsProvider(arg)

		vp.updateCache()
		vp.lock.RLock()
		cache := vp.cache
		vp.lock.RUnlock()
		assert.Len(t, cache, 1)
		assert.Contains(t, cache, hex.EncodeToString([]byte("validator1")))
	})
}

func TestValidatorsProvider_RefreshCache(t *testing.T) {
	t.Parallel()

	// StartConsensus calls this once the storage bootstrap restored the epoch,
	// so the cache must be rebuilt against that epoch rather than the one the
	// provider was constructed with
	arg := createDefaultValidatorsProviderArg()
	// the constructor's background refresh goroutine also queries the coordinator
	var queriedMut sync.Mutex
	queriedEpochs := make([]uint32, 0)
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			queriedMut.Lock()
			queriedEpochs = append(queriedEpochs, epoch)
			queriedMut.Unlock()

			return [][]byte{[]byte("validator1")}, nil
		},
	}
	arg.ValidatorStatistics = &mock.ValidatorStatisticsProcessorStub{
		LastFinalizedRootHashCalled: func() []byte {
			return []byte("rootHash")
		},
		GetValidatorInfoForRootHashCalled: func(_ []byte) ([]*state.ValidatorInfo, error) {
			return []*state.ValidatorInfo{
				{PublicKey: []byte("validator1"), List: string(core.EligibleList)},
			}, nil
		},
	}

	vp, err := NewValidatorsProvider(arg)
	require.Nil(t, err)

	vp.RefreshCache(7)

	vp.lock.RLock()
	currentEpoch := vp.currentEpoch
	cache := vp.cache
	vp.lock.RUnlock()

	queriedMut.Lock()
	queried := append([]uint32{}, queriedEpochs...)
	queriedMut.Unlock()

	require.Equal(t, uint32(7), currentEpoch)
	require.Contains(t, cache, hex.EncodeToString([]byte("validator1")))
	require.Contains(t, queried, uint32(7))
}

func TestValidatorsProvider_EpochStartRefreshesThroughChannel(t *testing.T) {
	t.Parallel()

	// the epoch start notification hands the new epoch to startRefreshProcess
	// over refreshCache, which is the only path that updates currentEpoch from
	// the background goroutine
	arg := createDefaultValidatorsProviderArg()
	epochStartNotifier := &mock.EpochStartNotifierStub{}
	arg.EpochStartEventNotifier = epochStartNotifier

	vp, err := NewValidatorsProvider(arg)
	require.Nil(t, err)
	defer func() {
		_ = vp.Close()
	}()

	epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Epoch: 9}})

	require.Eventually(t, func() bool {
		vp.lock.RLock()
		defer vp.lock.RUnlock()

		return vp.currentEpoch == uint32(9)
	}, 2*time.Second, 5*time.Millisecond)
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
		vp, _ := NewValidatorsProvider(arg)

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
