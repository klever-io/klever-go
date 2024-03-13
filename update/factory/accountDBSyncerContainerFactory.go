package factory

import (
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/syncer"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/update"
	containers "github.com/klever-io/klever-go/update/container"
	"github.com/klever-io/klever-go/update/genesis"
)

// ArgsNewAccountsDBSyncersContainerFactory defines the arguments needed to create accounts DB syncers container
type ArgsNewAccountsDBSyncersContainerFactory struct {
	TrieCacher                storage.Cacher
	RequestHandler            update.RequestHandler
	Hasher                    hashing.Hasher
	Marshalizer               marshal.Marshalizer
	TrieStorageManager        data.StorageManager
	TimoutGettingTrieNode     time.Duration
	MaxTrieLevelInMemory      uint
	NumConcurrentTrieSyncers  int
	MaxHardCapForMissingNodes int
}

type accountDBSyncersContainerFactory struct {
	trieCacher                storage.Cacher
	requestHandler            update.RequestHandler
	container                 update.AccountsDBSyncContainer
	hasher                    hashing.Hasher
	marshalizer               marshal.Marshalizer
	timeoutGettingTrieNode    time.Duration
	trieStorageManager        data.StorageManager
	maxTrieLevelinMemory      uint
	numConcurrentTrieSyncers  int
	maxHardCapForMissingNodes int
}

// NewAccountsDBSContainerFactory creates a factory for trie syncers container
func NewAccountsDBSContainerFactory(args ArgsNewAccountsDBSyncersContainerFactory) (*accountDBSyncersContainerFactory, error) {
	if check.IfNil(args.RequestHandler) {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(args.TrieCacher) {
		return nil, common.ErrNilCacher
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.TrieStorageManager) {
		return nil, update.ErrNilStorageManager
	}
	if args.NumConcurrentTrieSyncers < 1 {
		return nil, update.ErrInvalidNumConcurrentTrieSyncers
	}
	if args.MaxHardCapForMissingNodes < 1 {
		return nil, update.ErrInvalidMaxHardCapForMissingNodes
	}

	t := &accountDBSyncersContainerFactory{
		trieCacher:                args.TrieCacher,
		requestHandler:            args.RequestHandler,
		hasher:                    args.Hasher,
		marshalizer:               args.Marshalizer,
		trieStorageManager:        args.TrieStorageManager,
		timeoutGettingTrieNode:    args.TimoutGettingTrieNode,
		maxTrieLevelinMemory:      args.MaxTrieLevelInMemory,
		numConcurrentTrieSyncers:  args.NumConcurrentTrieSyncers,
		maxHardCapForMissingNodes: args.MaxHardCapForMissingNodes,
	}

	return t, nil
}

// Create creates all the needed syncers and returns the container
func (a *accountDBSyncersContainerFactory) Create() (update.AccountsDBSyncContainer, error) {
	a.container = containers.NewAccountsDBSyncersContainer()

	err := a.createUserAccountsSyncer()
	if err != nil {
		return nil, err
	}

	err = a.createValidatorAccountsSyncer()
	if err != nil {
		return nil, err
	}

	return a.container, nil
}

func (a *accountDBSyncersContainerFactory) createUserAccountsSyncer() error {
	thr, err := throttler.NewNumGoRoutinesThrottler(int32(a.numConcurrentTrieSyncers))
	if err != nil {
		return err
	}

	args := syncer.ArgsNewUserAccountsSyncer{
		ArgsNewBaseAccountsSyncer: syncer.ArgsNewBaseAccountsSyncer{
			Hasher:                    a.hasher,
			Marshalizer:               a.marshalizer,
			TrieStorageManager:        a.trieStorageManager,
			RequestHandler:            a.requestHandler,
			Timeout:                   a.timeoutGettingTrieNode,
			Cacher:                    a.trieCacher,
			MaxTrieLevelInMemory:      a.maxTrieLevelinMemory,
			MaxHardCapForMissingNodes: a.maxHardCapForMissingNodes,
		},
		Throttler: thr,
	}
	accountSyncer, err := syncer.NewUserAccountsSyncer(args)
	if err != nil {
		return err
	}
	trieId := genesis.CreateTrieIdentifier(genesis.UserAccount)

	return a.container.Add(trieId, accountSyncer)
}

func (a *accountDBSyncersContainerFactory) createValidatorAccountsSyncer() error {
	args := syncer.ArgsNewValidatorAccountsSyncer{
		ArgsNewBaseAccountsSyncer: syncer.ArgsNewBaseAccountsSyncer{
			Hasher:                    a.hasher,
			Marshalizer:               a.marshalizer,
			TrieStorageManager:        a.trieStorageManager,
			RequestHandler:            a.requestHandler,
			Timeout:                   a.timeoutGettingTrieNode,
			Cacher:                    a.trieCacher,
			MaxTrieLevelInMemory:      a.maxTrieLevelinMemory,
			MaxHardCapForMissingNodes: a.maxHardCapForMissingNodes,
		},
	}
	accountSyncer, err := syncer.NewValidatorAccountsSyncer(args)
	if err != nil {
		return err
	}
	trieId := genesis.CreateTrieIdentifier(genesis.ValidatorAccount)

	return a.container.Add(trieId, accountSyncer)
}

// IsInterfaceNil returns true if the underlying object is nil
func (a *accountDBSyncersContainerFactory) IsInterfaceNil() bool {
	return a == nil
}
