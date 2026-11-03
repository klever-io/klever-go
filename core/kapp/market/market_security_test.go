package market

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
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
		// Forks activate in order: a schedule before V3 is also before V7.
		forkController.FixAuditChangesV7Value = fixEnabled

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
		require.NoError(t, marketKappAcc.AddInternalKDA(collectionID, assetID, mock.NFTHolderRecord(1)))

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

// KLC-2802: ids derived through one contract collide before V7; from V7 they hash the tx and the
// id's index in it, and an id in use is refused.

var (
	idContract = ctrlSeller
	idTxA      = []byte("tx-a")
	idTxB      = []byte("tx-b")
	idNFT2     = []byte("8")
)

func idAssetID(nonce []byte) []byte {
	return []byte(string(ctrlCollectionID) + kapps.Sp + string(nonce))
}

// expectedMarketID is the id idContract derives in the controls block, salted as from
// FixAuditChangesV7 when salt is given and as before the fork when not.
func expectedMarketID(salt ...[]byte) []byte {
	return kdautils.ToMarketID(&sha256.Sha256{}, []byte("seed"), idContract, 1, 0, kdautils.MarketKeyLength, salt...)
}

// indexSalt salts the index-th id derived in a transaction from FixAuditChangesV7.
func indexSalt(index uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, index)
}

// inTx makes txHash the current transaction. Like txProcess, a new transaction gets a fresh
// KappContext, so the tx hash and the id index the market salts with come from the real context.
func (e *controlsEnv) inTx(txHash []byte) {
	if e.txCtx != nil && bytes.Equal(e.txCtx.TxHash(), txHash) {
		return
	}

	e.txCtx = kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: idContract,
		ContractType:   -1,
		Block:          &block.Block{Header: &block.BlockHeader{Timestamp: e.blockTime, RandSeed: []byte("seed")}},
		TxHash:         txHash,
		TX:             &transaction.Transaction{RawData: &transaction.Transaction_Raw{Nonce: 1}},
	})
}

func newIDEnv(t *testing.T, fixAuditChangesV7 bool) *controlsEnv {
	t.Helper()

	env := newControlsEnv(t, true, controlsCurrency(false, false), controlsCollection(false, false))
	env.setFixAuditChangesV7(fixAuditChangesV7)
	env.giveNFT(idContract, ctrlAssetID)
	env.giveNFT(idContract, idNFT2)
	return env
}

// listInTx lists NFT nonce of the controls collection from idContract in the transaction txHash and
// returns the order id Sell derived.
func (e *controlsEnv) listInTx(txHash []byte, nonce []byte) ([]byte, transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	e.inTx(txHash)
	status, err := e.listAs(idContract, nonce)
	if err != nil {
		return nil, status, err
	}

	sells := e.txCtx.Receipts().GetByType(int8(txProcess.Sell))
	return sells[len(sells)-1].Data[1], status, nil
}

// createInTx creates a marketplace named name from idContract in the transaction txHash and returns
// the id CreateMarketplace derived.
func (e *controlsEnv) createInTx(txHash []byte, name string) ([]byte, transaction.Transaction_TXResultCode, error) {
	e.t.Helper()

	e.inTx(txHash)
	status, err := e.market.CreateMarketplace(idContract, &transaction.CreateMarketplaceContract{Name: []byte(name)})
	if err != nil {
		return nil, status, err
	}

	created := e.txCtx.Receipts().GetByType(int8(txProcess.CreateMarketplace))
	return created[len(created)-1].Data[1], status, nil
}

// storeLiveOrderAt stores at id a live auction of another NFT, with a bid on it.
func (e *controlsEnv) storeLiveOrderAt(id []byte) {
	e.t.Helper()

	marketKappAcc, err := e.accCacher.LoadKApp(kapps.MarketKAppAddress)
	require.NoError(e.t, err)
	require.NoError(e.t, e.market.SetMarketOrder(marketKappAcc, &kapps.MarketOrderData{
		ID: id, MarketplaceID: ctrlMarketplaceID, MarketType: kapps.MarketOrderData_Auction,
		OwnerAddress: ctrlBuyer, CollectionID: ctrlCollectionID, AssetID: []byte("9"), CurrencyID: e.currency.ID,
		ReservePrice: ctrlBid / 2, CurrentBid: ctrlBid, CurrentBidder: ctrlPrevBidder, EndTime: ctrlBlockTime + 1000,
	}))
	require.NoError(e.t, e.accCacher.UpdateKapp(marketKappAcc))
}

func (e *controlsEnv) marketplaceName(id []byte) string {
	e.t.Helper()

	_, marketplace, err := e.market.GetMarketplace(id)
	require.NoError(e.t, err)
	return string(marketplace.Name)
}

func requireListed(t *testing.T, status transaction.Transaction_TXResultCode, err error) {
	t.Helper()

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, status)
}

