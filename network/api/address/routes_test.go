package address_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	mmock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
)

const validAddress = "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAddressRoute_EmptyTrailReturns404(t *testing.T) {
	t.Parallel()
	facade := mock.Facade{}
	ws := startNodeServer(&facade)

	req, _ := http.NewRequest("GET", "/address", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestGetKDA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		assetQuery   string
		mockHandler  func(address, asset string) (*kapps.UserKDA, error)
		expectedCode int
		expectedData map[string]interface{}
		expectedErr  string
	}{
		{
			name:         "Success with valid address and asset",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, asset string) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{Balance: 100}, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{"userKDA": map[string]interface{}{"Balance": 100.0}, "address": validAddress, "asset": "KLV"},
		},
		{
			name:         "Success with default asset",
			addressParam: validAddress,
			assetQuery:   "",
			mockHandler: func(address, asset string) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{Balance: 100}, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{"userKDA": map[string]interface{}{"Balance": 100.0}, "address": validAddress, "asset": "KLV"},
		},
		{
			name:         "Missing address parameter",
			addressParam: "",
			assetQuery:   "KLV",
			mockHandler:  nil, // Not needed since the handler won't be called
			expectedCode: http.StatusBadRequest,
			expectedErr:  "get userKDA error: address is empty",
		},
		{
			name:         "Error from facade",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, asset string) (*kapps.UserKDA, error) {
				return nil, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mock facade
			facade := mock.Facade{}
			if tc.mockHandler != nil {
				facade.GetUserKDAHandler = tc.mockHandler
			}

			ws := startNodeServer(&facade)

			// Construct URL
			url := fmt.Sprintf("/address/%s/kda", tc.addressParam)
			if tc.assetQuery != "" {
				url = fmt.Sprintf("%s?asset=%s", url, tc.assetQuery)
			}

			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response map[string]interface{}
			if tc.expectedErr != "" {
				// Verify error response
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], tc.expectedErr)
			} else {
				// Verify success response
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				assert.NoError(t, err)
				for key, value := range tc.expectedData {
					assert.Equal(t, value, response["data"].(map[string]interface{})[key])
				}
			}
		})
	}
}

func TestGetAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		mockHandler  func(address string) (state.UserAccountHandler, error)
		expectedCode int
		expectedErr  string
	}{
		{
			name:         "Success with valid address",
			addressParam: validAddress,
			mockHandler: func(address string) (state.UserAccountHandler, error) {
				return &mmock.AccountWrapMock{}, nil
			},
			expectedCode: http.StatusOK,
		},
		{
			name:         "Error from facade",
			addressParam: "invalidAddress",
			mockHandler: func(address string) (state.UserAccountHandler, error) {
				return nil, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			facade.GetAccountHandler = tc.mockHandler

			ws := startNodeServer(&facade)
			req, _ := http.NewRequest("GET", fmt.Sprintf("/address/%s", tc.addressParam), nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
			} else {
				assert.Empty(t, response.Error)
				assert.NotNil(t, response.Data)
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		assetQuery   string
		mockHandler  func(address, asset string) (int64, error)
		expectedCode int
		expectedData map[string]interface{}
		expectedErr  string
	}{
		{
			name:         "Success with valid address and asset",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, asset string) (int64, error) {
				return 1000, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{"balance": float64(1000)},
		},
		{
			name:         "Success with valid address and assetDefault",
			addressParam: validAddress,
			assetQuery:   "",
			mockHandler: func(address, asset string) (int64, error) {
				if asset == "KLV" {
					return 1000, nil
				}

				return 0, fmt.Errorf("asset not found")
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{"balance": float64(1000)},
		},
		{
			name:         "Empty address",
			addressParam: "",
			assetQuery:   "KLV",
			mockHandler:  nil,
			expectedCode: http.StatusBadRequest,
			expectedErr:  "address is empty",
		},
		{
			name:         "Error from facade",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, asset string) (int64, error) {
				return 0, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			if tc.mockHandler != nil {
				facade.BalanceHandler = tc.mockHandler
			}

			ws := startNodeServer(&facade)
			url := fmt.Sprintf("/address/%s/balance", tc.addressParam)
			if tc.assetQuery != "" {
				url = fmt.Sprintf("%s?asset=%s", url, tc.assetQuery)
			}

			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
			} else {
				assert.Equal(t, tc.expectedData, response.Data)
			}
		})
	}
}

func TestGetAccountNonce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		mockHandler  func(address string) (uint64, uint64, uint64, error)
		expectedCode int
		expectedData map[string]interface{}
		expectedErr  string
	}{
		{
			name:         "Success with valid address",
			addressParam: validAddress,
			mockHandler: func(address string) (uint64, uint64, uint64, error) {
				return 1, 2, 3, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{
				"nonce":             float64(1),
				"firstPendingNonce": float64(2),
				"txPending":         float64(3),
			},
		},
		{
			name:         "Error from facade",
			addressParam: "invalidAddress",
			mockHandler: func(address string) (uint64, uint64, uint64, error) {
				return 0, 0, 0, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			facade.GetNextNonceHandler = tc.mockHandler

			ws := startNodeServer(&facade)
			req, _ := http.NewRequest("GET", fmt.Sprintf("/address/%s/nonce", tc.addressParam), nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
			} else {
				assert.Equal(t, tc.expectedData, response.Data)
			}
		})
	}
}

