package triggerito

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction_TriggerITO_ShouldError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0

	numWallets := 5
	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	ownerWallet := wallets[0]
	whitelistAddress := processorNode.TestAddressPubkeyConverter.Encode(wallets[4].Address)

	// ################### CREATE ASSET ###################
	createAssetContract := &models.CreateAssetTXRequest{
		Type:          uint32(*transaction.CreateAssetContract_NonFungible.Enum()),
		Name:          "NFTT",
		Ticker:        "NFTT",
		OwnerAddress:  processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
		InitialSupply: 0,
		MaxSupply:     10000,
		Royalties: &models.RoyaltiesInfo{
			MarketFixed:      1000,
			MarketPercentage: 100,
			TransferFixed:    1000,
		},
		Properties: &models.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	_, createAssetTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_CreateAssetContractType,
		createAssetContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err := integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// Check if asset exists
	tx, err := commonTxTest.GetAndCheckTransaction(consensusNodes[1], createAssetTxHash)
	require.Nil(t, err)

	assetIdentifierGenerated, err := integrationTest.GetAssetId(tx.Receipts)
	require.Nil(t, err)

	asset, err := consensusNodes[1].GetAsset([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, asset)

	// ################### CONFIG ITO ###################
	configITOContract := &models.ConfigITOTXRequest{
		KDA:                    assetIdentifierGenerated,
		ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
		Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
		MaxAmount:              10000,
		PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
		WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress: {Limit: 1}},
		WhitelistStartTime:     time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:       time.Now().AddDate(0, 0, 2).Unix(),
		StartTime:              time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:                time.Now().AddDate(0, 0, 2).Unix(),
	}
	require.Nil(t, err)

	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		configITOContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// Check if ITO exists
	ito, err := consensusNodes[1].GetITO([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, ito)

	var invalidHashes [][]byte

	// ################### TRIGGER ITO TESTS ###################

	// TRIGGER: SET ITO PRICES
	// error: invalid asset
	_, hashTriggerITOContractSetITOInvalidAsset, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         "INVALID",
			PackInfo:    map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
		},
	)
	require.NoError(t, err)

	invalidHashes = append(invalidHashes, hashTriggerITOContractSetITOInvalidAsset)

	// error: invalid pack price
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         assetIdentifierGenerated,
			PackInfo:    map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: -1}}}},
		},
	)
	require.Error(t, err)

	// error: invalid pack amount
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         assetIdentifierGenerated,
			PackInfo:    map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: -1}}}},
		},
	)
	require.Error(t, err)

	// error: empty pack price
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         assetIdentifierGenerated,
			PackInfo:    map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{}}},
		},
	)
	require.Error(t, err)

	// error: invalid pack asset
	_, hashTriggerITOContractSetITOInvalidPackAsset, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         assetIdentifierGenerated,
			PackInfo:    map[string]models.PackInfoRequest{"INVALID": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
		},
	)
	require.NoError(t, err)

	invalidHashes = append(invalidHashes, hashTriggerITOContractSetITOInvalidPackAsset)

	// TRIGGER: UPDATE STATUS
	// error: invalid status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateStatus),
			KDA:         assetIdentifierGenerated,
			Status:      99,
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE RECEIVER ADDRESS
	// error: invalid address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:     uint32(transaction.ITOTriggerContract_UpdateReceiverAddress),
			KDA:             assetIdentifierGenerated,
			ReceiverAddress: "INVALID",
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE MAX AMOUNT
	// error: invalid value
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateMaxAmount),
			KDA:         assetIdentifierGenerated,
			MaxAmount:   -1000,
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE DEFAULT LIMIT ADDRESS
	// error: invalid value
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:                    assetIdentifierGenerated,
			TriggerType:            uint32(transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress),
			DefaultLimitPerAddress: -1,
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE TIMES
	// error: start time greater than the end time
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateTimes),
			StartTime:   time.Now().Unix(),
			EndTime:     time.Now().Add(-4 * time.Hour).Unix(),
		},
	)
	require.Error(t, err)

	// TRIGGER: ADD TO WHITELIST
	// error: nil ito info
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:           assetIdentifierGenerated,
			TriggerType:   uint32(transaction.ITOTriggerContract_AddToWhitelist),
			WhitelistInfo: nil,
		},
	)
	require.Error(t, err)

	// error: invalid whitelist address limit
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_AddToWhitelist),
			WhitelistInfo: map[string]models.WhitelistInfoRequest{
				"klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap": {
					Limit: -1,
				},
			},
		},
	)
	require.Error(t, err)

	// error: invalid whitelist address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_AddToWhitelist),
			WhitelistInfo: map[string]models.WhitelistInfoRequest{
				"INVALID": {},
			},
		},
	)
	require.Error(t, err)

	// TRIGGER: REMOVE FROM WHITELIST

	// error: invalid whitelist address limit
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:           assetIdentifierGenerated,
			TriggerType:   uint32(transaction.ITOTriggerContract_RemoveFromWhitelist),
			WhitelistInfo: nil,
		},
	)
	require.Error(t, err)

	// error: invalid whitelist address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_RemoveFromWhitelist),
			WhitelistInfo: map[string]models.WhitelistInfoRequest{
				"INVALID": {},
			},
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE WHITELIST TIMES
	// error: start time greater than the end time
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:        uint32(transaction.ITOTriggerContract_UpdateWhitelistTimes),
			WhitelistStartTime: time.Now().Unix(),
			WhitelistEndTime:   time.Now().Add(-3 * time.Hour).Unix(),
			KDA:                assetIdentifierGenerated,
		},
	)
	require.Error(t, err)

	// TRIGGER: UPDATE WHITELIST STATUS

	// error: invalid default status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:     uint32(transaction.ITOTriggerContract_UpdateWhitelistStatus),
			WhitelistStatus: int32(transaction.ITOTriggerContract_DefaultITO),
			KDA:             assetIdentifierGenerated,
		},
	)
	require.Error(t, err)

	// error: invalid status - not exists
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateWhitelistStatus),
			Status:      int32(100),
			KDA:         assetIdentifierGenerated,
		},
	)
	require.Error(t, err)

	time.Sleep(500 * time.Millisecond)

	_, _, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	for _, invalidHash := range invalidHashes {
		tx, err := commonTxTest.GetAndCheckTransaction(consensusNodes[1], invalidHash)
		require.Error(t, err)
		require.Nil(t, tx)
	}
}

