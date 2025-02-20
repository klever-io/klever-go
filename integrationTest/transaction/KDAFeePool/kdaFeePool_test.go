package kdaFeePool

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	transactionTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
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
