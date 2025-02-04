package transaction_test

import (
	"bytes"
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
	"github.com/klever-io/klever-go/data/transaction"
	apiErrors "github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	tr "github.com/klever-io/klever-go/network/api/transaction"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transactionResponseData struct {
	TxResp *api.Transaction `json:"transaction,omitempty"`
}

type transactionResponse struct {
	Data  transactionResponseData `json:"data"`
	Error string                  `json:"error"`
	Code  string                  `json:"code"`
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestTransaction_FailsWithWrongFacadeTypeConversion(t *testing.T) {
	t.Parallel()

	ws := startNodeServerWrongFacade()
	txHash := strings.Repeat("a", 64)
	req, _ := http.NewRequest("GET", "/transaction/"+txHash, nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	statusRsp := transactionResponse{}
	loadResponse(resp.Body, &statusRsp)

	assert.Equal(t, resp.Code, http.StatusInternalServerError)
	assert.Equal(t, statusRsp.Error, apiErrors.ErrInvalidAppContext.Error())
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
					{Name: "/estimate-fee", Open: true},
				},
			},
		},
	}
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
				Status: api.TRANSACTION_STATUS_ON_CHAIN,
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
				Status: api.TRANSACTION_STATUS_ON_CHAIN,
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

func TestEstimateTransactionFees_NilContextShouldError(t *testing.T) {
	t.Parallel()
	ws := startNodeServer(nil)

	req, _ := http.NewRequest("POST", "/transaction/estimate-fee", nil)
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeInternalError, response.Code)
	assert.True(t, strings.Contains(response.Error, apiErrors.ErrNilAppContext.Error()))
}

func TestEstimateTransactionFees_ShouldFailWithBadRequest_NilRequestBody(t *testing.T) {
	t.Parallel()

	facade := mock.Facade{
		EstimateTransactionFeesHandler: func(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
			return &transaction.FeesResponse{}, nil
		},
	}

	ws := startNodeServer(&facade)

	req, _ := http.NewRequest("POST", "/transaction/estimate-fee", nil)
	resp := httptest.NewRecorder()

	ws.ServeHTTP(resp, req)

	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, shared.ReturnCodeRequestError, response.Code)
}

func TestEstimateTransactionFees_ShouldFailWithBadRequest_InvalidBodyReceived(t *testing.T) {
	t.Parallel()

	facade := mock.Facade{
		EstimateTransactionFeesHandler: func(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
			return &transaction.FeesResponse{}, nil
		},
	}

	ws := startNodeServer(&facade)

	invalidBody := map[string]interface{}{
		"RawData": []byte("invalid type"),
	}

	bodyBytes, err := json.Marshal(invalidBody)
	require.Nil(t, err)

	req, _ := http.NewRequest("POST", "/transaction/estimate-fee", bytes.NewReader(bodyBytes))
	resp := httptest.NewRecorder()

	ws.ServeHTTP(resp, req)

	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Nil(t, response.Data)
	assert.Equal(t, shared.ReturnCodeRequestError, response.Code)
}

func TestEstimateTransactionFees_ShouldFailWithBadRequest_EstimateFeeReturnedError(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")

	facade := mock.Facade{
		EstimateTransactionFeesHandler: func(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
			return nil, fmt.Errorf("validation failed")
		},
	}

	ws := startNodeServer(&facade)

	tx := transaction.NewBaseTransaction(sender, 0, nil, 0, 0)
	txBytes, err := json.Marshal(tx)
	require.Nil(t, err)

	req, _ := http.NewRequest("POST", "/transaction/estimate-fee", bytes.NewReader(txBytes))
	resp := httptest.NewRecorder()

	ws.ServeHTTP(resp, req)

	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Nil(t, response.Data)
	assert.Equal(t, shared.ReturnCodeRequestError, response.Code)
}

func TestEstimateTransactionFees_ShouldWork(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")

	expectedKAppFee := int64(1e6)
	expectedBandwidthFee := int64(1e6)

	facade := mock.Facade{
		EstimateTransactionFeesHandler: func(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
			return &transaction.FeesResponse{
				CostResponse: &transaction.CostResponse{
					KAppFee:      expectedKAppFee,
					BandwidthFee: expectedBandwidthFee,
				},
			}, nil
		},
	}

	ws := startNodeServer(&facade)

	tx := transaction.NewBaseTransaction(sender, 0, nil, 0, 0)

	txBytes, err := json.Marshal(tx)
	require.Nil(t, err)

	req, _ := http.NewRequest("POST", "/transaction/estimate-fee", bytes.NewReader(txBytes))
	resp := httptest.NewRecorder()

	ws.ServeHTTP(resp, req)

	var costResponse transaction.CostResponse
	response := shared.GenericAPIResponse{
		Data: &costResponse,
	}
	loadResponse(resp.Body, &response)

	assert.Equal(t, expectedKAppFee, costResponse.KAppFee)
	assert.Equal(t, expectedBandwidthFee, costResponse.BandwidthFee)

	assert.Equal(t, shared.ReturnCodeSuccess, response.Code)
}
