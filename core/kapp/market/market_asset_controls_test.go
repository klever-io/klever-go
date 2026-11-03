package market

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/require"
)

// A sale of ctrlBid with a 5% referral and a 10% royalty split half to ctrlSplit and half to
// ctrlRoyalty pays 50,000 to each of the referral, the split and the royalty address, and
// 850,000 to the seller.
const (
	ctrlBid       = int64(1_000_000)
	ctrlFunds     = int64(10_000_000)
	ctrlBlockTime = int64(1_700_000_000)
	ctrlLeg       = int64(50_000)
	ctrlSellerCut = int64(850_000)
)

var (
	ctrlCurrencyID    = []byte("RST-1234")
	ctrlCollectionID  = []byte("NFTC-1234")
	ctrlAssetID       = []byte("7")
	ctrlMarketplaceID = []byte("mp-controls")
	ctrlOrderID       = []byte("order-controls")
	// ctrlUnreadableID is an asset whose KDA record fails to load with errCtrlUnreadable.
	ctrlUnreadableID  = []byte("BROKEN-1234")
	errCtrlUnreadable = errors.New("kda record unreadable")
	ctrlIssuer        = makeAddress("issuer")
	ctrlSeller        = makeAddress("seller")
	ctrlBuyer         = makeAddress("buyer")
	ctrlPrevBidder    = makeAddress("prev-bidder")
	ctrlReferral      = makeAddress("referral")
	ctrlRoyalty       = makeAddress("royalty")
	ctrlSplit         = makeAddress("split")
	ctrlPayees        = [][]byte{ctrlReferral, ctrlSplit, ctrlRoyalty, ctrlSeller}
)

func controlsCurrency(paused bool, limitTransfer bool, roles ...*kapps.RolesData) *kapps.KDAData {
	return &kapps.KDAData{
		ID:           ctrlCurrencyID,
		OwnerAddress: ctrlIssuer,
		AssetType:    kapps.KDAData_Fungible,
		Attributes:   &kapps.AttributesData{IsPaused: paused},
		Properties:   &kapps.PropertiesData{LimitTransfer: limitTransfer},
		Roles:        roles,
	}
}

func controlsCollection(paused bool, limitTransfer bool, roles ...*kapps.RolesData) *kapps.KDAData {
	return &kapps.KDAData{
		ID:           ctrlCollectionID,
		OwnerAddress: ctrlIssuer,
		AssetType:    kapps.KDAData_NonFungible,
		Attributes:   &kapps.AttributesData{IsPaused: paused},
		Properties:   &kapps.PropertiesData{LimitTransfer: limitTransfer},
		Roles:        roles,
		Royalties: &kapps.RoyaltiesData{
			Address:          ctrlRoyalty,
			MarketPercentage: 1000,
			SplitRoyalties: map[string]*kapps.RoyaltySplitData{
				hex.EncodeToString(ctrlSplit): {PercentMarketPercentage: 5000},
			},
		},
	}
}

func depositRole(address []byte) *kapps.RolesData {
	return &kapps.RolesData{Address: address, HasRoleDeposit: true}
}

func transferRole(address []byte) *kapps.RolesData {
	return &kapps.RolesData{Address: address, HasRoleTransfer: true}
}

func depositRolesExcept(skip []byte) []*kapps.RolesData {
	roles := make([]*kapps.RolesData, 0, len(ctrlPayees))
	for _, payee := range ctrlPayees {
		if !bytes.Equal(payee, skip) {
			roles = append(roles, depositRole(payee))
		}
	}
	return roles
}

type controlsEnv struct {
	t          *testing.T
	market     *marketKapp
	accCacher  state.AccountsCacher
	currency   *kapps.KDAData
	collection *kapps.KDAData
	blockTime  int64
	lookups    map[string]int
	receipts   *mock.ReceiptsContextStub
	// txCtx, once a test enters a transaction (inTx), is the real KappContext the market reads in
	// place of the stub, as txProcess builds one per transaction.
	txCtx kapp.KappContext
}

func newControlsEnv(t *testing.T, fixOn bool, currency *kapps.KDAData, collection *kapps.KDAData) *controlsEnv {
	t.Helper()

	marketKApp, accCacher, fc := createTestMarketKApp(t)
	fc.FixAuditChangesV6Value = fixOn
	// V7 is not active on mainnet yet, so the controls run with V6 alone; the V7 tests turn it on
	// (see newV7ControlsEnv).
	fc.FixAuditChangesV7Value = false

	env := &controlsEnv{
		t: t, market: marketKApp, accCacher: accCacher,
		currency: currency, collection: collection,
		blockTime: ctrlBlockTime, lookups: make(map[string]int),
	}

	for _, addr := range [][]byte{ctrlIssuer, ctrlSeller, ctrlBuyer, ctrlPrevBidder, ctrlReferral, ctrlRoyalty, ctrlSplit} {
		acc, err := accCacher.LoadUser(addr)
		require.NoError(t, err)
		if bytes.Equal(addr, ctrlBuyer) || bytes.Equal(addr, ctrlPrevBidder) {
			require.NoError(t, acc.AddToBalance(ctrlFunds, currency.ID, true))
		}
		require.NoError(t, accCacher.UpdateUser(acc))
	}

	marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(t, err)
	require.NoError(t, marketKApp.SetMarketplace(marketKappAcc, &kapps.Marketplace{
		ID: ctrlMarketplaceID, OwnerAddress: ctrlSeller, Name: []byte("controls"),
		ReferralAddress: ctrlReferral, ReferralPercentage: 500,
	}))
	require.NoError(t, accCacher.UpdateKapp(marketKappAcc))

	receipts := mock.NewReceiptsContextStub()
	env.receipts = receipts
	ctx := &mock.KAppContextStub{
		ContractIDCalled:    func() int { return 0 },
		TxNonceCalled:       func() uint64 { return 1 },
		ReceiptsCalled:      func() kapp.ReceiptsContext { return receipts },
		SetReturnDataCalled: func(_ [][]byte) {},
		BlockCalled: func() *block.Block {
			return &block.Block{Header: &block.BlockHeader{Timestamp: env.blockTime, RandSeed: []byte("seed")}}
		},
	}
	require.NoError(t, marketKApp.SetKAppController(&stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			if env.txCtx != nil {
				return env.txCtx
			}
			return ctx
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(id []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					env.lookups[string(id)]++
					switch {
					case bytes.Equal(id, env.currency.ID):
						return nil, env.currency, nil
					case bytes.Equal(id, ctrlCollectionID):
						return nil, env.collection, nil
					case bytes.Equal(id, ctrlUnreadableID):
						return nil, nil, errCtrlUnreadable
					}
					return nil, nil, common.ErrAssetNotFound
				},
			}
		},
	}))

	return env
}

