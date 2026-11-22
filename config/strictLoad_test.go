package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShippedYAML_LoadsUnderStrictRule fails when a shipped file carries a key no struct tag matches.
func TestShippedYAML_LoadsUnderStrictRule(t *testing.T) {
	t.Parallel()

	loadConfig := func(p string) error { _, err := config.LoadFromPath(p); return err }
	loadAPI := func(p string) error { _, err := config.LoadAPIConfig(p); return err }
	loadExternal := func(p string) error { _, err := config.LoadExternalConfig(p); return err }
	loadEnableEpochs := func(p string) error { _, err := config.LoadEnableEpochsConfig(p); return err }
	loadGasSchedule := func(p string) error { _, err := config.LoadGasScheduleConfig(p); return err }

	files := map[string]func(string) error{
		"node/config.yaml":                             loadConfig,
		"seednode/config.yaml":                         loadConfig,
		"../integrationTest/config/config.yaml":        loadConfig,
		"node/api.yaml":                                loadAPI,
		"seednode/api.yaml":                            loadAPI,
		"node/external.yaml":                           loadExternal,
		"node/enableEpochs.yaml":                       loadEnableEpochs,
		"../integrationTest/config/enableEpochs.yaml":  loadEnableEpochs,
		"node/gasScheduleV1.yaml":                      loadGasSchedule,
		"../integrationTest/config/gasScheduleV1.yaml": loadGasSchedule,
	}
	for path, load := range files {
		assert.NoError(t, load(filepath.FromSlash(path)), path)
	}
}

func TestLoadFromPath_RejectsUnknownKeyNamingFileAndKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "dbLookupExtensions:\n  enabled: false\n  EpochByHashStorage:\n    cache:\n      capacity: 1\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	_, err := config.LoadFromPath(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "line 3: field EpochByHashStorage not found in type config.DbLookupExtensionsConfig")
}

func TestLoadAPIConfig_RejectsUnknownKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.yaml")
	require.NoError(t, os.WriteFile(path, []byte("loging: true\n"), 0o600))

	_, err := config.LoadAPIConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field loging not found")
}
