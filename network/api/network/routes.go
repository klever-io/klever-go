package network

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const (
	getConfigPath          = "/config"
	getStatusPath          = "/status"
	economicsPath          = "/economics"
	proposalParametersPath = "/network-parameters"
)

// FacadeHandler interface defines methods that can be used by the gin webserver
type FacadeHandler interface {
	StatusMetrics() core.StatusMetricsHandler
	GetProposalParameters() (map[int32]*kapps.Parameter, error)
	GetEconomics() (*models.EconomicsResponse, error)
	IsInterfaceNil() bool
}

type parameterResponse struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Routes defines address related routes
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, getConfigPath, GetNetworkConfig)
	router.RegisterHandler(http.MethodGet, getStatusPath, GetNetworkStatus)
	router.RegisterHandler(http.MethodGet, proposalParametersPath, GetProposalParameters)
	router.RegisterHandler(http.MethodGet, economicsPath, GetEconomics)
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

// @Summary returns metrics related to the network configuration
// @Tags Network
// @Produce json
// @Success 200 object shared.GenericAPIResponse{data=object{config=object}} "ok"
// @Router /network/config [get]
// GetNetworkConfig returns metrics related to the network configuration
func GetNetworkConfig(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	configMetrics := facade.StatusMetrics().ConfigMetrics()
	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"config": configMetrics},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns metrics related to the network status (shard specific)
// @Tags Network
// @Produce json
// @Success 200 object shared.GenericAPIResponse{data=object{status=object}} "ok"
// @Router /network/status [get]
// GetNetworkStatus returns metrics related to the network status (shard specific)
func GetNetworkStatus(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	networkMetrics := facade.StatusMetrics().NetworkMetrics()
	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"status": networkMetrics},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns the active parameters of the proposal controller
// @Tags Network
// @Produce json
// @Success 200 object shared.GenericAPIResponse{data=object{parameters=object}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /network/network-parameters [get]
// GetProposalParameters returns the active parameters of the proposal controller
func GetProposalParameters(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	proposalParameters, err := facade.GetProposalParameters()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrGetProposalParameters.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	parameterResp := make(map[int32]*parameterResponse, len(proposalParameters))
	for i, parameter := range proposalParameters {
		parameterResp[i] = &parameterResponse{
			Type:  parameter.Type.Enum().String(),
			Value: string(parameter.Value),
		}
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"parameters": parameterResp},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}

// @Summary returns KLV economics: supply figures plus node-state held aggregates
// @Tags Network
// @Produce json
// @Success 200 object shared.GenericAPIResponse{data=object{economics=models.EconomicsResponse}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /network/economics [get]
// GetEconomics returns live KLV supply figures and node-state held aggregates. See KLC-2506.
func GetEconomics(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	economics, err := facade.GetEconomics()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			shared.GenericAPIResponse{
				Data:  nil,
				Error: fmt.Sprintf("%s: %s", errors.ErrGetEconomics.Error(), err.Error()),
				Code:  shared.ReturnCodeInternalError,
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"economics": economics},
			Error: "",
			Code:  shared.ReturnCodeSuccess,
		},
	)
}
