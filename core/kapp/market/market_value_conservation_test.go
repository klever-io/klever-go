package market

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

// Market value conservation
//
// The market holds no balance of its own: Sell takes the fixed deposit in KLV from the seller
// and a bid takes its amount from the bidder, and both are owed to someone through the order
// until it closes. Settlement may only pay out what was taken, so across every account the
// totals never change and, while the order is open, what left the accounts is exactly what the
// order records as escrowed. The royalty split payouts are where that went wrong before
// (FixMarketBuyOverflow), so the cases below are generated collections: up to
// core.MaxTransferRoyalties splits with uneven shares and odd amounts so every percentage
// rounds, in a KDA currency and in KLV, on every settlement path, with and without the owner
// changing the royalties after the listing, before and after FixAuditChangesV6.

var (
	consRedirect      = makeAddress("cons-redirect")
	consRedirectSplit = makeAddress("cons-redirect-split")
)

// consChanges are the owner's royalty changes while the order is listed: an UpdateRoyalties that
// pays half of each royalty to a new split and the rest to a new receiver, and a
// ChangeRoyaltiesReceiver.
var consChanges = map[string]func(*controlsEnv){
	"UpdateRoyalties": func(e *controlsEnv) {
		e.collection.Royalties = &kapps.RoyaltiesData{
			Address:          consRedirect,
			MarketPercentage: e.collection.Royalties.GetMarketPercentage(),
			MarketFixed:      e.collection.Royalties.GetMarketFixed(),
			SplitRoyalties: map[string]*kapps.RoyaltySplitData{
				hex.EncodeToString(consRedirectSplit): {PercentMarketPercentage: 5000, PercentMarketFixed: 5000},
			},
		}
	},
	"ChangeRoyaltiesReceiver": func(e *controlsEnv) {
		e.collection.Royalties.Address = consRedirect
	},
}

type conservationCase struct {
	name       string
	fixOn      bool
	klv        bool
	collection *kapps.KDAData
	payees     [][]byte
	path       string
	change     string
	bid        int64
	outbid     bool
}

func generateConservationCases(seed int64, count int) []conservationCase {
	rnd := rand.New(rand.NewSource(seed))
	paths := []string{"Buy", "EarlyAccept", "ExpiredClaim"}
	changes := []string{"", "UpdateRoyalties", "ChangeRoyaltiesReceiver"}

	cases := make([]conservationCase, 0, count)
	for i := range count {
		collection := controlsCollection(false, false)
		collection.Royalties.MarketPercentage = uint32(rnd.Intn(9501)) // plus the 5% referral, at most 100%
		collection.Royalties.MarketFixed = int64(rnd.Intn(10_008))
		collection.Royalties.SplitRoyalties = map[string]*kapps.RoyaltySplitData{}

		splits := rnd.Intn(21)
		payees := make([][]byte, 0, splits)
		for j := range splits {
			payee := makeAddress(fmt.Sprintf("cons-%d-%d", i, j))
			payees = append(payees, payee)
			// Each share is at most an even part of 100%, so the shares sum to at most 100% and
			// rarely to exactly that.
			collection.Royalties.SplitRoyalties[hex.EncodeToString(payee)] = &kapps.RoyaltySplitData{
				PercentMarketPercentage: uint32(rnd.Intn(10_000/splits + 1)),
				PercentMarketFixed:      uint32(rnd.Intn(10_000/splits + 1)),
			}
		}

		c := conservationCase{
			fixOn:      rnd.Intn(4) != 0,
			klv:        rnd.Intn(2) == 0,
			collection: collection,
			payees:     payees,
			path:       paths[rnd.Intn(len(paths))],
			change:     changes[rnd.Intn(len(changes))],
			bid:        int64(1 + rnd.Intn(int(ctrlFunds/2))),
			outbid:     rnd.Intn(2) == 0,
		}
		c.name = fmt.Sprintf("%03d_%s_fix%t_klv%t_splits%d_%s", i, c.path, c.fixOn, c.klv, splits, c.change)
		cases = append(cases, c)
	}

	return cases
}

func klvCurrency() *kapps.KDAData {
	return &kapps.KDAData{
		ID: kdautils.KLVIdentifier, AssetType: kapps.KDAData_Fungible,
		Attributes: &kapps.AttributesData{}, Properties: &kapps.PropertiesData{},
	}
}

