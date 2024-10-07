package transaction_test

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var assetITOIdentifier = []byte("TEST")

func (c *Controller) CreateAssetForITO() {
	assetKey := kdautils.ToKDAKey(assetITOIdentifier, nil)

	asset := kapps.KDAData{
		ID:                assetITOIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("TEST"),
		Ticker:            assetITOIdentifier,
		OwnerAddress:      sender,
		AdminAddress:      admin,
		InitialSupply:     0,
		CirculatingSupply: 0,
		MaxSupply:         1000,
		IssueDate:         time.Now().Unix(),
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: false,
		},
		Roles: []*kapps.RolesData{{
			Address:     sender,
			HasRoleMint: true,
		}},
	}

	assetData, _ := marshalizer.Marshal(&asset)

	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(assetKey, assetData)

	_ = c.kappDB.SaveAccount(c.kdaKapp)
}

func (c *Controller) CreateAssetWithITORoyalties() {
	assetKey := kdautils.ToKDAKey(assetITOIdentifier, nil)

	asset := kapps.KDAData{
		ID:                assetITOIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("TEST"),
		Ticker:            assetITOIdentifier,
		OwnerAddress:      sender,
		AdminAddress:      admin,
		InitialSupply:     0,
		CirculatingSupply: 0,
		MaxSupply:         1000,
		IssueDate:         time.Now().Unix(),
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: false,
		},
		Roles: []*kapps.RolesData{{
			Address:     sender,
			HasRoleMint: true,
		}},
		Royalties: &kapps.RoyaltiesData{
			Address:        sender,
			ITOFixed:       10_000,
			ITOPercentage:  130,
			SplitRoyalties: map[string]*kapps.RoyaltySplitData{hex.EncodeToString(itoReferralAddress): {PercentITOPercentage: 500, PercentITOFixed: 3000}},
		},
	}

	assetData, _ := marshalizer.Marshal(&asset)

	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(assetKey, assetData)

	_ = c.kappDB.SaveAccount(c.kdaKapp)
}

func (c *Controller) RunConfigITOTX(blk *block.Block, ownerAddress, receiverAddress, assetID []byte, maxAmount int64, limitPerAddress int64) {

	configITOContract := transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        receiverAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              maxAmount,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: limitPerAddress,
		WhitelistStatus:        transaction.ConfigITOContract_DefaultITO,
		WhitelistInfo:          map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: 100}},
		WhitelistStartTime:     time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:       time.Now().AddDate(0, 0, 3).Unix(),
		StartTime:              time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:                time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ := createTransactionMock(&configITOContract, transaction.TXContract_ConfigITOContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)
}

func (c *Controller) RunTriggerITOTX(blk *block.Block, ownerAddress []byte, ITOTriggerContract *transaction.ITOTriggerContract) {
	tx, _ := createTransactionMock(ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)
}

func (c *Controller) RunBuyITOTX(ownerAddress []byte, amount int64, currencyAmount int64) {
	buyITOContract := transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         amount,
		CurrencyAmount: currencyAmount,
	}

	tx, _ := createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	buyITOblk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(buyITOblk, hash, tx)
	require.Nil(c.t, err)
}

// ##################### TRIGGER ITO TRANSACTIONS
func TestITOTriggerTxProcessor_SetITOPrices_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Invalid asset
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     []byte("INVALID"),
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//Invalid pack amount
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: -1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Empty pack price
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Inexistent pack asset
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"INEXISTENT": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//User without have permission
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrRoleNotFound, err)

	// Update pack amount with a larger number than the ito max supply
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1001, Price: 10}}}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_SetITOPrices_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 10}}}},
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Same SetITOPrices with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Buy ITO with other account
	c.RunBuyITOTX(receiver, 1, 10)
	//Check buyer's klv balance - (initial balanace - pack price)
	c.CheckBalance(receiver, kdautils.KLVIdentifier, 1_000_000-10)
	//Check buyer's asset balance
	c.CheckBalance(receiver, assetITOIdentifier, 1)
}

