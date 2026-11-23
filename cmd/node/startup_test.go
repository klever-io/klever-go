package main

import (
	"flag"
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// newStartupContext builds a cli context with the node's storage flags
// registered and the given command line parsed, the way the node sees them.
func newStartupContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()

	set := flag.NewFlagSet("node", flag.ContinueOnError)
	keepOldEpochsData.Apply(set)
	require.NoError(t, set.Parse(args))

	return cli.NewContext(nil, set, nil)
}

func TestApplyStoragePruningFlags_KeepOldEpochsDataTurnsCleaningOff(t *testing.T) {
	t.Parallel()

	cfg := config.Config{StoragePruning: config.StoragePruningConfig{CleanOldEpochsData: true}}
	ctx := newStartupContext(t, "--keep-old-epochs-data")

	applyStoragePruningFlags(ctx, &cfg, logger.GetOrCreate("test"))

	require.False(t, cfg.StoragePruning.CleanOldEpochsData)
}

func TestApplyStoragePruningFlags_WithoutTheFlagKeepsTheConfig(t *testing.T) {
	t.Parallel()

	for _, cleanOldEpochsData := range []bool{true, false} {
		cfg := config.Config{StoragePruning: config.StoragePruningConfig{CleanOldEpochsData: cleanOldEpochsData}}
		ctx := newStartupContext(t)

		applyStoragePruningFlags(ctx, &cfg, logger.GetOrCreate("test"))

		require.Equal(t, cleanOldEpochsData, cfg.StoragePruning.CleanOldEpochsData)
	}
}

func TestApplyStoragePruningFlags_ExplicitFalseNeverTurnsCleaningOn(t *testing.T) {
	t.Parallel()

	cfg := config.Config{StoragePruning: config.StoragePruningConfig{CleanOldEpochsData: false}}
	ctx := newStartupContext(t, "--keep-old-epochs-data=false")

	applyStoragePruningFlags(ctx, &cfg, logger.GetOrCreate("test"))

	require.False(t, cfg.StoragePruning.CleanOldEpochsData)
}
