package main

import (
	"encoding/hex"
	"fmt"
	"math"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

type ToAmount struct {
	ToAddress string
	Amount    float64
}

func send(fromAddr, toAddr string, amount float64, kda string) (string, error) {
	return multiTransfer(fromAddr, kda, ToAmount{toAddr, amount})
}

func multiTransfer(fromAddr, kda string, toAmount ...ToAmount) (string, error) {
	precision := uint32(6)

	if len(kda) > 0 && kda != string(kdautils.KLVIdentifier) && kda != string(kdautils.KFIIdentifier) {
		kda, err := getAssetData(kda)
		if err != nil {
			return "", err
		}
		precision = kda.Precision
	}

	contracts := make([]interface{}, 0)
	for _, to := range toAmount {
		parsedAmount := to.Amount * math.Pow10(int(precision))
		contracts = append(contracts, models.TransferTXRequest{
			Receiver: to.ToAddress,
			Amount:   int64(parsedAmount),
			KDA:      kda,
		})
	}

	data, err := buildRequest(transaction.TXContract_TransferContractType, fromAddr, contracts)
	if err != nil {
		return "", err
	}

	log.Info("requesting transfer", "data", string(data))
	return sendSignAndBroadcast(data)
}

func signAndBroadcast(tx *transaction.Transaction) (string, error) {
	hash, err := SignTX(tx)
	if err != nil {
		return "", err
	}

	// multisign option
	txSingleSigner := &singlesig.Ed25519Signer{}
	for _, key := range multiSignKeys {

		signature, err := txSingleSigner.Sign(key, hash)
		if err != nil {
			return "", err
		}

		tx.Signature = append(tx.Signature, []byte(signature))
	}

	hashSt := hex.EncodeToString(hash)

	if createOnly {
		return hashSt, DumpTX(tx)
	}

	return hashSt, broadcast(tx)
}

func sendSignAndBroadcast(data []byte) (string, error) {
	result := struct {
		Data struct {
			TX *transaction.Transaction `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.PostURL(fmt.Sprintf("%s/transaction/send", nodeAPI), string(data), nil, &result)
	if err != nil {
		return "", err
	}

	if len(result.Error) != 0 {
		return "", fmt.Errorf("error creating transaction: %s", result.Error)
	}

	return signAndBroadcast(result.Data.TX)
}