func TestTransaction_TriggerITO_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0

	numWallets := 5
	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	ownerWallet := wallets[0]
	whitelistAddress := processorNode.TestAddressPubkeyConverter.Encode(wallets[4].Address)

	// ################### CREATE ASSET ###################
	createAssetContract := &models.CreateAssetTXRequest{
		Type:          uint32(*transaction.CreateAssetContract_NonFungible.Enum()),
		Name:          "NFTT",
		Ticker:        "NFTT",
		OwnerAddress:  processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
		InitialSupply: 0,
		MaxSupply:     10000,
		Royalties: &models.RoyaltiesInfo{
			MarketFixed:      1000,
			MarketPercentage: 100,
			TransferFixed:    1000,
		},
		Properties: &models.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	_, createAssetTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_CreateAssetContractType,
		createAssetContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err := integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// Check if asset exists
	tx, err := commonTxTest.GetAndCheckTransaction(consensusNodes[1], createAssetTxHash)
	require.Nil(t, err)

	assetIdentifierGenerated, err := integrationTest.GetAssetId(tx.Receipts)
	require.Nil(t, err)

	asset, err := consensusNodes[1].GetAsset([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, asset)

	// ################### CONFIG ITO ###################
	configITOContract := &models.ConfigITOTXRequest{
		KDA:                    assetIdentifierGenerated,
		ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
		Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
		MaxAmount:              10000,
		PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistStatus:        int32(*transaction.ConfigITOContract_PausedITO.Enum()),
		WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress: {Limit: 1}},
		WhitelistStartTime:     time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:       time.Now().AddDate(0, 0, 2).Unix(),
		StartTime:              time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:                time.Now().AddDate(0, 0, 2).Unix(),
	}
	require.Nil(t, err)

	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		configITOContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// Check if ITO exists
	ito, err := consensusNodes[1].GetITO([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, ito)

	// trigger: set ito prices
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_SetITOPrices),
			KDA:         assetIdentifierGenerated,
			PackInfo:    map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 2}}}},
		},
	)
	require.Nil(t, err)

	// trigger: update ITO status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateStatus),
			KDA:         assetIdentifierGenerated,
			Status:      int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
		},
	)
	require.Nil(t, err)

	// trigger: update receiver address
	newReceiverAddress := wallets[3].Address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:     uint32(transaction.ITOTriggerContract_UpdateReceiverAddress),
			KDA:             assetIdentifierGenerated,
			ReceiverAddress: processorNode.TestAddressPubkeyConverter.Encode(newReceiverAddress),
		},
	)
	require.Nil(t, err)

	// trigger: update max amount
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateMaxAmount),
			KDA:         assetIdentifierGenerated,
			MaxAmount:   int64(5000),
		},
	)
	require.Nil(t, err)

	// trigger: update default limit address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:            uint32(transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress),
			KDA:                    assetIdentifierGenerated,
			DefaultLimitPerAddress: int64(5),
		},
	)
	require.Nil(t, err)

	// trigger: update ITO times
	newStartTime := time.Now().AddDate(0, 0, 2).Unix()
	newEndTime := time.Now().AddDate(0, 0, 3).Unix()
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType: uint32(transaction.ITOTriggerContract_UpdateTimes),
			KDA:         assetIdentifierGenerated,
			StartTime:   newStartTime,
			EndTime:     newEndTime,
		},
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	// trigger: add to whitelist
	newWhitelistAddress := "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"
	newWhitelistAddress2 := "klv10vcm82mw30v48rr272s3m6llcs0p09tpp76hztsgnja276nzq4cqw38hgz"
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_AddToWhitelist),
			WhitelistInfo: map[string]models.WhitelistInfoRequest{
				newWhitelistAddress: {
					Limit: 1,
				},
				newWhitelistAddress2: {
					Limit: 1,
				},
			},
		},
	)
	require.Nil(t, err)

	// trigger: remove from whitelist
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			KDA:         assetIdentifierGenerated,
			TriggerType: uint32(transaction.ITOTriggerContract_RemoveFromWhitelist),
			WhitelistInfo: map[string]models.WhitelistInfoRequest{
				newWhitelistAddress2: {
					Limit: 1,
				},
			},
		},
	)
	require.Nil(t, err)

	// trigger: update whitelist times
	newWhitelistStartTime := time.Now().AddDate(0, 0, 2).Unix()
	newWhitelistEndTime := time.Now().AddDate(0, 0, 3).Unix()
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:        uint32(transaction.ITOTriggerContract_UpdateWhitelistTimes),
			KDA:                assetIdentifierGenerated,
			WhitelistStartTime: newWhitelistStartTime,
			WhitelistEndTime:   newWhitelistEndTime,
		},
	)
	require.Nil(t, err)

	// trigger: update whitelist status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ITOTriggerContractType,
		&models.ITOTriggerTXRequest{
			TriggerType:     uint32(transaction.ITOTriggerContract_UpdateWhitelistStatus),
			KDA:             assetIdentifierGenerated,
			WhitelistStatus: int32(*transaction.ITOTriggerContract_PausedITO.Enum()),
		},
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	_, _, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	ito, err = consensusNodes[1].GetITO([]byte(assetIdentifierGenerated))
	require.Nil(t, err)

	// check: set ITO price
	assert.Equal(t, ito.PackData["KLV"].Packs[0].Price, int64(2))

	// check: update status
	assert.Equal(t, true, ito.IsActive)

	// check: update receiver address
	assert.Equal(t, newReceiverAddress, ito.ReceiverAddress)

	// check: update max amount
	assert.Equal(t, int64(5000), ito.MaxAmount)

	// check: update default limit address
	assert.Equal(t, int64(5), ito.DefaultLimitPerAddress)

	// check: update times
	assert.Equal(t, newStartTime, ito.StartTime)
	assert.Equal(t, newEndTime, ito.EndTime)

	// check: add to whitelist
	decodedWhitelistAddress, _ := processorNode.TestAddressPubkeyConverter.Decode(newWhitelistAddress)
	addressITOWhitelistInfo, err := consensusNodes[1].GetITOWhitelistByAddress([]byte(assetIdentifierGenerated), hex.EncodeToString(decodedWhitelistAddress))
	require.Nil(t, err)
	assert.NotNil(t, 1, addressITOWhitelistInfo.Limit)

	// check: remove from whitelist
	decodedWhitelistAddress2, _ := processorNode.TestAddressPubkeyConverter.Decode(newWhitelistAddress)
	addressITOWhitelistInfo2, err := consensusNodes[1].GetITOWhitelistByAddress([]byte(assetIdentifierGenerated), hex.EncodeToString(decodedWhitelistAddress2))
	require.Nil(t, err)
	assert.NotNil(t, 0, addressITOWhitelistInfo2.Limit)

	// check: update whitelist time
	assert.Equal(t, newWhitelistEndTime, ito.WhitelistEndTime)

	// check: update whitelist status
	assert.Equal(t, false, ito.IsWhitelistActive)
}
