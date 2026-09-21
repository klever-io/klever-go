package main

import (
	"github.com/spf13/cobra"
)

const completionCmdName = "completion"

var claimTypeCompletions = []string{
	"0\tstaking rewards",
	"1\tallowance",
	"2\tmarketplace",
}

var completeBaseAsset = cobra.FixedCompletions([]string{
	"KLV\tKlever",
	"KFI\tKlever Finance",
}, cobra.ShellCompDirectiveNoFileComp)

// Bash and Zsh honour the extension filter; cobra 1.8.1's Fish and PowerShell scripts do not
// and fall back to the shell's unfiltered file completion.
var completeCSVFile = cobra.FixedCompletions([]string{"csv"}, cobra.ShellCompDirectiveFilterFileExt)

// completeClaimID suggests assets only for the claim types whose --id is an asset (staking,
// allowance); a marketplace claim's --id is a marketplace ID. Before the claim type is typed
// nothing is suggested, so a `--id <TAB>` pick cannot end up as a marketplace ID.
func completeClaimID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 && (args[0] == "0" || args[0] == "1") {
		return completeBaseAsset(cmd, args, toComplete)
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

// isCompletionRequest reports whether cmd is cobra's `completion` command or its hidden
// `__complete` hook. `__completeNoDesc` is an alias of `__complete`, so Name() covers both.
func isCompletionRequest(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case cobra.ShellCompRequestCmd, completionCmdName:
			return true
		}
	}

	return false
}

// isCompletionArgv reports whether argv is a completion request: the `completion <shell>`
// script generator, or the `__complete` / `__completeNoDesc` hook every generated script
// runs on TAB. Both are always argv[1].
func isCompletionArgv(argv []string) bool {
	if len(argv) < 2 {
		return false
	}

	switch argv[1] {
	case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, completionCmdName:
		return true
	}

	return false
}

// mustRegisterFlagCompletion panics when the flag does not exist, so a renamed flag fails
// at startup instead of silently degrading to file completion.
func mustRegisterFlagCompletion(cmd *cobra.Command, flag string, f func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) {
	if err := cmd.RegisterFlagCompletionFunc(flag, f); err != nil {
		panic(err)
	}
}
