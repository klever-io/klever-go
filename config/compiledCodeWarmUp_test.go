package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

func loadExecutionConfig(t *testing.T, yamlContent string) config.VirtualMachineConfig {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))
	cfg, err := config.LoadFromPath(path)
	require.NoError(t, err)
	return cfg.VirtualMachine.Execution
}

func TestCompiledCodeWarmUpConfig_IsEnabled(t *testing.T) {
	t.Parallel()

	enabled, disabled := true, false
	require.True(t, config.CompiledCodeWarmUpConfig{}.IsEnabled(), "an absent setting means enabled")
	require.True(t, config.CompiledCodeWarmUpConfig{Enabled: &enabled}.IsEnabled())
	require.False(t, config.CompiledCodeWarmUpConfig{Enabled: &disabled}.IsEnabled())
}

func TestCompiledCodeWarmUpConfig_Parse(t *testing.T) {
	t.Parallel()

	t.Run("section absent", func(t *testing.T) {
		execution := loadExecutionConfig(t, "virtualMachine:\n  execution:\n    timeOutForSCExecutionInMilliseconds: 500\n")
		require.Nil(t, execution.CompiledCodeWarmUp.Enabled)
		require.True(t, execution.CompiledCodeWarmUp.IsEnabled())
	})
	t.Run("key absent", func(t *testing.T) {
		execution := loadExecutionConfig(t, "virtualMachine:\n  execution:\n    compiledCodeWarmUp: {}\n")
		require.True(t, execution.CompiledCodeWarmUp.IsEnabled())
	})
	t.Run("enabled", func(t *testing.T) {
		execution := loadExecutionConfig(t, "virtualMachine:\n  execution:\n    compiledCodeWarmUp:\n      enabled: true\n")
		require.True(t, execution.CompiledCodeWarmUp.IsEnabled())
	})
	t.Run("disabled", func(t *testing.T) {
		execution := loadExecutionConfig(t, "virtualMachine:\n  execution:\n    compiledCodeWarmUp:\n      enabled: false\n")
		require.False(t, execution.CompiledCodeWarmUp.IsEnabled())
	})
}

// The shipped config.yaml is what operators start from: it must state the setting explicitly
// and ship it enabled.
func TestNodeConfig_CompiledCodeWarmUpShippedEnabled(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("node", "config.yaml"))
	require.NoError(t, err)

	warmUp := cfg.VirtualMachine.Execution.CompiledCodeWarmUp
	require.NotNil(t, warmUp.Enabled, "config.yaml must set virtualMachine.execution.compiledCodeWarmUp.enabled")
	require.True(t, warmUp.IsEnabled())
}
