package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/kvm/scenarioexec"
	scencontroller "github.com/klever-io/klever-go/kvm/scenarioexec/controller"
	"github.com/klever-io/klever-go/kvm/wasmer2"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/spf13/cobra"
)

func convertArguments(args []string) (string, error) {
	var result string
	for _, arg := range args {
		// split the argument into key and value
		kv := strings.SplitN(arg, ":", 2)

		// return hex encoded if has 1 part
		if len(kv) == 1 {
			result += "@" + hex.EncodeToString([]byte(kv[0]))
			continue
		}

		if len(kv) != 2 {
			return "", fmt.Errorf("invalid argument: %s", arg)
		}

		isOption := false
		// check if it is an option argument
		if strings.HasPrefix(kv[0], "option") {
			isOption = true
			kv[0] = kv[0][6:]
		}

		var value string

		// check type of value
		switch kv[0] {
		case "bi", "BI", "n", "N": // BigNumber
			var v *big.Int
			var ok bool
			// check 0x
			if strings.HasPrefix(kv[1], "0x") {
				// remove 0x and convert from hex string
				v, ok = new(big.Int).SetString(kv[1][2:], 16)
			} else {
				// convert from int string
				v, ok = new(big.Int).SetString(kv[1], 10)
			}
			if !ok {
				return "", fmt.Errorf("invalid value: %s", kv[1])
			}
			value = fmt.Sprintf("%X", v)
			// check padding
			if len(value)%2 != 0 {
				value = "0" + value
			}
		case "i", "I", "i64", "I64": // int64
			// string to int64
			v, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid value: %w", err)
			}
			value = fmt.Sprintf("%016X", v)
		case "u", "U", "u64", "U64": // uint64
			// string to uint64
			v, err := strconv.ParseUint(kv[1], 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid value: %w", err)
			}
			value = fmt.Sprintf("%016X", v)
		case "i32", "I32": // int32
			// string to int32
			str, err := strconv.ParseInt(kv[1], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid value: %w", err)
			}
			value = fmt.Sprintf("%08X", str)
		case "u32", "U32": // uint32
			// string to uint32
			str, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid value: %w", err)
			}
			value = fmt.Sprintf("%08X", str)
		case "s", "S": // String
			// convert to string hex
			value = hex.EncodeToString([]byte(kv[1]))
		case "x", "X": // Hex Value
			// remove 0x if exists
			kv[1] = strings.TrimPrefix(kv[1], "0x")
			// validate hex string
			_, err := hex.DecodeString(kv[1])
			if err != nil {
				return "", fmt.Errorf("invalid hex value: %s", kv[1])
			}
			value = kv[1]
		case "a", "A":
			// decode address
			v, err := walletPubKeyConverter.Decode(kv[1])
			if err != nil {
				return "", fmt.Errorf("invalid address: %s", kv[1])
			}
			value = hex.EncodeToString(v)
		case "0", "e", "E": // empty param
		default:
			return "", fmt.Errorf("invalid type: %s", kv[0])
		}

		if isOption {
			// append option
			value = "01" + value
		}

		// append param
		result += "@" + value
	}

	return result, nil
}

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

			bytecode, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			// write int into hex string
			value_hex := fmt.Sprintf("%04X", metadata.ToBytes())

			message = []string{fmt.Sprintf("%s@%s@%s", hex.EncodeToString(bytecode), vmType, value_hex)}

			if len(args) > 0 {
				message[0] += args[0]
			}

			argsParsed, err := convertArguments(arguments)
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

			bytecode, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			// write int into hex string
			value_hex := fmt.Sprintf("%04X", metadata.ToBytes())
			message = []string{fmt.Sprintf("upgradeContract@%s@%s", hex.EncodeToString(bytecode), value_hex)}

			if len(args) > 1 {
				message[0] += args[1]
			}

			argsParsed, err := convertArguments(arguments)
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

			message = make([]string, 0)

			if len(args) > 1 {
				message = []string{args[1]}
			}

			argsParsed, err := convertArguments(arguments)
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

	return []*cobra.Command{cmdCreate, cmdUpgrade, cmdDelete, cmdInvoke, cmdRunScenarios}
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
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.Payable, "payable", false, "Contract is payable")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.Upgradeable, "upgradeable", true, "Contract is upgradeable")
	cmdSmartContract.PersistentFlags().BoolVar(&metadata.PayableBySC, "payableBySC", false, "Contract is payableBySC")
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
