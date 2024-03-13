package sync

import (
	"testing"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncState_NilTrieSyncersShouldErr(t *testing.T) {
	t.Parallel()

	args := ArgsNewSyncAccountsDBsHandler{
		AccountsDBsSyncers: nil,
		ActiveAccountsDBs:  nil,
	}

	triesSyncHandler, err := NewSyncAccountsDBsHandler(args)
	require.Nil(t, triesSyncHandler)
	require.Equal(t, update.ErrNilAccountsDBSyncContainer, err)
}

func TestNewSyncState(t *testing.T) {
	t.Parallel()

	args := ArgsNewSyncAccountsDBsHandler{
		AccountsDBsSyncers: &mock.AccountsDBSyncersStub{
			GetCalled: func(key string) (syncer update.AccountsDBSyncer, err error) {
				return &mock.AccountsDBSyncerStub{}, nil
			},
		},
		ActiveAccountsDBs: make(map[state.AccountsDbIdentifier]state.AccountsAdapter),
	}

	args.ActiveAccountsDBs[state.UserAccountsState] = &cMock.AccountsStub{
		RecreateAllTriesCalled: func(rootHash []byte) (map[string]data.Trie, error) {
			tries := make(map[string]data.Trie)
			tries[string(rootHash)] = &mock.TrieStub{}
			return tries, nil
		},
	}

	args.ActiveAccountsDBs[state.PeerAccountsState] = &cMock.AccountsStub{
		RecreateAllTriesCalled: func(rootHash []byte) (map[string]data.Trie, error) {
			tries := make(map[string]data.Trie)
			tries[string(rootHash)] = &mock.TrieStub{}
			return tries, nil
		},
	}

	triesSyncHandler, err := NewSyncAccountsDBsHandler(args)
	require.Nil(t, err)

	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 1, Epoch: 1, TrieRoot: []byte("metaRootHash"),
			IsEpochStart: true,
		},
	}

	err = triesSyncHandler.SyncTriesFrom(metaBlock)
	require.Nil(t, err)

	tries, err := triesSyncHandler.GetTries()
	assert.NotNil(t, tries)
	assert.Nil(t, err)
}
