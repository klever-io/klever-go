package factory

import (
	"path"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/genesis"
)

// ArgsNewDataTrieFactory is the argument structure for the new data trie factory
type ArgsNewDataTrieFactory struct {
	StorageConfig        config.StorageConfig
	SyncFolder           string
	Marshalizer          marshal.Marshalizer
	Hasher               hashing.Hasher
	MaxTrieLevelInMemory uint
}

type dataTrieFactory struct {
	trieStorage          data.StorageManager
	marshalizer          marshal.Marshalizer
	hasher               hashing.Hasher
	maxTrieLevelInMemory uint
}

// NewDataTrieFactory creates a data trie factory
func NewDataTrieFactory(args ArgsNewDataTrieFactory) (*dataTrieFactory, error) {
	if len(args.SyncFolder) < 2 {
		return nil, update.ErrInvalidFolderName
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}

	dbConfig := storageFactory.GetDBFromConfig(args.StorageConfig.DB)
	dbConfig.FilePath = path.Join(args.SyncFolder, args.StorageConfig.DB.FilePath)
	accountsTrieStorage, err := storageUnit.NewStorageUnitFromConf(
		storageFactory.GetCacherFromConfig(args.StorageConfig.Cache),
		dbConfig,
		storageFactory.GetBloomFromConfig(args.StorageConfig.Bloom),
	)
	if err != nil {
		return nil, err
	}

	trieStorage, err := trie.NewTrieStorageManagerWithoutPruning(accountsTrieStorage)
	if err != nil {
		return nil, err
	}

	d := &dataTrieFactory{
		trieStorage:          trieStorage,
		marshalizer:          args.Marshalizer,
		hasher:               args.Hasher,
		maxTrieLevelInMemory: args.MaxTrieLevelInMemory,
	}

	return d, nil
}

// TrieStorageManager returns trie storage manager
func (d *dataTrieFactory) TrieStorageManager() data.StorageManager {
	return d.trieStorage
}

// Create creates a TriesHolder container to hold all the states
func (d *dataTrieFactory) Create() (state.TriesHolder, error) {
	container := state.NewDataTriesHolder()

	err := d.createAndAddOneTrie(genesis.UserAccount, container)
	if err != nil {
		return nil, err
	}

	err = d.createAndAddOneTrie(genesis.ValidatorAccount, container)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func (d *dataTrieFactory) createAndAddOneTrie(accType genesis.Type, container state.TriesHolder) error {
	dataTrie, err := trie.NewTrie(d.trieStorage, d.marshalizer, d.hasher, d.maxTrieLevelInMemory)
	if err != nil {
		return err
	}

	trieId := genesis.CreateTrieIdentifier(accType)
	container.Put([]byte(trieId), dataTrie)

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (d *dataTrieFactory) IsInterfaceNil() bool {
	return d == nil
}
