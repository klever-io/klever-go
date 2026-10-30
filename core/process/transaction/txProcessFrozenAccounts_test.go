package transaction_test

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

// frozenSender is one of the consensus-frozen accounts (root / minter) from
// common/frozenAccounts.go, decoded to its raw public key.
var frozenSender, _ = hex.DecodeString("54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b")

// thawedSender is the swap-service account released again at the FixAuditChangesV4 fork.
var thawedSender, _ = hex.DecodeString("bb687dbba23e1844fec674a32cb8809f0d3207506c53fc3d637e40dc56708d63")

func buildKLVTransferTx(t *testing.T, from, to []byte) *transaction.Transaction {
	transferContract := transaction.TransferContract{
		ToAddress: to,
		AssetID:   kdautils.KLVIdentifier,
		Amount:    100,
	}
	tx, err := createTransactionMock(&transferContract, transaction.TXContract_TransferContractType, from, 0)
	require.NoError(t, err)
	return tx
}

// TestTxProcessor_FrozenAccounts verifies the consensus account freeze
// (FixMarketBuyOverflow fork) on the apply path: a transaction is rejected with
// ErrAccountFrozen only when the sender is frozen AND the fork is active.
func TestTxProcessor_FrozenAccounts(t *testing.T) {
	t.Run("post-fork: tx from a frozen account is rejected", func(t *testing.T) {
		c := NewController(t) // FixMarketBuyOverflow active by default (config epoch 0)
		c.AddUser(frozenSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		blk := c.CreateBlockHeader(0, 1, 1)
		tx := buildKLVTransferTx(t, frozenSender, receiver)

		err := c.execTx.ProcessTransaction(blk, []byte("h-frozen-active"), tx)
		require.ErrorIs(t, err, common.ErrAccountFrozen)
	})

	t.Run("post-fork: tx from a normal account is not frozen-rejected", func(t *testing.T) {
		c := NewController(t)
		c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		blk := c.CreateBlockHeader(0, 1, 1)
		tx := buildKLVTransferTx(t, sender, receiver)

		err := c.execTx.ProcessTransaction(blk, []byte("h-normal-active"), tx)
		require.NotErrorIs(t, err, common.ErrAccountFrozen)
	})

	t.Run("pre-fork: tx from a frozen account is not blocked by the freeze", func(t *testing.T) {
		c := NewController(t)
		// FixMarketBuyOverflow activation epoch in the future -> inactive at epoch 0.
		c.UpdateForkController(config.EnableEpochs{FixMarketBuyOverflow: 1000})
		c.AddUser(frozenSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		blk := c.CreateBlockHeader(0, 1, 1)
		tx := buildKLVTransferTx(t, frozenSender, receiver)

		err := c.execTx.ProcessTransaction(blk, []byte("h-frozen-prefork"), tx)
		require.NotErrorIs(t, err, common.ErrAccountFrozen)
	})

	t.Run("pre-audit-v4: tx from the swap service account is still rejected", func(t *testing.T) {
		c := NewController(t)
		// FixAuditChangesV4 activation epoch in the future -> the thaw has not happened yet.
		c.UpdateForkController(config.EnableEpochs{FixAuditChangesV4: 1000})
		c.AddUser(thawedSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		blk := c.CreateBlockHeader(0, 1, 1)
		tx := buildKLVTransferTx(t, thawedSender, receiver)

		err := c.execTx.ProcessTransaction(blk, []byte("h-thawed-prev4"), tx)
		require.ErrorIs(t, err, common.ErrAccountFrozen)
	})

	t.Run("post-audit-v4: tx from the swap service account is accepted", func(t *testing.T) {
		c := NewController(t) // FixMarketBuyOverflow and FixAuditChangesV4 both active (config epoch 0)
		c.AddUser(thawedSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		blk := c.CreateBlockHeader(0, 1, 1)
		tx := buildKLVTransferTx(t, thawedSender, receiver)

		err := c.execTx.ProcessTransaction(blk, []byte("h-thawed-v4"), tx)
		require.NotErrorIs(t, err, common.ErrAccountFrozen)
	})
}

// TestTxProcessor_PreProcessTransaction_FrozenAccounts verifies that from FixAuditChangesV7
// a frozen sender is refused in PreProcessTransaction, before the bandwidth fee, a KDA
// fee-pool swap or the nonce are committed, so a block carrying such a tx no longer
// verifies. Before V7, PreProcessTransaction lets it through and ProcessTransaction
// refuses it after the fee, as before.
func TestTxProcessor_PreProcessTransaction_FrozenAccounts(t *testing.T) {
	t.Run("post-v7: frozen sender is refused before any fee", func(t *testing.T) {
		c := NewController(t)
		before := c.AddUser(frozenSender, 1_000_000, kdautils.KLVIdentifier)
		balanceBefore := before.GetBalance(kdautils.KLVIdentifier, true)
		nonceBefore := before.GetNonce()
		c.AddUser(receiver, 0, nil)

		tx := buildKLVTransferTx(t, frozenSender, receiver)

		_, _, err := c.execTx.PreProcessTransaction(tx)
		require.ErrorIs(t, err, common.ErrAccountFrozen)
		require.Equal(t, transaction.Transaction_ParameterInvalid, tx.ResultCode)

		after := loadUserAccount(c.accCacher, frozenSender)
		require.Equal(t, balanceBefore, after.GetBalance(kdautils.KLVIdentifier, true), "a refused tx must not touch the balance")
		require.Equal(t, nonceBefore, after.GetNonce(), "a refused tx must not bump the nonce")
	})

	t.Run("post-v7: normal sender is not frozen-refused", func(t *testing.T) {
		c := NewController(t)
		c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		tx := buildKLVTransferTx(t, sender, receiver)

		ownerAcc, _, err := c.execTx.PreProcessTransaction(tx)
		require.NoError(t, err)
		require.NotNil(t, ownerAcc)
	})

	t.Run("post-v7: thawed sender is not frozen-refused", func(t *testing.T) {
		c := NewController(t)
		c.AddUser(thawedSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		tx := buildKLVTransferTx(t, thawedSender, receiver)

		ownerAcc, _, err := c.execTx.PreProcessTransaction(tx)
		require.NoError(t, err)
		require.NotNil(t, ownerAcc)
	})

	t.Run("pre-v7: frozen sender passes pre-processing and is refused on apply", func(t *testing.T) {
		c := NewController(t)
		c.UpdateForkController(config.EnableEpochs{FixAuditChangesV7: math.MaxUint32})
		c.AddUser(frozenSender, 1_000_000, kdautils.KLVIdentifier)
		c.AddUser(receiver, 0, nil)

		tx := buildKLVTransferTx(t, frozenSender, receiver)

		_, _, err := c.execTx.PreProcessTransaction(tx)
		require.NoError(t, err)

		err = c.execTx.ProcessTransaction(c.CreateBlockHeader(0, 1, 1), []byte("h-frozen-prev7"), tx)
		require.ErrorIs(t, err, common.ErrAccountFrozen)
	})
}
