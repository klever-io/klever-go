package main

import (
	"bytes"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	logger "github.com/klever-io/klever-go-logger"
	utils "github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type cfg struct {
	numKeys    int
	keyType    string
	consoleOut bool
	noSplit    bool
}

const validatorType = "validator"
const walletType = "wallet"
const bothType = "both"

type key struct {
	skBytes []byte
	pkBytes []byte
}

const keysFolderPattern = "node-%d"
const blsPubkeyLen = 96
const txSignPubkeyLen = 32

var (
	pwd         string
	pwdFilePath string

	verbose bool
	g       = color.New(color.FgGreen).SprintFunc()

	argsConfig = &cfg{}

	walletKeyFilenameTemplate    = "walletKey%s.pem"
	validatorKeyFilenameTemplate = "validatorKey%s.pem"

	log = logger.GetOrCreate("keygenerator")

	validatorPubKeyConverter, _ = pubkeyConverter.NewHexPubkeyConverter(blsPubkeyLen)
	walletPubKeyConverter, _    = pubkeyConverter.NewBech32PubkeyConverter(txSignPubkeyLen)
)

var appVersion = core.UnVersionedAppString

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "keygenerator",
	Short:        "Wallet/Validtor key generator for kleverchain",
	SilenceUsage: true,
	Version:      appVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			log.SetLevel(logger.LogTrace)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		validatorKeys, walletKeys, err := generateKeys(argsConfig.keyType, argsConfig.numKeys)
		if err != nil {
			return err
		}

		pwd, err = utils.GetPassphraseWithConfirm(pwd, pwdFilePath)
		if err != nil {
			return err
		}

		return outputKeys(validatorKeys, walletKeys, argsConfig.consoleOut, argsConfig.noSplit, pwd)
	},
	Long: fmt.Sprintf(`
This binary will generate a validatorKey.pem and walletKey.pem, each containing private key(s)

%s`, g("type 'keygenerator --help' for details")),
}

func main() {
	// numKeys defines a flag for setting how many keys should generate
	rootCmd.Flags().IntVarP(&argsConfig.numKeys, "num-keys", "n", 1, "How many keys should generate. Example: 1")
	// keyType defines a flag for setting what keys should generate
	rootCmd.Flags().StringVarP(&argsConfig.keyType, "key-type", "t", "validator", fmt.Sprintf(
		"What king of keys should generate. Available options: %s, %s, %s",
		validatorType,
		walletType,
		bothType))
	// consoleOut is the flag that, if active, will print everything on the console, not on a physical file
	rootCmd.Flags().BoolVarP(&argsConfig.consoleOut, "console-out", "c", false, "Boolean option that will enable printing the generated keys directly on the console")
	// noSplit is the flag that, if active, will generate the keys in the same file
	rootCmd.Flags().BoolVarP(&argsConfig.noSplit, "no-split", "s", false, "Boolean option that will make each generated key added in the same file")

	rootCmd.Flags().StringVarP(&pwd, "password", "p", "", "Password encryption for generated file --password=MY_SECRET")
	rootCmd.Flags().Lookup("password").NoOptDefVal = "*"
	rootCmd.PersistentFlags().StringVar(&pwdFilePath, "password-file", "", "path to a file containing the password")

	Execute()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		errMsg := errors.Wrapf(err, "commit: %s, error", appVersion).Error()
		fmt.Fprintf(os.Stderr, errMsg+"\n")
		fmt.Fprintf(os.Stderr, "try adding a `--help` flag\n")
		os.Exit(1)
	}
}

func generateKeys(typeKey string, numKeys int) ([]key, []key, error) {
	if numKeys < 1 {
		return nil, nil, fmt.Errorf("number of keys should be a number greater or equal to 1")
	}

	validatorKeys := make([]key, 0)
	walletKeys := make([]key, 0)
	var err error

	blockSigningGenerator := signing.NewKeyGenerator(mcl.NewSuiteBLS12())
	txSigningGenerator := signing.NewKeyGenerator(ed25519.NewEd25519())

	for i := 0; i < numKeys; i++ {
		switch typeKey {
		case validatorType:
			validatorKeys, err = generateKey(blockSigningGenerator, validatorKeys)
			if err != nil {
				return nil, nil, err
			}
		case walletType:
			walletKeys, err = generateKey(txSigningGenerator, walletKeys)
			if err != nil {
				return nil, nil, err
			}
		case bothType:
			validatorKeys, err = generateKey(blockSigningGenerator, validatorKeys)
			if err != nil {
				return nil, nil, err
			}

			walletKeys, err = generateKey(txSigningGenerator, walletKeys)
			if err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("unknown key type %s", argsConfig.keyType)
		}
	}

	return validatorKeys, walletKeys, nil
}

func generateKey(keyGen crypto.KeyGenerator, list []key) ([]key, error) {
	sk, pk := keyGen.GeneratePair()
	skBytes, err := sk.ToByteArray()
	if err != nil {
		return nil, err
	}

	pkBytes, err := pk.ToByteArray()
	if err != nil {
		return nil, err
	}

	list = append(
		list,
		key{
			skBytes: skBytes,
			pkBytes: pkBytes,
		},
	)

	return list, nil
}

