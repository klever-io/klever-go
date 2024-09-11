package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools"
	"github.com/spf13/cobra"
)

func subAccount() []*cobra.Command {

	var (
		kdaID    string
		bucketID string
	)

	cmdCreate := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "create new wallet pem file",
		RunE: func(cmd *cobra.Command, args []string) error {
			pwd, err := utils.GetPassphraseWithConfirm(pwd, pwdFilePath)
			if err != nil {
				return err
			}

			_, _, err = tools.CreateWallet(walletPemFile, pwd, signing.NewKeyGenerator(ed25519.NewEd25519()), walletPubKeyConverter)
			return err
		},
	}

	var path string

	cmdImportSK := &cobra.Command{
		Use:     "import-sk",
		Aliases: []string{"sk"},
		Args:    cobra.ExactArgs(1),
		Short:   "import-sk creates pem file from hex encoded secret key",
		RunE: func(cmd *cobra.Command, args []string) error {
			pwd, err := utils.GetPassphraseWithConfirm(pwd, pwdFilePath)
			if err != nil {
				return err
			}

			skBytes, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}

			walletPath := walletPemFile
			if path != "" {
				walletPath = path
			}

			_, err = tools.PemFromSK(skBytes, walletPath, pwd, signing.NewKeyGenerator(ed25519.NewEd25519()), walletPubKeyConverter)
			return err
		},
	}

	cmdImportSK.Flags().StringVar(&path, "path", "", "file to be generated")

	cmdExportSK := &cobra.Command{
		Use:     "export-sk",
		Aliases: []string{"esk"},
		Args:    cobra.NoArgs,
		Short:   "export-sk print hex encoded private key from pem file",
		RunE: func(cmd *cobra.Command, args []string) error {
			sk, err := privateKey.ToByteArray()
			if err != nil {
				return err
			}

			if len(sk) != 64 {
				return fmt.Errorf("invalid secret key. Size: %d", len(sk))
			}

			log.Warn("********Secret Key********")
			log.Warn("DO NOT SHARE THIS INFORMATION")
			log.Info(signerAddress, "SK", sk[:32])

			return nil
		},
	}

	cmdBalance := &cobra.Command{
		Use:     "balance <address>",
		Aliases: []string{"b"},
		Short:   "returns wallet balance",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := ""
			if len(args) > 0 {
				addr = args[0]
			} else {
				addr = signerAddress
			}

			_, err := walletPubKeyConverter.Decode(addr)
			if err != nil {
				return fmt.Errorf("invalid address `%s`: %w", addr, err)
			}

			return accountBalance(addr)
		},
	}

	cmdInfo := &cobra.Command{
		Use:     "info <address>",
		Aliases: []string{"i"},
		Short:   "returns account details",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := ""
			if len(args) > 0 {
				addr = args[0]
			} else {
				addr = signerAddress
			}

			_, err := walletPubKeyConverter.Decode(addr)
			if err != nil {
				return fmt.Errorf("invalid address `%s`: %w", addr, err)
			}

			return getAccount(addr)
		},
	}

	cmdAllowance := &cobra.Command{
		Use:     "allowance <kdaID>",
		Aliases: []string{"i"},
		Short:   "return account allowance on a given KDA",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kda := "KLV"
			if len(args) > 0 {
				kda = args[0]
			}

			allowance, stakingRewards, err := getAllowanceData(signerAddress, kda)
			if err != nil {
				return err
			}

			log.Info(signerAddress, "allowance", allowance, "stakingRewards", stakingRewards)

			return nil
		},
	}

	cmdAddress := &cobra.Command{
		Use:     "address",
		Aliases: []string{"a"},
		Short:   "returns wallet address",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Wallet address: ", signerAddress)

			return nil
		},
	}

	cmdSend := &cobra.Command{
		Use:   "send [TO] [AMOUNT]",
		Short: "create a transfer transaction: send [TO] [AMOUNT]",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			amount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %s: %w", args[1], err)
			}

			toAddress := args[0]
			_, err = walletPubKeyConverter.Decode(toAddress)
			if err != nil {
				return fmt.Errorf("invalid receiver %s: %w", toAddress, err)
			}

			_, err = send(signerAddress, toAddress, amount, kdaID)
			return err
		},
	}
	cmdSend.Flags().StringVar(&kdaID, "kda", "KLV", "--kda=KDAID-0000")

	var batchValues map[string]string
	cmdBatchSend := &cobra.Command{
		Use:   "batchsend",
		Short: "create several transfer transactions: batchsend --values Addr1=Val1 Addr2=Val2 ...",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			toAmount := make([]ToAmount, 0)
			// conver amounts
			for addr, value := range batchValues {
				amount, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return fmt.Errorf("invalid amount %s: %w", value, err)
				}

				_, err = walletPubKeyConverter.Decode(addr)
				if err != nil {
					return fmt.Errorf("invalid receiver %s: %w", addr, err)
				}
				toAmount = append(toAmount, ToAmount{addr, amount})
			}

			if len(toAmount) == 0 {
				return fmt.Errorf("invalid batch values")
			}

			_, err := multiTransfer(signerAddress, kdaID, toAmount...)
			return err
		},
	}
	cmdBatchSend.Flags().StringVar(&kdaID, "kda", "KLV", "--kda=KDAID-0000")
	cmdBatchSend.Flags().StringToStringVar(&batchValues, "values", nil, "--values 'addr1=val1, addr2=val2, addr3=val3'")

	cmdFreeze := &cobra.Command{
		Use:     "freeze [AMOUNT]",
		Aliases: []string{"f"},
		Short:   "freeze asset",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			amount, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %s: %w", args[0], err)
			}

			return freeze(signerAddress, kdaID, amount)
		},
	}
	cmdFreeze.Flags().StringVar(&kdaID, "kda", "KLV", "--kda=KDAID-00000")

	cmdUnFreeze := &cobra.Command{
		Use:     "unfreeze",
		Aliases: []string{"uf"},
		Short:   "unfreeze asset bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			if kdaID == "KLV" || kdaID == "KFI" {
				if len(bucketID) != 64 {
					return fmt.Errorf("please specify a valid bucket ID to unfreeze")
				}
			} else {
				if len(bucketID) < 8 {
					return fmt.Errorf("please specify a valid bucket ID to unfreeze")
				}
			}

			return unfreeze(signerAddress, bucketID, kdaID)
		},
	}
	cmdUnFreeze.Flags().StringVar(&kdaID, "kda", "KLV", "--kda=KDAID-00000")
	cmdUnFreeze.Flags().StringVar(&bucketID, "bucketID", "", "--bucketID=000...000")

	cmdDelegate := &cobra.Command{
		Use:     "delegate [TO]",
		Aliases: []string{"d"},
		Short:   "delegate bucket to validator",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// check owner address
			_, err := walletPubKeyConverter.Decode(args[0])
			if err != nil {
				return fmt.Errorf("invalid to address %s: %w", args[0], err)
			}

			if len(bucketID) != 64 {
				return fmt.Errorf("please specify a valid bucket ID to delegate")
			}

			return delegate(signerAddress, args[0], bucketID)
		},
	}
	cmdDelegate.Flags().StringVar(&bucketID, "bucketID", "", "--bucketID=000...000")

	cmdUnDelegate := &cobra.Command{
		Use:     "undelegate",
		Aliases: []string{"ud"},
		Short:   "undelegate bucket id",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(bucketID) != 64 {
				return fmt.Errorf("please specify a valid bucket ID to undelegate")
			}

			return undelegate(signerAddress, bucketID)
		},
	}
	cmdUnDelegate.Flags().StringVar(&bucketID, "bucketID", "", "--bucketID=000...000")

	cmdClaim := &cobra.Command{
		Use:     "claim <CLAIM_TYPE>",
		Aliases: []string{"c"},
		Short:   "claim transaction",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			claimType := int64(0)
			var err error
			if len(args) == 1 {
				claimType, err = strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid claim type %s: %w", args[0], err)
				}
			}

			return claim(signerAddress, kdaID, int32(claimType)) // #nosec G115
		},
	}
	cmdClaim.Flags().StringVar(&kdaID, "id", "KLV", "--id=KDAID-00000")

	cmdWithdraw := &cobra.Command{
		Use:     "withdraw",
		Aliases: []string{"w"},
		Short:   "withdraw asset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withdraw(signerAddress, kdaID)
		},
	}
	cmdWithdraw.Flags().StringVar(&kdaID, "kda", "KLV", "--kda=KDAID-00000")

	cmdSetName := &cobra.Command{
		Use:     "set-name [NAME]",
		Aliases: []string{"sn"},
		Short:   "set an account name",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setAccountName(signerAddress, args[0])
		},
	}

	var perms []string
	cmdPermission := &cobra.Command{
		Use:   "permission",
		Short: "set wallet permission",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			permissions := make([]models.PermissionTXRequest, 0)
			for _, p := range perms {
				data := models.PermissionTXRequest{}
				err := json.Unmarshal([]byte(p), &data)
				if err != nil {
					return err
				}
				permissions = append(permissions, data)
			}

			return permission(signerAddress, permissions)
		},
	}
	cmdPermission.Flags().StringArrayVarP(&perms, "perm", "", []string{}, `--perm='{"type":1, "threshold": 2, "operations":"0fff", "signers": [{"address":"klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5", "weight": 1},{"address":"klv18cgjm6mfahuvnl3vj8kx3fph8qvth048we0486cd8xp9nenm4saqg0h9jl", "weight": 1}]}'`)

	cmdCSV := &cobra.Command{
		Use:   "csv FILENAME",
		Short: "send transactions based on CSV file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			return csvSend(args[0])
		},
	}

	return []*cobra.Command{
		cmdCreate,
		cmdImportSK,
		cmdExportSK,
		cmdSend,
		cmdBatchSend,
		cmdBalance,
		cmdAddress,
		cmdInfo,
		cmdFreeze,
		cmdUnFreeze,
		cmdDelegate,
		cmdUnDelegate,
		cmdClaim,
		cmdWithdraw,
		cmdSetName,
		cmdPermission,
		cmdAllowance,
		cmdCSV,
	}
}

func init() {
	cmdAccount := &cobra.Command{
		Use:   "account",
		Short: "account actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdAccount.AddCommand(subAccount()...)
	rootCmd.AddCommand(cmdAccount)
}
