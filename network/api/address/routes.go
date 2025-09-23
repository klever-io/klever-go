package address

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/data/state"

	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const (
	getAccountPath            = "/:address"
	getBalancePath            = "/:address/balance"
	getKDAPath                = "/:address/kda"
	getAccountNoncePath       = "/:address/nonce"
	getAvailableClaimPath     = "/:address/allowance"
	getAvailableClaimListPath = "/:address/allowance/list"

	defaultAsset = "KLV"
)

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	GetAccount(address string) (state.UserAccountHandler, error)
	GetNextNonce(address string) (*models.AccountNonceResponse, error)
	GetBalance(address string, kda string) (*models.BalanceResponse, error)
	GetUserKDA(address string, kda string) (*kapps.UserKDA, error)
	GetAvailableClaim(address string, assetId string) (*models.AvailableClaimResponse, error)
	IsInterfaceNil() bool
}

// Routes defines address related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, getAccountPath, GetAccount)
	router.RegisterHandler(http.MethodGet, getBalancePath, GetBalance)
	router.RegisterHandler(http.MethodGet, getKDAPath, GetKDA)
	router.RegisterHandler(http.MethodGet, getAccountNoncePath, GetAccountNonce)
	router.RegisterHandler(http.MethodGet, getAvailableClaimPath, GetAvailableClaim)
	router.RegisterHandler(http.MethodGet, getAvailableClaimListPath, GetAvailableClaimList)
}

func getFacade(c *gin.Context) (FacadeHandler, bool) {
	facadeObj, ok := c.Get("facade")
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrNilAppContext),
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
				Error: errors.APIErrorString(errors.ErrInvalidAppContext),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return nil, false
	}

	return facade, true
}

// @Summary returns an accountResponse
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Success 200 object shared.GenericAPIResponse{data=object{account=data.AccountInfo}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address} [get]
// GetAccount returns an accountResponse containing information
// about the account correlated with provided address
func GetAccount(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	acc, err := facade.GetAccount(addr)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrCouldNotGetAccount, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"account": acc},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the nonce and pending transaction info for a specific account
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Success 200 object shared.GenericAPIResponse{data=models.AccountNonceResponse} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/nonce [get]
// GetAccountNonce returns an account nonce info
func GetAccountNonce(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	nonceResponse, err := facade.GetNextNonce(addr)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrCouldNotGetAccount, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  nonceResponse,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the balance for the address parameter
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Param asset query string false "asset" default(KLV)
// @Success 200 object shared.GenericAPIResponse{data=models.BalanceResponse} "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/balance [get]
// GetBalance returns the balance for the address parameter
func GetBalance(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	asset := c.Query("asset")
	if asset == "" {
		asset = defaultAsset
	}

	addr := c.Param("address")
	if addr == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  &models.BalanceResponse{Balance: 0},
				Error: errors.APIErrorString(errors.ErrGetBalance, errors.ErrEmptyAddress),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	balanceResponse, err := facade.GetBalance(addr, asset)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  &models.BalanceResponse{Balance: 0},
				Error: errors.APIErrorString(errors.ErrGetBalance, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}
	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  balanceResponse,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the user kda for the address parameter
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Param asset query string false "asset" default(KLV)
// @Success 200 object shared.GenericAPIResponse{data=models.KDAResponse} "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/kda [get]
// GetKDA returns the balance for the address parameter
func GetKDA(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	asset := c.Query("asset")
	if asset == "" {
		asset = defaultAsset
	}

	addr := c.Param("address")
	if addr == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  &models.KDAResponse{UserKDA: nil, Address: addr, Asset: asset},
				Error: errors.APIErrorString(errors.ErrGetUserKDA, errors.ErrEmptyAddress),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	userKDA, err := facade.GetUserKDA(addr, asset)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  &models.KDAResponse{UserKDA: nil, Address: addr, Asset: asset},
				Error: errors.APIErrorString(errors.ErrGetUserKDA, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	response := &models.KDAResponse{
		UserKDA: userKDA,
		Address: addr,
		Asset:   asset,
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  response,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the rewards available for a specific asset in an account
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Param asset query string true "asset"
// @Success 200 object shared.GenericAPIResponse{data=models.AvailableClaimResponse} "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/allowance [get]
// GetAvailableClaim returns the rewards available for a specific asset in an account
func GetAvailableClaim(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	assetId := c.Query("asset")

	if addr == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaim, errors.ErrEmptyAddress),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}
	if assetId == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaim, errors.ErrEmptyAssetId),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	claimResponse, err := facade.GetAvailableClaim(addr, assetId)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaim, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  claimResponse,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the rewards available for a specific list of asset in an account
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Param asset query string true "assets (comma-separated asset IDs), e.g. KLV,KFI"
// @Success 200 object shared.GenericAPIResponse{data=models.AvailableClaimListResponse} "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/allowance/list [get]
// GetAvailableClaimList returns the rewards available for a specific list of asset in an account
func GetAvailableClaimList(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	assets := c.Query("asset") // need to be separated by comma

	// split assets
	raw := strings.Split(assets, ",")
	assetList := make([]string, 0, len(raw))
	for _, s := range raw {
		t := strings.TrimSpace(s)
		if t != "" {
			assetList = append(assetList, t)
		}
	}

	if addr == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaimList, errors.ErrEmptyAddress),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	if len(assetList) <= 0 || assetList[0] == "" {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaimList, errors.ErrEmptyAssetId),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	var wg sync.WaitGroup
	var lock sync.Mutex
	allowanceError := make(map[string]error)
	assetData := make(map[string]*models.AvailableClaimResponse, len(assetList))

	for _, asset := range assetList {
		wg.Add(1)
		go func(assetId string) {
			defer wg.Done()
			claimResponse, err := facade.GetAvailableClaim(addr, assetId)

			lock.Lock()
			defer lock.Unlock()
			if err != nil {
				allowanceError[assetId] = err
				return
			}
			assetData[assetId] = claimResponse
		}(asset)
	}

	wg.Wait()

	if len(allowanceError) > 0 {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: errors.APIErrorString(errors.ErrGetAvailableClaimList, allowanceError),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	response := &models.AvailableClaimListResponse{
		Assets: assetData,
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  response,
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
