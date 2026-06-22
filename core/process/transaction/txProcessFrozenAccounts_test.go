package transaction_test

import (
	"encoding/hex"
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
}
