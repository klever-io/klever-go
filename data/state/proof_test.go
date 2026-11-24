package state_test

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/stretchr/testify/require"
)

func newProofAccountsDB(t *testing.T) *state.AccountsDB {
	t.Helper()

	marshalizer := &mock.MarshalizerMock{}
	hsh := &mock.HasherMock{}
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hsh, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)
	return adb
}

func putBalance(t *testing.T, adb *state.AccountsDB, address []byte, value int64) {
	t.Helper()

	acc, err := adb.LoadAccount(address)
	require.NoError(t, err)
	require.NoError(t, acc.(state.UserAccountHandler).AddToBalance(value, nil, false))
	require.NoError(t, adb.SaveAccount(acc))
}

func TestAccountsDB_MerkleProofRoundTripLeavesLiveTrieAlone(t *testing.T) {
	t.Parallel()

	adb := newProofAccountsDB(t)
	address := make([]byte, 32)
	address[0] = 7

	putBalance(t, adb, address, 10)
	root1, err := adb.Commit()
	require.NoError(t, err)

	putBalance(t, adb, address, 5)
	journal := adb.JournalLen()
	require.NotZero(t, journal)
	dirtyRoot, err := adb.RootHash()
	require.NoError(t, err)
	require.NotEqual(t, root1, dirtyRoot)

	// The current proof is anchored to the committed root, not the dirty one.
	current, err := adb.GetMerkleProof(address)
	require.NoError(t, err)
	require.Equal(t, root1, current.RootHash)
	require.NotEmpty(t, current.Proof)
	require.NotEmpty(t, current.Value)

	putBalance(t, adb, address, 1)
	root2, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 4)
	journal = adb.JournalLen()
	dirtyRoot, err = adb.RootHash()
	require.NoError(t, err)

	latest, err := adb.GetMerkleProof(address)
	require.NoError(t, err)
	require.Equal(t, root2, latest.RootHash)

	historical, err := adb.GetMerkleProofAtRoot(root1, address)
	require.NoError(t, err)
	require.Equal(t, root1, historical.RootHash)
	require.Equal(t, current.Value, historical.Value)
	require.NotEqual(t, latest.Value, historical.Value)

	ok, err := adb.VerifyMerkleProof(root1, address, historical.Proof)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = adb.VerifyMerkleProof(root2, address, latest.Proof)
	require.NoError(t, err)
	require.True(t, ok)

	// A proof built for one root must not verify under the other.
	ok, err = adb.VerifyMerkleProof(root1, address, latest.Proof)
	require.NoError(t, err)
	require.False(t, ok)

	// The first encoded node is hashed with the chain hasher over the raw bytes.
	hsh := &mock.HasherMock{}
	require.Equal(t, historical.RootHash, hsh.Compute(string(historical.Proof[0])))

	require.Equal(t, journal, adb.JournalLen())
	liveRoot, err := adb.RootHash()
	require.NoError(t, err)
	require.Equal(t, dirtyRoot, liveRoot)

	acc, err := adb.LoadAccount(address)
	require.NoError(t, err)
	require.Equal(t, int64(20), acc.(state.UserAccountHandler).GetBalance(nil, false))
}

func TestAccountsDB_MerkleProofMissingRootAndAccount(t *testing.T) {
	t.Parallel()

	adb := newProofAccountsDB(t)
	address := make([]byte, 32)
	address[0] = 9
	putBalance(t, adb, address, 1)

	// Nothing committed yet, so there is no root to prove against.
	_, err := adb.GetMerkleProof(address)
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)

	root, err := adb.Commit()
	require.NoError(t, err)

	_, err = adb.GetMerkleProof(nil)
	require.ErrorIs(t, err, common.ErrNilAddress)
	_, err = adb.GetMerkleProofAtRoot(nil, address)
	require.ErrorIs(t, err, state.ErrInvalidProofRequest)
	_, err = adb.VerifyMerkleProof(root, nil, nil)
	require.ErrorIs(t, err, common.ErrNilAddress)

	missing := make([]byte, 32)
	missing[0] = 3
	_, err = adb.GetMerkleProof(missing)
	require.ErrorIs(t, err, common.ErrAccNotFound)

	unknown := make([]byte, len(root))
	copy(unknown, root)
	unknown[0] ^= 0xff
	_, err = adb.GetMerkleProofAtRoot(unknown, address)
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	ok, err := adb.VerifyMerkleProof(unknown, address, [][]byte{[]byte("nope")})
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	require.False(t, ok)

	live, err := adb.RootHash()
	require.NoError(t, err)
	require.Equal(t, root, live)
}

