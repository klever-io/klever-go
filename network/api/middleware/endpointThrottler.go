package middleware

import (
	"bytes"
	stdErrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
)

type throttlerGetter interface {
	GetThrottlerForEndpoint(endpoint string) (core.Throttler, bool)
}

// CreateEndpointThrottler will create a middleware-type of handler to be used in conjunction with special
// REST API end points that need to be better protected
func CreateEndpointThrottler(throttlerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tgObj, ok := c.Get("facade")
		if !ok {
			c.JSON(
				http.StatusInternalServerError,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: errors.ErrNilAppContext.Error(),
					Code:  shared.ReturnCodeInternalError,
				},
			)
			return
		}

		tg, ok := tgObj.(throttlerGetter)
		if !ok {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: errors.ErrInvalidAppContext.Error(),
					Code:  shared.ReturnCodeInternalError,
				},
			)
			return
		}

		endpointThrottler, ok := tg.GetThrottlerForEndpoint(throttlerName)
		if !ok {
			c.Next()
			return
		}

		// Read the body before taking a slot, so a client that sends its body slowly
		// holds no endpoint slot while it does. The body is already capped in size
		// and read time by the http server, and the handler would read it all anyway.
		if !bufferRequestBody(c) {
			return
		}

		if !endpointThrottler.TryStartProcessing() {
			c.AbortWithStatusJSON(
				http.StatusTooManyRequests,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: fmt.Sprintf("%s for endpoint %s", errors.ErrTooManyRequests.Error(), throttlerName),
					Code:  shared.ReturnCodeSystemBusy,
				},
			)
			return
		}

		defer endpointThrottler.EndProcessing()

		c.Next()
	}
}

// bufferRequestBody reads the whole request body and puts it back for the handler.
// It answers the request itself and returns false when the body cannot be read.
//
// GET and HEAD handlers never read a body, so theirs is left alone. The original body
// is never closed here: net/http closes it after the handler chain returns, outside
// every throttler. Closing it here would drain an unread body with the read deadline
// already cleared, holding the throttler slots until the client disconnects.
func bufferRequestBody(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return true
	}
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return true
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if stdErrors.As(err, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
		}
		shared.RespondWith(c, status, nil, fmt.Sprintf("%s: %s", errors.ErrValidation.Error(), err.Error()), shared.ReturnCodeRequestError)
		c.Abort()
		return false
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	return true
}
