package asset_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/asset"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAddressRoute_EmptyTrailReturns404(t *testing.T) {
	t.Parallel()
	facade := mock.Facade{}
	ws := startNodeServer(&facade)

	req, _ := http.NewRequest("GET", "/asset", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func startNodeServer(handler asset.FacadeHandler) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	assetRoutes := ws.Group("/asset")
	if handler != nil {
		assetRoutes.Use(middleware.WithFacade(handler))
	}
	assetRoute, _ := wrapper.NewRouterWrapper("asset", assetRoutes, getRoutesConfig())
	address.Routes(assetRoute)
	return ws
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"asset": {
				Routes: []config.RouteConfig{
					{Name: "/:id", Open: true},
				},
			},
		},
	}
}
