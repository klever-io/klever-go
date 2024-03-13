package blockchain

import (
	"encoding/json"
	"errors"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func buildRequest(
	txType transaction.TXContract_ContractType,
	fromAddr string,
	contracts ...interface{},
) ([]byte, error) {

	var txNonce uint64
	if nonce, err := getAccountNonce(fromAddr); err == nil {
		txNonce = nonce
	}

	if len(contracts) == 0 {
		return nil, errors.New("need at least a contract")
	}

	if len(contracts) > 1 {
		return json.Marshal(&models.SendTXRequest{
			Type:      uint32(txType),
			Sender:    fromAddr,
			Nonce:     txNonce,
			PermID:    0,
			Contracts: contracts,
		})
	}

	return json.Marshal(&models.SendTXRequest{
		Type:     uint32(txType),
		Sender:   fromAddr,
		Nonce:    txNonce,
		PermID:   0,
		Contract: contracts[0],
	})
}
