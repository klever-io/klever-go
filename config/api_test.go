package config_test

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/stretchr/testify/assert"
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
