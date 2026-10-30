package market

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// ctrlPaymentNFT is NFT 8 of the controls collection, and ctrlCompoundCurrency names it as a
// currency. kdautils.ToKDAKey builds the same trie key for both, so a bid priced in the
// compound ID would debit the NFT through the fungible balance path (KLC-2816).
var (
	ctrlPaymentNFT       = []byte("8")
	ctrlCompoundCurrency = []byte(string(ctrlCollectionID) + kapps.Sp + string(ctrlPaymentNFT))
)

func (e *controlsEnv) setFixAuditChangesV7(active bool) {
	e.market.forkController.(*mock.ForkControllerStub).FixAuditChangesV7Value = active
}

// storeBuyItNowPricedIn stores a buy-it-now order for the escrowed NFT priced at price in
// currencyID, bypassing Sell.
func (e *controlsEnv) storeBuyItNowPricedIn(currencyID []byte, price int64) {
	e.t.Helper()

	e.storeOrderPricedIn(&kapps.MarketOrderData{MarketType: kapps.MarketOrderData_BuyItNow, Price: price}, currencyID)
}

// storeOrderPricedIn stores order for the escrowed NFT priced in currencyID, bypassing Sell.
func (e *controlsEnv) storeOrderPricedIn(order *kapps.MarketOrderData, currencyID []byte) {
	e.t.Helper()

	e.storeOrder(order, 0)

	stored := e.storedOrder()
	stored.CurrencyID = currencyID
	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	require.NoError(e.t, e.market.SetMarketOrder(marketKappAcc, stored))
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

// newV7ControlsEnv is newControlsEnv with FixAuditChangesV6 and FixAuditChangesV7 active.
func newV7ControlsEnv(t *testing.T) *controlsEnv {
	t.Helper()

	env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
	env.setFixAuditChangesV7(true)
	return env
}

// giveNFT stores a held record of NFT nonce of the controls collection on addr.
func (e *controlsEnv) giveNFT(addr []byte, nonce []byte) {
	e.t.Helper()

	e.giveNFTRecord(addr, nonce, mock.NFTHolderRecord(1))
}

func (e *controlsEnv) giveNFTRecord(addr []byte, nonce []byte, data []byte) {
	e.t.Helper()

	acc, err := e.accCacher.LoadUser(addr)
	require.NoError(e.t, err)
	require.NoError(e.t, acc.AddInternalKDA(ctrlCollectionID, nonce, data))
	require.NoError(e.t, e.accCacher.UpdateUser(acc))
}

func (e *controlsEnv) compoundBalance(addr []byte) int64 {
	e.t.Helper()

	acc, err := e.accCacher.LoadUser(addr)
	require.NoError(e.t, err)
	return acc.GetBalance(ctrlCompoundCurrency, true)
}

func (e *controlsEnv) buyInCompoundCurrency() (transaction.Transaction_TXResultCode, error) {
	return e.market.Buy(ctrlBuyer, &transaction.BuyContract{
		BuyType: transaction.BuyContract_MarketBuy, ID: ctrlOrderID, CurrencyID: ctrlCompoundCurrency, Amount: 1,
	})
}

// Under FixAuditChangesV6 a bid loads its currency for the asset controls before anything
// moves, and a compound NFT ID is not an asset, so the bid is refused and the payment NFT stays
// with the buyer, even before FixAuditChangesV7 rejects the currency outright. Before V6 the
// bid debits the payment NFT through the fungible path and the sale settles.
func TestMarketCurrency_CompoundIDBidBeforeV7(t *testing.T) {
	t.Parallel()

	newEnv := func(t *testing.T, fixAuditChangesV6 bool) *controlsEnv {
		env := newControlsEnv(t, fixAuditChangesV6, controlsCurrency(false, false), controlsCollection(false, false))
		env.storeBuyItNowPricedIn(ctrlCompoundCurrency, 1)
		env.giveNFT(ctrlBuyer, ctrlPaymentNFT)
		require.Equal(t, int64(1), env.compoundBalance(ctrlBuyer), "the compound ID reads the NFT's record")
		return env
	}

	t.Run("V6 refuses the bid", func(t *testing.T) {
		env := newEnv(t, true)

		status, err := env.buyInCompoundCurrency()

		require.Error(t, err)
		require.Equal(t, transaction.Transaction_KAPPError, status)
		require.Equal(t, int64(1), env.compoundBalance(ctrlBuyer), "the payment NFT stays with the buyer")
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress), "the listed NFT stays escrowed")
	})

	t.Run("before V6 the payment NFT is spent as currency", func(t *testing.T) {
		env := newEnv(t, false)

		status, err := env.buyInCompoundCurrency()

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Zero(t, env.compoundBalance(ctrlBuyer))
		require.Equal(t, int64(1), env.compoundBalance(ctrlSeller), "the seller is paid NFT 8 with no mint")
		require.True(t, env.holdsNFT(ctrlBuyer))
	})
}

