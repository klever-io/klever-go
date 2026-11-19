package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

func TestNodeConfig_WasmVMVersionsMatchHost(t *testing.T) {
	paths := []string{
		filepath.Join("node", "config.yaml"),
		filepath.Join("..", "integrationTest", "config", "config.yaml"),
	}

	for _, path := range paths {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		execution := cfg.VirtualMachine.Execution.WasmVMVersions
		querying := cfg.VirtualMachine.Querying.WasmVMVersions
		require.NotEmpty(t, execution, "%s: execution has no Wasm VM version", path)

		versions := make([]config.WasmVMVersionByEpoch, 0, len(execution)+len(querying))
		versions = append(versions, execution...)
		versions = append(versions, querying...)

		for _, ver := range versions {
			require.Equal(t, vmhost.VMVersion, ver.Version,
				"%s: Wasm VM version from epoch %d must be the VM host version", path, ver.StartEpoch)
		}
	}
}