func TestITOTriggerTxProcessor_UpdateStatus_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateStatus,
		AssetID:     assetITOIdentifier,
		Status:      transaction.ITOTriggerContract_ActiveITO,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid status - default
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateStatus,
		AssetID:     assetITOIdentifier,
		Status:      transaction.ITOTriggerContract_DefaultITO,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid status - inexistent
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateStatus,
		AssetID:     assetITOIdentifier,
		Status:      11111,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrITOStatusInvalid, err)
}

func TestITOTriggerTxProcessor_UpdateStatus_ShouldWork(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateStatus,
		AssetID:     assetITOIdentifier,
		Status:      transaction.ITOTriggerContract_ActiveITO,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Same UpdateStatus with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Make ito buy after it's active
	c.RunBuyITOTX(receiver, 1, 1)

	//Check buyer's asset balance
	c.CheckBalance(receiver, assetITOIdentifier, 1)

	//Pausing ITO
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateStatus,
		AssetID:     assetITOIdentifier,
		Status:      transaction.ITOTriggerContract_PausedITO,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateStatus with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Make ITO buy after it's paused
	buyITOContract := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         assetITOIdentifier,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	buyITOblk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(buyITOblk, hash, tx)
	assert.Equal(t, common.ErrITONotActive, err)
}

func TestITOTriggerTxProcessor_UpdateReceiverAddress_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 1000)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
		AssetID:         assetITOIdentifier,
		ReceiverAddress: receiver,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid Address
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
		AssetID:         assetITOIdentifier,
		ReceiverAddress: []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)
}

func TestITOTriggerTxProcessor_UpdateReceiverAddress_ShouldWork(t *testing.T) {
	c := NewController(t)
	buyerAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksz")
	newReceiverAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksx")

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(buyerAddress, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Buy ITO
	c.RunBuyITOTX(buyerAddress, 1, 1)
	//Check receiver address balance (initial balance + pack price)
	c.CheckBalance(sender, kdautils.KLVIdentifier, 1_000_000+1)

	//Change ITO receiver address
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
		AssetID:         assetITOIdentifier,
		ReceiverAddress: newReceiverAddress,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateReceiverAddress with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Buy ITO
	c.RunBuyITOTX(buyerAddress, 1, 1)
	//Check receiver address balance (initial balance + pack price)
	c.CheckBalance(newReceiverAddress, kdautils.KLVIdentifier, 1)
	//Check old receiver address (same balance)
	c.CheckBalance(sender, kdautils.KLVIdentifier, 1_000_000+1)
}

func TestITOTriggerTxProcessor_UpdateMaxAmount_ShouldErr(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateMaxAmount,
		AssetID:     assetITOIdentifier,
		MaxAmount:   12,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid Value - negative
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateMaxAmount,
		AssetID:     assetITOIdentifier,
		MaxAmount:   -1,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid Value - Surpass minted amount

	//Run Buy ITO
	c.RunBuyITOTX(sender, 100, 100)

	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateMaxAmount,
		AssetID:     assetITOIdentifier,
		MaxAmount:   99,
	}

	// Block with a higher timestamp to start the ITO
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateMaxAmount_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 10, 10)

	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateMaxAmount,
		AssetID:     assetITOIdentifier,
		MaxAmount:   12,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateMaxAmount with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Try to buy new max amount
	buyITOContract := transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         12,
		CurrencyAmount: 12,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	buyITOblk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(buyITOblk, hash, tx)
	assert.Equal(t, nil, err)
}

