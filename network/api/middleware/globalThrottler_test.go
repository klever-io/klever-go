package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func startNodeServerGlobalThrottler(handler address.FacadeHandler, maxConnections uint32) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	globalThrottler, _ := middleware.NewGlobalThrottler(maxConnections)
	ws.Use(globalThrottler.MiddlewareHandlerFunc())
	ginAddressRoutes := ws.Group("/address")
	if handler != nil {
		ginAddressRoutes.Use(middleware.WithFacade(handler))
	}
	addressRoutes, _ := wrapper.NewRouterWrapper("address", ginAddressRoutes, getRoutesConfig())
	address.Routes(addressRoutes)
	return ws
}

func TestNewGlobalThrottler_InvalidMaxConnectionsShouldErr(t *testing.T) {
	t.Parallel()

	gt, err := middleware.NewGlobalThrottler(0)

	assert.True(t, check.IfNil(gt))
	assert.Equal(t, middleware.ErrInvalidMaxNumRequests, err)
}

func TestNewGlobalThrottler(t *testing.T) {
	t.Parallel()

	gt, err := middleware.NewGlobalThrottler(1)

	assert.False(t, check.IfNil(gt))
	assert.Nil(t, err)
}

func TestGlobalThrottler_LimitUnderShouldProcessRequest(t *testing.T) {
	t.Parallel()

	addr := "testAddress"
	facade := mock.Facade{
		BalanceHandler: func(s string, k string) (value int64, e error) {
			return 10, nil
		},
	}

	maxConnections := uint32(1000)
	ws := startNodeServerGlobalThrottler(&facade, maxConnections)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/address/%s/balance", addr), nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestGlobalThrottler_LimitOverShouldError(t *testing.T) {
	t.Parallel()

	numCalls := uint32(0)
	responseDelay := time.Second
	facade := mock.Facade{
		BalanceHandler: func(s string, k string) (value int64, e error) {
			time.Sleep(responseDelay)
			atomic.AddUint32(&numCalls, 1)

			return 10, nil
		},
	}

	maxConnections := uint32(1)
	ws := startNodeServerGlobalThrottler(&facade, maxConnections)

	mutResponses := sync.Mutex{}
	responses := make(map[int]int)
	numRequests := 10
	numBatches := 2

	for j := 0; j < numBatches; j++ {
		fmt.Printf("Starting batch requests %d, making %d simultaneous requests...\n", j, numRequests)

		for i := 0; i < numRequests; i++ {
			go makeRequestGlobalThrottler(ws, &mutResponses, responses)
		}

		time.Sleep(responseDelay + time.Second)
	}

	assert.Equal(t, uint32(numBatches), atomic.LoadUint32(&numCalls))
	mutResponses.Lock()
	assert.Equal(t, numBatches, responses[http.StatusOK])
	assert.Equal(t, numBatches*(numRequests-1), responses[http.StatusTooManyRequests])
	mutResponses.Unlock()
}

func makeRequestGlobalThrottler(ws *gin.Engine, mutResponses *sync.Mutex, responses map[int]int) {
	addr := "testAddress"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/address/%s/balance", addr), nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	mutResponses.Lock()
	responses[resp.Code]++
	mutResponses.Unlock()
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"address": {
				Routes: []config.RouteConfig{
					{Name: "/:address", Open: true},
					{Name: "/:address/balance", Open: true},
				},
			},
		},
	}
}
