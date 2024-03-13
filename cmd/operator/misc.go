package main

import (
	"math"

	"github.com/klever-io/klever-go/kapps"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func setAccountName(fromAddr, name string) error {
	accountName := models.SetAccountNameTXRequest{
		Name: name,
	}

	data, err := buildRequest(transaction.TXContract_SetAccountNameContractType, fromAddr, []interface{}{accountName})
	if err != nil {
		return err
	}

	log.Info("setting account name", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func proposal(fromAddr, description string, parameters map[int32]string, duration uint32) error {
	proposal := models.ProposalTXRequest{
		Parameters:     parameters,
		EpochsDuration: duration,
		Description:    description,
	}

	data, err := buildRequest(transaction.TXContract_ProposalContractType, fromAddr, []interface{}{proposal})
	if err != nil {
		return err
	}

	log.Info("creating proposal", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func vote(fromAddr string, proposalID uint64, amount float64, voteType uint64) error {
	vote := models.VoteTXRequest{
		Type:       uint32(voteType),
		ProposalID: proposalID,
		Amount:     int64(amount * 1000000),
	}

	data, err := buildRequest(transaction.TXContract_VoteContractType, fromAddr, []interface{}{vote})
	if err != nil {
		return err
	}

	log.Info("setting vote", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func configITO(fromAddr string, configITO models.ConfigITOTXRequest) error {
	data, err := buildRequest(transaction.TXContract_ConfigITOContractType, fromAddr, []interface{}{configITO})
	if err != nil {
		return err
	}

	log.Info("setting config ITO", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func setITOPrices(fromAddr, kdaID string, packs []ParsedPack, message string) error {
	kda, err := getAssetData(kdaID)
	if err != nil {
		return err
	}

	packInfo := make(map[string]models.PackInfoRequest)
	for _, p := range packs {
		packPrecision := uint32(6)
		if p.Kda != string(kdautils.KLVIdentifier) && p.Kda != string(kdautils.KFIIdentifier) {
			packKDA, err := getAssetData(p.Kda)
			if err != nil {
				return err
			}

			packPrecision = packKDA.Precision
		}

		packItems := make([]models.PackItemRequest, 0)
		for _, pItem := range p.Packs {
			parsedItemAmount := pItem.Amount * math.Pow10(int(kda.Precision))
			parsedItemPrice := pItem.Price * math.Pow10(int(packPrecision))

			packItems = append(packItems, models.PackItemRequest{Amount: int64(parsedItemAmount), Price: int64(parsedItemPrice)})
		}

		packInfo[p.Kda] = models.PackInfoRequest{Packs: packItems}
	}

	setITOPrices := models.SetITOPricesTXRequest{
		KDA:      kdaID,
		PackInfo: packInfo,
	}

	data, err := buildRequest(transaction.TXContract_SetITOPricesContractType, fromAddr, []interface{}{setITOPrices})
	if err != nil {
		return err
	}

	log.Info("setting ITO prices", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func newITOTrigger(fromAddr string, itoTrigger models.ITOTriggerTXRequest) error {
	data, err := buildRequest(transaction.TXContract_ITOTriggerContractType, fromAddr, []interface{}{itoTrigger})
	if err != nil {
		return err
	}

	log.Info("requesting asset trigger", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func buy(fromAddr, id, currency string, amount float64, buyType int32, message string) error {
	parsedAmount := amount

	if buyType == 0 {
		kda, err := getAssetData(id)
		if err != nil {
			return err
		}

		if kda.AssetType != kapps.KDAData_NonFungible {
			parsedAmount = amount * math.Pow10(int(kda.Precision))
		}
	} else {
		kda, err := getAssetData(currency)
		if err != nil {
			return err
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
		return err
	}

	log.Info("setting buy", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func sell(fromAddr, kdaID, currency, mktID string, price, reservePrice float64, endTime int64, mktType int32, message string) error {
	parsedPrecision := uint32(6)
	if len(currency) > 0 && currency != string(kdautils.KLVIdentifier) && currency != string(kdautils.KFIIdentifier) {
		kda, err := getAssetData(currency)
		if err != nil {
			return err
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
		return err
	}

	log.Info("setting sell", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func cancelMarket(fromAddr, orderID string, message string) error {
	cancelMarket := models.CancelMarketOrderTXRequest{
		OrderID: orderID,
	}

	data, err := buildRequest(transaction.TXContract_CancelMarketOrderContractType, fromAddr, []interface{}{cancelMarket})
	if err != nil {
		return err
	}

	log.Info("setting cancel market order", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func createMarketplace(fromAddr, name, referralAddr string, referralPercent float64, message string) error {
	parsedReferralPercent := referralPercent * math.Pow10(2)

	createMarketplace := models.CreateMarketplaceTXRequest{
		Name:               name,
		ReferralAddress:    referralAddr,
		ReferralPercentage: uint32(parsedReferralPercent),
	}

	data, err := buildRequest(transaction.TXContract_CreateMarketplaceContractType, fromAddr, []interface{}{createMarketplace})
	if err != nil {
		return err
	}

	log.Info("setting create marketplace", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func configMarketplace(fromAddr, id, name, referralAddr string, referralPercent float64, message string) error {
	parsedReferralPercent := referralPercent * math.Pow10(2)

	configMarketplace := models.ConfigMarketplaceTXRequest{
		MarketplaceID:      id,
		Name:               name,
		ReferralAddress:    referralAddr,
		ReferralPercentage: uint32(parsedReferralPercent),
	}

	data, err := buildRequest(transaction.TXContract_ConfigMarketplaceContractType, fromAddr, []interface{}{configMarketplace})
	if err != nil {
		return err
	}

	log.Info("setting config marketplace", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}
