package blockchain

import (
	"math"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func CreateValidator(fromAddr, blsKey, ownerAddr, rewardAddr, logo string, commission, maxDelegation float64, canDelegate bool, uris map[string]string, name string) (string, error) {
	parsedCommission := commission * math.Pow10(2)
	parsedMaxDelegation := maxDelegation * math.Pow10(6)

	data, err := buildRequest(transaction.TXContract_CreateValidatorContractType, fromAddr, []interface{}{models.CreateValidatorTXRequest{
		BLSPublicKey:        blsKey,
		OwnerAddress:        ownerAddr,
		RewardAddress:       rewardAddr,
		Commission:          uint32(parsedCommission),
		CanDelegate:         canDelegate,
		MaxDelegationAmount: int64(parsedMaxDelegation),
		Logo:                logo,
		Name:                name,
		URIs:                uris,
	}})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func ValidatorConfig(fromAddr, blsKey, rewardAddr, logo string, commission, maxDelegation float64, canDelegate bool, uris map[string]string, name string) (string, error) {
	parsedCommission := commission * math.Pow10(2)
	parsedMaxDelegation := maxDelegation * math.Pow10(6)

	data, err := buildRequest(transaction.TXContract_ValidatorConfigContractType, fromAddr, []interface{}{models.ValidatorConfigTXRequest{
		BLSPublicKey:        blsKey,
		RewardAddress:       rewardAddr,
		CanDelegate:         canDelegate,
		Commission:          uint32(parsedCommission),
		MaxDelegationAmount: int64(parsedMaxDelegation),
		Logo:                logo,
		Name:                name,
		URIs:                uris,
	}})

	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