// storeOrder escrows the NFT and stores order with referralPercentage and the 10% royalty snapshotted.
func (e *controlsEnv) storeOrder(order *kapps.MarketOrderData, referralPercentage uint32) {
	e.t.Helper()

	e.storeOrderWith(order, referralPercentage, true)
}

// storeOrderWith is storeOrder with the referral and royalty snapshots set to snapshotted. An
// order listed before FixAuditChangesV4 has none, and its settlement reads the live
// marketplace referral address and the live collection royalty.
func (e *controlsEnv) storeOrderWith(order *kapps.MarketOrderData, referralPercentage uint32, snapshotted bool) {
	e.t.Helper()

	order.ID = ctrlOrderID
	order.MarketplaceID = ctrlMarketplaceID
	order.OwnerAddress = ctrlSeller
	order.CollectionID = ctrlCollectionID
	order.AssetID = ctrlAssetID
	order.CurrencyID = e.currency.ID
	order.ReferralPercentage = referralPercentage
	order.ReferralAddress = ctrlReferral
	order.ReferralSnapshotted = snapshotted
	order.RoyaltySnapshotted = snapshotted
	if snapshotted {
		order.RoyaltyPercentage = 1000
	}
	if order.EndTime == 0 {
		order.EndTime = ctrlBlockTime + 1000
	}

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	require.NoError(e.t, marketKappAcc.AddInternalKDA(ctrlCollectionID, ctrlAssetID, mock.NFTHolderRecord(1)))
	require.NoError(e.t, e.market.SetMarketOrder(marketKappAcc, order))
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

func (e *controlsEnv) storeBuyItNow() {
	e.storeOrder(&kapps.MarketOrderData{MarketType: kapps.MarketOrderData_BuyItNow, Price: ctrlBid}, 500)
}

func (e *controlsEnv) storeAuction(endTime int64, currentBid int64, currentBidder []byte) {
	e.storeOrder(&kapps.MarketOrderData{
		MarketType: kapps.MarketOrderData_Auction, ReservePrice: ctrlBid / 2,
		EndTime: endTime, CurrentBid: currentBid, CurrentBidder: currentBidder,
	}, 500)
}

func (e *controlsEnv) buy(sender []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	return e.market.Buy(sender, &transaction.BuyContract{
		BuyType: transaction.BuyContract_MarketBuy, ID: ctrlOrderID, CurrencyID: e.currency.ID, Amount: amount,
	})
}

func (e *controlsEnv) claim(sender []byte) (transaction.Transaction_TXResultCode, error) {
	return e.market.Claim(sender, &transaction.ClaimContract{ClaimType: transaction.ClaimContract_MarketClaim, ID: ctrlOrderID})
}

func (e *controlsEnv) cancel() (transaction.Transaction_TXResultCode, error) {
	return e.market.CancelOrder(ctrlSeller, &transaction.CancelMarketOrderContract{OrderID: ctrlOrderID})
}

func (e *controlsEnv) sell() (transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	return e.sellIn(e.currency.ID)
}

// sellIn lists the seller's NFT as an auction priced in currencyID.
func (e *controlsEnv) sellIn(currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	sellerAcc, err := e.accCacher.LoadUser(ctrlSeller)
	require.NoError(e.t, err)
	require.NoError(e.t, sellerAcc.AddInternalKDA(ctrlCollectionID, ctrlAssetID, mock.NFTHolderRecord(1)))
	require.NoError(e.t, e.accCacher.UpdateUser(sellerAcc))

	assetID := append(append(append([]byte{}, ctrlCollectionID...), []byte(kapps.Sp)...), ctrlAssetID...)
	return e.market.Sell(ctrlSeller, &transaction.SellContract{
		MarketType: transaction.SellContract_EnumMarketType(kapps.MarketOrderData_Auction), MarketplaceID: ctrlMarketplaceID,
		AssetID: assetID, CurrencyID: currencyID, ReservePrice: ctrlBid / 2, EndTime: ctrlBlockTime + 1000,
	})
}

func (e *controlsEnv) balance(addr []byte) int64 {
	e.t.Helper()

	acc, err := e.accCacher.LoadUser(addr)
	require.NoError(e.t, err)
	return acc.GetBalance(e.currency.ID, true)
}

func (e *controlsEnv) holdsNFT(addr []byte) bool {
	e.t.Helper()

	var tracker state.DataTrieTracker
	if bytes.Equal(addr, kapps.MarketKAppAddress) {
		acc, err := e.accCacher.LoadKApp(addr)
		require.NoError(e.t, err)
		tracker = acc.DataTrieTracker()
	} else {
		acc, err := e.accCacher.LoadUser(addr)
		require.NoError(e.t, err)
		tracker = acc.DataTrieTracker()
	}
	value, err := tracker.RetrieveValue(kdautils.ToKDAKey(ctrlCollectionID, ctrlAssetID))
	// A key never written reads ErrNilTrie on a fresh account, and a key cleared in this
	// block reads back as an empty dirty value, which the trim reports as ErrNegativeValue.
	if errors.Is(err, common.ErrNilTrie) || errors.Is(err, common.ErrNegativeValue) {
		return false
	}
	require.NoError(e.t, err)
	return len(value) > 0
}

func (e *controlsEnv) storedOrder() *kapps.MarketOrderData {
	e.t.Helper()

	_, order, err := e.market.GetMarketOrder(ctrlOrderID)
	require.NoError(e.t, err)
	return order
}

// requireUnsettled checks nothing moved: the NFT is still escrowed and no payee was paid.
func (e *controlsEnv) requireUnsettled(buyerBalance int64) {
	e.t.Helper()

	require.True(e.t, e.holdsNFT(kapps.MarketKAppAddress), "the NFT must stay escrowed")
	require.False(e.t, e.holdsNFT(ctrlBuyer))
	require.Equal(e.t, buyerBalance, e.balance(ctrlBuyer))
	for _, payee := range ctrlPayees {
		require.Zero(e.t, e.balance(payee))
	}
}

// requireSettled checks the full sale of ctrlBid to the buyer.
func (e *controlsEnv) requireSettled(status transaction.Transaction_TXResultCode, err error, buyerBalance int64) {
	e.t.Helper()

	require.NoError(e.t, err)
	require.Equal(e.t, transaction.Transaction_Ok, status)
	require.True(e.t, e.holdsNFT(ctrlBuyer))
	require.False(e.t, e.holdsNFT(kapps.MarketKAppAddress))
	require.Equal(e.t, buyerBalance, e.balance(ctrlBuyer))
	require.Equal(e.t, ctrlLeg, e.balance(ctrlReferral))
	require.Equal(e.t, ctrlLeg, e.balance(ctrlSplit))
	require.Equal(e.t, ctrlLeg, e.balance(ctrlRoyalty))
	require.Equal(e.t, ctrlSellerCut, e.balance(ctrlSeller))
}

func requirePaused(t *testing.T, status transaction.Transaction_TXResultCode, err error) {
	t.Helper()

	require.ErrorIs(t, err, process.ErrAssetIsPaused)
	require.Equal(t, transaction.Transaction_AssetPaused, status)
}

func requireNotAllowed(t *testing.T, status transaction.Transaction_TXResultCode, err error) {
	t.Helper()

	require.ErrorIs(t, err, process.ErrKDATransferNotAllowed)
	require.Equal(t, transaction.Transaction_KDATransferNotAllowed, status)
}

// requireTransferRefusedReceipt checks a refused transfer failed with its single error receipt.
func (e *controlsEnv) requireTransferRefusedReceipt() {
	e.t.Helper()

	errorReceipts := e.receipts.GetByType(int8(kapp.ReceiptTypeError))
	require.Len(e.t, errorReceipts, 1)
	require.Equal(e.t, []byte(common.ErrFieldTransferNotAllowed), errorReceipts[0].Data[1])
}

// requirePausedReceipt checks a paused asset failed with its single error receipt.
func (e *controlsEnv) requirePausedReceipt() {
	e.t.Helper()

	errorReceipts := e.receipts.GetByType(int8(kapp.ReceiptTypeError))
	require.Len(e.t, errorReceipts, 1)
	require.Equal(e.t, []byte(common.ErrFieldAssetPaused), errorReceipts[0].Data[1])
}

func TestMarketAssetControls_PausedCurrency(t *testing.T) {
	t.Parallel()

	t.Run("BuyItNowRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requirePaused(t, status, err)
		env.requirePausedReceipt()
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("AuctionBidRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requirePaused(t, status, err)
		env.requirePausedReceipt()
		env.requireUnsettled(ctrlFunds)
		require.Empty(t, env.storedOrder().CurrentBidder)
	})

	t.Run("FixOff_BuyItNowSettlesWithoutReadingTheCurrency", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
		require.Zero(t, env.lookups[string(ctrlCurrencyID)], "pre-fork must not load the currency")
	})

	t.Run("FixOff_AuctionBidAcceptedWithoutAnyKDALookup", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, ctrlFunds-ctrlBid, env.balance(ctrlBuyer))
		require.Empty(t, env.lookups, "a pre-fork bid must not load any KDA")
	})
}

