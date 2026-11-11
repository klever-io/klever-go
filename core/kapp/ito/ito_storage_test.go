package ito_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
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

// TestITO_ZeroPackPrice_AllPaths checks that ConfigITO, the SetITOPrices trigger and
// SetPrices agree on a zero pack price: only ConfigITO and the trigger accept it, and
// only while KdaFpr is active and FixAuditChangesV6 is not (GHSA-jwr2-h2vc-8vvg).
func TestITO_ZeroPackPrice_AllPaths(t *testing.T) {
	const inactive = uint32(100_000)
	itoAsset := []byte("ITO-1PUA")
	zeroPricePacks := map[string]*transaction.PackInfo{
		string(defaultAssetID): {Packs: []*transaction.PackItem{{Amount: 1, Price: 0}, {Amount: 100, Price: 10}}},
	}

	paths := map[string]struct {
		zeroPriceAllowedOnKdaFpr bool
		store                    func(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error)
	}{
		"ConfigITO": {
			zeroPriceAllowedOnKdaFpr: true,
			store: func(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
				return kc.GetITOKApp().Config(defaultSender, &transaction.ConfigITOContract{
					AssetID:         itoAsset,
					ReceiverAddress: defaultSender,
					Status:          transaction.ConfigITOContract_ActiveITO,
					PackInfo:        zeroPricePacks,
					WhitelistStatus: transaction.ConfigITOContract_PausedITO,
				})
			},
		},
		"SetITOPrices trigger": {
			zeroPriceAllowedOnKdaFpr: true,
			store: func(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
				return kc.GetITOKApp().Trigger(defaultSender, &transaction.ITOTriggerContract{
					TriggerType: transaction.ITOTriggerContract_SetITOPrices,
					AssetID:     itoAsset,
					PackInfo:    zeroPricePacks,
				})
			},
		},
		"SetPrices": {
			zeroPriceAllowedOnKdaFpr: false,
			store: func(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
				return kc.GetITOKApp().SetPrices(defaultSender, &transaction.SetITOPricesContract{
					AssetID:  itoAsset,
					PackInfo: zeroPricePacks,
				})
			},
		},
	}

	for name, path := range paths {
		for _, kdaFpr := range []bool{false, true} {
			for _, fixV6 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/KdaFpr=%t/FixAuditChangesV6=%t", name, kdaFpr, fixV6), func(t *testing.T) {
					epochs := config.EnableEpochs{KdaFpr: inactive, FixAuditChangesV6: inactive}
					if kdaFpr {
						epochs.KdaFpr = 0
					}
					if fixV6 {
						epochs.FixAuditChangesV6 = 0
					}

					kc, err := createMockControllers(epochs)
					require.NoError(t, err)

					createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)
					createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 0, 10_000_000, 10_000_000)
					if name != "ConfigITO" {
						createDefaultITO(t, kc, string(defaultAssetID), itoAsset)
						grantSetITOPricesRole(t, kc, itoAsset, defaultSender)
					}

					status, err := path.store(kc)

					if kdaFpr && !fixV6 && path.zeroPriceAllowedOnKdaFpr {
						require.NoError(t, err)
						assert.Equal(t, transaction.Transaction_Ok, status)
						assert.Equal(t, int64(0), GetIto(t, kc, itoAsset).PackData[string(defaultAssetID)].Packs[0].Price)
						return
					}

					assert.Equal(t, common.ErrInvalidValue, err)
					assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
				})
			}
		}
	}
}

func grantSetITOPricesRole(t *testing.T, kc kapp.KAppController, assetID []byte, address []byte) {
	kdaKapp, asset, err := kc.GetKDAKApp().GetKDA(assetID)
	require.NoError(t, err)

	asset.Roles = append(asset.Roles, &kapps.RolesData{Address: address, HasRoleSetITOPrices: true})
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, assetID, asset))
}

