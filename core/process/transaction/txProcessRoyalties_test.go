package transaction_test

import (
	"encoding/hex"

	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kapps"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var assetWithRoyaltiesIdentifier = []byte("TEST")
var splitAddress1, _ = addressConverter.Decode("klv1g5ys9yu6knlhs7khks8q4wpaxx78a59hrtnmqtkcjsq37nsw0d4s5lf6hm")
var splitAddress2, _ = addressConverter.Decode("klv1w3mxqga3usmszvucy8yeaz4n9v9mvrgk4898ytltn2ty02h09tlqwghth5")
var mainRoyaltyAddress, _ = addressConverter.Decode("klv1d9q8lp3vx6nt4u2ndvnhsesz4d5m5fk8ltfggaqu5axyzp0kwpkqsea3mr")
var sender, _ = addressConverter.Decode("klv1g5khe6ec5yhwqd773gghc55q0kvpanevgqzj8h5r5sj2eeke2heqnt5dfp")
var admin, _ = addressConverter.Decode("klv1mt8yw657z6nk9002pccmwql8w90k0ac6340cjqkvm9e7lu0z2wjqudt69s")
var receiver, _ = addressConverter.Decode("klv1u75daf827dd864ug63mh5xvntd06h9wjk7rau3sd2c7vktqegfjqe7kvnz")
var marketplaceReferralAddress, _ = addressConverter.Decode("klv1nrgrqs3psf4r7e7y440fzqckhk7sndp9mq3dyxqpg76e5djk203qvq0ft8")
var itoReferralAddress, _ = addressConverter.Decode("klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap")

func (c *Controller) CreateAssetWithSplitRoyalties(assetType kapps.KDAData_EnumAssetType, royaltiesInfo *kapps.RoyaltiesData) {
	assetKey := kdautils.ToKDAKey(assetWithRoyaltiesIdentifier, nil)

	asset := kapps.KDAData{
		ID:                assetWithRoyaltiesIdentifier,
		AssetType:         assetType,
		Name:              []byte("TEST"),
		Ticker:            assetWithRoyaltiesIdentifier,
		OwnerAddress:      OwnerAddress,
		InitialSupply:     0,
		CirculatingSupply: 0,
		MaxSupply:         1000,
		IssueDate:         time.Now().Unix(),
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Royalties: royaltiesInfo,
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

func (c *Controller) RunTransferTX(blk *block.Block, ownerAddress, receiverAddress, assetID []byte, amount int64, opts ...int64) []byte {
	opts = append(opts, make([]int64, 2-len(opts))...)
	transferContract := transaction.TransferContract{
		ToAddress:    receiverAddress,
		AssetID:      assetID,
		Amount:       amount,
		KDARoyalties: opts[0],
		KLVRoyalties: opts[1],
	}

	tx, _ := createTransactionMock(&transferContract, transaction.TXContract_TransferContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	return tx.Receipts[1].Data[1]
}

func (c *Controller) RunCreateMarketplaceTX(blk *block.Block, ownerAddress, referralAddress []byte, referralPercentage uint32) []byte {
	createMarketplaceContract := transaction.CreateMarketplaceContract{
		Name:               []byte("NewMarketplace"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: referralPercentage,
	}

	tx, _ := createTransactionMock(&createMarketplaceContract, transaction.TXContract_CreateMarketplaceContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	// return marketplaceId generated
	return tx.Receipts[1].Data[1]
}

func (c *Controller) RunMintTX(blk *block.Block, ownerAddress []byte, receiverAddress []byte, assetId []byte) {
	mintContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetId,
		ToAddress:   receiverAddress,
		Amount:      3,
	}

	tx, _ := createTransactionMock(&mintContract, transaction.TXContract_AssetTriggerContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)
}

func (c *Controller) RunSellOrderTX(blk *block.Block, ownerAddress []byte, marketType int32, marketplaceId []byte, assetId []byte, currencyId []byte, price, reservePrice, endTime int64) []byte {
	sellContract := transaction.SellContract{
		MarketType:    transaction.SellContract_EnumMarketType(marketType),
		MarketplaceID: marketplaceId,
		AssetID:       assetId,
		CurrencyID:    currencyId,
		Price:         price,
		ReservePrice:  reservePrice,
		EndTime:       endTime,
	}

	tx, _ := createTransactionMock(&sellContract, transaction.TXContract_SellContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	orderId := kdautils.ToMarketID(&commonMock.HasherMock{}, blk.GetRandSeed(), tx.GetSender(), tx.GetNonce(), 0, kdautils.MarketKeyLength)

	// return orderId generated
	return orderId
}

func (c *Controller) RunBuyOrderTX(blk *block.Block, ownerAddress []byte, buyType int32, orderId []byte, currencyId []byte, amount int64) string {
	buyContract := transaction.BuyContract{
		BuyType:    transaction.BuyContract_EnumBuyType(buyType),
		ID:         orderId,
		CurrencyID: currencyId,
		Amount:     amount,
	}

	tx, _ := createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	// return orderId generated
	return hex.EncodeToString(tx.Receipts[1].Data[1])
}

func (c *Controller) RunTriggerRoyaltiesTX(blk *block.Block, ownerAddress []byte, assetId []byte, royaltiesInfo *kapps.RoyaltiesData) string {
	royaltyInfo := make([]*transaction.RoyaltyInfo, 0)
	for _, r := range royaltiesInfo.TransferPercentage {
		royalty := &transaction.RoyaltyInfo{
			Amount:     r.Amount,
			Percentage: r.Percentage,
		}

		royaltyInfo = append(royaltyInfo, royalty)
	}

	splitRoyalties := make(map[string]*transaction.RoyaltySplitInfo, 0)
	for addr, info := range royaltiesInfo.SplitRoyalties {
		splitRoyalties[addr] = &transaction.RoyaltySplitInfo{
			PercentTransferPercentage: info.PercentTransferPercentage,
			PercentTransferFixed:      info.PercentTransferFixed,
			PercentMarketPercentage:   info.PercentMarketPercentage,
			PercentMarketFixed:        info.PercentMarketFixed,
			PercentITOPercentage:      info.PercentITOFixed,
			PercentITOFixed:           info.PercentITOFixed,
		}
	}

	triggerContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateRoyalties,
		AssetID:     assetId,
		Royalties: &transaction.RoyaltiesInfo{
			Address:            royaltiesInfo.Address,
			TransferFixed:      royaltiesInfo.TransferFixed,
			TransferPercentage: royaltyInfo,
			MarketPercentage:   royaltiesInfo.MarketPercentage,
			MarketFixed:        royaltiesInfo.MarketFixed,
			ITOFixed:           royaltiesInfo.ITOFixed,
			ITOPercentage:      royaltiesInfo.ITOPercentage,
			SplitRoyalties:     splitRoyalties,
		},
	}

	tx, _ := createTransactionMock(&triggerContract, transaction.TXContract_AssetTriggerContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	return hex.EncodeToString(tx.Receipts[1].Data[1])
}

func computeRoyalty(amount int64, totalPercentage uint32) int64 {
	return int64(float64(amount) * float64(totalPercentage) / float64(core.HundredPercent))
}

// ##################### CREATE ASSET - SPLIT ROYALTIES
func TestSplitRoyaltiesTxProcessor_CreateAsset_ShouldErr(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)

	// Create NFT split royalties - invalid market percent
	contract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: sender,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketPercentage: 100,
			SplitRoyalties:   make(map[string]*transaction.RoyaltySplitInfo),
		},
	}

	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &transaction.RoyaltySplitInfo{
		PercentTransferFixed: 1000000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, sender, 0)

	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Create NFT split royalties - invalid market fixed
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: sender,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketFixed:    100,
			SplitRoyalties: make(map[string]*transaction.RoyaltySplitInfo),
		},
	}

	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &transaction.RoyaltySplitInfo{
		PercentTransferFixed: 1000000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, sender, 0)

	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Create NFT split royalties - invalid address
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: sender,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketFixed:    100,
			SplitRoyalties: make(map[string]*transaction.RoyaltySplitInfo),
		},
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}

	contract.Royalties.SplitRoyalties["INVALID"] = &transaction.RoyaltySplitInfo{
		PercentMarketFixed: 100,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, sender, 0)

	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidSplitRoyaltiesAddr, err)

	// Create NFT split royalties - invalid total split value
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: sender,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketPercentage: 100,
			SplitRoyalties:   make(map[string]*transaction.RoyaltySplitInfo),
		},
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}

	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &transaction.RoyaltySplitInfo{
		PercentMarketPercentage: 5000,
	}
	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &transaction.RoyaltySplitInfo{
		PercentMarketPercentage: 5001,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, sender, 0)

	_, hash, err = c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestSplitRoyaltiesTxProcessor_CreateAssetWithSplitRoyalties_ShouldWork(t *testing.T) {
	c := NewController(t)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)

	contract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: sender,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketPercentage: 100,
			SplitRoyalties:   make(map[string]*transaction.RoyaltySplitInfo),
		},
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}

	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &transaction.RoyaltySplitInfo{
		PercentMarketPercentage: 100,
	}
	contract.Royalties.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &transaction.RoyaltySplitInfo{
		PercentMarketPercentage: 200,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, sender, 0)

	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = c.execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, nil, err)
}