// An order listed before FixAuditChangesV7 in a currency checkCurrency now refuses can still be
// closed: CancelOrder and a Claim below the reserve return the NFT to the seller and the bid to
// the bidder without checking or loading the currency.
func TestMarketCurrency_LegacyOrderUnwindsFromV7(t *testing.T) {
	t.Parallel()

	// The bid of NFT 8, priced in its compound ID, was taken before V6 and left the bidder's
	// record spent.
	newEnv := func(t *testing.T, endTime int64) *controlsEnv {
		env := newV7ControlsEnv(t)
		env.giveNFTRecord(ctrlBuyer, ctrlPaymentNFT, mock.NFTHolderRecord(0))
		env.storeOrderPricedIn(&kapps.MarketOrderData{
			MarketType: kapps.MarketOrderData_Auction, ReservePrice: 2, EndTime: endTime,
			CurrentBid: 1, CurrentBidder: ctrlBuyer,
		}, ctrlCompoundCurrency)
		return env
	}

	requireReturned := func(t *testing.T, env *controlsEnv, status transaction.Transaction_TXResultCode, err error) {
		t.Helper()

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(ctrlSeller), "the NFT returns to the seller")
		require.False(t, env.holdsNFT(kapps.MarketKAppAddress))
		require.Equal(t, int64(1), env.compoundBalance(ctrlBuyer), "the refund restores the bidder's payment NFT")
	}

	t.Run("CancelOrder", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, ctrlBlockTime+1000)

		status, err := env.cancel()

		requireReturned(t, env, status, err)
	})

	t.Run("expired Claim below the reserve", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, ctrlBlockTime-1)

		status, err := env.claim(ctrlSeller)

		requireReturned(t, env, status, err)
	})
}

// requireCurrencyRefused checks a refused currency failed with its single error receipt.
func (e *controlsEnv) requireCurrencyRefused(status transaction.Transaction_TXResultCode, err error) {
	e.t.Helper()

	require.ErrorIs(e.t, err, common.ErrInvalidValue)
	require.Equal(e.t, transaction.Transaction_ParameterInvalid, status)
	errorReceipts := e.receipts.GetByType(int8(kapp.ReceiptTypeError))
	require.Len(e.t, errorReceipts, 1)
	require.Equal(e.t, []byte(common.ErrFieldOrderCurrencyInvalid), errorReceipts[0].Data[1])
}

