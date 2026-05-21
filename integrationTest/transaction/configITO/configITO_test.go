package configito

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/require"
)

func TestTransaction_CreateConfigITO_ShouldWork(t *testing.T) {
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

	// create asset tx request
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

	// check if asset exists
	tx, err := commonTxTest.GetAndCheckTransaction(consensusNodes[1], createAssetTxHash)
	require.Nil(t, err)

	assetIdentifierGenerated, err := integrationTest.GetAssetId(tx.Receipts)
	require.Nil(t, err)

	asset, err := consensusNodes[1].GetAsset([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, asset)

	// create config ITO request
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

	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		configITOContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	_, _, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// check if ITO exists
	ito, err := consensusNodes[1].GetITO([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, ito)
}

func TestTransaction_CreateConfigITO_ShouldError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

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

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	ownerWallet := wallets[0]
	whitelistAddress1 := processorNode.TestAddressPubkeyConverter.Encode(wallets[3].Address)
	whitelistAddress2 := processorNode.TestAddressPubkeyConverter.Encode(wallets[4].Address)

	// ################### CREATE ASSET ###################
	_, createAssetTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_CreateAssetContractType,
		&models.CreateAssetTXRequest{
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
		},
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err := integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// check if asset exists
	tx, err := consensusNodes[xidProposerBlock].GetTransaction(createAssetTxHash, true)
	require.Nil(t, err)

	assetIdentifierGenerated, err := integrationTest.GetAssetId(tx.Receipts)
	require.Nil(t, err)

	asset, err := consensusNodes[xidProposerBlock].GetAsset([]byte(assetIdentifierGenerated))
	require.Nil(t, err)
	require.NotNil(t, asset)

	// Block timestamps in tests are deterministic: blockTs(s) = slot0Ts + s*slotDuration,
	// where slot0Ts is the proposer's slot-0 reference time (SlotManagerMock captures
	// time.Now() at node construction — not actual chain genesis, just the anchor the
	// proposer uses to compute block timestamps in block.go).
	// Pin the ITO window to slot-relative anchors so it doesn't race wall-clock — KAPP
	// validates ITO windows against ctx.Block().GetTimestamp(), not time.Now().
	//   configITOTxHash is processed at block(slot)   → ts = slot0Ts + slot*slotDuration
	//   buy txs are processed at block(slot+1)        → ts = slot0Ts + (slot+1)*slotDuration
	//   "out of window" buy at block(slot+2)          → ts = slot0Ts + (slot+2)*slotDuration
	// Required: blockTs(slot) < itoStart ≤ blockTs(slot+1) ≤ itoEnd < blockTs(slot+2).
	slotDuration := int64(nodes[xidProposerBlock].SlotManager.TimeDuration().Seconds())
	slot0Ts := nodes[xidProposerBlock].SlotManager.Timestamp().Unix()
	itoStart := slot0Ts + int64(slot+1)*slotDuration
	// Strictly less than blockTs(slot+2) regardless of slotDuration — the buy rejection
	// at ito.go:336-337 is `EndTime < block.Timestamp`, so equality at block(slot+2)
	// would leave the final "out of window" buy in-window if slotDuration ever became 1s.
	itoEnd := slot0Ts + int64(slot+2)*slotDuration - 1
	// Deliberately end-before-start values for the two malformed-time test cases.
	itoTimeInvertedStart := slot0Ts + int64(slot+2)*slotDuration
	itoTimeInvertedEnd := itoStart

	// ################### CONFIG ITO TESTS ###################

	// error: invalid asset
	_, configITOInvalidAssetTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    "INVALID",
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Nil(t, err)

	//error: invalid receiver address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        "INVALID",
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(99),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid max amount
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              -10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid pack amount
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: -1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid pack price
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: -1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: inexistent asset on a pack
	_, configITOInexistentAssetTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"INEXISTENT": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Nil(t, err)

	// error: empty pack
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid default limit per address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: -10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid whitelist status
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(5),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid whitelist address
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, "INVALID": {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid whitelist address limit
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: -1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid ito whitelist start and end time
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoTimeInvertedStart,
			WhitelistEndTime:       itoTimeInvertedEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Error(t, err)

	// error: invalid ito start and end time
	_, _, err = commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoTimeInvertedStart,
			EndTime:                itoTimeInvertedEnd,
		},
	)
	require.Error(t, err)

	// config ito should work
	_, configITOTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		ownerWallet,
		transaction.TXContract_ConfigITOContractType,
		&models.ConfigITOTXRequest{
			KDA:                    assetIdentifierGenerated,
			ReceiverAddress:        processorNode.TestAddressPubkeyConverter.Encode(ownerWallet.Address),
			Status:                 int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			MaxAmount:              10000,
			PackInfo:               map[string]models.PackInfoRequest{"KLV": {Packs: []models.PackItemRequest{{Amount: 1, Price: 1}}}},
			DefaultLimitPerAddress: 10,
			WhitelistStatus:        int32(*transaction.ConfigITOContract_ActiveITO.Enum()),
			WhitelistInfo:          map[string]models.WhitelistInfoRequest{whitelistAddress1: {Limit: 1}, whitelistAddress2: {Limit: 1}},
			WhitelistStartTime:     itoStart,
			WhitelistEndTime:       itoEnd,
			StartTime:              itoStart,
			EndTime:                itoEnd,
		},
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	slot, nonce, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[xidProposerBlock], configITOInvalidAssetTxHash)
	require.Error(t, err)
	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[xidProposerBlock], configITOInexistentAssetTxHash)
	require.Error(t, err)
	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[xidProposerBlock], configITOTxHash)
	require.NoError(t, err)

	// error: address not in whitelist
	buyerWalletNotInWhitelist := wallets[2]
	_, buyITOAddressNotInWhitelistTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		buyerWalletNotInWhitelist,
		transaction.TXContract_BuyContractType,
		&models.BuyTXRequest{
			BuyType:        int32(*transaction.BuyContract_ITOBuy.Enum()),
			ID:             assetIdentifierGenerated,
			CurrencyID:     string(kdautils.KLVIdentifier),
			Amount:         1,
			CurrencyAmount: 1,
		},
	)
	require.Nil(t, err)

	// address in whitelist but will reach limit in second tx
	buyerWalletInWhitelist1 := wallets[3]
	_, buyITOAddressInWhitelistTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		buyerWalletInWhitelist1,
		transaction.TXContract_BuyContractType,
		&models.BuyTXRequest{
			BuyType:        int32(*transaction.BuyContract_ITOBuy.Enum()),
			ID:             assetIdentifierGenerated,
			CurrencyID:     string(kdautils.KLVIdentifier),
			Amount:         1,
			CurrencyAmount: 1,
		},
	)
	require.Nil(t, err)

	// error: address reach limit
	_, buyITOAddressInWhitelistReachedLimitTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		buyerWalletInWhitelist1,
		transaction.TXContract_BuyContractType,
		&models.BuyTXRequest{
			BuyType:        int32(*transaction.BuyContract_ITOBuy.Enum()),
			ID:             assetIdentifierGenerated,
			CurrencyID:     string(kdautils.KLVIdentifier),
			Amount:         1,
			CurrencyAmount: 1,
		},
	)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	slot, nonce, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[1], buyITOAddressNotInWhitelistTxHash)
	require.NotNil(t, err)
	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[1], buyITOAddressInWhitelistTxHash)
	require.Nil(t, err)
	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[1], buyITOAddressInWhitelistReachedLimitTxHash)
	require.NotNil(t, err)

	// error: buy out of ito time
	buyerWalletInWhitelist2 := wallets[4]
	_, buyITOAddressOutofITOTimeTxHash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		buyerWalletInWhitelist2,
		transaction.TXContract_BuyContractType,
		&models.BuyTXRequest{
			BuyType:    int32(*transaction.BuyContract_ITOBuy.Enum()),
			ID:         assetIdentifierGenerated,
			CurrencyID: string(kdautils.KLVIdentifier),
			Amount:     1,
		},
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	_, _, consensusNodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	_, err = commonTxTest.GetAndCheckTransaction(consensusNodes[1], buyITOAddressOutofITOTimeTxHash)
	require.NotNil(t, err)
}
