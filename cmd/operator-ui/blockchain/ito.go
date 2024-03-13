package blockchain

import (
	"math"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func ConfigITO(fromAddr string, maxAmount float64, configITO models.ConfigITOTXRequest) (string, error) {
	kda, err := GetAssetData(configITO.KDA)
	if err != nil {
		return "", err
	}

	configITO.MaxAmount = int64(maxAmount * math.Pow10(int(kda.Precision)))
	data, err := buildRequest(transaction.TXContract_ConfigITOContractType, fromAddr, []interface{}{configITO})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func ITOSetPrices(fromAddr string, setPrices models.SetITOPricesTXRequest) (string, error) {
	data, err := buildRequest(transaction.TXContract_ITOTriggerContractType, fromAddr, []interface{}{setPrices})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func ITOTrigger(fromAddr string, maxAmount float64, itoTrigger models.ITOTriggerTXRequest) (string, error) {
	kda, err := GetAssetData(itoTrigger.KDA)
	if err != nil {
		return "", err
	}

	itoTrigger.MaxAmount = int64(maxAmount * math.Pow10(int(kda.Precision)))

	data, err := buildRequest(transaction.TXContract_ITOTriggerContractType, fromAddr, []interface{}{itoTrigger})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