func TestITOTriggerTxProcessor_UpdateDefaultLimitPerAddress_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:            transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress,
		AssetID:                assetITOIdentifier,
		DefaultLimitPerAddress: 10,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid Amount - negative
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:            transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress,
		AssetID:                assetITOIdentifier,
		DefaultLimitPerAddress: -10,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateDefaultLimitPerAddress_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Change limit per address
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:            transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress,
		AssetID:                assetITOIdentifier,
		DefaultLimitPerAddress: 12,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateDefaultLimitPerAddress with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Add address in whitelist
	c.RunTriggerITOTX(blk, sender, &transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {}},
	})

	//Active whitelist
	activateWhitelistBlk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	c.RunTriggerITOTX(activateWhitelistBlk, sender, &transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	})

	//Try to buy with default limit quantity
	buyITOContract := transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         12,
		CurrencyAmount: 12,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)

	//Try to buy a higher amount than the default limit
	buyITOContract = transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         13,
		CurrencyAmount: 13,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateTimes_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateTimes,
		AssetID:     assetITOIdentifier,
		StartTime:   time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:     time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid time - Start time passed
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateTimes,
		AssetID:     assetITOIdentifier,
		StartTime:   time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:     time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	blkWithHigherTimestamp := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(blkWithHigherTimestamp, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid time - Start time higher than endtime
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateTimes,
		AssetID:     assetITOIdentifier,
		StartTime:   time.Now().AddDate(0, 0, 2).Unix(),
		EndTime:     time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateTimes_ShouldWork(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_UpdateTimes,
		AssetID:     assetITOIdentifier,
		StartTime:   time.Now().AddDate(0, 0, 1).Unix(),
		EndTime:     time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateTimes with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)
}

func TestITOTriggerTxProcessor_AddToWhitelist_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: 100}},
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Empty whitelist info
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid whitelist address
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{"INVALID": {Limit: 100}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, process.ErrInvalidWhitelistAddr, err)

	//Invalid whitelist address limit
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: -10}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_AddToWhitelist_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Activate whitelist
	activateWhitelistBlk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	c.RunTriggerITOTX(activateWhitelistBlk, sender, &transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	})

	//Try to buy without being in the whitelist
	buyITOContract := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         assetITOIdentifier,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     12,
	}

	tx, _ := createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Add in whitelist
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {Limit: 100}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same AddToWhitelist with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Try to buy beign in the whitelist
	buyITOContract = transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         12,
		CurrencyAmount: 12,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)
}

func TestITOTriggerTxProcessor_RemoveFromWhitelist_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_RemoveFromWhitelist,
		AssetID:     assetITOIdentifier,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Empty whitelist info
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_RemoveFromWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid whitelist address
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_RemoveFromWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{"INVALID": {Limit: 100}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, process.ErrInvalidWhitelistAddr, err)
}

func TestITOTriggerTxProcessor_RemoveFromWhitelist_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Add address in whitelist
	c.RunTriggerITOTX(blk, sender, &transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {}},
	})

	//Activate whitelist
	activateWhitelistBlk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	c.RunTriggerITOTX(activateWhitelistBlk, sender, &transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	})

	//Buy ITO
	buyITOContract := transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         1,
		CurrencyAmount: 1,
	}

	tx, _ := createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)

	//Remove from whitelist
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_RemoveFromWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {Limit: 100}},
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same RemoveFromWhitelist with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, process.ErrInvalidWhitelistAddr, err)

	//Try to buy ITO
	buyITOContract = transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         1,
		CurrencyAmount: 1,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateWhitelistTimes_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 100)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:        transaction.ITOTriggerContract_UpdateWhitelistTimes,
		AssetID:            assetITOIdentifier,
		WhitelistStartTime: time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:   time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid whitelist time - Start time passed
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:        transaction.ITOTriggerContract_UpdateWhitelistTimes,
		AssetID:            assetITOIdentifier,
		WhitelistStartTime: time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:   time.Now().AddDate(0, 0, 3).Unix(),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	blkWithHigherTimestamp := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(blkWithHigherTimestamp, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid whitelist time - Start time higher than endtime
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:        transaction.ITOTriggerContract_UpdateWhitelistTimes,
		AssetID:            assetITOIdentifier,
		WhitelistStartTime: time.Now().AddDate(0, 0, 2).Unix(),
		WhitelistEndTime:   time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestITOTriggerTxProcessor_UpdateWhitelistTimes_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Add address in whitelist
	c.RunTriggerITOTX(blk, sender, &transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {}},
	})

	//Activate whitelist
	c.RunTriggerITOTX(blk, sender, &transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	})

	//Try to buy ITO - Active but out of time
	buyITOContract := transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         1,
		CurrencyAmount: 1,
	}

	tx, _ := createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Change ITO Time
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:        transaction.ITOTriggerContract_UpdateWhitelistTimes,
		AssetID:            assetITOIdentifier,
		WhitelistStartTime: time.Now().AddDate(0, 0, 3).Unix(),
		WhitelistEndTime:   time.Now().AddDate(0, 0, 4).Unix(),
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateWhitelistTimes with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Buy ITO in time
	buyITOContract = transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         1,
		CurrencyAmount: 1,
	}

	activateWhitelistBlk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 3).Unix(), 2, 2)
	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)
}

