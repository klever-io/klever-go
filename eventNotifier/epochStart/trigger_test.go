package epochStart

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

func createMemUnit() storage.Storer {
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10, Shards: 1, SizeInBytes: 0})
	persist, _ := memorydb.NewlruDB(100000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)

	return unit
}

func newTestTrigger(t *testing.T, epoch uint32, epochStartSlot, slotsPerEpoch uint64) *trigger {
	t.Helper()

	store := retriever.NewChainStorer()
	store.AddStorer(retriever.BootstrapUnit, createMemUnit())
	store.AddStorer(retriever.BlockUnit, createMemUnit())

	trig, err := NewEpochStartTrigger(&ArgsNewEpochStartTrigger{
		GenesisTime:        time.Unix(0, 0),
		Epoch:              epoch,
		EpochStartSlot:     epochStartSlot,
		SlotsPerEpoch:      slotsPerEpoch,
		EpochStartNotifier: notifier.NewEpochStartSubscriptionHandler(),
		Marshalizer:        marshal.NewProtoMarshalizer(),
		Hasher:             &sha256.Sha256{},
		Storage:            store,
		ForkController:     mock.NewForkControllerStub(),
	})
	require.NoError(t, err)

	return trig
}

// TestTrigger_EpochStateSnapshotReturnsCurrentState verifies the snapshot mirrors
// exactly the six fields Update mutates.
func TestTrigger_EpochStateSnapshotReturnsCurrentState(t *testing.T) {
	t.Parallel()

	trig := newTestTrigger(t, 3, 30, 10)

	trig.mutTrigger.Lock()
	trig.epoch = 3
	trig.isEpochStart = true
	trig.currentSlot = 42
	trig.currEpochStartSlot = 40
	trig.prevEpochStartSlot = 30
	trig.nextEpochStartSlot = 55
	trig.mutTrigger.Unlock()

	epoch, isEpochStart, currentSlot, currEpochStartSlot, prevEpochStartSlot, nextEpochStartSlot := trig.EpochStateSnapshot()

	require.Equal(t, uint32(3), epoch)
	require.True(t, isEpochStart)
	require.Equal(t, uint64(42), currentSlot)
	require.Equal(t, uint64(40), currEpochStartSlot)
	require.Equal(t, uint64(30), prevEpochStartSlot)
	require.Equal(t, uint64(55), nextEpochStartSlot)
}

// TestTrigger_RestoreEpochStateOverwritesState verifies RestoreEpochState writes
// back exactly the six fields it receives.
func TestTrigger_RestoreEpochStateOverwritesState(t *testing.T) {
	t.Parallel()

	trig := newTestTrigger(t, 5, 50, 10)

	// mutate to a different, "advanced" state
	trig.mutTrigger.Lock()
	trig.epoch = 6
	trig.isEpochStart = true
	trig.currentSlot = 61
	trig.currEpochStartSlot = 60
	trig.prevEpochStartSlot = 50
	trig.nextEpochStartSlot = 77
	trig.mutTrigger.Unlock()

	trig.RestoreEpochState(5, false, 55, 50, 40, disabledSlotForForceEpochStart)

	trig.mutTrigger.RLock()
	defer trig.mutTrigger.RUnlock()
	require.Equal(t, uint32(5), trig.epoch)
	require.False(t, trig.isEpochStart)
	require.Equal(t, uint64(55), trig.currentSlot)
	require.Equal(t, uint64(50), trig.currEpochStartSlot)
	require.Equal(t, uint64(40), trig.prevEpochStartSlot)
	require.Equal(t, uint64(disabledSlotForForceEpochStart), trig.nextEpochStartSlot)
}

// TestTrigger_SnapshotThenRestoreUndoesUpdate is the end-to-end guarantee the
// feature relies on: snapshot before Update, then restore, must leave the trigger
// byte-for-byte where it started for the six epoch-sensitive fields.
func TestTrigger_SnapshotThenRestoreUndoesUpdate(t *testing.T) {
	t.Parallel()

	trig := newTestTrigger(t, 2, 20, 10)

	beforeEpoch, beforeIsStart, beforeCurrent, beforeCurrStart, beforePrevStart, beforeNext :=
		trig.EpochStateSnapshot()

	// drive an epoch-start Update (slot well past currEpochStartSlot+slotsPerEpoch,
	// nonce past the minimum to trigger)
	trig.Update(35, minSlotsToTrigger+1)

	// Update must actually have advanced the trigger, otherwise the test is vacuous.
	afterEpoch, afterIsStart, _, _, _, _ := trig.EpochStateSnapshot()
	require.Equal(t, beforeEpoch+1, afterEpoch)
	require.True(t, afterIsStart)
	require.False(t, beforeIsStart)

	trig.RestoreEpochState(beforeEpoch, beforeIsStart, beforeCurrent, beforeCurrStart, beforePrevStart, beforeNext)

	restoredEpoch, restoredIsStart, restoredCurrent, restoredCurrStart, restoredPrevStart, restoredNext :=
		trig.EpochStateSnapshot()
	require.Equal(t, beforeEpoch, restoredEpoch)
	require.Equal(t, beforeIsStart, restoredIsStart)
	require.Equal(t, beforeCurrent, restoredCurrent)
	require.Equal(t, beforeCurrStart, restoredCurrStart)
	require.Equal(t, beforePrevStart, restoredPrevStart)
	require.Equal(t, beforeNext, restoredNext)
}

// TestTrigger_RestoreEpochStateIsConcurrencySafe exercises the mutex guarding the
// snapshot/restore pair under the race detector.
func TestTrigger_RestoreEpochStateIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	trig := newTestTrigger(t, 1, 10, 10)

	done := make(chan struct{})
	go func() {
		for range 1000 {
			trig.RestoreEpochState(1, false, 15, 10, 0, disabledSlotForForceEpochStart)
		}
		close(done)
	}()

	for range 1000 {
		trig.EpochStateSnapshot()
	}
	<-done
}