// From FixAuditChangesV7 an order's currency must be KLV or an existing fungible asset, at Sell
// and at Buy, the latter for orders listed before the fork.
func TestMarketCurrency_MustBeFungibleFromV7(t *testing.T) {
	t.Parallel()

	const nonAsset = "NOPE-0000"

	refused := map[string][]byte{
		"a compound NFT ID":     ctrlCompoundCurrency,
		"an NFT collection":     ctrlCollectionID,
		"an asset that is none": []byte(nonAsset),
	}
	for name, currencyID := range refused {
		currencyID := currencyID

		t.Run("Sell refuses "+name, func(t *testing.T) {
			t.Parallel()

			env := newV7ControlsEnv(t)

			status, err := env.sellIn(currencyID)

			env.requireCurrencyRefused(status, err)
			require.True(t, env.holdsNFT(ctrlSeller), "the NFT stays with the seller")
		})
	}

	// buyLegacyOrderIn bids on an order listed before the fork in currencyID and checks nothing
	// moved.
	buyLegacyOrderIn := func(t *testing.T, fixAuditChangesV6 bool, currencyID []byte) (*controlsEnv, transaction.Transaction_TXResultCode, error) {
		t.Helper()

		env := newControlsEnv(t, fixAuditChangesV6, controlsCurrency(false, false), controlsCollection(false, false))
		env.setFixAuditChangesV7(true)
		env.storeBuyItNowPricedIn(currencyID, 1)
		env.giveNFT(ctrlBuyer, ctrlPaymentNFT)

		status, err := env.market.Buy(ctrlBuyer, &transaction.BuyContract{
			BuyType: transaction.BuyContract_MarketBuy, ID: ctrlOrderID, CurrencyID: currencyID, Amount: 1,
		})

		require.Equal(t, int64(1), env.compoundBalance(ctrlBuyer), "the payment NFT stays with the buyer")
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress), "the listed NFT stays escrowed")
		return env, status, err
	}

	// The Buy check does not rely on the FixAuditChangesV6 bid controls, which also load the
	// currency.
	for name, currencyID := range refused {
		for v6, fixAuditChangesV6 := range map[string]bool{"with V6": true, "without V6": false} {
			currencyID, fixAuditChangesV6 := currencyID, fixAuditChangesV6

			t.Run("Buy refuses "+name+" "+v6, func(t *testing.T) {
				t.Parallel()

				env, status, err := buyLegacyOrderIn(t, fixAuditChangesV6, currencyID)

				env.requireCurrencyRefused(status, err)
			})
		}
	}

	t.Run("Sell returns a currency read error as is", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)

		status, err := env.sellIn(ctrlUnreadableID)

		require.ErrorIs(t, err, errCtrlUnreadable)
		require.Equal(t, transaction.Transaction_KAPPError, status)
		require.True(t, env.holdsNFT(ctrlSeller), "the NFT stays with the seller")
	})

	t.Run("Sell accepts a fungible currency", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)

		status, err := env.sell()

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})

	t.Run("Sell accepts KLV without loading it", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)

		status, err := env.sellIn(kdautils.KLVIdentifier)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Zero(t, env.lookups[string(kdautils.KLVIdentifier)])
	})

	t.Run("before V7 Sell lists a compound NFT ID", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))

		status, err := env.sellIn(ctrlCompoundCurrency)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
	})
}

// listAs lists NFT ctrlAssetID of the controls collection as sender, priced in the env currency,
// from whatever record sender already holds.
func (e *controlsEnv) listAs(sender []byte) (transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	return e.market.Sell(sender, &transaction.SellContract{
		MarketType: transaction.SellContract_EnumMarketType(kapps.MarketOrderData_Auction), MarketplaceID: ctrlMarketplaceID,
		AssetID:    []byte(string(ctrlCollectionID) + kapps.Sp + string(ctrlAssetID)),
		CurrencyID: e.currency.ID, ReservePrice: ctrlBid / 2, EndTime: ctrlBlockTime + 1000,
	})
}

func (e *controlsEnv) escrowedRecord() []byte {
	e.t.Helper()

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	value, err := marketKappAcc.DataTrieTracker().RetrieveValue(kdautils.ToKDAKey(ctrlCollectionID, ctrlAssetID))
	require.NoError(e.t, err)
	return value
}

// A spent NFT record (balance 0, kept in the trie by its metadata) holds no unit. From
// FixAuditChangesV7 it cannot be listed; before it, Sell escrows it like a held one (KLC-2835).
func TestMarketSell_SpentNFTRecord(t *testing.T) {
	t.Parallel()

	newEnv := func(t *testing.T, fixAuditChangesV7 bool) *controlsEnv {
		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
		env.setFixAuditChangesV7(fixAuditChangesV7)
		env.giveNFTRecord(ctrlSeller, ctrlAssetID, mock.NFTHolderRecord(0))
		return env
	}

	t.Run("refused from V7", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, true)

		status, err := env.listAs(ctrlSeller)

		require.ErrorIs(t, err, common.ErrAssetNotFound)
		require.Equal(t, transaction.Transaction_BalanceError, status)
		errorReceipts := env.receipts.GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldInsufficientFunds), errorReceipts[0].Data[1])
		require.False(t, env.holdsNFT(kapps.MarketKAppAddress))
		require.True(t, env.holdsNFT(ctrlSeller), "the refused record stays with the seller")
	})

	t.Run("listed before V7", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, false)

		status, err := env.listAs(ctrlSeller)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
	})
}

