package api

import (
	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core"
)

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
	WSMaxConnections() uint32
	WSMaxConnectionsPerIP() uint32
	WSMaxAddressesPerSubscribe() uint32
	WSMaxAddressesPerClient() uint32
	WSPostWorkers() uint32
	WSPostQueueSize() uint32
	// AppStatusHandler exposes the node's status/metrics sink, so the /subscribe mirror
	// worker pool can export its drop/failure counters (see
	// core.MetricWSMirrorQueueDroppedTotal/MetricWSMirrorPostFailuresTotal) alongside every
	// other node metric instead of only logging them.
	AppStatusHandler() core.AppStatusHandler
	IsInterfaceNil() bool
}
