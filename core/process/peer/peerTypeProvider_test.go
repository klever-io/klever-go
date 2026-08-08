package peer

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPeerTypeProvider_NilNodesCoordinator(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = nil

	ptp, err := NewPeerTypeProvider(arg)
	assert.Nil(t, ptp)
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewPeerTypeProvider_NilEpochStartNotifier(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()
	arg.EpochStartEventNotifier = nil

	ptp, err := NewPeerTypeProvider(arg)
	assert.Nil(t, ptp)
	assert.Equal(t, common.ErrNilEpochStartNotifier, err)
}

func TestNewPeerTypeProvider_ShouldWork(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()

	ptp, err := NewPeerTypeProvider(arg)
	assert.Nil(t, err)
	assert.NotNil(t, ptp)
}

func TestPeerTypeProvider_CallsPopulateAndRegister(t *testing.T) {
	numRegisterHandlerCalled := int32(0)
	numPopulateCacheCalled := int32(0)

	arg := createDefaultArgPeerTypeProvider()
	arg.EpochStartEventNotifier = &mock.EpochStartNotifierStub{
		RegisterHandlerCalled: func(handler eventNotifier.ActionHandler) {
			atomic.AddInt32(&numRegisterHandlerCalled, 1)
		},
	}

	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			atomic.AddInt32(&numPopulateCacheCalled, 1)
			return nil, nil
		},
	}

	_, _ = NewPeerTypeProvider(arg)

	assert.Equal(t, int32(1), atomic.LoadInt32(&numPopulateCacheCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&numRegisterHandlerCalled))
}

func TestPeerTypeProvider_UpdateCache(t *testing.T) {
	pk := "pk1"
	elected := [][]byte{
		[]byte(pk),
	}
	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return elected, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	ptp.updateCache(0)

	assert.NotNil(t, ptp.cache)
	assert.Equal(t, len(elected), len(ptp.cache))
	assert.NotNil(t, ptp.cache[pk])
	assert.Equal(t, core.ElectedList, ptp.cache[pk].pType)
}

func TestNewPeerTypeProvider_createCache(t *testing.T) {
	pkElected := "pk1"
	pkEligible := "pk2"

	elected := [][]byte{
		[]byte(pkElected),
	}

	eligible := [][]byte{
		[]byte(pkEligible),
	}

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return elected, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return eligible, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache, ok := ptp.createNewCache(0)

	require.True(t, ok)

	assert.NotNil(t, cache)

	assert.NotNil(t, cache[pkElected])
	assert.Equal(t, core.ElectedList, cache[pkElected].pType)

	assert.NotNil(t, cache[pkEligible])
	assert.Equal(t, core.EligibleList, cache[pkEligible].pType)
}

func TestNewPeerTypeProvider_createCacheIncludesWaitingList(t *testing.T) {
	pkElected := "pk1"
	pkEligible := "pk2"
	pkWaiting := "pk3"

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pkElected)}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pkEligible)}, nil
		},
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pkWaiting)}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache, ok := ptp.createNewCache(0)

	require.True(t, ok)

	require.NotNil(t, cache[pkWaiting])
	require.Equal(t, core.WaitingList, cache[pkWaiting].pType)
}

func TestPeerTypeProvider_RefreshCacheRebuildsFromCoordinator(t *testing.T) {
	// mimic a restart: at construction time the coordinator only holds stale
	// data (no validators); after LoadState restores the real configs a
	// RefreshCache call must rebuild the cache from the coordinator, querying
	// it with the exact epoch the caller passed
	pk := []byte("pk1")
	restored := false
	restoredEpoch := uint32(5)
	var lastQueriedEpoch uint32

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			lastQueriedEpoch = epoch
			if restored {
				return [][]byte{pk}, nil
			}
			return [][]byte{}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	peerType, _, err := ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)

	restored = true
	ptp.RefreshCache(restoredEpoch)

	require.Equal(t, restoredEpoch, lastQueriedEpoch)

	peerType, _, err = ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)
}

func TestPeerTypeProvider_RefreshCacheReplacesCacheWhenListsAreEmptyWithoutError(t *testing.T) {
	pk := []byte("pk1")
	demoted := false

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			if demoted {
				return [][]byte{}, nil
			}
			return [][]byte{pk}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{}, nil
		},
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	peerType, _, err := ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	// successful fetches returning empty lists are a genuine state (validator
	// demoted) and must replace the cache, unlike the all-fetches-failed case
	demoted = true
	ptp.RefreshCache(6)

	peerType, _, err = ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)
}

