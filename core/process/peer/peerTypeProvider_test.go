package peer

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/stretchr/testify/assert"
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
