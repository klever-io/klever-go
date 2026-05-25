package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/data/transaction"

	"github.com/spf13/cobra"
)

var (
	multisignAPI string
)

func subMS() []*cobra.Command {

	cmdDecodeTransaction := &cobra.Command{
		Use:   "decode [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "decode a transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			txm := args[0]
			// try marshal data
			TX := &transaction.Transaction{}
			err := json.Unmarshal([]byte(txm), TX)
			if err != nil {
				return err
			}

			return formatAndDumpRawTX(TX)
		},
	}

	cmdMultisignEncodeTransaction := &cobra.Command{
		Use:   "encode [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "encode a transaction form multisign API",
		RunE: func(cmd *cobra.Command, args []string) error {
			txm := args[0]
			// try marshal data
			tx := &transaction.Transaction{}
			err := json.Unmarshal([]byte(txm), tx)
			if err != nil {
				return err
			}

			encoded, err := encodeMSApiData(tx)
			if err != nil {
				return err
			}

			return DumpAsJson(encoded)
		},
	}

	cmdMultisignAddTransaction := &cobra.Command{
		Use:   "append [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "append transaction data into multisign API",
		RunE: func(cmd *cobra.Command, args []string) error {
			txm := args[0]

			encoded := &MSApiEncoded{}
			// ty encode as raw data
			tx := &transaction.Transaction{}
			err := json.Unmarshal([]byte(txm), tx)
			if err != nil {
				// try to decode API data
				log.Info("decoding multisign API data")
				err = json.Unmarshal([]byte(txm), encoded)
				if err != nil {
					return err
				}
			} else {
				encoded, err = encodeMSApiData(tx)
				if err != nil {
					return err
				}
			}
			return doPostMSTransactionSignature(encoded)

		},
	}

	cmdMultisignBroadcast := &cobra.Command{
		Use:   "broadcast [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "broadcast a transaction from multisign API",
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

			result := struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}{}

			err = utils.PostURL(fmt.Sprintf("%s/broadcast/%s", multisignAPI, hash), "", nil, &result)
			if err != nil {
				return err
			}
			if len(result.Error) != 0 {
				return fmt.Errorf("error broadcasting transaction: %s", result.Error)
			}
			log.Info("successful added", "txHash", hash)

			return nil
		},
	}

	cmdMultisignFetch := &cobra.Command{
		Use:   "by-hash [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "fetch a transaction form multisign API",
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := args[0]
			result, err := getMSApiTransaction(hash)
			if err != nil {
				return err
			}
			return DumpAsJson(result)
		},
	}

	cmdMultisignByAddress := &cobra.Command{
		Use:   "by-address [Address]",
		Args:  cobra.ExactArgs(1),
		Short: "fetch a transaction form multisign API",
		RunE: func(cmd *cobra.Command, args []string) error {
			address := args[0]

			result := make([]MSApiTransaction, 0)
			err := utils.GetURL(fmt.Sprintf("%s/transaction/by-address/%s", multisignAPI, address), &result)
			if err != nil {
				return err
			}

			return DumpAsJson(result)
		},
	}
	cmdMultiSignAndPost := &cobra.Command{
		Use:   "sign [txHash]",
		Short: "sign a transaction from multisign API and post the signature",
		Example: `operator ms sign <txHash>   — sign specific TX by hash
operator ms sign            — interactively choose from pending transactions`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("signing transaction from multisign API", "address", signerAddress)
			var tx *MSApiTransaction = nil
			var err error
			if len(args) == 1 {
				tx, err = getMSApiTransaction(args[0])
			} else {
				log.Info("fetching pending transactions for signing")
				result := make([]MSApiTransaction, 0)
				err = utils.GetURL(fmt.Sprintf("%s/transaction/by-address/%s", multisignAPI, signerAddress), &result)
				if err != nil && err.Error() != "EOF" {
					return err
				}
				if len(result) == 0 {
					log.Info("no transactions found for signing")
					return nil
				}
				tx, err = pendingMsTransactionPicker(&result)
			}
			if err != nil {
				return err
			}
			if tx != nil {
				err = doSignAndPost(tx)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	return []*cobra.Command{
		cmdDecodeTransaction,
		cmdMultisignEncodeTransaction,
		cmdMultisignAddTransaction,
		cmdMultisignBroadcast,
		cmdMultisignFetch,
		cmdMultisignByAddress,
		cmdMultiSignAndPost,
	}
}

func init() {
	cmdMS := &cobra.Command{
		Use:   "ms",
		Short: "multisign actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdMS.AddCommand(subMS()...)
	cmdMS.PersistentFlags().StringVar(&multisignAPI, "multisign-api", "https://multisign.mainnet.klever.org", "multisign API URL")
	rootCmd.AddCommand(cmdMS)
}