// TestITO_NonFungiblePackCurrency_AllPaths checks that, from FixAuditChangesV7, ConfigITO, the
// SetITOPrices trigger and SetPrices refuse a pack priced in an NFT or SFT collection: GetKDA finds
// it, but its bare key holds no balance, so the pack could never be paid (KLC-2876). Before the
// fork all three store it, and a fungible currency is stored on both sides.
func TestITO_NonFungiblePackCurrency_AllPaths(t *testing.T) {
	const inactive = uint32(100_000)
	itoAsset := []byte("ITO-1PUA")
	currencies := map[string]kapps.KDAData_EnumAssetType{
		"NFT-1A2B":             kapps.KDAData_NonFungible,
		"SFT-1A2B":             kapps.KDAData_SemiFungible,
		string(defaultAssetID): kapps.KDAData_Fungible,
	}

	paths := packPricePaths(itoAsset)

	for name, store := range paths {
		for currency, currencyType := range currencies {
			for _, fixV7 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/FixAuditChangesV7=%t", name, currency, fixV7), func(t *testing.T) {
					epochs := config.EnableEpochs{FixAuditChangesV7: inactive}
					if fixV7 {
						epochs.FixAuditChangesV7 = 0
					}
					kc, err := createMockControllers(epochs)
					require.NoError(t, err)

					createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)
					createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 0, 10_000_000, 10_000_000)
					if currencyType != kapps.KDAData_Fungible {
						registerCollection(t, kc, []byte(currency), currencyType)
					}
					if name != "ConfigITO" {
						createDefaultITO(t, kc, string(defaultAssetID), itoAsset)
						grantSetITOPricesRole(t, kc, itoAsset, defaultSender)
					}

					status, err := store(kc, map[string]*transaction.PackInfo{
						currency: {Packs: []*transaction.PackItem{{Amount: 1, Price: 10}}},
					})

					if !fixV7 || currencyType == kapps.KDAData_Fungible {
						require.NoError(t, err)
						assert.Equal(t, transaction.Transaction_Ok, status)
						assert.Contains(t, GetIto(t, kc, itoAsset).PackData, currency)
						return
					}

					assert.Equal(t, common.ErrAssetTypeInvalid, err)
					assert.Equal(t, transaction.Transaction_AssetTypeInvalid, status)
					fields := errorReceiptFields(kc)
					require.NotEmpty(t, fields)
					assert.Equal(t, common.ErrFieldInvalidAssetType, fields[len(fields)-1])
					if name == "ConfigITO" {
						itoReader := kc.GetITOKApp().(interface {
							GetITO(assetID []byte) (state.KAppAccountHandler, *kapps.ITOData, error)
						})
						_, _, err = itoReader.GetITO(itoAsset)
						assert.ErrorIs(t, err, common.ErrNotFoundInKApp)
						return
					}
					assert.NotContains(t, GetIto(t, kc, itoAsset).PackData, currency)
				})
			}
		}
	}
}

// TestITO_PackCurrencyOrderIsDeterministic checks that from FixAuditChangesV7 every path checks the
// pack currencies in key order: a missing asset (KAPPError) sorts before an NFT collection
// (AssetTypeInvalid), so the result is KAPPError on every run, never the map's order (KLC-2876).
func TestITO_PackCurrencyOrderIsDeterministic(t *testing.T) {
	itoAsset := []byte("ITO-1PUA")
	packs := map[string]*transaction.PackInfo{
		"MISS-0000": {Packs: []*transaction.PackItem{{Amount: 1, Price: 10}}},
		"NFT-1A2B":  {Packs: []*transaction.PackItem{{Amount: 1, Price: 10}}},
	}

	for name, store := range packPricePaths(itoAsset) {
		t.Run(name, func(t *testing.T) {
			kc, err := createMockControllers(config.EnableEpochs{})
			require.NoError(t, err)

			createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)
			createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 0, 10_000_000, 10_000_000)
			registerCollection(t, kc, []byte("NFT-1A2B"), kapps.KDAData_NonFungible)
			if name != "ConfigITO" {
				createDefaultITO(t, kc, string(defaultAssetID), itoAsset)
				grantSetITOPricesRole(t, kc, itoAsset, defaultSender)
			}

			for run := 0; run < 30; run++ {
				status, err := store(kc, packs)
				require.Error(t, err)
				require.Equal(t, transaction.Transaction_KAPPError, status, "run %d", run)
			}

			// SetPrices and the SetITOPrices trigger add a receipt for the missing asset; ConfigITO never has
			missing := 0
			for _, field := range errorReceiptFields(kc) {
				if field == common.ErrFieldAssetNotFound {
					missing++
				}
			}
			want := 30
			if name == "ConfigITO" {
				want = 0
			}
			assert.Equal(t, want, missing)
		})
	}
}

