package fpr

import (
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction_TransactionFPR(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numWallets := 5
	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	blkSlot := uint64(0)
	blkNonce := uint64(0)
	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)

	initialSupply := int64(20_000_000_000_000) // 20M

	tx1, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "FPR Test",
			Ticker:        "FPR",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: initialSupply,
			MaxSupply:     120_000_000_000_000, // 120M
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanFreeze:      true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
			},
			Attributes: &models.AttributesInfo{
				IsPaused:                 false,
				IsNFTMintStopped:         false,
				IsRoyaltiesChangeStopped: false,
			},
			Staking: &models.StakingInfo{
				InterestType: 1,
			},
		},
	)
	require.NoError(t, err)

	time.Sleep(400 * time.Millisecond)

	for i := 0; i < 6; i++ {
		blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
		require.NoError(t, err, fmt.Sprintf("Index: %d", i))
	}

	mainWalletFee := tx1.GetTotalFees()

	tx, err := commonTxTest.GetAndCheckTransaction(nodes[1], txHash)
	require.NoError(t, err)
	require.NotNil(t, tx)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)
	require.NotEmpty(t, assetId)

	//check account balances after create asset
	{
		senderAccount := integrationTest.GetUserAccount(nodes[1], sender.Address)
		require.NotNil(t, senderAccount)
		assert.Equal(t, uint64(1), senderAccount.GetNonce())
		assert.Equal(t, initialBalance-(tx1.GetTotalFees()), senderAccount.GetBalance(kdautils.KLVIdentifier))
		assert.Equal(t, initialSupply, senderAccount.GetBalance([]byte(assetId)))
	}

	//check account initial supply

	amountMinted := int64(1_000_000_000) // 1K FPR token

	tx2, txHash2, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: 0,
			AssetID:     assetId,
			Receiver:    processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
			Amount:      amountMinted,
		})
	require.NoError(t, err)

	mainWalletFee += tx2.GetTotalFees()

	tx3, txHash3, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: 0,
			AssetID:     assetId,
			Receiver:    processorNode.TestAddressPubkeyConverter.Encode(wallets[2].Address),
			Amount:      amountMinted,
		})
	require.NoError(t, err)

	mainWalletFee += tx3.GetTotalFees()

	time.Sleep(400 * time.Millisecond)

	blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash2)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash3)
	require.NoError(t, err)
	// check if mint works as expected
	{
		receiver1 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver1.IsInterfaceNil())
		assert.Equal(t, amountMinted, receiver1.GetBalance([]byte(assetId)))

		receiver2 := integrationTest.GetUserAccount(nodes[1], wallets[2].Address)
		require.False(t, receiver2.IsInterfaceNil())
		assert.Equal(t, amountMinted, receiver2.GetBalance([]byte(assetId)))

		senderAccount := integrationTest.GetUserAccount(nodes[1], sender.Address)
		require.False(t, senderAccount.IsInterfaceNil())
		assert.Equal(t, uint64(3), senderAccount.GetNonce())
		assert.Equal(t, initialBalance-mainWalletFee, senderAccount.GetBalance(kdautils.KLVIdentifier))
	}

	tx4, txHash4, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[1],
		transaction.TXContract_FreezeContractType,
		models.FreezeTXRequest{
			KDA:    assetId,
			Amount: amountMinted,
		})
	require.NoError(t, err)

	tx5, txHash5, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[2],
		transaction.TXContract_FreezeContractType,
		models.FreezeTXRequest{
			KDA:    assetId,
			Amount: amountMinted,
		})
	require.NoError(t, err)

	time.Sleep(400 * time.Millisecond)

	blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash4)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash5)
	require.NoError(t, err)

	// check balances after freeze
	{
		receiver1 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver1.IsInterfaceNil())
		assert.Equal(t, int64(0), receiver1.GetBalance([]byte(assetId)))
		assert.Equal(t, initialBalance-tx4.GetTotalFees(), receiver1.GetBalance(kdautils.KLVIdentifier))
		assert.Equal(t, amountMinted, receiver1.GetFrozenBalance([]byte(assetId)))

		receiver2 := integrationTest.GetUserAccount(nodes[1], wallets[2].Address)
		require.False(t, receiver2.IsInterfaceNil())
		assert.Equal(t, int64(0), receiver2.GetBalance([]byte(assetId)))
		assert.Equal(t, initialBalance-tx5.GetTotalFees(), receiver2.GetBalance(kdautils.KLVIdentifier))
		assert.Equal(t, amountMinted, receiver2.GetFrozenBalance([]byte(assetId)))
	}

	depositedAmount := int64(1000_000_000) // 1000 KLV
	depositedCurrency := string(kdautils.KLVIdentifier)
	depositedHolders := int64(2) // 2 addresses freezing the asset

	tx6, txHash6, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[0],
		transaction.TXContract_DepositContractType,
		models.DepositTXRequest{
			DepositType: 0,
			KDA:         assetId,
			CurrencyID:  depositedCurrency,
			Amount:      depositedAmount,
		})
	require.NoError(t, err)

	time.Sleep(400 * time.Millisecond)

	blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash6)
	require.NoError(t, err)

	mainWalletFee += tx6.GetTotalFees()
	mainWalletFee += depositedAmount

	_, staking, err := integrationTest.GetStaking(nodes[1], []byte(assetId))
	require.NoError(t, err)

	{
		senderAccount := integrationTest.GetUserAccount(nodes[1], wallets[0].Address)
		require.False(t, senderAccount.IsInterfaceNil())
		assert.Equal(t, uint64(4), senderAccount.GetNonce())
		assert.Equal(t, initialBalance-mainWalletFee, senderAccount.GetBalance(kdautils.KLVIdentifier))
	}

	blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		blkSlot, blkNonce, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
		require.NoError(t, err)
	}

	{
		receiver1 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver1.IsInterfaceNil())

		receiver1Kda, err := receiver1.GetUserKDA([]byte(assetId), nil)
		require.NoError(t, err)
		receiver1Rewards, err := receiver1.ComputeAvailableClaim([]byte(assetId),
			nodes[1].Blkc.GetCurrentBlockHeader().GetEpoch(),
			nodes[1].Blkc.GetCurrentBlockHeader().GetTimestamp(),
			receiver1Kda,
			staking,
			nodes[1].ForkController,
		)
		require.NoError(t, err)

		assert.Equal(t, depositedAmount/depositedHolders, receiver1Rewards[depositedCurrency])

		receiver2 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver2.IsInterfaceNil())

		receiver2Kda, err := receiver2.GetUserKDA([]byte(assetId), nil)
		require.NoError(t, err)
		receiver2Rewards, err := receiver2.ComputeAvailableClaim([]byte(assetId),
			nodes[1].Blkc.GetCurrentBlockHeader().GetEpoch(),
			nodes[1].Blkc.GetCurrentBlockHeader().GetTimestamp(),
			receiver2Kda,
			staking,
			nodes[1].ForkController,
		)
		require.NoError(t, err)

		assert.Equal(t, depositedAmount/depositedHolders, receiver2Rewards[depositedCurrency])
	}

	tx7, txHash7, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[1],
		transaction.TXContract_ClaimContractType,
		models.ClaimTXRequest{
			ClaimType: 0,
			ID:        assetId,
		})
	require.NoError(t, err)

	tx8, txHash8, err := commonTxTest.CreateAndSendTransaction(nodes[0],
		wallets[2],
		transaction.TXContract_ClaimContractType,
		models.ClaimTXRequest{
			ClaimType: 0,
			ID:        assetId,
		})
	require.NoError(t, err)

	time.Sleep(400 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash7)
	require.NoError(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[1], txHash8)
	require.NoError(t, err)

	{
		receiver1 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver1.IsInterfaceNil())

		receiver1Kda, err := receiver1.GetUserKDA([]byte(assetId), nil)
		require.NoError(t, err)
		receiver1Rewards, err := receiver1.ComputeAvailableClaim([]byte(assetId),
			nodes[1].Blkc.GetCurrentBlockHeader().GetEpoch(),
			nodes[1].Blkc.GetCurrentBlockHeader().GetTimestamp(),
			receiver1Kda,
			staking,
			nodes[1].ForkController,
		)
		require.NoError(t, err)

		assert.Equal(t, uint64(2), receiver1.GetNonce())
		assert.Equal(t, int64(0), receiver1Rewards[depositedCurrency])
		assert.Equal(t, initialBalance-tx4.GetTotalFees()-tx7.GetTotalFees()+(depositedAmount/depositedHolders), receiver1.GetBalance(kdautils.KLVIdentifier))

		receiver2 := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
		require.False(t, receiver2.IsInterfaceNil())

		receiver2Kda, err := receiver2.GetUserKDA([]byte(assetId), nil)
		require.NoError(t, err)
		receiver2Rewards, err := receiver2.ComputeAvailableClaim([]byte(assetId),
			nodes[1].Blkc.GetCurrentBlockHeader().GetEpoch(),
			nodes[1].Blkc.GetCurrentBlockHeader().GetTimestamp(),
			receiver2Kda,
			staking,
			nodes[1].ForkController,
		)
		require.NoError(t, err)

		assert.Equal(t, uint64(2), receiver2.GetNonce())
		assert.Equal(t, int64(0), receiver2Rewards[depositedCurrency])
		assert.Equal(t, initialBalance-tx5.GetTotalFees()-tx8.GetTotalFees()+(depositedAmount/depositedHolders), receiver2.GetBalance(kdautils.KLVIdentifier))
	}
}