func TestPeerTypeProvider_RefreshCacheAppliesForEpochOlderThanConstruction(t *testing.T) {
	// mimic a restart where the storage bootstrap walks back to an older epoch:
	// construction happens at StartEpoch 5 (pre-restore data), then the restored
	// chain head is in epoch 4; RefreshCache(4) must rebuild the cache, the
	// construction-time seed must not claim epoch precedence
	pkRestored := []byte("pkRestored")

	arg := createDefaultArgPeerTypeProvider()
	arg.StartEpoch = 5
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			if epoch == 4 {
				return [][]byte{pkRestored}, nil
			}
			return [][]byte{}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	peerType, _, err := ptp.ComputeForPubKey(pkRestored)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)

	ptp.RefreshCache(4)

	peerType, _, err = ptp.ComputeForPubKey(pkRestored)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)
}

func TestPeerTypeProvider_UpdateCacheAcceptsBackwardEpochForRevert(t *testing.T) {
	// a revert from epoch 5 to epoch 4 arrives through the epoch start
	// notification (RevertStateToBlock -> SetProcessed -> NotifyAll) and must
	// update the cache with the older epoch's data; the previous monotonic
	// guard blocked this and left the reverted-to epoch's validators stuck as
	// observers
	pkNew := []byte("pkNewEpoch")
	pkOld := []byte("pkOldEpoch")

	arg := createDefaultArgPeerTypeProvider()
	epochStartNotifier := &mock.EpochStartNotifierStub{}
	arg.EpochStartEventNotifier = epochStartNotifier
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			if epoch == 5 {
				return [][]byte{pkNew}, nil
			}
			return [][]byte{pkOld}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Epoch: 5}})

	peerType, _, err := ptp.ComputeForPubKey(pkNew)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Epoch: 4}})

	peerType, _, err = ptp.ComputeForPubKey(pkOld)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	peerType, _, err = ptp.ComputeForPubKey(pkNew)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)
}

func TestPeerTypeProvider_RefreshCacheAppliesAfterEpochStartMovedCacheForward(t *testing.T) {
	// LoadStorage can restore an epoch start block, which notifies epoch 5 and
	// moves cacheEpoch forward, and then walk back to an earlier checkpoint. The
	// post-bootstrap RefreshCache at the end of StartConsensus then carries the
	// epoch actually restored (4) and must be applied; ordering the two would
	// leave the node reporting epoch 5's validator set
	pkOld := []byte("pkOldEpoch")
	pkNew := []byte("pkNewEpoch")

	arg := createDefaultArgPeerTypeProvider()
	epochStartNotifier := &mock.EpochStartNotifierStub{}
	arg.EpochStartEventNotifier = epochStartNotifier
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			if epoch == 5 {
				return [][]byte{pkNew}, nil
			}
			return [][]byte{pkOld}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Epoch: 5}})

	peerType, _, err := ptp.ComputeForPubKey(pkNew)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	ptp.RefreshCache(4)

	peerType, _, err = ptp.ComputeForPubKey(pkOld)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	peerType, _, err = ptp.ComputeForPubKey(pkNew)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)
}

func TestPeerTypeProvider_RefreshCacheKeepsPreviousCacheWhenAllListsFail(t *testing.T) {
	pk := []byte("pk1")
	failing := false

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			if failing {
				return nil, sharding.ErrEpochNodesConfigDoesNotExist
			}
			return [][]byte{pk}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			if failing {
				return nil, sharding.ErrEpochNodesConfigDoesNotExist
			}
			return [][]byte{}, nil
		},
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			if failing {
				return nil, sharding.ErrEpochNodesConfigDoesNotExist
			}
			return [][]byte{}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	peerType, _, err := ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)

	// a refresh for an epoch the coordinator does not know must not wipe the
	// previously valid cache
	failing = true
	ptp.RefreshCache(42)

	peerType, _, err = ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)
}

func TestNewPeerTypeProvider_CallsUpdateCacheOnEpochChange(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()
	callNumber := 0
	epochStartNotifier := &mock.EpochStartNotifierStub{}
	arg.EpochStartEventNotifier = epochStartNotifier
	pkElectedInTrie := "pk1"
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			callNumber++
			// first call comes from the constructor
			if callNumber == 1 {
				return nil, nil
			}

			return [][]byte{
				[]byte(pkElectedInTrie),
			}, nil
		},
	}

	ptp, _ := NewPeerTypeProvider(arg)

	assert.Equal(t, 0, len(ptp.GetCache())) // nothing in cache
	epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 3}})
	assert.Equal(t, 1, len(ptp.GetCache()))
	assert.NotNil(t, ptp.GetCache()[pkElectedInTrie])
}

