package hostCore_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)
var testOwnerAddress, _ = addressConverter.Decode("klv10gq6xsegedacd084vmpr2xus950j3d6lhqjfe8ue2xkmfwtkzavqnqhz99")
var testToAddress, _ = addressConverter.Decode("klv15zssmvht00ugvge5le9n885kahc5ykxzvmxx6xwz5ya2an562yyssfa0c5")

var marshalizer = marshal.NewProtoMarshalizer()

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

func loadUsersAccounts(kappDB state.AccountsAdapter, accCacher state.AccountsCacher) {
	ownerAcc, _ := accCacher.LoadUser(testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc, _ := accCacher.LoadUser(testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	kdaKapp, _ := accCacher.LoadKApp(kapps.KDAKAppAddress)
	kappDB.SaveAccount(kdaKapp)

	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	klv := kapps.KDAData{
		ID:                kdautils.KLVIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER"),
		Ticker:            kdautils.KLVIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	klvData, _ := marshalizer.Marshal(&klv)

	_ = kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
}

func createFullArgumentsForKAppsProcessingMemory() state.AccountsCacher {
	hasher := &sha256.Sha256{}
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

	loadUsersAccounts(kappAccountsDB, accCacher)

	return accCacher
}

func TestExecuteKDATransfer(t *testing.T) {
	hostParams := makeHostParameters()

	accCacher := createFullArgumentsForKAppsProcessingMemory()

	mockWorld := worldmock.NewMockWorld()
	mockWorld.AccountsCacher = accCacher

	_, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	err = accCacher.SaveAll()
	require.NoError(t, err)

	mockWorld.InitBuiltinFunctions(hostParams.GasSchedule, hostParams.ForkController)

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParams)
	require.NoError(t, err)

	value := int64(100)
	vmHost.SetRuntimeContext(&contextmock.RuntimeContextMock{})
	transfers := []*vmcommon.KDATransfer{
		{
			KDATokenName: kdautils.KLVIdentifier,
			KDATokenType: uint32(core.Fungible),
			KDAValue:     big.NewInt(value),
		},
		{
			KDATokenName: kdautils.KLVIdentifier,
			KDATokenType: uint32(core.Fungible),
			KDAValue:     big.NewInt(value),
		},
	}

	runtime := vmHost.Runtime()
	metering := vmHost.Metering()

	oneTransferCost := metering.GasSchedule().BuiltInCost.Transfer
	gasProvided := oneTransferCost*uint64(len(transfers)) + 1
	input := &vmcommon.ContractCallInput{
		RecipientAddr: testToAddress,
		VMInput: vmcommon.VMInput{
			CallerAddr:  testOwnerAddress,
			GasProvided: gasProvided,
		},
	}

	runtime.SetVMInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)

	args := &vmhost.KDATransfersArgs{
		Sender:         testOwnerAddress,
		Destination:    vmhost.ParentAddress,
		OriginalCaller: runtime.GetOriginalCallerAddress(),
		Transfers:      transfers,
	}

	vmOutput, gasConsumed, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, uint64(2), gasConsumed)
}
