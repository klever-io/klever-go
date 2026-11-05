package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/stretchr/testify/require"
)

// The shipped node config is what operators run. yaml.v2 matches keys
// case-sensitively, and VirtualMachineConfig's tag is timeOutTolerancePercentage.
// The previous PascalCase key loaded as 0, and both consumers then fall back to
// core.DefaultTolerancePercentage, so the mismatch was invisible while the
// shipped number equalled that default. Pin the parsed value so renaming the
// key back cannot drop the setting without failing this test.
func TestNodeConfig_TimeOutTolerancePercentageIsParsed(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join("node", "config.yaml"))
	require.NoError(t, err)

	got := cfg.VirtualMachine.Execution.TimeOutTolerancePercentage
	require.Equal(t, uint32(core.DefaultTolerancePercentage), got,
		"shipped timeOutTolerancePercentage must load, and stay equal to the unset fallback")
}