func TestMarketAssetControls_LimitTransferCurrency(t *testing.T) {
	t.Parallel()

	refused := map[string]func() *kapps.KDAData{
		"RolelessBuyer":        func() *kapps.KDAData { return controlsCurrency(false, true) },
		"ReferralLacksDeposit": func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlReferral)...) },
		"SplitLacksDeposit":    func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlSplit)...) },
		"RoyaltyLacksDeposit":  func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlRoyalty)...) },
		"SellerLacksDeposit":   func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlSeller)...) },
	}
	for name, currency := range refused {
		t.Run(name+"_BuyItNowRefused", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, currency(), controlsCollection(false, false))
			env.storeBuyItNow()

			status, err := env.buy(ctrlBuyer, ctrlBid)
			requireNotAllowed(t, status, err)
			env.requireUnsettled(ctrlFunds)
		})

		t.Run(name+"_AuctionBidRefused", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, currency(), controlsCollection(false, false))
			env.storeAuction(0, 0, nil)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			requireNotAllowed(t, status, err)
			env.requireUnsettled(ctrlFunds)
		})

		t.Run(name+"_FixOff_Settles", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, currency(), controlsCollection(false, false))
			env.storeBuyItNow()

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireSettled(status, err, ctrlFunds-ctrlBid)
		})
	}

	allowed := map[string]func() *kapps.KDAData{
		"BuyerHasRoleTransfer":     func() *kapps.KDAData { return controlsCurrency(false, true, transferRole(ctrlBuyer)) },
		"EveryPayeeHasRoleDeposit": func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(nil)...) },
	}
	for name, currency := range allowed {
		t.Run(name+"_BuyItNowSettles", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, currency(), controlsCollection(false, false))
			env.storeBuyItNow()

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireSettled(status, err, ctrlFunds-ctrlBid)
		})
	}

	// A leg paid nothing is not a transfer: a 0% referral to an address without HasRoleDeposit
	// must not refuse the sale.
	t.Run("ZeroReferralLegIsNotChecked", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, true, depositRolesExcept(ctrlReferral)...), controlsCollection(false, false))
		env.storeOrder(&kapps.MarketOrderData{MarketType: kapps.MarketOrderData_BuyItNow, Price: ctrlBid}, 0)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Zero(t, env.balance(ctrlReferral))
		require.Equal(t, ctrlSellerCut+ctrlLeg, env.balance(ctrlSeller))
		require.True(t, env.holdsNFT(ctrlBuyer))
	})

	// The refund to an outbid bidder returns their own escrowed bid and is never checked, even
	// when that bidder could no longer receive the currency from anyone.
	t.Run("OutbidRefundIsNotBlocked", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, true, transferRole(ctrlBuyer)), controlsCollection(false, false))
		env.storeAuction(0, ctrlBid/2, ctrlPrevBidder)

		status, err := env.buy(ctrlBuyer, ctrlBid-1)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, ctrlFunds+ctrlBid/2, env.balance(ctrlPrevBidder))
		require.Equal(t, ctrlFunds-(ctrlBid-1), env.balance(ctrlBuyer))
		require.Equal(t, ctrlBuyer, env.storedOrder().CurrentBidder)
	})
}

func TestMarketAssetControls_PausedCollection(t *testing.T) {
	t.Parallel()

	t.Run("SellRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(true, false))
		status, err := env.sell()
		requirePaused(t, status, err)
		env.requirePausedReceipt()
		require.True(t, env.holdsNFT(ctrlSeller))
	})

	t.Run("FixOff_SellAccepted", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(true, false))
		status, err := env.sell()
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.False(t, env.holdsNFT(ctrlSeller))
	})

	t.Run("BuyItNowRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requirePaused(t, status, err)
		env.requirePausedReceipt()
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("AuctionBidRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requirePaused(t, status, err)
		env.requirePausedReceipt()
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("ExpiredClaimUnwinds", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeExpiredBid(true, ctrlFixedDeposit)

		status, err := env.claim(ctrlBuyer)
		env.requireUnwound(status, err, ctrlFixedDeposit)
	})

	t.Run("FixOff_SettlingClaimSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeAuction(ctrlBlockTime-1, ctrlBid, ctrlBuyer)

		status, err := env.claim(ctrlBuyer)
		env.requireSettled(status, err, ctrlFunds)
		require.Zero(t, env.lookups[string(ctrlCurrencyID)], "pre-fork must not load the currency")
	})
}

