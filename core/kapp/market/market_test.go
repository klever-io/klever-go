package market

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
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
		require.NotNil(t, kappAcc)     // Returns kapp account even if not found
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
		require.NotNil(t, order)   // Returns empty struct when not found
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
		ID:                    []byte("order-1"),
		MarketplaceID:         []byte("marketplace-1"),
		RoyaltiesFixedDeposit: 100000,
	}

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address:        defaultAddr,
			SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
		},
	}

	t.Run("ZeroRoyaltiesFixedDeposit", func(t *testing.T) {
		orderZero := &kapps.MarketOrderData{
			ID:                    []byte("order-zero"),
			MarketplaceID:         []byte("marketplace-1"),
			RoyaltiesFixedDeposit: 0,
		}
		status, err := marketKApp.computeRoyaltiesFixedDeposit(ctx, orderZero, asset)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("NegativeRoyaltiesFixedDeposit", func(t *testing.T) {
		orderNeg := &kapps.MarketOrderData{
			ID:                    []byte("order-neg"),
			MarketplaceID:         []byte("marketplace-1"),
			RoyaltiesFixedDeposit: -100,
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
			Address:          defaultAddr,
			MarketPercentage: 1000, // 10%
			SplitRoyalties:   make(map[string]*kapps.RoyaltySplitData),
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

func TestMarketKApp_computeSplitRoyalties(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Create split royalty recipient account
	splitRecipientAddr := makeAddress("splitRecipient")
	splitRecipientAcc, err := accCacher.LoadUser(splitRecipientAddr)
	require.NoError(t, err)
	err = accCacher.UpdateUser(splitRecipientAcc)
	require.NoError(t, err)

	marketOrder := &kapps.MarketOrderData{
		ID:            []byte("order-1"),
		MarketplaceID: []byte("marketplace-1"),
		Price:         1000000,
	}

	// Address needs to be hex encoded for computeSplitRoyalties
	hexAddress := hex.EncodeToString(splitRecipientAddr)

	t.Run("ValidSplitRoyalties", func(t *testing.T) {
		// Create fresh receipts stub for this test
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int {
				return 1
			},
			ReceiptsCalled: func() kapp.ReceiptsContext {
				return receiptsStub
			},
		}

		initialBalance := splitRecipientAcc.GetBalance([]byte("KLV"), false)
		royaltiesToPay := int64(100000) // Total royalties
		percentage := int64(5000)       // 50%

		// Verify no receipts before
		require.Len(t, receiptsStub.Get(), 0)

		status, err := marketKApp.ComputeSplitRoyalties(ctx, hexAddress, marketOrder, []byte("KLV"), 100000, percentage, &royaltiesToPay)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Expected split amount: 100000 * 5000 / 10000 = 50000
		expectedSplit := int64(50000)

		// Verify royaltiesToPay was reduced
		require.Equal(t, int64(50000), royaltiesToPay)

		// Verify balance increased
		updatedAcc, err := accCacher.LoadUser(splitRecipientAddr)
		require.NoError(t, err)
		finalBalance := updatedAcc.GetBalance([]byte("KLV"), false)
		require.Equal(t, initialBalance+expectedSplit, finalBalance)

		// Verify Transfer receipt was added
		require.Len(t, receiptsStub.Get(), 1, "Expected 1 Transfer receipt")
	})

	t.Run("InvalidHexAddress", func(t *testing.T) {
		royaltiesToPay := int64(100000)

		status, err := marketKApp.ComputeSplitRoyalties(nil, "invalid-hex", marketOrder, []byte("KLV"), 100000, 5000, &royaltiesToPay)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_LoadAccountError, status)
	})

	t.Run("ZeroPercentage", func(t *testing.T) {
		// Create fresh receipts stub for this test
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int {
				return 1
			},
			ReceiptsCalled: func() kapp.ReceiptsContext {
				return receiptsStub
			},
		}

		royaltiesToPay := int64(100000)
		initialRoyalties := royaltiesToPay

		status, err := marketKApp.ComputeSplitRoyalties(ctx, hexAddress, marketOrder, []byte("KLV"), 100000, 0, &royaltiesToPay)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// With 0% percentage, royaltiesToPay should remain unchanged
		require.Equal(t, initialRoyalties, royaltiesToPay)

		// Still adds a Transfer receipt even with 0 amount
		require.Len(t, receiptsStub.Get(), 1, "Expected 1 Transfer receipt even with 0 amount")
	})
}

func TestMarketKApp_computeMarketOwnerAmount(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Create market order owner account
	ownerAddr := makeAddress("orderOwner")
	ownerAcc, err := accCacher.LoadUser(ownerAddr)
	require.NoError(t, err)
	err = accCacher.UpdateUser(ownerAcc)
	require.NoError(t, err)

	marketOrder := &kapps.MarketOrderData{
		ID:            []byte("order-1"),
		MarketplaceID: []byte("marketplace-1"),
		OwnerAddress:  ownerAddr,
		Price:         1000000,
	}

	t.Run("ZeroMarketOwnerAmount", func(t *testing.T) {
		status, err := marketKApp.ComputeMarketOwnerAmount(nil, marketOrder, []byte("KLV"), 0)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("NegativeMarketOwnerAmount", func(t *testing.T) {
		status, err := marketKApp.ComputeMarketOwnerAmount(nil, marketOrder, []byte("KLV"), -100)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("ValidMarketOwnerAmount", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int {
				return 1
			},
			ReceiptsCalled: func() kapp.ReceiptsContext {
				return receiptsStub
			},
		}

		initialBalance := ownerAcc.GetBalance([]byte("KLV"), false)
		marketOwnerAmount := int64(850000) // Amount after referral and royalties

		status, err := marketKApp.ComputeMarketOwnerAmount(ctx, marketOrder, []byte("KLV"), marketOwnerAmount)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify balance increased
		updatedAcc, err := accCacher.LoadUser(ownerAddr)
		require.NoError(t, err)
		finalBalance := updatedAcc.GetBalance([]byte("KLV"), false)
		require.Equal(t, initialBalance+marketOwnerAmount, finalBalance)

		// Verify Transfer receipt was added
		require.Len(t, receiptsStub.Get(), 1, "Expected 1 Transfer receipt")
	})
}

func TestMarketKApp_CreateMarketplace(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load market kapp account to initialize it
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	t.Run("NilName", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.CreateMarketplaceContract{
			Name: nil,
		}

		status, err := marketKApp.CreateMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("InvalidUTF8Name", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.CreateMarketplaceContract{
			Name: []byte{0xff, 0xfe}, // Invalid UTF-8
		}

		status, err := marketKApp.CreateMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("InvalidReferralAddress", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.CreateMarketplaceContract{
			Name:            []byte("Test Marketplace"),
			ReferralAddress: []byte("short"), // Invalid length
		}

		status, err := marketKApp.CreateMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("InvalidReferralPercentage", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.CreateMarketplaceContract{
			Name:               []byte("Test Marketplace"),
			ReferralPercentage: 10001, // > 100%
		}

		status, err := marketKApp.CreateMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("Success", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		returnData := make([][]byte, 0)
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			TxNonceCalled:    func() uint64 { return 1 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
			BlockCalled: func() *block.Block {
				return &block.Block{Header: &block.BlockHeader{RandSeed: []byte("randseed")}}
			},
			SetReturnDataCalled: func(data [][]byte) {
				returnData = data
			},
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.CreateMarketplaceContract{
			Name:               []byte("Test Marketplace"),
			ReferralPercentage: 500, // 5%
		}

		status, err := marketKApp.CreateMarketplace(defaultAddr, tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify marketplace ID was returned
		require.Len(t, returnData, 1)
		require.NotEmpty(t, returnData[0])

		// Verify receipt was added
		require.Len(t, receiptsStub.Get(), 1)
	})
}

func TestMarketKApp_ConfigMarketplace(t *testing.T) {
	t.Parallel()

	marketKApp, accCacher, _ := createTestMarketKApp(t)

	// Load and setup market kapp account
	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)

	// Create an existing marketplace
	existingMarketplace := &kapps.Marketplace{
		ID:                 []byte("existing-marketplace"),
		OwnerAddress:       defaultAddr,
		Name:               []byte("Existing Marketplace"),
		ReferralAddress:    defaultAddr,
		ReferralPercentage: 500,
	}
	err = marketKApp.SetMarketplace(marketKappAcc, existingMarketplace)
	require.NoError(t, err)
	err = accCacher.UpdateKapp(marketKappAcc)
	require.NoError(t, err)

	t.Run("InvalidReferralAddress", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID:   existingMarketplace.ID,
			ReferralAddress: []byte("short"), // Invalid length
		}

		status, err := marketKApp.ConfigMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("InvalidReferralPercentage", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID:      existingMarketplace.ID,
			ReferralPercentage: 10001, // > 100%
		}

		status, err := marketKApp.ConfigMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("MarketplaceNotFound", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID: []byte("nonexistent"),
		}

		status, err := marketKApp.ConfigMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("NotOwner", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID: existingMarketplace.ID,
		}

		// Use different sender (not owner)
		status, err := marketKApp.ConfigMarketplace(defaultOther, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("InvalidName", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID: existingMarketplace.ID,
			Name:          []byte{0xff, 0xfe}, // Invalid UTF-8
		}

		status, err := marketKApp.ConfigMarketplace(defaultAddr, tc)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	})

	t.Run("Success", func(t *testing.T) {
		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		}
		_ = marketKApp.SetKAppController(controllerStub)

		tc := &transaction.ConfigMarketplaceContract{
			MarketplaceID:      existingMarketplace.ID,
			Name:               []byte("Updated Marketplace"),
			ReferralPercentage: 1000, // 10%
		}

		status, err := marketKApp.ConfigMarketplace(defaultAddr, tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		// Verify receipt was added
		require.Len(t, receiptsStub.Get(), 1)

		// Verify marketplace was updated
		_, updatedMarketplace, err := marketKApp.GetMarketplace(existingMarketplace.ID)
		require.NoError(t, err)
		require.Equal(t, []byte("Updated Marketplace"), updatedMarketplace.Name)
		require.Equal(t, uint32(1000), updatedMarketplace.ReferralPercentage)
	})
}

func TestMarketKApp_ExecuteBuyMarket_RoyaltyReferralInflation(t *testing.T) {
	t.Parallel()

	const bid = int64(25600000000000)

	collectionID := []byte("NFLATION-ESGO")
	assetID := []byte("1")
	marketplaceID := []byte("41dc8c7826c6840e")

	buildScenario := func(t *testing.T, fixEnabled bool) (*marketKapp, state.AccountsCacher, state.KAppAccountHandler, state.UserAccountHandler, *kapps.MarketOrderData) {
		marketKApp, accCacher, forkController := createTestMarketKApp(t)
		forkController.FixMarketBuyOverflowValue = fixEnabled

		attacker := defaultAddr
		bidder := defaultOther

		attackerAcc, err := accCacher.LoadUser(attacker)
		require.NoError(t, err)
		require.NoError(t, accCacher.UpdateUser(attackerAcc))

		bidderAcc, err := accCacher.LoadUser(bidder)
		require.NoError(t, err)
		require.NoError(t, accCacher.UpdateUser(bidderAcc))

		marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)

		marketplace := &kapps.Marketplace{
			ID:                 marketplaceID,
			OwnerAddress:       attacker,
			Name:               []byte("Inflation Market"),
			ReferralAddress:    attacker,
			ReferralPercentage: core.HundredPercent,
		}
		require.NoError(t, marketKApp.SetMarketplace(marketKappAcc, marketplace))
		require.NoError(t, marketKappAcc.AddInternalKDA(collectionID, assetID, []byte("nft-data")))
		require.NoError(t, accCacher.UpdateKapp(marketKappAcc))

		asset := &kapps.KDAData{
			OwnerAddress: attacker,
			Royalties: &kapps.RoyaltiesData{
				Address:          attacker,
				MarketPercentage: core.HundredPercent,
				SplitRoyalties:   make(map[string]*kapps.RoyaltySplitData),
			},
		}

		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		controllerStub := &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
			GetKDAKAppCalled: func() kapp.KDAKapp {
				return &stub.KDAKappStub{
					GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
						return nil, asset, nil
					},
				}
			},
		}
		require.NoError(t, marketKApp.SetKAppController(controllerStub))

		order := &kapps.MarketOrderData{
			ID:                 []byte("f0a545db033c07b2"),
			MarketplaceID:      marketplaceID,
			MarketType:         kapps.MarketOrderData_BuyItNow,
			OwnerAddress:       attacker,
			CollectionID:       collectionID,
			AssetID:            assetID,
			CurrencyID:         kdautils.KLVIdentifier,
			Price:              bid,
			ReferralPercentage: core.HundredPercent,
			CurrentBid:         bid,
			CurrentBidder:      bidder,
		}

		return marketKApp, accCacher, marketKappAcc, bidderAcc, order
	}

	t.Run("FixDisabled_MintsKLVFromThinAir", func(t *testing.T) {
		marketKApp, accCacher, marketKappAcc, bidderAcc, order := buildScenario(t, false)

		status, err := marketKApp.ExecuteBuyMarket(bidderAcc, marketKappAcc, order, kdautils.KLVIdentifier)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		attackerAcc, err := accCacher.LoadUser(defaultAddr)
		require.NoError(t, err)
		// Buyer paid `bid` once, but the attacker collected the bid twice
		// (referral 100% + royalties 100%): 2*bid credited with nothing debited.
		require.Equal(t, int64(2*bid), attackerAcc.GetBalance(kdautils.KLVIdentifier, false))
	})

	t.Run("FixEnabled_RejectsInflation", func(t *testing.T) {
		marketKApp, accCacher, marketKappAcc, bidderAcc, order := buildScenario(t, true)

		status, err := marketKApp.ExecuteBuyMarket(bidderAcc, marketKappAcc, order, kdautils.KLVIdentifier)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_AmountInvalid, status)

		attackerAcc, err := accCacher.LoadUser(defaultAddr)
		require.NoError(t, err)
		require.Equal(t, int64(0), attackerAcc.GetBalance(kdautils.KLVIdentifier, false))
	})
}

func TestMarketKApp_ComputeSplitRoyalties_OverflowGuard(t *testing.T) {
	const pool = int64(1000000)
	const overflowPct = int64(0x80000000)

	run := func(t *testing.T, fixActive bool) (transaction.Transaction_TXResultCode, error, int64, int64) {
		marketKApp, accCacher, fc := createTestMarketKApp(t)
		fc.FixMarketBuyOverflowValue = fixActive

		recipient := makeAddress("splitRcpt")
		acc, err := accCacher.LoadUser(recipient)
		require.NoError(t, err)
		require.NoError(t, accCacher.UpdateUser(acc))

		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 1 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		order := &kapps.MarketOrderData{ID: []byte("order-1"), MarketplaceID: []byte("mp-1")}

		royaltiesToPay := pool
		status, err := marketKApp.ComputeSplitRoyalties(ctx, hex.EncodeToString(recipient), order,
			kdautils.KLVIdentifier, pool, overflowPct, &royaltiesToPay)

		bal := int64(0)
		if a, e := accCacher.LoadUser(recipient); e == nil {
			bal = a.GetBalance(kdautils.KLVIdentifier, false)
		}
		return status, err, bal, royaltiesToPay
	}

	t.Run("PreFork_StillMints", func(t *testing.T) {
		status, err, bal, rtp := run(t, false)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Greater(t, bal, pool, "pre-fork: split recipient over-paid (mint)")
		require.Less(t, rtp, int64(0))
	})

	t.Run("PostFork_Rejected", func(t *testing.T) {
		status, err, bal, rtp := run(t, true)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		require.Equal(t, int64(0), bal, "post-fork: nothing credited")
		require.Equal(t, pool, rtp)
	})
}

func TestMarketKApp_ExecuteBuyMarket_SplitOverflowGuard(t *testing.T) {
	const bid = int64(1000000)
	collectionID := []byte("OVF-COLL")
	assetID := []byte("1")
	marketplaceID := []byte("mp-ovf")

	run := func(t *testing.T, fixActive bool) (transaction.Transaction_TXResultCode, error, int64) {
		marketKApp, accCacher, fc := createTestMarketKApp(t)
		fc.FixMarketBuyOverflowValue = fixActive

		seller := defaultAddr
		bidder := defaultOther
		splitRecipient := makeAddress("attackerSplit")
		for _, addr := range [][]byte{seller, bidder, splitRecipient} {
			acc, err := accCacher.LoadUser(addr)
			require.NoError(t, err)
			require.NoError(t, accCacher.UpdateUser(acc))
		}

		marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)
		require.NoError(t, marketKApp.SetMarketplace(marketKappAcc, &kapps.Marketplace{
			ID: marketplaceID, OwnerAddress: seller, Name: []byte("ovf"),
			ReferralAddress: seller, ReferralPercentage: 0,
		}))
		require.NoError(t, marketKappAcc.AddInternalKDA(collectionID, assetID, []byte("nft")))
		require.NoError(t, accCacher.UpdateKapp(marketKappAcc))

		asset := &kapps.KDAData{
			OwnerAddress: seller,
			Royalties: &kapps.RoyaltiesData{
				Address:          seller,
				MarketPercentage: 1000,
				SplitRoyalties: map[string]*kapps.RoyaltySplitData{
					hex.EncodeToString(splitRecipient): {PercentMarketPercentage: 0x80000000},
				},
			},
		}

		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
		}
		require.NoError(t, marketKApp.SetKAppController(&stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
			GetKDAKAppCalled: func() kapp.KDAKapp {
				return &stub.KDAKappStub{
					GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
						return nil, asset, nil
					},
				}
			},
		}))

		order := &kapps.MarketOrderData{
			ID: []byte("order-ovf"), MarketplaceID: marketplaceID,
			MarketType: kapps.MarketOrderData_BuyItNow, OwnerAddress: seller,
			CollectionID: collectionID, AssetID: assetID, CurrencyID: kdautils.KLVIdentifier,
			Price: bid, ReferralPercentage: 0, CurrentBid: bid, CurrentBidder: bidder,
		}
		bidderAcc, err := accCacher.LoadUser(bidder)
		require.NoError(t, err)

		status, err := marketKApp.ExecuteBuyMarket(bidderAcc, marketKappAcc, order, kdautils.KLVIdentifier)
		splitBal := int64(0)
		if a, e := accCacher.LoadUser(splitRecipient); e == nil {
			splitBal = a.GetBalance(kdautils.KLVIdentifier, false)
		}
		return status, err, splitBal
	}

	t.Run("PreFork_MintsDespiteOldGuard", func(t *testing.T) {
		status, err, splitBal := run(t, false)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Greater(t, splitBal, bid, "pre-fork: split overflow mints > bid even though marketOwnerAmount >= 0")
	})

	t.Run("PostFork_Rejected", func(t *testing.T) {
		status, err, splitBal := run(t, true)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		require.Equal(t, int64(0), splitBal, "post-fork: no mint, buy rejected")
	})
}

