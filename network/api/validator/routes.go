package validator

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const statisticsPath = "/statistics"
const peersPath = "/peers"

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error)
	PeersAPI() ([]state.PeerAccountHandler, error)
	IsInterfaceNil() bool
}

// Routes defines validators' related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, statisticsPath, Statistics)
	router.RegisterHandler(http.MethodGet, peersPath, Peers)
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

// @Summary will return the validation statistics for all validators
// @Tags Validator
// @Produce  json
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Router /validator/statistics [get]
// Statistics will return the validation statistics for all validators
func Statistics(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}
	valStats, err := facade.ValidatorStatisticsAPI()
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: err.Error(),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"statistics": valStats},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary will return the peers
// @Tags Validator
// @Produce  json
// @Success 200 object shared.GenericAPIResponse "ok"
// @Failure 400 object shared.GenericAPIResponse "some error"
// @Router /validator/peers [get]
// Peers will return the peers
func Peers(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}
	valStats, err := facade.PeersAPI()
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: err.Error(),
				Code:  shared.ReturnCodeRequestError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"peers": valStats},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