func TestNewPeerTypeProvider_ComputeForKeyFromCache(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()
	pk := []byte("pk1")

	popMutex := sync.RWMutex{}
	populateCacheCalled := false
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			populateCacheCalled = true
			return [][]byte{
				pk,
			}, nil
		},
	}

	ptp, _ := NewPeerTypeProvider(arg)
	popMutex.Lock()
	populateCacheCalled = false
	popMutex.Unlock()
	peerType, _, err := ptp.ComputeForPubKey(pk)

	popMutex.RLock()
	called := populateCacheCalled
	popMutex.RUnlock()
	assert.False(t, called)
	assert.Equal(t, core.ElectedList, peerType)
	assert.Nil(t, err)
}

func TestNewPeerTypeProvider_ComputeForKeyNotFoundInCacheReturnsObserver(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()
	pk := []byte("pk1")
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{}, nil
		},
	}

	ptp, _ := NewPeerTypeProvider(arg)

	peerType, _, err := ptp.ComputeForPubKey(pk)

	assert.Equal(t, core.ObserverList, peerType)
	assert.Nil(t, err)
}

func TestNewPeerTypeProvider_IsInterfaceNil(t *testing.T) {
	arg := createDefaultArgPeerTypeProvider()

	ptp, _ := NewPeerTypeProvider(arg)
	assert.False(t, ptp.IsInterfaceNil())
}

func createDefaultArgPeerTypeProvider() ArgPeerTypeProvider {
	return ArgPeerTypeProvider{
		NodesCoordinator:        &mock.NodesCoordinatorMock{},
		StartEpoch:              0,
		EpochStartEventNotifier: &mock.EpochStartNotifierStub{},
	}
}

func TestPeerTypeProvider_EpochStartEventHandler(t *testing.T) {
	t.Parallel()

	arg := createDefaultArgPeerTypeProvider()
	ptp, _ := NewPeerTypeProvider(arg)

	handler := ptp.epochStartEventHandler()
	require.NotNil(t, handler)

	// Check if the cache was updated
	ptp.mutCache.RLock()
	cacheLen := len(ptp.cache)
	ptp.mutCache.RUnlock()

	assert.Equal(t, cacheLen, 0)

	// change elected list
	ptp.nodesCoordinator.(*mock.NodesCoordinatorMock).GetAllElectedValidatorsKeysCalled = func() ([][]byte, error) {
		return [][]byte{[]byte("pk1")}, nil
	}

	header := &block.Block{Header: &block.BlockHeader{
		Nonce: 100,
		Epoch: 1,
	}}

	handler.EpochStartAction(header)

	// Allow some time for the goroutine to execute
	time.Sleep(time.Millisecond * 100)

	// Check if the cache was updated
	ptp.mutCache.RLock()
	cacheLen = len(ptp.cache)
	ptp.mutCache.RUnlock()

	assert.Greater(t, cacheLen, 0)
}

func TestPeerTypeProvider_CreateNewCacheScenarios(t *testing.T) {
	t.Parallel()

	arg := createDefaultArgPeerTypeProvider()

	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("elected1"), []byte("elected2")}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("eligible1"), []byte("elected1")}, nil
		},
	}
	ptp, _ := NewPeerTypeProvider(arg)

	cache, ok := ptp.createNewCache(0)

	require.True(t, ok)
	assert.Len(t, cache, 3)
	assert.Equal(t, core.EligibleList, cache["elected1"].pType) // elected1 is also eligible as it have been updated in the eligible list
	assert.Equal(t, core.ElectedList, cache["elected2"].pType)
	assert.Equal(t, core.EligibleList, cache["eligible1"].pType)
}

