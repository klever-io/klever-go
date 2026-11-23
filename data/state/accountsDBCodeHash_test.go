package state_test

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
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

// KLC-3001: GetAccountCode is the read the compiled-code warmer uses off the block path.
// Unlike GetExistingAccount it must not load the account's data trie, which would add a trie
// the block never touched to the set the consensus adapter tracks and Commit iterates. And it
// must read the account and its code under one hold of the adapter lock: RecreateTrie swaps
// the main trie under that lock.

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

func TestAccountsDB_GetAccountCodeReturnsTheCode(t *testing.T) {
	t.Parallel()

	adb, address, codeHash := newAccountsDBWithContract(t)

	got, err := adb.GetAccountCode(address)
	require.NoError(t, err)
	require.Equal(t, []byte("contract code"), got)
	require.Equal(t, adb.GetCode(codeHash), got)
}

func TestAccountsDB_GetAccountCodeOfAnAccountWithoutCode(t *testing.T) {
	t.Parallel()

	adb, _, _ := newAccountsDBWithContract(t)

	address := make([]byte, 32)
	address[31] = 8
	account, err := adb.LoadAccount(address)
	require.NoError(t, err)
	require.NoError(t, adb.SaveAccount(account))

	got, err := adb.GetAccountCode(address)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestAccountsDB_GetAccountCodeDoesNotTrackTheDataTrie(t *testing.T) {
	t.Parallel()

	adb, address, _ := newAccountsDBWithContract(t)

	_, err := adb.GetAccountCode(address)
	require.NoError(t, err)
	require.Equal(t, 0, adb.NumTrackedDataTries(), "the code read must not load the data trie")
	require.Equal(t, 0, adb.JournalLen())

	// control: the consensus read does track it, so the assertion above is meaningful
	_, err = adb.GetExistingAccount(address)
	require.NoError(t, err)
	require.Equal(t, 1, adb.NumTrackedDataTries())
}

// TestAccountsDB_GetAccountCodeIsSafeAgainstRecreateTrie must run under -race. Reading the code
// hash and then the code in two separate calls left the second read of adb.mainTrie outside
// the lock RecreateTrie swaps it under, and the race detector reported it on that sequence.
func TestAccountsDB_GetAccountCodeIsSafeAgainstRecreateTrie(t *testing.T) {
	t.Parallel()

	adb, address, _ := newAccountsDBWithContract(t)
	rootHash, err := adb.RootHash()
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 20000; i++ {
			code, errRead := adb.GetAccountCode(address)
			if errRead != nil || len(code) == 0 {
				errs <- fmt.Errorf("iteration %d: code %q, error %v", i, code, errRead)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 20000; i++ {
			errRecreate := adb.RecreateTrie(rootHash)
			if errRecreate != nil {
				errs <- errRecreate
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	close(errs)

	for errLoop := range errs {
		require.NoError(t, errLoop)
	}
}

func TestAccountsDB_GetAccountCodeMissingAccount(t *testing.T) {
	t.Parallel()

	adb, _, _ := newAccountsDBWithContract(t)

	missing := make([]byte, 32)
	missing[0] = 1
	got, err := adb.GetAccountCode(missing)
	require.Nil(t, got)
	require.True(t, errors.Is(err, common.ErrAccNotFound), "got %v", err)
}

func TestAccountsDB_GetAccountCodeEmptyAddress(t *testing.T) {
	t.Parallel()

	adb, _, _ := newAccountsDBWithContract(t)

	got, err := adb.GetAccountCode(nil)
	require.Nil(t, got)
	require.True(t, errors.Is(err, common.ErrNilAddress), "got %v", err)
}

func TestAccountsDB_GetAccountCodeTrieError(t *testing.T) {
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

	got, err := adb.GetAccountCode(make([]byte, 32))
	require.Nil(t, got)
	require.True(t, errors.Is(err, errTrie), "got %v", err)
}

func TestAccountsDB_GetAccountCodeHashReadsOnlyTheAccount(t *testing.T) {
	t.Parallel()

	adb, address, codeHash := newAccountsDBWithContract(t)

	got, err := adb.GetAccountCodeHash(address)
	require.NoError(t, err)
	require.Equal(t, codeHash, got, "the hash the runtime keys the compiled module on")
	require.Equal(t, 0, adb.NumTrackedDataTries())

	_, err = adb.GetAccountCodeHash(append(bytes.Clone(address[:31]), 8))
	require.ErrorIs(t, err, common.ErrAccNotFound)
	_, err = adb.GetAccountCodeHash(nil)
	require.ErrorIs(t, err, common.ErrNilAddress)
}

func TestAccountsDB_WarmUpCodeReadsAreNotMeasured(t *testing.T) {
	t.Parallel()

	adb, address, codeHash := newAccountsDBWithContract(t)
	before := adb.NumCodeLoadsMeasured()

	_, err := adb.GetAccountCode(address)
	require.NoError(t, err)
	_, err = adb.GetAccountCodeHash(address)
	require.NoError(t, err)
	require.Equal(t, before, adb.NumCodeLoadsMeasured(), "the warmer's reads stay out of the block path's code-load stats")

	_ = adb.GetCode(codeHash)
	require.Equal(t, before+1, adb.NumCodeLoadsMeasured(), "GetCode is still measured")
}
