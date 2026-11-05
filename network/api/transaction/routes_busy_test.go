package transaction_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func busySimulatorFacade() *mock.Facade {
	return &mock.Facade{
		CreateTransactionHandler: func(_ uint32, _ *transaction.TXBaseInfo, _ []json.RawMessage, _ bool) (*transaction.Transaction, []byte, error) {
			return nil, nil, common.ErrTxSimulatorBusy
		},
		EstimateTransactionFeesHandler: func(_ *transaction.Transaction) (*transaction.FeesResponse, error) {
			return nil, fmt.Errorf("computing fees: %w", common.ErrTxSimulatorBusy)
		},
	}
}

func TestSimulatorRoutes_BusySimulatorAnswersTooManyRequests(t *testing.T) {
	t.Parallel()

	estimateBody, err := json.Marshal(transaction.NewBaseTransaction([]byte("sender"), 0, nil, 0, 0))
	require.NoError(t, err)

	requests := []struct {
		endpoint string
		body     string
	}{
		{"/transaction/send", `{"sender":"klv1sender","nonce":1}`},
		{"/transaction/simulate", `{"sender":"klv1sender","nonce":1}`},
		{"/transaction/estimate-fee", string(estimateBody)},
	}

	ws := startNodeServer(busySimulatorFacade())
	for _, r := range requests {
		req, _ := http.NewRequest(http.MethodPost, r.endpoint, bytes.NewBufferString(r.body))
		resp := httptest.NewRecorder()
		ws.ServeHTTP(resp, req)

		response := shared.GenericAPIResponse{}
		loadResponse(resp.Body, &response)

		assert.Equal(t, http.StatusTooManyRequests, resp.Code, r.endpoint)
		assert.Equal(t, shared.ReturnCodeSystemBusy, response.Code, r.endpoint)
		assert.True(t, strings.Contains(response.Error, common.ErrTxSimulatorBusy.Error()), "%s: %s", r.endpoint, response.Error)
	}
}

func TestSimulatorRoutes_OtherErrorStaysBadRequest(t *testing.T) {
	t.Parallel()

	facade := &mock.Facade{
		CreateTransactionHandler: func(_ uint32, _ *transaction.TXBaseInfo, _ []json.RawMessage, _ bool) (*transaction.Transaction, []byte, error) {
			return nil, nil, fmt.Errorf("invalid contract")
		},
	}
	ws := startNodeServer(facade)

	req, _ := http.NewRequest(http.MethodPost, "/transaction/simulate", strings.NewReader(`{"sender":"klv1sender","nonce":1}`))
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)

	response := shared.GenericAPIResponse{}
	loadResponse(resp.Body, &response)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, shared.ReturnCodeRequestError, response.Code)
}
