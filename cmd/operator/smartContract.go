package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/klever-io/klever-go/kvm/scenarioexec"
	scencontroller "github.com/klever-io/klever-go/kvm/scenarioexec/controller"
	"github.com/klever-io/klever-go/kvm/wasmer2"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/spf13/cobra"
)

var (
	arguments []string
	callValue map[string]int64
	metadata  vmcommon.CodeMetadata
)

func subSC() []*cobra.Command {
	var (
		file     string
		vmType   string
		traceGas bool
		scOutput string
	)

	cmdCreate := &cobra.Command{
		Use:     "create",
		Aliases: []string{"csc"},
		Short:   "create a smart contract",
		Args:    cobra.MatchAll(cobra.MinimumNArgs(0), cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("invalid file path provided: %s", file)
			}

			bytecode, err := os.ReadFile(filepath.Clean(file))
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			// write int into hex string
			value_hex := fmt.Sprintf("%04X", metadata.ToBytes())

			message = []string{fmt.Sprintf("%s@%s@%s", hex.EncodeToString(bytecode), vmType, value_hex)}

			if len(args) > 0 {
				message[0] += args[0]
			}

			argsParsed, err := EncodeInput(arguments)
			if err != nil {
				return err
			}

			message[0] += argsParsed

			return smartContractTrigger(
				signerAddress,
				models.SmartContractRequest{
					SCType:    int32(transaction.SmartContract_SCDeploy),
					CallValue: callValue,
				})
		},
	}

	cmdCreate.Flags().StringVar(&file, "wasm", "", "Wasm file path")
	cmdCreate.Flags().StringVar(&vmType, "vmType", "0500", "Vm type")

	cmdUpgrade := &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"usc"},
		Short:   "upgrade a smart contract",
		Args:    cobra.MatchAll(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			toAddress := strings.ReplaceAll(args[0], `"`, ``)
			_, err := walletPubKeyConverter.Decode(toAddress)
			if err != nil {
				return fmt.Errorf("invalid receiver %s: %w", toAddress, err)
			}

			if file == "" {
				return fmt.Errorf("invalid file path provided: %s", file)
			}

			bytecode, err := os.ReadFile(filepath.Clean(file))
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			// write int into hex string
			value_hex := fmt.Sprintf("%04X", metadata.ToBytes())
			message = []string{fmt.Sprintf("upgradeContract@%s@%s", hex.EncodeToString(bytecode), value_hex)}

			if len(args) > 1 {
				message[0] += args[1]
			}

			argsParsed, err := EncodeInput(arguments)
			if err != nil {
				return err
			}

			message[0] += argsParsed

			return smartContractTrigger(
				signerAddress,
				models.SmartContractRequest{
					SCType:    int32(transaction.SmartContract_SCInvoke),
					Address:   toAddress,
					CallValue: callValue,
				})
		},
	}

	cmdUpgrade.Flags().StringVar(&file, "wasm", "", "Wasm file path")

	cmdDelete := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"dsc"},
		Short:   "delete a smart contract",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toAddress := strings.ReplaceAll(args[0], `"`, ``)
			_, err := walletPubKeyConverter.Decode(toAddress)
			if err != nil {
				return fmt.Errorf("invalid receiver %s: %w", toAddress, err)
			}

			message = []string{"deleteContract"}

			return smartContractTrigger(
				signerAddress,
				models.SmartContractRequest{
					SCType:    int32(transaction.SmartContract_SCInvoke),
					Address:   toAddress,
					CallValue: callValue,
				})
		},
	}

	cmdInvoke := &cobra.Command{
		Use:     "invoke",
		Aliases: []string{"isc"},
		Short:   "invoke smart contract: Address FunctionName",
		Args:    cobra.MatchAll(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			toAddress := strings.ReplaceAll(args[0], `"`, ``)
			_, err := walletPubKeyConverter.Decode(toAddress)
			if err != nil {
				return fmt.Errorf("invalid receiver %s: %w", toAddress, err)
			}

			if len(message) == 0 {
				message = make([]string, 1)
			}

			if len(args) > 1 {
				message = []string{args[1]}
			}

			argsParsed, err := EncodeInput(arguments)
			if err != nil {
				return err
			}

			message[0] += argsParsed

			return smartContractTrigger(
				signerAddress,
				models.SmartContractRequest{
					SCType:    int32(transaction.SmartContract_SCInvoke),
					Address:   toAddress,
					CallValue: callValue,
				})
		},
	}

	cmdRunScenarios := &cobra.Command{
		Use:     "run-scenarios",
		Aliases: []string{"rs"},
		Short:   "run smart contract scenarios: PathToScenarios",
		Args:    cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// parse options
			options := &scencontroller.RunScenarioOptions{
				ForceTraceGas: traceGas,
			}

			if len(file) == 0 {
				return fmt.Errorf("invalid file path provided: %s", file)
			}

			// directory of this executable
			exeDir, err := os.Getwd()
			if err != nil {
				return err
			}

			jsonFilePath, isDir, err := resolveScenarioPathArgument(exeDir, file)
			if err != nil {
				return err
			}

			// init
			executor, err := scenarioexec.NewVMTestExecutor()
			if err != nil {
				panic("Could not instantiate VM VM")
			}
			executor.OverrideVMExecutor = wasmer2.ExecutorFactory()

			// execute
			switch {
			case isDir:
				runner := scencontroller.NewScenarioController(
					executor,
					scencontroller.NewDefaultFileResolver(),
				)
				err = runner.RunAllJSONScenariosInDirectory(
					jsonFilePath,
					"",
					".scen.json",
					[]string{},
					options)
			case strings.HasSuffix(jsonFilePath, ".scen.json"):
				runner := scencontroller.NewScenarioController(
					executor,
					scencontroller.NewDefaultFileResolver(),
				)
				err = runner.RunSingleJSONScenario(jsonFilePath, options)
			default:
				runner := scencontroller.NewTestRunner(
					executor,
					scencontroller.NewDefaultFileResolver(),
				)
				err = runner.RunSingleJSONTest(jsonFilePath)
			}
			// print result
			if err == nil {
				log.Info("SUCCESS")
			}
			return err
		},
	}

	cmdRunScenarios.Flags().StringVar(&file, "path", "", "scenario file path")
	cmdRunScenarios.Flags().BoolVar(&traceGas, "traceGas", false, "force gas trace")

	cmdParseScOutput := &cobra.Command{
		Use:     "parse-output [hex | query] [endpoint/view]",
		Aliases: []string{"scpo"},
		Short:   "parse smart contract hex/query output",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(file) == 0 {
				return fmt.Errorf("invalid file path provided: %s", file)
			}

			if len(scOutput) == 0 {
				return fmt.Errorf("empty smart contract output not allowed, fill it correctly")
			}

			abi, err := os.Open(filepath.Clean(file))
			if err != nil {
				return err
			}

			abiDecoder := &vmOutputData{}

			err = abiDecoder.LoadAbi(abi)
			if err != nil {
				return err
			}

			var parsedValue interface{}
			switch args[0] {
			case "query":
				v, err := abiDecoder.DecodeQuery(args[1], []string{scOutput})
				if err != nil {
					return err
				}

				parsedValue = v
			case "hex":
				v, err := abiDecoder.DecodeHex(args[1], []string{scOutput})
				if err != nil {
					return err
				}

				parsedValue = v
			default:
				return fmt.Errorf("invalid parse option '%s'\n\nvalid ones are 'query'  and 'hex'", args[0])
			}

			log.Info("Smart contract output successfully parsed!", "\nParsed value", parsedValue)
			if resultOnly {
				fmt.Println(parsedValue)
			}

			return nil
		},
	}

	cmdParseScOutput.Flags().StringVar(&file, "abi", "", "Your contract abi file path")
	cmdParseScOutput.Flags().StringVar(&scOutput, "raw-output", "", "Raw output of your smart contract endpoint/view")

	return []*cobra.Command{cmdCreate, cmdUpgrade, cmdDelete, cmdInvoke, cmdRunScenarios, cmdParseScOutput}
}

