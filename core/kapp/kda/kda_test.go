package kda_test

import (
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
)

var sender = "sender"
var defaultSender []byte = makeAddress(sender)

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func createMemUnit() storage.Storer {
	capacity := uint32(10)
	shards := uint32(1)
	sizeInBytes := uint64(0)
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: capacity, Shards: shards, SizeInBytes: sizeInBytes})
	persist, _ := memorydb.NewlruDB(100000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)

	return unit
}

func createAccountsDB(
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	accountFactory state.AccountFactory,
	trieStorageManager data.StorageManager,
) *state.AccountsDB {
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	return adb
}

func loadKAppAccount(kappsDB state.AccountsAdapter, address []byte) state.KAppAccountHandler {
	acc, _ := kappsDB.LoadAccount(address)
	kappAcc := acc.(state.KAppAccountHandler)

	return kappAcc
}

func loadUserAccount(userDB state.AccountsAdapter, address []byte) state.UserAccountHandler {
	acc, _ := userDB.LoadAccount(address)
	user := acc.(state.UserAccountHandler)

	return user
}

func createFullArgumentsForKAppsProcessingMemory() (core.ForkController, kapps.ActiveProposalController, state.AccountsCacher) {
	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)

	trieFactoryManagerAcc, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	trieFactoryManagerKApp, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	trieFactoryManagerPeer, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())

	userAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewAccountCreator(), trieFactoryManagerAcc)
	kappAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewKAppAccountCreator(), trieFactoryManagerKApp)
	peerAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewPeerAccountCreator(), trieFactoryManagerPeer)

	accCacher, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: userAccountsDB,
			Kapps:    kappAccountsDB,
			Peers:    peerAccountsDB,
		},
	)
	accCacher.ResetAll(true)

	// add funds to default sender
	acc := loadUserAccount(userAccountsDB, defaultSender)
	_ = acc.AddToBalance(100000000000, nil, true, nil)

	// create Default KApps
	kdaKapp := loadKAppAccount(kappAccountsDB, kapps.KDAKAppAddress)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(nil, nil)
	stakingKapp := loadKAppAccount(kappAccountsDB, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(kappAccountsDB, kapps.ProposalKAppAddress)
	atcivePC, _ := proposalKapp.StartProposalsKApp(forkController)

	_ = kappAccountsDB.SaveAccounts(kdaKapp, stakingKapp, proposalKapp)

	return forkController, atcivePC, accCacher
}

// default blockchain for tests
func createMockControllers() (kapp.KAppController, error) {
	hasher := &sha256.Sha256{}
	forkController, atcivePC, accCacher := createFullArgumentsForKAppsProcessingMemory()
	marshalizerMock := &mock.ProtoMarshalizerMock{}

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         hasher,
		Marshalizer:    marshalizerMock,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    &mock.RatingsInfoMock{},
	}

	kc, err := kappcontroller.NewKappController(argsKapp)
	if err != nil {
		return nil, err
	}

	err = kc.InitKApps(accCacher)
	if err != nil {
		return nil, err
	}

	err = kc.SetProposalController(atcivePC)
	if err != nil {
		return nil, err
	}

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: defaultSender,
		ContractID:     0,
		Block:          &block.Block{},
	})

	kc.SetCurrentKAppContext(ctx)
	return kc, nil
}
