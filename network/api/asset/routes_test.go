package asset_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/asset"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAssetRoute_EmptyTrailReturns404(t *testing.T) {
	t.Parallel()
	facade := mock.Facade{}
	ws := startNodeServer(t, &facade)

	req, _ := http.NewRequest("GET", "/asset", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestGetAsset(t *testing.T) {
	t.Parallel()

	const (
		klvLogo = "https://kleverscan.org/assets/klv-logo.png"
		kfiLogo = "https://kleverscan.org/assets/kfi-logo.png"
	)

	expectedOverriddenURIs := map[string]string{
		"Whitepaper": "https://bc.klever.finance/wp",
		"Exchange":   "https://bitcoin.me/",
		"Github":     "https://github.com/klever-io",
		"Instagram":  "https://instagram.com/klever.io",
		"Twitter":    "https://x.com/klever_org",
		"Wallet":     "https://klever.io/",
		"Website":    "https://klever.org/",
	}

	staleURIs := map[string]string{
		"Website": "https://stale.example/",
	}

	tests := []struct {
		name         string
		assetID      string
		mockHandler  func(assetID string) (*kapps.KDAData, error)
		expectedCode int
		expectedLogo string
		expectedURIs map[string]string
		expectedErr  string
	}{
		{
			name:    "Success with non-native asset returns asset unchanged",
			assetID: "ABC-1234",
			mockHandler: func(assetID string) (*kapps.KDAData, error) {
				return &kapps.KDAData{
					ID:   []byte(assetID),
					Logo: "https://example.com/abc.png",
					URIs: map[string]string{"Website": "https://abc.example/"},
				}, nil
			},
			expectedCode: http.StatusOK,
			expectedLogo: "https://example.com/abc.png",
			expectedURIs: map[string]string{"Website": "https://abc.example/"},
		},
		{
			name:    "Success with KLV overrides logo and URIs",
			assetID: "KLV",
			mockHandler: func(assetID string) (*kapps.KDAData, error) {
				return &kapps.KDAData{
					ID:   []byte(assetID),
					Logo: "https://stale.example/klv.png",
					URIs: staleURIs,
				}, nil
			},
			expectedCode: http.StatusOK,
			expectedLogo: klvLogo,
			expectedURIs: expectedOverriddenURIs,
		},
		{
			name:    "Success with KFI overrides logo and URIs",
			assetID: "KFI",
			mockHandler: func(assetID string) (*kapps.KDAData, error) {
				return &kapps.KDAData{
					ID:   []byte(assetID),
					Logo: "https://stale.example/kfi.png",
					URIs: staleURIs,
				}, nil
			},
			expectedCode: http.StatusOK,
			expectedLogo: kfiLogo,
			expectedURIs: expectedOverriddenURIs,
		},
		{
			name:    "Facade returns error",
			assetID: "KLV",
			mockHandler: func(assetID string) (*kapps.KDAData, error) {
				return nil, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "could not get requested asset: facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			facade.GetAssetHandler = tc.mockHandler

			ws := startNodeServer(t, &facade)

			req, _ := http.NewRequest("GET", fmt.Sprintf("/asset/%s", tc.assetID), nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
				assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
				return
			}

			assert.Empty(t, response.Error)
			assert.Equal(t, shared.ReturnCodeSuccess, response.Code)

			data, ok := response.Data.(map[string]any)
			assert.True(t, ok, "response data should be an object")

			assetData, ok := data["asset"].(map[string]any)
			assert.True(t, ok, "response data should contain asset object")

			assert.Equal(t, tc.expectedLogo, assetData["Logo"])

			uris, ok := assetData["URIs"].(map[string]any)
			assert.True(t, ok, "URIs should be a map")
			assert.Len(t, uris, len(tc.expectedURIs))
			for k, v := range tc.expectedURIs {
				assert.Equal(t, v, uris[k])
			}
		})
	}
}

func TestGetAsset_InvalidFacade(t *testing.T) {
	t.Parallel()

	ws := gin.New()
	ws.Use(cors.Default())
	assetRoutes := ws.Group("/asset")
	assetRoutes.Use(func(c *gin.Context) {
		c.Set("facade", "not a facade")
		c.Next()
	})
	assetRoute, err := wrapper.NewRouterWrapper("asset", assetRoutes, getRoutesConfig())
	require.NoError(t, err)
	asset.Routes(assetRoute)

	req, _ := http.NewRequest("GET", "/asset/KLV", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	var response shared.GenericAPIResponse
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
}

func startNodeServer(t *testing.T, handler asset.FacadeHandler) *gin.Engine {
	t.Helper()
	ws := gin.New()
	ws.Use(cors.Default())
	assetRoutes := ws.Group("/asset")
	if handler != nil {
		assetRoutes.Use(middleware.WithFacade(handler))
	}
	assetRoute, err := wrapper.NewRouterWrapper("asset", assetRoutes, getRoutesConfig())
	require.NoError(t, err)
	asset.Routes(assetRoute)
	return ws
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"asset": {
				Routes: []config.RouteConfig{
					{Name: "/:id", Open: true},
					{Name: "/nft/:owner/*id", Open: true},
					{Name: "/pool/:id/", Open: true},
				},
			},
		},
	}
}
