package asset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const (
	getAssetPath     = "/:id"
	getAssetsPath    = "/"
	getNFTAssetsPath = "/nft/:owner/*id"
	getKDAPoolPath   = "/pool/:id/"

	klvAssetID = "KLV"
	kfiAssetID = "KFI"
)

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	GetAsset(assetID string) (*kapps.KDAData, error)
	GetNFT(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error)
	GetKDAFeePool(address string) (*kdafeespool.KDAFeesPoolData, error)
	IsInterfaceNil() bool
}

// Routes defines address related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, getNFTAssetsPath, GetNFT)
	router.RegisterHandler(http.MethodGet, getAssetPath, GetAsset)
	router.RegisterHandler(http.MethodGet, getKDAPoolPath, GetKDAFeePool)
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

// @Summary returns an assetResponse containing information about the asset
// @Tags Asset
// @Produce json
// @Param id path string true "asset id"
// @Success 200 object shared.GenericAPIResponse{data=object{asset=kapps.KDAData}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /asset/{id} [get]
// GetAsset returns an assetResponse containing information
//
//	about the asset correlated with provided asset
func GetAsset(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("id")
	asset, err := facade.GetAsset(addr)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrCouldNotGetAsset.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	// We replace the KDA info for KLV and KFI to avoid showing outdated information that is registered at genesis.
	replaceKDAInfo(asset)

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"asset": asset},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// replaceKDAInfo overrides the Logo and URIs returned for KLV and KFI.
// Their values are registered at genesis and updating them on-chain would
// require a hard fork, so we patch the API response instead.
func replaceKDAInfo(asset *kapps.KDAData) {
	if asset == nil {
		return
	}

	id := string(asset.ID)
	if id != klvAssetID && id != kfiAssetID {
		return
	}

	asset.Logo = "https://kleverscan.org/assets/klv-logo.png"
	if id == kfiAssetID {
		asset.Logo = "https://kleverscan.org/assets/kfi-logo.png"
	}

	asset.URIs = map[string]string{
		"Whitepaper": "https://bc.klever.finance/wp",
		"Exchange":   "https://bitcoin.me/",
		"Github":     "https://github.com/klever-io",
		"Instagram":  "https://instagram.com/klever.io",
		"Twitter":    "https://x.com/klever_org",
		"Wallet":     "https://klever.io/",
		"Website":    "https://klever.org/",
	}
}

type NFTAsset struct {
	*kapps.KDAData
	UserKDA *kapps.UserKDA `json:"userKDA"`
}

// @Summary returns an assetResponse containing information about the asset
// @Tags Asset
// @Produce json
// @Param owner path string true "asset owner"
// @Param id path string true "asset id"
// @Success 200 object shared.GenericAPIResponse{data=object{asset=NFTAsset}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /asset/nft/{owner}/{id} [get]
// GetNFT returns an nftAssetResponse containing information
//
//	about the asset correlated with provided asset
func GetNFT(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("owner")
	id := c.Param("id")
	userKDA, asset, err := facade.GetNFT(addr, id)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrCouldNotGetAsset.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	info := NFTAsset{
		asset,
		userKDA,
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"asset": info},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns an poolResponse containing information about the asset pool
// @Tags Asset
// @Produce json
// @Param id path string true "asset id"
// @Success 200 object shared.GenericAPIResponse{data=object{pool=kdafeespool.KDAFeesPoolData}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /asset/pool/{id} [get]
// GetKDAFeePool returns an poolResponse containing information
//
//	about the kda fee pool of asset correlated with provided asset
func GetKDAFeePool(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("id")
	pool, err := facade.GetKDAFeePool(addr)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrCouldNotGetAsset.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"pool": pool},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
