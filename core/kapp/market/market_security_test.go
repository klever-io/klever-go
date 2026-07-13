package market

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/require"
)

// TestMarketKApp_Buy_ZombieOrderIsClaimedGuard is the regression test for
// GHSA-26r5-4mm2-px5c ("zombie-order theft": Buy missing the IsClaimed guard).
//
// A seller settles a resting-bid auction early via Claim's seller-accept branch,
// which sets IsClaimed=true but does NOT reset EndTime and does NOT delete the
// order -> a "zombie" order that still reads as live. A later bidder that calls
// Buy on that zombie order is debited and then stranded, because both Claim and
// CancelOrder revert on IsClaimed.
//
// The fix adds an IsClaimed guard at the top of Buy, gated behind
// FixAuditChangesV3. This test asserts both fork states:
//   - PostFix (guard active): the victim's Buy is rejected; no funds move.
//   - PreFix  (legacy):       the victim's Buy still succeeds and the victim is
//     stranded, proving historical/replay behaviour is preserved.
func TestMarketKApp_Buy_ZombieOrderIsClaimedGuard(t *testing.T) {
	t.Parallel()

	const (
		blockTime    = int64(1000)
		endTime      = int64(1_001_000) // future relative to blockTime
		reserve      = int64(1_000_000) // R
		bidX         = int64(1_000_000) // attacker's resting bid (== reserve)
		bidY         = int64(2_000_000) // victim's bid on the zombie order (> X)
		fundAttacker = int64(10_000_000)
		fundVictim   = int64(10_000_000)
	)

	klv := kdautils.KLVIdentifier
	collectionID := []byte("ZOMBIE-COLL")
	assetID := []byte("1")
	marketplaceID := []byte("mp-zombie")
	orderID := []byte("order-zombie")

	// driveToZombie lists a resting-bid auction, places the attacker's resting bid,
	// and settles it early via Claim so the order becomes a claimed-but-live
	// "zombie". Steps 1-2 act on a not-yet-claimed order, so the setup is identical
	// for both fork states.
	driveToZombie := func(t *testing.T, fixEnabled bool) (*marketKapp, func([]byte) int64) {
		attacker := defaultAddr // A == S (seller and first bidder)
		victim := defaultOther  // B

		marketKApp, accCacher, forkController := createTestMarketKApp(t)
		forkController.FixAuditChangesV3Value = fixEnabled

		fund := func(addr []byte, amount int64) {
			acc, err := accCacher.LoadUser(addr)
			require.NoError(t, err)
			require.NoError(t, acc.AddToBalance(amount, klv, false))
			require.NoError(t, accCacher.UpdateUser(acc))
		}
		fund(attacker, fundAttacker)
		fund(victim, fundVictim)

		marketKappAcc, err := accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)
		require.NoError(t, marketKApp.SetMarketplace(marketKappAcc, &kapps.Marketplace{
			ID:                 marketplaceID,
			OwnerAddress:       attacker,
			Name:               []byte("Zombie Market"),
			ReferralAddress:    attacker,
			ReferralPercentage: 0,
		}))
		// NFT escrowed in the market KApp (as if the seller deposited it via Sell).
		require.NoError(t, marketKappAcc.AddInternalKDA(collectionID, assetID, []byte("nft-data")))

		// Auction with Price=0, ReservePrice=R -> bids REST (no auto-settle).
		require.NoError(t, marketKApp.SetMarketOrder(marketKappAcc, &kapps.MarketOrderData{
			ID:            orderID,
			MarketplaceID: marketplaceID,
			MarketType:    kapps.MarketOrderData_Auction,
			OwnerAddress:  attacker,
			CollectionID:  collectionID,
			AssetID:       assetID,
			CurrencyID:    klv,
			Price:         0,
			ReservePrice:  reserve,
			StartTime:     blockTime,
			EndTime:       endTime,
			IsClaimed:     false,
		}))
		require.NoError(t, accCacher.UpdateKapp(marketKappAcc))

		receiptsStub := mock.NewReceiptsContextStub()
		ctx := &mock.KAppContextStub{
			ContractIDCalled: func() int { return 0 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsStub },
			BlockCalled: func() *block.Block {
				return &block.Block{Header: &block.BlockHeader{Timestamp: blockTime}}
			},
			TxNonceCalled: func() uint64 { return 1 },
		}
		// Zero-royalty asset: executeBuyMarket pays only the owner payout (== bid).
		asset := &kapps.KDAData{
			OwnerAddress: attacker,
			Royalties: &kapps.RoyaltiesData{
				Address:          attacker,
				MarketPercentage: 0,
				SplitRoyalties:   make(map[string]*kapps.RoyaltySplitData),
			},
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

		balance := func(addr []byte) int64 {
			a, e := accCacher.LoadUser(addr)
			require.NoError(t, e)
			return a.GetBalance(klv, false)
		}

		// Step 1: attacker places a RESTING bid X (Price=0 -> no settlement).
		status, err := marketKApp.Buy(attacker, &transaction.BuyContract{ID: orderID, CurrencyID: klv, Amount: bidX})
		require.NoError(t, err, "resting bid should succeed")
		require.Equal(t, transaction.Transaction_Ok, status)

		_, rested, err := marketKApp.GetMarketOrder(orderID)
		require.NoError(t, err)
		require.Equal(t, bidX, rested.CurrentBid, "bid must REST, not settle")
		require.False(t, rested.IsClaimed)
		require.Equal(t, fundAttacker-bidX, balance(attacker))

		// Step 2: seller(=attacker) accepts the resting bid early via Claim. This
		// settles the order (IsClaimed=true) but leaves EndTime in the future and
		// does not delete it -> zombie order.
		status, err = marketKApp.Claim(attacker, &transaction.ClaimContract{ID: orderID})
		require.NoError(t, err, "early seller-accept claim should succeed")
		require.Equal(t, transaction.Transaction_Ok, status)

		_, settled, err := marketKApp.GetMarketOrder(orderID)
		require.NoError(t, err, "order must remain loadable after early claim (not deleted)")
		require.True(t, settled.IsClaimed, "order is now claimed/settled")
		require.Equal(t, endTime, settled.EndTime, "settle path leaves EndTime in the future (order looks live)")
		require.Equal(t, fundAttacker, balance(attacker), "attacker recovered X as the owner payout")

		return marketKApp, balance
	}

	t.Run("PostFix_RejectsBuyOnClaimedOrder", func(t *testing.T) {
		t.Parallel()
		attacker := defaultAddr
		victim := defaultOther

		marketKApp, balance := driveToZombie(t, true)

		// Victim tries to Buy the zombie order: the guard must reject it.
		status, err := marketKApp.Buy(victim, &transaction.BuyContract{ID: orderID, CurrencyID: klv, Amount: bidY})
		require.Error(t, err, "Buy on an already-claimed order must be rejected")
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)

		// No funds moved: victim keeps everything, attacker gains no phantom refund.
		require.Equal(t, fundVictim, balance(victim), "victim must not be debited")
		require.Equal(t, fundAttacker, balance(attacker), "attacker must not receive a phantom refund")

		// The order is untouched by the rejected Buy.
		_, order, err := marketKApp.GetMarketOrder(orderID)
		require.NoError(t, err)
		require.Equal(t, attacker, order.CurrentBidder, "current bidder unchanged")
		require.Equal(t, bidX, order.CurrentBid, "current bid unchanged")
		require.True(t, order.IsClaimed)
	})

	t.Run("PreFix_LegacyBehaviourStrandsVictim", func(t *testing.T) {
		t.Parallel()
		attacker := defaultAddr
		victim := defaultOther

		marketKApp, balance := driveToZombie(t, false)

		// Legacy (pre-fork): Buy on the zombie order still SUCCEEDS (the vuln),
		// proving the fix does not alter historical replay behaviour.
		status, err := marketKApp.Buy(victim, &transaction.BuyContract{ID: orderID, CurrencyID: klv, Amount: bidY})
		require.NoError(t, err, "legacy behaviour: Buy on a claimed order still succeeds")
		require.Equal(t, transaction.Transaction_Ok, status)

		require.Equal(t, fundVictim-bidY, balance(victim), "victim debited Y")
		require.Equal(t, fundAttacker+bidX, balance(attacker), "attacker siphoned a phantom refund X, funded by the victim")

		// Harm: the victim is now stranded on a claimed order.
		status, err = marketKApp.Claim(victim, &transaction.ClaimContract{ID: orderID})
		require.Error(t, err, "victim's Claim reverts on IsClaimed")
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)

		status, err = marketKApp.CancelOrder(victim, &transaction.CancelMarketOrderContract{OrderID: orderID})
		require.Error(t, err, "victim's CancelOrder reverts on IsClaimed")
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)

		require.Equal(t, fundVictim-bidY, balance(victim), "victim is permanently down Y with no recovery path")
	})
}
