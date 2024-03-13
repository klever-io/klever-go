package transaction

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

var log = logger.GetOrCreate("api/transaction/logging")

const (
	sendTransactionEndpoint   = "/transaction/send"
	decodeTransactionEndpoint = "/transaction/decode"
	getTransactionEndpoint    = "/transaction/:hash"
	broadcastEndpoint         = "/transaction/broadcast"
	poolEndpoint              = "/transaction/pool"
	simulateEndpoint          = "/transaction/simulate"
	sendPath                  = "/send"
	decodePath                = "/decode"
	broadcastPath             = "/broadcast"
	poolPath                  = "/pool"
	getTransactionPath        = "/:txhash"
	simulatePath              = "/simulate"
	queryParamWithResults     = "withResults"
)

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	CreateTransaction(txType uint32, base *transaction.TXBaseInfo, contracts []json.RawMessage, skipValidate bool) (*transaction.Transaction, []byte, error)
	GetThrottlerForEndpoint(endpoint string) (core.Throttler, bool)
	DecodeTransaction(*transaction.Transaction) (*indexerData.Transaction, error)
	SendBulkTransactions([]*transaction.Transaction) ([]string, error)
	SendTransaction(*transaction.Transaction) (string, error)
	GetTransaction(hash string, withResults bool) (*api.Transaction, error)
	EncodeAddressPubkey(addr []byte) (string, error)
	TXPool(sender string, page int, pageSize int) ([]*api.Transaction, int, error)
	EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error)
	IsInterfaceNil() bool
}

// Routes defines address related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(
		http.MethodPost,
		sendPath,
		middleware.CreateEndpointThrottler(sendTransactionEndpoint),
		SendTX,
	)
	router.RegisterHandler(
		http.MethodPost,
		broadcastPath,
		middleware.CreateEndpointThrottler(broadcastEndpoint),
		BroadcastTX,
	)
	router.RegisterHandler(
		http.MethodPost,
		decodePath,
		middleware.CreateEndpointThrottler(decodeTransactionEndpoint),
		DecodeTX,
	)
	router.RegisterHandler(
		http.MethodGet,
		poolPath,
		middleware.CreateEndpointThrottler(poolEndpoint),
		TXMemPool,
	)
	router.RegisterHandler(
		http.MethodGet,
		getTransactionPath,
		middleware.CreateEndpointThrottler(getTransactionEndpoint),
		GetTransaction,
	)
	router.RegisterHandler(
		http.MethodPost,
		simulatePath,
		middleware.CreateEndpointThrottler(simulateEndpoint),
		SimulateTransaction,
	)
}

func getFacade(c *gin.Context) (FacadeHandler, bool) {
	facadeObj, ok := c.Get("facade")
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.ErrNilAppContext.Error(),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return nil, false
	}

	facade, ok := facadeObj.(FacadeHandler)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.ErrInvalidAppContext.Error(),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return nil, false
	}

	return facade, true
}

type sendTXRequest struct {
	*models.SendTXRequest
	Contract  json.RawMessage   `form:"contract" json:"contract"`
	Contracts []json.RawMessage `form:"contracts" json:"contracts"`
}