// ##################### TRANSACTIONS - SPLIT ROYALTIES
func TestSplitRoyaltiesTxProcessor_FungibleAsset_TransferPercentage_TwoSplitAddresses_ShouldWork(t *testing.T) {
	amountToTransfer := int64(700)
	senderBalance := int64(1000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address: mainRoyaltyAddress,
		TransferPercentage: []*kapps.RoyaltyData{{
			Amount:     100,
			Percentage: 5000,
		}, {
			Amount:     500,
			Percentage: 3000,
		}},
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 2700,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 1700,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_Fungible, royaltiesInfo)
	c.AddUser(sender, senderBalance, assetWithRoyaltiesIdentifier)
	c.AddUser(receiver, 0, nil)

	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	royalty1 := computeRoyalty(royaltiesInfo.TransferPercentage[0].Amount, royaltiesInfo.TransferPercentage[0].Percentage)
	royalty2 := computeRoyalty(royaltiesInfo.TransferPercentage[1].Amount, royaltiesInfo.TransferPercentage[1].Percentage)
	royalty3 := computeRoyalty(amountToTransfer-royaltiesInfo.TransferPercentage[1].Amount-royaltiesInfo.TransferPercentage[0].Amount, royaltiesInfo.TransferPercentage[1].Percentage)

	calculatedKDARoyalties := royalty1 + royalty2 + royalty3
	c.RunTransferTX(blk, sender, receiver, assetWithRoyaltiesIdentifier, amountToTransfer, calculatedKDARoyalties)

	c.CheckBalance(sender, assetWithRoyaltiesIdentifier, senderBalance-amountToTransfer-calculatedKDARoyalties)

	c.CheckBalance(receiver, assetWithRoyaltiesIdentifier, amountToTransfer)

	split1 := computeRoyalty(calculatedKDARoyalties, royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)].PercentTransferPercentage)
	split2 := computeRoyalty(calculatedKDARoyalties, royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)].PercentTransferPercentage)

	//Check Split Royalty Addresses
	c.CheckBalance(splitAddress1, assetWithRoyaltiesIdentifier, split1)
	c.CheckBalance(splitAddress2, assetWithRoyaltiesIdentifier, split2)

	//Check Main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, assetWithRoyaltiesIdentifier, calculatedKDARoyalties-split1-split2)
}