func newStubProofAccountsDB(t *testing.T, tr *mock.TrieStub) *state.AccountsDB {
	t.Helper()

	if tr.GetStorageManagerCalled == nil {
		sm, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
		require.NoError(t, err)
		tr.GetStorageManagerCalled = func() data.StorageManager { return sm }
	}
	adb, err := state.NewAccountsDB(tr, &mock.HasherMock{}, &mock.MarshalizerMock{}, factory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)
	return adb
}

func TestAccountsDB_MerkleProofTrieErrors(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	live := []byte("live-root")
	other := []byte("other-root")
	key := []byte("key")

	t.Run("nil key", func(t *testing.T) {
		t.Parallel()
		adb := newStubProofAccountsDB(t, &mock.TrieStub{})
		_, err := adb.GetMerkleProofAtRoot(live, nil)
		require.ErrorIs(t, err, common.ErrNilAddress)
	})

	t.Run("live root hash fails", func(t *testing.T) {
		t.Parallel()
		adb := newStubProofAccountsDB(t, &mock.TrieStub{
			RootCalled: func() ([]byte, error) { return nil, errBoom },
		})
		_, err := adb.GetMerkleProofAtRoot(live, key)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("live trie get and proof fail", func(t *testing.T) {
		t.Parallel()
		tr := &mock.TrieStub{
			RootCalled: func() ([]byte, error) { return live, nil },
			GetCalled:  func([]byte) ([]byte, error) { return nil, errBoom },
		}
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(live, key)
		require.ErrorIs(t, err, errBoom)

		tr.GetCalled = func([]byte) ([]byte, error) { return []byte("value"), nil }
		tr.GetProofCalled = func([]byte) ([][]byte, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(live, key)
		require.ErrorIs(t, err, errBoom)

		tr.GetProofCalled = func([]byte) ([][]byte, error) { return nil, nil }
		proof, err := adb.GetMerkleProofAtRoot(live, key)
		require.NoError(t, err)
		require.Nil(t, proof.Proof)
	})

	t.Run("recreate failures", func(t *testing.T) {
		t.Parallel()
		tr := &mock.TrieStub{
			RootCalled: func() ([]byte, error) { return live, nil },
			RecreateCalled: func([]byte) (data.Trie, error) {
				t.Fatal("proofs must not use the snapshot-capable Recreate")
				return nil, nil
			},
		}
		tr.RecreateFromMainDbCalled = recreateAtRoot(tr, func() (data.Trie, error) { return nil, trie.ErrHashNotFound })
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		tr.RecreateFromMainDbCalled = recreateAtRoot(tr, func() (data.Trie, error) { return nil, errBoom })
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)

		tr.RecreateFromMainDbCalled = recreateAtRoot(tr, func() (data.Trie, error) { return nil, nil })
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	})

	t.Run("empty trie recreate fails", func(t *testing.T) {
		t.Parallel()
		adb := newStubProofAccountsDB(t, &mock.TrieStub{
			RootCalled: func() ([]byte, error) { return live, nil },
			RecreateFromMainDbCalled: func(root []byte) (data.Trie, error) {
				require.Nil(t, root, "the live trie must only recreate the empty trie")
				return nil, errBoom
			},
		})
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("recreated trie root mismatch or error", func(t *testing.T) {
		t.Parallel()
		recreated := &mock.TrieStub{RootCalled: func() ([]byte, error) { return []byte("wrong"), nil }}
		tr := &mock.TrieStub{RootCalled: func() ([]byte, error) { return live, nil }}
		tr.RecreateFromMainDbCalled = recreateAtRoot(tr, func() (data.Trie, error) { return recreated, nil })
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		recreated.RootCalled = func() ([]byte, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)
	})
}

// recreateAtRoot stubs RecreateFromMainDb: the empty-root call returns tr itself, as a proof
// first recreates an empty trie, and the historical-root call returns atRoot().
func recreateAtRoot(tr *mock.TrieStub, atRoot func() (data.Trie, error)) func([]byte) (data.Trie, error) {
	return func(root []byte) (data.Trie, error) {
		if root == nil {
			return tr, nil
		}
		return atRoot()
	}
}

// hookDb calls onGet before each read, so a test can act while a trie walk is in progress.
type hookDb struct {
	*mock.MemDbMock
	onGet func(key []byte)
}

func (h *hookDb) Get(key []byte) ([]byte, error) {
	if h.onGet != nil {
		h.onGet(key)
	}
	return h.MemDbMock.Get(key)
}

// newPruningProofAccountsDB builds accounts over a trie storage manager that prunes, on db.
func newPruningProofAccountsDB(t *testing.T, db data.DBWriteCacher, marshalizer *mock.ProtobufMarshalizerMock) *state.AccountsDB {
	t.Helper()

	hsh := &mock.HasherMock{}
	ewl, err := mock.NewEvictionWaitingList(100, mock.NewMemDbMock(), marshalizer)
	require.NoError(t, err)
	tsm, err := trie.NewTrieStorageManager(
		db,
		marshalizer,
		hsh,
		config.DBConfig{
			FilePath:          t.TempDir(),
			Type:              string(storageUnit.LvlDBSerial),
			BatchDelaySeconds: 1,
			MaxBatchSize:      1,
			MaxOpenFiles:      10,
		},
		ewl,
		config.TrieStorageManagerConfig{PruningBufferLen: 1000, SnapshotsBufferLen: 10, MaxSnapshots: 2},
	)
	require.NoError(t, err)
	tr, err := trie.NewTrie(tsm, marshalizer, hsh, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)
	return adb
}

func putStorage(t *testing.T, adb *state.AccountsDB, address []byte, key []byte, value []byte) {
	t.Helper()

	acc, err := adb.LoadAccount(address)
	require.NoError(t, err)
	require.NoError(t, acc.(state.UserAccountHandler).DataTrieTracker().SaveKeyValue(key, value))
	require.NoError(t, adb.SaveAccount(acc))
}

func TestAccountsDB_MerkleProofHoldsPruneLockNotAccountsLock(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
	adb := newPruningProofAccountsDB(t, db, marshalizer)

	address := make([]byte, 32)
	address[0] = 1
	other := make([]byte, 32)
	other[0] = 2
	putBalance(t, adb, address, 10)
	putBalance(t, adb, other, 3)
	root1, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 5)
	_, err = adb.Commit()
	require.NoError(t, err)

	// Child nodes are read only while the proof walks root1. At every such read PruneTrie
	// must still be shut out, while the accounts lock block processing needs stays free.
	walkReads := 0
	db.onGet = func(key []byte) {
		if bytes.Equal(key, root1) {
			return
		}
		walkReads++
		require.False(t, adb.TryLockMutPrune(), "prune lock released during the proof walk")
		require.True(t, adb.TryLockMutOp(), "accounts lock held during the historical proof walk")
	}
	proof, err := adb.GetMerkleProofAtRoot(root1, address)
	db.onGet = nil
	require.NoError(t, err)
	require.Positive(t, walkReads)

	// The last node is the account leaf: a protobuf CollapsedLn followed by the leaf type byte.
	last := proof.Proof[len(proof.Proof)-1]
	require.Equal(t, byte(1), last[len(last)-1])
	leaf := &trie.CollapsedLn{}
	require.NoError(t, marshalizer.Unmarshal(leaf, last[:len(last)-1]))
	require.Equal(t, proof.Value, leaf.Value)

	// Once root1 is pruned, a proof for it is a clear unavailable-root error.
	adb.CancelPrune(root1, data.NewRoot)
	adb.PruneTrie(root1, data.OldRoot)
	_, err = adb.GetMerkleProofAtRoot(root1, address)
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)
}

func TestAccountsDB_MerkleProofLiveRootHoldsAccountsLock(t *testing.T) {
	t.Parallel()

	live := []byte("live-root")
	var adb *state.AccountsDB
	tr := &mock.TrieStub{
		RootCalled: func() ([]byte, error) { return live, nil },
		GetCalled: func([]byte) ([]byte, error) {
			// block processing mutates the live trie under the accounts lock, so a walk
			// of it must hold that lock too
			require.False(t, adb.TryLockMutOp(), "accounts lock released during the live proof walk")
			return []byte("value"), nil
		},
		GetProofCalled: func([]byte) ([][]byte, error) { return nil, nil },
	}
	adb = newStubProofAccountsDB(t, tr)

	_, err := adb.GetMerkleProofAtRoot(live, []byte("key"))
	require.NoError(t, err)
}

// A historical proof blocked mid-walk lets Commit through and holds PruneTrie off until the
// walk ends; then proofs, commits and prunes interleave without deadlock or data race.
func TestAccountsDB_MerkleProofInterleavesWithCommitAndPruneWithoutDeadlock(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
	adb := newPruningProofAccountsDB(t, db, marshalizer)

	address := make([]byte, 32)
	address[0] = 1
	other := make([]byte, 32)
	other[0] = 2
	putBalance(t, adb, address, 1)
	putBalance(t, adb, other, 1)
	root, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 2)
	next, err := adb.Commit()
	require.NoError(t, err)
	adb.CancelPrune(next, data.NewRoot)
	putBalance(t, adb, address, 3)

	// Block the historical walk of root at its first child read.
	walkStarted := make(chan struct{})
	releaseWalk := make(chan struct{})
	var blockOnce sync.Once
	db.onGet = func(key []byte) {
		if bytes.Equal(key, root) {
			return
		}
		blockOnce.Do(func() {
			close(walkStarted)
			<-releaseWalk
		})
	}
	proofDone := make(chan error, 1)
	go func() {
		_, errProof := adb.GetMerkleProofAtRoot(root, address)
		proofDone <- errProof
	}()
	select {
	case <-walkStarted:
	case errProof := <-proofDone:
		t.Fatalf("proof ended without reading a child node: %v", errProof)
	}

	// Block processing still commits while the walk is open.
	third, err := adb.Commit()
	require.NoError(t, err)

	// Pruning root must wait for the walk: if it ran now the walk would lose its nodes.
	pruneDone := make(chan struct{})
	go func() {
		adb.PruneTrie(root, data.OldRoot)
		close(pruneDone)
	}()
	require.False(t, adb.TryLockMutPrune(), "prune lock free during the historical proof walk")
	require.Never(t, func() bool {
		select {
		case <-pruneDone:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond, "prune completed during the historical proof walk")

	close(releaseWalk)
	require.NoError(t, <-proofDone)
	<-pruneDone
	db.onGet = nil

	adb.CancelPrune(third, data.NewRoot)
	adb.PruneTrie(next, data.OldRoot)

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				// the root is pruned early on, after which unavailable is the only acceptable failure
				_, errProof := adb.GetMerkleProofAtRoot(root, address)
				if errProof != nil && !errors.Is(errProof, state.ErrStateRootUnavailable) {
					t.Errorf("unexpected proof error: %v", errProof)
					return
				}
			}
		}()
	}

	previous := third
	for i := 0; i < 50; i++ {
		putBalance(t, adb, address, 1)
		next, errCommit := adb.Commit()
		require.NoError(t, errCommit)
		adb.CancelPrune(next, data.NewRoot)
		adb.PruneTrie(previous, data.OldRoot)
		previous = next
	}
	close(done)
	wg.Wait()
}