func TestMarketKApp_ExecuteBuyMarket_MultiRecipientGuardMidLoop(t *testing.T) {
	const bid = int64(1000000)
	collectionID := []byte("MR-COLL")
	assetID := []byte("1")
	marketplaceID := []byte("mp-mr")

	marketKApp, accCacher, fc := createTestMarketKApp(t)
	fc.FixMarketBuyOverflowValue = true

	seller := defaultAddr
	bidder := defaultOther
	r1 := makeAddress("split-r1")
	r2 := makeAddress("split-r2")
	for _, addr := range [][]byte{seller, bidder, r1, r2} {
		acc, err := accCacher.LoadUser(addr)
		require.NoError(t, err)
		require.NoError(t, accCacher.UpdateUser(acc))
	}

	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)
	require.NoError(t, marketKApp.SetMarketplace(marketKappAcc, &kapps.Marketplace{
		ID: marketplaceID, OwnerAddress: seller, Name: []byte("mr"),
		ReferralAddress: seller, ReferralPercentage: 0,
	}))
	require.NoError(t, marketKappAcc.AddInternalKDA(collectionID, assetID, []byte("nft")))
	require.NoError(t, accCacher.UpdateKapp(marketKappAcc))

	asset := &kapps.KDAData{
		OwnerAddress: seller,
		Royalties: &kapps.RoyaltiesData{
			Address:          seller,
			MarketPercentage: 1000,
			SplitRoyalties: map[string]*kapps.RoyaltySplitData{
				hex.EncodeToString(r1): {PercentMarketPercentage: 6000},
				hex.EncodeToString(r2): {PercentMarketPercentage: 6000},
			},
		},
	}

	receiptsStub := mock.NewReceiptsContextStub()
	ctx := &mock.KAppContextStub{
		ContractIDCalled: func() int { return 0 },
		ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
	}
	require.NoError(t, marketKApp.SetKAppController(&stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, asset, nil
				},
			}
		},
	}))

	order := &kapps.MarketOrderData{
		ID: []byte("order-mr"), MarketplaceID: marketplaceID,
		MarketType: kapps.MarketOrderData_BuyItNow, OwnerAddress: seller,
		CollectionID: collectionID, AssetID: assetID, CurrencyID: kdautils.KLVIdentifier,
		Price: bid, ReferralPercentage: 0, CurrentBid: bid, CurrentBidder: bidder,
	}
	bidderAcc, err := accCacher.LoadUser(bidder)
	require.NoError(t, err)

	status, err := marketKApp.ExecuteBuyMarket(bidderAcc, marketKappAcc, order, kdautils.KLVIdentifier)
	require.Error(t, err)
	require.Equal(t, transaction.Transaction_ParameterInvalid, status)

	royaltyPool := bid * 1000 / int64(core.HundredPercent)
	expectedOne := royaltyPool * 6000 / int64(core.HundredPercent)
	r1acc, err := accCacher.LoadUser(r1)
	require.NoError(t, err)
	r2acc, err := accCacher.LoadUser(r2)
	require.NoError(t, err)
	credited := r1acc.GetBalance(kdautils.KLVIdentifier, false) + r2acc.GetBalance(kdautils.KLVIdentifier, false)
	require.Equal(t, expectedOne, credited)
}
