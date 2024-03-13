package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	trieFactory "github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// TriesComponentsFactoryArgs holds the arguments needed for creating a tries components factory
type TriesComponentsFactoryArgs struct {
	Marshalizer marshal.Marshalizer
	Hasher      hashing.Hasher
	PathManager storage.PathManagerHandler
	Config      config.Config
}

type triesComponentsFactory struct {
	marshalizer marshal.Marshalizer
	hasher      hashing.Hasher
	pathManager storage.PathManagerHandler
	config      config.Config
}

// NewTriesComponentsFactory return a new instance of tries components factory
func NewTriesComponentsFactory(args TriesComponentsFactoryArgs) (*triesComponentsFactory, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.PathManager) {
		return nil, ErrNilPathManager
	}

	return &triesComponentsFactory{
		config:      args.Config,
		marshalizer: args.Marshalizer,
		hasher:      args.Hasher,
		pathManager: args.PathManager,
	}, nil
}

// Create creates and returns
func (tcf *triesComponentsFactory) Create() (*TriesComponents, error) {
	trieContainer := state.NewDataTriesHolder()
	trieFactoryArgs := trieFactory.TrieFactoryArgs{
		EvictionWaitingListCfg:   tcf.config.EvictionWaitingList,
		SnapshotDbCfg:            tcf.config.TrieSnapshotDB,
		Marshalizer:              tcf.marshalizer,
		Hasher:                   tcf.hasher,
		PathManager:              tcf.pathManager,
		TrieStorageManagerConfig: tcf.config.TrieStorageManagerConfig,
	}

	trieFactoryObj, err := trieFactory.NewTrieFactory(trieFactoryArgs)
	if err != nil {
		return nil, err
	}

	trieStorageManagers := make(map[string]data.StorageManager)
	userStorageManager, userAccountTrie, err := trieFactoryObj.Create(
		tcf.config.AccountsTrieStorage,
		tcf.config.StateTriesConfig.AccountsStatePruningEnabled,
		tcf.config.StateTriesConfig.MaxStateTrieLevelInMemory,
	)
	if err != nil {
		return nil, err
	}
	trieContainer.Put([]byte(trieFactory.UserAccountTrie), userAccountTrie)
	trieStorageManagers[trieFactory.UserAccountTrie] = userStorageManager

	peerStorageManager, peerAccountsTrie, err := trieFactoryObj.Create(
		tcf.config.PeerAccountsTrieStorage,
		tcf.config.StateTriesConfig.PeerStatePruningEnabled,
		tcf.config.StateTriesConfig.MaxPeerTrieLevelInMemory,
	)
	if err != nil {
		return nil, err
	}
	trieContainer.Put([]byte(trieFactory.PeerAccountTrie), peerAccountsTrie)
	trieStorageManagers[trieFactory.PeerAccountTrie] = peerStorageManager

	kappStorageManager, kappAccountsTrie, err := trieFactoryObj.Create(
		tcf.config.KAppAccountsTrieStorage,
		tcf.config.StateTriesConfig.KAppStatePruningEnabled,
		tcf.config.StateTriesConfig.MaxKAppTrieLevelInMemory,
	)
	if err != nil {
		return nil, err
	}
	trieContainer.Put([]byte(trieFactory.KAppAccountTrie), kappAccountsTrie)
	trieStorageManagers[trieFactory.KAppAccountTrie] = kappStorageManager

	return &TriesComponents{
		TriesContainer:      trieContainer,
		TrieStorageManagers: trieStorageManagers,
	}, nil
}
