package main

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
)

func subValidator() []*cobra.Command {

	var (
		blsKey         string
		validatorName  string
		rewardsAddress string
		logo           string
		commission     float64
		maxDelegation  float64
		canDelegate    bool
		uris           map[string]string
	)

	cmdCreate := &cobra.Command{
		Use:     "create [OWNER_ADDRESS]",
		Aliases: []string{"new"},
		Short:   "create a validator",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// check owner address
			_, err := walletPubKeyConverter.Decode(args[0])
			if err != nil {
				return fmt.Errorf("invalid owner address %s: %w", args[0], err)
			}

			if rewardsAddress == "OWNER" {
				rewardsAddress = args[0]
			}
			_, err = walletPubKeyConverter.Decode(rewardsAddress)
			if err != nil {
				return fmt.Errorf("invalid rewards address %s: %w", rewardsAddress, err)
			}

			var b []byte
			if len(blsKey) > 0 {
				// check if can convert to hex
				b, err = hex.DecodeString(blsKey)
				if err != nil {
					return fmt.Errorf("invalid BLS pubblicKey: %w", err)
				}
			}
			if len(b) != blsPubkeyLen {
				return fmt.Errorf("invalid BLS pubblicKey length: %d <=> %d", len(blsKey), blsPubkeyLen)
			}

			if commission > 100 || commission < 0 {
				return fmt.Errorf("invalid commission, must be 0 <= commission <= 100")
			}

			if maxDelegation != 0 && maxDelegation < 10_000_000 {
				return fmt.Errorf("invalid commission, must be 0 or commission >= 10_000_000")
			}

			return createValidator(args[0],
				blsKey,
				args[0],
				rewardsAddress,
				logo,
				commission,
				maxDelegation,
				canDelegate,
				uris,
				validatorName,
			)
		},
	}

	cmdCreate.Flags().StringVar(&blsKey, "bls", "", "set node BLS PublicKey")
	cmdCreate.Flags().StringVar(&rewardsAddress, "rewards", "OWNER", "set validator rewards address")
	cmdCreate.Flags().StringVar(&validatorName, "name", "", "set validator name")
	cmdCreate.Flags().StringVar(&logo, "logo", "", "configure validator logo URI")
	cmdCreate.Flags().Float64Var(&commission, "commission", 10.0, "set validator commission")
	cmdCreate.Flags().Float64Var(&maxDelegation, "maxDelegation", 100_000_000, "set validator max delegation")
	cmdCreate.Flags().StringToStringVar(&uris, "uris", nil, "validator uris")
	cmdCreate.Flags().BoolVar(&canDelegate, "canDelegate", true, "set if validator can delegate")

	cmdConfig := &cobra.Command{
		Use:     "config [OWNER_ADDRESS]",
		Aliases: []string{"c"},
		Short:   "config a validator",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// check owner address
			_, err := walletPubKeyConverter.Decode(args[0])
			if err != nil {
				return fmt.Errorf("invalid owner address %s: %w", args[0], err)
			}

			if rewardsAddress == "OWNER" {
				rewardsAddress = args[0]
			}
			_, err = walletPubKeyConverter.Decode(rewardsAddress)
			if err != nil {
				return fmt.Errorf("invalid rewards address %s: %w", rewardsAddress, err)
			}

			if len(blsKey) != 0 && len(blsKey) != 192 {
				// check if can convert to hex
				b, err := hex.DecodeString(blsKey)
				if err != nil {
					return fmt.Errorf("invalid BLS pubblicKey: %w", err)
				}
				if len(b) != blsPubkeyLen {
					return fmt.Errorf("invalid BLS pubblicKey length: %d <=> %d", len(blsKey), blsPubkeyLen)
				}
			}

			if commission > 100 || commission < 0 {
				return fmt.Errorf("invalid commission, must be 0 <= commission <= 100")
			}

			if maxDelegation != 0 && maxDelegation < 10_000_000 {
				return fmt.Errorf("invalid commission, must be 0 or commission >= 10_000_000")
			}

			return validatorConfig(
				signerAddress,
				blsKey,
				rewardsAddress,
				logo,
				commission,
				maxDelegation,
				canDelegate,
				uris,
				validatorName,
			)
		},
	}
	cmdConfig.Flags().StringVar(&blsKey, "bls", "", "set node BLS PublicKey")
	cmdConfig.Flags().StringVar(&rewardsAddress, "rewards", "OWNER", "set validator rewards address")
	cmdConfig.Flags().StringVar(&validatorName, "name", "", "set validator name")
	cmdConfig.Flags().StringVar(&logo, "logo", "", "configure validator logo URI")
	cmdConfig.Flags().Float64Var(&commission, "commission", 10.0, "set validator commission")
	cmdConfig.Flags().Float64Var(&maxDelegation, "maxDelegation", 100_000_000, "set validator max delegation")
	cmdConfig.Flags().StringToStringVar(&uris, "uris", nil, "validator uris")
	cmdConfig.Flags().BoolVar(&canDelegate, "canDelegate", true, "set if validator can delegate")

	cmdUnjail := &cobra.Command{
		Use:     "unjail [OWNER_ADDRESS]",
		Aliases: []string{"uj"},
		Short:   "unjail validator",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// check owner address
			_, err := walletPubKeyConverter.Decode(args[0])
			if err != nil {
				return fmt.Errorf("invalid owner address %s: %w", args[0], err)
			}

			if len(multiSignKeys) == 0 && args[0] != signerAddress {
				return fmt.Errorf("owner and signer key does not match (%s: %s)", args[0], signerAddress)
			}

			return unjail(args[0])
		},
	}

	return []*cobra.Command{cmdCreate, cmdConfig, cmdUnjail}

}

func init() {
	cmdValidator := &cobra.Command{
		Use:   "validator",
		Short: "validator actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdValidator.AddCommand(subValidator()...)
	rootCmd.AddCommand(cmdValidator)
}
