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
	"github.com/klever-io/klever-go/network/api/httpserver"
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

const (
	logPackage = "log"
	logRoute   = "/log"
)

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

	// Hardened http.Server instead of ws.Run: adds the ReadHeaderTimeout that
	// http.ListenAndServe lacks (slow-header DoS, GHSA-w4c6-7r69-w7j9).
	return httpserver.NewHardenedServer(kleverFacade.RestAPIInterface(), ws.Handler()).ListenAndServe()
}

func registerRouteGroup(ws *gin.Engine, name string, routesConfig config.APIRoutesConfig, authHandler gin.HandlerFunc, register func(*wrapper.RouterWrapper)) {
	group := ws.Group("/" + name)
	w, err := wrapper.NewRouterWrapper(name, group, routesConfig, authHandler)
	if err == nil {
		register(w)
	}
}

// RegisterRoutes will register all routes available on the web server
func RegisterRoutes(ctx context.Context, ws *gin.Engine, routesConfig config.APIRoutesConfig, kleverFacade middleware.Handler) {
	authHandler := middleware.NewAuthenticationFunc(routesConfig)

	registerRouteGroup(ws, "node", routesConfig, authHandler, node.Routes)
	registerRouteGroup(ws, "address", routesConfig, authHandler, address.Routes)
	registerRouteGroup(ws, "network", routesConfig, authHandler, network.Routes)
	registerRouteGroup(ws, "transaction", routesConfig, authHandler, transaction.Routes)
	registerRouteGroup(ws, "validator", routesConfig, authHandler, valStats.Routes)
	registerRouteGroup(ws, "block", routesConfig, authHandler, block.Routes)
	registerRouteGroup(ws, "asset", routesConfig, authHandler, asset.Routes)
	registerRouteGroup(ws, "marketplace", routesConfig, authHandler, marketplace.Routes)
	registerRouteGroup(ws, "vm", routesConfig, authHandler, vm.Routes)

	apiHandler, ok := kleverFacade.(MainAPIHandler)
	if ok && apiHandler.PprofEnabled() {
		pprof.Register(ws)
	}

	if routesConfig.IsRouteEnabled(logPackage, logRoute) {
		marshalizerForLogs := &marshal.ProtoMarshalizer{}
		registerLoggerWsRoute(ws, marshalizerForLogs, routesConfig)
	}

	if routesConfig.IsRouteEnabled("subscribe", "/subscribe") {
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

func registerLoggerWsRoute(ws *gin.Engine, marshalizer marshal.Marshalizer, routesConfig config.APIRoutesConfig) {
	upgrader := websocket.Upgrader{}

	// Only an authenticated (secured) /log may apply a client-supplied logger profile to the
	// process-global logger; on an unauthenticated /log profiles are ignored (GHSA-9v8p-frvj-2pcm).
	secured := routesConfig.IsRouteSecured(logPackage, logRoute)

	logHandler := func(c *gin.Context) {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls, err := logs.NewLogSender(marshalizer, conn, log, secured)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls.StartSendingBlocking()
	}

	handlers := []gin.HandlerFunc{logHandler}
	if secured {
		// GHSA-9v8p-frvj-2pcm / KLC-2438: enforce authentication before the WebSocket
		// upgrade when /log is marked secured. This route is registered directly on the
		// engine (outside the RouterWrapper), so without this the `secured: true` flag
		// was silently ignored and /log was always unauthenticated.
		handlers = append([]gin.HandlerFunc{middleware.NewAuthenticationFunc(routesConfig)}, handlers...)
	}

	ws.GET(logRoute, handlers...)
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
