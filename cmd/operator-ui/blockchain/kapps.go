package blockchain

import (
	"math"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
)

func CreateMarketplace(fromAddr, name, referralAddr string, referralPercent float64) (string, error) {
	parsedReferralPercent := referralPercent * math.Pow10(2)

	createMarketplace := models.CreateMarketplaceTXRequest{
		Name:               name,
		ReferralAddress:    referralAddr,
		ReferralPercentage: uint32(parsedReferralPercent),
	}

	data, err := buildRequest(transaction.TXContract_CreateMarketplaceContractType, fromAddr, []interface{}{createMarketplace})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func ConfigMarketplace(fromAddr, id, name, referralAddr string, referralPercent float64) (string, error) {
	parsedReferralPercent := referralPercent * math.Pow10(2)

	configMarketplace := models.ConfigMarketplaceTXRequest{
		MarketplaceID:      id,
		Name:               name,
		ReferralAddress:    referralAddr,
		ReferralPercentage: uint32(parsedReferralPercent),
	}

	data, err := buildRequest(transaction.TXContract_ConfigMarketplaceContractType, fromAddr, []interface{}{configMarketplace})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Buy(fromAddr, id, currency string, amount float64, buyType int32) (string, error) {
	parsedAmount := amount

	if buyType == 0 {
		kda, err := GetAssetData(id)
		if err != nil {
			return "", err
		}

		if kda.AssetType == kapps.KDAData_Fungible {
			parsedAmount = amount * math.Pow10(int(kda.Precision))
		}
	} else {
		kda, err := GetAssetData(currency)
		if err != nil {
			return "", err
		}

		parsedAmount = amount * math.Pow10(int(kda.Precision))
	}

	buy := models.BuyTXRequest{
		BuyType:    buyType,
		ID:         id,
		CurrencyID: currency,
		Amount:     int64(parsedAmount),
	}

	data, err := buildRequest(transaction.TXContract_BuyContractType, fromAddr, []interface{}{buy})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Sell(fromAddr, kdaID, currency, mktID string, price, reservePrice float64, endTime int64, mktType int32) (string, error) {
	parsedPrecision := uint32(6)
	if len(currency) > 0 && currency != string(kdautils.KLVIdentifier) && currency != string(kdautils.KFIIdentifier) {
		kda, err := GetAssetData(currency)
		if err != nil {
			return "", err
		}

		parsedPrecision = kda.Precision
	}

	parsedPrice := price * math.Pow10(int(parsedPrecision))
	parsedReservePrice := reservePrice * math.Pow10(int(parsedPrecision))

	sell := models.SellTXRequest{
		MarketType:    mktType,
		MarketplaceID: mktID,
		AssetID:       kdaID,
		CurrencyID:    currency,
		Price:         int64(parsedPrice),
		ReservePrice:  int64(parsedReservePrice),
		EndTime:       endTime,
	}

	data, err := buildRequest(transaction.TXContract_SellContractType, fromAddr, []interface{}{sell})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func CancelMarket(fromAddr, orderID string) (string, error) {
	cancelMarket := models.CancelMarketOrderTXRequest{
		OrderID: orderID,
	}

	data, err := buildRequest(transaction.TXContract_CancelMarketOrderContractType, fromAddr, []interface{}{cancelMarket})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