// listAt sells the controls NFT at price with reserve, funding the seller's fixed deposit, and
// returns the new order's ID.
func (e *controlsEnv) listAt(marketType kapps.MarketOrderData_EnumMarketType, price int64, reserve int64) ([]byte, transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	sellerAcc, err := e.accCacher.LoadUser(ctrlSeller)
	require.NoError(e.t, err)
	require.NoError(e.t, sellerAcc.AddInternalKDA(ctrlCollectionID, ctrlAssetID, mock.NFTHolderRecord(1)))
	require.NoError(e.t, sellerAcc.AddToBalance(e.collection.Royalties.GetMarketFixed(), kdautils.KLVIdentifier, true))
	require.NoError(e.t, e.accCacher.UpdateUser(sellerAcc))

	status, err := e.market.Sell(ctrlSeller, &transaction.SellContract{
		MarketType: transaction.SellContract_EnumMarketType(marketType), MarketplaceID: ctrlMarketplaceID,
		AssetID:    append(append(append([]byte{}, ctrlCollectionID...), []byte(kapps.Sp)...), ctrlAssetID...),
		CurrencyID: e.currency.ID, Price: price, ReservePrice: reserve, EndTime: ctrlBlockTime + 1000,
	})
	if err != nil {
		return nil, status, err
	}

	sells := e.receipts.GetByType(int8(txProcess.Sell))
	require.Len(e.t, sells, 1)
	return sells[0].Data[1], status, nil
}

func (e *controlsEnv) order(id []byte) *kapps.MarketOrderData {
	e.t.Helper()

	_, order, err := e.market.GetMarketOrder(id)
	require.NoError(e.t, err)
	return order
}

func (e *controlsEnv) bid(id []byte, bidder []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	return e.market.Buy(bidder, &transaction.BuyContract{
		BuyType: transaction.BuyContract_MarketBuy, ID: id, CurrencyID: e.currency.ID, Amount: amount,
	})
}

func (e *controlsEnv) claimOrder(id []byte) (transaction.Transaction_TXResultCode, error) {
	return e.market.Claim(ctrlSeller, &transaction.ClaimContract{ClaimType: transaction.ClaimContract_MarketClaim, ID: id})
}

// total sums asset over every account in accounts.
func (e *controlsEnv) total(accounts [][]byte, asset []byte) int64 {
	e.t.Helper()

	sum := int64(0)
	for _, addr := range accounts {
		acc, err := e.accCacher.LoadUser(addr)
		require.NoError(e.t, err)
		sum += acc.GetBalance(asset, true)
	}
	return sum
}

// marketNetTransfers is, per asset, what the Transfer receipts moved into the market KApp less
// what they moved out of it.
func (e *controlsEnv) marketNetTransfers() map[string]int64 {
	e.t.Helper()

	net := make(map[string]int64)
	for _, r := range e.receipts.Get() {
		// NewReceipt data: the type and contract ID, then the sender, the receiver, the amount
		// and the asset.
		if len(r.Data) < 5 || txProcess.ReceiptType(r.Data[0][0]) != txProcess.Transfer {
			continue
		}
		amount, err := strconv.ParseInt(string(r.Data[3]), 10, 64)
		require.NoError(e.t, err)
		if bytes.Equal(r.Data[2], kapps.MarketKAppAddress) {
			net[string(r.Data[4])] += amount
		}
		if bytes.Equal(r.Data[1], kapps.MarketKAppAddress) {
			net[string(r.Data[4])] -= amount
		}
	}
	return net
}

