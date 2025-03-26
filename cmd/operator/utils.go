package main

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
)

func subHelper() []*cobra.Command {
	cmdConverter := &cobra.Command{
		Use:   "convert-address",
		Short: "convert address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			from := args[0]

			// check if starts with `klv1`
			if len(from) > 4 && from[:4] == "klv1" {
				bValue, err := walletPubKeyConverter.Decode(from)
				if err != nil {
					return err
				}
				hexValue := hex.EncodeToString(bValue)
				fmt.Printf("0x%s\n", hexValue)
				return nil
			}

			// remove 0x if any and convert to klv1 format
			if len(from) > 2 && from[:2] == "0x" {
				from = from[2:]
			}
			if len(from)%2 != 0 {
				from = "0" + from
			}
			bValue, err := hex.DecodeString(from)
			if err != nil {
				return err
			}

			// check length
			if len(bValue) != 32 {
				return fmt.Errorf("invalid length, expected 32 bytes, got %d", len(bValue))
			}

			klv1Value := walletPubKeyConverter.Encode(bValue)
			fmt.Println(klv1Value)

			return nil
		},
	}

	return []*cobra.Command{
		cmdConverter,
	}
}

func init() {
	cmdAccount := &cobra.Command{
		Use:   "helper",
		Short: "helper actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdAccount.AddCommand(subHelper()...)
	rootCmd.AddCommand(cmdAccount)
}
