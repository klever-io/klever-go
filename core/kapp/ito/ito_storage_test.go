package ito_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sender = "sender"
var defaultSender []byte = makeAddress(sender)
var marshalizer = &marshal.ProtoMarshalizer{}

var defaultTicker []byte = []byte("KDA")
var defaultAssetID []byte = []byte("KDA-3W0I")

var defaultWhitelistLen = int32(2)

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

func createFullArgumentsForKAppsProcessingMemory(enableEpochs config.EnableEpochs) (core.ForkController, kapps.ActiveProposalController, state.AccountsCacher) {
	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(
		enableEpochs,
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

	_ = kappAccountsDB.SaveAccount(kdaKapp)
	_ = kappAccountsDB.SaveAccount(stakingKapp)
	_ = kappAccountsDB.SaveAccount(proposalKapp)

	return forkController, atcivePC, accCacher
}

// default blockchain for tests
func createMockControllers(enableEpochs config.EnableEpochs) (kapp.KAppController, error) {
	hasher := &sha256.Sha256{}
	forkController, atcivePC, accCacher := createFullArgumentsForKAppsProcessingMemory(enableEpochs)
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

func createAsset(
	t *testing.T,
	kc kapp.KAppController,
	ticker []byte,
	assetType transaction.CreateAssetContract_EnumAssetType,
	precision uint32,
	initialSupply int64,
	maxSupply int64,
) {
	tc := &transaction.CreateAssetContract{
		Type:          assetType,
		Name:          ticker,
		Ticker:        ticker,
		OwnerAddress:  defaultSender,
		Precision:     precision,
		InitialSupply: initialSupply,
		MaxSupply:     maxSupply,
		Properties: &transaction.PropertiesInfo{
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanMint:        true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
			LimitTransfer:  false,
		},
	}

	_, err := kc.GetKDAKApp().Create(defaultSender, tc)
	require.NoError(t, err)
}

func createDefaultITO(
	t *testing.T,
	kc kapp.KAppController,
	currency string,
	assetId []byte,
) {
	packs := make(map[string]*transaction.PackInfo, 0)

	packsAsset := make([]*transaction.PackItem, 0)
	packsAsset = append(packsAsset, &transaction.PackItem{
		Amount: 1,
		Price:  1,
	})
	packsAsset = append(packsAsset, &transaction.PackItem{
		Amount: 100,
		Price:  100,
	})
	packs[currency] = &transaction.PackInfo{
		Packs: packsAsset,
	}

	other := makeAddress("other")
	whitelist := make(map[string]*transaction.WhitelistInfo, 0)

	whitelist[hex.EncodeToString(defaultSender)] = &transaction.WhitelistInfo{
		Limit: 1000000000,
	}
	whitelist[hex.EncodeToString(other)] = &transaction.WhitelistInfo{
		Limit: 100,
	}

	tc := &transaction.ConfigITOContract{
		AssetID:                assetId,
		ReceiverAddress:        defaultSender,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		PackInfo:               packs,
		DefaultLimitPerAddress: 100000000,
		WhitelistStatus:        transaction.ConfigITOContract_ActiveITO,
		WhitelistInfo:          whitelist,
	}

	_, err := kc.GetITOKApp().Config(defaultSender, tc)
	require.NoError(t, err)
}

func GetIto(t *testing.T, kc kapp.KAppController, asset []byte) *kapps.ITOData {
	accCacher := kc.GetITOKApp().GetAccountsCacher()
	itoKapp, err := accCacher.GetExistingKapp(kapps.ITOKAppAddress)
	require.NoError(t, err)

	key := kdautils.ToITOKey(asset)
	itoBytes, err := itoKapp.DataTrieTracker().RetrieveValue(key)
	require.NoError(t, err)

	ito := &kapps.ITOData{}
	err = marshalizer.Unmarshal(ito, itoBytes)
	require.NoError(t, err)
	return ito
}

func GetItoWhitelistByAddress(t *testing.T, kc kapp.KAppController, asset []byte, address []byte) (*kapps.WhitelistData, error) {
	accCacher := kc.GetITOKApp().GetAccountsCacher()
	itoKapp, err := accCacher.GetExistingKapp(kapps.ITOKAppAddress)
	require.NoError(t, err)

	key := kdautils.ToITOWhitelistKey(asset, hex.EncodeToString(address))
	itoBytes, err := itoKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}

	whitelistData := &kapps.WhitelistData{}
	err = marshalizer.Unmarshal(whitelistData, itoBytes)
	require.NoError(t, err)
	return whitelistData, nil
}

func TestITO_RemoveAddressFromWhitelist_BeforeSCFork(t *testing.T) {
	kc, err := createMockControllers(config.EnableEpochs{
		SmartContracts: 100_000,
	})

	require.NoError(t, err)
	ticker := []byte("FAKE")
	asset := "FAKE-1SDT"

	createAsset(t, kc, ticker, transaction.CreateAssetContract_Fungible, 6, 1_000_000, 1_000_000)
	createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 6, 0, 1_000_000)

	// default ito create 2 whitelist
	createDefaultITO(t, kc, asset, defaultAssetID)

	ito := GetIto(t, kc, defaultAssetID)
	assert.Equal(t, defaultWhitelistLen, ito.WhitelistLen)

	itoWhiteList, err := GetItoWhitelistByAddress(t, kc, defaultAssetID, defaultSender)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000000), itoWhiteList.Limit)

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo, 0)
	whitelistToRemove[hex.EncodeToString(defaultSender)] = &transaction.WhitelistInfo{
		Limit: 1000000000,
	}

	tc := &transaction.ITOTriggerContract{
		AssetID:       defaultAssetID,
		TriggerType:   transaction.ITOTriggerContract_RemoveFromWhitelist,
		WhitelistInfo: whitelistToRemove,
	}

	_, err = kc.GetITOKApp().Trigger(defaultSender, tc)
	require.NoError(t, err)

	ito = GetIto(t, kc, defaultAssetID)
	assert.Equal(t, defaultWhitelistLen, ito.WhitelistLen) // even if we remove whitelist the len keep the same value.

	_, err = GetItoWhitelistByAddress(t, kc, defaultAssetID, defaultSender) // and the whitelist is removed from whitelist kapp
	require.Error(t, err)
}