func TestMarketKApp_ConservesValue(t *testing.T) {
	t.Parallel()

	for _, c := range generateConservationCases(2796, 120) {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			currency := controlsCurrency(false, false)
			if c.klv {
				currency = klvCurrency()
			}
			env := newControlsEnv(t, c.fixOn, currency, c.collection)
			accounts := append([][]byte{
				ctrlIssuer, ctrlSeller, ctrlBuyer, ctrlPrevBidder, ctrlReferral, ctrlRoyalty, ctrlSplit,
				consRedirect, consRedirectSplit,
			}, c.payees...)
			// Every account exists up front: before FixAuditChangesV6 the receiver is loaded only
			// if it already exists (KLC-2751), which is not what this test is about.
			for _, addr := range accounts {
				acc, errLoad := env.accCacher.LoadUser(addr)
				require.NoError(t, errLoad)
				require.NoError(t, env.accCacher.UpdateUser(acc))
			}

			// The seller is funded with the deposit inside listAt, so it counts from the start.
			deposit := c.collection.Royalties.MarketFixed
			startKLV := env.total(accounts, kdautils.KLVIdentifier) + deposit
			startCurrency := env.total(accounts, currency.ID)
			startPayeesKLV := env.total(c.payees, kdautils.KLVIdentifier)
			startPayeesCurrency := env.total(c.payees, currency.ID)
			startRedirectSplitKLV := env.total([][]byte{consRedirectSplit}, kdautils.KLVIdentifier)
			startRoyaltyKLV := env.total([][]byte{ctrlRoyalty}, kdautils.KLVIdentifier)
			startRoyaltyCurrency := env.total([][]byte{ctrlRoyalty}, currency.ID)

			// requireEscrowed checks every unit that left the accounts is the order's escrow,
			// in the balances and in the receipts.
			requireEscrowed := func(id []byte, stage string) {
				t.Helper()

				order := env.order(id)
				escrowKLV, escrowCurrency := int64(0), int64(0)
				if !order.IsClaimed {
					escrowKLV, escrowCurrency = order.RoyaltiesFixedDeposit, order.CurrentBid
				}

				net := env.marketNetTransfers()
				if c.klv {
					require.Equal(t, startKLV, env.total(accounts, kdautils.KLVIdentifier)+escrowKLV+escrowCurrency, stage)
					require.Equal(t, escrowKLV+escrowCurrency, net[string(kdautils.KLVIdentifier)], stage+": receipts")
					return
				}
				require.Equal(t, startKLV, env.total(accounts, kdautils.KLVIdentifier)+escrowKLV, stage+": KLV")
				require.Equal(t, startCurrency, env.total(accounts, currency.ID)+escrowCurrency, stage+": currency")
				require.Equal(t, escrowKLV, net[string(kdautils.KLVIdentifier)], stage+": KLV receipts")
				require.Equal(t, escrowCurrency, net[string(currency.ID)], stage+": currency receipts")
			}

			marketType, price, reserve := kapps.MarketOrderData_Auction, int64(0), int64(1)
			if c.path == "Buy" {
				marketType, price, reserve = kapps.MarketOrderData_BuyItNow, c.bid, 0
			}
			id, status, err := env.listAt(marketType, price, reserve)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.Equal(t, deposit, env.order(id).RoyaltiesFixedDeposit, "the order owes exactly the deposit taken")
			requireEscrowed(id, "listed")

			if c.path != "Buy" {
				if c.outbid && c.bid > 1 {
					status, err = env.bid(id, ctrlPrevBidder, c.bid-1)
					require.NoError(t, err)
					require.Equal(t, transaction.Transaction_Ok, status)
					requireEscrowed(id, "first bid")
				}
				status, err = env.bid(id, ctrlBuyer, c.bid)
				require.NoError(t, err)
				require.Equal(t, transaction.Transaction_Ok, status)
				requireEscrowed(id, "winning bid")
			}

			if change := consChanges[c.change]; change != nil {
				change(env)
			}

			switch c.path {
			case "Buy":
				status, err = env.bid(id, ctrlBuyer, c.bid)
			case "EarlyAccept":
				status, err = env.claimOrder(id)
			case "ExpiredClaim":
				env.blockTime = ctrlBlockTime + 2000
				status, err = env.claimOrder(id)
			}
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)
			require.True(t, env.holdsNFT(ctrlBuyer))
			require.True(t, env.order(id).IsClaimed)
			requireEscrowed(id, "settled")

			// The bid left only the buyer, and the outbid bidder got everything back.
			require.Equal(t, ctrlFunds-c.bid, env.balance(ctrlBuyer))
			require.Equal(t, ctrlFunds, env.balance(ctrlPrevBidder))

			// Settlement pays the royalties the collection has now, not the ones it had at the
			// listing: the replaced splits get nothing and the new split gets its half of the
			// deposit (KLC-2796 kept this behaviour). Either change replaces the receiver, so the
			// one at the listing gets nothing.
			if c.change != "" {
				require.Equal(t, startRoyaltyKLV, env.total([][]byte{ctrlRoyalty}, kdautils.KLVIdentifier), "replaced receiver: KLV")
				require.Equal(t, startRoyaltyCurrency, env.total([][]byte{ctrlRoyalty}, currency.ID), "replaced receiver: currency")
			}
			if c.change == "UpdateRoyalties" {
				require.Equal(t, startPayeesKLV, env.total(c.payees, kdautils.KLVIdentifier), "replaced splits: KLV")
				require.Equal(t, startPayeesCurrency, env.total(c.payees, currency.ID), "replaced splits: currency")
				if deposit >= 2 {
					require.GreaterOrEqual(t, env.total([][]byte{consRedirectSplit}, kdautils.KLVIdentifier)-startRedirectSplitKLV, deposit/2, "updated split")
				}
			}

			// A settled order can be settled again by no path.
			_, err = env.claimOrder(id)
			require.Error(t, err)
			_, err = env.bid(id, ctrlBuyer, c.bid+1)
			require.Error(t, err)
			requireEscrowed(id, "after retries")
		})
	}
}
