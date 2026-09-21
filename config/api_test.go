package config_test

import (
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"log": {Routes: []config.RouteConfig{{Name: "/log", Open: true, Secured: true}}},
			"node": {Routes: []config.RouteConfig{
				{Name: "/status", Open: true, Secured: false},
				{Name: "/metrics", Open: false, Secured: false},
			}},
		},
	}
}

func TestAPIRoutesConfig_IsRouteEnabled(t *testing.T) {
	t.Parallel()
	cfg := sampleRoutesConfig()

	assert.True(t, cfg.IsRouteEnabled("log", "/log"))
	assert.True(t, cfg.IsRouteEnabled("node", "/status"))
	assert.False(t, cfg.IsRouteEnabled("node", "/metrics"), "open:false")
	assert.False(t, cfg.IsRouteEnabled("node", "/missing"), "route absent")
	assert.False(t, cfg.IsRouteEnabled("missing", "/log"), "package absent")
	assert.False(t, config.APIRoutesConfig{}.IsRouteEnabled("log", "/log"), "empty config")
}

func TestAPIRoutesConfig_IsRouteSecured(t *testing.T) {
	t.Parallel()
	cfg := sampleRoutesConfig()

	assert.True(t, cfg.IsRouteSecured("log", "/log"))
	assert.False(t, cfg.IsRouteSecured("node", "/status"), "secured:false")
	assert.False(t, cfg.IsRouteSecured("node", "/missing"), "route absent")
	assert.False(t, cfg.IsRouteSecured("missing", "/log"), "package absent")
	assert.False(t, config.APIRoutesConfig{}.IsRouteSecured("log", "/log"), "empty config")
}

// The shipped api.yaml is what operators run, and every other case here builds a
// synthetic config, so nothing else in the tree fails when a flag is lost to a
// reformat or a merge resolution. The node diagnostic routes expose cached internal
// state and network topology, and /log streams node-wide logs, so their secured
// flags are pinned against the real file.
func TestNodeAPIConfig_DiagnosticRoutesRequireAuthentication(t *testing.T) {
	cfg, err := config.LoadAPIConfig(filepath.Join("node", "api.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, cfg.APIPackages, "shipped api.yaml parsed to an empty config")

	diagnosticRoutes := []string{"/debug", "/peerinfo", "/p2pstatus", "/heartbeatstatus"}
	for _, route := range diagnosticRoutes {
		require.True(t, cfg.IsRouteSecured("node", route),
			"node route %s must require authentication", route)
		require.True(t, cfg.IsRouteEnabled("node", route),
			"node route %s must remain reachable for authenticated operators", route)
	}
	require.True(t, cfg.IsRouteSecured("log", "/log"), "/log must require Basic Auth")
}

// Securing the diagnostic routes must not take the remaining node routes off the
// API. This pins reachability only: whether any of them later grows an auth
// requirement is a separate decision, and asserting they stay unauthenticated
// would turn that hardening into a test failure in this package.
func TestNodeAPIConfig_RemainingRoutesStayEnabled(t *testing.T) {
	cfg, err := config.LoadAPIConfig(filepath.Join("node", "api.yaml"))
	require.NoError(t, err)

	for _, route := range []string{"/status", "/metrics", "/overview", "/statistics", "/enable-epochs"} {
		require.True(t, cfg.IsRouteEnabled("node", route), "node route %s must stay enabled", route)
	}
}
