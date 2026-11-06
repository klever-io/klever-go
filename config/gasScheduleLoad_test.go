package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
)

func TestLoadGasScheduleConfig_RejectsMalformedCosts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		reason  string
	}{
		{"scalar section", "BuiltInCost: 1\n", "not a map of costs"},
		{"empty section", "BuiltInCost:\n", "not a map of costs"},
		{"negative cost", "MaxPerTransaction:\n  MaxBuiltInCallsPerTx: -1\n", "MaxPerTransaction.MaxBuiltInCallsPerTx"},
		{"fractional cost", "BaseOperationCost:\n  PersistPerByte: 1.5\n", "BaseOperationCost.PersistPerByte"},
		{"negative whole float cost", "BaseOperationCost:\n  PersistPerByte: -2.0\n", "BaseOperationCost.PersistPerByte"},
		{"float cost beyond uint64", "BaseOperationCost:\n  PersistPerByte: 1.0e20\n", "BaseOperationCost.PersistPerByte"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "gas.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			var gasSchedule map[string]map[string]uint64
			var err error
			require.NotPanics(t, func() { gasSchedule, err = config.LoadGasScheduleConfig(path) })
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.reason)
			require.Nil(t, gasSchedule)
		})
	}
}

func TestLoadGasScheduleConfig_AcceptsWholeCosts(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gas.yaml")
	content := "BaseOperationCost:\n  StorePerByte: 0\n  PersistPerByte: 1000\n  CompilePerByte: 2.0\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	gasSchedule, err := config.LoadGasScheduleConfig(path)
	require.NoError(t, err)
	require.Equal(t, map[string]uint64{"StorePerByte": 0, "PersistPerByte": 1000, "CompilePerByte": 2},
		gasSchedule["BaseOperationCost"])
}