func TestSplitRoyaltiesTxProcessor_FungibleAsset_TransferFixed_TwoSplitAddresses_ShouldWork(t *testing.T) {
	amountToTransfer := int64(100_000)
	senderBalance := int64(1_000_000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:        mainRoyaltyAddress,
		TransferFixed:  100,
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferFixed: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferFixed: 200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_Fungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000, assetWithRoyaltiesIdentifier)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 0, nil)

	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	//Sender transfer to receiver address
	c.RunTransferTX(blk, sender, receiver, assetWithRoyaltiesIdentifier, amountToTransfer, 0, 100)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	//Check receiver asset balance
	c.CheckBalance(receiver, assetWithRoyaltiesIdentifier, amountToTransfer)
	//Check sender asset balance
	c.CheckBalance(sender, assetWithRoyaltiesIdentifier, senderBalance-amountToTransfer)
	//Check sender KLV balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance-estimatedTotalRoyalty)

	//Check Main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, estimatedTotalRoyalty-estimatedTotalSplitRoyalty)

	//Check Split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 200))
}

func TestSplitRoyaltiesTxProcessor_FungibleAsset_TransferFixedAndPercentage_TwoSplitAddresses_ShouldWork(t *testing.T) {
	amountToTransfer := int64(100_000)
	senderBalance := int64(1_000_000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:            mainRoyaltyAddress,
		TransferFixed:      100,
		TransferPercentage: []*kapps.RoyaltyData{{Amount: 100, Percentage: 100}},
		SplitRoyalties:     make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferFixed:      100,
		PercentTransferPercentage: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferFixed:      200,
		PercentTransferPercentage: 200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_Fungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000, assetWithRoyaltiesIdentifier)
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 0, nil)

	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	estimatedFixedTotalRoyalty := int64(100)
	estimatedFixedTotalSplitRoyalty := computeRoyalty(estimatedFixedTotalRoyalty, 300)
	estimatedPercentageTotalRoyalty := computeRoyalty(amountToTransfer, 100)
	estimatedPercentageTotalSplitRoyalty := computeRoyalty(estimatedPercentageTotalRoyalty, 300)

	//Sender transfer to receiver address
	c.RunTransferTX(blk, sender, receiver, assetWithRoyaltiesIdentifier, amountToTransfer, estimatedPercentageTotalRoyalty, 100)

	//Check receiver asset balance
	c.CheckBalance(receiver, assetWithRoyaltiesIdentifier, amountToTransfer)
	//Check sender asset balance
	c.CheckBalance(sender, assetWithRoyaltiesIdentifier, senderBalance-(amountToTransfer+estimatedPercentageTotalRoyalty))
	//Check sender KLV balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance-estimatedFixedTotalRoyalty)

	//Check Main Royalty Address KLV balance
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, estimatedFixedTotalRoyalty-estimatedFixedTotalSplitRoyalty)

	//Check Main Royalty Address asset balance
	c.CheckBalance(mainRoyaltyAddress, assetWithRoyaltiesIdentifier, estimatedPercentageTotalRoyalty-estimatedPercentageTotalSplitRoyalty)

	//Check Split Royalty Addresses KLV balance
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(estimatedFixedTotalRoyalty, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(estimatedFixedTotalRoyalty, 200))

	//Check Split Royalty Addresses asset balance
	c.CheckBalance(splitAddress1, assetWithRoyaltiesIdentifier, computeRoyalty(estimatedPercentageTotalRoyalty, 100))
	c.CheckBalance(splitAddress2, assetWithRoyaltiesIdentifier, computeRoyalty(estimatedPercentageTotalRoyalty, 200))
}

