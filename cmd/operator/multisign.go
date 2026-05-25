package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/display"
	"github.com/spf13/cobra"
)

var (
	multisignAPI string
	multisignYes bool
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
			return doPostMSTransaction(encoded)

		},
	}

	cmdMultisignBroadcast := &cobra.Command{
		Use:   "broadcast [Transaction]",
		Args:  cobra.ExactArgs(1),
		Short: "broadcast a transaction form multisign API",
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
		Short: "sign and broadcast a transaction from multisign API",
		Example: `operator ms sign <txHash>   — sign specific TX by hash
operator ms sign            — interactively choose from pending transactions`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("signing transaction from multisign API", "address", signerAddress)
			if len(args) == 1 {
				tx, err := getMSApiTransaction(args[0])
				if err != nil {
					return err
				}
				err = doSignAndPost(tx)
				if err != nil {
					return err
				}
				return nil
			}
			log.Info("fetching pending transactions for signing")
			pages := [][]*display.LineData{make([]*display.LineData, 0)}
			currPage := 0
			perPage := 3
			pushToTable := func(txs []MSApiTransaction) {
				for _, tx := range txs {
					if len(pages[currPage]) >= perPage {
						currPage++
						pages = append(pages, make([]*display.LineData, 0))
					}
					signed := false
					currSignatures := (int64)(0)
					currSignatureWeight := (int64)(0)
					for _, signer := range tx.Signers {
						if !signer.Signed {
							continue
						}
						if signer.Address == signerAddress {
							signed = true
						}
						currSignatures++
						currSignatureWeight += signer.Weight
					}
					pages[currPage] = append(pages[currPage], &display.LineData{Values: []string{
						fmt.Sprintf("%d", len(pages[currPage])),
						tx.Hash,
						tx.Address,
						strings.Split(tx.Raw.RawData.GetContract()[0].GetParameter().TypeUrl, "proto.")[1],
						fmt.Sprintf("%d/%d", currSignatures, len(tx.Signers)),
						fmt.Sprintf("%d/%d", currSignatureWeight, tx.Threshold),
						fmt.Sprintf("%t", signed),
					}})
				}
			}

			result := make([]MSApiTransaction, 0)
			err := utils.GetURL(fmt.Sprintf("%s/transaction/by-address/%s", multisignAPI, signerAddress), &result)
			if err != nil && err.Error() != "EOF" {
				return err
			}
			pushToTable(result)
			currPage = 0
			if pages[currPage] == nil || len(pages[currPage]) == 0 {
				fmt.Printf("no pending transactions found for signing for address %s \n", signerAddress)
				return nil
			}
			for {
				ln, err := display.CreateTableString([]string{"#", "hash", "address", "type", "signers", "weight", "signed"}, pages[currPage])
				if err != nil {
					return err
				}
				ln += fmt.Sprintf("\n Page %d/%d [0-%d]: Select Transaction", currPage+1, len(pages), len(pages[currPage])-1)
				if currPage > 0 {
					ln += ", [p]revious page"
				}
				if currPage < len(pages)-1 {
					ln += ", [n]ext page"
				}
				ln += ", [q]uit \n Input: "
				fmt.Print(ln)
				var input string
				fmt.Scan(&input)
				idx, nanErr := strconv.Atoi(input)
				switch {
				case nanErr == nil && idx >= 0 && idx < len(pages[currPage]):
					tx := result[idx+(currPage*perPage)]
					return doSignAndPost(&tx)
				case input == "n" && currPage < len(pages)-1:
					currPage++
				case input == "p" && currPage > 0:
					currPage--
				case input == "q":
					return nil
				}
				fmt.Printf("\033[%dA\r\033[J", strings.Count(ln, "\n")+1)
				continue
			}
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
	cmdMS.PersistentFlags().BoolVarP(&multisignYes, "yes", "y", false, "skip the confirmation prompt; useful for scripts and non-interactive use cases")
	rootCmd.AddCommand(cmdMS)
}
