package syncer

import (
	"context"
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

// Regression GHSA-fw38-pc54-jvx9 (KLC-2420): every error path out of a
// syncAccountDataTries worker must release its throttler slot; otherwise the
// fan-out spins on !TryStartProcessing() forever.

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

func TestUserAccountsSyncer_syncAccountDataTries_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestUserAccountsSyncer(t, 2)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	rootHashes := [][]byte{
		[]byte("11111111111111111111111111111111"),
		[]byte("22222222222222222222222222222222"),
		[]byte("33333333333333333333333333333333"),
	}

	err := syncer.syncAccountDataTries(rootHashes, statistics.NewTrieSyncStatistics(), ctx)
	require.ErrorIs(t, err, trie.ErrTimeIsOut,
		"empty storage and no peer responses must surface trie.ErrTimeIsOut, not the fan-out giving up on a leaked slot")

	require.True(t, thr.TryStartProcessing(),
		"throttler slots must be released on every error path (GHSA-fw38-pc54-jvx9)")
	require.True(t, thr.TryStartProcessing())
}

func TestKappAccountsSyncer_syncAccountDataTries_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestKappAccountsSyncer(t, 2)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	rootHashes := [][]byte{
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		[]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		[]byte("dddddddddddddddddddddddddddddddd"),
	}

	err := syncer.syncAccountDataTries(rootHashes, statistics.NewTrieSyncStatistics(), ctx)
	require.ErrorIs(t, err, trie.ErrTimeIsOut,
		"empty storage and no peer responses must surface trie.ErrTimeIsOut, not the fan-out giving up on a leaked slot")

	require.True(t, thr.TryStartProcessing(),
		"throttler slots must be released on every error path (GHSA-fw38-pc54-jvx9)")
	require.True(t, thr.TryStartProcessing())
}

func TestUserAccountsSyncer_syncAccountDataTries_duplicateRootStillReleasesThrottler(t *testing.T) {
	t.Parallel()

	syncer, thr := newTestUserAccountsSyncer(t, 1)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	rootHash := []byte("ccccccccccccccccccccccccccccccccc")

	// Pre-seed the dataTries map so the duplicate-root early return is exercised
	// without going through trieSyncer.StartSyncing().
	syncer.syncerMutex.Lock()
	syncer.dataTries[string(rootHash)] = nil
	syncer.syncerMutex.Unlock()

	err := syncer.syncAccountDataTries([][]byte{rootHash, rootHash, rootHash}, statistics.NewTrieSyncStatistics(), ctx)
	require.NoError(t, err, "a single-slot throttler must be reusable after each duplicate-root early return")

	require.True(t, thr.TryStartProcessing(),
		"duplicate-root early return must release the throttler slot")
}
