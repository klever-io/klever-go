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

// KLC-3001: GetAccountCodeHash is the read the compiled-code warmer uses off the block path.
// Unlike GetExistingAccount it must not load the account's data trie, which would add a trie
// the block never touched to the set the consensus adapter tracks and Commit iterates.

func newAccountsDBWithContract(t *testing.T) (*state.AccountsDB, []byte, []byte) {
	t.Helper()

	marshalizer := &mock.MarshalizerMock{}
	hasher := mock.HasherMock{}
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hasher, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hasher, marshalizer, factory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)

	address := make([]byte, 32)
	address[31] = 7
	code := []byte("contract code")

	account, err := adb.LoadAccount(address)
	require.NoError(t, err)
	userAccount, ok := account.(state.UserAccountHandler)
	require.True(t, ok)
	userAccount.SetCode(code)
	require.NoError(t, userAccount.SaveKeyValue([]byte("key"), []byte("value")))
	require.NoError(t, adb.SaveAccount(userAccount))
	_, err = adb.Commit()
	require.NoError(t, err)
	require.Equal(t, 0, adb.NumTrackedDataTries(), "Commit leaves no tracked data tries")

	return adb, address, hasher.Compute(string(code))
}

func TestAccountsDB_GetAccountCodeHashReturnsTheCodeHash(t *testing.T) {
	t.Parallel()

	adb, address, codeHash := newAccountsDBWithContract(t)

	got, err := adb.GetAccountCodeHash(address)
	require.NoError(t, err)
	require.Equal(t, codeHash, got)
	require.Equal(t, []byte("contract code"), adb.GetCode(got))
}

func TestAccountsDB_GetAccountCodeHashDoesNotTrackTheDataTrie(t *testing.T) {
	t.Parallel()

	adb, address, _ := newAccountsDBWithContract(t)

	_, err := adb.GetAccountCodeHash(address)
	require.NoError(t, err)
	require.Equal(t, 0, adb.NumTrackedDataTries(), "the code hash read must not load the data trie")
	require.Equal(t, 0, adb.JournalLen())

	// control: the consensus read does track it, so the assertion above is meaningful
	_, err = adb.GetExistingAccount(address)
	require.NoError(t, err)
	require.Equal(t, 1, adb.NumTrackedDataTries())
}

func TestAccountsDB_GetAccountCodeHashMissingAccount(t *testing.T) {
	t.Parallel()

	adb, _, _ := newAccountsDBWithContract(t)

	missing := make([]byte, 32)
	missing[0] = 1
	got, err := adb.GetAccountCodeHash(missing)
	require.Nil(t, got)
	require.True(t, errors.Is(err, common.ErrAccNotFound), "got %v", err)
}

func TestAccountsDB_GetAccountCodeHashEmptyAddress(t *testing.T) {
	t.Parallel()

	adb, _, _ := newAccountsDBWithContract(t)

	got, err := adb.GetAccountCodeHash(nil)
	require.Nil(t, got)
	require.True(t, errors.Is(err, common.ErrNilAddress), "got %v", err)
}

func TestAccountsDB_GetAccountCodeHashTrieError(t *testing.T) {
	t.Parallel()

	errTrie := errors.New("trie failure")
	trieStub := &mock.TrieStub{
		GetCalled: func(_ []byte) ([]byte, error) { return nil, errTrie },
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)

	got, err := adb.GetAccountCodeHash(make([]byte, 32))
	require.Nil(t, got)
	require.True(t, errors.Is(err, errTrie), "got %v", err)
}
