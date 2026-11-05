package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	vmData "github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allVMRoutes = []struct {
	path     string
	endpoint string
}{
	{hexPath, hexEndpoint},
	{stringPath, stringEndpoint},
	{intPath, intEndpoint},
	{queryPath, queryEndpoint},
}

type vmFacadeStub struct {
	executeSCQueryCalled func(*process.SCQuery) (*vmData.VMOutputApi, error)
	getThrottlerCalled   func(endpoint string) (core.Throttler, bool)
}

func (f *vmFacadeStub) ExecuteSCQuery(query *process.SCQuery) (*vmData.VMOutputApi, error) {
	return f.executeSCQueryCalled(query)
}

func (f *vmFacadeStub) DecodeAddressPubkey(_ string) ([]byte, error) {
	return []byte("address"), nil
}

func (f *vmFacadeStub) GetThrottlerForEndpoint(endpoint string) (core.Throttler, bool) {
	if f.getThrottlerCalled != nil {
		return f.getThrottlerCalled(endpoint)
	}
	return nil, false
}

func (f *vmFacadeStub) IsInterfaceNil() bool {
	return f == nil
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func startVMServer(facade *vmFacadeStub) *gin.Engine {
	ws := gin.New()
	group := ws.Group("/vm")
	group.Use(middleware.WithFacade(facade))

	routes := make([]config.RouteConfig, 0, len(allVMRoutes))
	for _, r := range allVMRoutes {
		routes = append(routes, config.RouteConfig{Name: r.path, Open: true})
	}
	routesConfig := config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{"vm": {Routes: routes}},
	}
	router, _ := wrapper.NewRouterWrapper("vm", group, routesConfig)
	Routes(router)

	return ws
}

func postQuery(t *testing.T, ws *gin.Engine, endpoint string) (*httptest.ResponseRecorder, shared.GenericAPIResponse) {
	body, err := json.Marshal(VMValueRequest{ScAddress: "sc", FuncName: "get"})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	response := shared.GenericAPIResponse{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&response))

	return resp, response
}

func TestRoutes_EachVMRouteIsThrottledUnderItsEndpointName(t *testing.T) {
	t.Parallel()

	mut := sync.Mutex{}
	requested := make(map[string]bool)
	facade := &vmFacadeStub{
		executeSCQueryCalled: func(_ *process.SCQuery) (*vmData.VMOutputApi, error) {
			t.Error("a throttled request must not reach the query service")
			return nil, nil
		},
		getThrottlerCalled: func(endpoint string) (core.Throttler, bool) {
			mut.Lock()
			requested[endpoint] = true
			mut.Unlock()
			return &mock.ThrottlerStub{CanProcessCalled: func() bool { return false }}, true
		},
	}
	ws := startVMServer(facade)

	for _, r := range allVMRoutes {
		resp, response := postQuery(t, ws, r.endpoint)
		assert.Equal(t, http.StatusTooManyRequests, resp.Code, r.endpoint)
		assert.Equal(t, shared.ReturnCodeSystemBusy, response.Code, r.endpoint)
		assert.True(t, requested[r.endpoint], "no throttler looked up for %s", r.endpoint)
	}
}

func TestRoutes_BusyQueryServiceAnswersTooManyRequests(t *testing.T) {
	t.Parallel()

	facade := &vmFacadeStub{
		executeSCQueryCalled: func(_ *process.SCQuery) (*vmData.VMOutputApi, error) {
			return nil, process.ErrQueryServiceBusy
		},
	}
	ws := startVMServer(facade)

	for _, r := range allVMRoutes {
		resp, response := postQuery(t, ws, r.endpoint)
		assert.Equal(t, http.StatusTooManyRequests, resp.Code, r.endpoint)
		assert.Equal(t, shared.ReturnCodeSystemBusy, response.Code, r.endpoint)
		assert.Contains(t, response.Error, process.ErrQueryServiceBusy.Error(), r.endpoint)
	}
}

func TestRoutes_OtherQueryErrorStaysBadRequest(t *testing.T) {
	t.Parallel()

	facade := &vmFacadeStub{
		executeSCQueryCalled: func(_ *process.SCQuery) (*vmData.VMOutputApi, error) {
			return nil, errors.New("contract not found")
		},
	}
	ws := startVMServer(facade)

	resp, response := postQuery(t, ws, queryEndpoint)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, shared.ReturnCodeRequestError, response.Code)
}

func TestShippedConfig_ThrottlesEveryVMRoute(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"../../../config/node/config.yaml", "../../../integrationTest/config/config.yaml"} {
		cfg, err := config.LoadFromPath(path)
		require.NoError(t, err, path)

		configured := make(map[string]int32)
		for _, e := range cfg.Antiflood.WebServer.EndpointsThrottlers {
			configured[e.Endpoint] = e.MaxNumGoRoutines
		}

		for _, r := range allVMRoutes {
			assert.Greater(t, configured[r.endpoint], int32(0), "%s: %s is not throttled", path, r.endpoint)
		}
	}
}
