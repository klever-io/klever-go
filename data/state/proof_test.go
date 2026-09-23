package state_test

import (
	"bytes"
	"errors"
	"testing"

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
			RootCalled:               func() ([]byte, error) { return live, nil },
			RecreateFromMainDbCalled: func([]byte) (data.Trie, error) { return nil, trie.ErrHashNotFound },
			RecreateCalled: func([]byte) (data.Trie, error) {
				t.Fatal("proofs must not use the snapshot-capable Recreate")
				return nil, nil
			},
		}
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		tr.RecreateFromMainDbCalled = func([]byte) (data.Trie, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)

		tr.RecreateFromMainDbCalled = func([]byte) (data.Trie, error) { return nil, nil }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	})

	t.Run("recreated trie root mismatch or error", func(t *testing.T) {
		t.Parallel()
		recreated := &mock.TrieStub{RootCalled: func() ([]byte, error) { return []byte("wrong"), nil }}
		adb := newStubProofAccountsDB(t, &mock.TrieStub{
			RootCalled:               func() ([]byte, error) { return live, nil },
			RecreateFromMainDbCalled: func([]byte) (data.Trie, error) { return recreated, nil },
		})
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		recreated.RootCalled = func() ([]byte, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)
	})
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

func TestAccountsDB_MerkleProofHoldsLockAgainstPruning(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	hsh := &mock.HasherMock{}
	db := &hookDb{MemDbMock: mock.NewMemDbMock()}
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

	// Child nodes are read only while the proof walks root1. PruneTrie takes the same
	// accounts lock, so the lock must still be held at every such read.
	walkReads := 0
	db.onGet = func(key []byte) {
		if bytes.Equal(key, root1) {
			return
		}
		walkReads++
		require.False(t, adb.TryLockMutOp(), "accounts lock released during the proof walk")
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