func TestSplitRoyaltiesTxProcessor_NFTAsset_TransferFixed_TwoSplitAddresses_ShouldWork(t *testing.T) {
	senderBalance := int64(1_000_000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:        mainRoyaltyAddress,
		TransferFixed:  100,
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferFixed: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferFixed: 200,
	}

	c := NewController(t)
	//Create NFT with royalties
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_NonFungible, royaltiesInfo)

	//Create Wallets
	c.AddUser(sender, 1_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 0, nil)
	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)

	// Mint NFT
	blk := c.CreateBlockHeader(0, 1, 1)
	c.RunMintTX(blk, sender, sender, assetWithRoyaltiesIdentifier)

	nftKey := []byte(string(assetWithRoyaltiesIdentifier) + kapps.Sp + "1")
	//Sender transfer to receiver address
	c.RunTransferTX(blk, sender, receiver, nftKey, 1, 0, 100)

	estimatedTotalRoyalty := int64(100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	//Check receiver asset balance
	c.CheckBalance(receiver, nftKey, 1)
	//Check sender asset balance
	c.CheckBalance(sender, nftKey, 0)
	//Check sender KLV balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance-estimatedTotalRoyalty)

	//Check Main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, estimatedTotalRoyalty-estimatedTotalSplitRoyalty)

	//Check Split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 200))
}

