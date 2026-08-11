package factory

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/require"
)

var errTestGenesis = errors.New("test error")

func TestIndexGenesisAccounts_RootHashError(t *testing.T) {
	t.Parallel()

	accounts := &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return nil, errTestGenesis
		},
	}

	err := indexGenesisAccounts(0, accounts, &mock.EventsProcessorStub{}, &mock.MarshalizerMock{})
	require.Error(t, err)
}

func TestIndexGenesisAccounts_GetAllLeavesError(t *testing.T) {
	t.Parallel()

	accounts := &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return []byte("roothash"), nil
		},
		GetAllLeavesCalled: func(rootHash []byte) (*data.TrieIteratorChannels, error) {
			return nil, errTestGenesis
		},
	}

	err := indexGenesisAccounts(0, accounts, &mock.EventsProcessorStub{}, &mock.MarshalizerMock{})
	require.Error(t, err)
}

func TestIndexGenesisAccounts_CallsSaveAccounts(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	acc, err := state.NewUserAccount([]byte("test-address-1234"))
	require.NoError(t, err)
	accBytes, err := marshalizer.Marshal(acc)
	require.NoError(t, err)

	leaves := data.NewCompletedTrieIteratorChannels(keyValStorage.NewKeyValStorage([]byte("test-address-1234"), accBytes))

	accounts := &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return []byte("roothash"), nil
		},
		GetAllLeavesCalled: func(rootHash []byte) (*data.TrieIteratorChannels, error) {
			return leaves, nil
		},
	}

	var saveAccountsCalled int32
	eventsProc := &mock.EventsProcessorStub{
		SaveAccountsCalled: func(blockTimestamp int64, accs []state.UserAccountHandler) {
			atomic.AddInt32(&saveAccountsCalled, 1)
			require.Equal(t, int64(100), blockTimestamp)
			require.Len(t, accs, 1)
		},
	}

	err = indexGenesisAccounts(100, accounts, eventsProc, marshalizer)
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&saveAccountsCalled))
}

func TestIndexGenesisAccounts_TruncatedWalkIsNotIndexed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	acc, _ := state.NewUserAccount([]byte("test-address-1234"))
	accBytes, _ := marshalizer.Marshal(acc)

	accounts := &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return []byte("roothash"), nil
		},
		GetAllLeavesCalled: func(rootHash []byte) (*data.TrieIteratorChannels, error) {
			return data.NewFailedTrieIteratorChannels(
				errTestGenesis,
				keyValStorage.NewKeyValStorage([]byte("test-address-1234"), accBytes),
			), nil
		},
	}

	var saveAccountsCalled int32
	eventsProc := &mock.EventsProcessorStub{
		SaveAccountsCalled: func(_ int64, _ []state.UserAccountHandler) {
			atomic.AddInt32(&saveAccountsCalled, 1)
		},
	}

	err := indexGenesisAccounts(100, accounts, eventsProc, marshalizer)
	require.ErrorIs(t, err, errTestGenesis)
	require.Zero(t, atomic.LoadInt32(&saveAccountsCalled))
}

func TestIndexGenesisAccounts_SkipsBadLeaves(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	acc, _ := state.NewUserAccount([]byte("good-address-12345"))
	accBytes, _ := marshalizer.Marshal(acc)
	leaves := data.NewCompletedTrieIteratorChannels(
		keyValStorage.NewKeyValStorage([]byte("bad-address"), []byte("invalid-data")),
		keyValStorage.NewKeyValStorage([]byte("good-address-12345"), accBytes),
	)

	accounts := &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return []byte("roothash"), nil
		},
		GetAllLeavesCalled: func(rootHash []byte) (*data.TrieIteratorChannels, error) {
			return leaves, nil
		},
	}

	var savedAccounts []state.UserAccountHandler
	eventsProc := &mock.EventsProcessorStub{
		SaveAccountsCalled: func(blockTimestamp int64, accs []state.UserAccountHandler) {
			savedAccounts = accs
		},
	}

	err := indexGenesisAccounts(100, accounts, eventsProc, marshalizer)
	require.NoError(t, err)
	require.Len(t, savedAccounts, 1)
}