func TestMarketAssetControls_LimitTransferCollection(t *testing.T) {
	t.Parallel()

	t.Run("RolelessSellerToRolelessBuyer_BidRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requireNotAllowed(t, status, err)
		env.requireUnsettled(ctrlFunds)
	})

	// The seller's early accept settles through executeBuyMarket and is refused. Once the
	// auction expires the Claim unwinds instead (TestMarketAssetControls_ExpiredAuctionUnwind).
	t.Run("RolelessSellerToRolelessBuyer_EarlyAcceptRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeAuction(0, ctrlBid, ctrlBuyer)

		status, err := env.claim(ctrlSeller)
		requireNotAllowed(t, status, err)
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("FixOff_SettlementSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeAuction(ctrlBlockTime-1, ctrlBid, ctrlBuyer)

		status, err := env.claim(ctrlSeller)
		env.requireSettled(status, err, ctrlFunds)
	})

	t.Run("SellerHasRoleTransfer_Settles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true, transferRole(ctrlSeller)))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
	})

	t.Run("BuyerHasRoleDeposit_Settles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true, depositRole(ctrlBuyer)))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
	})

	// The final buyer is unknown at Sell and the escrow is only an intermediary.
	t.Run("SellAllowed", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true))
		status, err := env.sell()
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
	})
}

// A pause applied after the order was listed or bid on refuses the sale and the settling
// Claim, but never the paths that return the NFT and the bid to their owners.
func TestMarketAssetControls_PausedAfterListing(t *testing.T) {
	t.Parallel()

	pauseCurrency := func(env *controlsEnv) { env.currency.Attributes.IsPaused = true }
	pauseCollection := func(env *controlsEnv) { env.collection.Attributes.IsPaused = true }

	for name, pause := range map[string]func(*controlsEnv){"Currency": pauseCurrency, "Collection": pauseCollection} {

		t.Run(name+"_BuyItNowRefused", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeBuyItNow()
			pause(env)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			requirePaused(t, status, err)
			env.requireUnsettled(ctrlFunds)
		})

		t.Run(name+"_AfterBid_ClaimRefusedAndCancelReturnsEverything", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(0, 0, nil)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.Equal(t, ctrlFunds-ctrlBid, env.balance(ctrlBuyer))

			pause(env)

			// A later bid is refused and the current bid stays in place.
			status, err = env.buy(ctrlPrevBidder, ctrlBid+1)
			requirePaused(t, status, err)
			require.Equal(t, ctrlFunds, env.balance(ctrlPrevBidder))

			// The seller's early accept settles through executeBuyMarket and is refused.
			status, err = env.claim(ctrlSeller)
			requirePaused(t, status, err)
			require.True(t, env.holdsNFT(kapps.MarketKAppAddress))

			// Cancelling returns the NFT to the seller and the bid to the bidder.
			status, err = env.cancel()
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.True(t, env.holdsNFT(ctrlSeller))
			require.Equal(t, ctrlFunds, env.balance(ctrlBuyer))
		})

		t.Run(name+"_EarlyAcceptRefusedWithItsReceipt", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(0, ctrlBid, ctrlBuyer)
			pause(env)

			status, err := env.claim(ctrlSeller)
			requirePaused(t, status, err)
			env.requirePausedReceipt()
			require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
		})

		t.Run(name+"_AfterBid_ExpiredClaimUnwinds", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(0, 0, nil)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)

			pause(env)
			env.blockTime = ctrlBlockTime + 2000

			// The pause would otherwise hold the other party's asset until the issuer lifts it.
			status, err = env.claim(ctrlBuyer)
			env.requireUnwound(status, err, 0)
		})

		t.Run(name+"_ReserveNotMet_ClaimReturnsEverything", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(ctrlBlockTime-1, ctrlBid/4, ctrlBuyer)
			pause(env)

			status, err := env.claim(ctrlSeller)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.True(t, env.holdsNFT(ctrlSeller))
			require.Equal(t, ctrlFunds+ctrlBid/4, env.balance(ctrlBuyer))
		})
	}
}

// Splits that add up to more than the royalty fail settlement under FixMarketBuyOverflow. Under
// FixAuditChangesV6 the settlement check computes the same shares, so it refuses with the same
// code before the first split is credited, and on a bid that could never settle.
func TestMarketAssetControls_SplitOverflowRefusedBeforeAnyCredit(t *testing.T) {
	t.Parallel()

	overflowing := func() *kapps.KDAData {
		collection := controlsCollection(false, false)
		collection.Royalties.SplitRoyalties = map[string]*kapps.RoyaltySplitData{
			hex.EncodeToString(ctrlSplit):   {PercentMarketPercentage: 6000},
			hex.EncodeToString(ctrlRoyalty): {PercentMarketPercentage: 6000},
		}
		return collection
	}

	t.Run("BuyItNow", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), overflowing())
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("AuctionBid", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), overflowing())
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("FixOff_AuctionBidAccepted", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), overflowing())
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})
}

// KLC-2739/KLC-2764: expired-auction unwind
//
// An expired auction whose reserve was met settles through Claim. Under FixAuditChangesV6 a
// sale a direct transfer would refuse cannot settle, and CancelOrder and Buy refuse an expired
// order, so such a Claim returns the NFT and the bid to their owners instead of failing, or
// they would stay in escrow forever. That happens when the bid was escrowed before the fork
// without any check, when a non-snapshotted order's live referral address changes after the
// bid, when the issuer changes a role after the bid, or when either asset is paused.

const ctrlFixedDeposit = int64(7_000)

var ctrlNewReferral = makeAddress("new-referral")

// storeExpiredBid stores an auction that expired with ctrlBuyer's bid of ctrlBid meeting the
// reserve, as a bid accepted before FixAuditChangesV6 left it: the bid is already escrowed
// (taken from the buyer) and no control was checked.
func (e *controlsEnv) storeExpiredBid(snapshotted bool, fixedDeposit int64) {
	e.t.Helper()

	e.storeOrderWith(&kapps.MarketOrderData{
		MarketType: kapps.MarketOrderData_Auction, ReservePrice: ctrlBid / 2, EndTime: ctrlBlockTime - 1,
		CurrentBid: ctrlBid, CurrentBidder: ctrlBuyer, RoyaltiesFixedDeposit: fixedDeposit,
	}, 500, snapshotted)

	buyerAcc, err := e.accCacher.LoadUser(ctrlBuyer)
	require.NoError(e.t, err)
	require.NoError(e.t, buyerAcc.SubFromBalance(ctrlBid, e.currency.ID, true))
	require.NoError(e.t, e.accCacher.UpdateUser(buyerAcc))
}

