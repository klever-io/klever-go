package main

import (
	"fmt"
	"strconv"

	"github.com/klever-io/klever-go/tools"
	"github.com/spf13/cobra"
)

func subGov() []*cobra.Command {
	var (
		description   string
		epochDuration uint32
		parameters    map[string]string
	)

	cmdCreate := &cobra.Command{
		Use:     "create-proposal",
		Aliases: []string{"cp"},
		Short:   "create a network proposal upgrade",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(parameters) == 0 {
				return fmt.Errorf("at least one parameter change must be provided")
			}

			parsedParams := make(map[int32]string)
			for key, value := range parameters {
				paramID, err := tools.SafeStringToI32(key)
				if err != nil {
					return fmt.Errorf("invalid param (%s): %w", key, err)
				}

				parsedParams[paramID] = value
			}

			return proposal(signerAddress,
				description,
				parsedParams,
				epochDuration,
			)
		},
	}
	cmdCreate.Flags().StringToStringVar(&parameters, "parameters", nil, "--parameters '0=10000,1=250000,5=1000'")
	cmdCreate.Flags().StringVar(&description, "description", "", "--description description of the proposal")
	cmdCreate.Flags().Uint32Var(&epochDuration, "duration", 4, "--duration [epochs] (default = 4)")

	cmdVote := &cobra.Command{
		Use:     "vote [PROPOSAL_ID] [VOTE_AMOUNT] [VOTE_TYPE]",
		Aliases: []string{"v"},
		Short:   "vote for network proposal",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// convert ID to int
			proposalID, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid proposalID (%s): %w", args[0], err)
			}

			amount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %s: %w", args[1], err)
			}

			voteType := uint64(0)
			if len(args) > 2 {
				voteType, err = strconv.ParseUint(args[2], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid vote Type (%s): %w", args[0], err)
				}
			}

			return vote(signerAddress,
				proposalID,
				amount,
				voteType,
			)
		},
	}

	return []*cobra.Command{cmdCreate, cmdVote}

}

func init() {
	cmdValidator := &cobra.Command{
		Use:   "gov",
		Short: "governance actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdValidator.AddCommand(subGov()...)
	rootCmd.AddCommand(cmdValidator)
}
