package syncer

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/trie/statistics"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// Regression GHSA-fw38-pc54-jvx9 (KLC-2420): every data trie started by
// syncAccountDataTries must release its throttler slot, whatever the outcome.

func newTestThrottler(t *testing.T, max int32) *throttler.NumGoRoutinesThrottler {
	t.Helper()

	thr, err := throttler.NewNumGoRoutinesThrottler(max)
	require.NoError(t, err)

	return thr
}

// requireFullBudget fails on a leaked slot (fewer than max claims succeed) and on
// an excess release (a claim past max succeeds), then gives the slots back.
func requireFullBudget(t *testing.T, thr *throttler.NumGoRoutinesThrottler, max int) {
	t.Helper()

	for i := 0; i < max; i++ {
		require.True(t, thr.TryStartProcessing(), "slot %d must be free after the fan-out", i)
	}
	require.False(t, thr.TryStartProcessing(), "a claim past the limit means a slot was released twice")
	for i := 0; i < max; i++ {
		thr.EndProcessing()
	}
}

// countFanOutGoroutines counts goroutines running a syncAccountDataTries closure,
// including ones still waiting for a slot; "created by" lines are skipped so children don't count.
func countFanOutGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}

	count := 0
	for _, g := range strings.Split(string(buf), "\n\n") {
		for _, line := range strings.Split(g, "\n") {
			if !strings.HasPrefix(line, "created by ") && strings.Contains(line, "syncAccountDataTries.func") {
				count++
				break
			}
		}
	}
	return count
}

// peakFanOut runs fanOut, returns the most fan-out goroutines alive at once before ctx
// expires and waits for them all to exit, since a timed-out call returns before its workers do.
// Workers only stop on ctx (the syncer Timeout is longer), so before expiry no worker is in its
// deferred cleanup and each live goroutine holds a slot.
func peakFanOut(t *testing.T, ctx context.Context, fanOut func()) int {
	t.Helper()

	var peak int32
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			// checked after the snapshot, so a recorded count predates the expiry
			if n := int32(countFanOutGoroutines()); ctx.Err() == nil && n > atomic.LoadInt32(&peak) {
				atomic.StoreInt32(&peak, n)
			}
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()

	fanOut()
	close(stop)
	<-done

	require.Eventually(t, func() bool { return countFanOutGoroutines() == 0 },
		5*time.Second, time.Millisecond, "fan-out goroutines must exit before the test ends")

	return int(atomic.LoadInt32(&peak))
}

func manyRootHashes(n int) [][]byte {
	roots := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		roots = append(roots, []byte(fmt.Sprintf("%032d", i)))
	}
	return roots
}

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

func newTestUserAccountsSyncer(t *testing.T, thr data.GoRoutineThrottler) *userAccountsSyncer {
	t.Helper()

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	unit, interceptedNodes := makeSyncerStorageCacher(t)
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(unit)
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

	return syncer
}

func newTestKappAccountsSyncer(t *testing.T, thr data.GoRoutineThrottler) *kappAccountsSyncer {
	t.Helper()

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	unit, interceptedNodes := makeSyncerStorageCacher(t)
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(unit)
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

	return syncer
}

func TestUserAccountsSyncer_syncAccountDataTries_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	thr := newTestThrottler(t, 2)
	syncer := newTestUserAccountsSyncer(t, thr)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()

	rootHashes := [][]byte{
		[]byte("11111111111111111111111111111111"),
		[]byte("22222222222222222222222222222222"),
	}

	err := syncer.syncAccountDataTries(rootHashes, ssh, ctx)
	require.ErrorIs(t, err, trie.ErrTimeIsOut,
		"empty storage and no peer responses must surface trie.ErrTimeIsOut")

	requireFullBudget(t, thr, 2)
}

func TestKappAccountsSyncer_syncAccountDataTries_releasesThrottlerOnTimeout(t *testing.T) {
	t.Parallel()

	thr := newTestThrottler(t, 2)
	syncer := newTestKappAccountsSyncer(t, thr)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()

	rootHashes := [][]byte{
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		[]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
	}

	err := syncer.syncAccountDataTries(rootHashes, ssh, ctx)
	require.ErrorIs(t, err, trie.ErrTimeIsOut,
		"empty storage and no peer responses must surface trie.ErrTimeIsOut")

	requireFullBudget(t, thr, 2)
}

func TestUserAccountsSyncer_syncAccountDataTries_duplicateRootStillReleasesThrottler(t *testing.T) {
	t.Parallel()

	thr := newTestThrottler(t, 2)
	syncer := newTestUserAccountsSyncer(t, thr)

	ctx := t.Context()

	ssh := statistics.NewTrieSyncStatistics()
	rootHash := []byte("ccccccccccccccccccccccccccccccccc")

	// Pre-seed the dataTries map so the duplicate-root early return is exercised
	// without going through trieSyncer.StartSyncing().
	syncer.syncerMutex.Lock()
	syncer.dataTries[string(rootHash)] = nil
	syncer.syncerMutex.Unlock()

	// Three roots on a limit of 2: the third only starts once a slot is released.
	err := syncer.syncAccountDataTries([][]byte{rootHash, rootHash, rootHash}, ssh, ctx)
	require.NoError(t, err)

	requireFullBudget(t, thr, 2)
}

// The loop must claim each slot before spawning, so no fan-out goroutine waits for a slot.
func TestUserAccountsSyncer_syncAccountDataTries_neverExceedsLimit(t *testing.T) {
	// not parallel: the goroutine dump would also see the other test's fan-out
	syncer := newTestUserAccountsSyncer(t, newTestThrottler(t, 2))

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	peak := peakFanOut(t, ctx, func() {
		_ = syncer.syncAccountDataTries(manyRootHashes(500), statistics.NewTrieSyncStatistics(), ctx)
	})

	// spawning before the claim would give one goroutine per root
	require.Equal(t, 2, peak, "the fan-out must fill the limit and never pass it")
}

func TestKappAccountsSyncer_syncAccountDataTries_neverExceedsLimit(t *testing.T) {
	// not parallel: the goroutine dump would also see the other test's fan-out
	syncer := newTestKappAccountsSyncer(t, newTestThrottler(t, 2))

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	peak := peakFanOut(t, ctx, func() {
		_ = syncer.syncAccountDataTries(manyRootHashes(500), statistics.NewTrieSyncStatistics(), ctx)
	})

	// spawning before the claim would give one goroutine per root
	require.Equal(t, 2, peak, "the fan-out must fill the limit and never pass it")
}