// setLiveReferral repoints the marketplace's referral address, which a non-snapshotted order
// pays at settlement.
func (e *controlsEnv) setLiveReferral(address []byte) {
	e.t.Helper()

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	require.NoError(e.t, e.market.SetMarketplace(marketKappAcc, &kapps.Marketplace{
		ID: ctrlMarketplaceID, OwnerAddress: ctrlSeller, Name: []byte("controls"),
		ReferralAddress: address, ReferralPercentage: 500,
	}))
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

func (e *controlsEnv) klvBalance(addr []byte) int64 {
	e.t.Helper()

	acc, err := e.accCacher.LoadUser(addr)
	require.NoError(e.t, err)
	return acc.GetBalance(kdautils.KLVIdentifier, true)
}

// requireUnwound checks the order was closed by returning everything: the NFT and the fixed
// deposit to the seller, the bid to the buyer, nothing to any payee, and no error receipt.
func (e *controlsEnv) requireUnwound(status transaction.Transaction_TXResultCode, err error, fixedDeposit int64) {
	e.t.Helper()

	require.NoError(e.t, err)
	require.Equal(e.t, transaction.Transaction_Ok, status)
	require.True(e.t, e.holdsNFT(ctrlSeller), "the NFT must return to the seller")
	require.False(e.t, e.holdsNFT(kapps.MarketKAppAddress))
	require.False(e.t, e.holdsNFT(ctrlBuyer))
	require.Equal(e.t, ctrlFunds, e.balance(ctrlBuyer), "the bid must return to the bidder")
	for _, payee := range ctrlPayees {
		require.Zero(e.t, e.balance(payee))
	}
	require.Equal(e.t, fixedDeposit, e.klvBalance(ctrlSeller), "the fixed deposit must return to the seller")
	require.Zero(e.t, e.klvBalance(ctrlRoyalty))
	require.True(e.t, e.storedOrder().IsClaimed)
	require.Empty(e.t, e.receipts.GetByType(int8(kapp.ReceiptTypeError)), "an unwind is not a failure")
}

// requireStillEscrowed checks a refused Claim moved nothing and left the order open.
func (e *controlsEnv) requireStillEscrowed() {
	e.t.Helper()

	e.requireUnsettled(ctrlFunds - ctrlBid)
	require.False(e.t, e.holdsNFT(ctrlSeller))
	require.Zero(e.t, e.klvBalance(ctrlSeller))
	require.False(e.t, e.storedOrder().IsClaimed)
}

// Sales a direct transfer would refuse, on a bid escrowed before the fork.
var ctrlRefusedSales = map[string]func() (*kapps.KDAData, *kapps.KDAData){
	"CurrencySellerLacksDeposit": func() (*kapps.KDAData, *kapps.KDAData) {
		return controlsCurrency(false, true, depositRolesExcept(ctrlSeller)...), controlsCollection(false, false)
	},
	"CurrencyRolelessBuyerAndPayees": func() (*kapps.KDAData, *kapps.KDAData) {
		return controlsCurrency(false, true), controlsCollection(false, false)
	},
	"CollectionRolelessSellerToRolelessBuyer": func() (*kapps.KDAData, *kapps.KDAData) {
		return controlsCurrency(false, false), controlsCollection(false, true)
	},
}

func TestMarketAssetControls_ExpiredAuctionUnwind(t *testing.T) {
	t.Parallel()

	claimants := map[string][]byte{"ClaimedBySeller": ctrlSeller, "ClaimedByBidder": ctrlBuyer}

	for name, assets := range ctrlRefusedSales {
		for claimantName, claimant := range claimants {
			t.Run(name+"_"+claimantName+"_Unwinds", func(t *testing.T) {
				t.Parallel()

				currency, collection := assets()
				env := newControlsEnv(t, true, currency, collection)
				env.storeExpiredBid(true, ctrlFixedDeposit)

				status, err := env.claim(claimant)
				env.requireUnwound(status, err, ctrlFixedDeposit)
			})
		}

		t.Run(name+"_FixOff_Settles", func(t *testing.T) {
			t.Parallel()

			currency, collection := assets()
			env := newControlsEnv(t, false, currency, collection)
			env.storeExpiredBid(true, ctrlFixedDeposit)

			status, err := env.claim(ctrlSeller)
			env.requireSettled(status, err, ctrlFunds-ctrlBid)
			require.Equal(t, ctrlFixedDeposit, env.klvBalance(ctrlRoyalty), "the fixed deposit is paid as a sale")
			require.Zero(t, env.klvBalance(ctrlSeller))
			require.True(t, env.storedOrder().IsClaimed)
			require.Zero(t, env.lookups[string(ctrlCurrencyID)], "pre-fork must not load the currency")
		})
	}

	// The unwind writes exactly the receipts the below-reserve Claim always has.
	t.Run("ReceiptsMatchTheBelowReserveClaim", func(t *testing.T) {
		t.Parallel()

		unwound := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, true))
		unwound.storeExpiredBid(true, ctrlFixedDeposit)
		status, err := unwound.claim(ctrlSeller)
		unwound.requireUnwound(status, err, ctrlFixedDeposit)

		belowReserve := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		belowReserve.storeExpiredBid(true, ctrlFixedDeposit)
		order := belowReserve.storedOrder()
		order.ReservePrice = ctrlBid + 1
		marketKappAcc, err := belowReserve.accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)
		require.NoError(t, belowReserve.market.SetMarketOrder(marketKappAcc, order))
		require.NoError(t, belowReserve.accCacher.UpdateKapp(marketKappAcc))
		status, err = belowReserve.claim(ctrlSeller)
		belowReserve.requireUnwound(status, err, ctrlFixedDeposit)

		require.Len(t, unwound.receipts.Get(), 4)
		require.Equal(t, belowReserve.receipts.Get(), unwound.receipts.Get())
	})
}

// A pause on either asset unwinds the expired Claim like a refused transfer, so the issuer of
// one asset cannot hold the other party's asset in escrow, and the unwind carries no paused
// receipt.
func TestMarketAssetControls_ExpiredAuctionPausedUnwinds(t *testing.T) {
	t.Parallel()

	pauses := map[string]func() (*kapps.KDAData, *kapps.KDAData){
		"CurrencyPaused": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(true, false), controlsCollection(false, false)
		},
		"CollectionPaused": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(false, false), controlsCollection(true, false)
		},
		"CurrencyPausedAndRestricted": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(true, true), controlsCollection(false, true)
		},
		"CollectionPausedAndRestricted": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(false, true), controlsCollection(true, true)
		},
	}
	for name, assets := range pauses {
		for claimantName, claimant := range map[string][]byte{"ClaimedBySeller": ctrlSeller, "ClaimedByBidder": ctrlBuyer} {
			t.Run(name+"_"+claimantName, func(t *testing.T) {
				t.Parallel()

				currency, collection := assets()
				env := newControlsEnv(t, true, currency, collection)
				env.storeExpiredBid(true, ctrlFixedDeposit)

				status, err := env.claim(claimant)
				env.requireUnwound(status, err, ctrlFixedDeposit)
			})
		}
	}
}

