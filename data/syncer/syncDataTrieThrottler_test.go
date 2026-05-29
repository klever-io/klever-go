package syncer

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/trie/statistics"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// Regression GHSA-fw38-pc54-jvx9 (KLC-2420): every error path out of
// syncDataTrie must release the throttler slot. Two timeouts on a throttler
// sized to 2 must leave CanProcess()==true; otherwise the parent fan-out
// in syncAccountDataTries spins on !CanProcess() forever.

func makeSyncerStorageCacher(t *testing.T) (storage.Storer, storage.Cacher) {
	t.Helper()

	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{
		Type:        storageUnit.LRUCache,
		Capacity:    16,
		Shards:      1,
		SizeInBytes: 0,
	})
	require.NoError(t, err)

	persist, err := memorydb.NewlruDB(1024)
	require.NoError(t, err)

	unit, err := storageUnit.NewStorageUnit(cache, persist)
	require.NoError(t, err)

	interceptedNodes, err := storageUnit.NewCache(storageUnit.CacheConfig{
		Type:        storageUnit.LRUCache,
		Capacity:    16,
		Shards:      1,
		SizeInBytes: 0,
	})
	require.NoError(t, err)

	return unit, interceptedNodes
}

func newTestUserAccountsSyncer(t *testing.T, max int32) (*userAccountsSyncer, *throttler.NumGoRoutinesThrottler) {
	t.Helper()

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	unit, interceptedNodes := makeSyncerStorageCacher(t)
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(unit)
	require.NoError(t, err)

	thr, err := throttler.NewNumGoRoutinesThrottler(max)
	require.NoError(t, err)

	args := ArgsNewUserAccountsSyncer{
		ArgsNewBaseAccountsSyncer: ArgsNewBaseAccountsSyncer{
			Hasher:                    hasher,
			Marshalizer:               marshalizer,
			TrieStorageManager:        storageManager,
			RequestHandler:            &mock.RequestHandlerStub{},
			Timeout:                   time.Second,
			Cacher:                    interceptedNodes,
			MaxTrieLevelInMemory:      5,
			MaxHardCapForMissingNodes: 100,
		},
		ShardId:   0,
		Throttler: thr,
	}

	syncer, err := NewUserAccountsSyncer(args)
	require.NoError(t, err)

	return syncer, thr
}

func newTestKappAccountsSyncer(t *testing.T, max int32) (*kappAccountsSyncer, *throttler.NumGoRoutinesThrottler) {
	t.Helper()

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	unit, interceptedNodes := makeSyncerStorageCacher(t)
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(unit)
	require.NoError(t, err)

	thr, err := throttler.NewNumGoRoutinesThrottler(max)
	require.NoError(t, err)

	args := ArgsNewKappAccountsSyncer{
		ArgsNewBaseAccountsSyncer: ArgsNewBaseAccountsSyncer{
			Hasher:                    hasher,
			Marshalizer:               marshalizer,
			TrieStorageManager:        storageManager,
			RequestHandler:            &mock.RequestHandlerStub{},
			Timeout:                   time.Second,
			Cacher:                    interceptedNodes,
			MaxTrieLevelInMemory:      5,
			MaxHardCapForMissingNodes: 100,
		},
		Throttler: thr,
	}

	syncer, err := NewKappAccountsSyncer(args)
	require.NoError(t, err)

	return syncer, thr
}

func TestUserAccountsSyncer_syncDataTrie_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestUserAccountsSyncer(t, 2)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()

	rootHashes := [][]byte{
		[]byte("11111111111111111111111111111111"),
		[]byte("22222222222222222222222222222222"),
	}

	for _, rh := range rootHashes {
		err := syncer.syncDataTrie(rh, ssh, ctx)
		require.ErrorIs(t, err, trie.ErrTimeIsOut,
			"empty storage and no peer responses must surface trie.ErrTimeIsOut")
	}

	require.True(t, thr.CanProcess(),
		"throttler slot must be released on every error path; CanProcess()==false here means the slot leaked (GHSA-fw38-pc54-jvx9)")
}

func TestKappAccountsSyncer_syncDataTrie_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestKappAccountsSyncer(t, 2)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()

	rootHashes := [][]byte{
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		[]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
	}

	for _, rh := range rootHashes {
		err := syncer.syncDataTrie(rh, ssh, ctx)
		require.ErrorIs(t, err, trie.ErrTimeIsOut,
			"empty storage and no peer responses must surface trie.ErrTimeIsOut")
	}

	require.True(t, thr.CanProcess(),
		"throttler slot must be released on every error path; CanProcess()==false here means the slot leaked (GHSA-fw38-pc54-jvx9)")
}

func TestUserAccountsSyncer_syncDataTrie_duplicateRootStillReleasesThrottler(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestUserAccountsSyncer(t, 2)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()
	rootHash := []byte("ccccccccccccccccccccccccccccccccc")

	// Pre-seed the dataTries map so the duplicate-root early return is exercised
	// without going through trieSyncer.StartSyncing().
	syncer.syncerMutex.Lock()
	syncer.dataTries[string(rootHash)] = nil
	syncer.syncerMutex.Unlock()

	for range 3 {
		err := syncer.syncDataTrie(rootHash, ssh, ctx)
		require.NoError(t, err)
	}

	require.True(t, thr.CanProcess(),
		"duplicate-root early return must release the throttler slot")
}
