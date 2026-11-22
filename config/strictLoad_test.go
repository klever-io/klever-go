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

func TestLoadAPIConfig_RejectsSecondDocument(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.yaml")
	require.NoError(t, os.WriteFile(path, []byte("apiPackages: {}\n---\nloging: true\n"), 0o600))

	_, err := config.LoadAPIConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "multiple YAML documents")
}

func TestLoadAPIConfig_AcceptsEmptyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.yaml")
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	_, err := config.LoadAPIConfig(path)
	require.NoError(t, err)
}

func TestLoadFromPath_RejectsDuplicateStructField(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "dbLookupExtensions:\n  enabled: false\ndbLookupExtensions:\n  enabled: true\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	_, err := config.LoadFromPath(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "already set")
}

func TestLoadGasScheduleConfig_RejectsDuplicateOperation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gasSchedule.yaml")
	data := "BaseOperationCost:\n  StorePerByte: 1\n  StorePerByte: 2\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	_, err := config.LoadGasScheduleConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "already set")
}