func TestGetAvailableClaim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		assetQuery   string
		mockHandler  func(address, assetId string) (int64, map[string]int64, int64, error)
		expectedCode int
		expectedData map[string]interface{}
		expectedErr  string
	}{
		{
			name:         "Success with valid parameters",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				return 100, map[string]int64{"KLV": 200}, 300, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{
				"stakingRewards":    float64(100),
				"allStakingRewards": map[string]interface{}{"KLV": float64(200)},
				"allowance":         float64(300),
			},
		},
		{
			name:         "Empty address",
			addressParam: "",
			assetQuery:   "KLV",
			mockHandler:  nil,
			expectedCode: http.StatusBadRequest,
			expectedErr:  "address is empty",
		},
		{
			name:         "Empty asset",
			addressParam: validAddress,
			assetQuery:   "",
			mockHandler:  nil,
			expectedCode: http.StatusBadRequest,
			expectedErr:  "could not get rewards for requested account asset: assetId is empty",
		},
		{
			name:         "Error from facade",
			addressParam: validAddress,
			assetQuery:   "KLV",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				return 0, nil, 0, fmt.Errorf("facade error")
			},
			expectedCode: http.StatusInternalServerError,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			if tc.mockHandler != nil {
				facade.RewardsAvailableToClaimHandler = tc.mockHandler
			}

			ws := startNodeServer(&facade)
			url := fmt.Sprintf("/address/%s/allowance", tc.addressParam)
			if tc.assetQuery != "" {
				url = fmt.Sprintf("%s?asset=%s", url, tc.assetQuery)
			}

			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
			} else {
				assert.Equal(t, tc.expectedData, response.Data)
			}
		})
	}
}

