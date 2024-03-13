package blockchain

import (
	"errors"
	"math"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func Freeze(fromAddr, kda string, amount float64) (string, error) {
	precision := uint32(6)
	if len(kda) > 0 && kda != string(kdautils.KLVIdentifier) && kda != string(kdautils.KFIIdentifier) {
		kda, err := GetAssetData(kda)
		if err != nil {
			return "", err
		}

		precision = kda.Precision
	}

	parsedAmount := amount * math.Pow10(int(precision))

	data, err := buildRequest(transaction.TXContract_FreezeContractType, fromAddr, models.FreezeTXRequest{
		Amount: int64(parsedAmount),
		KDA:    kda,
	})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Unfreeze(fromAddr, bucketId, kda string, allbuckets bool) (string, error) {
	var unfreezes []struct {
		ContractType uint64 `json:"contractType"`
		models.UnfreezeTXRequest
	}

	if allbuckets {
		accountData, err := GetAccountData(fromAddr)
		if err != nil {
			return "", err
		}

		asset, ok := accountData.Assets[kda]
		if !ok {
			return "", errors.New("asset not found")
		}

		for _, bucket := range asset.Buckets {
			unfreezes = append(unfreezes, struct {
				ContractType uint64 `json:"contractType"`
				models.UnfreezeTXRequest
			}{
				ContractType: uint64(transaction.TXContract_UnfreezeContractType),
				UnfreezeTXRequest: models.UnfreezeTXRequest{
					BucketID: bucket.Id,
					KDA:      kda,
				},
			})
		}
	} else {
		unfreezes = append(unfreezes, struct {
			ContractType uint64 `json:"contractType"`
			models.UnfreezeTXRequest
		}{
			ContractType: uint64(transaction.TXContract_UnfreezeContractType),
			UnfreezeTXRequest: models.UnfreezeTXRequest{
				BucketID: bucketId,
				KDA:      kda,
			},
		})
	}

	data, err := buildRequest(transaction.TXContract_UnfreezeContractType, fromAddr, unfreezes)
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Delegate(fromAddr, toAddr, bucketId string) (string, error) {
	data, err := buildRequest(transaction.TXContract_DelegateContractType, fromAddr, models.DelegateTXRequest{
		Receiver: toAddr,
		BucketID: bucketId,
	})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Claim(fromAddr, id string, claimType int32) (string, error) {
	data, err := buildRequest(transaction.TXContract_ClaimContractType, fromAddr, models.ClaimTXRequest{
		ClaimType: claimType,
		ID:        id,
	})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Undelegate(fromAddr, bucketId string) (string, error) {
	data, err := buildRequest(transaction.TXContract_UndelegateContractType, fromAddr, models.UndelegateTXRequest{
		BucketID: bucketId,
	})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Withdraw(fromAddr, kda string, withdrawType int32) (string, error) {
	data, err := buildRequest(transaction.TXContract_WithdrawContractType, fromAddr, models.WithdrawTXRequest{
		KDA:          kda,
		WithdrawType: withdrawType,
	})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Unjail(fromAddr string) (string, error) {
	data, err := buildRequest(transaction.TXContract_UnjailContractType, fromAddr, models.UnjailTXRequest{})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func SetAccountName(fromAddr, name string) (string, error) {
	accountName := models.SetAccountNameTXRequest{
		Name: name,
	}

	data, err := buildRequest(transaction.TXContract_SetAccountNameContractType, fromAddr, []interface{}{accountName})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func SetPermission(fromAddr string, permissions []models.PermissionTXRequest) (string, error) {
	data, err := buildRequest(transaction.TXContract_UpdateAccountPermissionContractType, fromAddr, []interface{}{models.UpdateAccountPermissionTXRequest{
		Permissions: permissions,
	}})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
