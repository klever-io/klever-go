package state_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
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

	storageWithRoot := func(t *testing.T, root []byte) data.StorageManager {
		db := mock.NewMemDbMock()
		require.NoError(t, db.Put(root, []byte("node")))
		sm, err := trie.NewTrieStorageManagerWithoutPruning(db)
		require.NoError(t, err)
		return sm
	}

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

	t.Run("root not in main db or no storage manager", func(t *testing.T) {
		t.Parallel()
		tr := &mock.TrieStub{
			RootCalled: func() ([]byte, error) { return live, nil },
		}
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		tr.GetStorageManagerCalled = func() data.StorageManager { return nil }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	})

	t.Run("recreate failures", func(t *testing.T) {
		t.Parallel()
		sm := storageWithRoot(t, other)
		tr := &mock.TrieStub{
			RootCalled:              func() ([]byte, error) { return live, nil },
			GetStorageManagerCalled: func() data.StorageManager { return sm },
			RecreateCalled:          func([]byte) (data.Trie, error) { return nil, trie.ErrHashNotFound },
		}
		adb := newStubProofAccountsDB(t, tr)
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		tr.RecreateCalled = func([]byte) (data.Trie, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)

		tr.RecreateCalled = func([]byte) (data.Trie, error) { return nil, nil }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)
	})

	t.Run("recreated trie root mismatch or error", func(t *testing.T) {
		t.Parallel()
		sm := storageWithRoot(t, other)
		recreated := &mock.TrieStub{RootCalled: func() ([]byte, error) { return []byte("wrong"), nil }}
		adb := newStubProofAccountsDB(t, &mock.TrieStub{
			RootCalled:              func() ([]byte, error) { return live, nil },
			GetStorageManagerCalled: func() data.StorageManager { return sm },
			RecreateCalled:          func([]byte) (data.Trie, error) { return recreated, nil },
		})
		_, err := adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, state.ErrStateRootUnavailable)

		recreated.RootCalled = func() ([]byte, error) { return nil, errBoom }
		_, err = adb.GetMerkleProofAtRoot(other, key)
		require.ErrorIs(t, err, errBoom)
	})
}