// TestITO_ConfigWhitelistOrderIsDeterministic checks that from FixAuditChangesV7 ConfigITO checks its
// whitelist in key order: a valid hex address with a negative limit (ParameterInvalid) sorts before a
// key that is not hex (AccountError), so the result is ParameterInvalid on every run (KLC-2770).
func TestITO_ConfigWhitelistOrderIsDeterministic(t *testing.T) {
	itoAsset := []byte("ITO-1PUA")
	whitelist := map[string]*transaction.WhitelistInfo{
		hex.EncodeToString(defaultSender): {Limit: -1},
		"zz-not-hex":                      {Limit: 1},
	}

	newConfigRun := func(t *testing.T, epochs config.EnableEpochs) func() transaction.Transaction_TXResultCode {
		kc, err := createMockControllers(epochs)
		require.NoError(t, err)
		createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)

		return func() transaction.Transaction_TXResultCode {
			status, err := kc.GetITOKApp().Config(defaultSender, &transaction.ConfigITOContract{
				AssetID:         itoAsset,
				ReceiverAddress: defaultSender,
				Status:          transaction.ConfigITOContract_ActiveITO,
				WhitelistStatus: transaction.ConfigITOContract_PausedITO,
				WhitelistInfo:   whitelist,
			})
			require.Error(t, err)
			return status
		}
	}

	t.Run("first refusal in key order wins from FixAuditChangesV7", func(t *testing.T) {
		run := newConfigRun(t, config.EnableEpochs{})
		for i := 0; i < 100; i++ {
			require.Equal(t, transaction.Transaction_ParameterInvalid, run(), "run %d", i)
		}
	})

	t.Run("either refusal may fire before FixAuditChangesV7", func(t *testing.T) {
		run := newConfigRun(t, config.EnableEpochs{FixAuditChangesV7: 1_000})
		seen := make(map[transaction.Transaction_TXResultCode]bool)
		for i := 0; i < 100; i++ {
			seen[run()] = true
		}
		require.Equal(t, map[transaction.Transaction_TXResultCode]bool{
			transaction.Transaction_ParameterInvalid: true,
			transaction.Transaction_AccountError:     true,
		}, seen, "pre-fork order must stay Go's map order")
	})
}

// TestITO_AddToWhitelistOrderIsDeterministic checks that from FixAuditChangesV7 the AddToWhitelist trigger
// checks its entries in key order, as ConfigITO does: a valid hex address with a negative limit
// (ParameterInvalid) sorts before a key that is not hex (AccountError) (KLC-2770).
func TestITO_AddToWhitelistOrderIsDeterministic(t *testing.T) {
	itoAsset := []byte("ITO-1PUA")
	whitelist := map[string]*transaction.WhitelistInfo{
		hex.EncodeToString(defaultSender): {Limit: -1},
		"zz-not-hex":                      {Limit: 1},
	}

	newTriggerRun := func(t *testing.T, epochs config.EnableEpochs) func() transaction.Transaction_TXResultCode {
		kc, err := createMockControllers(epochs)
		require.NoError(t, err)
		createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)
		createAsset(t, kc, defaultTicker, transaction.CreateAssetContract_Fungible, 0, 10_000_000, 10_000_000)
		createDefaultITO(t, kc, string(defaultAssetID), itoAsset)

		return func() transaction.Transaction_TXResultCode {
			status, err := kc.GetITOKApp().Trigger(defaultSender, &transaction.ITOTriggerContract{
				TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
				AssetID:       itoAsset,
				WhitelistInfo: whitelist,
			})
			require.Error(t, err)
			return status
		}
	}

	t.Run("first refusal in key order wins from FixAuditChangesV7", func(t *testing.T) {
		run := newTriggerRun(t, config.EnableEpochs{})
		for i := 0; i < 100; i++ {
			require.Equal(t, transaction.Transaction_ParameterInvalid, run(), "run %d", i)
		}
	})

	t.Run("either refusal may fire before FixAuditChangesV7", func(t *testing.T) {
		run := newTriggerRun(t, config.EnableEpochs{FixAuditChangesV7: 1_000})
		seen := make(map[transaction.Transaction_TXResultCode]bool)
		for i := 0; i < 100; i++ {
			seen[run()] = true
		}
		require.Equal(t, map[transaction.Transaction_TXResultCode]bool{
			transaction.Transaction_ParameterInvalid: true,
			transaction.Transaction_AccountError:     true,
		}, seen, "pre-fork order must stay Go's map order")
	})
}

