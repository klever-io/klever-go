package kdaFeePool

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	transactionTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction_KDAFeePoolFlow_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "kdaFeePool",
			Ticker:        "KFP",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: 20_000_000_000_000,
			MaxSupply:     120_000_000_000_000,
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanMint: true,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	tx, err := transactionTest.GetAndCheckTransaction(nodes[0], txHash)
	require.NotNil(t, tx)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	asset, err := integrationTest.GetAsset(nodes[0], assetId)
	require.NotNil(t, asset)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     assetId,
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	depositAmount := int64(10_000_000)

	kdaPool, err := integrationTest.GetKDAPool(nodes[0], assetId)
	require.Nil(t, err)
	assert.Equal(t, assetId, string(kdaPool.KDA))
	assert.Equal(t, fixedRatioKDA, kdaPool.FRatioKDA)
	assert.Equal(t, fixedRatioKLV, kdaPool.FRatioKLV)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	// Deposit KLV ir our created kda pool
	depositTX, _, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_DepositContractType,
		models.DepositTXRequest{
			DepositType: int32(transaction.DepositContract_KDAPool),
			KDA:         assetId,
			CurrencyID:  "KLV",
			Amount:      depositAmount,
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	user := integrationTest.GetUserAccount(nodes[0], sender.Address)

	balanceBeforeDeposit := user.GetBalance(kdautils.KLVIdentifier, true)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	kdaPool, err = integrationTest.GetKDAPool(nodes[0], assetId)
	require.Nil(t, err)
	assert.Equal(t, depositAmount, kdaPool.KLVBalance)

	// Verify user balance after deposit transaction
	user = integrationTest.GetUserAccount(nodes[0], sender.Address)
	balanceAfterDeposit := user.GetBalance(kdautils.KLVIdentifier, true)
	require.Equal(t, balanceBeforeDeposit-(depositTX.GetTotalFees()+depositAmount), balanceAfterDeposit)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	// Withdraw KLV ir our created kda pool
	withdrawTX, _, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_WithdrawContractType,
		models.WithdrawTXRequest{
			KDA:          assetId,
			WithdrawType: int32(transaction.WithdrawContract_KDAPool),
			Amount:       depositAmount,
			CurrencyID:   string(kdautils.KLVIdentifier),
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	user = integrationTest.GetUserAccount(nodes[0], sender.Address)
	balanceBeforeWitdraw := user.GetBalance(kdautils.KLVIdentifier, true)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	user = integrationTest.GetUserAccount(nodes[0], sender.Address)
	balanceAfterWitdraw := user.GetBalance(kdautils.KLVIdentifier, true)
	require.Equal(t, balanceBeforeWitdraw+depositAmount-withdrawTX.GetTotalFees(), balanceAfterWitdraw)
}

func TestTransaction_CreateKDAPool_EmptyAsset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, hash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     "",
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	_, err = transactionTest.GetAndCheckTransaction(nodes[0], hash)
	require.NotNil(t, err)
}

func TestTransaction_CreateKDAPool_WrongAsset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, hash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     "invalid-asset-id",
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	_, err = transactionTest.GetAndCheckTransaction(nodes[0], hash)
	require.NotNil(t, err)
}

func TestTransaction_CreateKDAPool_InvalidRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(0)

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "kdaFeePool",
			Ticker:        "KFP",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: 20_000_000_000_000,
			MaxSupply:     120_000_000_000_000,
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanMint: true,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	tx := transactionTest.GetTransaction(nodes[0], txHash)
	require.NotNil(t, tx)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	asset, err := integrationTest.GetAsset(nodes[0], assetId)
	require.NotNil(t, asset)
	require.NoError(t, err)

	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     assetId,
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.NotNil(t, err)
}

func TestTransaction_DepositKDAFeePool_ZeroDepositAmount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "KdaFeePool",
			Ticker:        "KFP",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: 20_000_000_000_000,
			MaxSupply:     120_000_000_000_000,
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanMint: true,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	tx := transactionTest.GetTransaction(nodes[0], txHash)
	require.NotNil(t, tx)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	asset, err := integrationTest.GetAsset(nodes[0], assetId)
	require.NotNil(t, asset)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     assetId,
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	depositAmount := int64(0)

	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_DepositContractType,
		models.DepositTXRequest{
			DepositType: int32(transaction.DepositContract_KDAPool),
			KDA:         assetId,
			CurrencyID:  "KLV",
			Amount:      depositAmount,
		},
	)
	require.NotNil(t, err)
}

func TestTransaction_DepositKDAFeePool_WrongDepositAmount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)
	assert.Equal(t, numNodes, len(nodes))
	assert.Equal(t, numWallets, len(wallets))

	time.Sleep(300 * time.Millisecond)

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

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "KdaFeePool",
			Ticker:        "KFP",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: 20_000_000_000_000,
			MaxSupply:     120_000_000_000_000,
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanMint: true,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	tx := transactionTest.GetTransaction(nodes[0], txHash)
	require.NotNil(t, tx)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	asset, err := integrationTest.GetAsset(nodes[0], assetId)
	require.NotNil(t, asset)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     assetId,
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	// deposit more than the user have
	depositAmount := initialBalance + 1

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	_, hash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_DepositContractType,
		models.DepositTXRequest{
			DepositType: int32(transaction.DepositContract_KDAPool),
			KDA:         assetId,
			CurrencyID:  "KLV",
			Amount:      depositAmount,
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	_, err = transactionTest.GetAndCheckTransaction(nodes[0], hash)
	require.NotNil(t, err)
}

