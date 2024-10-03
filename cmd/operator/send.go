package main

import (
	"encoding/hex"
	"errors"
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

// shouldSign prompts the user to sign a transaction
// returns an error if the user does not confirm the transaction
func shouldSign(TX *transaction.Transaction) error {
	if autoSign {
		return nil
	}

	// print TX
	fmt.Println("Requested Transaction Details:")
	err := formatAndDumpRawTX(TX)
	if err != nil {
		return err
	}

	// prompt
	fmt.Println("Please carefully review the transaction above.")
	fmt.Println("Would you like to sign this transaction? (yes/no)")
	fmt.Println("Tip: You can skip this prompt in the future by using the --sign flag.")
	confirmation := askForConfirmation()

	// confirm user input by feedback
	if confirmation {
		fmt.Println("Signing transaction...")
	} else {
		return errors.New("transaction signing has been canceled by the user")
	}
	return nil
}

// signAndBroadcast is a utility function that signs and broadcasts a transaction
// for all operator transactions
func signAndBroadcast(tx *transaction.Transaction) (string, error) {
	// ask for permission right before signing
	err := shouldSign(tx)
	if err != nil {
		return "", err
	}

	hash, err := SignTX(tx)
	if err != nil {
		return "", err
	}

	// Multisign option
	txSingleSigner := &singlesig.Ed25519Signer{}

	// add multi sign keys
	for _, key := range multiSignKeys {
		signature, err := txSingleSigner.Sign(key, hash)
		if err != nil {
			return "", err
		}

		// add signature and check duplicates
		tx.AddSignature(signature)
	}

	hashSt := hex.EncodeToString(hash)

	if createOnly {
		return hashSt, DumpAsJson(tx)
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
