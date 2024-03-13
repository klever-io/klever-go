package networksharding_test

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common"
	testscommon "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/sharding/mock"
	"github.com/klever-io/klever-go/sharding/networksharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

const epochZero = uint32(0)

//------- NewPeerShardMapper

func createPeerShardMapper() *networksharding.PeerShardMapper {
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{},
		epochZero,
	)
	return psm
}

func TestNewPeerShardMapper_NilNodesCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	psm, err := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		nil,
		epochZero,
	)

	assert.True(t, check.IfNil(psm))
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewPeerShardMapper_NilCacherForPeerIDPkShouldErr(t *testing.T) {
	t.Parallel()

	psm, err := networksharding.NewPeerShardMapper(
		nil,
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{},
		epochZero,
	)

	assert.True(t, check.IfNil(psm))
	assert.Equal(t, common.ErrNilCacher, err)
}

func TestNewPeerShardMapper_NilCacherForPeerIDShardIDShouldErr(t *testing.T) {
	t.Parallel()

	psm, err := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		nil,
		&nodesCoordinatorStub{},
		epochZero,
	)

	assert.True(t, check.IfNil(psm))
	assert.Equal(t, common.ErrNilCacher, err)
}

func TestNewPeerShardMapper_ShouldWork(t *testing.T) {
	t.Parallel()

	epoch := uint32(8843)
	psm, err := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{},
		epoch,
	)

	assert.False(t, check.IfNil(psm))
	assert.Nil(t, err)
	assert.Equal(t, epoch, psm.Epoch())
}

//------- UpdatePeerIDPublicKey

func TestPeerShardMapper_UpdatePeerIDPublicKeyShouldWork(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pid := core.PeerID("dummy peer ID")
	pk := []byte("dummy pk")

	psm.UpdatePeerIDPublicKey(pid, pk)

	pkRecovered := psm.GetPkFromPidPk(pid)
	assert.Equal(t, pk, pkRecovered)
}

func TestPeerShardMapper_UpdatePeerIDPublicKeyMorePidsThanAllowedShouldTrim(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pk := []byte("dummy pk")
	pids := make([]core.PeerID, networksharding.MaxNumPidsPerPk+1)
	for i := 0; i < networksharding.MaxNumPidsPerPk+1; i++ {
		pids[i] = core.PeerID(fmt.Sprintf("pid %d", i))
		psm.UpdatePeerIDPublicKey(pids[i], pk)
	}

	for i := 0; i < networksharding.MaxNumPidsPerPk+1; i++ {
		shouldExists := i > 0 //the pid is evicted based on the first-in-first-out rule
		pkRecovered := psm.GetPkFromPidPk(pids[i])

		if shouldExists {
			assert.Equal(t, pk, pkRecovered)
		} else {
			assert.Nil(t, pkRecovered)
		}
	}
}

func TestPeerShardMapper_UpdatePeerIDPublicKeyShouldUpdatePkForExistentPid(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pk1 := []byte("dummy pk1")
	pk2 := []byte("dummy pk2")
	pids := make([]core.PeerID, networksharding.MaxNumPidsPerPk+1)
	for i := 0; i < networksharding.MaxNumPidsPerPk; i++ {
		pids[i] = core.PeerID(fmt.Sprintf("pid %d", i))
	}

	newPid := core.PeerID("new pid")
	psm.UpdatePeerIDPublicKey(pids[0], pk1)
	psm.UpdatePeerIDPublicKey(newPid, pk1)

	for i := 0; i < networksharding.MaxNumPidsPerPk; i++ {
		psm.UpdatePeerIDPublicKey(pids[i], pk2)
	}

	for i := 0; i < networksharding.MaxNumPidsPerPk; i++ {
		pkRecovered := psm.GetPkFromPidPk(pids[i])

		assert.Equal(t, pk2, pkRecovered)
	}

	assert.Equal(t, []core.PeerID{newPid}, psm.GetFromPkPeerID(pk1))
}

func TestPeerShardMapper_UpdatePeerIDPublicKeyWrongTypePkInPeerIDPkShouldRemove(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pk1 := []byte("dummy pk1")
	pid1 := core.PeerID("pid1")

	wrongTypePk := uint64(7)
	psm.PeerIDPk().Put([]byte(pid1), wrongTypePk, 8)

	psm.UpdatePeerIDPublicKey(pid1, pk1)

	pkRecovered := psm.GetPkFromPidPk(pid1)
	assert.Equal(t, pk1, pkRecovered)
}

func TestPeerShardMapper_UpdatePeerIDPublicKeyShouldWorkConcurrently(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pid := core.PeerID("dummy peer ID")
	pk := []byte("dummy pk")

	numUpdates := 100
	wg := &sync.WaitGroup{}
	wg.Add(numUpdates)
	for i := 0; i < numUpdates; i++ {
		go func() {
			psm.UpdatePeerIDPublicKey(pid, pk)
			wg.Done()
		}()
	}
	wg.Wait()

	pkRecovered := psm.GetPkFromPidPk(pid)
	assert.Equal(t, pk, pkRecovered)
}

