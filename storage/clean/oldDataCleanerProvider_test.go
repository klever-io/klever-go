package clean

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func TestNewOldDataCleanerProvider_ShouldWork(t *testing.T) {
	t.Parallel()

	odcp, err := NewOldDataCleanerProvider(config.StoragePruningConfig{})
	require.NoError(t, err)
	require.False(t, check.IfNil(odcp))
}

func TestOldDataCleanerProvider_ShouldClean(t *testing.T) {
	t.Parallel()

	storagePruningConfig := config.StoragePruningConfig{
		CleanOldEpochsData: true,
	}

	odcp, _ := NewOldDataCleanerProvider(storagePruningConfig)
	require.NotNil(t, odcp)

	require.True(t, odcp.ShouldClean())
}
