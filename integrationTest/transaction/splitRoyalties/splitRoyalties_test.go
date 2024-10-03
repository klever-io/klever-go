package splitroyalties

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/require"
)

func TestTransaction_SplitRoyalties_Fungible_TransferPercentage_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)
	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	transferAmount := int64(100_000)
	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)

	receiver := wallets[1]
	receiverAddress := processorNode.TestAddressPubkeyConverter.Encode(receiver.Address)

	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	// Create Spliy Royalties Asset

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:          uint32(kapps.KDAData_Fungible), // Fungible
		Name:          "SplitRoyalties",
		Ticker:        "SRT",
		OwnerAddress:  senderAddress,
		InitialSupply: 1_000_000,
		MaxSupply:     1_000_000,
		Royalties: &models.RoyaltiesInfo{
			Address: mainRoyaltyAddress,
			TransferPercentage: []*models.RoyaltyData{{
				Amount:     100,
				Percentage: 100,
			}},
			SplitRoyalties: make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferPercentage: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferPercentage: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		models.TransferTXRequest{
			Receiver: receiverAddress,
			Amount:   transferAmount,
			KDA:      assetId,
		},
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	estimatedTotalRoyalty := computeRoyalty(transferAmount, 100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	time.Sleep(1000 * time.Millisecond)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	mainRoyaltyKDABalance, err := mainRoyaltyAccount.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)
	splitAccount1KDABalance, err := splitAccount1.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)
	splitAccount2KDABalance, err := splitAccount2.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyKDABalance.Balance)
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1KDABalance.Balance)
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2KDABalance.Balance)
}

func TestTransaction_SplitRoyalties_Fungible_TransferFixed_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	transferAmount := int64(100_000)
	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)

	receiver := wallets[1]
	receiverAddress := processorNode.TestAddressPubkeyConverter.Encode(receiver.Address)

	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	// Create Spliy Royalties Asset

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:          uint32(kapps.KDAData_Fungible), // Fungible
		Name:          "SplitRoyalties",
		Ticker:        "SRT",
		OwnerAddress:  senderAddress,
		InitialSupply: 1_000_000,
		MaxSupply:     1_000_000,
		Royalties: &models.RoyaltiesInfo{
			Address:        mainRoyaltyAddress,
			TransferFixed:  100,
			SplitRoyalties: make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		models.TransferTXRequest{
			Receiver: receiverAddress,
			Amount:   transferAmount,
			KDA:      assetId,
		},
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	time.Sleep(1000 * time.Millisecond)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTransaction_SplitRoyalties_Fungible_TransferFixedAndPercentage_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)
	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	transferAmount := int64(100_000)
	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)

	receiver := wallets[1]
	receiverAddress := processorNode.TestAddressPubkeyConverter.Encode(receiver.Address)

	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	// Create Spliy Royalties Asset

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:          uint32(kapps.KDAData_Fungible), // Fungible
		Name:          "SplitRoyalties",
		Ticker:        "SRT",
		OwnerAddress:  senderAddress,
		InitialSupply: 1_000_000,
		MaxSupply:     1_000_000,
		Royalties: &models.RoyaltiesInfo{
			Address:            mainRoyaltyAddress,
			TransferFixed:      100,
			TransferPercentage: []*models.RoyaltyData{{Amount: 100, Percentage: 100}},
			SplitRoyalties:     make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed:      100,
		PercentTransferPercentage: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed:      200,
		PercentTransferPercentage: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		models.TransferTXRequest{
			Receiver: receiverAddress,
			Amount:   transferAmount,
			KDA:      assetId,
		},
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	estimatedFixedTotalRoyalty := int64(100)
	estimatedFixedTotalSplitRoyalty := computeRoyalty(estimatedFixedTotalRoyalty, 300)
	estimatedPercentageTotalRoyalty := computeRoyalty(transferAmount, 100)
	estimatedPercentageTotalSplitRoyalty := computeRoyalty(estimatedPercentageTotalRoyalty, 300)
	time.Sleep(1000 * time.Millisecond)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedFixedTotalRoyalty-estimatedFixedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedFixedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedFixedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))

	mainRoyaltyKDABalance, err := mainRoyaltyAccount.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)
	splitAccount1KDABalance, err := splitAccount1.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)
	splitAccount2KDABalance, err := splitAccount2.GetUserKDA([]byte(assetId), nil, true)
	require.Nil(t, err)

	require.Equal(t, estimatedPercentageTotalRoyalty-estimatedPercentageTotalSplitRoyalty, mainRoyaltyKDABalance.Balance)
	require.Equal(t, computeRoyalty(estimatedPercentageTotalRoyalty, 100), splitAccount1KDABalance.Balance)
	require.Equal(t, computeRoyalty(estimatedPercentageTotalRoyalty, 200), splitAccount2KDABalance.Balance)

}