func TestSplitRoyaltiesTxProcessor_NFTAsset_MarketPercentage_TwoSplitAddresses_ShouldWork(t *testing.T) {
	senderBalance := int64(1_000_000_000)
	orderPrice := int64(1_200_000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:          mainRoyaltyAddress,
		MarketPercentage: 100,
		SplitRoyalties:   make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_NonFungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)
	c.AddUser(marketplaceReferralAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	// Mint NFT
	c.RunMintTX(blk, sender, sender, assetWithRoyaltiesIdentifier)

	// Create marketplace
	marketplaceId := c.RunCreateMarketplaceTX(blk, sender, marketplaceReferralAddress, 0)

	// Create sell order
	nftKey1 := []byte(string(assetWithRoyaltiesIdentifier) + kapps.Sp + "1")
	orderId := c.RunSellOrderTX(blk, sender, 0, marketplaceId, nftKey1, kdautils.KLVIdentifier, 1_200_000, 1_000_000, time.Now().AddDate(0, 0, 1).Unix())

	// Create buy order
	c.RunBuyOrderTX(blk, receiver, 1, orderId, kdautils.KLVIdentifier, orderPrice)

	estimatedTotalRoyalty := computeRoyalty(orderPrice, 100)
	estimatedTotalSplitRoyalty := computeRoyalty(estimatedTotalRoyalty, 300)

	// Check Seller Balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance+(orderPrice-estimatedTotalRoyalty))

	// Check Main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, estimatedTotalRoyalty-estimatedTotalSplitRoyalty)

	// Check Split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(estimatedTotalRoyalty, 200))
}

func TestSplitRoyaltiesTxProcessor_NFTAsset_MarketFixed_TwoSplitAddresses_ShouldWork(t *testing.T) {
	senderBalance := int64(1_000_000_000)
	orderPrice := int64(1_200_000)
	fixedRoyaltyAmount := int64(100)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:        mainRoyaltyAddress,
		MarketFixed:    100,
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentMarketFixed: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentMarketFixed: 200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_NonFungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)
	c.AddUser(marketplaceReferralAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	// Mint NFT
	c.RunMintTX(blk, sender, sender, assetWithRoyaltiesIdentifier)

	// Create marketplace
	marketplaceId := c.RunCreateMarketplaceTX(blk, sender, marketplaceReferralAddress, 0)

	// Create sell order
	nftKey1 := []byte(string(assetWithRoyaltiesIdentifier) + kapps.Sp + "1")
	orderId := c.RunSellOrderTX(blk, sender, 0, marketplaceId, nftKey1, kdautils.KLVIdentifier, 1_200_000, 1_000_000, time.Now().AddDate(0, 0, 1).Unix())

	// Create buy order
	c.RunBuyOrderTX(blk, receiver, 1, orderId, kdautils.KLVIdentifier, orderPrice)

	estimatedTotalSplitRoyalty := computeRoyalty(fixedRoyaltyAmount, 300)

	// Check seller Balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance+(orderPrice-fixedRoyaltyAmount))

	// Check main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, fixedRoyaltyAmount-estimatedTotalSplitRoyalty)

	// Check split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(fixedRoyaltyAmount, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(fixedRoyaltyAmount, 200))
}

func TestSplitRoyaltiesTxProcessor_NFTAsset_MarketFixed_TwoSplitAddressesWithMarketPercentage_ShouldWorkWithZeroRoyaltiesGerenatedForSplitAddresses(t *testing.T) {
	senderBalance := int64(1_000_000_000)
	orderPrice := int64(1_200_000)
	fixedRoyaltyAmount := int64(100)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:        mainRoyaltyAddress,
		MarketFixed:    100,
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_NonFungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)
	c.AddUser(marketplaceReferralAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	// Mint NFT
	c.RunMintTX(blk, sender, sender, assetWithRoyaltiesIdentifier)

	// Create marketplace
	marketplaceId := c.RunCreateMarketplaceTX(blk, sender, marketplaceReferralAddress, 0)

	// Create sell order
	nftKey1 := []byte(string(assetWithRoyaltiesIdentifier) + kapps.Sp + "1")
	orderId := c.RunSellOrderTX(blk, sender, 0, marketplaceId, nftKey1, kdautils.KLVIdentifier, 1_200_000, 1_000_000, time.Now().AddDate(0, 0, 1).Unix())

	// Create buy order
	c.RunBuyOrderTX(blk, receiver, 1, orderId, kdautils.KLVIdentifier, orderPrice)

	// Check seller Balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance+(orderPrice-fixedRoyaltyAmount))

	// Check main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, fixedRoyaltyAmount)

	// Check split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, 0)
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, 0)
}

