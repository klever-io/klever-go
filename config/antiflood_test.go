package config_test

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestEndpointsThrottlersConfig_EndpointIsParsed(t *testing.T) {
	t.Parallel()

	data := []byte("endpointsThrottlers:\n  - endpoint: /vm/query\n    maxNumGoRoutines: 7\n")
	cfg := config.WebServerAntifloodConfig{}
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	require.Len(t, cfg.EndpointsThrottlers, 1)
	assert.Equal(t, "/vm/query", cfg.EndpointsThrottlers[0].Endpoint)
	assert.Equal(t, int32(7), cfg.EndpointsThrottlers[0].MaxNumGoRoutines)
}

func TestShippedConfig_EndpointsThrottlersAreNamedAndUnique(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"./node/config.yaml", "../integrationTest/config/config.yaml"} {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		throttlers := cfg.Antiflood.WebServer.EndpointsThrottlers
		require.NotEmpty(t, throttlers, path)

		seen := make(map[string]bool)
		for _, e := range throttlers {
			assert.NotEmpty(t, e.Endpoint, "%s: throttler with no endpoint", path)
			assert.Greater(t, e.MaxNumGoRoutines, int32(0), "%s: %s", path, e.Endpoint)
			assert.False(t, seen[e.Endpoint], "%s: %s configured twice", path, e.Endpoint)
			seen[e.Endpoint] = true
		}
	}
}

func TestShippedConfig_QueryVMSettingsAreParsed(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadFromPath("./node/config.yaml")
	require.NoError(t, err)

	querying := cfg.VirtualMachine.Querying
	assert.Equal(t, uint32(1000), querying.MaxQueryWaitInMilliseconds)
	assert.Equal(t, uint32(500), querying.TimeOutForSCExecutionInMilliseconds)
	assert.NotEmpty(t, querying.WasmVMVersions)
}
