package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
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

	current, err := adb.GetMerkleProof(address)
	require.NoError(t, err)
	require.Equal(t, dirtyRoot, current.RootHash)
	require.NotEmpty(t, current.Proof)
	require.NotEmpty(t, current.Value)

	historical, err := adb.GetMerkleProofAtRoot(root1, address)
	require.NoError(t, err)
	require.Equal(t, root1, historical.RootHash)
	require.NotEqual(t, current.Value, historical.Value)

	ok, err := adb.VerifyMerkleProof(root1, address, historical.Proof)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = adb.VerifyMerkleProof(dirtyRoot, address, current.Proof)
	require.NoError(t, err)
	require.True(t, ok)

	// A proof built for one root must not verify under the other.
	ok, err = adb.VerifyMerkleProof(root1, address, current.Proof)
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
	require.Equal(t, int64(15), acc.(state.UserAccountHandler).GetBalance(nil, false))
}

func TestAccountsDB_MerkleProofMissingRootAndAccount(t *testing.T) {
	t.Parallel()

	adb := newProofAccountsDB(t)
	address := make([]byte, 32)
	address[0] = 9
	putBalance(t, adb, address, 1)
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