func init() {
	cmdSmartContract := &cobra.Command{
		Use:   "sc",
		Short: "smart contract actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmdSmartContract.AddCommand(subSC()...)
	cmdSmartContract.PersistentFlags().StringArrayVar(&arguments, "args", []string{}, "SC invoke  arguments")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.Payable, "payable", false, "Contract is payable. Allow direct transfer.")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.Upgradeable, "upgradeable", true, "Contract is upgradeable")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.PayableBySC, "payableBySC", false, "Contract is payableBySC. Allow direct transfer by SC")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.Readable, "readable", false, "Contract is readable")
	cmdSmartContract.PersistentFlags().StringToInt64Var(&callValue, "values", nil, "--values 'KLV=val1,KFI=val2,KDA-1234=val3'")
	rootCmd.AddCommand(cmdSmartContract)
}

func smartContractTrigger(fromAddr string, ccReq models.SmartContractRequest) error {
	data, err := buildRequest(transaction.TXContract_SmartContractType, fromAddr, []interface{}{ccReq})
	if err != nil {
		return err
	}

	log.Info("requesting smart contract trigger", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func resolveScenarioPathArgument(exeDir string, arg string) (string, bool, error) {
	fi, err := os.Stat(arg)
	if os.IsNotExist(err) {
		arg = filepath.Join(exeDir, arg)
		fi, err = os.Stat(arg)
	}
	if err != nil {
		return "", false, err
	}
	return arg, fi.IsDir(), nil
}
