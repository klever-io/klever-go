package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/spf13/cobra"
)

func init() {
	cmdTXByID := &cobra.Command{
		Use:     "tx-by-id [HASH]",
		Aliases: []string{"tx"},
		Args:    cobra.ExactArgs(1),
		Short:   "get transaction by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := args[0]
			hash = strings.Replace(hash, "0x", "", 1)
			if len(hash) != 64 {
				return fmt.Errorf("invalid TX hash length: %d", len(hash))
			}

			_, err := hex.DecodeString(hash)
			if len(hash) != 64 || err != nil {
				return fmt.Errorf("invalid TX hash %s", hash)
			}

			// Retry loop for getTXByID
			const maxRetries = 5
			var lastErr error
			for i := 0; i < maxRetries; i++ {
				if i > 0 {
					// Wait before retry (exponential backoff)
					time.Sleep(time.Duration(i) * time.Second)
					log.Info("Retrying getTXByID", "attempt", i+1, "hash", hash)
				}

				err := getTXByID(hash)
				if err == nil {
					return nil
				}

				lastErr = err
			}

			return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
		},
	}

	rootCmd.AddCommand(cmdTXByID)
}

func buildRequest(
	txType transaction.TXContract_ContractType,
	fromAddr string,
	contracts []interface{},
) ([]byte, error) {

	if len(contracts) == 0 || len(contracts) > core.MaxLengthOfContracts {
		return nil, fmt.Errorf("invalid len of contracts to build request: %d", len(contracts))
	}

	if len(permFrom) > 0 {
		// overwrite sender
		fromAddr = permFrom
	}

	var parsedMessage [][]byte

	for _, m := range message {
		parsedMessage = append(parsedMessage, []byte(m))
	}

	var contract interface{}
	if len(contracts) == 1 {
		contract = contracts[0]
	}

	return json.Marshal(&models.SendTXRequest{
		Type:      uint32(txType), // #nosec G115 - type casting
		Sender:    fromAddr,
		Nonce:     txNonce,
		PermID:    permID,
		Data:      parsedMessage,
		Contract:  contract,
		Contracts: contracts,
		KDAFee:    kdaFee,
	})
}