func outputKeys(
	validatorKeys []key,
	walletKeys []key,
	consoleOut bool,
	noSplit bool,
	pwd string,
) error {
	if consoleOut {
		return printKeys(validatorKeys, walletKeys, pwd)
	}

	return saveKeys(validatorKeys, walletKeys, noSplit, pwd)
}

func printKeys(validatorKeys []key, walletKeys []key, pwd string) error {
	if len(validatorKeys)+len(walletKeys) == 0 {
		return fmt.Errorf("internal error: no keys to print")
	}

	var errFound error
	if len(validatorKeys) > 0 {
		err := printSliceKeys("Validator keys:", validatorKeys, validatorPubKeyConverter, pwd)
		if err != nil {
			errFound = err
		}
	}
	if len(walletKeys) > 0 {
		err := printSliceKeys("Wallet keys:", walletKeys, walletPubKeyConverter, pwd)
		if err != nil {
			errFound = err
		}
	}

	return errFound
}

func printSliceKeys(message string, sliceKeys []key, converter core.PubkeyConverter, pwd string) error {
	data := []string{message + "\n"}

	for _, k := range sliceKeys {
		buf := bytes.NewBuffer(make([]byte, 0))
		err := writeKeyToStream(buf, k, converter, pwd)
		if err != nil {
			return err
		}

		data = append(data, buf.String())
	}

	log.Info(strings.Join(data, ""))
	return nil
}

func writeKeyToStream(writer io.Writer, key key, pubkeyConverter core.PubkeyConverter, pwd string) error {
	if check.IfNilReflect(writer) {
		return fmt.Errorf("nil writer")
	}

	pkString := pubkeyConverter.Encode(key.pkBytes)

	blk := &pem.Block{
		Type:  "PRIVATE KEY for " + pkString,
		Bytes: []byte(hex.EncodeToString(key.skBytes)),
	}

	if len(pwd) > 0 {
		var err error
		blk, err = tools.EncryptPEMBlock(blk.Type, []byte(hex.EncodeToString(key.skBytes)), pwd)
		if err != nil {
			return err
		}
	}

	return pem.Encode(writer, blk)
}

func saveKeys(validatorKeys []key, walletKeys []key, noSplit bool, pwd string) error {
	if len(validatorKeys)+len(walletKeys) == 0 {
		return fmt.Errorf("internal error: no keys to save")
	}

	var errFound error
	if len(validatorKeys) > 0 {
		err := saveSliceKeys(validatorKeyFilenameTemplate, validatorKeys, validatorPubKeyConverter, noSplit, pwd)
		if err != nil {
			errFound = err
		}
	}
	if len(walletKeys) > 0 {
		err := saveSliceKeys(walletKeyFilenameTemplate, walletKeys, walletPubKeyConverter, noSplit, pwd)
		if err != nil {
			errFound = err
		}
	}

	return errFound
}

func saveSliceKeys(baseFilenameTemplate string, keys []key, pubkeyConverter core.PubkeyConverter, noSplit bool, pwd string) error {
	var file *os.File
	var err error
	for i, k := range keys {
		shouldCreateFile := !noSplit || i == 0
		if shouldCreateFile {
			file, err = generateFile(i, len(keys), noSplit, baseFilenameTemplate)
			if err != nil {
				return err
			}
		}

		err = writeKeyToStream(file, k, pubkeyConverter, pwd)
		if err != nil {
			return err
		}
		if !noSplit {
			err = file.Close()
			if err != nil {
				return err
			}
		}
	}

	if noSplit {
		return file.Close()
	}

	return nil
}

func generateFile(index int, numKeys int, noSplit bool, baseFilenameTemplate string) (*os.File, error) {
	folder, err := generateFolder(index, numKeys, noSplit)
	if err != nil {
		return nil, err
	}

	filename := filepath.Join(folder, baseFilenameTemplate)
	backupFileIfExists(filename)
	//replace the %s with empty string
	filename = fmt.Sprintf(filename, "")
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return os.OpenFile(filepath.Clean(filename), os.O_CREATE|os.O_WRONLY, common.FileModeUserReadWrite)
}

func generateFolder(index int, numKeys int, noSplit bool) (string, error) {
	absPath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	shouldCreateDirectory := numKeys > 1 && !noSplit
	if shouldCreateDirectory {
		absPath = filepath.Join(absPath, fmt.Sprintf(keysFolderPattern, index))
	}

	log.Info("generating files in", "folder", absPath)

	err = os.MkdirAll(absPath, common.DefaultDirPermission)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

func backupFileIfExists(filenameTemplate string) {
	existingFilename := fmt.Sprintf(filenameTemplate, "")
	if _, err := os.Stat(existingFilename); err != nil {
		if os.IsNotExist(err) {
			return
		}
	}
	//if we reached here the file probably exists, make a timestamped backup
	_ = os.Rename(existingFilename, fmt.Sprintf(filenameTemplate, fmt.Sprintf("_%d", time.Now().Unix())))
}
