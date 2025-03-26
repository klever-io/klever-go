package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const blsPubkeyLen = 96
const txSignPubkeyLen = 32

var (
	log = logger.GetOrCreate("operator")

	walletPubKeyConverter, _    = pubkeyConverter.NewBech32PubkeyConverter(txSignPubkeyLen)
	validatorPubKeyConverter, _ = pubkeyConverter.NewHexPubkeyConverter(blsPubkeyLen)
)

var (
	appVersion = core.UnVersionedAppString
	g          = color.New(color.FgGreen).SprintFunc()
)

var (
	privateKey crypto.PrivateKey

	multiSignKeys []crypto.PrivateKey
	multiSign     []string

	verbose       bool
	nodeAPI       string
	genDocs       bool
	walletPemFile string
	signerAddress string
	message       []string
	txNonce       uint64
	nonceCheck    string
	permID        int32
	permFrom      string
	kdaFee        string
	pwd           string
	pwdFilePath   string
	createOnly    bool
	autoSign      bool
	await         bool
	resultOnly    bool
)

const (
	NONCE_CHECK_CURRENT       = "current"
	NONCE_CHECK_FIRST_PENDING = "first-pending"
	NONCE_CHECK_PENDING       = "pending"
)

var allowDisabledLogsCommand = []string{
	"convert-address",
	"parse-output",
}

func isDisabledLogsCommand(cmd *cobra.Command) bool {
	for _, disabled := range allowDisabledLogsCommand {
		if strings.HasPrefix(cmd.Use, disabled) {
			return true
		}
	}
	return false
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "operator",
	Short:        "Klever Operator ",
	SilenceUsage: true,
	Version:      appVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if resultOnly {
			if !await && !isDisabledLogsCommand(cmd) {
				return fmt.Errorf("await flag is required to use result-only")
			}
			log.SetLevel(logger.LogNone)
		}

		if verbose {
			log.SetLevel(logger.LogTrace)
		}

		if createOnly || strings.HasPrefix(cmd.Use, "sign") {
			log.SetLevel(logger.LogNone)
		}

		if cmd.Short == "create new wallet pem file" {
			return nil
		}

		// check if args and messages flag were set together
		if len(arguments) > 0 && len(message) > 0 {
			return fmt.Errorf("can only use args or messages flag, not both")
		}

		log.Info("Loading", "keyFile", walletPemFile)
		skPK, pubKey, addr, err := utils.LoadKey(walletPemFile, 0, walletPubKeyConverter, pwd, pwdFilePath)
		if err != nil {
			return err
		}

		signerAddress = addr
		if len(addr) > 0 {
			log.Info("KeyLoaded", "operator", signerAddress)
			if txNonce == 0 {
				if len(permFrom) > 0 {
					// overwrite sender
					addr = permFrom
				}

				if nonce, err := getAccountNonce(addr); err == nil {
					txNonce = nonce
				}
			}
			suite := ed25519.NewEd25519()
			keyGen := signing.NewKeyGenerator(suite)
			privateKey, err = keyGen.PrivateKeyFromByteArray(skPK)
			if err != nil {
				return err
			}
		} else {
			if len(walletPemFile) > 0 &&
				// error if not found and not default key file
				walletPemFile != "./walletKey.pem" {
				return fmt.Errorf("KeyLoaded not found")
			}
			log.Warn("KeyLoaded not found")
		}

		if len(multiSign) > 0 {
			for _, signKey := range multiSign {
				skPK, _, _, err := utils.LoadKey(signKey, 0, walletPubKeyConverter, pwd, pwdFilePath)
				if err != nil {
					return err
				}

				suite := ed25519.NewEd25519()
				keyGen := signing.NewKeyGenerator(suite)
				privateKey, err := keyGen.PrivateKeyFromByteArray(skPK)
				if err != nil {
					return err
				}

				multiSignKeys = append(multiSignKeys, privateKey)
			}

		}

		_ = pubKey
		return nil
	},
	Long: fmt.Sprintf(`
OPERATOR interface to Klever Blockchain

%s`, g("type 'operator --help' for details")),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&walletPemFile, "key-file", "k", "./walletKey.pem", "set walelt pem file --key-file=./walletKey.pem")
	rootCmd.PersistentFlags().StringVarP(&nodeAPI, "node", "n", "http://localhost:8080", "entrypoint to node API --node=https://node.testnet.klever.finance")
	rootCmd.PersistentFlags().Uint64Var(&txNonce, "nonce", 0, "set TX nonce --nonce=33")
	rootCmd.PersistentFlags().StringVar(&nonceCheck, "nonce-check", "current", fmt.Sprintf("use [%s, %s, %s] nonce for the account", NONCE_CHECK_CURRENT, NONCE_CHECK_FIRST_PENDING, NONCE_CHECK_PENDING))
	rootCmd.PersistentFlags().StringArrayVar(&message, "message", nil, "set TX message --message=\"MyMessage\"")
	rootCmd.PersistentFlags().Int32Var(&permID, "permID", 0, "set TX permission ID --permID=0")
	rootCmd.PersistentFlags().StringVar(&permFrom, "fromAddress", "", "overwrite fromAddress")
	rootCmd.PersistentFlags().StringVarP(&pwd, "password", "p", "", "--password=MY_SECRET")
	rootCmd.PersistentFlags().Lookup("password").NoOptDefVal = "*"
	rootCmd.PersistentFlags().StringVar(&pwdFilePath, "password-file", "", "path to a file containing the password")
	rootCmd.PersistentFlags().StringArrayVarP(&multiSign, "multi-files", "m", []string{}, "add more files to sign tx. Ex: -m=./file.pem -m=./file2.pem")
	rootCmd.PersistentFlags().BoolVarP(&createOnly, "create-only", "c", false, "only create transaction to be signed later")
	rootCmd.PersistentFlags().BoolVarP(&autoSign, "sign", "s", false, "auto signs transactions without confirmation")
	rootCmd.PersistentFlags().StringVar(&kdaFee, "kdaFee", "", "use KDA to pay for fees")
	rootCmd.PersistentFlags().BoolVar(&await, "await", false, "wait for transaction to be posted on blockchain")
	rootCmd.PersistentFlags().BoolVar(&resultOnly, "result-only", false, "print result only")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "log trace level")
	rootCmd.MarkFlagsMutuallyExclusive("result-only", "verbose")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		errMsg := errors.Wrapf(err, "commit: %s, error", appVersion).Error()
		fmt.Fprintf(os.Stderr, "%s\n", errMsg)
		fmt.Fprintf(os.Stderr, "try adding a `--help` flag\n")
		os.Exit(1)
	}
}

func main() {
	processEnv()

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr,
				"Klever Operator. %v version %s\n",
				path.Base(os.Args[0]), appVersion)
			os.Exit(0)
			return nil
		},
	})

	if genDocs {
		err := doc.GenMarkdownTree(rootCmd, "./cmd/operator/docs")
		if err != nil {
			os.Exit(1)
		}
	}
	Execute()
}

func processEnv() {
	_ = godotenv.Load()

	// replace nodeAPI from env
	envNode := os.Getenv("KLEVER_NODE")
	if len(envNode) > 0 {
		nodeAPI = envNode
	}

	envDocs := os.Getenv("GEN_DOCS")
	if len(envDocs) > 0 {
		genDocs = true
	}

}
