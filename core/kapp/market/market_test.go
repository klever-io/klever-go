package market

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

var (
	sender       = "sender"
	otherSender  = "otherSender"
	defaultAddr  = makeAddress(sender)
	defaultOther = makeAddress(otherSender)
)

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func createMemUnit() storage.Storer {
	capacity := uint32(10)
	shards := uint32(1)
	sizeInBytes := uint64(0)
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{
		Type:        storageUnit.LRUCache,
		Capacity:    capacity,
		Shards:      shards,
		SizeInBytes: sizeInBytes,
	})
	persist, _ := memorydb.NewlruDB(100000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)
	return unit
}

func createAccountsDB(marshalizer marshal.Marshalizer, accountFactory state.AccountFactory) *state.AccountsDB {
	hasher := &sha256.Sha256{}
	trieStorageManager, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	return adb
}

func createTestMarketKApp(t *testing.T) (*marketKapp, state.AccountsCacher, *mock.ForkControllerStub) {
	marshalizer := marshal.NewProtoMarshalizer()
	hasher := &sha256.Sha256{}
	forkController := mock.NewForkControllerStub()
	pubkeyConv := mock.NewPubkeyConverterMock(32)

	// Create accounts databases
	userAccountsDB := createAccountsDB(marshalizer, factory.NewAccountCreator())
	kappAccountsDB := createAccountsDB(marshalizer, factory.NewKAppAccountCreator())
	peerAccountsDB := createAccountsDB(marshalizer, factory.NewPeerAccountCreator())

	// Create accounts cacher
	accCacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: userAccountsDB,
		Kapps:    kappAccountsDB,
		Peers:    peerAccountsDB,
	})
	require.NoError(t, err)
	accCacher.ResetAll(true)

	// Create market KApp
	args := &ArgsNewMarketKApp{
		Hasher:         hasher,
		Marshalizer:    marshalizer,
		PubkeyConv:     pubkeyConv,
		ForkController: forkController,
	}

	marketKApp, err := NewMarketKApp(args)
	require.NoError(t, err)
	require.NotNil(t, marketKApp)

	err = marketKApp.SetAccountsCacher(accCacher)
	require.NoError(t, err)

	return marketKApp, accCacher, forkController
}

func TestNewMarketKApp(t *testing.T) {
	t.Parallel()

	t.Run("NilHasher", func(t *testing.T) {
		args := &ArgsNewMarketKApp{
			Hasher:         nil,
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}

		marketKApp, err := NewMarketKApp(args)
		require.Error(t, err)
		require.Nil(t, marketKApp)
	})

	t.Run("NilMarshalizer", func(t *testing.T) {
		args := &ArgsNewMarketKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    nil,
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}

		marketKApp, err := NewMarketKApp(args)
		require.Error(t, err)
		require.Nil(t, marketKApp)
	})

	t.Run("NilPubkeyConverter", func(t *testing.T) {
		args := &ArgsNewMarketKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     nil,
			ForkController: mock.NewForkControllerStub(),
		}

		marketKApp, err := NewMarketKApp(args)
		require.Error(t, err)
		require.Nil(t, marketKApp)
	})

	t.Run("Success", func(t *testing.T) {
		args := &ArgsNewMarketKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}

		marketKApp, err := NewMarketKApp(args)
		require.NoError(t, err)
		require.NotNil(t, marketKApp)
		require.False(t, marketKApp.IsInterfaceNil())
	})
}

func TestMarketKApp_SetAccountsCacher(t *testing.T) {
	t.Parallel()

	t.Run("NilCacher", func(t *testing.T) {
		marketKApp, _, _ := createTestMarketKApp(t)

		err := marketKApp.SetAccountsCacher(nil)
		require.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		marketKApp, accCacher, _ := createTestMarketKApp(t)

		err := marketKApp.SetAccountsCacher(accCacher)
		require.NoError(t, err)

		retrievedCacher := marketKApp.GetAccountsCacher()
		require.Equal(t, accCacher, retrievedCacher)
	})
}

func TestMarketKApp_SetKAppController(t *testing.T) {
	t.Parallel()

	marketKApp, _, _ := createTestMarketKApp(t)

	err := marketKApp.SetKAppController(nil)
	require.NoError(t, err)
}

func TestMarketKApp_GetMarketplace(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load market kapp account
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	// Save market kapp account
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	t.Run("NilMarketplaceID", func(t *testing.T) {
		kappAcc, marketplace, err := marketKApp.GetMarketplace(nil)
		require.NoError(t, err)
		require.NotNil(t, kappAcc)
		require.NotNil(t, marketplace)
	})

	t.Run("NonExistentMarketplace", func(t *testing.T) {
		marketplaceID := []byte("nonexistent")
		kappAcc, marketplace, err := marketKApp.GetMarketplace(marketplaceID)
		require.Error(t, err)
		require.NotNil(t, kappAcc) // Returns kapp account even if not found
		require.NotNil(t, marketplace) // Returns empty struct when not found
		require.Nil(t, marketplace.ID)
	})
}

