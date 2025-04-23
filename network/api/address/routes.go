package address

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/errors"
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
	GetNextNonce(address string) (uint64, uint64, uint64, error)
	GetBalance(address string, kda string) (int64, error)
	GetUserKDA(address string, kda string) (*kapps.UserKDA, error)
	GetAvailableClaim(address string, assetId string) (int64, map[string]int64, int64, error)
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
// @Success 200 object shared.GenericAPIResponse "ok"
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
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/nonce [get]
// GetAccountNonce returns an account nonce info
func GetAccountNonce(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	accountNonce, firstPendingNonce, txPending, err := facade.GetNextNonce(addr)
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
			Data:  gin.H{"nonce": accountNonce, "firstPendingNonce": firstPendingNonce, "txPending": txPending},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the balance for the address parameter
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Success 200 object shared.GenericAPIResponse "ok"
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
				Data:  gin.H{"balance": 0},
				Error: errors.APIErrorString(errors.ErrGetBalance, errors.ErrEmptyAddress),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	balance, err := facade.GetBalance(addr, asset)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  gin.H{"balance": 0},
				Error: errors.APIErrorString(errors.ErrGetBalance, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}
	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"balance": balance},
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
// @Success 200 object shared.GenericAPIResponse "ok"
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
				Data:  gin.H{"userKDA": nil, "address": addr, "asset": asset},
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
				Data:  gin.H{"userKDA": nil, "address": addr, "asset": asset},
				Error: errors.APIErrorString(errors.ErrGetUserKDA, err),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}
	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"userKDA": userKDA, "address": addr, "asset": asset},
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
// @Success 200 object shared.GenericAPIResponse "ok"
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

	rewards, allRewards, allowance, err := facade.GetAvailableClaim(addr, assetId)
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
			Data:  gin.H{"stakingRewards": rewards, "allStakingRewards": allRewards, "allowance": allowance},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the rewards available for a specific list of asset in an account
// @Tags Address
// @Produce json
// @Param address path string true "address"
// @Param asset query string true "asset"
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /address/{address}/allowance [get]
// GetAvailableClaimList returns the rewards available for a specific list of asset in an account
func GetAvailableClaimList(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("address")
	assets := c.Query("asset") // need to be separated by comma

	// split assets
	assetList := strings.Split(assets, ",")

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
	assetData := make(map[string]interface{})

	for _, asset := range assetList {
		wg.Add(1)
		go func(assetId string) {
			defer wg.Done()
			rewards, allRewards, allowance, err := facade.GetAvailableClaim(addr, assetId)

			defer lock.Unlock()
			lock.Lock()
			if err != nil {
				allowanceError[assetId] = err
				return
			}
			assetData[assetId] = gin.H{"stakingRewards": rewards, "allStakingRewards": allRewards, "allowance": allowance}
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

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"assets": assetData},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