func TestPeerTypeProvider_GetAllPeerTypeInfos(t *testing.T) {
	t.Parallel()

	arg := createDefaultArgPeerTypeProvider()
	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	// Manually populate the cache with some test data
	ptp.cache = map[string]*peerListAndShard{
		"pk1": {pType: core.ElectedList, pShard: 0},
		"pk2": {pType: core.EligibleList, pShard: 1},
		"pk3": {pType: core.WaitingList, pShard: 2},
		"pk4": {pType: core.ObserverList, pShard: 3},
	}

	peerTypeInfos := ptp.GetAllPeerTypeInfos()

	assert.Equal(t, 4, len(peerTypeInfos), "Should return 4 peer type infos")

	// Create a map for easier assertion
	peerTypeInfoMap := make(map[string]*state.PeerTypeInfo)
	for _, pti := range peerTypeInfos {
		peerTypeInfoMap[pti.PublicKey] = pti
	}

	// Assert each peer type info
	assert.Equal(t, &state.PeerTypeInfo{PublicKey: "pk1", PeerType: string(core.ElectedList), ShardId: 0}, peerTypeInfoMap["pk1"])
	assert.Equal(t, &state.PeerTypeInfo{PublicKey: "pk2", PeerType: string(core.EligibleList), ShardId: 0}, peerTypeInfoMap["pk2"])
	assert.Equal(t, &state.PeerTypeInfo{PublicKey: "pk3", PeerType: string(core.WaitingList), ShardId: 0}, peerTypeInfoMap["pk3"])
	assert.Equal(t, &state.PeerTypeInfo{PublicKey: "pk4", PeerType: string(core.ObserverList), ShardId: 0}, peerTypeInfoMap["pk4"])

	// Test with empty cache
	ptp.cache = make(map[string]*peerListAndShard)
	emptyPeerTypeInfos := ptp.GetAllPeerTypeInfos()
	assert.Empty(t, emptyPeerTypeInfos, "Should return empty slice for empty cache")
}

func TestNewPeerTypeProvider_KeepsEmptyCacheWhenConstructionSeedFails(t *testing.T) {
	// all three list fetches fail at construction (no config for the epoch
	// yet); the provider must still construct with an empty cache, and a later
	// refresh (where all getters succeed) must fill it
	pk := []byte("pk1")
	restored := false

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(_ uint32) ([][]byte, error) {
			if restored {
				return [][]byte{pk}, nil
			}
			return nil, errors.New("no config for epoch")
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			if restored {
				return [][]byte{}, nil
			}
			return nil, errors.New("no config for epoch")
		},
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			if restored {
				return [][]byte{}, nil
			}
			return nil, errors.New("no config for epoch")
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	peerType, _, err := ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ObserverList, peerType)

	restored = true
	ptp.RefreshCache(1)

	peerType, _, err = ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)
}

func TestPeerTypeProvider_EpochStartPrepareIsANoOp(t *testing.T) {
	pk := []byte("pk1")

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{pk}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	handler := ptp.epochStartEventHandler()
	handler.EpochStartPrepare(&block.Block{Header: &block.BlockHeader{Epoch: 1}})

	// prepare must not touch the cache; the refresh happens on the action event
	peerType, _, err := ptp.ComputeForPubKey(pk)
	require.Nil(t, err)
	require.Equal(t, core.ElectedList, peerType)
}

func TestPeerTypeProvider_ConcurrentRefreshAndEpochStartAreRaceFree(t *testing.T) {
	// two writers exist in production: the epoch-start notifier callback and
	// RefreshCache from StartConsensus, both racing readers of ComputeForPubKey.
	// backward writes are applied rather than ordered, so the surviving cache is
	// whichever writer landed last; what must hold is that a reader never sees a
	// half-installed cache mixing the two epochs
	pkOldEpoch := []byte("pkOldEpoch")
	pkNewEpoch := []byte("pkNewEpoch")

	arg := createDefaultArgPeerTypeProvider()
	epochStartNotifier := &mock.EpochStartNotifierStub{}
	arg.EpochStartEventNotifier = epochStartNotifier
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllElectedValidatorsKeysWithEpochCalled: func(epoch uint32) ([][]byte, error) {
			if epoch == 2 {
				return [][]byte{pkNewEpoch}, nil
			}
			return [][]byte{pkOldEpoch}, nil
		},
	}

	ptp, err := NewPeerTypeProvider(arg)
	require.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		ptp.RefreshCache(1)
	}()
	go func() {
		defer wg.Done()
		epochStartNotifier.NotifyAll(&block.Block{Header: &block.BlockHeader{Epoch: 2}})
	}()
	// GetAllPeerTypeInfos reads the whole cache under one lock, so a mix of the
	// two epochs here would be a genuine torn read rather than a scheduling
	// artefact of two separate lookups
	mixedRead := int32(0)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			infos := ptp.GetAllPeerTypeInfos()
			if len(infos) > 1 {
				atomic.StoreInt32(&mixedRead, 1)
			}
		}
	}()

	wg.Wait()

	require.Zero(t, atomic.LoadInt32(&mixedRead), "a reader saw a cache mixing both epochs")

	// exactly one of the two caches survived, never a merge of both
	infos := ptp.GetAllPeerTypeInfos()
	require.Len(t, infos, 1)
	require.Contains(t, []string{string(pkNewEpoch), string(pkOldEpoch)}, infos[0].PublicKey)
	require.Equal(t, string(core.ElectedList), infos[0].PeerType)
}