func TestTransaction_SplitRoyalties_NFT_TransferFixed_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)
	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	receiver := wallets[1]
	receiverAddress := processorNode.TestAddressPubkeyConverter.Encode(receiver.Address)

	// Create Spliy Royalties NFT

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:         uint32(kapps.KDAData_NonFungible),
		Name:         "SplitRoyalties",
		Ticker:       "SRT",
		OwnerAddress: senderAddress,
		MaxSupply:    10_000,
		Royalties: &models.RoyaltiesInfo{
			Address:        mainRoyaltyAddress,
			TransferFixed:  100,
			SplitRoyalties: make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentTransferFixed: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)

	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	// Mint NFT

	numMinted := int64(3)

	mintContract := models.AssetTriggerTXRequest{
		TriggerType: uint32(transaction.AssetTriggerContract_Mint),
		AssetID:     assetId,
		Receiver:    senderAddress,
		Amount:      numMinted,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		mintContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		models.TransferTXRequest{
			Receiver: receiverAddress,
			Amount:   1,
			KDA:      assetId + kapps.Sp + "1",
		},
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	time.Sleep(1000 * time.Millisecond)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTransaction_SplitRoyalties_NFT_MarketPercentage_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)
	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)
	marketplaceReferral := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)
	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	receiver := wallets[1]

	// Create Spliy Royalties NFT

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:         uint32(kapps.KDAData_NonFungible),
		Name:         "SplitRoyalties",
		Ticker:       "SRT",
		OwnerAddress: senderAddress,
		MaxSupply:    10_000,
		Royalties: &models.RoyaltiesInfo{
			Address:          mainRoyaltyAddress,
			MarketPercentage: 100,
			SplitRoyalties:   make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketPercentage: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketPercentage: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)

	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	// Mint NFT

	numMinted := int64(3)

	mintContract := models.AssetTriggerTXRequest{
		TriggerType: uint32(transaction.AssetTriggerContract_Mint),
		AssetID:     assetId,
		Receiver:    senderAddress,
		Amount:      numMinted,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		mintContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	// Create Marketplace

	marketplaceReferralAddress := processorNode.TestAddressPubkeyConverter.Encode(marketplaceReferral.Address)

	mktplaceContract := models.CreateMarketplaceTXRequest{
		Name:               "KleverMarketplace",
		ReferralAddress:    marketplaceReferralAddress,
		ReferralPercentage: 0,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateMarketplaceContractType,
		mktplaceContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	marketplaceId, err := integrationTest.GetMarketplaceId(tx.Receipts)
	require.NoError(t, err)

	// Sell Order

	sellOrderContract := models.SellTXRequest{
		MarketType:    int32(transaction.SellContract_BuyItNowMarket),
		MarketplaceID: marketplaceId,
		AssetID:       assetId + kapps.Sp + "1",
		CurrencyID:    string(kdautils.KLVIdentifier),
		Price:         1_200_000,
		ReservePrice:  1_000_00,
		EndTime:       time.Now().Add(24 * time.Hour).Unix(),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_SellContractType,
		sellOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	orderId, err := integrationTest.GetOrderID(tx.Receipts)
	require.NoError(t, err)

	// Buy Order

	buyOrderContract := models.BuyTXRequest{
		ID:         orderId,
		CurrencyID: string(kdautils.KLVIdentifier),
		Amount:     1_200_000,
		BuyType:    int32(transaction.BuyContract_MarketBuy),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[1],
		transaction.TXContract_BuyContractType,
		buyOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	receiverAccount := integrationTest.GetUserAccount(nodes[0], receiver.Address)
	require.NotNil(t, receiverAccount)

	nftBalance, _ := receiverAccount.GetUserKDA([]byte(assetId), []byte("1"), true)
	require.Equal(t, int64(1), nftBalance.Balance)

	estimatedTotalRoyalty := computeRoyalty(1_200_000, 100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTransaction_SplitRoyalties_NFT_MarketFixed_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)
	marketplaceReferral := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)
	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	receiver := wallets[1]

	// Create Spliy Royalties NFT

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:         uint32(kapps.KDAData_NonFungible),
		Name:         "Split Royalties",
		Ticker:       "SRT",
		OwnerAddress: senderAddress,
		MaxSupply:    10_000,
		Royalties: &models.RoyaltiesInfo{
			Address:        mainRoyaltyAddress,
			MarketFixed:    100,
			SplitRoyalties: make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketFixed: 100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketFixed: 200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)

	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	// Mint NFT

	numMinted := int64(3)

	mintContract := models.AssetTriggerTXRequest{
		TriggerType: uint32(transaction.AssetTriggerContract_Mint),
		AssetID:     assetId,
		Receiver:    senderAddress,
		Amount:      numMinted,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		mintContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	// Create Marketplace

	marketplaceReferralAddress := processorNode.TestAddressPubkeyConverter.Encode(marketplaceReferral.Address)

	mktplaceContract := models.CreateMarketplaceTXRequest{
		Name:               "KleverMarketplace",
		ReferralAddress:    marketplaceReferralAddress,
		ReferralPercentage: 0,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateMarketplaceContractType,
		mktplaceContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	marketplaceId, err := integrationTest.GetMarketplaceId(tx.Receipts)
	require.NoError(t, err)

	// Sell Order

	sellOrderContract := models.SellTXRequest{
		MarketType:    int32(transaction.SellContract_BuyItNowMarket),
		MarketplaceID: marketplaceId,
		AssetID:       assetId + kapps.Sp + "1",
		CurrencyID:    string(kdautils.KLVIdentifier),
		Price:         1_200_000,
		ReservePrice:  1_000_00,
		EndTime:       time.Now().Add(24 * time.Hour).Unix(),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_SellContractType,
		sellOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	orderId, err := integrationTest.GetOrderID(tx.Receipts)
	require.NoError(t, err)

	// Buy Order

	buyOrderContract := models.BuyTXRequest{
		ID:         orderId,
		CurrencyID: string(kdautils.KLVIdentifier),
		Amount:     1_200_000,
		BuyType:    int32(transaction.BuyContract_MarketBuy),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[1],
		transaction.TXContract_BuyContractType,
		buyOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	receiverAccount := integrationTest.GetUserAccount(nodes[0], receiver.Address)
	require.NotNil(t, receiverAccount)

	nftBalance, _ := receiverAccount.GetUserKDA([]byte(assetId), []byte("1"), true)
	require.Equal(t, int64(1), nftBalance.Balance)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTransaction_SplitRoyalties_NFT_MarketFixedAndPercentage_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0
	numWallets := 2

	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	// Setup New Addresses

	split1 := commonTxTest.CreateAndMintAccount(0, nodes)
	split2 := commonTxTest.CreateAndMintAccount(0, nodes)
	mainRoyalty := commonTxTest.CreateAndMintAccount(0, nodes)
	marketplaceReferral := commonTxTest.CreateAndMintAccount(0, nodes)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	sender := wallets[0]
	senderAddress := processorNode.TestAddressPubkeyConverter.Encode(sender.Address)
	mainRoyaltyAddress := processorNode.TestAddressPubkeyConverter.Encode(mainRoyalty.Address)

	receiver := wallets[1]

	// Create Spliy Royalties NFT

	splitRoyaltiesContract := models.CreateAssetTXRequest{
		Type:         uint32(kapps.KDAData_NonFungible),
		Name:         "Split Royalties",
		Ticker:       "SRT",
		OwnerAddress: senderAddress,
		MaxSupply:    10_000,
		Royalties: &models.RoyaltiesInfo{
			Address:        mainRoyaltyAddress,
			MarketFixed:    100,
			SplitRoyalties: make(map[string]*models.RoyaltySplitInfo),
		},
		Properties: &models.PropertiesInfo{
			CanMint: true,
		},
	}

	firstSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split1.Address)
	secondSplitedAddress := processorNode.TestAddressPubkeyConverter.Encode(split2.Address)

	splitRoyaltiesContract.Royalties.SplitRoyalties[firstSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketPercentage: 100,
		PercentMarketFixed:      100,
	}

	splitRoyaltiesContract.Royalties.SplitRoyalties[secondSplitedAddress] = &models.RoyaltySplitInfo{
		PercentMarketPercentage: 200,
		PercentMarketFixed:      200,
	}

	_, txHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		splitRoyaltiesContract,
	)

	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Get AssetID
	tx, err := commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)

	// Mint NFT

	numMinted := int64(3)

	mintContract := models.AssetTriggerTXRequest{
		TriggerType: uint32(transaction.AssetTriggerContract_Mint),
		AssetID:     assetId,
		Receiver:    senderAddress,
		Amount:      numMinted,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_AssetTriggerContractType,
		mintContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	// Create Marketplace

	marketplaceReferralAddress := processorNode.TestAddressPubkeyConverter.Encode(marketplaceReferral.Address)

	mktplaceContract := models.CreateMarketplaceTXRequest{
		Name:               "KleverMarketplace",
		ReferralAddress:    marketplaceReferralAddress,
		ReferralPercentage: 0,
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateMarketplaceContractType,
		mktplaceContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	marketplaceId, err := integrationTest.GetMarketplaceId(tx.Receipts)
	require.NoError(t, err)

	// Sell Order

	sellOrderContract := models.SellTXRequest{
		MarketType:    int32(transaction.SellContract_BuyItNowMarket),
		MarketplaceID: marketplaceId,
		AssetID:       assetId + kapps.Sp + "1",
		CurrencyID:    string(kdautils.KLVIdentifier),
		Price:         1_200_000,
		ReservePrice:  1_000_00,
		EndTime:       time.Now().Add(24 * time.Hour).Unix(),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_SellContractType,
		sellOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	slot, nonce, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	tx, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	orderId, err := integrationTest.GetOrderID(tx.Receipts)
	require.NoError(t, err)

	// Buy Order

	buyOrderContract := models.BuyTXRequest{
		ID:         orderId,
		CurrencyID: string(kdautils.KLVIdentifier),
		Amount:     1_200_000,
		BuyType:    int32(transaction.BuyContract_MarketBuy),
	}

	_, txHash, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[1],
		transaction.TXContract_BuyContractType,
		buyOrderContract,
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, _, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(nodes[0], txHash)
	require.Nil(t, err)

	receiverAccount := integrationTest.GetUserAccount(nodes[0], receiver.Address)
	require.NotNil(t, receiverAccount)

	nftBalance, _ := receiverAccount.GetUserKDA([]byte(assetId), []byte("1"), true)
	require.Equal(t, int64(1), nftBalance.Balance)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	mainRoyaltyAccount := integrationTest.GetUserAccount(nodes[0], mainRoyalty.Address)
	require.NotNil(t, mainRoyaltyAccount)
	splitAccount1 := integrationTest.GetUserAccount(nodes[0], split1.Address)
	require.NotNil(t, splitAccount1)
	splitAccount2 := integrationTest.GetUserAccount(nodes[0], split2.Address)
	require.NotNil(t, splitAccount2)

	require.Equal(t, estimatedTotalRoyalty-estimatedTotalSplitRoyalty, mainRoyaltyAccount.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 100), splitAccount1.GetBalance(kdautils.KLVIdentifier, true))
	require.Equal(t, computeRoyalty(estimatedTotalRoyalty, 200), splitAccount2.GetBalance(kdautils.KLVIdentifier, true))
}