// A prune a view held back runs from the next CancelPrune, which must wait for a historical
// proof walk of that root as PruneTrie does.
func TestAccountsDB_CancelPruneRunningAHeldPruneWaitsForTheProofWalk(t *testing.T) {
	t.Parallel()

	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
	adb := newPruningProofAccountsDB(t, db, &mock.ProtobufMarshalizerMock{})
	address := viewAddress(1)
	putBalance(t, adb, address, 1)
	putBalance(t, adb, viewAddress(2), 1)
	root, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 2)
	next, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(root)
	require.NoError(t, err)
	finalize(adb, root) // held by the view
	view.Release()

	walkStarted := make(chan struct{})
	releaseWalk := make(chan struct{})
	var blockOnce sync.Once
	db.onGet = func(key []byte) {
		if bytes.Equal(key, root) {
			return
		}
		blockOnce.Do(func() {
			close(walkStarted)
			<-releaseWalk
		})
	}
	proofDone := make(chan error, 1)
	go func() {
		_, errProof := adb.GetMerkleProofAtRoot(root, address)
		proofDone <- errProof
	}()
	select {
	case <-walkStarted:
	case errProof := <-proofDone:
		t.Fatalf("proof ended without reading a child node: %v", errProof)
	}

	cancelDone := make(chan struct{})
	go func() {
		adb.CancelPrune(next, data.NewRoot)
		close(cancelDone)
	}()
	require.Never(t, func() bool {
		select {
		case <-cancelDone:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond, "held prune ran during the historical proof walk")

	close(releaseWalk)
	require.NoError(t, <-proofDone)
	<-cancelDone
	db.onGet = nil

	_, err = db.Get(root)
	require.Error(t, err, "the held prune did not run after the walk")
}

// While a historical proof reads its root node, block processing must still save and
// commit: the read must not hold the live trie's own lock.
func TestAccountsDB_MerkleProofRootReadLeavesLiveTrieFree(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
	adb := newPruningProofAccountsDB(t, db, marshalizer)

	address := make([]byte, 32)
	address[0] = 1
	putBalance(t, adb, address, 1)
	root, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 2)
	_, err = adb.Commit()
	require.NoError(t, err)

	rootRead := make(chan struct{})
	releaseRead := make(chan struct{})
	var blockOnce sync.Once
	db.onGet = func(key []byte) {
		if !bytes.Equal(key, root) {
			return
		}
		blockOnce.Do(func() {
			close(rootRead)
			<-releaseRead
		})
	}
	proofDone := make(chan error, 1)
	go func() {
		_, errProof := adb.GetMerkleProofAtRoot(root, address)
		proofDone <- errProof
	}()
	<-rootRead

	commitDone := make(chan error, 1)
	go func() {
		putBalance(t, adb, address, 3)
		_, errCommit := adb.Commit()
		commitDone <- errCommit
	}()
	select {
	case errCommit := <-commitDone:
		require.NoError(t, errCommit)
	case <-time.After(5 * time.Second):
		close(releaseRead)
		t.Fatal("commit waited on the historical proof's root read")
	}

	close(releaseRead)
	require.NoError(t, <-proofDone)
}