func TestMarketKApp_SetAndGetMarketplace(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load market kapp account
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	marketplace := &kapps.Marketplace{
		ID:                 []byte("marketplace-1"),
		OwnerAddress:       defaultAddr,
		Name:               []byte("Test Marketplace"),
		ReferralAddress:    defaultOther,
		ReferralPercentage: 500, // 5%
	}

	// Set marketplace
	err = marketKApp.SetMarketplace(marketKappAcc, marketplace)
	require.NoError(t, err)

	// Save market kapp account
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	// Get marketplace back
	retrievedKappAcc, retrievedMarketplace, err := marketKApp.GetMarketplace(marketplace.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKappAcc)
	require.NotNil(t, retrievedMarketplace)
	require.Equal(t, marketplace.ID, retrievedMarketplace.ID)
	require.Equal(t, marketplace.Name, retrievedMarketplace.Name)
	require.Equal(t, marketplace.ReferralPercentage, retrievedMarketplace.ReferralPercentage)
}

func TestMarketKApp_GetMarketOrder(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load market kapp account
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	// Save market kapp account
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	t.Run("NilOrderID", func(t *testing.T) {
		kappAcc, order, err := marketKApp.GetMarketOrder(nil)
		require.NoError(t, err)
		require.NotNil(t, kappAcc)
		require.NotNil(t, order)
	})

	t.Run("NonExistentOrder", func(t *testing.T) {
		orderID := []byte("nonexistent")
		kappAcc, order, err := marketKApp.GetMarketOrder(orderID)
		require.Error(t, err)
		require.NotNil(t, kappAcc) // Returns kapp account even if not found
		require.NotNil(t, order) // Returns empty struct when not found
		require.Nil(t, order.ID)
	})
}

func TestMarketKApp_SetAndGetMarketOrder(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load market kapp account
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	order := &kapps.MarketOrderData{
		ID:            []byte("order-1"),
		MarketplaceID: []byte("marketplace-1"),
		MarketType:    kapps.MarketOrderData_BuyItNow,
		OwnerAddress:  defaultAddr,
		CollectionID:  []byte("KLV-ABC"),
		AssetID:       []byte("001"),
		CurrencyID:    []byte("KLV"),
		Price:         1000000,
		ReservePrice:  0,
		StartTime:     1000,
		EndTime:       2000,
		IsClaimed:     false,
	}

	// Set market order
	err = marketKApp.SetMarketOrder(marketKappAcc, order)
	require.NoError(t, err)

	// Save market kapp account
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	// Get order back
	retrievedKappAcc, retrievedOrder, err := marketKApp.GetMarketOrder(order.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKappAcc)
	require.NotNil(t, retrievedOrder)
	require.Equal(t, order.ID, retrievedOrder.ID)
	require.Equal(t, order.Price, retrievedOrder.Price)
	require.Equal(t, order.MarketType, retrievedOrder.MarketType)
}

