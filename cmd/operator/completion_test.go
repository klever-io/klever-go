package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runOperator executes rootCmd with args, capturing cobra's output and restoring the
// process-wide state the run mutates.
func runOperator(t *testing.T, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs(args)
	level := log.GetLevel()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		log.SetLevel(level)
	})

	err := rootCmd.Execute()
	return out.String(), err
}

// The guard has no output of its own; it is observable only through what it skips. A
// --password-file that does not exist makes PersistentPreRunE fail for every real command
// before the wallet is read, so a completion request succeeds only if the guard fires.
func TestCompletionRequestSkipsWalletLoading(t *testing.T) {
	prev := pwdFilePath
	pwdFilePath = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { pwdFilePath = prev })

	_, err := runOperator(t, "account")
	require.ErrorContains(t, err, "password file not found")

	out, err := runOperator(t, cobra.ShellCompRequestCmd, "account", "claim", "")
	require.NoError(t, err)
	assert.Equal(t, "0\tstaking rewards\n1\tallowance\n2\tmarketplace\n:4\n", out)

	rootCmd.InitDefaultCompletionCmd()
	completion, _, err := rootCmd.Find([]string{"completion", "bash"})
	require.NoError(t, err)
	assert.True(t, isCompletionRequest(completion))
	assert.False(t, isCompletionRequest(rootCmd))
}

func TestCompletionsResolveThroughCobra(t *testing.T) {
	const assets = "KLV\tKlever\nKFI\tKlever Finance\n:4\n"

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"account", "csv", ""}, "csv\n:8\n"},
		{[]string{"account", "send", "--kda", ""}, assets},
		{[]string{"account", "batchsend", "--kda", ""}, assets},
		{[]string{"account", "freeze", "--kda", ""}, assets},
		{[]string{"account", "unfreeze", "--kda", ""}, assets},
		{[]string{"account", "claim", "--id", ""}, assets},
		{[]string{"account", "claim", "0", "--id", ""}, assets},
		{[]string{"account", "claim", "1", "--id", ""}, assets},
		{[]string{"account", "claim", "2", "--id", ""}, ":4\n"},
		{[]string{"account", "withdraw", "--kda", ""}, assets},
	} {
		out, err := runOperator(t, append([]string{cobra.ShellCompRequestCmd}, tc.args...)...)
		require.NoError(t, err, strings.Join(tc.args, " "))
		assert.Equal(t, tc.want, out, strings.Join(tc.args, " "))
	}
}

func TestIsCompletionArgv(t *testing.T) {
	assert.True(t, isCompletionArgv([]string{"operator", cobra.ShellCompRequestCmd, "account", ""}))
	assert.True(t, isCompletionArgv([]string{"operator", cobra.ShellCompNoDescRequestCmd, "account", ""}))
	assert.False(t, isCompletionArgv([]string{"operator", "account", "send", "--message", cobra.ShellCompRequestCmd}))
	assert.False(t, isCompletionArgv([]string{"operator"}))
}
