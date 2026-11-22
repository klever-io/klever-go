package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/cmd/seednode/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAPIRoutesConfig_MissingFileUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := loadAPIRoutesConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	require.NoError(t, err)
	assert.Equal(t, api.DefaultRoutesConfig(), cfg)
}

func TestLoadAPIRoutesConfig_UnknownKeyFailsInsteadOfFallingBack(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.yaml")
	require.NoError(t, os.WriteFile(path, []byte("loging: true\n"), 0o600))

	_, err := loadAPIRoutesConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field loging not found")
}

func TestLoadAPIRoutesConfig_ReadErrorFailsInsteadOfFallingBack(t *testing.T) {
	t.Parallel()

	_, err := loadAPIRoutesConfig(t.TempDir())
	require.Error(t, err)
}