func TestMarketSell_OrderIDThroughOneContract(t *testing.T) {
	t.Parallel()

	t.Run("before V7 two transactions share one id and the second listing overwrites the first", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, false)
		idA, status, err := env.listInTx(idTxA, ctrlAssetID)
		requireListed(t, status, err)
		idB, status, err := env.listInTx(idTxB, idNFT2)
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(), idA, "the pre-fork id does not hash the tx or an id index")
		require.Equal(t, idA, idB)
		require.Equal(t, idNFT2, env.order(idA).AssetID, "the second listing replaced the first order")
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress), "the first NFT stays escrowed with no order")
	})

	t.Run("from V7 two transactions get distinct ids and both orders stay live", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		idA, status, err := env.listInTx(idTxA, ctrlAssetID)
		requireListed(t, status, err)
		idB, status, err := env.listInTx(idTxB, idNFT2)
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(idTxA, indexSalt(0)), idA)
		require.Equal(t, expectedMarketID(idTxB, indexSalt(0)), idB)
		require.Equal(t, ctrlAssetID, env.order(idA).AssetID)
		require.Equal(t, idNFT2, env.order(idB).AssetID)
	})

	t.Run("from V7 two listings in one transaction get distinct ids", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		idA, status, err := env.listInTx(idTxA, ctrlAssetID)
		requireListed(t, status, err)
		idB, status, err := env.listInTx(idTxA, idNFT2)
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(idTxA, indexSalt(0)), idA)
		require.Equal(t, expectedMarketID(idTxA, indexSalt(1)), idB)
		require.Equal(t, ctrlAssetID, env.order(idA).AssetID)
		require.Equal(t, idNFT2, env.order(idB).AssetID)
	})

	t.Run("from V7 a relist of the same NFT after a cancel in one transaction gets a new id", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		idA, status, err := env.listInTx(idTxA, ctrlAssetID)
		requireListed(t, status, err)
		status, err = env.market.CancelOrder(idContract, &transaction.CancelMarketOrderContract{OrderID: idA})
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		idB, status, err := env.listInTx(idTxA, ctrlAssetID)
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(idTxA, indexSalt(1)), idB)
		require.True(t, env.order(idA).IsClaimed, "the cancelled order keeps its record")
		relisted := env.order(idB)
		require.False(t, relisted.IsClaimed)
		require.Equal(t, ctrlAssetID, relisted.AssetID)
		require.True(t, env.holdsNFT(kapps.MarketKAppAddress), "the relisted NFT is escrowed")
	})

	t.Run("from V7 an id in use is refused", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		id := expectedMarketID(idTxA, indexSalt(0))
		env.storeLiveOrderAt(id)

		_, status, err := env.listInTx(idTxA, ctrlAssetID)

		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Equal(t, transaction.Transaction_KeyConflict, status)
		errorReceipts := env.txCtx.Receipts().GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldInvalidOrder), errorReceipts[0].Data[1])
		live := env.order(id)
		require.Equal(t, ctrlBuyer, live.OwnerAddress, "the live order is untouched")
		require.Equal(t, []byte("9"), live.AssetID)
		require.Equal(t, ctrlBid, live.CurrentBid, "its bid is untouched")
		require.Equal(t, ctrlPrevBidder, live.CurrentBidder)
		require.True(t, env.holdsNFT(idContract), "the refused NFT stays with the seller")
	})

	t.Run("before V7 an id in use is overwritten", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, false)
		id := expectedMarketID()
		env.storeLiveOrderAt(id)

		_, status, err := env.listInTx(idTxA, ctrlAssetID)

		requireListed(t, status, err)
		replaced := env.order(id)
		require.Equal(t, idContract, replaced.OwnerAddress)
		require.Equal(t, ctrlAssetID, replaced.AssetID)
		require.Zero(t, replaced.CurrentBid, "the bid on the overwritten order is lost")
		require.False(t, env.holdsNFT(idContract))
	})
}

func TestMarketCreateMarketplace_IDThroughOneContract(t *testing.T) {
	t.Parallel()

	t.Run("before V7 two transactions share one id and the second creation overwrites the first", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, false)
		idA, status, err := env.createInTx(idTxA, "first")
		requireListed(t, status, err)
		idB, status, err := env.createInTx(idTxB, "second")
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(), idA, "the pre-fork id does not hash the tx")
		require.Equal(t, idA, idB)
		require.Equal(t, "second", env.marketplaceName(idA))
	})

	t.Run("from V7 two transactions get distinct ids", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		idA, status, err := env.createInTx(idTxA, "first")
		requireListed(t, status, err)
		idB, status, err := env.createInTx(idTxB, "second")
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(idTxA, indexSalt(0)), idA)
		require.Equal(t, expectedMarketID(idTxB, indexSalt(0)), idB)
		require.Equal(t, "first", env.marketplaceName(idA))
		require.Equal(t, "second", env.marketplaceName(idB))
	})

	t.Run("from V7 two creations in one transaction get distinct ids", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		idA, status, err := env.createInTx(idTxA, "first")
		requireListed(t, status, err)
		idB, status, err := env.createInTx(idTxA, "second")
		requireListed(t, status, err)

		require.Equal(t, expectedMarketID(idTxA, indexSalt(0)), idA)
		require.Equal(t, expectedMarketID(idTxA, indexSalt(1)), idB)
		require.Equal(t, "first", env.marketplaceName(idA))
		require.Equal(t, "second", env.marketplaceName(idB))
	})

	t.Run("from V7 an id in use is refused", func(t *testing.T) {
		t.Parallel()

		env := newIDEnv(t, true)
		id := expectedMarketID(idTxA, indexSalt(0))
		marketKappAcc, err := env.accCacher.LoadKApp(kapps.MarketKAppAddress)
		require.NoError(t, err)
		require.NoError(t, env.market.SetMarketplace(marketKappAcc, &kapps.Marketplace{ID: id, OwnerAddress: ctrlBuyer, Name: []byte("first")}))
		require.NoError(t, env.accCacher.UpdateKapp(marketKappAcc))

		_, status, err := env.createInTx(idTxA, "second")

		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Equal(t, transaction.Transaction_KeyConflict, status)
		errorReceipts := env.txCtx.Receipts().GetByType(int8(kapp.ReceiptTypeError))
		require.Len(t, errorReceipts, 1)
		require.Equal(t, []byte(common.ErrFieldMarketplaceError), errorReceipts[0].Data[1])
		require.Equal(t, "first", env.marketplaceName(id))
	})
}
