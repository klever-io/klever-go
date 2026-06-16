package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func subscribeRoutesConfig(open, secured bool) config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"subscribe": {Routes: []config.RouteConfig{{Name: "/subscribe", Open: open, Secured: secured}}},
		},
		Credentials: []config.Credential{{Username: "u", Password: "p"}},
		Hasher:      config.TypeConfig{Type: "sha256"},
	}
}

// subscribeStatus registers the routes for cfg and returns the HTTP status of a plain
// (non-WebSocket) GET /subscribe, which is enough to distinguish the three behaviors:
//
//	404 — route not registered (open:false)
//	401 — auth enforced before the upgrade (secured:true) — the bug this PR fixes
//	400 — handler reached, no auth gate, fails the WS handshake (open, not secured)
func subscribeStatus(t *testing.T, cfg config.APIRoutesConfig) int {
	t.Helper()
	ws := gin.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	RegisterRoutes(ctx, ws, cfg, &mock.Facade{})

	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/subscribe", nil))
	return resp.Code
}

// secured:true must enforce auth before the upgrade — an unauthenticated handshake is
// rejected with 401. Previously secured was silently ignored (the route bypassed the
// auth wrapper), so this guards the actual fix end-to-end through RegisterRoutes.
func TestRegisterRoutes_SecuredRejectsUnauthenticated(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, subscribeStatus(t, subscribeRoutesConfig(true, true)))
}

// open:true, secured:false — no auth gate; the plain GET reaches the handler and fails
// the WS upgrade (400). Proves the route is live and NOT auth-gated (would be 401) and
// is registered (would be 404).
func TestRegisterRoutes_OpenNotSecuredReachesHandler(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, subscribeStatus(t, subscribeRoutesConfig(true, false)))
}

// open:false — the route is not registered at all (404), even with secured:true.
func TestRegisterRoutes_NotOpenNotRegistered(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, subscribeStatus(t, subscribeRoutesConfig(false, true)))
}
