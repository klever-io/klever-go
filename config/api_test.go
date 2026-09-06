package config_test

import (
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
// reformat or a merge resolution. /debug serves cached interceptor and resolver
// state, so its secured flag is pinned against the real file.
func TestShippedAPIConfig_SecuredRoutes(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadAPIConfig("node/api.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, cfg.APIPackages, "shipped api.yaml parsed to an empty config")

	assert.True(t, cfg.IsRouteEnabled("node", "/debug"), "/debug must stay registered")
	assert.True(t, cfg.IsRouteSecured("node", "/debug"), "/debug must require Basic Auth")
	assert.True(t, cfg.IsRouteSecured("log", "/log"), "/log must require Basic Auth")
}