// An order listed before FixAuditChangesV4 pays the marketplace's live referral address.
func TestMarketAssetControls_LiveReferralAddress(t *testing.T) {
	t.Parallel()

	// Every payee but the new referral address may receive the restricted currency.
	currency := func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(nil)...) }
	auction := func() *kapps.MarketOrderData {
		return &kapps.MarketOrderData{MarketType: kapps.MarketOrderData_Auction, ReservePrice: ctrlBid / 2}
	}

	t.Run("RepointedBeforeTheBid_BidRefused", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, currency(), controlsCollection(false, false))
		env.storeOrderWith(auction(), 500, false)
		env.setLiveReferral(ctrlNewReferral)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		requireNotAllowed(t, status, err)
		env.requireTransferRefusedReceipt()
		env.requireUnsettled(ctrlFunds)
		require.Empty(t, env.storedOrder().CurrentBidder)
	})

	t.Run("RepointedAfterTheBid_ExpiredClaimUnwinds", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, currency(), controlsCollection(false, false))
		order := auction()
		order.RoyaltiesFixedDeposit = ctrlFixedDeposit
		env.storeOrderWith(order, 500, false)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, ctrlFunds-ctrlBid, env.balance(ctrlBuyer))

		env.setLiveReferral(ctrlNewReferral)
		env.blockTime = ctrlBlockTime + 2000

		status, err = env.claim(ctrlBuyer)
		env.requireUnwound(status, err, ctrlFixedDeposit)
		require.Zero(t, env.balance(ctrlNewReferral))
	})
}

// A role the issuer removes after the bid refuses the sale. The seller's early accept fails as
// any refused settlement does (the seller can still cancel), and the expired Claim unwinds.
func TestMarketAssetControls_RoyaltyLostDepositAfterBid(t *testing.T) {
	t.Parallel()

	bidThenRevoke := func(t *testing.T) *controlsEnv {
		env := newControlsEnv(t, true, controlsCurrency(false, true, depositRolesExcept(nil)...), controlsCollection(false, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		env.currency.Roles = depositRolesExcept(ctrlRoyalty)
		return env
	}

	t.Run("EarlyAcceptRefused", func(t *testing.T) {
		t.Parallel()

		env := bidThenRevoke(t)

		status, err := env.claim(ctrlSeller)
		requireNotAllowed(t, status, err)
		env.requireTransferRefusedReceipt()
		env.requireStillEscrowed()
	})

	t.Run("ExpiredClaimUnwinds", func(t *testing.T) {
		t.Parallel()

		env := bidThenRevoke(t)
		env.blockTime = ctrlBlockTime + 2000

		status, err := env.claim(ctrlSeller)
		env.requireUnwound(status, err, 0)
	})
}

// KLV is a KDA like any other: under FixAuditChangesV6 it is loaded for its controls and an
// unrestricted KLV sale settles.
func TestMarketAssetControls_KLVCurrencySettles(t *testing.T) {
	t.Parallel()

	klv := func() *kapps.KDAData {
		return &kapps.KDAData{
			ID: kdautils.KLVIdentifier, AssetType: kapps.KDAData_Fungible,
			Attributes: &kapps.AttributesData{}, Properties: &kapps.PropertiesData{},
		}
	}

	t.Run("ExpiredClaim", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, klv(), controlsCollection(false, false))
		env.storeExpiredBid(true, 0)

		status, err := env.claim(ctrlBuyer)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
		require.True(t, env.storedOrder().IsClaimed)
		require.NotZero(t, env.lookups[string(kdautils.KLVIdentifier)], "the KLV controls must be read")
	})

	t.Run("BuyItNow", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, klv(), controlsCollection(false, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
	})
}

// Pre-fork twins (FixAuditChangesV6 off). Before the fork neither asset's pause nor transfer
// restriction is read, so every case the controls refuse above is accepted and settled as it was,
// a legacy behaviour kept for replay, with no new receipt and no new KDA lookup.

func (e *controlsEnv) requireNoErrorReceipt() {
	e.t.Helper()

	require.Empty(e.t, e.receipts.GetByType(int8(kapp.ReceiptTypeError)))
}

// requireBidAccepted checks the bid of ctrlBid by ctrlBuyer was escrowed and no KDA was read.
func (e *controlsEnv) requireBidAccepted(status transaction.Transaction_TXResultCode, err error) {
	e.t.Helper()

	require.NoError(e.t, err)
	require.Equal(e.t, transaction.Transaction_Ok, status)
	require.Equal(e.t, ctrlFunds-ctrlBid, e.balance(ctrlBuyer))
	require.Equal(e.t, ctrlBuyer, e.storedOrder().CurrentBidder)
	require.True(e.t, e.holdsNFT(kapps.MarketKAppAddress))
	require.Empty(e.t, e.lookups, "a pre-fork bid must not load any KDA")
	e.requireNoErrorReceipt()
}

// requireSettledQuietly is requireSettled with no error receipt and no read of the currency.
func (e *controlsEnv) requireSettledQuietly(status transaction.Transaction_TXResultCode, err error, buyerBalance int64) {
	e.t.Helper()

	e.requireSettled(status, err, buyerBalance)
	e.requireNoErrorReceipt()
	require.Zero(e.t, e.lookups[string(e.currency.ID)], "pre-fork must not load the currency")
}

func TestMarketAssetControls_PreFork_PausedCollection(t *testing.T) {
	t.Parallel()

	t.Run("BuyItNowSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
	})

	t.Run("AuctionBidAccepted", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(true, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireBidAccepted(status, err)
	})

	t.Run("SellAcceptedWithoutReceipt", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(true, false))
		status, err := env.sell()
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
		env.requireNoErrorReceipt()
	})
}

func TestMarketAssetControls_PreFork_PausedCurrency(t *testing.T) {
	t.Parallel()

	t.Run("BuyItNowSettlesWithoutReceipt", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
	})

	t.Run("AuctionBidAcceptedWithoutReceipt", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(true, false), controlsCollection(false, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireBidAccepted(status, err)
	})
}