// Two accounts can hold a record of the same NFT when one was credited through the fungible
// balance path. From FixAuditChangesV7 the second listing is refused and the first order's escrow
// and the second holder's record are untouched; before it, the second Sell overwrites the escrow
// (KLC-2835).
func TestMarketSell_EscrowedNFTListedAgain(t *testing.T) {
	t.Parallel()

	accomplice := makeAddress("accomplice")
	duplicate, err := marshal.NewProtoMarshalizer().Marshal(&kapps.UserKDA{Balance: 1})
	require.NoError(t, err)

	newEnv := func(t *testing.T, fixAuditChangesV7 bool) *controlsEnv {
		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
		env.setFixAuditChangesV7(fixAuditChangesV7)
		env.giveNFT(ctrlSeller, ctrlAssetID)
		env.giveNFTRecord(accomplice, ctrlAssetID, duplicate)

		status, err := env.listAs(ctrlSeller)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, mock.NFTHolderRecord(1), env.escrowedRecord())
		return env
	}

	t.Run("refused from V7", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, true)

		status, err := env.listAs(accomplice)

		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Equal(t, transaction.Transaction_KeyConflict, status)
		errorReceipts := env.receipts.GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldInvalidOrder), errorReceipts[0].Data[1])
		require.Equal(t, mock.NFTHolderRecord(1), env.escrowedRecord(), "the first order's escrow is untouched")
		require.True(t, env.holdsNFT(accomplice), "the second holder keeps its record")
	})

	t.Run("overwrites the escrow before V7", func(t *testing.T) {
		t.Parallel()

		env := newEnv(t, false)

		status, err := env.listAs(accomplice)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Equal(t, duplicate, env.escrowedRecord())
	})
}

// An NFT sold earlier in the block has its escrow key cleared, which reads back as an empty dirty
// value: it is not escrowed, and its buyer can list it again.
func TestMarketSell_RelistAfterSaleInTheSameBlock(t *testing.T) {
	t.Parallel()

	env := newV7ControlsEnv(t)
	env.storeBuyItNow()
	status, err := env.buy(ctrlBuyer, ctrlBid)
	env.requireSettled(status, err, ctrlFunds-ctrlBid)

	status, err = env.listAs(ctrlBuyer)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, status)
	require.True(t, env.holdsNFT(kapps.MarketKAppAddress))
}

// replaceEscrowedRecord replaces the stored order's escrowed record with data, as an order listed
// before FixAuditChangesV7 may hold (KLC-2835).
func (e *controlsEnv) replaceEscrowedRecord(data []byte) {
	e.t.Helper()

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	require.NoError(e.t, marketKappAcc.AddInternalKDA(ctrlCollectionID, ctrlAssetID, data))
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

func (e *controlsEnv) spendEscrowedRecord() {
	e.t.Helper()

	e.replaceEscrowedRecord(mock.NFTHolderRecord(0))
}

// clearEscrowedRecord clears the stored order's escrow key in this block, as the sale of another
// order on the same key does (KLC-2835).
func (e *controlsEnv) clearEscrowedRecord() {
	e.t.Helper()

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	_, err = marketKappAcc.SubInternalKDA(ctrlCollectionID, ctrlAssetID)
	require.NoError(e.t, err)
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

// An order listed before FixAuditChangesV7 may escrow a spent or undecodable record (KLC-2835).
// From the fork it is not sold: a bid and the seller's early accept are refused, and an expired
// Claim returns the record to the seller and the bid to the bidder. Before the fork the buyer
// pays for it.
func TestMarketSettle_SpentEscrowedRecord(t *testing.T) {
	t.Parallel()

	t.Run("a bid is refused from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeBuyItNow()
		env.spendEscrowedRecord()

		status, err := env.buy(ctrlBuyer, ctrlBid)

		require.ErrorIs(t, err, common.ErrAssetNotFound)
		require.Equal(t, transaction.Transaction_BalanceError, status)
		errorReceipts := env.receipts.GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldAssetNotFound), errorReceipts[0].Data[1])
		env.requireUnsettled(ctrlFunds)
		require.Equal(t, mock.NFTHolderRecord(0), env.escrowedRecord())
	})

	t.Run("the seller cannot accept a bid from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeAuction(ctrlBlockTime+1000, ctrlBid, ctrlBuyer)
		env.spendEscrowedRecord()

		status, err := env.claim(ctrlSeller)

		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		errorReceipts := env.receipts.GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldClaimNotAvailable), errorReceipts[0].Data[1])
		env.requireUnsettled(ctrlFunds)
		require.Equal(t, mock.NFTHolderRecord(0), env.escrowedRecord())
	})

	t.Run("an expired Claim unwinds from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeExpiredBid(true, 0)
		env.spendEscrowedRecord()

		status, err := env.claim(ctrlBuyer)

		env.requireUnwound(status, err, 0)
	})

	t.Run("before V7 the buyer pays for it", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
		env.storeBuyItNow()
		env.spendEscrowedRecord()

		status, err := env.buy(ctrlBuyer, ctrlBid)

		env.requireSettled(status, err, ctrlFunds-ctrlBid)
	})

	undecodable := []byte{0xff}

	t.Run("a bid on an undecodable record is refused from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeBuyItNow()
		env.replaceEscrowedRecord(undecodable)

		status, err := env.buy(ctrlBuyer, ctrlBid)

		require.ErrorIs(t, err, common.ErrAssetNotFound)
		require.Equal(t, transaction.Transaction_BalanceError, status)
		env.requireUnsettled(ctrlFunds)
	})

	t.Run("an expired Claim unwinds an undecodable record from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeExpiredBid(true, 0)
		env.replaceEscrowedRecord(undecodable)

		status, err := env.claim(ctrlBuyer)

		env.requireUnwound(status, err, 0)
	})
}

