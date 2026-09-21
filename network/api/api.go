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
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/asset"
	"github.com/klever-io/klever-go/network/api/block"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/klever-io/klever-go/network/api/marketplace"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/network"
	"github.com/klever-io/klever-go/network/api/node"
	"github.com/klever-io/klever-go/network/api/shared"
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

	subscribePackage = "subscribe"
	subscribeRoute   = "/subscribe"

	nodePackage = "node"

	// defaultLogWSMaxConnections is the node-wide /log cap used when logWebSocketConnections
	// resolves to 0 (a config.yaml predating the key). It cannot be disabled: before the cap
	// existed, live /log connections still ran inside the gin global throttler's slot and were
	// bounded by simultaneousRequests. Streaming now runs off the request goroutine, so that
	// slot is released at the upgrade — treating 0 as "unlimited" here would leave an
	// unmigrated node strictly weaker than before. To lift the cap, set an explicit high value
	// (the same rule the address caps follow).
	defaultLogWSMaxConnections = 32
)

// nodeDiagnosticRoutes expose cached interceptor and resolver state, peer addresses
// and validator keys, and the node's own p2p listen addresses.
var nodeDiagnosticRoutes = []string{"/debug", "/p2pstatus", "/peerinfo", "/heartbeatstatus"}

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
		var logMaxConns, logMaxConnsPerIP uint32
		var logAllowedOrigins []string
		if ok {
			logMaxConns = apiHandler.LogWSMaxConnections()
			logMaxConnsPerIP = apiHandler.LogWSMaxConnectionsPerIP()
			logAllowedOrigins = apiHandler.LogWSAllowedOrigins()
		}
		registerLoggerWsRoute(ws, &marshal.ProtoMarshalizer{}, routesConfig, logMaxConns, logMaxConnsPerIP, logAllowedOrigins)
	}

	// secured only attaches the auth handler; the route is registered on open. Warn on
	// the secured-but-not-open footgun so it does not read as "auth-protected" when it
	// is actually absent.
	if routesConfig.IsRouteSecured(subscribePackage, subscribeRoute) && !routesConfig.IsRouteEnabled(subscribePackage, subscribeRoute) {
		log.Warn("subscribe route has secured:true but open:false; /subscribe will not be registered. Set open:true to enable it (secured then requires Basic Auth).")
	}

	// Upgrading the binary does not rewrite an operator's api.yaml, so a node installed
	// before the diagnostic routes were secured keeps serving cached internal state and
	// network topology unauthenticated with nothing to signal it. The edit is the
	// operator's to make.
	for _, route := range nodeDiagnosticRoutes {
		if routesConfig.IsRouteEnabled(nodePackage, route) && !routesConfig.IsRouteSecured(nodePackage, route) {
			log.Warn("node diagnostic route is open but not secured; it answers unauthenticated. Add secured:true to it in api.yaml.", "route", nodePackage+route)
		}
	}

	if routesConfig.IsRouteEnabled(subscribePackage, subscribeRoute) {
		var postConnUrl, postConnApiKey string
		var subscribeOpts wsocket.SubscribeOptions
		var hubLimits clientSocket.Limits
		var appStatusHandler core.AppStatusHandler
		if ok {
			postConnUrl, postConnApiKey = apiHandler.WSConnectionURL(), apiHandler.WSConnectionAPIKey()
			subscribeOpts.MaxConnections = apiHandler.WSMaxConnections()
			subscribeOpts.MaxConnectionsPerIP = apiHandler.WSMaxConnectionsPerIP()
			hubLimits.MaxAddressesPerSubscribe = wsocket.ClampUint32ToInt(apiHandler.WSMaxAddressesPerSubscribe())
			hubLimits.MaxAddressesPerClient = wsocket.ClampUint32ToInt(apiHandler.WSMaxAddressesPerClient())
			hubLimits.PostWorkers = wsocket.ClampUint32ToInt(apiHandler.WSPostWorkers())
			hubLimits.PostQueueSize = wsocket.ClampUint32ToInt(apiHandler.WSPostQueueSize())
			appStatusHandler = apiHandler.AppStatusHandler()
		}
		// Honour `secured: true` for /subscribe. The route is registered directly on the
		// engine (not via the RouterWrapper), so the auth handler must be applied here or
		// the flag is silently ignored (GHSA-4fwh-wrm6-97xm).
		if routesConfig.IsRouteSecured(subscribePackage, subscribeRoute) {
			subscribeOpts.AuthHandlers = []gin.HandlerFunc{authHandler}
		}

		var wsFacade clientSocket.WSFacade
		if f, ok := kleverFacade.(clientSocket.WSFacade); ok {
			wsFacade = f
		}

		indexer.UseEventQueue = true
		hub := clientSocket.NewHub(postConnUrl, postConnApiKey, wsFacade, hubLimits)
		hub.SetAppStatusHandler(appStatusHandler)
		wsocket.SubscribeTopics(ws, hub, subscribeOpts)
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

func registerLoggerWsRoute(
	ws *gin.Engine,
	marshalizer marshal.Marshalizer,
	routesConfig config.APIRoutesConfig,
	maxConns uint32,
	maxConnsPerIP uint32,
	allowedOrigins []string,
) {
	// Built once and never mutated afterwards. The origin check used to be assigned on every
	// request, which raced with Upgrade reading it as soon as two clients dialled /log at the
	// same time — the exact concurrency this route now invites.
	upgrader := wsocket.NewUpgrader(allowedOrigins)

	if maxConns == 0 {
		maxConns = defaultLogWSMaxConnections
		log.Warn("logWebSocketConnections is 0; applying the built-in /log cap",
			"maxConnections", maxConns)
	}
	// maxConnsPerIP keeps 0 = unlimited: behind a reverse proxy every client shares the proxy's
	// IP, so the per-IP dimension has to stay disableable. It is a new protection, not a
	// replacement for one, and maxConns still bounds the route when it is off.
	limiter := wsocket.NewConnLimiter(maxConns, maxConnsPerIP)
	// A rejected upgrade is peer-driven: a page an operator visits can retry an unlisted
	// origin at will on cached credentials, and on an unsecured /log a bare GET does the same.
	// One budget for the route, as /subscribe has: one line per window with the count.
	upgradeFailWarn := clientSocket.NewDropWarner(clientSocket.PeerDrivenLogWindow)

	// Only an authenticated (secured) /log may apply a client-supplied logger profile to the
	// process-global logger; on an unauthenticated /log profiles are ignored (GHSA-9v8p-frvj-2pcm).
	secured := routesConfig.IsRouteSecured(logPackage, logRoute)

	logHandler := func(c *gin.Context) {
		release, ok := limiter.Acquire(wsocket.RemoteIP(c.Request))
		if !ok {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, map[string]string{"error": "too many websocket connections"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			release()
			if count, ok := upgradeFailWarn.Fire(); ok {
				log.Warn("/log websocket upgrade failed", "error", shared.QuoteForLog(err.Error()), "similarSinceLastLog", count)
			}
			return
		}

		ls, err := logs.NewLogSender(marshalizer, conn, log, secured)
		if err != nil {
			release()
			_ = conn.Close()
			log.Error("/log cannot create log sender", "error", shared.QuoteForLog(err.Error()))
			return
		}

		// Streaming no longer runs on the request goroutine, so gin.Recovery() no longer
		// contains it; SafeGo does, and release() runs on the unwind.
		shared.SafeGo(log, "/log stream", conn, func() {
			defer release()
			ls.StartSendingBlocking()
		})
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
