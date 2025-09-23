package marketplace

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const (
	getMarketplacePath  = "/:id"
	getMarketplacesPath = "/"
)

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	GetMarketplace(marketplaceID string) (*api.Marketplace, error)
	IsInterfaceNil() bool
}

// Routes defines address related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, getMarketplacePath, GetMarketplace)
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

// @Summary returns an marketplaceResponse containing information about the marketplace
// @Tags Marketplace
// @Produce json
// @Param id path string true "marketplace Id"
// @Success 200 object shared.GenericAPIResponse{data=object{marketplace=api.Marketplace}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /marketplace/{id} [get]
// GetMarketplace returns an marketplaceResponse containing information
// about the marketplace correlated with provided marketplace
func GetMarketplace(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	addr := c.Param("id")
	marketplace, err := facade.GetMarketplace(addr)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrCouldNotGetMarketplace.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"marketplace": marketplace},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
