package api

import (
	"bytes"
	"context"
	"net/http"
	"reflect"

	"github.com/klever-io/klever-go/network/api/vm"

	"github.com/klever-io/klever-go/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	indexer "github.com/klever-io/klever-go/indexer"
	clientSocket "github.com/klever-io/klever-go/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/asset"
	"github.com/klever-io/klever-go/network/api/block"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/klever-io/klever-go/network/api/marketplace"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/network"
	"github.com/klever-io/klever-go/network/api/node"
	"github.com/klever-io/klever-go/network/api/transaction"
	valStats "github.com/klever-io/klever-go/network/api/validator"
	wsocket "github.com/klever-io/klever-go/network/api/websocket"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"gopkg.in/go-playground/validator.v8"
)

var log = logger.GetOrCreate("api")

type validatorInput struct {
	Name      string
	Validator validator.Func
}

type ginWriter struct {
}

func (gv *ginWriter) Write(p []byte) (n int, err error) {
	trimmed := bytes.TrimSpace(p)
	log.Trace("gin server", "message", string(trimmed))

	return len(p), nil
}

type ginErrorWriter struct {
}

func (gev *ginErrorWriter) Write(p []byte) (n int, err error) {
	trimmed := bytes.TrimSpace(p)
	log.Trace("gin server", "error", string(trimmed))

	return len(p), nil
}

// Start will boot up the api and appropriate routes, handlers and validators
func Start(ctx context.Context, kleverFacade MainAPIHandler, routesConfig config.APIRoutesConfig, processors ...MiddlewareProcessor) error {
	var ws *gin.Engine
	if !kleverFacade.RestAPIServerDebugMode() {
		gin.DefaultWriter = &ginWriter{}
		gin.DefaultErrorWriter = &ginErrorWriter{}
		gin.DisableConsoleColor()
		gin.SetMode(gin.ReleaseMode)
	}
	ws = gin.Default()
	ws.Use(cors.Default())
	ws.Use(middleware.WithFacade(kleverFacade))
	for _, proc := range processors {
		if check.IfNil(proc) {
			continue
		}

		ws.Use(proc.MiddlewareHandlerFunc())
	}

	err := RegisterDefaultValidators()
	if err != nil {
		return err
	}

	ws.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName(docs.SwaggerInfonode.InstanceName())))
	RegisterRoutes(ctx, ws, routesConfig, kleverFacade)

	return ws.Run(kleverFacade.RestAPIInterface())
}

// RegisterRoutes will register all routes available on the web server
func RegisterRoutes(ctx context.Context, ws *gin.Engine, routesConfig config.APIRoutesConfig, kleverFacade middleware.Handler) {
	authHandler := middleware.NewAuthenticationFunc(routesConfig)

	nodeRoutes := ws.Group("/node")
	wrappedNodeRouter, err := wrapper.NewRouterWrapper("node", nodeRoutes, routesConfig, authHandler)
	if err == nil {
		node.Routes(wrappedNodeRouter)
	}

	addressRoutes := ws.Group("/address")
	wrappedAddressRouter, err := wrapper.NewRouterWrapper("address", addressRoutes, routesConfig, authHandler)
	if err == nil {
		address.Routes(wrappedAddressRouter)
	}

	networkRoutes := ws.Group("/network")
	wrappedNetworkRoutes, err := wrapper.NewRouterWrapper("network", networkRoutes, routesConfig, authHandler)
	if err == nil {
		network.Routes(wrappedNetworkRoutes)
	}

	txRoutes := ws.Group("/transaction")
	wrappedTransactionRouter, err := wrapper.NewRouterWrapper("transaction", txRoutes, routesConfig, authHandler)
	if err == nil {
		transaction.Routes(wrappedTransactionRouter)
	}

	validatorRoutes := ws.Group("/validator")
	wrappedValidatorsRouter, err := wrapper.NewRouterWrapper("validator", validatorRoutes, routesConfig, authHandler)
	if err == nil {
		valStats.Routes(wrappedValidatorsRouter)
	}

	blockRoutes := ws.Group("/block")
	wrappedBlockRouter, err := wrapper.NewRouterWrapper("block", blockRoutes, routesConfig, authHandler)
	if err == nil {
		block.Routes(wrappedBlockRouter)
	}

	assetRoutes := ws.Group("/asset")
	wrappedAssetRouter, err := wrapper.NewRouterWrapper("asset", assetRoutes, routesConfig, authHandler)
	if err == nil {
		asset.Routes(wrappedAssetRouter)
	}

	marketplaceRoutes := ws.Group("/marketplace")
	wrappedMarketplaceRouter, err := wrapper.NewRouterWrapper("marketplace", marketplaceRoutes, routesConfig, authHandler)
	if err == nil {
		marketplace.Routes(wrappedMarketplaceRouter)
	}

	vmRoutes := ws.Group("/vm")
	wrappedVMRouter, err := wrapper.NewRouterWrapper("vm", vmRoutes, routesConfig, authHandler)
	if err == nil {
		vm.Routes(wrappedVMRouter)
	}

	apiHandler, ok := kleverFacade.(MainAPIHandler)
	if ok && apiHandler.PprofEnabled() {
		pprof.Register(ws)
	}

	if isLogRouteEnabled(routesConfig) {
		marshalizerForLogs := &marshal.ProtoMarshalizer{}
		registerLoggerWsRoute(ws, marshalizerForLogs)
	}

	if isSubscriptionRouteEnabled(routesConfig) {
		var postConnUrl, postConnApiKey string
		if ok {
			postConnUrl, postConnApiKey = apiHandler.WSConnectionURL(), apiHandler.WSConnectionAPIKey()
		}

		var wsFacade clientSocket.WSFacade
		if f, ok := kleverFacade.(clientSocket.WSFacade); ok {
			wsFacade = f
		}

		indexer.UseEventQueue = true
		hub := clientSocket.NewHub(postConnUrl, postConnApiKey, wsFacade)
		wsocket.SubscribeTopics(ws, hub)
		go hub.StartServer(ctx)
	}
}

func isSubscriptionRouteEnabled(routesConfig config.APIRoutesConfig) bool {
	logConfig, ok := routesConfig.APIPackages["subscribe"]
	if !ok {
		return false
	}

	for _, cfg := range logConfig.Routes {
		if cfg.Name == "/subscribe" && cfg.Open {
			return true
		}
	}

	return false
}

func isLogRouteEnabled(routesConfig config.APIRoutesConfig) bool {
	logConfig, ok := routesConfig.APIPackages["log"]
	if !ok {
		return false
	}

	for _, cfg := range logConfig.Routes {
		if cfg.Name == "/log" && cfg.Open {
			return true
		}
	}

	return false
}

// RegisterDefaultValidators will call register validation on all validator functions
func RegisterDefaultValidators() error {
	validators := []validatorInput{
		{Name: "skValidator", Validator: skValidator},
	}
	for _, validatorFunc := range validators {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			err := v.RegisterValidation(validatorFunc.Name, validatorFunc.Validator)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func registerLoggerWsRoute(ws *gin.Engine, marshalizer marshal.Marshalizer) {
	upgrader := websocket.Upgrader{}

	ws.GET("/log", func(c *gin.Context) {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls, err := logs.NewLogSender(marshalizer, conn, log)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls.StartSendingBlocking()
	})
}

// skValidator validates a secret key from user input for correctness
func skValidator(
	_ *validator.Validate,
	_ reflect.Value,
	_ reflect.Value,
	_ reflect.Value,
	_ reflect.Type,
	_ reflect.Kind,
	_ string,
) bool {
	return true
}
