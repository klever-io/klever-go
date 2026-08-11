package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

func kappLeafWithRootHash(t *testing.T, rootHash []byte) data.KeyValueHolder {
	t.Helper()

	account := state.NewEmptyKAppAccount()
	account.RootHash = rootHash
	raw, err := marshal.NewProtoMarshalizer().Marshal(account)
	require.NoError(t, err)

	return &mock.KeyValueHolderStub{ValueCalled: func() []byte { return raw }}
}

func TestKappAccountsSyncer_FindAllAccountRootHashesReportsATruncatedWalk(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestKappAccountsSyncer(t, 2)

	expectedErr := errors.New("trie iteration failed")
	mainTrie := &mock.TrieStub{
		RootCalled: func() ([]byte, error) { return []byte("root"), nil },
		GetAllLeavesOnChannelCalled: func(_ []byte) (*data.TrieIteratorChannels, error) {
			return data.NewFailedTrieIteratorChannels(expectedErr, kappLeafWithRootHash(t, []byte("datatrie1"))), nil
		},
	}

	// A partial list would leave the rest of the state unsynced without anyone noticing.
	rootHashes, err := syncer.findAllAccountRootHashes(mainTrie, context.Background())
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, rootHashes)
}

func TestKappAccountsSyncer_FindAllAccountRootHashesReturnsEveryDataTrie(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestKappAccountsSyncer(t, 2)

	mainTrie := &mock.TrieStub{
		RootCalled: func() ([]byte, error) { return []byte("root"), nil },
		GetAllLeavesOnChannelCalled: func(_ []byte) (*data.TrieIteratorChannels, error) {
			return data.NewCompletedTrieIteratorChannels(
				kappLeafWithRootHash(t, []byte("datatrie1")),
				kappLeafWithRootHash(t, nil),
			), nil
		},
	}

	rootHashes, err := syncer.findAllAccountRootHashes(mainTrie, context.Background())
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("datatrie1")}, rootHashes)
}