// errorReceiptFields returns the field of every error receipt in the current KApp context.
func errorReceiptFields(kc kapp.KAppController) []string {
	var fields []string
	for _, receipt := range kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error)) {
		fields = append(fields, string(receipt.Data[1]))
	}
	return fields
}

// packPricePaths returns the three ways to set an ITO's pack prices, each storing packs for itoAsset.
func packPricePaths(itoAsset []byte) map[string]func(kc kapp.KAppController, packs map[string]*transaction.PackInfo) (transaction.Transaction_TXResultCode, error) {
	return map[string]func(kc kapp.KAppController, packs map[string]*transaction.PackInfo) (transaction.Transaction_TXResultCode, error){
		"ConfigITO": func(kc kapp.KAppController, packs map[string]*transaction.PackInfo) (transaction.Transaction_TXResultCode, error) {
			return kc.GetITOKApp().Config(defaultSender, &transaction.ConfigITOContract{
				AssetID:         itoAsset,
				ReceiverAddress: defaultSender,
				Status:          transaction.ConfigITOContract_ActiveITO,
				PackInfo:        packs,
				WhitelistStatus: transaction.ConfigITOContract_PausedITO,
			})
		},
		"SetITOPrices trigger": func(kc kapp.KAppController, packs map[string]*transaction.PackInfo) (transaction.Transaction_TXResultCode, error) {
			return kc.GetITOKApp().Trigger(defaultSender, &transaction.ITOTriggerContract{
				TriggerType: transaction.ITOTriggerContract_SetITOPrices,
				AssetID:     itoAsset,
				PackInfo:    packs,
			})
		},
		"SetPrices": func(kc kapp.KAppController, packs map[string]*transaction.PackInfo) (transaction.Transaction_TXResultCode, error) {
			return kc.GetITOKApp().SetPrices(defaultSender, &transaction.SetITOPricesContract{
				AssetID:  itoAsset,
				PackInfo: packs,
			})
		},
	}
}

// registerCollection writes an NFT or SFT collection straight into the KDA trie: what GetKDA finds
// for a bare collection ID.
func registerCollection(t *testing.T, kc kapp.KAppController, id []byte, assetType kapps.KDAData_EnumAssetType) {
	kdaKapp, _, err := kc.GetKDAKApp().GetKDA(nil)
	require.NoError(t, err)
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, id, &kapps.KDAData{
		ID:           id,
		Ticker:       id,
		AssetType:    assetType,
		OwnerAddress: defaultSender,
		Properties:   &kapps.PropertiesData{},
		Attributes:   &kapps.AttributesData{},
	}))
}

// TestITO_BuyPackPricedInACollection checks a buy of a pack priced, before FixAuditChangesV7, in an NFT
// or SFT collection: from the fork it is refused for the currency's type, before it the debit of the
// empty bare collection key fails (KLC-2876).
func TestITO_BuyPackPricedInACollection(t *testing.T) {
	itoAsset := []byte("ITO-1PUA")
	collectionID := []byte("COLL-1A2B")

	for _, currencyType := range []kapps.KDAData_EnumAssetType{kapps.KDAData_NonFungible, kapps.KDAData_SemiFungible} {
		for _, fixV7 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/FixAuditChangesV7=%t", currencyType, fixV7), func(t *testing.T) {
				kc, err := createMockControllers(config.EnableEpochs{FixAuditChangesV7: 1})
				require.NoError(t, err)

				createAsset(t, kc, []byte("ITO"), transaction.CreateAssetContract_Fungible, 0, 100_000, 100_000)
				registerCollection(t, kc, collectionID, currencyType)
				createDefaultITO(t, kc, string(collectionID), itoAsset)
				if fixV7 {
					kc.GetForkController().(core.EpochSubscriberHandler).EpochConfirmed(1)
				}

				status, err := kc.GetITOKApp().Buy(defaultSender, &transaction.BuyContract{
					CurrencyID:     collectionID,
					ID:             itoAsset,
					Amount:         1,
					CurrencyAmount: 1,
				})

				if fixV7 {
					assert.Equal(t, common.ErrAssetTypeInvalid, err)
					assert.Equal(t, transaction.Transaction_AssetTypeInvalid, status)
					return
				}
				require.Error(t, err)
				assert.Equal(t, transaction.Transaction_BalanceError, status)
			})
		}
	}
}