func TestSplitRoyaltiesTxProcessor_NFTAsset_MarketFixed_TwoSplitAddressesWithMarketPercentageAndMarketFixed_ShouldWorkWithOnlyMarketFixedRoyaltiesGerenatedForSplitAddresses(t *testing.T) {
	senderBalance := int64(1_000_000_000)
	orderPrice := int64(1_200_000)
	fixedRoyaltyAmount := int64(100)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address:        mainRoyaltyAddress,
		MarketFixed:    100,
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 100,
		PercentMarketFixed:      100,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentMarketPercentage: 200,
		PercentMarketFixed:      200,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_NonFungible, royaltiesInfo)
	c.AddUser(sender, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(receiver, 1_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(splitAddress1, 0, nil)
	c.AddUser(splitAddress2, 0, nil)
	c.AddUser(mainRoyaltyAddress, 0, nil)
	c.AddUser(marketplaceReferralAddress, 0, nil)

	blk := c.CreateBlockHeader(0, 1, 1)

	// Mint NFT
	c.RunMintTX(blk, sender, sender, assetWithRoyaltiesIdentifier)

	// Create marketplace
	marketplaceId := c.RunCreateMarketplaceTX(blk, sender, marketplaceReferralAddress, 0)

	// Create sell order
	nftKey1 := []byte(string(assetWithRoyaltiesIdentifier) + kapps.Sp + "1")
	orderId := c.RunSellOrderTX(blk, sender, 0, marketplaceId, nftKey1, kdautils.KLVIdentifier, 1_200_000, 1_000_000, time.Now().AddDate(0, 0, 1).Unix())

	// Create buy order
	c.RunBuyOrderTX(blk, receiver, 1, orderId, kdautils.KLVIdentifier, orderPrice)

	estimatedTotalSplitRoyalty := computeRoyalty(fixedRoyaltyAmount, 300)

	// Check seller Balance
	c.CheckBalance(sender, kdautils.KLVIdentifier, senderBalance+(orderPrice-fixedRoyaltyAmount))

	// Check main Royalty Address
	c.CheckBalance(mainRoyaltyAddress, kdautils.KLVIdentifier, fixedRoyaltyAmount-estimatedTotalSplitRoyalty)

	// Check split Royalty Addresses
	c.CheckBalance(splitAddress1, kdautils.KLVIdentifier, computeRoyalty(fixedRoyaltyAmount, 100))
	c.CheckBalance(splitAddress2, kdautils.KLVIdentifier, computeRoyalty(fixedRoyaltyAmount, 200))
}

func TestSplitRoyaltiesTxProcessor_TriggerUpdateSplitRoyalties(t *testing.T) {
	senderBalance := int64(1000)

	percentage1 := uint32(5000)
	percentage2 := uint32(3000)

	royaltiesInfo := &kapps.RoyaltiesData{
		Address: mainRoyaltyAddress,
		TransferPercentage: []*kapps.RoyaltyData{{
			Amount:     100,
			Percentage: percentage1,
		}, {
			Amount:     500,
			Percentage: percentage2,
		}},
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 2700,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 1700,
	}

	c := NewController(t)
	c.CreateAssetWithSplitRoyalties(kapps.KDAData_Fungible, royaltiesInfo)
	c.AddUser(mainRoyaltyAddress, 0, nil)
	c.AddUser(OwnerAddress, 0, nil)
	c.AddUser(sender, senderBalance, assetWithRoyaltiesIdentifier)
	c.AddUser(receiver, 0, nil)

	newRoyaltiesInfo := &kapps.RoyaltiesData{
		Address: mainRoyaltyAddress,
		TransferPercentage: []*kapps.RoyaltyData{{
			Amount:     100,
			Percentage: percentage2,
		}, {
			Amount:     500,
			Percentage: percentage1,
		}},
		SplitRoyalties: make(map[string]*kapps.RoyaltySplitData),
	}
	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress1)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 2700,
	}

	royaltiesInfo.SplitRoyalties[hex.EncodeToString(splitAddress2)] = &kapps.RoyaltySplitData{
		PercentTransferPercentage: 1700,
	}

	blk := c.CreateBlockHeader(0, 1, 1)

	c.RunTriggerRoyaltiesTX(blk, OwnerAddress, assetWithRoyaltiesIdentifier, newRoyaltiesInfo)

	kda, err := c.GetAsset(assetWithRoyaltiesIdentifier)
	require.Nil(t, err)

	assert.Equal(t, kda.Royalties.TransferPercentage[0].Percentage, percentage2)
	assert.Equal(t, kda.Royalties.TransferPercentage[1].Percentage, percentage1)
}
