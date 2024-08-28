package process

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	factoryState "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/genesis/parsing"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/require"
)

// FIXME improve code coverage of this package
func createMockArgument(
	t *testing.T,
	genesisFilename string,
	initialNodes genesis.InitialNodesHandler,
	entireSupply int64,
) ArgsGenesisBlockCreator {

	memDBMock := mock.NewMemDbMock()
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(memDBMock)

	trieStorageManagers := make(map[string]data.StorageManager)
	trieStorageManagers[factory.UserAccountTrie] = storageManager
	trieStorageManagers[factory.PeerAccountTrie] = storageManager
	trieStorageManagers[factory.KAppAccountTrie] = storageManager

	arg := ArgsGenesisBlockCreator{
		GenesisTime:              0,
		StartEpochNum:            0,
		PubkeyConv:               mock.NewPubkeyConverterMock(32),
		Blkc:                     &mock.BlockChainStub{},
		Marshalizer:              &mock.MarshalizerMock{},
		SignMarshalizer:          &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		Uint64ByteSliceConverter: &mock.Uint64ByteSliceConverterMock{},
		DataPool:                 mock.NewPoolsHolderMock(),
		TrieStorageManagers:      trieStorageManagers,
		BlockSignKeyGen:          &mock.KeyGenMock{},
	}

	var err error
	arg.Accounts, err = createAccountAdapter(
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		factoryState.NewAccountCreator(),
		trieStorageManagers[factory.UserAccountTrie],
	)
	require.Nil(t, err)

	arg.PeerAccounts = &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return make([]byte, 0), nil
		},
		CommitCalled: func() ([]byte, error) {
			return make([]byte, 0), nil
		},
		SaveAccountCalled: func(account state.AccountHandler) error {
			return nil
		},
		LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return state.NewEmptyPeerAccount(), nil
		},
	}

	arg.KAppAccounts = &mock.AccountsStub{
		RootHashCalled: func() ([]byte, error) {
			return make([]byte, 0), nil
		},
		CommitCalled: func() ([]byte, error) {
			return make([]byte, 0), nil
		},
		SaveAccountCalled: func(account state.AccountHandler) error {
			return nil
		},
		LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return state.NewEmptyPeerAccount(), nil
		},
	}

	arg.Store = &cMock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return cMock.NewStorerMock("", 0)
		},
	}

	arg.AccountsParser, err = parsing.NewAccountsParser(
		genesisFilename,
		arg.PubkeyConv,
		&cMock.KeyGeneratorStub{},
	)
	require.Nil(t, err)

	arg.InitialNodesSetup = initialNodes

	return arg
}
