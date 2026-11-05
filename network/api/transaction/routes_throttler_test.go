package transaction

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShippedConfig_ThrottlerNamesMatchTransactionRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"../../../config/node/config.yaml", "../../../integrationTest/config/config.yaml"} {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		configured := make(map[string]bool)
		for _, e := range cfg.Antiflood.WebServer.EndpointsThrottlers {
			configured[e.Endpoint] = true
		}

		for _, endpoint := range []string{sendTransactionEndpoint, getTransactionEndpoint, simulateEndpoint} {
			assert.True(t, configured[endpoint], "%s: %s is not throttled", path, endpoint)
		}
	}
}
