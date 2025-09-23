package vm

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/klever-io/klever-go/vmcommon"
)

const (
	hexPath    = "/hex"
	stringPath = "/string"
	intPath    = "/int"
	queryPath  = "/query"
)

// vmValuesFacadeHandler defines the methods to be implemented by a facade for vm-values requests
type FacadeHandler interface {
	ExecuteSCQuery(*process.SCQuery) (*vm.VMOutputApi, error)
	DecodeAddressPubkey(pk string) ([]byte, error)
	IsInterfaceNil() bool
}

func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodPost, hexPath, getHex)
	router.RegisterHandler(http.MethodPost, stringPath, getString)
	router.RegisterHandler(http.MethodPost, intPath, getInt)
	router.RegisterHandler(http.MethodPost, queryPath, executeQuery)
}

// VMValueRequest represents the structure on which user input for generating a new transaction will validate against
type VMValueRequest struct {
	ScAddress      string           `json:"scAddress"`
	FuncName       string           `json:"funcName"`
	CallerAddr     string           `json:"caller"`
	CallValue      map[string]int64 `json:"value"`
	Args           []string         `json:"args"`
	SameScState    bool             `json:"sameScState"`
	ShouldBeSynced bool             `json:"shouldBeSynced"`
}

// @Summary  returns the data as bytes, hex-encoded
// @Tags VM
// @Produce json
// @Param data body vm.VMValueRequest true "vm request data"
// @Success 200 object shared.GenericAPIResponse{data=object{data=string}} "ok"
// @Failure 400 object shared.GenericAPIResponse "bad request"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /vm/hex [post]
func getHex(context *gin.Context) {
	doGetVMValue(context, vm.AsHex)
}

// @Summary  returns the data as string
// @Tags VM
// @Produce json
// @Param data body vm.VMValueRequest true "vm request data"
// @Success 200 object shared.GenericAPIResponse{data=object{data=string}} "ok"
// @Failure 400 object shared.GenericAPIResponse "bad request"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /vm/string [post]
func getString(context *gin.Context) {
	doGetVMValue(context, vm.AsString)
}

// @Summary  returns the data as big int
// @Tags VM
// @Produce json
// @Param data body vm.VMValueRequest true "vm request data"
// @Success 200 object shared.GenericAPIResponse{data=object{data=string}} "ok"
// @Failure 400 object shared.GenericAPIResponse "bad request"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /vm/int [post]
func getInt(context *gin.Context) {
	doGetVMValue(context, vm.AsBigIntString)
}

func doGetVMValue(c *gin.Context, asType vm.ReturnDataKind) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	vmOutput, execErrMsg, err := doExecuteQuery(facade, c)
	if err != nil {
		returnBadRequest(c, "doGetVMValue", err)
		return
	}

	returnData, err := vmOutput.GetFirstReturnData(asType)
	if err != nil {
		execErrMsg += " " + err.Error()
	}

	returnOkResponse(c, returnData, execErrMsg)
}

// @Summary  returns the VM output of a smart contract query
// @Tags VM
// @Produce json
// @Param data body vm.VMValueRequest true "vm request data"
// @Success 200 object shared.GenericAPIResponse{data=object{data=vm.VMOutputApi}} "ok"
// @Failure 500 object shared.GenericAPIResponse "internal error"
// @Router /vm/query [post]
func executeQuery(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	vmOutput, execErrMsg, err := doExecuteQuery(facade, c)
	if err != nil {
		returnBadRequest(c, "executeQuery", err)
		return
	}

	returnOkResponse(c, vmOutput, execErrMsg)
}

func doExecuteQuery(facade FacadeHandler, c *gin.Context) (*vm.VMOutputApi, string, error) {
	request := VMValueRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		return nil, "", errors.ErrInvalidJSONRequest
	}

	command, err := createSCQuery(facade, &request)
	if err != nil {
		return nil, "", err
	}

	vmOutputApi, err := facade.ExecuteSCQuery(command)
	if err != nil {
		return nil, "", err
	}

	vmExecErrMsg := ""
	if len(vmOutputApi.ReturnCode) > 0 && vmOutputApi.ReturnCode != vmcommon.Ok.String() {
		vmExecErrMsg = vmOutputApi.ReturnCode + ":" + vmOutputApi.ReturnMessage
	}

	return vmOutputApi, vmExecErrMsg, nil
}

func createSCQuery(facade FacadeHandler, request *VMValueRequest) (*process.SCQuery, error) {
	decodedAddress, err := facade.DecodeAddressPubkey(request.ScAddress)
	if err != nil {
		return nil, fmt.Errorf("'%s' is not a valid address: %s", request.ScAddress, err.Error())
	}

	arguments := make([][]byte, len(request.Args))
	var argBytes []byte
	for i, arg := range request.Args {
		argBytes, err = hex.DecodeString(arg)
		if err != nil {
			return nil, fmt.Errorf("'%s' is not a valid hex string: %s", arg, err.Error())
		}

		arguments[i] = append(arguments[i], argBytes...)
	}

	scQuery := &process.SCQuery{
		ScAddress:      decodedAddress,
		FuncName:       request.FuncName,
		Arguments:      arguments,
		SameScState:    request.SameScState,
		ShouldBeSynced: request.ShouldBeSynced,
	}

	if len(request.CallerAddr) > 0 {
		callerAddress, errDecodeCaller := facade.DecodeAddressPubkey(request.CallerAddr)
		if errDecodeCaller != nil {
			return nil, errDecodeCaller
		}

		scQuery.CallerAddr = callerAddress
	}

	scQuery.CallValue = request.CallValue

	return scQuery, nil
}

func returnBadRequest(context *gin.Context, errScope string, err error) {
	message := fmt.Sprintf("%s: %s", errScope, err)
	context.JSON(
		http.StatusBadRequest,
		shared.GenericAPIResponse{
			Data:  nil,
			Error: message,
			Code:  shared.ReturnCodeRequestError,
		},
	)
}

func returnOkResponse(context *gin.Context, data interface{}, errorMsg string) {
	context.JSON(
		http.StatusOK,
		shared.GenericAPIResponse{
			Data:  gin.H{"data": data},
			Error: errorMsg,
			Code:  shared.ReturnCodeSuccess,
		},
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