func TestITO_RemoveAddressFromWhitelist_AfterSCFork(t *testing.T) {
	kc, err := createMockControllers(config.EnableEpochs{
		SmartContracts: 0,
	})

	require.NoError(t, err)
	ticker := []byte("FAKE")
	asset := "FAKE-1SDT"

	createAsset(t, kc, ticker, transaction.CreateAssetContract_Fungible, 6, 1_000_000, 1_000_000)
	createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 6, 0, 1_000_000)

	// default ito create 2 whitelist
	createDefaultITO(t, kc, asset, defaultAssetID)

	ito := GetIto(t, kc, defaultAssetID)
	assert.Equal(t, defaultWhitelistLen, ito.WhitelistLen)

	itoWhiteList, err := GetItoWhitelistByAddress(t, kc, defaultAssetID, defaultSender)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000000), itoWhiteList.Limit)

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo, 0)
	whitelistToRemove[hex.EncodeToString(defaultSender)] = &transaction.WhitelistInfo{
		Limit: 1000000000,
	}

	tc := &transaction.ITOTriggerContract{
		AssetID:       defaultAssetID,
		TriggerType:   transaction.ITOTriggerContract_RemoveFromWhitelist,
		WhitelistInfo: whitelistToRemove,
	}

	_, err = kc.GetITOKApp().Trigger(defaultSender, tc)
	require.NoError(t, err)

	ito = GetIto(t, kc, defaultAssetID) // now the whitelist len is correct
	assert.Equal(t, defaultWhitelistLen-1, ito.WhitelistLen)

	_, err = GetItoWhitelistByAddress(t, kc, defaultAssetID, defaultSender) // and the whitelist is removed from whitelist kapp
	require.Error(t, err)
}

