package network_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/network"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/stretchr/testify/assert"
)

func TestNetworkConfigMetrics_NilContextShouldError(t *testing.T) {
	t.Parallel()
	ws := startNodeServer(nil)

	req, _ := http.NewRequest("GET", "/network/config", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
	assert.True(t, strings.Contains(response.Error, errors.ErrNilAppContext.Error()))
}

func TestNetworkStatusMetrics_NilContextShouldError(t *testing.T) {
	t.Parallel()
	ws := startNodeServer(nil)

	req, _ := http.NewRequest("GET", "/network/status", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
	assert.True(t, strings.Contains(response.Error, errors.ErrNilAppContext.Error()))
}

func TestNetworkConfigMetrics_ShouldWork(t *testing.T) {
	t.Parallel()

	statusMetricsProvider := statusHandler.NewStatusMetrics()
	key := core.MetricSlotInterval
	value := uint64(37)
	statusMetricsProvider.SetUInt64Value(key, value)

	facade := mock.Facade{}
	facade.StatusMetricsHandler = func() core.StatusMetricsHandler {
		return statusMetricsProvider
	}

	ws := startNodeServer(&facade)
	req, _ := http.NewRequest("GET", "/network/config", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	respBytes, _ := io.ReadAll(resp.Body)
	respStr := string(respBytes)
	assert.Equal(t, resp.Code, http.StatusOK)

	keyAndValueFoundInResponse := strings.Contains(respStr, key) && strings.Contains(respStr, fmt.Sprintf("%d", value))
	assert.True(t, keyAndValueFoundInResponse)
}

func TestNetwork_FailsWithWrongFacadeTypeConversion(t *testing.T) {
	t.Parallel()

	ws := startNodeServerWrongFacade()
	req, _ := http.NewRequest("GET", "/network/config", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	statusRsp := GeneralResponse{}
	loadResponse(resp.Body, &statusRsp)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, statusRsp.Error, errors.ErrInvalidAppContext.Error())
}

func TestNetworkStatusMetrics_ShouldWork(t *testing.T) {
	t.Parallel()

	statusMetricsProvider := statusHandler.NewStatusMetrics()
	key := core.MetricEpochNumber
	value := uint64(37)
	statusMetricsProvider.SetUInt64Value(key, value)

	facade := mock.Facade{}
	facade.StatusMetricsHandler = func() core.StatusMetricsHandler {
		return statusMetricsProvider
	}

	ws := startNodeServer(&facade)
	req, _ := http.NewRequest("GET", "/network/status", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	respBytes, _ := io.ReadAll(resp.Body)
	respStr := string(respBytes)
	assert.Equal(t, resp.Code, http.StatusOK)

	keyAndValueFoundInResponse := strings.Contains(respStr, key) && strings.Contains(respStr, fmt.Sprintf("%d", value))
	assert.True(t, keyAndValueFoundInResponse)
}

func TestNetworkStatus_FailsWithWrongFacadeTypeConversion(t *testing.T) {
	t.Parallel()

	ws := startNodeServerWrongFacade()
	req, _ := http.NewRequest("GET", "/network/status", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	statusRsp := GeneralResponse{}
	loadResponse(resp.Body, &statusRsp)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, statusRsp.Error, errors.ErrInvalidAppContext.Error())
}

func loadResponse(rsp io.Reader, destination interface{}) {
	jsonParser := json.NewDecoder(rsp)
	err := jsonParser.Decode(destination)
	logError(err)
}

func logError(err error) {
	if err != nil {
		fmt.Println("ERR >>>", err)
	}
}

func startNodeServer(handler network.FacadeHandler) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	networkRoutes := ws.Group("/network")
	if handler != nil {
		networkRoutes.Use(middleware.WithFacade(handler))
	}
	networkRouteWrapper, _ := wrapper.NewRouterWrapper("network", networkRoutes, getRoutesConfig())
	network.Routes(networkRouteWrapper)
	return ws
}

func startNodeServerWrongFacade() *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	ws.Use(func(c *gin.Context) {
		c.Set("facade", mock.WrongFacade{})
	})
	networkRoute := ws.Group("/network")
	networkRouteWrapper, _ := wrapper.NewRouterWrapper("network", networkRoute, getRoutesConfig())
	network.Routes(networkRouteWrapper)
	return ws
}

type GeneralResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"network": {
				Routes: []config.RouteConfig{
					{Name: "/config", Open: true},
					{Name: "/status", Open: true},
					{Name: "/economics", Open: true},
					{Name: "/total-staked", Open: true},
				},
			},
		},
	}
}