func TestMarketKApp_computeReferralAmount(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Setup marketplace with referral
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	marketplace := &kapps.Marketplace{
		ID:                 []byte("marketplace-1"),
		OwnerAddress:       defaultAddr,
		Name:               []byte("Test Marketplace"),
		ReferralAddress:    defaultOther,
		ReferralPercentage: 500, // 5%
	}
	err = marketKApp.SetMarketplace(marketKappAcc, marketplace)
	require.NoError(t, err)
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	// Create referral account with initial balance
	userReferralAcc, err := accCacher.LoadUser(defaultOther)
	require.NoError(t, err)
	err = accCacher.UpdateUser(userReferralAcc)
	require.NoError(t, err)

	// Create KApp context with receipts mock
	receiptsStub := mock.NewReceiptsContextStub()
	ctx := &mock.KAppContextStub{
		ContractIDCalled: func() int {
			return 1
		},
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsStub
		},
	}

	marketOrder := &kapps.MarketOrderData{
		ID:            []byte("order-1"),
		MarketplaceID: marketplace.ID,
		Price:         1000000,
	}

	t.Run("ZeroReferralAmount", func(t *testing.T) {
		status, err := marketKApp.computeReferralAmount(ctx, marketOrder, 0, []byte("KLV"))
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("NegativeReferralAmount", func(t *testing.T) {
		status, err := marketKApp.computeReferralAmount(ctx, marketOrder, -100, []byte("KLV"))
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("ValidReferralAmount", func(t *testing.T) {
		referralAmount := int64(50000) // 5% of 1000000
		initialBalance := userReferralAcc.GetBalance([]byte("KLV"), false)

		status, err := marketKApp.computeReferralAmount(ctx, marketOrder, referralAmount, []byte("KLV"))
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify balance increased
		updatedUserAcc, err := accCacher.LoadUser(defaultOther)
		require.NoError(t, err)
		finalBalance := updatedUserAcc.GetBalance([]byte("KLV"), false)
		require.Equal(t, initialBalance+referralAmount, finalBalance)
	})

	t.Run("InvalidMarketplace", func(t *testing.T) {
		invalidOrder := &kapps.MarketOrderData{
			ID:            []byte("order-invalid"),
			MarketplaceID: []byte("nonexistent-marketplace"),
			Price:         1000000,
		}
		status, err := marketKApp.computeReferralAmount(ctx, invalidOrder, 50000, []byte("KLV"))
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})
}

func TestMarketKApp_computeRoyaltiesFixedDeposit(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Create KApp context with receipts mock
	receiptsStub := mock.NewReceiptsContextStub()
	ctx := &mock.KAppContextStub{
		ContractIDCalled: func() int {
			return 1
		},
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsStub
		},
	}

	// Create asset owner account
	userOwnerAcc, err := accCacher.LoadUser(defaultAddr)
	require.NoError(t, err)
	err = accCacher.UpdateUser(userOwnerAcc)
	require.NoError(t, err)

	marketOrder := &kapps.MarketOrderData{
		ID:                      []byte("order-1"),
		MarketplaceID:           []byte("marketplace-1"),
		RoyaltiesFixedDeposit:   100000,
	}

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address:         defaultAddr,
			SplitRoyalties:  make(map[string]*kapps.RoyaltySplitData),
		},
	}

	t.Run("ZeroRoyaltiesFixedDeposit", func(t *testing.T) {
		orderZero := &kapps.MarketOrderData{
			ID:                      []byte("order-zero"),
			MarketplaceID:           []byte("marketplace-1"),
			RoyaltiesFixedDeposit:   0,
		}
		status, err := marketKApp.computeRoyaltiesFixedDeposit(ctx, orderZero, asset)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("NegativeRoyaltiesFixedDeposit", func(t *testing.T) {
		orderNeg := &kapps.MarketOrderData{
			ID:                      []byte("order-neg"),
			MarketplaceID:           []byte("marketplace-1"),
			RoyaltiesFixedDeposit:   -100,
		}
		status, err := marketKApp.computeRoyaltiesFixedDeposit(ctx, orderNeg, asset)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("ValidRoyaltiesFixedDeposit", func(t *testing.T) {
		initialBalance := userOwnerAcc.GetBalance([]byte("KLV"), false)

		status, err := marketKApp.computeRoyaltiesFixedDeposit(ctx, marketOrder, asset)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify balance increased
		updatedUserAcc, err := accCacher.LoadUser(defaultAddr)
		require.NoError(t, err)
		finalBalance := updatedUserAcc.GetBalance([]byte("KLV"), false)
		require.Equal(t, initialBalance+marketOrder.RoyaltiesFixedDeposit, finalBalance)
	})
}

func TestMarketKApp_computeRoyaltiesAmount(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, forkController := createTestMarketKApp(t)

	// Enable KDA FPR feature
	forkController.KdaFprValue = true

	// Create KApp context with receipts mock
	receiptsStub := mock.NewReceiptsContextStub()
	ctx := &mock.KAppContextStub{
		ContractIDCalled: func() int {
			return 1
		},
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsStub
		},
	}

	// Create asset owner account
	userOwnerAcc, err := accCacher.LoadUser(defaultAddr)
	require.NoError(t, err)
	err = accCacher.UpdateUser(userOwnerAcc)
	require.NoError(t, err)

	marketOrder := &kapps.MarketOrderData{
		ID:            []byte("order-1"),
		MarketplaceID: []byte("marketplace-1"),
		Price:         1000000,
	}

	asset := &kapps.KDAData{
		OwnerAddress: defaultAddr,
		Royalties: &kapps.RoyaltiesData{
			Address:           defaultAddr,
			MarketPercentage:  1000, // 10%
			SplitRoyalties:    make(map[string]*kapps.RoyaltySplitData),
		},
	}

	t.Run("ZeroRoyaltiesAmount", func(t *testing.T) {
		status, err := marketKApp.computeRoyaltiesAmount(ctx, marketOrder, asset, []byte("KLV"), 0)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("NegativeRoyaltiesAmount", func(t *testing.T) {
		status, err := marketKApp.computeRoyaltiesAmount(ctx, marketOrder, asset, []byte("KLV"), -100)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("ValidRoyaltiesAmount", func(t *testing.T) {
		royaltiesAmount := int64(100000) // 10% of 1000000
		initialBalance := userOwnerAcc.GetBalance([]byte("KLV"), false)

		status, err := marketKApp.computeRoyaltiesAmount(ctx, marketOrder, asset, []byte("KLV"), royaltiesAmount)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify balance increased
		updatedUserAcc, err := accCacher.LoadUser(defaultAddr)
		require.NoError(t, err)
		finalBalance := updatedUserAcc.GetBalance([]byte("KLV"), false)
		require.Equal(t, initialBalance+royaltiesAmount, finalBalance)
	})
}
