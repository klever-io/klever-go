package blockchain

import (
	"math"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func CreateAsset(fromAddr string, createAsset models.CreateAssetTXRequest) (string, error) {
	data, err := buildRequest(transaction.TXContract_CreateAssetContractType, fromAddr, []interface{}{createAsset})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func TriggerAsset(fromAddr string, createAsset models.AssetTriggerTXRequest) (string, error) {
	data, err := buildRequest(transaction.TXContract_AssetTriggerContractType, fromAddr, []interface{}{createAsset})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Deposit(fromAddr string, amount float64, depositReq models.DepositTXRequest) (string, error) {
	v, err := GetAssetData(depositReq.CurrencyID)
	if err != nil {
		return "", err
	}

	depositReq.Amount = int64(amount * math.Pow10(int(v.Precision)))
	data, err := buildRequest(transaction.TXContract_DepositContractType, fromAddr, []interface{}{depositReq})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
