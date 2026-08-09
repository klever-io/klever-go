package peer

import (
	"fmt"
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

	ptp.UpdateCache(0)

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

	cache := ptp.createNewCache(0)

	assert.NotNil(t, cache)

	assert.NotNil(t, cache[pkElected])
	assert.Equal(t, core.ElectedList, cache[pkElected].pType)

	assert.NotNil(t, cache[pkEligible])
	assert.Equal(t, core.EligibleList, cache[pkEligible].pType)
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

	cache := ptp.createNewCache(0)
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

func TestPeerTypeProvider_CreateNewCache_IncludesWaitingList(t *testing.T) {
	t.Parallel()

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("waiting1")}, nil
		},
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("elected1")}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("eligible1")}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache := ptp.createNewCache(0)

	assert.Len(t, cache, 3)
	assert.Equal(t, core.WaitingList, cache["waiting1"].pType)
	assert.Equal(t, core.ElectedList, cache["elected1"].pType)
	assert.Equal(t, core.EligibleList, cache["eligible1"].pType)
}

func TestPeerTypeProvider_CreateNewCache_WaitingOverwrittenByElected(t *testing.T) {
	t.Parallel()

	pk := "overlapping_pk"

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache := ptp.createNewCache(0)

	assert.Len(t, cache, 1)
	assert.Equal(t, core.ElectedList, cache[pk].pType)
}

func TestPeerTypeProvider_CreateNewCache_AllThreeOverlap(t *testing.T) {
	t.Parallel()

	pk := "overlapping_pk"

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache := ptp.createNewCache(0)

	assert.Len(t, cache, 1)
	assert.Equal(t, core.EligibleList, cache[pk].pType)
}

func TestPeerTypeProvider_CreateNewCache_WaitingErrorDoesNotAffectOtherLists(t *testing.T) {
	t.Parallel()

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return nil, fmt.Errorf("waiting list unavailable")
		},
		GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("elected1")}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte("eligible1")}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache := ptp.createNewCache(0)

	assert.Len(t, cache, 2)
	assert.Equal(t, core.ElectedList, cache["elected1"].pType)
	assert.Equal(t, core.EligibleList, cache["eligible1"].pType)
}

func TestPeerTypeProvider_CreateNewCache_WaitingOverwrittenByEligible(t *testing.T) {
	t.Parallel()

	pk := "overlapping_pk"

	arg := createDefaultArgPeerTypeProvider()
	arg.NodesCoordinator = &mock.NodesCoordinatorMock{
		GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
		GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(pk)}, nil
		},
	}

	ptp := PeerTypeProvider{
		nodesCoordinator: arg.NodesCoordinator,
		cache:            nil,
		mutCache:         sync.RWMutex{},
	}

	cache := ptp.createNewCache(0)

	assert.Len(t, cache, 1)
	assert.Equal(t, core.EligibleList, cache[pk].pType)
}
