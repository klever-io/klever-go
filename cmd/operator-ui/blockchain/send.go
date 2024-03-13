package blockchain

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	factoryHasher "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal/factory"
)

var PK []byte

func Send(fromAddr, toAddr string, amount float64, kda string) (string, error) {
	precision := uint32(6)

	var isNFT bool
	if strings.Contains(kda, "/") {
		isNFT = true
		precision = 0
	}

	if !isNFT && len(kda) > 0 && kda != string(kdautils.KLVIdentifier) && kda != string(kdautils.KFIIdentifier) {
		kda, err := GetAssetData(kda)
		if err != nil {
			return "", err
		}
		precision = kda.Precision
	}

	parsedAmount := amount * math.Pow10(int(precision))

	data, err := buildRequest(transaction.TXContract_TransferContractType, fromAddr, models.TransferTXRequest{
		Receiver: toAddr,
		Amount:   int64(parsedAmount),
		KDA:      kda,
	})
	if err != nil {
		fmt.Println("6", err)
		return "", err
	}

	return sendSignAndBroadcast(data)
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

	// hash TX and verify
	hasher, err := factoryHasher.NewHasher("blake2b")
	if err != nil {
		return "", err
	}

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	if err != nil {
		return "", err
	}

	hash, err := tools.CalculateHash(internalMarshalizer, hasher, result.Data.TX.RawData)
	if err != nil {
		return "", err
	}

	// sign
	suite := ed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(suite)
	privateKey, err := keyGen.PrivateKeyFromByteArray(PK)
	if err != nil {
		return "", err
	}

	txSingleSigner := &singlesig.Ed25519Signer{}
	signature, err := txSingleSigner.Sign(privateKey, hash)
	if err != nil {
		return "", err
	}

	result.Data.TX.Signature = [][]byte{signature}

	toBroadcast := struct {
		TX *transaction.Transaction `json:"tx"`
	}{
		TX: result.Data.TX,
	}

	data, err = json.Marshal(toBroadcast)
	if err != nil {
		return "", err
	}

	broadcastResult := struct {
		Data struct {
			TXCount int    `json:"txCount"`
			TXHash  string `json:"txHash"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	// broadcast
	err = utils.PostURL(fmt.Sprintf("%s/transaction/broadcast", nodeAPI), string(data), nil, &broadcastResult)
	if err != nil {
		return "", err
	}

	if len(broadcastResult.Error) != 0 {
		return "", fmt.Errorf("error broadcasting transcation: %s", broadcastResult.Error)
	}

	return broadcastResult.Data.TXHash, nil
}
