package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	factoryHasher "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/nwidger/jsoncolor"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	fbToNode string
)

func init() {
	cmdSign := &cobra.Command{
		Use:   "sign [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "sign a transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			txm := args[0]
			// try marshal data

			TX := &transaction.Transaction{}
			err := json.Unmarshal([]byte(txm), TX)
			if err != nil {
				return err
			}

			_, err = SignTX(TX)
			if err != nil {
				return err
			}

			return DumpTX(TX)
		},
	}

	cmdBroadcast := &cobra.Command{
		Use:   "broadcast [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "broadcast a transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			txm := args[0]
			// try marshal data

			TX := &transaction.Transaction{}
			err := json.Unmarshal([]byte(txm), TX)
			if err != nil {
				return err
			}

			return broadcast(TX)
		},
	}

	cmdForceBroadcast := &cobra.Command{
		Use:   "fb [Address/TXHASH]",
		Args:  cobra.ExactArgs(1),
		Short: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := args[0]
			hash = strings.Replace(hash, "0x", "", 1)

			if !strings.HasPrefix(hash, "klv1") {
				return broadcastHash(hash)
			}

			// fetch TX from pool
			result := struct {
				Data struct {
					Transactions []struct {
						Hash  string `json:"hash"`
						Nonce int64  `json:"nonce"`
					} `json:"transactions"`
				} `json:"data"`
				Error string `json:"error"`
				Code  string `json:"code"`
			}{}

			log.Info("checking transactions", "address", hash)

			err := utils.GetURL(fmt.Sprintf("%s/transaction/pool?sender=%s", nodeAPI, hash), &result)
			if err != nil {
				return err
			}

			// send all
			for _, tx := range result.Data.Transactions {
				log.Info("sending  tx", "hash", tx.Hash, "nonce", tx.Nonce)
				err = broadcastHash(tx.Hash)
				if err != nil {
					return err
				}
			}

			return nil

		},
	}
	cmdForceBroadcast.Flags().StringVar(&fbToNode, "fb-node", "", "forwarding to")

	rootCmd.AddCommand(cmdSign, cmdBroadcast, cmdForceBroadcast)
}

func broadcastHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid TX hash length: %d", len(hash))
	}

	_, err := hex.DecodeString(hash)
	if len(hash) != 64 || err != nil {
		return fmt.Errorf("invalid TX hash %s", hash)
	}

	tx, err := fetchTX(hash)
	if err != nil {
		return err
	}

	return broadcast(tx.Transaction)
}

func broadcast(tx *transaction.Transaction) error {
	toBroadcast := struct {
		TX *transaction.Transaction `json:"tx"`
	}{
		TX: tx,
	}

	data, err := json.Marshal(toBroadcast)
	if err != nil {
		return err
	}

	broadcastResult := struct {
		Data struct {
			TXCount int    `json:"txCount"`
			TXHash  string `json:"txHash"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	if len(fbToNode) == 0 {
		fbToNode = nodeAPI
	}

	// broadcast
	log.Info("broadcasting...")
	err = utils.PostURL(fmt.Sprintf("%s/transaction/broadcast", fbToNode), string(data), nil, &broadcastResult)
	if err != nil {
		return err
	}

	if len(broadcastResult.Error) != 0 {
		return fmt.Errorf("error broadcasting transaction: %s", broadcastResult.Error)
	}

	log.Info("successful", "txCount", broadcastResult.Data.TXCount, "txHash", broadcastResult.Data.TXHash)

	return checkForTX(await, broadcastResult.Data.TXHash)
}

// checkForTX is a function that checks if we need to wait for a transaction to be posted on a chain
func checkForTX(await bool, hash string) error {
	// If we need to wait
	if !await {
		return nil
	}

	// Create a new ticker that fires every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Define a timeout after 10 attempts (20 seconds)
	timeout := 10

	// Loop until we either find the transaction or hit the timeout
	for attempts := 0; attempts < timeout; attempts++ {
		// Try to get the transaction by its ID
		err := getTXByID(hash)

		// If we didn't get an error, the transaction was found
		if err == nil {
			return nil
		}

		// Wait for the next ticker tick before retrying
		<-ticker.C
	}

	// If we've hit the timeout and still haven't found the transaction, return an error
	return errors.New("transaction not found within the specified timeout")
}

func DumpTX(TX *transaction.Transaction) error {
	// Make a custom formatter with indent set
	// create custom formatter
	f := jsoncolor.NewFormatter()
	f.Indent = "    "

	// marshal v with custom formatter,
	// dst contains colorized output
	dst, err := jsoncolor.MarshalIndentWithFormatter(TX, "", "    ", f)
	if err != nil {
		return err
	}

	// print colorized output to stdout
	fmt.Println(string(dst))
	return nil
}

func SignTX(TX *transaction.Transaction) ([]byte, error) {
	// hash TX and verify
	hasher, err := factoryHasher.NewHasher("blake2b")
	if err != nil {
		return nil, err
	}

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	if err != nil {
		return nil, err
	}

	hash, err := tools.CalculateHash(internalMarshalizer, hasher, TX.RawData)
	if err != nil {
		return nil, err
	}

	// sign
	txSingleSigner := &singlesig.Ed25519Signer{}
	signature, err := txSingleSigner.Sign(privateKey, hash)
	if err != nil {
		return nil, err
	}
	if TX.Signature == nil {
		TX.Signature = make([][]byte, 0)
	}

	TX.Signature = append(TX.Signature, []byte(signature))

	return hash, nil
}
