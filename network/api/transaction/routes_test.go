package transaction_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/api"
	apiErrors "github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	tr "github.com/klever-io/klever-go/network/api/transaction"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func startNodeServer(handler tr.FacadeHandler) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	ginTransactionRoute := ws.Group("/transaction")
	if handler != nil {
		ginTransactionRoute.Use(middleware.WithFacade(handler))
	}
	transactionRoute, _ := wrapper.NewRouterWrapper("transaction", ginTransactionRoute, getRoutesConfig())
	tr.Routes(transactionRoute)
	return ws
}

func startNodeServerWrongFacade() *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	ws.Use(func(c *gin.Context) {
		c.Set("facade", mock.WrongFacade{})
	})
	ginTransactionRoute := ws.Group("/transaction")
	transactionRoute, _ := wrapper.NewRouterWrapper("transaction", ginTransactionRoute, getRoutesConfig())
	tr.Routes(transactionRoute)
	return ws
}

func getRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"transaction": {
				Routes: []config.RouteConfig{
					{Name: "/send", Open: true},
					{Name: "/broadcast", Open: true},
					{Name: "/:txhash", Open: true},
				},
			},
		},
	}
}

type transactionResponseData struct {
	TxResp *api.Transaction `json:"transaction,omitempty"`
}

type transactionResponse struct {
	Data  transactionResponseData `json:"data"`
	Error string                  `json:"error"`
	Code  string                  `json:"code"`
}

func loadResponse(rsp io.Reader, destination interface{}) {
	jsonParser := json.NewDecoder(rsp)
	err := jsonParser.Decode(destination)
	if err != nil {
		logError(err)
	}
}

func logError(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func TestGetTransaction_NilContextShouldError(t *testing.T) {
	t.Parallel()
	ws := startNodeServer(nil)

	req, _ := http.NewRequest("GET", "/transaction/hash", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
	assert.True(t, strings.Contains(response.Error, apiErrors.ErrNilAppContext.Error()))
}

func TestGetTransaction_WithCorrectHashShouldReturnTransaction(t *testing.T) {
	hash := "hash"
	facade := mock.Facade{
		GetTransactionHandler: func(hash string, withEvents bool) (i *api.Transaction, e error) {
			return &api.Transaction{
				Status: "onChain",
				Hash:   hash,
			}, nil
		},
	}

	req, _ := http.NewRequest("GET", "/transaction/"+hash, nil)
	ws := startNodeServer(&facade)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	response := transactionResponse{}
	loadResponse(resp.Body, &response)
	txResp := response.Data.TxResp
	assert.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, txResp)
	assert.Equal(t, hash, txResp.Hash)
}

func TestGetTransaction_WithUnknownHashShouldReturnNil(t *testing.T) {
	wrongHash := "wronghash"
	facade := mock.Facade{
		GetTransactionHandler: func(hash string, withEvents bool) (*api.Transaction, error) {
			if hash == wrongHash {
				return nil, errors.New("local error")
			}
			return &api.Transaction{
				Status: "onChain",
				Hash:   hash,
			}, nil
		},
	}

	req, _ := http.NewRequest("GET", "/transaction/"+wrongHash, nil)
	ws := startNodeServer(&facade)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	txResp := transactionResponse{}
	loadResponse(resp.Body, &txResp)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Empty(t, txResp.Data)
}

func TestSendTransaction_NilContextShouldError(t *testing.T) {
	t.Parallel()
	ws := startNodeServer(nil)

	req, _ := http.NewRequest("POST", "/transaction/send", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
	assert.True(t, strings.Contains(response.Error, apiErrors.ErrNilAppContext.Error()))
}