func TestGetAvailableClaimList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addressParam string
		assetsQuery  string
		mockHandler  func(address, assetId string) (int64, map[string]int64, int64, error)
		expectedCode int
		expectedData map[string]interface{}
		expectedErr  string
	}{
		{
			name:         "Success with valid parameters",
			addressParam: validAddress,
			assetsQuery:  "KLV,BTC",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				return 100, map[string]int64{assetId: 200}, 300, nil
			},
			expectedCode: http.StatusOK,
			expectedData: map[string]interface{}{
				"assets": map[string]interface{}{
					"KLV": map[string]interface{}{
						"stakingRewards":    float64(100),
						"allStakingRewards": map[string]interface{}{"KLV": float64(200)},
						"allowance":         float64(300),
					},
					"BTC": map[string]interface{}{
						"stakingRewards":    float64(100),
						"allStakingRewards": map[string]interface{}{"BTC": float64(200)},
						"allowance":         float64(300),
					},
				},
			},
		},
		{
			name:         "Empty address",
			addressParam: "",
			assetsQuery:  "KLV,BTC",
			mockHandler:  nil,
			expectedCode: http.StatusBadRequest,
			expectedErr:  "address is empty",
		},
		{
			name:         "Empty assets",
			addressParam: validAddress,
			assetsQuery:  "",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				return 0, nil, 0, fmt.Errorf("invalid assetID")
			},
			expectedCode: http.StatusBadRequest,
			expectedErr:  "could not get rewards for requested account asset list: assetId is empty",
		},
		{
			name:         "Invalid assets",
			addressParam: validAddress,
			assetsQuery:  "KLV,INVALID",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				if assetId == "INVALID" {
					return 0, nil, 0, fmt.Errorf("invalid assetID")
				}

				return 100, map[string]int64{assetId: 200}, 300, nil
			},
			expectedCode: http.StatusBadRequest,
			expectedErr:  "could not get rewards for requested account asset list: map[INVALID:invalid assetID]",
		},
		{
			name:         "Error from facade",
			addressParam: validAddress,
			assetsQuery:  "KLV,BTC",
			mockHandler: func(address, assetId string) (int64, map[string]int64, int64, error) {
				return 0, nil, 0, fmt.Errorf("facade error for %s", assetId)
			},
			expectedCode: http.StatusBadRequest,
			expectedErr:  "facade error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facade := mock.Facade{}
			if tc.mockHandler != nil {
				facade.RewardsAvailableToClaimHandler = tc.mockHandler
			}

			ws := startNodeServer(&facade)
			url := fmt.Sprintf("/address/%s/allowance/list", tc.addressParam)
			if tc.assetsQuery != "" {
				url = fmt.Sprintf("%s?asset=%s", url, tc.assetsQuery)
			}

			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			var response shared.GenericAPIResponse
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tc.expectedErr != "" {
				assert.Contains(t, response.Error, tc.expectedErr)
			} else {
				assert.Equal(t, tc.expectedData, response.Data)
			}
		})
	}
}

func TestInvalidFacade(t *testing.T) {
	t.Parallel()

	endpoints := []struct {
		name string
		path string
	}{
		{"GetAccount", "/address/test"},
		{"GetBalance", "/address/test/balance"},
		{"GetKDA", "/address/test/kda"},
		{"GetAccountNonce", "/address/test/nonce"},
		{"GetAvailableClaim", "/address/test/allowance?asset=KLV"},
		{"GetAvailableClaimList", "/address/test/allowance/list?asset=KLV"},
	}

	t.Run("nil facade", func(t *testing.T) {
		for _, endpoint := range endpoints {
			t.Run(endpoint.name, func(t *testing.T) {
				ws := startNodeServer(nil)
				req, _ := http.NewRequest("GET", endpoint.path, nil)
				resp := httptest.NewRecorder()
				ws.ServeHTTP(resp, req)

				assert.Equal(t, http.StatusInternalServerError, resp.Code)

				var response shared.GenericAPIResponse
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "nil app context")
			})
		}
	})

	t.Run("invalid facade type", func(t *testing.T) {
		for _, endpoint := range endpoints {
			t.Run(endpoint.name, func(t *testing.T) {
				ws := gin.New()
				invalidFacade := "invalid facade"
				addressGroup := ws.Group("/address")
				addressGroup.Use(func(c *gin.Context) {
					c.Set("facade", invalidFacade) // Set invalid facade type
					c.Next()
				})
				addressRoute, _ := wrapper.NewRouterWrapper("address", addressGroup, getRoutesConfig())
				address.Routes(addressRoute)

				req, _ := http.NewRequest("GET", endpoint.path, nil)
				resp := httptest.NewRecorder()
				ws.ServeHTTP(resp, req)

				assert.Equal(t, http.StatusInternalServerError, resp.Code)

				var response shared.GenericAPIResponse
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "invalid app context")
			})
		}
	})
}

func startNodeServer(handler address.FacadeHandler) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	addressRoutes := ws.Group("/address")
	if handler != nil {
		addressRoutes.Use(middleware.WithFacade(handler))
	}
	addressRoute, _ := wrapper.NewRouterWrapper("address", addressRoutes, getRoutesConfig())
	address.Routes(addressRoute)
	return ws
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"address": {
				Routes: []config.RouteConfig{
					{Name: "/:address/kda", Open: true},
					{Name: "/:address/balance", Open: true},
					{Name: "/:address/nonce", Open: true},
					{Name: "/:address/allowance", Open: true},
					{Name: "/:address/allowance/list", Open: true},
					{Name: "/:address", Open: true},
				},
			},
		},
	}
}