// Two orders listed before FixAuditChangesV7 can share one escrow key, and the sale or unwind of one
// clears it (KLC-2835). From the fork the other order takes no bid and is not sold, and its expired
// Claim or its CancelOrder refunds the bid and the deposit and closes it with no NFT to return,
// even in the block that cleared the key. Before the fork its CancelOrder fails.
func TestMarketUnwind_EscrowWithoutRecord(t *testing.T) {
	t.Parallel()

	const deposit = int64(7)

	requireClosed := func(t *testing.T, env *controlsEnv, status transaction.Transaction_TXResultCode, err error, buyerBalance int64) {
		t.Helper()

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.False(t, env.holdsNFT(ctrlSeller))
		require.False(t, env.holdsNFT(ctrlBuyer))
		require.False(t, env.holdsNFT(kapps.MarketKAppAddress))
		require.Equal(t, buyerBalance, env.balance(ctrlBuyer), "the bid must return to the bidder")
		for _, payee := range ctrlPayees {
			require.Zero(t, env.balance(payee))
		}
		require.Equal(t, deposit, env.klvBalance(ctrlSeller), "the fixed deposit must return to the seller")
		require.True(t, env.storedOrder().IsClaimed)
		require.Empty(t, env.receipts.GetByType(int8(kapp.ReceiptTypeError)))
	}

	t.Run("an expired Claim refunds and closes from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeExpiredBid(true, deposit)
		env.clearEscrowedRecord()

		status, err := env.claim(ctrlBuyer)

		requireClosed(t, env, status, err, ctrlFunds)
	})

	t.Run("CancelOrder refunds and closes from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeOrder(&kapps.MarketOrderData{
			MarketType: kapps.MarketOrderData_Auction, ReservePrice: ctrlBid / 2, EndTime: ctrlBlockTime + 1000,
			CurrentBid: ctrlBid, CurrentBidder: ctrlBuyer, RoyaltiesFixedDeposit: deposit,
		}, 500)
		env.clearEscrowedRecord()

		status, err := env.cancel()

		// storeOrder does not debit the bid, so its refund adds to the buyer's funds.
		requireClosed(t, env, status, err, ctrlFunds+ctrlBid)
	})

	t.Run("a bid is refused from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeBuyItNow()
		env.clearEscrowedRecord()

		status, err := env.buy(ctrlBuyer, ctrlBid)

		require.ErrorIs(t, err, common.ErrAssetNotFound)
		require.Equal(t, transaction.Transaction_BalanceError, status)
		require.Equal(t, ctrlFunds, env.balance(ctrlBuyer))
		require.False(t, env.holdsNFT(ctrlBuyer))
	})

	t.Run("the seller cannot accept a bid from V7", func(t *testing.T) {
		t.Parallel()

		env := newV7ControlsEnv(t)
		env.storeAuction(ctrlBlockTime+1000, ctrlBid, ctrlBuyer)
		env.clearEscrowedRecord()

		status, err := env.claim(ctrlSeller)

		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		errorReceipts := env.receipts.GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldClaimNotAvailable), errorReceipts[0].Data[1])
	})

	t.Run("before V7 CancelOrder fails", func(t *testing.T) {
		t.Parallel()

		env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
		env.storeAuction(ctrlBlockTime+1000, ctrlBid, ctrlBuyer)
		env.clearEscrowedRecord()

		status, err := env.cancel()

		require.Error(t, err)
		require.Equal(t, transaction.Transaction_BalanceError, status)
	})
}