func TestMarketAssetControls_PreFork_LimitTransferCurrency(t *testing.T) {
	t.Parallel()

	currencies := map[string]func() *kapps.KDAData{
		"RolelessBuyer":            func() *kapps.KDAData { return controlsCurrency(false, true) },
		"ReferralLacksDeposit":     func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlReferral)...) },
		"SplitLacksDeposit":        func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlSplit)...) },
		"RoyaltyLacksDeposit":      func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlRoyalty)...) },
		"SellerLacksDeposit":       func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(ctrlSeller)...) },
		"BuyerHasRoleTransfer":     func() *kapps.KDAData { return controlsCurrency(false, true, transferRole(ctrlBuyer)) },
		"EveryPayeeHasRoleDeposit": func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(nil)...) },
	}
	for name, currency := range currencies {
		t.Run(name+"_AuctionBidAccepted", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, currency(), controlsCollection(false, false))
			env.storeAuction(0, 0, nil)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireBidAccepted(status, err)
		})

		t.Run(name+"_BuyItNowSettlesWithoutReceipt", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, currency(), controlsCollection(false, false))
			env.storeBuyItNow()

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
		})
	}

	t.Run("ZeroReferralLegSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, true, depositRolesExcept(ctrlReferral)...), controlsCollection(false, false))
		env.storeOrder(&kapps.MarketOrderData{MarketType: kapps.MarketOrderData_BuyItNow, Price: ctrlBid}, 0)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Zero(t, env.balance(ctrlReferral))
		require.Equal(t, ctrlSellerCut+ctrlLeg, env.balance(ctrlSeller))
		require.True(t, env.holdsNFT(ctrlBuyer))
	})

	t.Run("OutbidRefund", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, true, transferRole(ctrlBuyer)), controlsCollection(false, false))
		env.storeAuction(0, ctrlBid/2, ctrlPrevBidder)

		status, err := env.buy(ctrlBuyer, ctrlBid-1)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, ctrlFunds+ctrlBid/2, env.balance(ctrlPrevBidder))
		require.Equal(t, ctrlFunds-(ctrlBid-1), env.balance(ctrlBuyer))
		require.Equal(t, ctrlBuyer, env.storedOrder().CurrentBidder)
		require.Empty(t, env.lookups)
	})
}

func TestMarketAssetControls_PreFork_LimitTransferCollection(t *testing.T) {
	t.Parallel()

	t.Run("RolelessSellerToRolelessBuyer_BidAccepted", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireBidAccepted(status, err)
	})

	t.Run("RolelessSellerToRolelessBuyer_EarlyAcceptSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeAuction(0, ctrlBid, ctrlBuyer)

		status, err := env.claim(ctrlSeller)
		env.requireSettledQuietly(status, err, ctrlFunds)
	})

	t.Run("RolelessSellerToRolelessBuyer_BuyItNowSettles", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
	})

	for name, collection := range map[string]*kapps.KDAData{
		"SellerHasRoleTransfer": controlsCollection(false, true, transferRole(ctrlSeller)),
		"BuyerHasRoleDeposit":   controlsCollection(false, true, depositRole(ctrlBuyer)),
	} {
		t.Run(name+"_Settles", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, controlsCurrency(false, false), collection)
			env.storeBuyItNow()

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
		})
	}

	t.Run("SellAllowed", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		status, err := env.sell()
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
	})
}

// Before the fork a pause applied after listing changes nothing: every path settles or returns
// as it always did.
func TestMarketAssetControls_PreFork_PausedAfterListing(t *testing.T) {
	t.Parallel()

	pauseCurrency := func(env *controlsEnv) { env.currency.Attributes.IsPaused = true }
	pauseCollection := func(env *controlsEnv) { env.collection.Attributes.IsPaused = true }

	for name, pause := range map[string]func(*controlsEnv){"Currency": pauseCurrency, "Collection": pauseCollection} {
		bidThenPause := func(t *testing.T) *controlsEnv {
			env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(0, 0, nil)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.Equal(t, ctrlFunds-ctrlBid, env.balance(ctrlBuyer))

			pause(env)
			return env
		}

		t.Run(name+"_BuyItNowSettles", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeBuyItNow()
			pause(env)

			status, err := env.buy(ctrlBuyer, ctrlBid)
			env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
		})

		t.Run(name+"_AfterBid_LaterBidAccepted", func(t *testing.T) {
			t.Parallel()

			env := bidThenPause(t)

			status, err := env.buy(ctrlPrevBidder, ctrlBid+1)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.Equal(t, ctrlFunds, env.balance(ctrlBuyer), "the outbid bidder is refunded")
			require.Equal(t, ctrlFunds-(ctrlBid+1), env.balance(ctrlPrevBidder))
			require.Equal(t, ctrlPrevBidder, env.storedOrder().CurrentBidder)
		})

		t.Run(name+"_AfterBid_EarlyAcceptSettles", func(t *testing.T) {
			t.Parallel()

			env := bidThenPause(t)

			status, err := env.claim(ctrlSeller)
			env.requireSettled(status, err, ctrlFunds-ctrlBid)
			env.requireNoErrorReceipt()
			require.True(t, env.storedOrder().IsClaimed)
		})

		t.Run(name+"_AfterBid_ExpiredClaimSettles", func(t *testing.T) {
			t.Parallel()

			env := bidThenPause(t)
			env.blockTime = ctrlBlockTime + 2000

			status, err := env.claim(ctrlBuyer)
			env.requireSettled(status, err, ctrlFunds-ctrlBid)
			env.requireNoErrorReceipt()
			require.True(t, env.storedOrder().IsClaimed)
		})

		t.Run(name+"_AfterBid_CancelReturnsEverything", func(t *testing.T) {
			t.Parallel()

			env := bidThenPause(t)

			status, err := env.cancel()
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.True(t, env.holdsNFT(ctrlSeller))
			require.False(t, env.holdsNFT(kapps.MarketKAppAddress))
			require.Equal(t, ctrlFunds, env.balance(ctrlBuyer))
			env.requireNoErrorReceipt()
		})

		t.Run(name+"_ReserveNotMet_ClaimReturnsEverything", func(t *testing.T) {
			t.Parallel()

			env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, false))
			env.storeAuction(ctrlBlockTime-1, ctrlBid/4, ctrlBuyer)
			pause(env)

			status, err := env.claim(ctrlSeller)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.True(t, env.holdsNFT(ctrlSeller))
			require.Equal(t, ctrlFunds+ctrlBid/4, env.balance(ctrlBuyer))
			env.requireNoErrorReceipt()
		})
	}
}

func TestMarketAssetControls_PreFork_SplitOverflow(t *testing.T) {
	t.Parallel()

	collection := controlsCollection(false, false)
	collection.Royalties.SplitRoyalties = map[string]*kapps.RoyaltySplitData{
		hex.EncodeToString(ctrlSplit):   {PercentMarketPercentage: 6000},
		hex.EncodeToString(ctrlRoyalty): {PercentMarketPercentage: 6000},
	}

	// FixMarketBuyOverflow, a different fork, already refuses an overflowing split at settlement
	// with the same code the V6 check returns; only the bid is accepted before V6.
	env := newControlsEnv(t, false, controlsCurrency(false, false), collection)
	env.storeBuyItNow()

	status, err := env.buy(ctrlBuyer, ctrlBid)
	require.Error(t, err)
	require.Equal(t, transaction.Transaction_ParameterInvalid, status)
	// pre-fork: legacy behaviour, kept for replay. The refusal comes after the buyer was debited,
	// a partial state the failed transaction discards; V6 refuses before any credit.
	require.Equal(t, ctrlFunds-ctrlBid, env.balance(ctrlBuyer))
	require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
}

