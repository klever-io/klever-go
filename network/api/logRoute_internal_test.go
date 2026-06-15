package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
)

func logRoutesConfig(secured bool) config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"log": {Routes: []config.RouteConfig{{Name: "/log", Open: true, Secured: secured}}},
		},
		Credentials: []config.Credential{{Username: "user", Password: "deadbeef"}},
		Hasher:      config.TypeConfig{Type: "sha256"},
	}
}

func TestIsLogRouteSecured(t *testing.T) {
	t.Parallel()

	assert.True(t, logRoutesConfig(true).IsRouteSecured("log", "/log"))
	assert.False(t, logRoutesConfig(false).IsRouteSecured("log", "/log"))
	assert.False(t, config.APIRoutesConfig{}.IsRouteSecured("log", "/log"))
}

// TestRegisterLoggerWsRoute_SecuredRequiresAuth verifies GHSA-9v8p-frvj-2pcm / KLC-2438:
// when /log is secured, an unauthenticated request is rejected before the WebSocket upgrade.
func TestRegisterLoggerWsRoute_SecuredRequiresAuth(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ws := gin.New()
	registerLoggerWsRoute(ws, &marshal.ProtoMarshalizer{}, logRoutesConfig(true))

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestRegisterLoggerWsRoute_UnsecuredReachesUpgrade confirms an unsecured /log has no auth
// gate: the request reaches the gorilla upgrader, which rejects the non-WebSocket GET with 400.
func TestRegisterLoggerWsRoute_UnsecuredReachesUpgrade(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ws := gin.New()
	registerLoggerWsRoute(ws, &marshal.ProtoMarshalizer{}, logRoutesConfig(false))

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}
