package api

import "github.com/gin-gonic/gin"

// MiddlewareProcessor defines a processor used internally by the web server when processing requests
type MiddlewareProcessor interface {
	MiddlewareHandlerFunc() gin.HandlerFunc
	IsInterfaceNil() bool
}

// MainAPIHandler interface defines methods that can be used from `kleverFacade` context variable
type MainAPIHandler interface {
	RestAPIInterface() string
	RestAPIServerDebugMode() bool
	PprofEnabled() bool
	WSConnectionURL() string
	WSConnectionAPIKey() string
	IsInterfaceNil() bool
}