// Past MaxHistoricalProofWalks walks in flight a historical proof fails at once with
// ErrProofBusy, while a live-root proof still runs; slots come back when walks end.
func TestAccountsDB_MerkleProofCapsHistoricalWalks(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
	adb := newPruningProofAccountsDB(t, db, marshalizer)

	address := make([]byte, 32)
	address[0] = 1
	other := make([]byte, 32)
	other[0] = 2
	putBalance(t, adb, address, 1)
	putBalance(t, adb, other, 1)
	root, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, address, 2)
	_, err = adb.Commit()
	require.NoError(t, err)

	// Every walk of root blocks at its first child read until released.
	walkStarted := make(chan struct{})
	releaseWalks := make(chan struct{})
	db.onGet = func(key []byte) {
		if bytes.Equal(key, root) {
			return
		}
		select {
		case <-releaseWalks:
			return
		case walkStarted <- struct{}{}:
		}
		<-releaseWalks
	}

	proofs := make(chan error, state.MaxHistoricalProofWalks)
	for i := 0; i < state.MaxHistoricalProofWalks; i++ {
		go func() {
			_, errProof := adb.GetMerkleProofAtRoot(root, address)
			proofs <- errProof
		}()
	}
	for i := 0; i < state.MaxHistoricalProofWalks; i++ {
		<-walkStarted
	}

	busy := make(chan error, 1)
	go func() {
		_, errBusy := adb.GetMerkleProofAtRoot(root, address)
		busy <- errBusy
	}()
	select {
	case err = <-busy:
		require.ErrorIs(t, err, state.ErrProofBusy)
	case <-time.After(time.Second):
		close(releaseWalks)
		t.Fatal("historical proof past the cap queued instead of failing with ErrProofBusy")
	}
	_, err = adb.GetMerkleProof(address)
	require.NoError(t, err, "a live-root proof must not count against the historical cap")

	close(releaseWalks)
	for i := 0; i < state.MaxHistoricalProofWalks; i++ {
		require.NoError(t, <-proofs)
	}
	_, err = adb.GetMerkleProofAtRoot(root, address)
	require.NoError(t, err)
}