//------- GetPeerInfo

func TestPeerShardMapper_GetPeerInfoPkNotFoundShouldReturnUnknown(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()
	pid := core.PeerID("dummy peer ID")

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.UnknownPeer,
		ShardID:  0,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoNodesCoordinatorHasTheShardID(t *testing.T) {
	t.Parallel()

	pk := []byte("dummy pk")
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				if bytes.Equal(publicKey, pk) {
					return nil, nil
				}

				return nil, nil
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.UpdatePeerIDPublicKey(pid, pk)

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.ValidatorPeer,
		ShardID:  0,
		PkBytes:  pk,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoNodesCoordinatorWrongTypeInCacheShouldReturnUnknown(t *testing.T) {
	t.Parallel()

	wrongTypePk := uint64(6)
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.PeerIDPk().Put([]byte(pid), wrongTypePk, 8)

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.UnknownPeer,
		ShardID:  0,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoNodesCoordinatorDoesntHaveItShouldReturnFromTheFallbackMap(t *testing.T) {
	t.Parallel()

	pk := []byte("dummy pk")
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				return nil, errors.New("not found")
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.UpdatePeerIDPublicKey(pid, pk)
	psm.UpdatePeerID(pid)

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.ObserverPeer,
		ShardID:  0,
		PkBytes:  pk,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoNodesCoordinatorDoesntHaveItWrongTypeInCacheShouldReturnUnknown(t *testing.T) {
	t.Parallel()

	pk := []byte("dummy pk")
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				return nil, errors.New("not found")
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.UpdatePeerIDPublicKey(pid, pk)

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.UnknownPeer,
		ShardID:  0,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoShouldRetUnknownShardID(t *testing.T) {
	t.Parallel()

	pk := []byte("dummy pk")
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				return nil, errors.New("not found")
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.UpdatePeerIDPublicKey(pid, pk)

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.UnknownPeer,
		ShardID:  0,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoWithWrongTypeInCacheShouldReturnUnknown(t *testing.T) {
	t.Parallel()

	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				return nil, errors.New("not found")
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	wrongTypeShardID := "shard 4"
	psm.FallbackPidShard().Put([]byte(pid), wrongTypeShardID, len(wrongTypeShardID))

	peerInfo := psm.GetPeerInfo(pid)
	expectedPeerInfo := core.P2PPeerInfo{
		PeerType: core.UnknownPeer,
		ShardID:  0,
	}

	assert.Equal(t, expectedPeerInfo, peerInfo)
}

func TestPeerShardMapper_GetPeerInfoShouldWorkConcurrently(t *testing.T) {
	t.Parallel()

	shardID := uint32(0)
	pk := []byte("dummy pk")
	psm, _ := networksharding.NewPeerShardMapper(
		testscommon.NewCacherMock(),
		testscommon.NewCacherMock(),
		&nodesCoordinatorStub{
			GetValidatorWithPublicKeyCalled: func(publicKey []byte) (validator sharding.Validator, e error) {
				return nil, errors.New("not found")
			},
		},
		epochZero,
	)
	pid := core.PeerID("dummy peer ID")
	psm.UpdatePeerIDPublicKey(pid, pk)
	psm.UpdatePeerID(pid)

	numUpdates := 100
	wg := &sync.WaitGroup{}
	wg.Add(numUpdates)
	for i := 0; i < numUpdates; i++ {
		go func() {
			peerInfo := psm.GetPeerInfo(pid)
			expectedPeerInfo := core.P2PPeerInfo{
				PeerType: core.ObserverPeer,
				ShardID:  shardID,
				PkBytes:  pk,
			}

			assert.Equal(t, expectedPeerInfo, peerInfo)

			wg.Done()
		}()
	}
	wg.Wait()
}

func TestPeerShardMapper_NotifyOrder(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()

	assert.Equal(t, uint32(core.NetworkShardingOrder), psm.NotifyOrder())
}

func TestPeerShardMapper_EpochStartPrepareShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not have panicked", r)
		}
	}()

	psm := createPeerShardMapper()
	psm.EpochStartPrepare(nil)
	psm.EpochStartPrepare(
		&mock.HeaderHandlerStub{
			GetEpochCaled: func() uint32 {
				return 0
			},
		},
	)
}

func TestPeerShardMapper_EpochStartActionWithnilHeaderShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not have panicked", r)
		}
	}()

	psm := createPeerShardMapper()
	psm.EpochStartAction(nil)
}

func TestPeerShardMapper_EpochStartActionShouldWork(t *testing.T) {
	t.Parallel()

	psm := createPeerShardMapper()

	epoch := uint32(676)
	psm.EpochStartAction(
		&mock.HeaderHandlerStub{
			GetEpochCaled: func() uint32 {
				return epoch
			},
		},
	)

	assert.Equal(t, epoch, psm.Epoch())
}
