package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// oneLogSlotFacade is mock.Facade with the /log node-wide cap overridden to 1, so a second dial
// can only be refused if RegisterRoutes actually carried the facade's value into the route.
type oneLogSlotFacade struct {
	*mock.Facade
}

func (f *oneLogSlotFacade) LogWSMaxConnections() uint32 { return 1 }

// perIPOneFacade overrides only the per-IP cap; the mock's node-wide 32 then cannot be what
// refuses a second dial from one address.
type perIPOneFacade struct {
	*mock.Facade
}

func (f *perIPOneFacade) LogWSMaxConnectionsPerIP() uint32 { return 1 }

// listedOriginFacade overrides only the allowlist; with the wiring dropped the route falls
// back to an empty list and rejects the origin it lists.
type listedOriginFacade struct {
	*mock.Facade
}

func (f *listedOriginFacade) LogWSAllowedOrigins() []string {
	return []string{"https://ops.example.com"}
}

func serveLogRouteFor(t *testing.T, facade middleware.Handler) string {
	t.Helper()

	ws := gin.New()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	RegisterRoutes(ctx, ws, logRoutesConfig(false), facade)

	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	return srv.Listener.Addr().String()
}

// TestRegisterRoutes_LogCapsComeFromTheFacade pins the config wiring end to end, one facade
// read at a time. Every other cap test calls registerLoggerWsRoute directly with literal
// values, so deleting any of the three facade reads in RegisterRoutes — which silently falls
// back to the built-in 32, unlimited, and an empty allowlist — left the suite green.
func TestRegisterRoutes_LogCapsComeFromTheFacade(t *testing.T) {
	t.Run("node-wide cap", func(t *testing.T) {
		addr := serveLogRouteFor(t, &oneLogSlotFacade{Facade: &mock.Facade{}})

		conn := dialLogRouteAndHandshake(t, addr)
		defer func() { _ = conn.Close() }()

		_, resp, err := dialLogRoute(addr)
		require.Error(t, err, "the facade's cap of 1 must reach the route; the built-in 32 would admit this dial")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})

	t.Run("per-IP cap", func(t *testing.T) {
		addr := serveLogRouteFor(t, &perIPOneFacade{Facade: &mock.Facade{}})

		conn := dialLogRouteAndHandshake(t, addr)
		defer func() { _ = conn.Close() }()

		_, resp, err := dialLogRoute(addr)
		require.Error(t, err, "the facade's per-IP cap of 1 must reach the route; unlimited would admit this dial")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})

	t.Run("allowlist", func(t *testing.T) {
		addr := serveLogRouteFor(t, &listedOriginFacade{Facade: &mock.Facade{}})

		conn, _, err := dialLogRouteWithOrigin(addr, "https://ops.example.com")
		require.NoError(t, err, "the facade's allowlist must reach the route; an empty one rejects every browser")
		_ = conn.Close()
	})
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
	return registeredRouteStatus(t, cfg, "/subscribe")
}

func registeredRouteStatus(t *testing.T, cfg config.APIRoutesConfig, path string) int {
	t.Helper()
	ws := gin.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	RegisterRoutes(ctx, ws, cfg, &mock.Facade{})

	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
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

func TestRegisterRoutes_LogRouteEnabledReachesHandler(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, registeredRouteStatus(t, logRoutesConfig(false), "/log"))
}

func TestRegisterRoutes_LogRouteSecuredRejectsUnauthenticated(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, registeredRouteStatus(t, logRoutesConfig(true), "/log"))
}

func TestRegisterRoutes_LogRouteNotEnabledNotRegistered(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, registeredRouteStatus(t, subscribeRoutesConfig(true, false), "/log"))
}

func proofRoutesConfig(open, secured bool) config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"proof": {Routes: []config.RouteConfig{
				{Name: "/address/:address", Open: open, Secured: secured},
				{Name: "/root-hash/:roothash/address/:address", Open: open, Secured: secured},
				{Name: "/verify", Open: open, Secured: secured},
			}},
		},
		Credentials: []config.Credential{{Username: "u", Password: "p"}},
		Hasher:      config.TypeConfig{Type: "sha256"},
	}
}

func TestRegisterRoutes_ProofOpenReachesHandler(t *testing.T) {
	// No facade is installed, so a registered handler answers 500 rather than 404.
	assert.Equal(t, http.StatusInternalServerError, registeredRouteStatus(t, proofRoutesConfig(true, false), "/proof/address/klv1addr"))
}

func TestRegisterRoutes_ProofSecuredRejectsUnauthenticated(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, registeredRouteStatus(t, proofRoutesConfig(true, true), "/proof/address/klv1addr"))
}

func TestRegisterRoutes_ProofNotOpenNotRegistered(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, registeredRouteStatus(t, proofRoutesConfig(false, true), "/proof/address/klv1addr"))
	assert.Equal(t, http.StatusNotFound, registeredRouteStatus(t, proofRoutesConfig(false, true), "/proof/verify"))
}