// @Summary returns send transaction
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param data body sendTXRequest true "body data"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Router /transaction/send [post]
// SendTX returns send transaction
func SendTX(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	skipValidateQuery := c.Query("skipValidate")

	skipValidate := false
	if skipValidateQuery == "true" {
		skipValidate = true
	}

	var gtx = sendTXRequest{}
	err := c.ShouldBindJSON(&gtx)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	var contracts []json.RawMessage = gtx.Contracts
	if len(contracts) == 0 && gtx.Contract != nil {
		contracts = []json.RawMessage{gtx.Contract}
	}

	baseInfo := &transaction.TXBaseInfo{
		Sender: gtx.Sender,
		// TODO: allow username?
		// SenderUsername:
		Nonce:          gtx.Nonce,
		SenderUsername: nil,
		DataField:      gtx.Data,
		PermID:         gtx.PermID,
		KDAFee:         gtx.KDAFee,
	}

	tx, txHash, err := facade.CreateTransaction(gtx.Type, baseInfo, contracts, skipValidate)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data: gin.H{
					"result": tx,
					"txHash": "",
				},
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data: gin.H{
				"result": tx,
				"txHash": hex.EncodeToString(txHash),
			},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// BroadcastTXRequest -
type BroadcastTXRequest struct {
	TX  *transaction.Transaction   `form:"tx" json:"tx"`
	TXs []*transaction.Transaction `form:"txs" json:"txs"`
}

// @Summary broadcast the TX
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param data body BroadcastTXRequest true "body data"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Router /transaction/broadcast [post]
// BroadcastTX broadcast the TX
func BroadcastTX(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	var gtx = BroadcastTXRequest{}
	err := c.ShouldBindJSON(&gtx)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	if gtx.TX != nil {
		txHash, err := facade.SendTransaction(
			gtx.TX,
		)

		if err != nil {
			c.JSON(
				http.StatusBadRequest,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
					Code:  shared.ReturnCodeRequestError,
				},
			)
			return
		}

		c.JSON(
			http.StatusOK,
			shared.GenericAPIResponse{
				Data:  gin.H{"txHash": txHash, "txCount": 1},
				Error: "",
				Code:  shared.ReturnCodeSuccess,
			},
		)
		return
	}

	txsHashes, err := facade.SendBulkTransactions(gtx.TXs)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"txsHashes": txsHashes},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary decode a given transaction
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param data body transaction.Transaction true "body data"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Router /transaction/decode [post]
// DecodeTX decode the transaction
func DecodeTX(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	var gtx = &transaction.Transaction{}
	err := c.ShouldBindJSON(&gtx)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	decodedTX, err := facade.DecodeTransaction(gtx)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"tx": decodedTX},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns transaction memory pool
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param sender query string true "sender"
// @Param page query int true "page"
// @Param pageSize query int true "page size"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "some internal error"
// @Router /transaction/pool [get]
// TXMemPool returns transaction memory pool
func TXMemPool(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	page := 0
	pageSize := 100
	values := c.Request.URL.Query()
	sender := values.Get("sender")
	if data := values.Get("page"); len(data) > 0 {
		v, err := strconv.ParseInt(data, 10, 32)
		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: fmt.Sprintf("%s: %s", errors.ErrGetTransaction.Error(), err.Error()),
					Code:  shared.ReturnCodeInternalError,
				},
			)
			return
		}
		page = int(v)
		if page < 0 {
			page = 0
		}
	}

	if data := values.Get("pageSize"); len(data) > 0 {
		v, err := strconv.ParseInt(data, 10, 32)
		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: fmt.Sprintf("%s: %s", errors.ErrGetTransaction.Error(), err.Error()),
					Code:  shared.ReturnCodeInternalError,
				},
			)
			return
		}
		pageSize = int(v)
		if pageSize < 1 {
			pageSize = 1
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
	}

	txs, total, err := facade.TXPool(sender, page, pageSize)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrGetTransaction.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	list := make([]map[string]interface{}, 0)
	for _, tx := range txs {
		addr, _ := facade.EncodeAddressPubkey(tx.GetSender())
		list = append(list, map[string]interface{}{
			"hash":   tx.Hash,
			"sender": addr,
			"nonce":  tx.GetNonce(),
		})
	}

	data := gin.H{"transactions": list, "total": total, "page": page, "pageSize": pageSize}
	if len(sender) > 0 {
		data["sender"] = sender
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  data,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns transaction details for a given txhash
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param txhash path string true "txhash"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "some internal error"
// @Router /transaction/{txhash} [get]
// GetTransaction returns transaction details for a given txhash
func GetTransaction(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	txhash := c.Param("txhash")
	if txhash == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), errors.ErrValidationEmptyTxHash.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	withResults, err := getQueryParamWithResults(c)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.ErrValidation.Error(),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	tx, err := facade.GetTransaction(txhash, withResults)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrGetTransaction.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"transaction": tx},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns transaction details for a given txhash
// @Tags Transaction
// @Accept  json
// @Produce  json
// @Param txhash path string true "txhash"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "some internal error"
// @Router /transaction/estimate-fee [get]
// EstimateFee returns estimated fee of given tx
func EstimateFee(c *gin.Context) {
}

func getQueryParamWithResults(c *gin.Context) (bool, error) {
	withResultsStr := c.Request.URL.Query().Get(queryParamWithResults)
	if withResultsStr == "" {
		return false, nil
	}

	return strconv.ParseBool(withResultsStr)
}

// SimulateTransaction returns how many gas units a transaction wil consume
func SimulateTransaction(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	var gtx = sendTXRequest{}
	err := c.ShouldBindJSON(&gtx)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	var contracts []json.RawMessage = gtx.Contracts
	if len(contracts) == 0 && gtx.Contract != nil {
		contracts = []json.RawMessage{gtx.Contract}
	}

	baseInfo := &transaction.TXBaseInfo{
		Sender:         gtx.Sender,
		Nonce:          gtx.Nonce,
		SenderUsername: nil,
		DataField:      gtx.Data,
		PermID:         gtx.PermID,
		KDAFee:         gtx.KDAFee,
	}

	start := time.Now()

	tx, _, err := facade.CreateTransaction(gtx.Type, baseInfo, contracts, false)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data: gin.H{
					"result": tx,
					"txHash": "",
				},
				Error: fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	duration := time.Since(start)
	if duration > 200*time.Millisecond {
		log.Debug(fmt.Sprintf("API call: SimulateTransaction took %s", duration))
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: err.Error(),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"bandwidthFee": tx.GetBandwidthFee(), "kappFee": tx.GetKAppFee()},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