func TestMarketAssetControls_PreFork_ExpiredAuction(t *testing.T) {
	t.Parallel()

	claimants := map[string][]byte{"ClaimedBySeller": ctrlSeller, "ClaimedByBidder": ctrlBuyer}

	for name, assets := range ctrlRefusedSales {
		for claimantName, claimant := range claimants {
			t.Run(name+"_"+claimantName+"_Settles", func(t *testing.T) {
				t.Parallel()

				currency, collection := assets()
				env := newControlsEnv(t, false, currency, collection)
				env.storeExpiredBid(true, ctrlFixedDeposit)

				status, err := env.claim(claimant)
				env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
				require.Equal(t, ctrlFixedDeposit, env.klvBalance(ctrlRoyalty), "the fixed deposit is paid as a sale")
				require.Zero(t, env.klvBalance(ctrlSeller))
				require.True(t, env.storedOrder().IsClaimed)
			})
		}
	}

	for name, assets := range map[string]func() (*kapps.KDAData, *kapps.KDAData){
		"CurrencyPaused": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(true, true), controlsCollection(false, true)
		},
		"CollectionPaused": func() (*kapps.KDAData, *kapps.KDAData) {
			return controlsCurrency(false, true), controlsCollection(true, true)
		},
	} {
		for claimantName, claimant := range claimants {
			t.Run(name+"_"+claimantName+"_Settles", func(t *testing.T) {
				t.Parallel()

				currency, collection := assets()
				env := newControlsEnv(t, false, currency, collection)
				env.storeExpiredBid(true, ctrlFixedDeposit)

				status, err := env.claim(claimant)
				env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
				require.Equal(t, ctrlFixedDeposit, env.klvBalance(ctrlRoyalty), "the fixed deposit is paid as a sale")
				require.True(t, env.storedOrder().IsClaimed)
			})
		}
	}

	t.Run("ReserveNotMetStillUnwinds", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, controlsCurrency(false, false), controlsCollection(false, true))
		env.storeExpiredBid(true, ctrlFixedDeposit)
		order := env.storedOrder()
		order.ReservePrice = ctrlBid + 1
		marketKappAcc, err := env.accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)
		require.NoError(t, env.market.SetMarketOrder(marketKappAcc, order))
		require.NoError(t, env.accCacher.UpdateKapp(marketKappAcc))

		status, err := env.claim(ctrlSeller)
		env.requireUnwound(status, err, ctrlFixedDeposit)
	})
}

// Before the fork a repointed live referral address is simply paid, restricted currency or not.
func TestMarketAssetControls_PreFork_LiveReferralAddress(t *testing.T) {
	t.Parallel()

	currency := func() *kapps.KDAData { return controlsCurrency(false, true, depositRolesExcept(nil)...) }
	auction := func() *kapps.MarketOrderData {
		return &kapps.MarketOrderData{MarketType: kapps.MarketOrderData_Auction, ReservePrice: ctrlBid / 2}
	}

	t.Run("RepointedBeforeTheBid_BidAccepted", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, currency(), controlsCollection(false, false))
		env.storeOrderWith(auction(), 500, false)
		env.setLiveReferral(ctrlNewReferral)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireBidAccepted(status, err)
	})

	t.Run("RepointedAfterTheBid_ExpiredClaimPaysTheNewReferral", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, currency(), controlsCollection(false, false))
		order := auction()
		order.RoyaltiesFixedDeposit = ctrlFixedDeposit
		env.storeOrderWith(order, 500, false)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		env.setLiveReferral(ctrlNewReferral)
		env.blockTime = ctrlBlockTime + 2000

		status, err = env.claim(ctrlBuyer)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(ctrlBuyer))
		require.Equal(t, ctrlLeg, env.balance(ctrlNewReferral))
		require.Zero(t, env.balance(ctrlReferral))
		require.Equal(t, ctrlLeg, env.balance(ctrlSplit))
		require.Equal(t, ctrlLeg, env.balance(ctrlRoyalty))
		require.Equal(t, ctrlSellerCut, env.balance(ctrlSeller))
		require.Equal(t, ctrlFixedDeposit, env.klvBalance(ctrlRoyalty))
		require.True(t, env.storedOrder().IsClaimed)
		env.requireNoErrorReceipt()
	})
}

func TestMarketAssetControls_PreFork_RoyaltyLostDepositAfterBid(t *testing.T) {
	t.Parallel()

	bidThenRevoke := func(t *testing.T) *controlsEnv {
		env := newControlsEnv(t, false, controlsCurrency(false, true, depositRolesExcept(nil)...), controlsCollection(false, false))
		env.storeAuction(0, 0, nil)

		status, err := env.buy(ctrlBuyer, ctrlBid)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)

		env.currency.Roles = depositRolesExcept(ctrlRoyalty)
		return env
	}

	t.Run("EarlyAcceptSettles", func(t *testing.T) {
		t.Parallel()

		env := bidThenRevoke(t)

		status, err := env.claim(ctrlSeller)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
		env.requireNoErrorReceipt()
	})

	t.Run("ExpiredClaimSettles", func(t *testing.T) {
		t.Parallel()

		env := bidThenRevoke(t)
		env.blockTime = ctrlBlockTime + 2000

		status, err := env.claim(ctrlSeller)
		env.requireSettled(status, err, ctrlFunds-ctrlBid)
		env.requireNoErrorReceipt()
		require.True(t, env.storedOrder().IsClaimed)
	})
}

// Before the fork KLV is not loaded for its controls.
func TestMarketAssetControls_PreFork_KLVCurrencySettles(t *testing.T) {
	t.Parallel()

	klv := func() *kapps.KDAData {
		return &kapps.KDAData{
			ID: kdautils.KLVIdentifier, AssetType: kapps.KDAData_Fungible,
			Attributes: &kapps.AttributesData{}, Properties: &kapps.PropertiesData{},
		}
	}

	t.Run("ExpiredClaim", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, klv(), controlsCollection(false, false))
		env.storeExpiredBid(true, 0)

		status, err := env.claim(ctrlBuyer)
		env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
		require.True(t, env.storedOrder().IsClaimed)
		require.Zero(t, env.lookups[string(kdautils.KLVIdentifier)], "pre-fork must not read the KLV controls")
	})

	t.Run("BuyItNow", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, false, klv(), controlsCollection(false, false))
		env.storeBuyItNow()

		status, err := env.buy(ctrlBuyer, ctrlBid)
		env.requireSettledQuietly(status, err, ctrlFunds-ctrlBid)
		require.Zero(t, env.lookups[string(kdautils.KLVIdentifier)], "pre-fork must not read the KLV controls")
	})
}
