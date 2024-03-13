package main

import (
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func permission(fromAddr string, permissions []models.PermissionTXRequest) error {
	data, err := buildRequest(transaction.TXContract_UpdateAccountPermissionContractType, fromAddr, []interface{}{models.UpdateAccountPermissionTXRequest{
		Permissions: permissions,
	}})

	if err != nil {
		return err
	}
	_, err = sendSignAndBroadcast(data)
	return err
}