func TestITO_BuyITO_ShouldWork_BeforeFork(t *testing.T) {
	kc, err := createMockControllers(config.EnableEpochs{
		SmartContracts: 100_000,
	})

	require.NoError(t, err)
	itoTicker := []byte("ITO")
	itoAsset := "ITO-1PUA"

	createAsset(t, kc, itoTicker, transaction.CreateAssetContract_Fungible, 8, 100_000, 100_000)
	createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 8, 10_000_000, 10_000_000)

	packs := make(map[string]*transaction.PackInfo, 0)
	packsAsset := make([]*transaction.PackItem, 0)
	packsAsset = append(packsAsset, &transaction.PackItem{
		Amount: 100,
		Price:  50,
	})
	packs["KDA-3W0I"] = &transaction.PackInfo{
		Packs: packsAsset,
	}

	tc := &transaction.ConfigITOContract{
		AssetID:                []byte(itoAsset),
		ReceiverAddress:        defaultSender,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		PackInfo:               packs,
		DefaultLimitPerAddress: 10_000_000,
		WhitelistStatus:        transaction.ConfigITOContract_PausedITO,
	}

	_, err = kc.GetITOKApp().Config(defaultSender, tc)
	require.NoError(t, err)

	bc := &transaction.BuyContract{
		CurrencyID: []byte(defaultAssetID),
		ID:         []byte(itoAsset),
		Amount:     100,
	}

	status, err := kc.GetITOKApp().Buy(defaultSender, bc)
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)

}

func TestITO_BuyITO_KdaAmount_AfterFork(t *testing.T) {
	kc, err := createMockControllers(config.EnableEpochs{
		SmartContracts: 0,
	})
	require.NoError(t, err)

	itoTicker := []byte("ITO")
	itoAsset := "ITO-1PUA"

	createAsset(t, kc, itoTicker, transaction.CreateAssetContract_Fungible, 6, 0, 100_000_000)
	createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 6, 10_000_000, 10_000_000)

	packs := make(map[string]*transaction.PackInfo, 0)
	packsAsset := make([]*transaction.PackItem, 0)
	packsAsset = append(packsAsset, &transaction.PackItem{
		Amount: 1000000,
		Price:  1000000,
	})
	packs["KDA-3W0I"] = &transaction.PackInfo{
		Packs: packsAsset,
	}

	tc := &transaction.ConfigITOContract{
		AssetID:                []byte(itoAsset),
		ReceiverAddress:        defaultSender,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		PackInfo:               packs,
		DefaultLimitPerAddress: 10_000_000,
		WhitelistStatus:        transaction.ConfigITOContract_PausedITO,
	}

	_, err = kc.GetITOKApp().Config(defaultSender, tc)
	require.NoError(t, err)

	// 100 KDA FOR  5000 KLV
	bc := &transaction.BuyContract{
		CurrencyID:     []byte(defaultAssetID),
		ID:             []byte(itoAsset),
		Amount:         1000000, // Amount in ITO KDA (1 ITO-1PUA)
		CurrencyAmount: 1000000, // now, need to send kda amount you want to spend(1 KDA-3W0I)
	}

	_, err = kc.GetITOKApp().Buy(defaultSender, bc)
	require.NoError(t, err)
}

func TestITO_BuyOverflow_AfterSCFork(t *testing.T) {
	kc, err := createMockControllers(config.EnableEpochs{
		SmartContracts: 0,
	})

	require.NoError(t, err)
	itoTicker := []byte("ITO")
	itoAsset := "ITO-1PUA"

	createAsset(t, kc, itoTicker, transaction.CreateAssetContract_Fungible, 8, 100_000, 100_000)
	createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 8, 10_000_000, 10_000_000)

	packs := make(map[string]*transaction.PackInfo, 0)
	packsAsset := make([]*transaction.PackItem, 0)
	packsAsset = append(packsAsset, &transaction.PackItem{
		Amount: 100_00000000,
		Price:  65536_00000000,
	})
	packs["KDA-3W0I"] = &transaction.PackInfo{
		Packs: packsAsset,
	}

	tc := &transaction.ConfigITOContract{
		AssetID:                []byte(itoAsset),
		ReceiverAddress:        defaultSender,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		PackInfo:               packs,
		DefaultLimitPerAddress: 10_000_000,
		WhitelistStatus:        transaction.ConfigITOContract_PausedITO,
	}

	_, err = kc.GetITOKApp().Config(defaultSender, tc)
	require.NoError(t, err)

	bc := &transaction.BuyContract{
		CurrencyID: []byte(defaultAssetID),
		ID:         []byte(itoAsset),
		Amount:     2814749_76710657,
	}

	status, err := kc.GetITOKApp().Buy(defaultSender, bc)
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}