func TestTransaction_WithdrawKdaPool_InvalidAmount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numNodes := 2
	numWallets := 5
	numConsensusSize := 2

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, numNodes, numConsensusSize, numWallets, config)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

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

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		models.CreateAssetTXRequest{
			Type:          0,
			Name:          "KdaFeePool",
			Ticker:        "KFP",
			OwnerAddress:  senderAddress,
			Precision:     6,
			InitialSupply: 20_000_000_000_000,
			MaxSupply:     120_000_000_000_000,
			Royalties: &models.RoyaltiesInfo{
				Address: senderAddress,
			},
			Properties: &models.PropertiesInfo{
				CanMint: true,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	tx := transactionTest.GetTransaction(nodes[0], txHash)
	require.NotNil(t, tx)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	asset, err := integrationTest.GetAsset(nodes[0], assetId)
	require.NotNil(t, asset)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)

	// Create KDA Pool
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		models.AssetTriggerTXRequest{
			TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
			AssetID:     assetId,
			KDAPool: &models.KDAPoolInfo{
				Active:       true,
				AdminAddress: senderAddress,
				FRatioKLV:    fixedRatioKLV,
				FRatioKDA:    fixedRatioKDA,
			},
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	depositAmount := int64(10_000_000)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	// Deposit KLV ir our created kda pool
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_DepositContractType,
		models.DepositTXRequest{
			DepositType: int32(transaction.DepositContract_KDAPool),
			KDA:         assetId,
			CurrencyID:  "KLV",
			Amount:      depositAmount,
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
	blkNonce++

	// Withdraw KLV ir our created kda pool
	_, hash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_WithdrawContractType,
		models.WithdrawTXRequest{
			KDA:          assetId,
			WithdrawType: int32(transaction.WithdrawContract_KDAPool),
			Amount:       depositAmount + 1,
			CurrencyID:   string(kdautils.KLVIdentifier),
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
	require.Nil(t, err)

	_, err = transactionTest.GetAndCheckTransaction(nodes[0], hash)
	require.NotNil(t, err)

}

// A transaction paying its fees in KDA that fails during processing pays only the
// bandwidth share: the KApp fee share stays with the sender in KDA rather than coming
// back as KLV taken from the pool, on the proposer and the validator alike.
func TestTransaction_KDAFeeFailedTxKeepsKAppShareInKDA(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	config := commonTxTest.LoadDefaultConfigs(t, commonTxTest.ConfigPath)

	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	nodes, wallets, err := commonTxTest.CreateSetupForTxTests(initialBalance, 2, 2, 2, config)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	blkSlot := uint64(0)
	blkNonce := uint64(0)

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)

	sendAndPropose := func(txType transaction.TXContract_ContractType, contract any) []byte {
		blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
		blkNonce++

		_, txHash, errSend := commonTxTest.CreateAndSendTransaction(nodes[0], sender, txType, contract)
		require.Nil(t, errSend)

		requireTxReachedPools(t, nodes, txHash)

		_, _, _, errSend = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
		require.Nil(t, errSend)

		return txHash
	}

	txHash := sendAndPropose(transaction.TXContract_CreateAssetContractType, models.CreateAssetTXRequest{
		Type:          0,
		Name:          "kdaFeePool",
		Ticker:        "KFP",
		OwnerAddress:  senderAddress,
		Precision:     6,
		InitialSupply: 20_000_000_000_000,
		MaxSupply:     120_000_000_000_000,
		Royalties:     &models.RoyaltiesInfo{Address: senderAddress},
		Properties:    &models.PropertiesInfo{CanMint: true},
	})

	tx, err := transactionTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)
	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	fixedRatioKLV := int64(1000)
	fixedRatioKDA := int64(100000)
	sendAndPropose(transaction.TXContract_AssetTriggerContractType, models.AssetTriggerTXRequest{
		TriggerType: uint32(transaction.AssetTriggerContract_UpdateKDAFeePool),
		AssetID:     assetId,
		KDAPool: &models.KDAPoolInfo{
			Active:       true,
			AdminAddress: senderAddress,
			FRatioKLV:    fixedRatioKLV,
			FRatioKDA:    fixedRatioKDA,
		},
	})

	sendAndPropose(transaction.TXContract_DepositContractType, models.DepositTXRequest{
		DepositType: int32(transaction.DepositContract_KDAPool),
		KDA:         assetId,
		CurrencyID:  "KLV",
		Amount:      1_000_000_000,
	})

	kdaAt := func(klv int64) int64 { return klv * fixedRatioKDA / fixedRatioKLV }

	// kdaFeeTransfer signs a transfer of amount KLV with a KApp fee well above its cost,
	// paid in full with the pool's KDA
	kdaFeeTransfer := func(amount int64) (*transaction.Transaction, []byte) {
		tx, _, errCreate := nodes[0].CreateTransaction(
			uint32(transaction.TXContract_TransferContractType),
			senderAddress,
			sender.Nonce,
			[]byte("chainID"),
			[][]byte{[]byte("data")},
			0,
			[]json.RawMessage{mustMarshal(t, models.TransferTXRequest{
				Receiver: processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
				Amount:   amount,
			})},
		)
		require.Nil(t, errCreate)

		tx.RawData.KAppFee += 400_000_000
		tx.RawData.KDAFee = &transaction.Transaction_KDAFee{
			KDA:    []byte(assetId),
			Amount: kdaAt(tx.GetTotalFees()),
		}

		hash, errHash := tools.CalculateHash(nodes[0].InternalMarshalizer, nodes[0].Hasher, tx.GetRaw())
		require.Nil(t, errHash)
		tx.Signature[0], errHash = sender.TxSingleSigner.Sign(sender.SkTxSign, hash)
		require.Nil(t, errHash)
		sender.Nonce++

		return tx, hash
	}

	type observed struct {
		senderKLV int64
		senderKDA int64
		poolKLV   int64
		poolKDA   int64
	}
	observe := func(node *processorNode.ProcessorNode) observed {
		user := integrationTest.GetUserAccount(node, sender.Address)
		pool, errPool := integrationTest.GetKDAPool(node, assetId)
		require.Nil(t, errPool)

		return observed{
			senderKLV: user.GetBalance(kdautils.KLVIdentifier, true),
			senderKDA: user.GetBalance([]byte(assetId), true),
			poolKLV:   pool.KLVBalance,
			poolKDA:   pool.KDABalance,
		}
	}

	// proposeKDAFeeTransfer includes tx in one block and returns, per node, the
	// balances before and after it
	proposeKDAFeeTransfer := func(tx *transaction.Transaction, hash []byte, want transaction.Transaction_TXResult) ([]observed, []observed) {
		before := make([]observed, len(nodes))
		for i, n := range nodes {
			before[i] = observe(n)
		}

		_, errSend := nodes[0].SendTransaction(tx)
		require.Nil(t, errSend)

		requireTxReachedPools(t, nodes, hash)

		blkSlot = integrationTest.IncrementAndPrintSlot(blkSlot)
		blkNonce++
		_, _, _, errSend = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, blkSlot, blkNonce)
		require.Nil(t, errSend)

		after := make([]observed, len(nodes))
		for i, n := range nodes {
			included, errGet := n.GetTransaction(hash, true)
			require.Nil(t, errGet)
			require.Equal(t, want, included.Result, "node %d", i)

			after[i] = observe(n)
		}

		return before, after
	}

	// a transfer far above the sender's balance passes admission and fails in processing
	failing, failingHash := kdaFeeTransfer(initialBalance * 10)
	bandwidthFee := failing.RawData.BandwidthFee

	before, after := proposeKDAFeeTransfer(failing, failingHash, transaction.Transaction_FAILED)
	for i := range nodes {
		assert.Equal(t, before[i].senderKLV, after[i].senderKLV, "node %d: sender must not keep KLV swapped out of the pool", i)
		assert.Equal(t, before[i].senderKDA-kdaAt(bandwidthFee), after[i].senderKDA, "node %d", i)
		assert.Equal(t, before[i].poolKLV-bandwidthFee, after[i].poolKLV, "node %d", i)
		assert.Equal(t, before[i].poolKDA+kdaAt(bandwidthFee), after[i].poolKDA, "node %d", i)
	}

	// the same fee on a transfer that succeeds pays both shares in KDA
	transferAmount := int64(1_000_000)
	succeeding, succeedingHash := kdaFeeTransfer(transferAmount)
	totalFees := succeeding.GetTotalFees()

	before, after = proposeKDAFeeTransfer(succeeding, succeedingHash, transaction.Transaction_SUCCESS)
	for i := range nodes {
		assert.Equal(t, before[i].senderKLV-transferAmount, after[i].senderKLV, "node %d: the fee must be paid in KDA only", i)
		assert.Equal(t, before[i].senderKDA-kdaAt(totalFees), after[i].senderKDA, "node %d", i)
		assert.Equal(t, before[i].poolKLV-totalFees, after[i].poolKLV, "node %d", i)
		assert.Equal(t, before[i].poolKDA+kdaAt(totalFees), after[i].poolKDA, "node %d", i)
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	raw, err := json.Marshal(v)
	require.Nil(t, err)

	return raw
}

// requireTxReachedPools waits until every node knows txHash, so a block proposed next
// carries it. commonTxTest.CheckTXIsPending reads the current block header and so
// cannot run before the first block.
func requireTxReachedPools(t *testing.T, nodes []*processorNode.ProcessorNode, txHash []byte) {
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if _, err := n.GetTransaction(txHash, true); err != nil {
				return false
			}
		}

		return true
	}, 5*time.Second, 10*time.Millisecond)
}