func TestITOTriggerTxProcessor_UpdateWhitelistStatus_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Not the asset owner
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Invalid whitelist status - default
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_DefaultITO,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Invalid whitelist status - inexistent
	ITOTriggerContract = transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: 11111,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrITOWhitelistStatusInvalid, err)
}

func TestITOTriggerTxProcessor_UpdateWhitelistStatus_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetForITO()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	//Add address in whitelist
	c.RunTriggerITOTX(blk, sender, &transaction.ITOTriggerContract{
		TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
		AssetID:       assetITOIdentifier,
		WhitelistInfo: map[string]*transaction.WhitelistInfo{hex.EncodeToString(receiver): {}},
	})

	//Try to buy ITO
	buyITOContract := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         assetITOIdentifier,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ := createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Activate ITO whitelist
	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType:     transaction.ITOTriggerContract_UpdateWhitelistStatus,
		AssetID:         assetITOIdentifier,
		WhitelistStatus: transaction.ITOTriggerContract_ActiveITO,
	}

	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	activateWhitelistBlk := c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)

	// Same UpdateWhitelistStatus with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	activateWhitelistBlk = c.CreateBlockHeader(time.Now().AddDate(0, 0, 2).Unix(), 2, 2)
	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)

	//Try to buy ITO
	buyITOContract = transaction.BuyContract{
		BuyType:        transaction.BuyContract_ITOBuy,
		ID:             assetITOIdentifier,
		CurrencyID:     kdautils.KLVIdentifier,
		Amount:         1,
		CurrencyAmount: 1,
	}

	tx, _ = createTransactionMock(&buyITOContract, transaction.TXContract_BuyContractType, receiver, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(activateWhitelistBlk, hash, tx)
	assert.Equal(t, nil, err)
}

func TestITOTriggerTxProcessor_BuyRoyalties_ShouldWork(t *testing.T) {
	c := NewController(t)

	c.AddUser(sender, 10_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(admin, 10_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()
	c.CreateAssetWithITORoyalties()
	c.RunConfigITOTX(blk, sender, sender, assetITOIdentifier, 1000, 10)

	ITOTriggerContract := transaction.ITOTriggerContract{
		TriggerType: transaction.ITOTriggerContract_SetITOPrices,
		AssetID:     assetITOIdentifier,
		PackInfo:    map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1_000_000_000}}}},
	}

	tx, _ := createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, sender, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	// Same SetITOPrices with asset admin address
	tx, _ = createTransactionMock(&ITOTriggerContract, transaction.TXContract_ITOTriggerContractType, admin, 0)
	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(t, nil, err)

	//Buy ITO with other account
	c.RunBuyITOTX(receiver, 1, 1000000000)
	//Check buyer's klv balance - (initial balanace - pack price - royalty fixed)
	c.CheckBalance(receiver, kdautils.KLVIdentifier, 10_000_000_000-1_000_000_000-10_000)
	//Check buyer's asset balance
	c.CheckBalance(receiver, assetITOIdentifier, 1)

	//Check sellers royalties klv balance - (initial balanace + 1 royalties fixed split + (pack price - total royalties percentage) + 1 royalties percentage split)
	c.CheckBalance(sender, kdautils.KLVIdentifier, 10_000_000_000+7_000+(1_000_000_000-13_000_000)+12_350_000)

	//Check ITO royalties receiver klv balance - (initial balanace + 2 royalties fixed split + 2 royalties percentage split)
	c.CheckBalance(itoReferralAddress, kdautils.KLVIdentifier, 0+3_000+650_000)
}
